package cli

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"

	"golang.org/x/term"

	"github.com/allen-hsu/hush/internal/config"
	"github.com/allen-hsu/hush/internal/resolve"
	"github.com/allen-hsu/hush/internal/store"
)

// cmdRun resolves the active profile, injects values into the child env, and
// execs the command. The only way to *use* secrets.
func cmdRun(args []string) error {
	if len(args) > 0 && args[0] == "--" {
		args = args[1:]
	}
	if len(args) == 0 {
		return usageErr("run: nothing to execute (usage: hush run -- <cmd> [args...])")
	}

	cfg, _, ctx, data, err := load()
	if err != nil {
		return err
	}
	if cfg.DenyAgentRun && hasAgentMarker() {
		return refusedErr("hush run is disabled for agents on this project (deny_agent_run) — ask a human to run secret-dependent commands")
	}
	vals, missing := resolve.Values(cfg, ctx, data)
	if len(missing) > 0 {
		return coded{exitDecrypt, fmt.Sprintf(
			"unset keys for profile %q: %s — run: hush set <KEY> (or hush edit)",
			ctx.Profile, strings.Join(missing, ", "))}
	}

	env := os.Environ()
	for k, v := range vals {
		env = append(env, k+"="+v)
	}

	c := exec.Command(args[0], args[1:]...)
	c.Env = env
	c.Stdin, c.Stdout, c.Stderr = os.Stdin, os.Stdout, os.Stderr
	if err := c.Run(); err != nil {
		if ee, ok := err.(*exec.ExitError); ok {
			os.Exit(ee.ExitCode())
		}
		return err
	}
	return nil
}

// cmdLs lists declared keys and which profile resolves each — never values.
func cmdLs(args []string) error {
	asJSON, _ := popJSON(args)

	cfg, _, ctx, data, err := load()
	if err != nil {
		return err
	}
	type row struct {
		Key  string `json:"key"`
		Set  bool   `json:"set"`
		From string `json:"from,omitempty"`
	}
	chain := []string{ctx.Profile}
	if cfg.Extends != "" && cfg.Extends != ctx.Profile {
		chain = append(chain, cfg.Extends)
	}
	proj := data[ctx.Project]
	rows := make([]row, 0, len(cfg.Keys))
	for _, k := range cfg.Keys {
		r := row{Key: k}
		for _, p := range chain {
			if _, ok := proj[p][k]; ok {
				r.Set, r.From = true, p
				break
			}
		}
		rows = append(rows, r)
	}

	if asJSON {
		return emitJSON(map[string]any{
			"project": ctx.Project, "profile": ctx.Profile, "keys": rows,
		})
	}
	fmt.Printf("project %s · profile %s\n", ctx.Project, ctx.Profile)
	for _, r := range rows {
		mark := "MISSING"
		if r.Set {
			mark = "set (" + r.From + ")"
		}
		fmt.Printf("  %-28s %s\n", r.Key, mark)
	}
	return nil
}

// cmdGet prints a single value — TTY only; refused for agents.
func cmdGet(args []string) error {
	asJSON, args := popJSON(args)
	if len(args) > 1 {
		return usageErr("get: at most one KEY (omit it to pick from a list)")
	}
	if isAgent() {
		return refusedErr("get is refused in agent/non-interactive mode (use `hush run`)")
	}
	cfg, _, ctx, data, err := load()
	if err != nil {
		return err
	}
	if cfg.DisableGet {
		return refusedErr("get is disabled for this project (disable_get in .hush.toml) — use `hush run`")
	}
	vals, _ := resolve.Values(cfg, ctx, data)

	key := ""
	if len(args) == 1 {
		key = args[0]
	} else {
		// No key given — let the human pick from the declared keys.
		key, err = pickKey(cfg.Keys)
		if err != nil {
			return err
		}
	}
	v, ok := vals[key]
	if !ok {
		return coded{exitDecrypt, "no value for " + key}
	}
	if asJSON {
		return emitJSON(map[string]any{"key": key, "value": v})
	}
	fmt.Println(v)
	return nil
}

// pickKey prints a numbered list of keys to stderr and reads a 1-based choice
// from stdin. Used by `hush get` with no key. TTY-gated by the caller.
func pickKey(keys []string) (string, error) {
	if len(keys) == 0 {
		return "", coded{exitUsage, "no keys declared in .hush.toml"}
	}
	for i, k := range keys {
		fmt.Fprintf(os.Stderr, "  %2d) %s\n", i+1, k)
	}
	fmt.Fprintf(os.Stderr, "pick a key [1-%d]: ", len(keys))
	var n int
	if _, err := fmt.Fscan(os.Stdin, &n); err != nil {
		return "", coded{exitUsage, "invalid selection"}
	}
	if n < 1 || n > len(keys) {
		return "", coded{exitUsage, "selection out of range"}
	}
	return keys[n-1], nil
}

