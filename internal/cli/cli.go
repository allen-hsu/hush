// Package cli implements the hush command dispatch and shared helpers.
package cli

import (
	"encoding/json"
	"fmt"
	"os"

	"golang.org/x/term"

	"github.com/allen-hsu/hush/internal/config"
	"github.com/allen-hsu/hush/internal/resolve"
	"github.com/allen-hsu/hush/internal/store"
)

// Version is set by main from the build-time ldflags value.
var Version = "dev"

// Exit codes (see spec "Security model").
const (
	exitOK      = 0
	exitUsage   = 2
	exitRefused = 3
	exitDecrypt = 4
)

const usage = `hush — local, agent-safe per-worktree secrets

usage:
  hush run -- <cmd> [args...]   resolve profile, inject env, exec (use, don't see)
  hush edit [--profile p]       edit a profile in $EDITOR (TTY only)
  hush set <KEY>                set one value (interactive prompt or piped stdin)
  hush unset <KEY>              remove a key from the active profile
  hush ls [--json]              list declared keys + resolving profile (no values)
  hush get <KEY>                print a value (TTY only; refused for agents)
  hush import [path] [flags]    import an existing .env into a profile
  hush fork [--from p]          copy a profile into the active profile
  hush cp <from> <to>           copy one profile's values into another
  hush init                     scaffold a .hush.toml in the current directory
  hush install                  add the shell hook to ~/.zshrc (idempotent)
  hush hook                     print the shell integration snippet
  hush scrub                    print shell cmds to clear hush vars/shims: eval "$(hush scrub)"

  --json is accepted by ls, get, set, unset, import, fork, cp for machine output.
`

// Run dispatches a command and returns a process exit code.
func Run(args []string) int {
	if len(args) == 0 {
		fmt.Fprint(os.Stderr, usage)
		return exitUsage
	}
	cmd, rest := args[0], args[1:]

	var err error
	switch cmd {
	case "run":
		err = cmdRun(rest)
	case "edit":
		err = cmdEdit(rest)
	case "set":
		err = cmdSet(rest)
	case "unset":
		err = cmdUnset(rest)
	case "ls":
		err = cmdLs(rest)
	case "get":
		err = cmdGet(rest)
	case "import":
		err = cmdImport(rest)
	case "fork":
		err = cmdFork(rest)
	case "cp":
		err = cmdCp(rest)
	case "init":
		err = cmdInit(rest)
	case "hook":
		err = cmdHook(rest)
	case "install":
		err = cmdInstall(rest)
	case "scrub":
		err = cmdScrub(rest)
	case "context":
		err = cmdContext(rest)
	case "version", "--version", "-v":
		fmt.Println("hush " + Version)
		return exitOK
	case "-h", "--help", "help":
		fmt.Print(usage)
		return exitOK
	default:
		fmt.Fprintf(os.Stderr, "hush: unknown command %q\n\n%s", cmd, usage)
		return exitUsage
	}

	if err != nil {
		if ce, ok := err.(coded); ok {
			fmt.Fprintln(os.Stderr, "hush: "+ce.msg)
			return ce.code
		}
		fmt.Fprintln(os.Stderr, "hush: "+err.Error())
		return exitDecrypt
	}
	return exitOK
}

// coded carries an explicit exit code alongside a message.
type coded struct {
	code int
	msg  string
}

func (c coded) Error() string { return c.msg }

func usageErr(format string, a ...any) error {
	return coded{exitUsage, fmt.Sprintf(format, a...)}
}
func refusedErr(format string, a ...any) error {
	return coded{exitRefused, fmt.Sprintf(format, a...)}
}

// popJSON removes a standalone `--json` token from args, reporting whether it
// was present. Commands that support machine-readable output call this; `run`
// and `edit` deliberately don't, so `--json` after `run --` reaches the child.
func popJSON(args []string) (bool, []string) {
	out := make([]string, 0, len(args))
	found := false
	for _, a := range args {
		if a == "--json" {
			found = true
			continue
		}
		out = append(out, a)
	}
	return found, out
}

// emitJSON writes obj as indented JSON to stdout.
func emitJSON(obj any) error {
	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	return enc.Encode(obj)
}

// isAgent reports whether we're running for an agent / non-interactive context,
// in which case secret-revealing commands (get, edit) are refused.
func isAgent() bool {
	if os.Getenv("CLAUDECODE") != "" || os.Getenv("HUSH_AGENT") != "" {
		return true
	}
	return !term.IsTerminal(int(os.Stdin.Fd()))
}

// load resolves config + store + addressing context for the cwd.
func load() (*config.Config, *store.Store, resolve.Context, store.Data, error) {
	cwd, err := os.Getwd()
	if err != nil {
		return nil, nil, resolve.Context{}, nil, err
	}
	cfg, err := config.Load(cwd)
	if err != nil {
		return nil, nil, resolve.Context{}, nil, err
	}
	st, err := store.Open()
	if err != nil {
		return nil, nil, resolve.Context{}, nil, err
	}
	data, err := st.Load()
	if err != nil {
		return nil, nil, resolve.Context{}, nil, err
	}
	ctx, err := resolve.Resolve(cfg)
	if err != nil {
		return nil, nil, resolve.Context{}, nil, err
	}
	if ctx.Warning != "" {
		fmt.Fprintln(os.Stderr, "hush: "+ctx.Warning)
	}
	return cfg, st, ctx, data, nil
}
