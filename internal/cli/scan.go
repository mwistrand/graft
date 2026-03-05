package cli

import (
	"context"
	"fmt"

	"github.com/mwistrand/graft/internal/config"
	"github.com/spf13/cobra"
)

// Scan-specific flags bound by Cobra.
var (
	scanSummarize        bool
	scanSkipOrdering     bool
	scanProviderName     string
	scanModelName        string
	scanReviewModelName  string
	scanOrderModelName   string
	scanSelectModel      bool
	scanNoDelta          bool
	scanTestsFirst       bool
	scanInlineTests      bool
	scanRefresh          bool
	scanNoAnalyze        bool
	scanAIReview         string
	scanPromptTimeout    int
	scanReviewCategories string
	scanReviewSeverity   string
	scanMajorOnly        bool
	scanQuickReview      bool
)

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
	scanCmd.Flags().BoolVar(&scanSummarize, "summarize", false, "Include AI summary of changes")
	scanCmd.Flags().BoolVar(&scanSkipOrdering, "no-order", false, "Skip AI ordering, use default order")
	scanCmd.Flags().StringVar(&scanProviderName, "provider", "", "AI provider to use (default from config)")
	scanCmd.Flags().StringVar(&scanModelName, "model", "", "Model to use (default from config)")
	scanCmd.Flags().StringVar(&scanReviewModelName, "review-model", "", "Model for review tasks (review, quick review)")
	scanCmd.Flags().StringVar(&scanOrderModelName, "order-model", "", "Model for ordering and summary tasks")
	scanCmd.Flags().BoolVar(&scanSelectModel, "select-model", false, "Force interactive model selection")
	scanCmd.Flags().BoolVar(&scanNoDelta, "no-delta", false, "Disable Delta rendering")
	scanCmd.Flags().BoolVar(&scanTestsFirst, "tests-first", false, "Show test files before implementation")
	scanCmd.Flags().BoolVar(&scanInlineTests, "inline-tests", false, "Show test files alongside their implementation")
	scanCmd.Flags().BoolVar(&scanRefresh, "refresh", false, "Re-analyze repository and refresh AI cache")
	scanCmd.Flags().BoolVar(&scanNoAnalyze, "no-analyze", false, "Skip repository analysis")
	scanCmd.Flags().StringVar(&scanAIReview, "ai-review", "", "Generate detailed AI code review (optionally specify output file)")
	scanCmd.Flags().Lookup("ai-review").NoOptDefVal = "true"
	scanCmd.Flags().IntVar(&scanPromptTimeout, "prompt-timeout", -1, "Timeout in minutes for interactive prompts (0 = no timeout, default: 30)")
	scanCmd.Flags().StringVar(&scanReviewCategories, "review-categories", "", "Focus AI review on specific categories (comma-separated: design,functionality,complexity,tests,naming,comments,style,documentation)")
	scanCmd.Flags().StringVar(&scanReviewSeverity, "review-severity", "", "Filter review output by minimum severity (critical, suggestion, nit)")
	scanCmd.Flags().BoolVar(&scanMajorOnly, "major-only", false, "Only review core and supporting groups, skip minor changes")
	scanCmd.Flags().BoolVar(&scanQuickReview, "quick", false, "Perform a quick initial assessment before full review")

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

	runner := newScanRunner(cfg)
	return runner.run(ctx)
}

// newScanRunner creates a reviewRunner configured for full-codebase scanning.
func newScanRunner(cfg *config.Config) *reviewRunner {
	return &reviewRunner{
		cfg:              cfg,
		fullCodebase:     true,
		testsFirst:       scanTestsFirst || cfg.TestsFirst,
		inlineTests:      scanInlineTests || cfg.InlineTests,
		noDelta:          scanNoDelta || cfg.NoDelta,
		noAnalyze:        scanNoAnalyze || cfg.NoAnalyze,
		majorOnly:        scanMajorOnly || cfg.MajorOnly,
		reviewCategories: firstNonEmpty(scanReviewCategories, cfg.ReviewCategories),
		reviewSeverity:   firstNonEmpty(scanReviewSeverity, cfg.ReviewSeverity),
		aiReviewFlag:     scanAIReview,
		promptTimeoutMin: resolveTimeout(scanPromptTimeout, cfg.PromptTimeout),
		providerName:     scanProviderName,
		modelName:        scanModelName,
		reviewModelName:  firstNonEmpty(scanReviewModelName, cfg.ReviewModel),
		orderModelName:   firstNonEmpty(scanOrderModelName, cfg.OrderModel),
		selectModel:      scanSelectModel,
		summarize:        scanSummarize || cfg.Summarize,
		skipOrdering:     scanSkipOrdering,
		doQuickReview:    scanQuickReview,
		refresh:          scanRefresh,
	}
}
