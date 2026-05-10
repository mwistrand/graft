package cli

import (
	"context"
	"fmt"

	"github.com/spf13/cobra"
)

// scanCmdFlags carries the flag values for `graft scan`. Mirrors review's
// flag surface; see reviewFlags in review_flags.go.
var scanCmdFlags reviewFlags

var scanCmd = &cobra.Command{
	Use:   "scan",
	Short: "Review the entire codebase using AI",
	Long: `Scan the full codebase, treating every tracked file as if newly added.

This command uses the same AI pipeline as "graft review" but operates on
the entire repository instead of a branch diff:
1. Determines the optimal file review order based on architectural flow
2. Displays file contents in that order, piped through Delta for rendering
3. Optionally summarizes what the codebase does using AI (--summarize)

Example:
  graft scan                          Scan the full codebase
  graft scan --model gpt-4o           Scan using a specific model
  graft scan --ai-review              Scan with detailed AI code review
  graft scan --major-only             Skip minor files (config, docs, etc.)`,
	Args: cobra.NoArgs,
	RunE: runScan,
}

func init() {
	scanCmdFlags.bind(scanCmd)
	rootCmd.AddCommand(scanCmd)
}

func runScan(cmd *cobra.Command, args []string) error {
	ctx := cmd.Context()
	if ctx == nil {
		ctx = context.Background()
	}

	cfg := GetConfig()
	if cfg == nil {
		return fmt.Errorf("configuration not loaded")
	}

	runner := scanCmdFlags.toRunner(cfg, "", true)
	return runner.run(ctx)
}
