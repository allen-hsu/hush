// Command hush is a local, agent-safe secret manager for per-worktree env.
// See docs/SPEC.md for the design.
package main

import (
	"os"

	"github.com/allen-hsu/hush/internal/cli"
)

// version is injected at build time via -ldflags "-X main.version=...".
var version = "dev"

func main() {
	cli.Version = version
	os.Exit(cli.Run(os.Args[1:]))
}
