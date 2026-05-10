package cli

import (
	"github.com/spf13/cobra"

	"github.com/mwistrand/graft/internal/config"
)

// reviewFlags holds the CLI flag values shared by `graft review` and
// `graft scan`. Both commands register identical flags; this struct keeps the
// bindings in one place so a new flag is added once and inherited by both.
type reviewFlags struct {
	summarize        bool
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
}

// bind registers every shared review/scan flag onto cmd, pointing at fields
// of f. Each command keeps its own reviewFlags instance, so flag values do
// not leak across commands.
func (f *reviewFlags) bind(cmd *cobra.Command) {
	cmd.Flags().BoolVar(&f.summarize, "summarize", false, "Include AI summary of changes")
	cmd.Flags().BoolVar(&f.skipOrdering, "no-order", false, "Skip AI ordering, use default order")
	cmd.Flags().StringVar(&f.providerName, "provider", "", "AI provider to use (default from config)")
	cmd.Flags().StringVar(&f.modelName, "model", "", "Model to use (default from config)")
	cmd.Flags().StringVar(&f.reviewModelName, "review-model", "", "Model for review tasks (review, quick review)")
	cmd.Flags().StringVar(&f.orderModelName, "order-model", "", "Model for ordering and summary tasks")
	cmd.Flags().BoolVar(&f.selectModel, "select-model", false, "Force interactive model selection")
	cmd.Flags().BoolVar(&f.noDelta, "no-delta", false, "Disable Delta rendering")
	cmd.Flags().BoolVar(&f.testsFirst, "tests-first", false, "Show test files before implementation")
	cmd.Flags().BoolVar(&f.inlineTests, "inline-tests", false, "Show test files alongside their implementation")
	cmd.Flags().BoolVar(&f.refresh, "refresh", false, "Re-analyze repository and refresh AI cache")
	cmd.Flags().BoolVar(&f.noAnalyze, "no-analyze", false, "Skip repository analysis")
	cmd.Flags().StringVar(&f.aiReview, "ai-review", "", "Generate detailed AI code review (optionally specify output file)")
	cmd.Flags().Lookup("ai-review").NoOptDefVal = "true"
	cmd.Flags().IntVar(&f.promptTimeout, "prompt-timeout", -1, "Timeout in minutes for interactive prompts (0 = no timeout, default: 30)")
	cmd.Flags().StringVar(&f.reviewCategories, "review-categories", "", "Focus AI review on specific categories (comma-separated: design,functionality,complexity,tests,naming,comments,style,documentation)")
	cmd.Flags().StringVar(&f.reviewSeverity, "review-severity", "", "Filter review output by minimum severity (critical, suggestion, nit)")
	cmd.Flags().BoolVar(&f.majorOnly, "major-only", false, "Only review core and supporting groups, skip minor changes")
	cmd.Flags().BoolVar(&f.quickReview, "quick", false, "Perform a quick initial assessment before full review")
}

// toRunner builds a reviewRunner from the resolved flag/config values.
// fullCodebase=true selects the `graft scan` mode (diff against the empty
// tree); baseRef is required for `graft review` and ignored when
// fullCodebase=true.
func (f *reviewFlags) toRunner(cfg *config.Config, baseRef string, fullCodebase bool) *reviewRunner {
	return &reviewRunner{
		cfg:              cfg,
		baseRef:          baseRef,
		fullCodebase:     fullCodebase,
		testsFirst:       f.testsFirst || cfg.TestsFirst,
		inlineTests:      f.inlineTests || cfg.InlineTests,
		noDelta:          f.noDelta || cfg.NoDelta,
		noAnalyze:        f.noAnalyze || cfg.NoAnalyze,
		majorOnly:        f.majorOnly || cfg.MajorOnly,
		reviewCategories: firstNonEmpty(f.reviewCategories, cfg.ReviewCategories),
		reviewSeverity:   firstNonEmpty(f.reviewSeverity, cfg.ReviewSeverity),
		aiReviewFlag:     f.aiReview,
		promptTimeoutMin: resolveTimeout(f.promptTimeout, cfg.PromptTimeout),
		providerName:     f.providerName,
		modelName:        f.modelName,
		reviewModelName:  firstNonEmpty(f.reviewModelName, cfg.ReviewModel),
		orderModelName:   firstNonEmpty(f.orderModelName, cfg.OrderModel),
		selectModel:      f.selectModel,
		summarize:        f.summarize || cfg.Summarize,
		skipOrdering:     f.skipOrdering,
		doQuickReview:    f.quickReview,
		refresh:          f.refresh,
	}
}
