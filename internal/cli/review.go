package cli

import (
	"context"
	"fmt"

	"github.com/spf13/cobra"
)

// Command flags bound by Cobra; resolved into a reviewRunner at startup.
var (
	skipSummary      bool
	skipOrdering     bool
	providerName     string
	modelName        string
	reviewModelName  string
	orderModelName   string
	selectModel      bool
	noDelta          bool
	testsFirst       bool
	inlineTests      bool
	refresh          bool
	noAnalyze        bool
	aiReview         string
	promptTimeout    int
	reviewCategories string
	reviewSeverity   string
	majorOnly        bool
	quickReview      bool
)

var reviewCmd = &cobra.Command{
	Use:   "review <base-branch|pr-url>",
	Short: "Review changes against a base branch or pull request",
	Long: `Review changes between the current branch and a base branch,
or review a pull request by providing its URL.

This command:
1. Summarizes the changes using AI (incorporating commit messages)
2. Determines the optimal file review order based on architectural flow
3. Displays diffs in that order, piped through Delta for beautiful rendering

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
	reviewCmd.Flags().BoolVar(&skipSummary, "no-summary", false, "Skip AI summary")
	reviewCmd.Flags().BoolVar(&skipOrdering, "no-order", false, "Skip AI ordering, use default order")
	reviewCmd.Flags().StringVar(&providerName, "provider", "", "AI provider to use (default from config)")
	reviewCmd.Flags().StringVar(&modelName, "model", "", "Model to use (default from config)")
	reviewCmd.Flags().StringVar(&reviewModelName, "review-model", "", "Model for review tasks (summarize, review, quick review)")
	reviewCmd.Flags().StringVar(&orderModelName, "order-model", "", "Model for file ordering")
	reviewCmd.Flags().BoolVar(&selectModel, "select-model", false, "Force interactive model selection")
	reviewCmd.Flags().BoolVar(&noDelta, "no-delta", false, "Disable Delta rendering")
	reviewCmd.Flags().BoolVar(&testsFirst, "tests-first", false, "Show test files before implementation")
	reviewCmd.Flags().BoolVar(&inlineTests, "inline-tests", false, "Show test files alongside their implementation")
	reviewCmd.Flags().BoolVar(&refresh, "refresh", false, "Re-analyze repository and refresh AI cache")
	reviewCmd.Flags().BoolVar(&noAnalyze, "no-analyze", false, "Skip repository analysis")
	reviewCmd.Flags().StringVar(&aiReview, "ai-review", "", "Generate detailed AI code review (optionally specify output file)")
	reviewCmd.Flags().Lookup("ai-review").NoOptDefVal = "true"
	reviewCmd.Flags().IntVar(&promptTimeout, "prompt-timeout", -1, "Timeout in minutes for interactive prompts (0 = no timeout, default: 30)")
	reviewCmd.Flags().StringVar(&reviewCategories, "review-categories", "", "Focus AI review on specific categories (comma-separated: design,functionality,complexity,tests,naming,comments,style,documentation)")
	reviewCmd.Flags().StringVar(&reviewSeverity, "review-severity", "", "Filter review output by minimum severity (critical, suggestion, nit)")
	reviewCmd.Flags().BoolVar(&majorOnly, "major-only", false, "Only review core and supporting groups, skip minor changes")
	reviewCmd.Flags().BoolVar(&quickReview, "quick", false, "Perform a quick initial assessment before full review")

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

	runner := newReviewRunner(cfg, args[0])
	return runner.run(ctx)
}
