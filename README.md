# hush

**English** · [繁體中文](README.zh-TW.md)

> Local, **agent-safe** secrets for per-worktree development. Zero service, zero daemon.

`.env` files are a liability the moment an AI coding agent is in your repo: agents love to
`cat .env` and `grep -r KEY`, and a stray value ends up in a model's context or an
accidental commit. **hush** keeps secrets out of your worktree entirely — the repo holds
only a value-free declaration; the actual values live age-encrypted outside it, and are
injected only into the child process of the command that needs them.

```sh
hush run -- npm run dev          # decrypts, injects env into THIS child only, execs
```

![hush demo](docs/demo.gif)

- 🔍 `cat .env` / `grep -r KEY` in a worktree finds **key names only, never values**.
- 🔐 Secrets are age-encrypted in one file outside the repo; the master key lives in the
  **macOS Keychain** — there is no plaintext key file on disk.
- 🧬 Values are injected only into the child process of `hush run` — **never** your shell.
- 🤖 **Smooth for humans, strict for agents.** Agent / non-interactive contexts (e.g.
  `CLAUDECODE` set, no TTY) are auto-detected and locked to "use, don't see".
- 🌳 **Per-worktree by design.** The declaration is committed, so a fresh worktree works
  immediately; values are keyed by git branch and inherit from a base profile.

> **Platform:** macOS only. hush relies on the macOS Keychain (master key) and an
> `hdiutil` RAM disk (so `edit` never writes plaintext to persistent storage).

---

## Install

**With Go (recommended — builds locally, no Gatekeeper quarantine):**

```sh
go install github.com/allen-hsu/hush@latest
hush install     # idempotently adds  eval "$(hush hook)"  to ~/.zshrc
```

**With Homebrew:**

```sh
brew install allen-hsu/tap/hush
hush install
```

**From source:**

```sh
git clone https://github.com/allen-hsu/hush && cd hush
go build -o ~/bin/hush .   # ensure ~/bin is on your PATH
hush install
```

Then restart your shell (or `source ~/.zshrc`).

---

## Quick start

```sh
cd my-project
hush init                    # writes .hush.toml (committed; declares keys, no values)
hush import .env --shred     # migrate an existing .env, then destroy the plaintext
hush ls                      # key names + which profile resolves each (no values)
hush run -- npm run dev      # decrypt, inject env into this child only, exec
```

That's the whole loop: declare → import → run. Your code reads `process.env` /
`os.environ` / `vm.envString` exactly as before — hush just populates the environment of
the process it launches. No SDK, no library, no code change.

---

## How it works

Three pieces, cleanly separated:

| | What | Where | Contains |
|---|---|---|---|
| **Declaration** | `.hush.toml` | committed in the repo | which keys, how to pick a profile — **never values** |
| **Store** | `store.age` | `~/.config/hush/` (outside any repo) | all values, age-encrypted, namespaced `project → profile → key` |
| **Master key** | age identity | macOS Keychain | generated on first use; no plaintext key file on disk |

`hush run` reads the declaration, resolves the active **profile** (by git branch by
default), decrypts the store with the Keychain key, injects the resolved values into the
child process, and `exec`s it. Plaintext exists only in that child's memory.

### Per-worktree workflow

`profile = "branch"` keys values by git branch, so each worktree/branch gets its own set:

```sh
git worktree add ../feature-x -b feature-x
cd ../feature-x
hush fork                 # clone the `base` profile into this branch's profile
hush set DATABASE_URL     # override only the few values that differ
```

The committed `.hush.toml` travels with the checkout, so the new worktree is ready at
once; you only `set` the diffs.

---

## Commands