// cmdSet writes one value: interactive prompt or piped stdin. No CLI value form.
func cmdSet(args []string) error {
	asJSON, args := popJSON(args)
	if len(args) != 1 {
		return usageErr("set: exactly one KEY required (value via prompt or stdin)")
	}
	key := args[0]

	var val string
	if !term.IsTerminal(int(os.Stdin.Fd())) {
		b, err := io.ReadAll(os.Stdin)
		if err != nil {
			return err
		}
		val = strings.TrimRight(string(b), "\n")
	} else {
		fmt.Fprintf(os.Stderr, "value for %s: ", key)
		b, err := term.ReadPassword(int(os.Stdin.Fd()))
		fmt.Fprintln(os.Stderr)
		if err != nil {
			return err
		}
		val = string(b)
	}

	cfg, st, ctx, data, err := load()
	if err != nil {
		return err
	}
	setVal(data, ctx.Project, ctx.Profile, key, val)
	if !contains(cfg.Keys, key) {
		cfg.Keys = append(cfg.Keys, key)
		if err := writeConfig(cfg); err != nil {
			return err
		}
	}
	if err := st.Save(data); err != nil {
		return err
	}
	if asJSON {
		return emitJSON(map[string]any{
			"ok": true, "action": "set", "key": key,
			"project": ctx.Project, "profile": ctx.Profile,
		})
	}
	fmt.Fprintf(os.Stderr, "set %s → %s/%s\n", key, ctx.Project, ctx.Profile)
	return nil
}

// cmdUnset removes a key from the active profile.
func cmdUnset(args []string) error {
	asJSON, args := popJSON(args)
	if len(args) != 1 {
		return usageErr("unset: exactly one KEY required")
	}
	_, st, ctx, data, err := load()
	if err != nil {
		return err
	}
	if p := data[ctx.Project]; p != nil && p[ctx.Profile] != nil {
		delete(p[ctx.Profile], args[0])
	}
	if err := st.Save(data); err != nil {
		return err
	}
	if asJSON {
		return emitJSON(map[string]any{
			"ok": true, "action": "unset", "key": args[0],
			"project": ctx.Project, "profile": ctx.Profile,
		})
	}
	fmt.Fprintf(os.Stderr, "unset %s from %s/%s\n", args[0], ctx.Project, ctx.Profile)
	return nil
}

// cmdPurge removes a key from EVERY profile in the project and drops it from
// .hush.toml — the "this key is gone from the project" action, vs unset which
// only clears the active profile's value. Destructive: needs --yes or a y/N
// confirmation on a TTY; refused for agents/non-interactive without --yes.
func cmdPurge(args []string) error {
	asJSON, args := popJSON(args)
	yes := false
	var rest []string
	for _, a := range args {
		if a == "--yes" || a == "-y" {
			yes = true
		} else {
			rest = append(rest, a)
		}
	}
	if len(rest) != 1 {
		return usageErr("purge: exactly one KEY required")
	}
	key := rest[0]

	cfg, st, ctx, data, err := load()
	if err != nil {
		return err
	}

	// Count profiles that currently hold the key.
	n := 0
	for _, kv := range data[ctx.Project] {
		if _, ok := kv[key]; ok {
			n++
		}
	}
	declared := contains(cfg.Keys, key)

	if !yes {
		if isAgent() {
			return refusedErr("purge needs confirmation — pass --yes (refused for agents/non-interactive)")
		}
		fmt.Fprintf(os.Stderr, "purge %q from project %s: %d profile value(s)%s? [y/N]: ",
			key, ctx.Project, n, map[bool]string{true: " + .hush.toml", false: ""}[declared])
		var ans string
		_, _ = fmt.Fscan(os.Stdin, &ans)
		if ans != "y" && ans != "Y" && ans != "yes" {
			return coded{exitUsage, "aborted"}
		}
	}

	// Remove the value from every profile, and the name from the declaration.
	for _, kv := range data[ctx.Project] {
		delete(kv, key)
	}
	if declared {
		newKeys := make([]string, 0, len(cfg.Keys))
		for _, k := range cfg.Keys {
			if k != key {
				newKeys = append(newKeys, k)
			}
		}
		cfg.Keys = newKeys
		if err := writeConfig(cfg); err != nil {
			return err
		}
	}
	if err := st.Save(data); err != nil {
		return err
	}

	if asJSON {
		return emitJSON(map[string]any{
			"ok": true, "action": "purge", "key": key,
			"project": ctx.Project, "profiles_cleared": n, "undeclared": declared,
		})
	}
	fmt.Fprintf(os.Stderr, "purged %s from project %s (%d profile value(s)%s)\n",
		key, ctx.Project, n, map[bool]string{true: ", removed from .hush.toml", false: ""}[declared])
	return nil
}

