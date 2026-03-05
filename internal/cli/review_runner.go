package cli

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/mwistrand/graft/internal/config"
	"github.com/mwistrand/graft/internal/filescan"
	"github.com/mwistrand/graft/internal/git"
	"github.com/mwistrand/graft/internal/pr"
	"github.com/mwistrand/graft/internal/prompt"
	"github.com/mwistrand/graft/internal/provider"
	"github.com/mwistrand/graft/internal/provider/testpair"
	"github.com/mwistrand/graft/internal/render"
	"github.com/mwistrand/graft/internal/tui"
)

// reviewResults holds accumulated AI-generated results from a review.
type reviewResults struct {
	summary           *provider.SummarizeResponse
	ordering          *provider.OrderResponse
	aiReview          *provider.ReviewResponse
	summaryFromCache  bool
	orderingFromCache bool
	reviewFromCache   bool
}

// reviewRunner orchestrates the review workflow with resolved options.
type reviewRunner struct {
	// Resolved options (CLI flags merged with config)
	cfg              *config.Config
	baseRef          string
	testsFirst       bool
	inlineTests      bool
	noDelta          bool
	noAnalyze        bool
	majorOnly        bool
	reviewCategories string
	reviewSeverity   string
	aiReviewFlag     string
	promptTimeoutMin int
	providerName     string
	modelName        string
	reviewModelName  string
	orderModelName   string
	selectModel      bool
	summarize        bool
	skipOrdering     bool
	doQuickReview    bool
	refresh          bool

	// Full codebase scan mode
	fullCodebase bool
	noGit        bool // true when scanning a non-git directory

	// State accumulated during run
	repo       *git.Repository
	repoDir    string
	headCommit *git.Commit
	diffResult *git.DiffResult
	repoCtx    string
	renderer   render.Renderer
	aiProvider provider.Provider
	cleanup    func()
	cache      *provider.ReviewCache
	cacheKey   string
}

// newReviewRunner creates a runner with CLI flags resolved against config.
func newReviewRunner(cfg *config.Config, baseRef string) *reviewRunner {
	return &reviewRunner{
		cfg:              cfg,
		baseRef:          baseRef,
		testsFirst:       testsFirst || cfg.TestsFirst,
		inlineTests:      inlineTests || cfg.InlineTests,
		noDelta:          noDelta || cfg.NoDelta,
		noAnalyze:        noAnalyze || cfg.NoAnalyze,
		majorOnly:        majorOnly || cfg.MajorOnly,
		reviewCategories: firstNonEmpty(reviewCategories, cfg.ReviewCategories),
		reviewSeverity:   firstNonEmpty(reviewSeverity, cfg.ReviewSeverity),
		aiReviewFlag:     aiReview,
		promptTimeoutMin: resolveTimeout(promptTimeout, cfg.PromptTimeout),
		providerName:     providerName,
		modelName:        modelName,
		reviewModelName:  firstNonEmpty(reviewModelName, cfg.ReviewModel),
		orderModelName:   firstNonEmpty(orderModelName, cfg.OrderModel),
		selectModel:      selectModel,
		summarize:        summarize || cfg.Summarize,
		skipOrdering:     skipOrdering,
		doQuickReview:    quickReview,
		refresh:          refresh,
	}
}

// firstNonEmpty returns the first non-empty string, or empty if all are empty.
func firstNonEmpty(vals ...string) string {
	for _, v := range vals {
		if v != "" {
			return v
		}
	}
	return ""
}

// resolveTimeout returns flag if explicitly set (>= 0), otherwise configVal.
func resolveTimeout(flag, configVal int) int {
	if flag >= 0 {
		return flag
	}
	return configVal
}

// validateModels checks that task-specific model configuration is complete.
// If a task-specific model is set but no default or counterpart model covers the other task,
// returns an error. Skip flags (--no-order) and opt-in flags (--summarize) are taken into account.
func (r *reviewRunner) validateModels() error {
	hasDefault := r.modelName != "" || r.cfg.Model != ""
	hasReview := r.reviewModelName != ""
	hasOrder := r.orderModelName != ""

	// Order model is needed when ordering or summary is requested
	needsOrder := !r.skipOrdering || r.summarize
	// When AI review and quick review are both disabled, no review model is needed
	needsReview := r.aiReviewFlag != "" || r.doQuickReview

	if hasReview && !hasDefault && !hasOrder && needsOrder {
		return fmt.Errorf("--review-model requires either --model (default) or --order-model to be set")
	}
	if hasOrder && !hasDefault && !hasReview && needsReview {
		return fmt.Errorf("--order-model requires either --model (default) or --review-model to be set")
	}
	return nil
}

