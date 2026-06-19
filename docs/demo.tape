# vhs tape for the hush demo GIF — regenerate with:  vhs docs/demo.tape
# https://github.com/charmbracelet/vhs
Output docs/demo.gif

Set Shell bash
Set FontSize 16
Set Width 1180
Set Height 760
Set Padding 20
Set Theme "Dracula"
Set TypingSpeed 40ms

# --- setup (hidden): throwaway store + a demo project with fake secrets ---
Hide
Type "export PATH=/opt/homebrew/bin:$PATH; export HUSH_STORE=$(mktemp -d)/store.age; export PS1='$ '" Enter
Type "rm -rf /tmp/hush-demo && mkdir -p /tmp/hush-demo && cd /tmp/hush-demo" Enter
Type `printf 'DATABASE_URL=postgres://localhost/myapp\nSTRIPE_KEY=sk_live_demo123\nOPENAI_API_KEY=sk-proj-demo\n' > .env` Enter
Type "clear" Enter
Show

Type "# hush — keep .env secrets out of your repo and away from AI agents" Enter
Sleep 1.2s
Type "hush init" Enter
Sleep 1.8s
Type "hush import .env --shred      # migrate into the encrypted store, destroy the plaintext" Enter
Sleep 2.2s
Type "hush ls                       # key names + profile — never the values" Enter
Sleep 2.2s
Type "cat .hush.toml                # committed, value-free" Enter
Sleep 2.2s
Type "grep -r sk_live .             # no secrets anywhere in the worktree" Enter
Sleep 2.2s
Type `hush run -- sh -c 'echo app sees STRIPE_KEY=$STRIPE_KEY'   # injected into the child only` Enter
Sleep 3s
