// Package config loads the per-repo .hush.toml — the value-free declaration of
// which keys a project needs and how to pick a profile.
package config

import (
	"errors"
	"os"
	"path/filepath"

	"github.com/BurntSushi/toml"
)

// FileName is the committed, value-free declaration file.
const FileName = ".hush.toml"

// ErrNotFound means no .hush.toml exists in the cwd or any parent.
var ErrNotFound = errors.New("no " + FileName + " found in this directory or any parent")

// Config mirrors .hush.toml. It never contains secret values.
type Config struct {
	Project string   `toml:"project,omitempty"` // store namespace; empty -> derived from repo path
	Profile string   `toml:"profile"`           // "branch" | "cwd" | "fixed:<name>"
	Extends string   `toml:"extends,omitempty"` // fallback profile chain target
	Keys    []string `toml:"keys"`              // declared contract
	Shims   []string `toml:"shims,omitempty"`   // commands to auto-wrap (opt-in)

	// DisableGet, when true, makes `hush get` always refuse (even on a TTY), so
	// values can never be printed to a terminal — `hush run` becomes the only way
	// to use them. For projects with especially sensitive secrets.
	DisableGet bool `toml:"disable_get,omitempty"`

	// DenyAgentRun, when true, makes `hush run` refuse whenever a known agent
	// marker is set (CLAUDECODE/CODEX_SANDBOX/HUSH_AGENT) — so an honest agent
	// can't pull values at all (not even via `hush run -- sh -c 'echo $X'`); a
	// human must run secret-dependent commands. Detection is marker-based and
	// best-effort: a process sharing your uid can unset the marker to evade, so
	// this guards against careless/honest agents, not a determined attacker.
	DenyAgentRun bool `toml:"deny_agent_run,omitempty"`

	// AgentProfile, when set, makes detected agents resolve against this profile
	// instead of the normal one — point it at a profile holding sandbox/test
	// credentials so an agent can still run the program, but only ever sees
	// throwaway values. The `extends` fallback is disabled for this case so a
	// missing key can't leak a real value from `base`. Alternative to
	// DenyAgentRun (which forbids running outright).
	AgentProfile string `toml:"agent_profile,omitempty"`

	dir string // directory containing the file
}

// Dir returns the directory that holds the .hush.toml.
func (c *Config) Dir() string { return c.dir }

// Load finds the nearest .hush.toml at or above start and parses it.
func Load(start string) (*Config, error) {
	dir, err := find(start)
	if err != nil {
		return nil, err
	}
	var c Config
	if _, err := toml.DecodeFile(filepath.Join(dir, FileName), &c); err != nil {
		return nil, err
	}
	if c.Profile == "" {
		c.Profile = "branch"
	}
	c.dir = dir
	return &c, nil
}

func find(start string) (string, error) {
	dir, err := filepath.Abs(start)
	if err != nil {
		return "", err
	}
	for {
		if _, err := os.Stat(filepath.Join(dir, FileName)); err == nil {
			return dir, nil
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return "", ErrNotFound
		}
		dir = parent
	}
}