// resolveReviewModel returns the model to use for review tasks (review, quick review).
// Returns empty to use provider default.
func (r *reviewRunner) resolveReviewModel() string {
	return r.reviewModelName
}

// resolveOrderModel returns the model to use for ordering and summary tasks.
// Returns empty to use provider default.
func (r *reviewRunner) resolveOrderModel() string {
	return r.orderModelName
}

// run executes the full review workflow.
func (r *reviewRunner) run(ctx context.Context) error {
	if err := r.validateModels(); err != nil {
		return err
	}
	if err := r.openRepo(ctx); err != nil {
		return err
	}
	if !r.fullCodebase {
		if err := r.resolvePR(ctx); err != nil {
			return err
		}
	}
	if err := r.loadDiff(ctx); err != nil {
		return err
	}
	if r.diffResult == nil {
		return nil // No changes to review
	}

	r.analyzeRepo()
	r.createRenderer()

	if err := r.initAIProvider(ctx); err != nil {
		return err
	}
	if r.cleanup != nil {
		defer r.cleanup()
	}

	if done, err := r.runQuickReview(ctx); err != nil || done {
		return err
	}

	results, err := r.generateResults(ctx)
	if err != nil {
		return err
	}

	return r.promptAndDisplay(ctx, results)
}

// openRepo opens the git repository in the current directory.
// In fullCodebase mode, falls back to filesystem scanning if not a git repo.
func (r *reviewRunner) openRepo(ctx context.Context) error {
	Verbose("Opening git repository...")
	repo, err := git.NewRepository("")
	if err != nil {
		if err == git.ErrNotARepository && r.fullCodebase {
			Verbose("Not a git repository, using filesystem scanning")
			r.noGit = true
			return nil
		}
		if err == git.ErrNotARepository {
			return fmt.Errorf("not in a git repository")
		}
		return fmt.Errorf("opening repository: %w", err)
	}
	r.repo = repo
	return nil
}

// resolvePR handles PR URL input by fetching metadata and adjusting the base ref.
func (r *reviewRunner) resolvePR(ctx context.Context) error {
	if !pr.IsPRURL(r.baseRef) {
		return nil
	}

	Verbose("Detected PR URL, fetching metadata...")
	prMeta, err := resolvePRURL(ctx, r.repo, r.baseRef)
	if err != nil {
		return err
	}

	r.baseRef = prMeta.BaseRef

	stateIndicator := ""
	switch prMeta.State {
	case pr.StateMerged:
		stateIndicator = " [MERGED]"
	case pr.StateClosed:
		stateIndicator = " [CLOSED]"
	}
	fmt.Printf("PR #%d%s: %s\n", prMeta.Number, stateIndicator, prMeta.Title)
	fmt.Printf("  %s -> %s\n", prMeta.HeadRef, prMeta.BaseRef)

	if prMeta.State != pr.StateOpen {
		fmt.Printf("  Note: Reviewing based on commit %s\n", truncateSHA(prMeta.HeadSHA))
	}
	fmt.Println()

	return nil
}

// loadDiff validates the base branch and loads diff information.
// Sets r.diffResult to nil if there are no changes.
func (r *reviewRunner) loadDiff(ctx context.Context) error {
	if r.fullCodebase {
		return r.loadFullCodebaseDiff(ctx)
	}

	Verbose("Validating base branch %s...", r.baseRef)
	if err := r.repo.ValidateBranch(ctx, r.baseRef); err != nil {
		return err
	}

	currentBranch, err := r.repo.GetCurrentBranch(ctx)
	if err != nil {
		return fmt.Errorf("getting current branch: %w", err)
	}
	fmt.Printf("Reviewing %s against %s\n\n", currentBranch, r.baseRef)

	Verbose("Getting diff information...")
	diffResult, err := r.repo.GetDiff(ctx, r.baseRef)
	if err != nil {
		return fmt.Errorf("getting diff: %w", err)
	}

	if len(diffResult.Files) == 0 {
		fmt.Println("No changes found between", currentBranch, "and", r.baseRef)
		return nil
	}

	fmt.Printf("Found %d changed files across %d commits\n\n",
		len(diffResult.Files), len(diffResult.Commits))

	repoDir, err := r.repo.GetRootDir(ctx)
	if err != nil {
		return fmt.Errorf("getting repo root: %w", err)
	}

	r.diffResult = diffResult
	r.repoDir = repoDir
	return nil
}