// cmdImport reads an existing .env into a profile and records the keys.
func cmdImport(args []string) error {
	asJSON, args := popJSON(args)
	path := ".env"
	profile := ""
	force, shred := false, false
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--profile":
			i++
			if i >= len(args) {
				return usageErr("import: --profile needs a value")
			}
			profile = args[i]
		case "--force":
			force = true
		case "--shred":
			shred = true
		default:
			path = args[i]
		}
	}

	f, err := os.Open(path)
	if err != nil {
		return err
	}
	pairs, err := parseDotenv(f)
	f.Close()
	if err != nil {
		return err
	}

	cfg, st, ctx, data, err := load()
	if err != nil {
		return err
	}
	target := ctx.Profile
	if profile != "" {
		target = profile
	}

	var nw, upd int
	for k, v := range pairs {
		if _, exists := get(data, ctx.Project, target, k); exists {
			if !force {
				upd++ // merge: overwrite in place (no --force gate for same-key update)
			}
		} else {
			nw++
		}
		setVal(data, ctx.Project, target, k, v)
		if !contains(cfg.Keys, k) {
			cfg.Keys = append(cfg.Keys, k)
		}
	}

	if shred {
		// Round-trip check before destroying the source.
		if err := st.Save(data); err != nil {
			return err
		}
		if err := secureRemove(path); err != nil {
			return err
		}
	} else {
		if err := st.Save(data); err != nil {
			return err
		}
	}
	if err := writeConfig(cfg); err != nil {
		return err
	}

	sort.Strings(cfg.Keys)
	if asJSON {
		return emitJSON(map[string]any{
			"ok": true, "action": "import", "imported": len(pairs),
			"new": nw, "updated": upd, "shredded": shred,
			"project": ctx.Project, "profile": target,
		})
	}
	fmt.Fprintf(os.Stderr, "imported %d keys → %s/%s (%d new, %d updated)\n",
		len(pairs), ctx.Project, target, nw, upd)
	if !shred {
		fmt.Fprintf(os.Stderr, "reminder: delete %s and add it to .gitignore\n", path)
	}
	return nil
}

// cmdFork copies a source profile into the active profile (sugar for cp).
func cmdFork(args []string) error {
	asJSON, args := popJSON(args)
	from := "base"
	for i := 0; i < len(args); i++ {
		if args[i] == "--from" {
			i++
			if i >= len(args) {
				return usageErr("fork: --from needs a value")
			}
			from = args[i]
		}
	}
	_, st, ctx, data, err := load()
	if err != nil {
		return err
	}
	n := copyProfile(data, ctx.Project, from, ctx.Profile, false)
	if err := st.Save(data); err != nil {
		return err
	}
	if asJSON {
		return emitJSON(map[string]any{
			"ok": true, "action": "fork", "copied": n,
			"from": from, "to": ctx.Profile, "project": ctx.Project,
		})
	}
	fmt.Fprintf(os.Stderr, "forked %d keys: %s → %s (project %s)\n", n, from, ctx.Profile, ctx.Project)
	return nil
}

// cmdCp copies one profile's values into another.
func cmdCp(args []string) error {
	asJSON, args := popJSON(args)
	var rest []string
	force := false
	for _, a := range args {
		if a == "--force" {
			force = true
		} else {
			rest = append(rest, a)
		}
	}
	if len(rest) != 2 {
		return usageErr("cp: usage: hush cp <from> <to> [--force]")
	}
	_, st, ctx, data, err := load()
	if err != nil {
		return err
	}
	n := copyProfile(data, ctx.Project, rest[0], rest[1], force)
	if err := st.Save(data); err != nil {
		return err
	}
	if asJSON {
		return emitJSON(map[string]any{
			"ok": true, "action": "cp", "copied": n,
			"from": rest[0], "to": rest[1], "project": ctx.Project,
		})
	}
	fmt.Fprintf(os.Stderr, "copied %d keys: %s → %s (project %s)\n", n, rest[0], rest[1], ctx.Project)
	return nil
}

