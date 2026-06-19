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
