package cli

// shellHook is printed by `hush hook`, meant to be eval'd from ~/.zshrc:
//
//	eval "$(hush hook)"
//
// It installs, per directory:
//   - a dim banner showing the active project/profile (no values, no decryption);
//   - shim functions for each command listed in .hush.toml `shims`, which wrap the
//     bare command in `hush run -- <cmd>` so env is injected into that child only.
//
// Everything is gated to an interactive, non-agent shell: with CLAUDECODE or
// HUSH_AGENT set (or a non-interactive shell) nothing installs, so an agent must
// call `hush run` explicitly and never inherits shims.
//
// The feed is `hush context`, which reads only .hush.toml (+ git) — it never opens
// the store, so `cd` stays fast and never triggers a keychain prompt. The shims
// themselves don't recurse: `hush run` execs the real binary via PATH (execve
// ignores shell functions), so `forge()` -> `hush run -- forge` -> real forge.
const shellHook = `# hush shell integration — add to ~/.zshrc:  eval "$(hush hook)"
if [ -z "$CLAUDECODE" ] && [ -z "$HUSH_AGENT" ] && [[ $- == *i* ]]; then
  typeset -ga _HUSH_SHIMS
  typeset -g _HUSH_LAST

  _hush_clear_shims() {
    local c
    for c in $_HUSH_SHIMS; do unfunction "$c" 2>/dev/null; done
    _HUSH_SHIMS=()
  }

  _hush_apply() {
    local out
    out=$(command hush context 2>/dev/null)
    # Re-apply only when the resolved project/profile/shims actually changed, so
    # cd within the same project (or into subdirs) is quiet and cheap.
    [ "$out" = "$_HUSH_LAST" ] && return
    _HUSH_LAST="$out"

    _hush_clear_shims
    [ -n "$out" ] || return   # left a hush dir — shims cleared, nothing to show

    local -a lines; lines=("${(@f)out}")
    [ -n "${lines[1]}" ] && print -P "%F{244}hush: ${lines[1]}%f"
    local c
    for c in "${lines[@]:1}"; do
      [ -n "$c" ] || continue
      functions[$c]="command hush run -- $c \"\$@\""
      _HUSH_SHIMS+="$c"
    done
  }

  autoload -Uz add-zsh-hook
  add-zsh-hook chpwd _hush_apply
  _hush_apply   # apply to the starting directory
fi
`
