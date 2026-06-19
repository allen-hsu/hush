# hush — design

A local, zero-service CLI for per-worktree secrets that is **smooth for humans,
strict for agents**. Secrets live encrypted outside the repo; the worktree only holds a
value-free declaration. Plaintext exists only inside the child process of `hush run`.

This document explains *why* hush is shaped the way it is. For usage, see the
[README](../README.md).

---

## Goals / Non-goals

**Goals**
- `cat .env` / `grep -r KEY` in a worktree yields **key names only, never values**.
- A fresh worktree is usable immediately (the declaration is committed; values inherited).
- Fork a worktree's private env in ~2 commands.
- A human terminal feels direnv-smooth; agent context is auto-detected and locked to a
  "use, don't see" broker mode.

**Non-goals** (deliberate, for a local single-user tool)
- No team sharing / multi-recipient encryption (single-user).
- No audit log, rotation, or dynamic/short-lived credentials.
- macOS only.
- Not a defense against a *same-user process that deliberately calls `hush`* — see
  [Security model](#security-model--threat-boundary).

---

## Files & formats

> **`.hush.toml` never contains values.** It is a declaration (which keys, how to pick a
> profile, project identity). All values live encrypted in `store.age` and change only
> through `hush set` / `hush edit` / `hush import` — never by hand-editing a file.

**In repo (committed, no secrets):** `.hush.toml`
```toml
project = "my-app"           # namespace in store.age; omit → derived from repo dir name
profile = "branch"           # branch | cwd | fixed:<name>  — how to pick a profile
extends = "base"             # fallback chain when a key is absent in the active profile
keys = ["DATABASE_URL", "STRIPE_KEY"]   # declared contract
shims = ["npm", "pnpm"]      # commands to auto-wrap with `hush run` (opt-in; empty = off)
```

**Central store (outside repo):** `~/.config/hush/store.age`
- One global age-encrypted file. Cleartext is namespaced **project → profile → key**, so
  two projects on the same branch name never collide:
  ```jsonc
  {
    "my-app": {
      "base":      { "DATABASE_URL": "...", "STRIPE_KEY": "..." },
      "feature-x": { "DATABASE_URL": "..." }          // overrides; rest inherited from base
    },
    "another-project": { "main": { "API_TOKEN": "..." } }
  }
  ```
- Master key is an age identity in the **macOS Keychain** (`security`), generated on first
  use. There is no plaintext key file on disk.
- One store + one key → a single key to manage, while values stay isolated per project.

---

## Profile resolution

1. Determine the **project** namespace: `.hush.toml` `project`, else the repo dir name.
2. Determine the active **profile**:
   - `branch` → current git branch; `cwd` → worktree dir name; `fixed:x` → `x`.
   - Detached HEAD / not a git repo under `branch` mode → fall back to `cwd` and warn.
3. For each declared key: look up `store[project][profile]`, then walk the `extends` chain
   (active → … → base) within the same project. First hit wins. A missing declared key is
   an error that names it.
4. Keys not declared in `.hush.toml` are ignored — the file is the contract.

---

## Commands

| Command | What it does |
|---|---|
| `hush run -- <cmd>` | Resolve profile, decrypt, inject into the **child env only**, `exec`. The only way to *use* secrets. |
| `hush edit [--profile p]` | Decrypt the profile into `$EDITOR`, edit like a normal `.env`, re-encrypt on save. Bulk edit. TTY-only; agent mode refuses. |
| `hush set <KEY>` | Set one value: interactive prompt (not echoed, not in history) or piped stdin. No command-line value form (would leak via history/`ps`). |
| `hush unset <KEY>` | Remove a key from the active profile. |
| `hush ls [--json]` | List declared keys + which profile resolves each. **Never prints values.** |
| `hush get [KEY] [--json]` | Print a value — **TTY only**; omit `KEY` to pick from a numbered list; non-TTY/agent → refuse (exit 3); always refused when `disable_get = true`. |
| `hush import [path] [--profile p] [--force] [--shred]` | Import an existing `.env` into a profile and record the keys. One-command migration. |
| `hush fork [--from base]` | Copy a profile into the active one, so you only `set` the diffs. Sugar for `cp`. |
| `hush cp <from> <to> [--force]` | Copy one profile's values into another. |
| `hush init` | Scaffold a commented, value-free `.hush.toml`. |
| `hush install` | Idempotently add `eval "$(hush hook)"` to `~/.zshrc`. |
| `hush hook` | Print the shell integration snippet (shims + chpwd banner). |
| `hush context` | Fast, keychain-free feed for the shell hook (project/profile + shim list). |
| `hush scrub` | Print shell commands to clear hush vars/shims before launching an agent. |
| `hush version` | Print the version. |

`--json` is accepted by `ls`, `get`, `set`, `unset`, `import`, `fork`, `cp`.

---

## Migration: `hush import`

Turning an existing plaintext `.env` into managed secrets in **one command** is what makes
trying hush free.

```bash
hush import                 # reads ./.env → active profile, appends keys to .hush.toml
hush import .env.local --profile base --shred
```

- Parses `KEY=value` with dotenv rules (`export ` prefix, quotes, `#` comments, blanks).
- Writes each pair into the target profile; merges, doesn't clobber unless `--force`.
- Adds new keys to `.hush.toml` `keys = [...]` (the file stays value-free; **comments and
  formatting are preserved** — only the `keys` array is rewritten).
- `--shred` securely removes the source `.env` after a round-trip check (decrypt ==
  source); otherwise it leaves the file and prints a reminder to delete + gitignore it.

---

## Consuming secrets

**No code changes, no SDK, no library.** hush works at the OS process layer: it decrypts,
populates the child process's environment, then `exec`s the command. Your program reads
env vars the standard way — hush is the thing that *sets* the environment, not something
the code talks to.

```js
const url = process.env.DATABASE_URL;        // JS — unchanged
```
```python
url = os.environ["DATABASE_URL"]             # Python — unchanged
```
```go
url := os.Getenv("DATABASE_URL")             // Go — unchanged
```

Only the launch changes:
```bash
node app.js          →  hush run -- node app.js
npm run dev          →  hush run -- npm run dev
# with shims installed, you type the bare command and the shim wraps it.
```

- **dotenv libraries**: drop `dotenv.config()` (env is already injected); leaving it in is
  harmless (no `.env` → no-op), so migration can be gradual.
- **Auto-loading frameworks** (Next.js / Vite / etc.): they merge their own `.env` files
  with `process.env`, and the injected `process.env` wins — so `hush run -- next dev`
  works with no `.env` file in the worktree at all.

### Universality & exceptions

Works for **anything that reads environment variables — every language, framework, and
CLI** (`node`, `python`, `go`, `psql`, `aws`, `docker`, `terraform`, Makefiles, shell
scripts…), because that is an OS-level mechanism below any language.

Out of scope (not "broken", just not what env injection covers):
- **Tools that only read a config file**, not env (e.g. `~/.pgpass`). Bridge with
  `hush run -- sh -c 'tool --pass=$DB_PASSWORD'`.
- **Daemons started by systemd/launchd**, not launched manually — make the service manager
  invoke through hush.
- **Mid-run rotation / short-lived creds** — hush injects a startup snapshot; dynamic
  rotation is a secrets-manager (e.g. Vault) concern.
- **CI / other machines** — the Keychain is local; CI keeps its own secret mechanism. hush
  targets local development.

---

## Editing values: `hush edit`

The `sops`-style EDITOR flow — the main way to change env day to day:

```bash
hush edit                 # active profile → $EDITOR → re-encrypt on save
hush edit --profile base
```

- Decrypt the active profile to a temp file, open `$EDITOR`, re-encrypt to `store.age` on
  save, then destroy the temp.
- **The temp file is RAM-backed.** On macOS it lives on an on-demand `hdiutil` RAM disk
  (`/Volumes/hush-edit-*`), so plaintext never touches persistent storage — secure-delete
  is unreliable on APFS/SSD (copy-on-write), so *not writing it* is the only sound
  guarantee. The whole volume is detached on exit. If a RAM disk can't be created, it falls
  back to a `0700` TMPDIR dir and warns that plaintext briefly touches disk.
- Needs a TTY + editor → **agent mode refuses** (agents use, don't edit).

---

## Dual-mode behavior (the core idea)

Mode is auto-detected, not configured:

```
agent / non-interactive  →  STRICT: no shims; only `hush run` works; `hush get` refused.
                            Detected by an env marker or no TTY (see below).
interactive human shell  →  SMOOTH: shell shims + chpwd banner active.
```

**Agent detection** is a small list of env markers set by known runtimes —
`CLAUDECODE` (Claude Code), `CODEX_SANDBOX` (Codex), `HUSH_AGENT` (universal manual
override) — plus a no-TTY fallback that catches agents which run commands
non-interactively without setting any marker. Adding a new runtime is one line in that
list; users can always force strict mode with `HUSH_AGENT=1`.

`disable_get = true` in `.hush.toml` is the stricter opt-in: `hush get` is then refused
even for a human on a TTY, so values can only ever be *used* (via `hush run`), never
printed.

**Smoothness (human only),** via `eval "$(hush hook)"`:
- **Command shims**: commands listed in `.hush.toml` `shims` (per-project opt-in, empty by
  default) auto-wrap to `hush run -- <cmd>`. Values land in that child only — the
  persistent shell stays clean. No recursion: `hush run` execs the real binary via PATH,
  which `execve` resolves without consulting shell functions.
- **chpwd banner**: on `cd` into a project, a one-line banner shows the active
  `project · profile`. It re-applies only when the resolved context actually changes
  (quiet across subdirs) and **never exports values**.

Because values never enter the persistent shell, launching an agent from that shell cannot
leak them by inheritance. `hush scrub` additionally clears any hush-managed vars/shims on
demand before you spawn an agent.

---

## Agent-friendliness contract

- `hush run` = "use, don't see." It is the agent's only entry point.
- `hush ls` lists names; `hush get` refuses without a TTY; values never reach stdout in
  agent mode.
- Non-TTY never prompts (won't hang an agent); stable exit codes; `--json` on read/write
  commands.
- Errors state the human next step, e.g.
  `unset keys for profile "main": DEPLOYER_KEY — run: hush set <KEY>`.
- Pairs well with a harness rule denying `Read(.env*)` and `cat/grep .env` for defense in
  depth.

---

## Security model & threat boundary

**Stops** the realistic, common failure modes:
- `cat` / `grep` of worktree files surfacing secret values;
- accidental commit of values (the repo only ever holds key *names*);
- secrets leaking into the persistent shell (and thereby into a launched agent);
- `edit` writing plaintext to persistent storage (RAM-disk backed; see above).

**Does NOT stop** a same-user process that *deliberately* runs `hush run -- env`. Any local
tool that can decrypt can be invoked by anything sharing your uid; closing that needs a
separate trust domain (daemon / per-binary Keychain ACL), which is out of scope. The goal
is to defeat accidental exposure and an agent's reflex to read files — not a determined
local attacker.

**Exit codes:** `0` ok · `2` config/usage · `3` refused (e.g. `get` without a TTY) ·
`4` decrypt/Keychain/runtime failure.

---

## Design decisions & rationale

1. **Language: Go.** Single static binary, easy to distribute; `age` has an official Go
   library (`filippo.io/age`), so encryption needs no external dependency.
2. **Shims are per-project opt-in, empty by default.** No global hardcoded list — wrapping
   e.g. `node` everywhere would pay decryption cost and inject secrets where unneeded. Each
   project declares `shims = [...]`; absent → behavior unchanged.
3. **Detached HEAD / no git falls back to `cwd` with a warning, never errors.** A detached
   checkout shouldn't block `hush run`; `cwd` is predictable and the warning flags that
   you're off the branch profile (safer than silently using base).
4. **One global `store.age`, namespaced project → profile → key.** One key to manage, with
   values isolated per project — rather than a per-repo key file that would scatter keys.
5. **The master key lives in the Keychain, not a key file.** age-encrypted `.env`
   alternatives still leave a plaintext private-key file on disk; the Keychain avoids that.
6. **Values inject into the child only, never the shell.** This is the dividing line from
   direnv-style tools, and what makes the dual-mode agent guarantee possible.