// loadFullCodebaseDiff loads all tracked files by diffing against the empty tree,
// or by scanning the filesystem if not in a git repository.
func (r *reviewRunner) loadFullCodebaseDiff(ctx context.Context) error {
	if r.noGit {
		return r.loadFilesystemScan()
	}

	commit, err := r.repo.GetCommit(ctx, "HEAD")
	if err != nil {
		return fmt.Errorf("getting HEAD commit: %w", err)
	}
	r.headCommit = commit

	fmt.Printf("Scanning full codebase (HEAD: %s)\n\n", commit.ShortHash)

	Verbose("Getting full codebase diff...")
	diffResult, err := r.repo.GetFullCodebaseDiff(ctx)
	if err != nil {
		return fmt.Errorf("getting full codebase diff: %w", err)
	}

	if len(diffResult.Files) == 0 {
		fmt.Println("No tracked files found in the repository")
		return nil
	}

	fmt.Printf("Found %d files\n\n", len(diffResult.Files))

	repoDir, err := r.repo.GetRootDir(ctx)
	if err != nil {
		return fmt.Errorf("getting repo root: %w", err)
	}

	r.baseRef = diffResult.BaseRef
	r.diffResult = diffResult
	r.repoDir = repoDir
	return nil
}

// loadFilesystemScan scans the current directory for files without using git.
func (r *reviewRunner) loadFilesystemScan() error {
	dir, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("getting current directory: %w", err)
	}

	fmt.Printf("Scanning directory: %s\n\n", dir)

	Verbose("Scanning filesystem...")
	diffResult, err := filescan.ScanDirectory(dir)
	if err != nil {
		return fmt.Errorf("scanning directory: %w", err)
	}

	if len(diffResult.Files) == 0 {
		fmt.Println("No files found in directory")
		return nil
	}

	fmt.Printf("Found %d files\n\n", len(diffResult.Files))

	r.diffResult = diffResult
	r.repoDir = dir
	return nil
}

// analyzeRepo performs repository structure analysis for smarter ordering.
func (r *reviewRunner) analyzeRepo() {
	if r.noAnalyze || r.skipOrdering {
		return
	}
	var err error
	r.repoCtx, err = getRepoContext(r.repoDir, r.refresh)
	if err != nil {
		Verbose("Warning: failed to analyze repository: %v", err)
	}
}

// createRenderer sets up the diff renderer.
func (r *reviewRunner) createRenderer() {
	opts := render.DefaultOptions()
	opts.UseDelta = !r.noDelta && render.IsDeltaAvailable()
	if !opts.UseDelta && !r.noDelta {
		fmt.Println("Note: Delta not found, using basic diff rendering.")
		fmt.Println("Install Delta for better rendering: https://github.com/dandavison/delta")
		fmt.Println()
	}
	r.renderer = render.New(opts)
}

// initAIProvider initializes the AI provider if any AI features are needed.
func (r *reviewRunner) initAIProvider(ctx context.Context) error {
	if !r.summarize && r.skipOrdering && !r.doQuickReview && r.aiReviewFlag == "" {
		return nil
	}

	// When both task-specific models are set, any one can initialize the provider
	// since per-request overrides will select the correct model for each call.
	allTasksCovered := r.resolveReviewModel() != "" && r.resolveOrderModel() != ""
	effectiveModel := firstNonEmpty(r.modelName, r.resolveReviewModel(), r.resolveOrderModel())

	Verbose("Initializing AI provider (model=%q, reviewModel=%q, orderModel=%q, configModel=%q)",
		r.modelName, r.resolveReviewModel(), r.resolveOrderModel(), r.cfg.Model)
	p, cleanup, err := initProvider(ctx, r.cfg, r.providerName, effectiveModel, r.selectModel, allTasksCovered)
	if err != nil {
		return fmt.Errorf("AI provider initialization failed: %w", err)
	}
	r.aiProvider = p
	r.cleanup = cleanup
	return nil
}

