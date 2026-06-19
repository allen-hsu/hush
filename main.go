// Command hush is a local, agent-safe secret manager for per-worktree env.
// See docs/SPEC.md for the design.
package main

import (
	"os"
	"runtime/debug"

	"github.com/allen-hsu/hush/internal/cli"
)

// version is injected by GoReleaser via -ldflags "-X main.version=...". For
// `go install`-built binaries (no ldflags) we fall back to the module version
// embedded in the build info, so those still report a real version.
var version = "dev"

func main() {
	if version == "dev" {
		if bi, ok := debug.ReadBuildInfo(); ok && bi.Main.Version != "" && bi.Main.Version != "(devel)" {
			version = bi.Main.Version
		}
	}
	cli.Version = version
	os.Exit(cli.Run(os.Args[1:]))
}
