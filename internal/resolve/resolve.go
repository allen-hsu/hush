// Package resolve turns a Config + the current git/cwd state into a concrete
// project, active profile, and the resolved key/value set (walking `extends`).
package resolve

import (
	"fmt"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/allen-hsu/hush/internal/config"
	"github.com/allen-hsu/hush/internal/store"
)

// Context is the resolved addressing for a hush operation.
type Context struct {
	Project string
	Profile string // active profile after any detached-HEAD fallback
	Warning string // non-fatal note (e.g. fell back to cwd)
}

// Resolve computes the project namespace and active profile from cfg.
func Resolve(cfg *config.Config) (Context, error) {
	ctx := Context{Project: project(cfg)}

	switch {
	case cfg.Profile == "cwd":
		ctx.Profile = filepath.Base(cfg.Dir())
	case strings.HasPrefix(cfg.Profile, "fixed:"):
		ctx.Profile = strings.TrimPrefix(cfg.Profile, "fixed:")
	default: // "branch"
		if b, ok := gitBranch(cfg.Dir()); ok {
			ctx.Profile = b
		} else {
			ctx.Profile = filepath.Base(cfg.Dir())
			ctx.Warning = fmt.Sprintf(
				"not on a branch (detached HEAD or no git); using cwd profile %q", ctx.Profile)
		}
	}
	return ctx, nil
}

// Values returns the resolved key=value map for the declared keys, walking the
// extends chain (active -> ... -> extends) within the same project. Missing
// declared keys are returned in the second result.
func Values(cfg *config.Config, ctx Context, data store.Data) (map[string]string, []string) {
	chain := []string{ctx.Profile}
	if cfg.Extends != "" && cfg.Extends != ctx.Profile {
		chain = append(chain, cfg.Extends)
	}

	proj := data[ctx.Project]
	out := map[string]string{}
	var missing []string
	for _, k := range cfg.Keys {
		found := false
		for _, p := range chain {
			if v, ok := proj[p][k]; ok {
				out[k] = v
				found = true
				break
			}
		}
		if !found {
			missing = append(missing, k)
		}
	}
	return out, missing
}

func project(cfg *config.Config) string {
	if cfg.Project != "" {
		return cfg.Project
	}
	return filepath.Base(cfg.Dir())
}

func gitBranch(dir string) (string, bool) {
	cmd := exec.Command("git", "-C", dir, "symbolic-ref", "--short", "-q", "HEAD")
	out, err := cmd.Output()
	if err != nil {
		return "", false
	}
	b := strings.TrimSpace(string(out))
	return b, b != ""
}