// runQuickReview performs a fast initial assessment if requested.
// Returns done=true if the review should stop (e.g., blocker found).
func (r *reviewRunner) runQuickReview(ctx context.Context) (done bool, err error) {
	if !r.doQuickReview || r.aiProvider == nil {
		return false, nil
	}

	Verbose("Performing quick initial assessment...")
	fmt.Println("Performing quick assessment...")

	quickResp, err := r.aiProvider.QuickReview(ctx, &provider.QuickReviewRequest{
		Model:   r.resolveReviewModel(),
		Files:   r.diffResult.Files,
		Commits: r.diffResult.Commits,
	})
	if err != nil {
		fmt.Printf("Warning: Quick review failed: %v\n\n", err)
		return false, nil
	}

	if err := outputQuickReview(quickResp); err != nil {
		return false, fmt.Errorf("outputting quick review: %w", err)
	}

	if quickResp.Verdict == provider.VerdictBlocker {
		fmt.Println("\nQuick review identified critical blockers.")
		fmt.Println("Address these issues before proceeding with full review.")
		return true, nil
	}

	return false, nil
}

// orderResult carries the outcome of background file ordering.
type orderResult struct {
	files *provider.OrderResponse
	err   error
}

// generateResults handles caching, AI summary, ordering, and review generation.
func (r *reviewRunner) generateResults(ctx context.Context) (*reviewResults, error) {
	results := &reviewResults{}

	// Set up cache
	r.cache = provider.NewReviewCache(r.repoDir)
	if r.noGit {
		r.cacheKey = provider.GenerateFullCodebaseCacheKey(
			filescan.ContentFingerprint(r.diffResult.Files),
		)
	} else if r.fullCodebase {
		r.cacheKey = provider.GenerateFullCodebaseCacheKey(r.headCommit.Hash)
	} else {
		r.cacheKey = provider.GenerateCacheKey(r.baseRef, r.diffResult.Commits)
	}

	var cachedReview *provider.CachedReview
	if !r.refresh {
		var err error
		cachedReview, err = r.cache.Load(r.cacheKey)
		if err != nil {
			Verbose("Warning: failed to load cached review: %v", err)
		}
		if cachedReview != nil {
			Verbose("Using cached AI review (key: %s)", r.cacheKey)
		}
	}

	// Get full diff for AI analysis (only if needed)
	var fullDiff string
	if r.aiProvider != nil && r.summarize && (cachedReview == nil || cachedReview.Summary == nil) {
		Verbose("Getting full diff for analysis...")
		var err error
		fullDiff, err = r.getFullDiffContent(ctx)
		if err != nil {
			return nil, fmt.Errorf("getting full diff: %w", err)
		}
	}

	// Start ordering in background while summary runs
	orderCh := r.startOrdering(ctx, cachedReview)

	// Generate summary (blocking — user reads while ordering runs)
	var err error
	fullDiff, err = r.generateSummary(ctx, cachedReview, fullDiff, results)
	if err != nil {
		return nil, err
	}

	// Generate AI review if requested
	if err := r.generateReview(ctx, cachedReview, fullDiff, results); err != nil {
		return nil, err
	}

	// Output AI review
	if err := r.outputReview(results); err != nil {
		return nil, err
	}

	// Collect ordering results
	if err := r.collectOrdering(orderCh, cachedReview, results); err != nil {
		return nil, err
	}

	// Save to cache
	r.saveCache(cachedReview, results)

	return results, nil
}

// startOrdering begins file ordering, possibly in a background goroutine.
func (r *reviewRunner) startOrdering(ctx context.Context, cached *provider.CachedReview) <-chan orderResult {
	ch := make(chan orderResult, 1)

	if r.aiProvider == nil || r.skipOrdering {
		ch <- orderResult{}
		return ch
	}

	if cached != nil && cached.Ordering != nil {
		Verbose("Using cached file ordering")
		ch <- orderResult{files: cached.Ordering}
		return ch
	}

	go func() {
		Verbose("Determining file review order...")
		files, err := r.aiProvider.OrderFiles(ctx, &provider.OrderRequest{
			Model:       r.resolveOrderModel(),
			Files:       r.diffResult.Files,
			Commits:     r.diffResult.Commits,
			RepoContext: r.repoCtx,
			TestsFirst:  r.testsFirst,
		})
		ch <- orderResult{files: files, err: err}
	}()

	return ch
}

