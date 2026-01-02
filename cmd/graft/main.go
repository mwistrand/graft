package main

import (
	"os"

	"github.com/mwistrand/graft/internal/cli"
)

// Build-time variables set via ldflags.
var (
	version = "dev"
	commit  = "none"
	date    = "unknown"
)

func main() {
	cli.SetVersionInfo(version, commit, date)

	if err := cli.Execute(); err != nil {
		os.Exit(1)
	}
}