// cmdEdit opens a profile in $EDITOR and re-encrypts on save. TTY only.
func cmdEdit(args []string) error {
	if isAgent() {
		return refusedErr("edit is refused in agent/non-interactive mode")
	}
	profile := ""
	for i := 0; i < len(args); i++ {
		if args[i] == "--profile" {
			i++
			if i >= len(args) {
				return usageErr("edit: --profile needs a value")
			}
			profile = args[i]
		}
	}
	cfg, st, ctx, data, err := load()
	if err != nil {
		return err
	}
	target := ctx.Profile
	if profile != "" {
		target = profile
	}

	// Plaintext goes on a RAM-backed volume so it never touches persistent
	// storage (secure-delete is unreliable on APFS/SSD). Falls back to TMPDIR
	// with a warning if a RAM disk can't be created.
	dir, cleanup, ramBacked, err := secureTempDir("hush-edit-")
	if err != nil {
		return err
	}
	defer cleanup()
	if !ramBacked {
		fmt.Fprintf(os.Stderr,
			"hush: warning: RAM disk unavailable; plaintext will briefly touch disk under %s\n", dir)
	}
	tmp := filepath.Join(dir, target+".env")
	if err := os.WriteFile(tmp, []byte(formatDotenv(data[ctx.Project][target])), 0o600); err != nil {
		return err
	}

	editor := os.Getenv("EDITOR")
	if editor == "" {
		editor = "vi"
	}
	ed := exec.Command("sh", "-c", editor+" "+shellQuote(tmp))
	ed.Stdin, ed.Stdout, ed.Stderr = os.Stdin, os.Stdout, os.Stderr
	if err := ed.Run(); err != nil {
		return err
	}

	edited, err := os.ReadFile(tmp)
	if err != nil {
		return err
	}
	_ = secureRemove(tmp)
	pairs, err := parseDotenv(bytes.NewReader(edited))
	if err != nil {
		return err
	}

	if data[ctx.Project] == nil {
		data[ctx.Project] = map[string]map[string]string{}
	}
	data[ctx.Project][target] = pairs
	for k := range pairs {
		if !contains(cfg.Keys, k) {
			cfg.Keys = append(cfg.Keys, k)
		}
	}
	if err := st.Save(data); err != nil {
		return err
	}
	if err := writeConfig(cfg); err != nil {
		return err
	}
	fmt.Fprintf(os.Stderr, "saved %d keys → %s/%s\n", len(pairs), ctx.Project, target)
	return nil
}

// cmdInit scaffolds a commented .hush.toml in the current directory.
func cmdInit(_ []string) error {
	if _, err := os.Stat(config.FileName); err == nil {
		return usageErr(config.FileName + " already exists")
	}
	if err := os.WriteFile(config.FileName, []byte(initTemplate), 0o644); err != nil {
		return err
	}
	fmt.Fprintf(os.Stderr, "wrote %s — declare your keys, then `hush import .env` or `hush set <KEY>`\n", config.FileName)
	return nil
}

// initTemplate is the scaffolded .hush.toml. It is committed and value-free.
const initTemplate = `# hush config — committed, value-free. Secrets live encrypted in the store
# (~/.config/hush/store.age); change values with ` + "`hush set`" + ` / ` + "`hush edit`" + `, never here.

# project = "my-project"   # store namespace; omit to derive from the repo dir name
profile = "branch"         # branch | cwd | fixed:<name> — how to pick a profile
extends = "base"           # fall back to this profile for keys absent in the active one

keys = []                  # declared keys (hush appends here on set/import; safe to edit)
# shims = ["npm", "pnpm"]    # commands to auto-wrap with ` + "`hush run`" + ` (needs: eval "$(hush hook)")
# disable_get = true         # forbid ` + "`hush get`" + ` entirely — values usable only via ` + "`hush run`" + `
# deny_agent_run = true      # also refuse ` + "`hush run`" + ` for detected agents — humans only
# agent_profile = "sandbox"  # detected agents resolve here instead — give it test creds
`

// cmdHook prints the shell integration snippet (shims + chpwd profile banner).
func cmdHook(_ []string) error {
	// io.WriteString (not fmt.Print): the snippet contains zsh `%F{...}` color
	// codes that fmt vet would mistake for format directives.
	_, err := io.WriteString(os.Stdout, shellHook)
	return err
}