// generateSummary produces the AI summary, using cache when available.
// Returns the (possibly updated) fullDiff value for downstream use.
func (r *reviewRunner) generateSummary(ctx context.Context, cached *provider.CachedReview, fullDiff string, results *reviewResults) (string, error) {
	if r.aiProvider == nil || !r.summarize {
		return fullDiff, nil
	}

	if cached != nil && cached.Summary != nil {
		Verbose("Using cached AI summary")
		results.summary = cached.Summary
		results.summaryFromCache = true
		if err := r.renderer.RenderSummary(results.summary); err != nil {
			return fullDiff, fmt.Errorf("rendering summary: %w", err)
		}
		return fullDiff, nil
	}

	Verbose("Generating AI summary...")
	fmt.Println("Analyzing changes...")

	summary, err := r.aiProvider.SummarizeChanges(ctx, &provider.SummarizeRequest{
		Model:    r.resolveOrderModel(),
		Files:    r.diffResult.Files,
		Commits:  r.diffResult.Commits,
		FullDiff: fullDiff,
		Options:  provider.DefaultSummarizeOptions(),
	})
	if err != nil {
		fmt.Printf("Warning: Failed to generate summary: %v\n\n", err)
		return fullDiff, nil
	}

	results.summary = summary
	if err := r.renderer.RenderSummary(summary); err != nil {
		return fullDiff, fmt.Errorf("rendering summary: %w", err)
	}
	return fullDiff, nil
}

// generateReview produces the detailed AI code review if requested.
func (r *reviewRunner) generateReview(ctx context.Context, cached *provider.CachedReview, fullDiff string, results *reviewResults) error {
	if r.aiReviewFlag == "" {
		return nil
	}

	if cached != nil && cached.Review != nil && cached.Review.Content != "" && !r.refresh {
		Verbose("Using cached AI review")
		results.aiReview = cached.Review
		results.reviewFromCache = true
		return nil
	}

	if r.aiProvider == nil {
		fmt.Println("Warning: AI review requested but no AI provider is configured")
		return nil
	}

	// Need full diff for review if not already fetched
	if fullDiff == "" {
		Verbose("Getting full diff for AI review...")
		var err error
		fullDiff, err = r.getFullDiffContent(ctx)
		if err != nil {
			return fmt.Errorf("getting full diff: %w", err)
		}
	}

	systemPrompt, err := loadReviewPrompt(r.repoDir)
	if err != nil {
		return fmt.Errorf("loading review prompt: %w", err)
	}

	Verbose("Generating AI code review...")
	fmt.Println("Generating detailed code review...")

	reviewOpts := provider.DefaultReviewOptions()
	reviewOpts.Categories = provider.ParseReviewCategories(r.reviewCategories)

	resp, err := r.aiProvider.ReviewChanges(ctx, &provider.ReviewRequest{
		Model:        r.resolveReviewModel(),
		Files:        r.diffResult.Files,
		Commits:      r.diffResult.Commits,
		FullDiff:     fullDiff,
		SystemPrompt: systemPrompt,
		Options:      reviewOpts,
	})
	if err != nil {
		fmt.Printf("Warning: Failed to generate AI review: %v\n\n", err)
		return nil
	}

	results.aiReview = resp
	return nil
}

// outputReview writes the AI review to the console or file.
func (r *reviewRunner) outputReview(results *reviewResults) error {
	if r.aiReviewFlag == "" {
		return nil
	}
	if results.aiReview == nil {
		fmt.Println("Warning: AI review was requested but no review was generated")
		return nil
	}

	severityFilter := provider.ParseReviewSeverity(r.reviewSeverity)
	outputPath := ""
	if r.aiReviewFlag != "true" {
		outputPath = r.aiReviewFlag
	}
	if err := outputAIReview(results.aiReview, outputPath, severityFilter); err != nil {
		return fmt.Errorf("outputting AI review: %w", err)
	}
	return nil
}

