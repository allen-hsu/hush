# Changelog

All notable changes to hush. Format based on [Keep a Changelog](https://keepachangelog.com);
this project follows [Semantic Versioning](https://semver.org).

## [0.1.5] — 2026-06-22

### Added
- `hush purge <KEY>` — remove a key from **every** profile and from `.hush.toml`, the
  project-wide counterpart to `unset` (which only clears the active profile's value and
  leaves the key declared). Destructive: requires a TTY `y/N` confirmation or `--yes`, and
  is refused for agents without `--yes`. Supports `--json`.

## [0.1.4] — 2026-06-19

### Added
- `deny_agent_run = true` (`.hush.toml`) — refuse `hush run` when an agent marker is set,
  so an honest agent can't pull values at all.
- `agent_profile = "<name>"` (`.hush.toml`) — detected agents resolve against a sandbox
  profile (test credentials) instead, so an agent can still run the program but only sees
  throwaway values. `extends` is disabled in this case so a missing key can't leak a real
  value from `base`.

### Documentation
- Spelled out the threat boundary: these are guardrails against honest/careless agents,
  not walls (same-uid can unset the marker); real isolation needs a separate user / broker
  / service-over-localhost.

## [0.1.3] — 2026-06-19

### Added
- Codex detection: `CODEX_SANDBOX` is now recognized as an agent marker alongside
  `CLAUDECODE` and the `HUSH_AGENT` override.
- `hush get` with no key shows an interactive numbered picker (no need to know key names).
- `disable_get = true` (`.hush.toml`) — forbid `hush get` entirely, even on a TTY.

## [0.1.2] — 2026-06-19

### Added
- Homebrew distribution: `brew install allen-hsu/tap/hush`. Releases now publish a tap
  formula via GoReleaser.

## [0.1.1] — 2026-06-19

### Fixed
- `go install`-built binaries report a real version (from build info) instead of `dev`.

## [0.1.0] — 2026-06-19

First release. A local, agent-safe, per-worktree secret manager for macOS.

### Added
- Encrypted store: values live age-encrypted in `~/.config/hush/store.age`, namespaced
  `project → profile → key`. The master key is an age identity in the macOS Keychain — no
  plaintext key file on disk.
- Value-free `.hush.toml` in the repo (key names + profile rule). `cat`/`grep` of a
  worktree never surfaces values; nothing to commit by accident.
- `hush run -- <cmd>` injects values into the child process only, never the shell.
- Per-worktree profiles keyed by git branch, with an `extends` fallback chain; `fork`/`cp`
  to derive a branch's set from a base. Detached HEAD falls back to `cwd` with a warning.
- Dual-mode: interactive shells get command shims + a `cd` profile banner
  (`eval "$(hush hook)"`); agent/non-interactive contexts are auto-detected and locked to
  "use, don't see" (`hush get`/`edit` refused).
- `hush edit` opens a profile in `$EDITOR` on a RAM-backed temp (plaintext never touches
  persistent storage).
- One-command migration: `hush import [.env] [--shred]`.
- `.hush.toml` comments and formatting are preserved across `set`/`import`/`edit` (only the
  `keys` array is rewritten).
- `--json` on read/write commands. Commands: `run · edit · set · unset · ls · get ·
  import · fork · cp · init · install · hook · context · scrub · version`.

[0.1.5]: https://github.com/allen-hsu/hush/releases/tag/v0.1.5
[0.1.4]: https://github.com/allen-hsu/hush/releases/tag/v0.1.4
[0.1.3]: https://github.com/allen-hsu/hush/releases/tag/v0.1.3
[0.1.2]: https://github.com/allen-hsu/hush/releases/tag/v0.1.2
[0.1.1]: https://github.com/allen-hsu/hush/releases/tag/v0.1.1
[0.1.0]: https://github.com/allen-hsu/hush/releases/tag/v0.1.0
