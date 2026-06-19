# hush — spec (draft for sign-off)

A local, zero-service CLI for per-worktree secrets that is **smooth for humans,
strict for agents**. Secrets live encrypted outside the repo; the worktree only
holds references. Plaintext exists only inside the child process of `hush run`.

---

## Goals / Non-goals

**Goals**
- `cat .env` / `grep -r KEY` in a worktree yields **key names only, never values**.
- New worktree is usable immediately (reference file is committed; values inherited).
- Fork a worktree's private env in ~2 commands.
- Human terminal feels direnv-smooth; agent context is auto-detected and locked to broker mode.

**Non-goals (v1)**
- No team sharing / multi-recipient (single-user, macOS only).
- No audit log, rotation, dynamic/short-lived creds.
- Not a defense against a *same-user malicious agent that actively calls `hush`*.
  See Threat boundary.

---

## Files & formats

> **`.hush.toml` never contains values.** It is a declaration (which keys, how to
> pick a profile, project identity). All values live encrypted in `store.age` and are
> changed only with `hush set` — never by hand-editing a file.

**In repo (committed, no secrets):** `.hush.toml`
```toml
project = "bridge-deposit"   # namespace in store.age; omit → derived from repo path
profile = "branch"           # branch | cwd | fixed:<name>  — how to pick a profile
extends = "base"             # fallback chain when a key is absent in the active profile
keys = ["DATABASE_URL", "DEPLOYER_KEY", "STRIPE_KEY"]   # declared contract
shims = ["forge", "cast"]    # commands to auto-wrap with `hush run` (opt-in; empty = off)
```

**Central store (outside repo):** `~/.config/hush/store.age`
- One global encrypted file. Cleartext is namespaced **project → profile → key**, so
  different projects on the same branch name never collide:
  ```jsonc
  {
    "bridge-deposit": {
      "base":      { "DATABASE_URL": "...", "DEPLOYER_KEY": "..." },
      "feature-x": { "DATABASE_URL": "..." }          // overrides; rest inherited from base
    },
    "another-project": { "main": { "STRIPE_KEY": "..." } }
  }
  ```
- Master key in **macOS Keychain** (`security`), item `hush-master`. No plaintext key file on disk.
- Single store + single key → one key to manage; values stay isolated per project.

---

## Profile resolution

1. Determine the **project** namespace: `.hush.toml` `project`, else derived from repo path.
2. Determine active profile from `.hush.toml`:
   - `branch` → current git branch name; `cwd` → worktree dir name; `fixed:x` → `x`.
   - Detached HEAD / not a git repo under `branch` mode → fall back to `cwd` + warn.
3. For each declared key: look up in `store[project][profile]`, then walk the `extends`
   chain (active → ... → base) within the same project. First hit wins. Missing
   declared key → error naming it.
4. Keys not declared in `.hush.toml` are ignored (the file is the contract).

---

## Commands

| Command | What it does |
|---|---|
| `hush run -- <cmd>` | Resolve profile, decrypt, inject into **child env only**, `exec`. The only way to *use* secrets. |
| `hush edit [--profile p]` | Decrypt the profile into `$EDITOR`, edit like a normal `.env`, re-encrypt on save. **Primary way to change values in bulk.** TTY-only; agent mode refuses. |
| `hush set <KEY>` | Set one value: interactive prompt (not echoed, not in history), or piped stdin (`echo v \| hush set KEY`). No command-line value form (would leak via history/`ps`). |
| `hush unset <KEY>` | Remove a key from the active profile. |
| `hush ls [--json]` | List declared keys + which profile resolves each. **Never prints values.** |
| `hush get <KEY>` | Print value — **only on a TTY**; non-TTY (agent) → refuse with exit 3. |
| `hush import [path] [--profile p] [--shred]` | Read an existing `.env` (default `./.env`), write all `KEY=value` pairs into the target profile, add the keys to `.hush.toml`. `--shred` deletes the plaintext `.env` after. One-command migration. |
| `hush fork [--from base]` | Copy `--from` profile into the active (branch) profile, so you only `set` the diffs. (Sugar for `cp` into the active profile.) |
| `hush cp <from> <to>` | Copy one profile's values into another (generalized `fork`). Merge; `--force` to overwrite. |
| `hush shell` | Drop into a subshell with env loaded (explicit, ephemeral; human convenience). |
| `hush scrub` | Unset hush-managed vars from the current shell (run before launching an agent). |

---

## Migration: `hush import` (adoption-critical)

Goal: turn an existing plaintext `.env` into managed secrets in **one command**, so
trying hush costs nothing.

```bash
hush import                 # reads ./.env → active profile, appends keys to .hush.toml
hush import .env.local --profile base --shred
```

Behavior:
- Parse `KEY=value` (dotenv rules: `export ` prefix, quotes, `#` comments, blank lines).
- Write each pair into the target profile in `store.age`; merge, don't clobber unless `--force`.
- Add any new keys to `.hush.toml` `keys = [...]` (the file stays value-free).
- Report a summary: `imported 7 keys → profile base (2 new, 5 updated)`.
- `--shred`: securely remove the plaintext `.env` afterward; otherwise leave it and
  print a reminder to delete + gitignore. Round-trip check (decrypt == source) before shred.

## Consuming secrets (how a program reads them)

**No code changes, no SDK, no library.** hush works at the OS process layer: it
decrypts, populates the child process's environment, then `exec`s the command. The
program reads env vars the standard way — hush is the thing that *sets* the environment,
not something the code talks to.