// collectOrdering waits for the background ordering goroutine and renders results.
func (r *reviewRunner) collectOrdering(orderCh <-chan orderResult, cached *provider.CachedReview, results *reviewResults) error {
	result := <-orderCh
	if result.err != nil {
		fmt.Printf("Warning: Failed to determine order: %v\n", result.err)
		fmt.Println("Using default file order.")
		fmt.Println()
		return nil
	}

	if result.files != nil {
		results.ordering = result.files
		if cached != nil && cached.Ordering != nil {
			results.orderingFromCache = true
		}
		if err := r.renderer.RenderOrdering(results.ordering); err != nil {
			return fmt.Errorf("rendering ordering: %w", err)
		}
	}

	return nil
}

// saveCache persists new AI results to disk.
func (r *reviewRunner) saveCache(cached *provider.CachedReview, results *reviewResults) {
	needsSave := !results.summaryFromCache || !results.orderingFromCache ||
		(r.aiReviewFlag != "" && !results.reviewFromCache && results.aiReview != nil)
	if !needsSave {
		return
	}

	// Preserve existing cached review if we didn't generate a new one
	reviewToCache := results.aiReview
	if reviewToCache == nil && cached != nil {
		reviewToCache = cached.Review
	}

	newCache := &provider.CachedReview{
		CacheKey: r.cacheKey,
		BaseRef:  r.baseRef,
		CommitHashes: func() []string {
			hashes := make([]string, len(r.diffResult.Commits))
			for i, c := range r.diffResult.Commits {
				hashes[i] = c.Hash
			}
			return hashes
		}(),
		Summary:  results.summary,
		Ordering: results.ordering,
		Review:   reviewToCache,
		CachedAt: time.Now(),
	}
	if err := r.cache.Save(newCache); err != nil {
		Verbose("Warning: failed to cache review: %v", err)
	} else {
		Verbose("Review cached (key: %s)", r.cacheKey)
	}
}

// getFullDiffContent returns the complete diff content for AI analysis.
func (r *reviewRunner) getFullDiffContent(ctx context.Context) (string, error) {
	if r.noGit {
		return filescan.GenerateFullDiff(r.repoDir, r.diffResult.Files)
	}
	if r.fullCodebase {
		return r.repo.GetFullCodebaseDiffContent(ctx)
	}
	return r.repo.GetFullDiff(ctx, r.baseRef)
}

// promptAndDisplay prompts the user to continue, handles group selection, and runs the TUI.
func (r *reviewRunner) promptAndDisplay(ctx context.Context, results *reviewResults) error {
	var timeoutDuration time.Duration
	if r.promptTimeoutMin > 0 {
		timeoutDuration = time.Duration(r.promptTimeoutMin) * time.Minute
	}

	confirmResult := prompt.ConfirmContinue("", timeoutDuration)
	if confirmResult.TimedOut {
		return fmt.Errorf("review timed out after %d minutes waiting for user input", r.promptTimeoutMin)
	}
	if !confirmResult.Continue {
		fmt.Println("Review cancelled.")
		return nil
	}

	// Build file list for display
	var filesToReview []provider.OrderedFile
	if results.ordering != nil && len(results.ordering.Groups) > 0 {
		groupsToShow := results.ordering.Groups
		if r.majorOnly {
			groupsToShow, _ = filterMajorGroups(results.ordering.Groups)
		}
		selectedGroups, err := promptGroupSelection(groupsToShow, results.ordering.Files)
		if err != nil {
			fmt.Printf("Warning: Group selection failed: %v\n", err)
			filesToReview = buildFileList(r.diffResult.Files, results.ordering)
		} else {
			filesToReview = buildGroupedFileList(results.ordering.Files, selectedGroups)
		}
	} else {
		filesToReview = buildFileList(r.diffResult.Files, results.ordering)
	}

	if r.inlineTests {
		filesToReview = testpair.PairFiles(filesToReview, r.testsFirst)
	}

	if err := tui.Run(filesToReview, r.repoDir, r.baseRef, !r.noDelta, r.fullCodebase, r.noGit); err != nil {
		return fmt.Errorf("running review TUI: %w", err)
	}

	fmt.Println("\nReview complete!")
	return nil
}