| Command | What it does |
|---|---|
| `hush run -- <cmd>` | Resolve profile, inject env into the child, exec. **Use, don't see.** |
| `hush edit` | Edit a profile in `$EDITOR` (TTY only; agents refused; RAM-disk backed). |
| `hush set <KEY>` | Set one value — interactive prompt or piped stdin. |
| `hush unset <KEY>` | Remove a key from the active profile. |
| `hush ls` | List declared keys + which profile resolves each. Never prints values. |
| `hush get [KEY]` | Print a value (TTY only; refused for agents). Omit `KEY` to pick from a numbered list; set `disable_get = true` to forbid it entirely. |
| `hush import [path]` | Import an existing `.env`. Flags: `--profile`, `--force`, `--shred`. |
| `hush fork [--from p]` | Copy a profile into the active profile. |
| `hush cp <from> <to>` | Copy one profile's values into another. |
| `hush init` | Scaffold a `.hush.toml`. |
| `hush install` | Idempotently add the shell hook to `~/.zshrc`. |
| `hush hook` | Print the shell integration snippet (`eval "$(hush hook)"`). |
| `hush scrub` | Print shell cmds to clear hush vars/shims before launching an agent. |

`--json` is accepted by `ls`, `get`, `set`, `unset`, `import`, `fork`, `cp` for
machine-readable output — useful for scripts and agents.

---

## Shell integration

`eval "$(hush hook)"` (added by `hush install`) gives you, in an interactive shell:

- a dim banner showing the active `project · profile` when you `cd` into a project;
- **shims**: for each command in `.hush.toml` `shims = [...]` (you choose them), typing
  the bare command auto-wraps it — `npm run dev` runs as `hush run -- npm run dev`.
  Values land in that child only; your persistent shell stays clean.

Leaving the project tears the shims down. When an agent is detected — `CLAUDECODE`
(Claude Code), `CODEX_SANDBOX` (Codex), or `HUSH_AGENT` set, or no TTY — **nothing
installs** and `hush get` is refused; the agent must call `hush run` explicitly and never
inherits shims or shell env. `HUSH_AGENT=1` is the universal override for any other
runtime. The `cd` feed (`hush context`) reads only `.hush.toml` (+ git), never the store,
so it never triggers a Keychain prompt.

```toml
# .hush.toml — committed, value-free
profile = "branch"          # branch | cwd | fixed:<name>
extends = "base"            # fall back to this profile for keys absent in the active one
keys    = ["DATABASE_URL", "DEPLOYER_KEY"]
shims   = ["npm", "pnpm"]   # opt-in; commands to auto-wrap with `hush run`
# disable_get = true        # forbid `hush get` entirely — values usable only via `hush run`
```

---

## Security model

**hush stops** the realistic, common failure modes:

- `cat` / `grep` of worktree files surfacing secret values;
- accidental commit of values (the repo only ever holds key *names*);
- secrets leaking into your persistent shell (and thereby into an agent you launch from it);
- `edit` writing plaintext to persistent storage (it uses a RAM disk; secure-delete is
  unreliable on APFS/SSD, so not-writing-it is the only sound guarantee).

**hush does not stop** a same-user process that *deliberately* runs `hush run -- env`.
Any local tool that can decrypt can be invoked by anything sharing your uid; closing that
needs a separate trust domain (daemon / per-binary Keychain ACL), which is intentionally
out of scope for a local single-user tool. The goal is to defeat accidental exposure and
an agent's reflex to read files — not a determined local attacker.

---

## Distributing / forking

hush is a single static Go binary with two runtime dependencies on macOS system tools
(`security`, `hdiutil`/`diskutil`). To ship your own builds, [GoReleaser](https://goreleaser.com)
can produce a Homebrew tap formula and GitHub Release archives in one step. Prefer
`go install` / `brew` (which build locally) over raw downloaded binaries: an unsigned,
un-notarized binary downloaded from a release will be quarantined by macOS Gatekeeper.

---

## Status

**v0.1 — released.** `go install github.com/allen-hsu/hush@latest`.

Full command set is in place and tested:
`run · edit · set · unset · ls · get · import · fork · cp · init · install · hook · context · scrub`.
Tests cover the store crypto (round-trip, encryption-at-rest, wrong-key rejection),
profile/extends resolution, dotenv parsing, and the RAM-disk temp path.

Out of scope (by design, for a local single-user tool): multi-recipient / team sharing,
secret rotation, non-macOS platforms.

See [docs/SPEC.md](docs/SPEC.md) for the full design rationale.

## License

MIT