```js
const url = process.env.DATABASE_URL;        // JS — unchanged
```
```python
url = os.environ["DATABASE_URL"]             # Python — unchanged
```
```solidity
string memory pk = vm.envString("DEPLOYER_KEY");  // foundry — unchanged
```

Only the launch changes:
```bash
node app.js                  →  hush run -- node app.js
forge script ...             →  hush run -- forge script ...
# with shims installed, you type the bare command and the shim wraps it.
```

- **dotenv libraries**: drop `dotenv.config()` (env is already injected); leaving it in
  is harmless (no `.env` → no-op), so migration can be gradual.
- **Auto-loading frameworks** (Next.js / Vite / foundry): they merge their own `.env`
  files with `process.env`, and injected `process.env` wins — so `hush run -- next dev`
  works with no `.env` file in the worktree at all.

### Universality & exceptions

Works for **anything that reads environment variables — every language, framework, and
CLI** (`node`, `python`, `go`, `forge`, `psql`, `aws`, `docker`, `terraform`, Makefiles,
shell scripts…), because that's an OS-level mechanism below any language.

Out of scope (not "broken", just not what env injection covers):
- **Tools that only read a config file**, not env (e.g. `~/.pgpass`). Bridge with
  `hush run -- sh -c 'tool --pass=$DB_PASSWORD'`.
- **Daemons started by systemd/launchd**, not launched manually — make the service
  manager invoke through hush.
- **Mid-run rotation / short-lived creds** — hush injects a startup snapshot; dynamic
  rotation is Vault territory, not v1.
- **CI / other machines** — Keychain is local; CI keeps its own secret mechanism. hush
  targets local development.

## Editing values: `hush edit`

The `sops`-style EDITOR flow — the main way to change env day to day:

```bash
hush edit                 # active profile → $EDITOR → re-encrypt on save
hush edit --profile base
```

- Decrypt active profile to a temp file, open `$EDITOR`, re-encrypt to `store.age` on
  save, then destroy the temp.
- **Temp file is RAM-backed (implemented):** on macOS the temp lives on an on-demand
  `hdiutil` RAM disk (`/Volumes/hush-edit-*`), so plaintext never touches persistent
  storage — secure-delete is unreliable on APFS/SSD (copy-on-write), so not-writing-it
  is the only sound guarantee. The whole volume is detached on exit. If a RAM disk can't
  be created, it falls back to a `0700` TMPDIR dir and **warns** that plaintext briefly
  touches disk.
- Needs a TTY + editor → **agent mode refuses** (agents use, don't edit).

## Dual-mode behavior (the core idea)

Mode is auto-detected, not configured:

```
agent / non-interactive  →  STRICT (broker): auto-load disabled; only `hush run` works;
                            `hush get` refused. Detected by: CLAUDECODE set, or no TTY.
interactive human shell  →  SMOOTH: shell shim + chpwd hook active.
```

**Smoothness (human only):**
- **Command shims**: commands listed in `.hush.toml` `shims` (per-project opt-in, empty
  by default) auto-wrap to `hush run -- <cmd>`. Values land in that child only — the
  persistent shell stays clean.
- **chpwd hook**: on `cd` into a worktree, switch the *active profile* and print a
  one-line banner (`hush: profile feature-x · 3 keys`). It **does not export values**.

Because values never enter the persistent shell, launching an agent from that shell
cannot leak them by inheritance. (`hush scrub` exists only for the `hush shell` path.)

---

## Agent-friendliness contract

- `hush run` = "use, don't see." It is the agent's only entry point.
- `hush ls` lists names; `hush get` refuses without a TTY; values never hit stdout in agent mode.
- Non-TTY never prompts (won't hang an agent); stable exit codes; `--json` everywhere.
- Errors state the human next step, e.g. `KEY DEPLOYER_KEY unset — run: hush set DEPLOYER_KEY`.
- Pairs with a harness hook denying `Read(.env*)` and `cat/grep .env` for defense in depth.

---

## Security model & threat boundary

**Stops:** `cat`/`grep` of worktree files surfacing secrets; accidental commit of values;
secrets leaking into the persistent shell (and thereby into a launched agent).

**Does NOT stop:** a same-user process that *deliberately calls `hush run -- env`* — any
local tool that can decrypt, an attacker sharing the uid can invoke too. Closing that
needs a separate trust domain (daemon / Keychain ACL per-binary) — explicitly out of v1.

Exit codes: `0` ok · `2` config/usage · `3` refused (e.g. `get` without TTY) · `4` decrypt/Keychain failure.

---

## MVP scope (v1)

`run` · `edit` · `set` · `unset` · `ls` · `import` · `fork` + command shims + chpwd profile switch + CLAUDECODE/TTY detection.
Defer: `shell`, `scrub`, `--json` on every command, multi-recipient, rotation.

---

## Decisions (signed off)

1. **Language: Go** — single static binary, easy to distribute; `age` has an official Go
   library (`filippo.io/age`) so encryption needs no external dependency.
2. **Shims: per-project opt-in, empty by default.** No global hardcoded list (wrapping
   e.g. `node` everywhere would pay decrypt cost and inject secrets where unneeded).
   Each project declares `shims = [...]` in `.hush.toml`; absent → behavior unchanged.
3. **Detached HEAD / no git: fall back to `cwd`, print a warning, don't error.** If that
   profile is also missing, walk the `extends` chain to base. Rationale: a detached
   checkout shouldn't block `hush run`; cwd is predictable and the warning flags that
   you're off the branch profile (safer than silently using base).
4. **One global `store.age`, namespaced internally project → profile → key** — one key to
   manage, values isolated per project.
