package cli

import (
	"context"
	"fmt"

	"github.com/spf13/cobra"
)

// reviewCmdFlags carries the flag values for `graft review`. Each command owns
// its own reviewFlags instance so cobra binds to a stable address but values
// do not bleed between subcommands.
var reviewCmdFlags reviewFlags

var reviewCmd = &cobra.Command{
	Use:   "review <base-branch|pr-url>",
	Short: "Review changes against a base branch or pull request",
	Long: `Review changes between the current branch and a base branch,
or review a pull request by providing its URL.

This command:
1. Determines the optimal file review order based on architectural flow
2. Displays diffs in that order, piped through Delta for beautiful rendering
3. Optionally summarizes the changes using AI (--summarize)

Example:
  graft review main                                    Review changes against main
  graft review origin/main                             Review changes against remote main
  graft review HEAD~5                                  Review the last 5 commits
  graft review https://github.com/owner/repo/pull/123  Review a GitHub pull request

GitHub PR URLs require the GitHub CLI (gh) to be installed and authenticated:
  brew install gh && gh auth login

Enterprise GitHub instances are also supported.`,
	Args: cobra.ExactArgs(1),
	RunE: runReview,
}

func init() {
	reviewCmdFlags.bind(reviewCmd)
	rootCmd.AddCommand(reviewCmd)
}

func runReview(cmd *cobra.Command, args []string) error {
	ctx := cmd.Context()
	if ctx == nil {
		ctx = context.Background()
	}

	cfg := GetConfig()
	if cfg == nil {
		return fmt.Errorf("configuration not loaded")
	}

	runner := reviewCmdFlags.toRunner(cfg, args[0], false)
	return runner.run(ctx)
}