// cmdInstall idempotently wires `eval "$(hush hook)"` into ~/.zshrc and warns if
// the binary isn't on PATH (the hook calls `hush` by bare name).
func cmdInstall(_ []string) error {
	home, err := os.UserHomeDir()
	if err != nil {
		return err
	}
	rc := filepath.Join(home, ".zshrc")
	const line = `eval "$(hush hook)"`

	raw, err := os.ReadFile(rc)
	if err != nil && !os.IsNotExist(err) {
		return err
	}
	if strings.Contains(string(raw), line) {
		fmt.Fprintf(os.Stderr, "hush: hook already present in %s\n", rc)
	} else {
		f, err := os.OpenFile(rc, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
		if err != nil {
			return err
		}
		_, werr := f.WriteString("\n# hush shell integration\n" + line + "\n")
		cerr := f.Close()
		if werr != nil {
			return werr
		}
		if cerr != nil {
			return cerr
		}
		fmt.Fprintf(os.Stderr, "hush: added hook to %s — restart your shell or run: source %s\n", rc, rc)
	}

	if _, err := exec.LookPath("hush"); err != nil {
		exe, _ := os.Executable()
		fmt.Fprintf(os.Stderr,
			"hush: warning: `hush` is not on PATH (the shell hook calls it by name).\n"+
				"      put the binary on PATH, e.g.:  cp %s ~/bin/hush  (ensure ~/bin is in PATH)\n", exe)
	}
	return nil
}

// cmdScrub prints shell commands that clear hush-managed state from the current
// shell — declared env vars and shim functions — meant to be eval'd before
// launching an agent so it can't inherit anything:  eval "$(hush scrub)"
//
// Keychain-free: it reads only .hush.toml, never the store.
func cmdScrub(_ []string) error {
	cwd, err := os.Getwd()
	if err != nil {
		return err
	}
	cfg, err := config.Load(cwd)
	if err != nil {
		return err
	}
	if len(cfg.Keys) > 0 {
		fmt.Println("unset " + strings.Join(cfg.Keys, " "))
	}
	for _, s := range cfg.Shims {
		fmt.Printf("unfunction %s 2>/dev/null\n", s)
	}
	return nil
}

// cmdContext is the fast, keychain-free feed for the shell hook. It reads only
// .hush.toml (+ git for the profile) — never the store — so it runs on every
// `cd` without decrypting or prompting. Output:
//
//	line 1:  project <p> · profile <q>
//	line 2+: one shim command name per line
func cmdContext(_ []string) error {
	cwd, err := os.Getwd()
	if err != nil {
		return err
	}
	cfg, err := config.Load(cwd)
	if err != nil {
		return err
	}
	ctx, err := resolve.Resolve(cfg)
	if err != nil {
		return err
	}
	fmt.Printf("project %s · profile %s\n", ctx.Project, ctx.Profile)
	for _, s := range cfg.Shims {
		fmt.Println(s)
	}
	return nil
}

// --- helpers ---------------------------------------------------------------

func setVal(d store.Data, proj, prof, k, v string) {
	if d[proj] == nil {
		d[proj] = map[string]map[string]string{}
	}
	if d[proj][prof] == nil {
		d[proj][prof] = map[string]string{}
	}
	d[proj][prof][k] = v
}

func get(d store.Data, proj, prof, k string) (string, bool) {
	v, ok := d[proj][prof][k]
	return v, ok
}

func copyProfile(d store.Data, proj, from, to string, force bool) int {
	src := d[proj][from]
	if d[proj] == nil {
		d[proj] = map[string]map[string]string{}
	}
	if d[proj][to] == nil {
		d[proj][to] = map[string]string{}
	}
	n := 0
	for k, v := range src {
		if _, exists := d[proj][to][k]; exists && !force {
			continue
		}
		d[proj][to][k] = v
		n++
	}
	return n
}

func contains(ss []string, s string) bool {
	for _, x := range ss {
		if x == s {
			return true
		}
	}
	return false
}

// writeConfig persists key-list changes to .hush.toml. It updates ONLY the
// `keys` array and leaves all other lines (comments, field order, project /
// profile / shims) untouched — hush never owns any other field.
func writeConfig(cfg *config.Config) error {
	return config.WriteKeys(cfg.Dir(), cfg.Keys)
}

// secureRemove overwrites a file with zeros before unlinking (best effort).
func secureRemove(path string) error {
	if fi, err := os.Stat(path); err == nil && fi.Mode().IsRegular() {
		if f, err := os.OpenFile(path, os.O_WRONLY, 0); err == nil {
			zeros := make([]byte, fi.Size())
			_, _ = f.Write(zeros)
			_ = f.Sync()
			_ = f.Close()
		}
	}
	return os.Remove(path)
}

func shellQuote(s string) string { return "'" + strings.ReplaceAll(s, "'", `'\''`) + "'" }
