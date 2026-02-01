package cli

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/mwistrand/graft/internal/analysis"
	"github.com/mwistrand/graft/internal/config"
	"github.com/mwistrand/graft/internal/git"
	"github.com/mwistrand/graft/internal/pr"
	"github.com/mwistrand/graft/internal/prompt"
	"github.com/mwistrand/graft/internal/provider"
	"github.com/mwistrand/graft/internal/provider/claude"
	"github.com/mwistrand/graft/internal/provider/copilot"
	"github.com/mwistrand/graft/internal/provider/prompts"
	"github.com/mwistrand/graft/internal/provider/testpair"
	"github.com/mwistrand/graft/internal/render"
	"github.com/mwistrand/graft/internal/tui"
)

var (
	skipSummary      bool
	skipOrdering     bool
	providerName     string
	modelName        string
	noDelta          bool
	testsFirst       bool
	inlineTests      bool
	refresh          bool
	noAnalyze        bool
	aiReview         bool
	aiReviewOutput   string
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
	reviewCmd.Flags().BoolVar(&noDelta, "no-delta", false, "Disable Delta rendering")
	reviewCmd.Flags().BoolVar(&testsFirst, "tests-first", false, "Show test files before implementation")
	reviewCmd.Flags().BoolVar(&inlineTests, "inline-tests", false, "Show test files alongside their implementation")
	reviewCmd.Flags().BoolVar(&refresh, "refresh", false, "Re-analyze repository and refresh AI cache")
	reviewCmd.Flags().BoolVar(&noAnalyze, "no-analyze", false, "Skip repository analysis")
	reviewCmd.Flags().BoolVar(&aiReview, "ai-review", false, "Generate detailed AI code review")
	reviewCmd.Flags().StringVar(&aiReviewOutput, "ai-review-output", "", "Write AI review to file instead of console")
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

	baseRef := args[0]

	// Get config
	cfg := GetConfig()
	if cfg == nil {
		return fmt.Errorf("configuration not loaded")
	}

	// Create git repository
	Verbose("Opening git repository...")
	repo, err := git.NewRepository("")
	if err != nil {
		if err == git.ErrNotARepository {
			return fmt.Errorf("not in a git repository")
		}
		return fmt.Errorf("opening repository: %w", err)
	}

	// Check if input is a PR URL
	var prMeta *pr.PRMetadata
	if pr.IsPRURL(baseRef) {
		Verbose("Detected PR URL, fetching metadata...")
		prMeta, err = resolvePRURL(ctx, repo, baseRef)
		if err != nil {
			return err
		}

		// Update baseRef to use the PR's base branch
		baseRef = prMeta.BaseRef

		// Display PR info with state indicator
		stateIndicator := ""
		switch prMeta.State {
		case pr.StateMerged:
			stateIndicator = " [MERGED]"
		case pr.StateClosed:
			stateIndicator = " [CLOSED]"
		}
		fmt.Printf("PR #%d%s: %s\n", prMeta.Number, stateIndicator, prMeta.Title)
		fmt.Printf("  %s -> %s\n", prMeta.HeadRef, prMeta.BaseRef)

		// Warn about merged/closed PRs
		if prMeta.State != pr.StateOpen {
			fmt.Printf("  Note: Reviewing based on commit %s\n", truncateSHA(prMeta.HeadSHA))
		}
		fmt.Println()
	}

	// Validate base branch
	Verbose("Validating base branch %s...", baseRef)
	if err := repo.ValidateBranch(ctx, baseRef); err != nil {
		return err
	}

	// Get current branch for display
	currentBranch, err := repo.GetCurrentBranch(ctx)
	if err != nil {
		return fmt.Errorf("getting current branch: %w", err)
	}

	fmt.Printf("Reviewing %s against %s\n\n", currentBranch, baseRef)

	// Get diff information
	Verbose("Getting diff information...")
	diffResult, err := repo.GetDiff(ctx, baseRef)
	if err != nil {
		return fmt.Errorf("getting diff: %w", err)
	}

	if len(diffResult.Files) == 0 {
		fmt.Println("No changes found between", currentBranch, "and", baseRef)
		return nil
	}

	fmt.Printf("Found %d changed files across %d commits\n\n",
		len(diffResult.Files), len(diffResult.Commits))

	// Get repository root for analysis
	repoDir, err := repo.GetRootDir(ctx)
	if err != nil {
		return fmt.Errorf("getting repo root: %w", err)
	}

	// Repository analysis for smarter ordering
	var repoContext string
	if !noAnalyze && !skipOrdering {
		repoContext, err = getRepoContext(repoDir)
		if err != nil {
			Verbose("Warning: failed to analyze repository: %v", err)
		}
	}

	// Create renderer
	renderOpts := render.DefaultOptions()
	renderOpts.UseDelta = !noDelta && render.IsDeltaAvailable()
	if !renderOpts.UseDelta && !noDelta {
		fmt.Println("Note: Delta not found, using basic diff rendering.")
		fmt.Println("Install Delta for better rendering: https://github.com/dandavison/delta")
		fmt.Println()
	}
	renderer := render.New(renderOpts)

	// Initialize AI provider if needed
	var aiProvider provider.Provider
	var cleanup func()
	if !skipSummary || !skipOrdering || quickReview {
		Verbose("Initializing AI provider...")
		aiProvider, cleanup, err = initProvider(ctx, cfg)
		if err != nil {
			fmt.Printf("Warning: %v\n", err)
			fmt.Println("Skipping AI analysis. Use --no-summary --no-order to suppress this warning.")
			fmt.Println()
			skipSummary = true
			skipOrdering = true
			quickReview = false
		}
		if cleanup != nil {
			defer cleanup()
		}
	}

	// Quick review: fast initial assessment before detailed review
	if quickReview && aiProvider != nil {
		Verbose("Performing quick initial assessment...")
		fmt.Println("Performing quick assessment...")

		quickResp, err := aiProvider.QuickReview(ctx, &provider.QuickReviewRequest{
			Files:   diffResult.Files,
			Commits: diffResult.Commits,
		})
		if err != nil {
			fmt.Printf("Warning: Quick review failed: %v\n\n", err)
		} else {
			if err := outputQuickReview(quickResp); err != nil {
				return fmt.Errorf("outputting quick review: %w", err)
			}

			// If blocker, stop here
			if quickResp.Verdict == provider.VerdictBlocker {
				fmt.Println("\nQuick review identified critical blockers.")
				fmt.Println("Address these issues before proceeding with full review.")
				return nil
			}
		}
	}

	// Set up review cache
	reviewCache := provider.NewReviewCache(repoDir)
	cacheKey := provider.GenerateCacheKey(baseRef, diffResult.Commits)

	// Check for cached review
	var cachedReview *provider.CachedReview
	if !refresh {
		cachedReview, err = reviewCache.Load(cacheKey)
		if err != nil {
			Verbose("Warning: failed to load cached review: %v", err)
		}
		if cachedReview != nil {
			Verbose("Using cached AI review (key: %s)", cacheKey)
		}
	}

	// Get full diff for AI analysis (only if needed)
	var fullDiff string
	if aiProvider != nil && !skipSummary && (cachedReview == nil || cachedReview.Summary == nil) {
		Verbose("Getting full diff for analysis...")
		fullDiff, err = repo.GetFullDiff(ctx, baseRef)
		if err != nil {
			return fmt.Errorf("getting full diff: %w", err)
		}
	}

	// Start file ordering in background while we generate and display summary
	type orderResult struct {
		files *provider.OrderResponse
		err   error
	}
	orderCh := make(chan orderResult, 1)

	if aiProvider != nil && !skipOrdering {
		// Check if we have cached ordering
		if cachedReview != nil && cachedReview.Ordering != nil {
			Verbose("Using cached file ordering")
			orderCh <- orderResult{files: cachedReview.Ordering}
		} else {
			go func() {
				Verbose("Determining file review order...")
				files, err := aiProvider.OrderFiles(ctx, &provider.OrderRequest{
					Files:       diffResult.Files,
					Commits:     diffResult.Commits,
					RepoContext: repoContext,
					TestsFirst:  testsFirst,
				})
				orderCh <- orderResult{files: files, err: err}
			}()
		}
	} else {
		// No ordering requested, send nil immediately
		orderCh <- orderResult{}
	}

	// AI Summary (blocking - user reads this while ordering runs in background)
	var summary *provider.SummarizeResponse
	var summaryFromCache bool
	if aiProvider != nil && !skipSummary {
		// Check if we have cached summary
		if cachedReview != nil && cachedReview.Summary != nil {
			Verbose("Using cached AI summary")
			summary = cachedReview.Summary
			summaryFromCache = true
			if err := renderer.RenderSummary(summary); err != nil {
				return fmt.Errorf("rendering summary: %w", err)
			}
		} else {
			Verbose("Generating AI summary...")
			fmt.Println("Analyzing changes...")

			summary, err = aiProvider.SummarizeChanges(ctx, &provider.SummarizeRequest{
				Files:    diffResult.Files,
				Commits:  diffResult.Commits,
				FullDiff: fullDiff,
				Options:  provider.DefaultSummarizeOptions(),
			})
			if err != nil {
				fmt.Printf("Warning: Failed to generate summary: %v\n\n", err)
			} else {
				if err := renderer.RenderSummary(summary); err != nil {
					return fmt.Errorf("rendering summary: %w", err)
				}
			}
		}
	}

	// Handle AI review generation (before prompting user to continue)
	var aiReviewResponse *provider.ReviewResponse
	var reviewFromCache bool
	if aiReview {
		// Check if we have cached review (with non-empty content)
		if cachedReview != nil && cachedReview.Review != nil && cachedReview.Review.Content != "" && !refresh {
			Verbose("Using cached AI review")
			aiReviewResponse = cachedReview.Review
			reviewFromCache = true
		} else if aiProvider == nil {
			fmt.Println("Warning: AI review requested but no AI provider is configured")
		} else {
			// Need full diff for review if not already fetched
			if fullDiff == "" {
				Verbose("Getting full diff for AI review...")
				fullDiff, err = repo.GetFullDiff(ctx, baseRef)
				if err != nil {
					return fmt.Errorf("getting full diff: %w", err)
				}
			}

			// Load system prompt (uses .graft/code-reviewer.md override or embedded default)
			systemPrompt, err := loadReviewPrompt(repoDir)
			if err != nil {
				return fmt.Errorf("loading review prompt: %w", err)
			}

			Verbose("Generating AI code review...")
			fmt.Println("Generating detailed code review...")

			reviewOpts := provider.DefaultReviewOptions()
			reviewOpts.Categories = provider.ParseReviewCategories(reviewCategories)

			aiReviewResponse, err = aiProvider.ReviewChanges(ctx, &provider.ReviewRequest{
				Files:        diffResult.Files,
				Commits:      diffResult.Commits,
				FullDiff:     fullDiff,
				SystemPrompt: systemPrompt,
				Options:      reviewOpts,
			})
			if err != nil {
				fmt.Printf("Warning: Failed to generate AI review: %v\n\n", err)
			}
		}
	}

	// Output AI review before prompting to continue
	if aiReview {
		if aiReviewResponse != nil {
			severityFilter := provider.ParseReviewSeverity(reviewSeverity)
			if err := outputAIReview(aiReviewResponse, aiReviewOutput, severityFilter); err != nil {
				return fmt.Errorf("outputting AI review: %w", err)
			}
		} else {
			fmt.Println("Warning: AI review was requested but no review was generated")
		}
	}

	// Prompt user to continue (after showing summary and AI review)
	if summary != nil || aiReviewResponse != nil {
		// Determine effective timeout (flag overrides config, -1 means use config)
		effectiveTimeout := cfg.PromptTimeout
		if promptTimeout >= 0 {
			effectiveTimeout = promptTimeout
		}

		var timeoutDuration time.Duration
		if effectiveTimeout > 0 {
			timeoutDuration = time.Duration(effectiveTimeout) * time.Minute
		}

		result := prompt.ConfirmContinue("", timeoutDuration)
		if result.TimedOut {
			return fmt.Errorf("review timed out after %d minutes waiting for user input", effectiveTimeout)
		}
		if !result.Continue {
			fmt.Println("Review cancelled.")
			return nil
		}
	}

	// Wait for ordering to complete
	var orderedFiles *provider.OrderResponse
	var orderingFromCache bool
	result := <-orderCh
	if result.err != nil {
		fmt.Printf("Warning: Failed to determine order: %v\n", result.err)
		fmt.Println("Using default file order.")
		fmt.Println()
	} else if result.files != nil {
		orderedFiles = result.files
		// Check if this came from cache (we set it directly, no goroutine)
		if cachedReview != nil && cachedReview.Ordering != nil {
			orderingFromCache = true
		}
		if err := renderer.RenderOrdering(orderedFiles); err != nil {
			return fmt.Errorf("rendering ordering: %w", err)
		}
	}

	// Save to cache if we got new results from AI
	if !summaryFromCache || !orderingFromCache || (aiReview && !reviewFromCache && aiReviewResponse != nil) {
		// Preserve existing cached review if we didn't generate a new one
		reviewToCache := aiReviewResponse
		if reviewToCache == nil && cachedReview != nil {
			reviewToCache = cachedReview.Review
		}

		newCache := &provider.CachedReview{
			CacheKey: cacheKey,
			BaseRef:  baseRef,
			CommitHashes: func() []string {
				hashes := make([]string, len(diffResult.Commits))
				for i, c := range diffResult.Commits {
					hashes[i] = c.Hash
				}
				return hashes
			}(),
			Summary:  summary,
			Ordering: orderedFiles,
			Review:   reviewToCache,
			CachedAt: time.Now(),
		}
		if err := reviewCache.Save(newCache); err != nil {
			Verbose("Warning: failed to cache review: %v", err)
		} else {
			Verbose("Review cached (key: %s)", cacheKey)
		}
	}

	// Build file list for display
	var filesToReview []provider.OrderedFile

	// If we have groups, let user select which to review
	if orderedFiles != nil && len(orderedFiles.Groups) > 0 {
		groupsToShow := orderedFiles.Groups

		// Filter out minor groups if --major-only is set
		if majorOnly {
			groupsToShow, _ = filterMajorGroups(orderedFiles.Groups)
		}

		selectedGroups, err := promptGroupSelection(groupsToShow, orderedFiles.Files)
		if err != nil {
			fmt.Printf("Warning: Group selection failed: %v\n", err)
			filesToReview = buildFileList(diffResult.Files, orderedFiles)
		} else {
			filesToReview = buildGroupedFileList(orderedFiles.Files, selectedGroups)
		}
	} else {
		filesToReview = buildFileList(diffResult.Files, orderedFiles)
	}

	// Pair test files with their implementation if --inline-tests is set
	if inlineTests {
		filesToReview = testpair.PairFiles(filesToReview, testsFirst)
	}

	// Display diffs with interactive TUI
	if err := tui.Run(filesToReview, repoDir, baseRef, !noDelta); err != nil {
		return fmt.Errorf("running review TUI: %w", err)
	}

	fmt.Println("\nReview complete!")
	return nil
}

// initProvider creates an AI provider based on configuration.
// Returns a cleanup function that should be called when done (may be nil).
func initProvider(ctx context.Context, cfg *config.Config) (provider.Provider, func(), error) {
	pName := providerName
	if pName == "" {
		pName = cfg.Provider
	}

	model := modelName
	if model == "" {
		model = cfg.Model
	}

	switch pName {
	case "claude", "":
		apiKey := cfg.AnthropicAPIKey
		if apiKey == "" {
			return nil, nil, fmt.Errorf("Anthropic API key not set. Run 'graft config set anthropic-api-key <key>' or set ANTHROPIC_API_KEY")
		}
		p, err := claude.New(apiKey, model)
		return p, nil, err

	case "copilot":
		baseURL := cfg.CopilotBaseURL
		copilotModel := modelName
		p, err := copilot.New(baseURL, copilotModel)
		if err != nil {
			return nil, nil, err
		}

		// Ensure the copilot-api proxy is running
		started, err := p.EnsureProxyRunning(ctx, func(format string, args ...any) {
			fmt.Printf(format+"\n", args...)
		})
		if err != nil {
			return nil, nil, fmt.Errorf("copilot proxy: %w", err)
		}

		// Return cleanup function if we started the proxy
		var cleanup func()
		if started {
			cleanup = func() {
				fmt.Println("Stopping copilot-api proxy...")
				p.Close()
			}
		}

		// Prompt for model selection if no --model flag was provided
		if modelName == "" {
			selected, err := promptForModel(ctx, p)
			if err != nil {
				// On error, fall back to default model and inform the user
				fmt.Printf("Note: %v\n", err)
				p.SetModel(copilot.DefaultModel)
				fmt.Printf("Using default model: %s\n\n", p.Model())
			} else if selected != "" {
				p.SetModel(selected)
				fmt.Printf("Using model: %s\n\n", selected)
			}
		}

		return p, cleanup, nil

	default:
		return nil, nil, fmt.Errorf("unknown provider %q; available: claude, copilot", pName)
	}
}

// buildFileList creates the ordered list of files to review.
func buildFileList(files []git.FileDiff, aiOrder *provider.OrderResponse) []provider.OrderedFile {
	// If we have AI ordering, use it
	if aiOrder != nil && len(aiOrder.Files) > 0 {
		return aiOrder.Files
	}

	// Default: convert FileDiff to OrderedFile in original order
	result := make([]provider.OrderedFile, len(files))
	for i, f := range files {
		result[i] = provider.OrderedFile{
			Path:        f.Path,
			Category:    categorizeFile(f.Path),
			Priority:    i + 1,
			Description: describeStatus(f.Status),
		}
	}
	return result
}

// filterMajorGroups removes minor groups from the list.
// Returns the filtered groups and the count of removed groups.
func filterMajorGroups(groups []provider.OrderGroup) ([]provider.OrderGroup, int) {
	var major []provider.OrderGroup
	minorCount := 0

	for _, g := range groups {
		sig := provider.NormalizeSignificance(g.Significance)
		if sig == provider.SignificanceMinor {
			minorCount++
		} else {
			major = append(major, g)
		}
	}

	if minorCount > 0 {
		fmt.Printf("Skipping %d minor group(s) (use without --major-only to include)\n\n", minorCount)
	}

	return major, minorCount
}

// promptGroupSelection presents an interactive menu for group selection.
// Returns the groups in the order the user wants to review them.
func promptGroupSelection(groups []provider.OrderGroup, files []provider.OrderedFile) ([]provider.OrderGroup, error) {
	// Count files per group for display
	fileCounts := make(map[string]int)
	for _, f := range files {
		if f.Group != "" {
			fileCounts[f.Group]++
		}
	}

	return prompt.SelectGroups(groups, fileCounts)
}

// buildGroupedFileList creates the file list based on selected group order.
// Files are ordered by group (in selected order), then by priority within each group.
func buildGroupedFileList(files []provider.OrderedFile, selectedGroups []provider.OrderGroup) []provider.OrderedFile {
	// Create a map of group name -> order index based on selection
	groupOrder := make(map[string]int)
	for i, g := range selectedGroups {
		groupOrder[g.Name] = i
	}

	// Build set of selected group names
	selectedSet := make(map[string]bool)
	for _, g := range selectedGroups {
		selectedSet[g.Name] = true
	}

	// Filter to only files in selected groups
	filtered := make([]provider.OrderedFile, 0, len(files))
	for _, f := range files {
		if f.Group == "" || selectedSet[f.Group] {
			filtered = append(filtered, f)
		}
	}

	// Sort files: first by group order, then by priority within group
	sort.SliceStable(filtered, func(i, j int) bool {
		gi, goki := groupOrder[filtered[i].Group]
		gj, gokj := groupOrder[filtered[j].Group]

		// Ungrouped files go at the end
		if !goki && gokj {
			return false
		}
		if goki && !gokj {
			return true
		}

		// Both grouped or both ungrouped - compare group order
		if gi != gj {
			return gi < gj
		}

		// Same group - sort by priority
		return filtered[i].Priority < filtered[j].Priority
	})

	return filtered
}

// categorizeFile assigns a category based on file path.
func categorizeFile(path string) string {
	switch {
	case containsAny(path, "_test.go", "_test.", "test_"):
		return provider.CategoryTest
	case containsAny(path, "cmd/", "main.go"):
		return provider.CategoryEntryPoint
	case containsAny(path, "internal/", "pkg/"):
		return provider.CategoryBusinessLogic
	case containsAny(path, "adapter", "repository", "client"):
		return provider.CategoryAdapter
	case containsAny(path, "model", "entity", "types"):
		return provider.CategoryModel
	case containsAny(path, "config", ".json", ".yaml", ".toml"):
		return provider.CategoryConfig
	case containsAny(path, ".md", "doc/", "docs/"):
		return provider.CategoryDocs
	default:
		return provider.CategoryOther
	}
}

// describeStatus returns a description based on file status.
func describeStatus(status string) string {
	switch status {
	case git.StatusAdded:
		return "New file"
	case git.StatusDeleted:
		return "Deleted"
	case git.StatusRenamed:
		return "Renamed"
	default:
		return "Modified"
	}
}

// containsAny returns true if s contains any of the given substrings.
func containsAny(s string, substrs ...string) bool {
	for _, substr := range substrs {
		if strings.Contains(s, substr) {
			return true
		}
	}
	return false
}

// getRepoContext analyzes the repository and returns context for AI ordering.
// Handles permission prompting and caching.
func getRepoContext(repoDir string) (string, error) {
	cache := analysis.NewCache(repoDir)

	// Check if we have cached analysis
	if !refresh && cache.Exists() {
		cached, err := cache.Load()
		if err != nil {
			return "", err
		}
		if cached != nil {
			Verbose("Using cached repository analysis")
			return cached.FormatContext(), nil
		}
	}

	// Need to run fresh analysis - prompt for permission if first time
	if !cache.Exists() {
		if !promptForAnalysisPermission() {
			return "", nil // User declined, continue without analysis
		}
	} else if refresh {
		fmt.Println("Refreshing repository analysis...")
	}

	// Run analysis
	fmt.Println("Analyzing repository structure...")
	result, isNew, err := analysis.GetOrAnalyze(repoDir, refresh)
	if err != nil {
		return "", err
	}

	if isNew {
		fmt.Printf("Detected: %s", result.Type)
		if len(result.Languages) > 0 {
			fmt.Printf(" (%s)", strings.Join(result.Languages, ", "))
		}
		if len(result.Frameworks) > 0 {
			fmt.Printf(" with %s", strings.Join(result.Frameworks, ", "))
		}
		fmt.Println()
		fmt.Printf("Analysis cached at %s\n\n", cache.CachePath())
	}

	return result.FormatContext(), nil
}

// promptForModel asks the user to select a model from the available options.
func promptForModel(ctx context.Context, p provider.Provider) (string, error) {
	lister, ok := p.(provider.ModelLister)
	if !ok {
		return "", fmt.Errorf("provider does not support listing models")
	}

	Verbose("Fetching available models...")
	models, err := lister.ListModels(ctx)
	if err != nil {
		return "", err
	}

	if len(models) == 0 {
		return "", fmt.Errorf("no models available from provider")
	}

	fmt.Println()
	return prompt.SelectModel(models)
}

// promptForAnalysisPermission asks the user if they want to analyze the repository.
func promptForAnalysisPermission() bool {
	fmt.Println("Graft can analyze your repository structure to provide smarter file ordering.")
	fmt.Println("This scans directory structure and config files (not code contents).")
	fmt.Println()
	fmt.Print("Allow repository analysis? [Y/n] ")

	reader := bufio.NewReader(os.Stdin)
	input, err := reader.ReadString('\n')
	if err != nil {
		return false
	}

	input = strings.TrimSpace(strings.ToLower(input))
	// Default to yes if empty, or explicit yes
	if input == "" || input == "y" || input == "yes" {
		fmt.Println()
		return true
	}

	fmt.Println("Skipping repository analysis.")
	return false
}

// loadReviewPrompt loads the review system prompt.
// First checks for a custom override at .graft/code-reviewer.md in the repository.
// Falls back to the embedded default prompt if no override exists.
func loadReviewPrompt(repoDir string) (string, error) {
	// Check for repository-specific override
	overridePath := filepath.Join(repoDir, ".graft", "code-reviewer.md")
	data, err := os.ReadFile(overridePath)
	if err == nil {
		Verbose("Using custom code reviewer prompt from %s", overridePath)
		return string(data), nil
	}
	if !os.IsNotExist(err) {
		return "", fmt.Errorf("reading review prompt override: %w", err)
	}

	// Use embedded default prompt
	return prompts.DefaultCodeReviewerPrompt, nil
}

// outputAIReview writes the AI review to console or a file.
// If severityFilter is set, only comments at or above that severity level are shown.
func outputAIReview(resp *provider.ReviewResponse, outputPath string, severityFilter provider.ReviewSeverity) error {
	if resp == nil || (resp.Content == "" && resp.Structured == nil) {
		return fmt.Errorf("AI review content is empty")
	}

	// Apply severity filter to structured review if present
	structured := resp.Structured
	if structured != nil && severityFilter != "" {
		structured = structured.FilterBySeverity(severityFilter)
	}

	if outputPath != "" {
		// Write markdown content to file
		var content string
		if structured != nil {
			content = provider.GenerateMarkdownFromReview(structured)
		} else {
			content = resp.Content
		}

		// Create parent directory if it doesn't exist
		dir := filepath.Dir(outputPath)
		if err := os.MkdirAll(dir, 0755); err != nil {
			return fmt.Errorf("creating output directory: %w", err)
		}
		if err := os.WriteFile(outputPath, []byte(content), 0600); err != nil {
			return fmt.Errorf("writing review to file: %w", err)
		}
		fmt.Printf("AI review written to: %s\n\n", outputPath)
	} else {
		// Console output
		fmt.Println("\n" + strings.Repeat("=", 60))
		fmt.Println("AI CODE REVIEW")
		if severityFilter != "" {
			fmt.Printf("(filtered: %s and above)\n", severityFilter)
		}
		fmt.Println(strings.Repeat("=", 60))

		if structured != nil {
			outputStructuredReview(structured)
		} else {
			fmt.Println()
			fmt.Println(resp.Content)
		}

		fmt.Println(strings.Repeat("=", 60) + "\n")
	}
	return nil
}

// outputStructuredReview renders a structured review to the console.
func outputStructuredReview(review *provider.StructuredReview) {
	// Summary
	if review.Summary != "" {
		fmt.Println()
		fmt.Println(review.Summary)
	}

	// Count by severity for stats
	counts := review.CountBySeverity()
	critical := counts[provider.SeverityCritical]
	suggestions := counts[provider.SeveritySuggestion]
	nits := counts[provider.SeverityNit]

	if critical+suggestions+nits > 0 {
		fmt.Printf("\nFindings: %d critical, %d suggestions, %d nits\n",
			critical, suggestions, nits)
	}

	// Group comments by category
	byCategory := review.CommentsByCategory()

	// Output in standard category order
	for _, cat := range provider.AllReviewCategories() {
		comments := byCategory[cat]
		if len(comments) == 0 {
			continue
		}

		// Category header with counts
		catCritical, catSuggestions, catNits := 0, 0, 0
		for _, c := range comments {
			switch c.Severity {
			case provider.SeverityCritical:
				catCritical++
			case provider.SeveritySuggestion:
				catSuggestions++
			case provider.SeverityNit:
				catNits++
			}
		}

		fmt.Println()
		fmt.Println(strings.Repeat("-", 60))
		header := strings.ToUpper(provider.CategoryDisplayName(cat))
		if cat == provider.CategoryPraise {
			fmt.Printf("%s (%d items)\n", header, len(comments))
		} else {
			parts := []string{}
			if catCritical > 0 {
				parts = append(parts, fmt.Sprintf("%d critical", catCritical))
			}
			if catSuggestions > 0 {
				parts = append(parts, fmt.Sprintf("%d suggestions", catSuggestions))
			}
			if catNits > 0 {
				parts = append(parts, fmt.Sprintf("%d nits", catNits))
			}
			if len(parts) > 0 {
				fmt.Printf("%s (%s)\n", header, strings.Join(parts, ", "))
			} else {
				fmt.Println(header)
			}
		}
		fmt.Println(strings.Repeat("-", 60))

		for _, c := range comments {
			// Severity indicator
			var indicator string
			switch c.Severity {
			case provider.SeverityCritical:
				indicator = "[!]"
			case provider.SeveritySuggestion:
				indicator = "[*]"
			case provider.SeverityNit:
				indicator = "[-]"
			}

			// For praise, use checkmark
			if cat == provider.CategoryPraise {
				indicator = "[+]"
			}

			// Title with optional file/line reference
			if c.File != "" {
				if c.Line > 0 {
					fmt.Printf("\n%s %s (%s:%d)\n", indicator, c.Title, c.File, c.Line)
				} else {
					fmt.Printf("\n%s %s (%s)\n", indicator, c.Title, c.File)
				}
			} else {
				fmt.Printf("\n%s %s\n", indicator, c.Title)
			}

			// Description (indented)
			if c.Description != "" {
				lines := strings.Split(c.Description, "\n")
				for _, line := range lines {
					fmt.Printf("    %s\n", line)
				}
			}

			// Suggestion (indented, with label)
			if c.Suggestion != "" {
				fmt.Println()
				fmt.Println("    Suggestion:")
				lines := strings.Split(c.Suggestion, "\n")
				for _, line := range lines {
					fmt.Printf("      %s\n", line)
				}
			}
		}
	}
	fmt.Println()
}

// outputQuickReview renders the quick review verdict to the console.
func outputQuickReview(resp *provider.QuickReviewResponse) error {
	if resp == nil {
		return fmt.Errorf("quick review response is empty")
	}

	fmt.Println()
	fmt.Println(strings.Repeat("=", 60))
	fmt.Println("QUICK ASSESSMENT")
	fmt.Println(strings.Repeat("=", 60))

	// Verdict with visual indicator
	var verdictIcon, verdictLabel string
	switch resp.Verdict {
	case provider.VerdictApprove:
		verdictIcon = "[OK]"
		verdictLabel = "APPROVE"
	case provider.VerdictConcerns:
		verdictIcon = "[?]"
		verdictLabel = "CONCERNS"
	case provider.VerdictBlocker:
		verdictIcon = "[!]"
		verdictLabel = "BLOCKER"
	default:
		verdictIcon = "[?]"
		verdictLabel = string(resp.Verdict)
	}

	fmt.Printf("\nVerdict: %s %s\n", verdictIcon, verdictLabel)
	fmt.Println()

	// Summary
	if resp.Summary != "" {
		fmt.Println(resp.Summary)
		fmt.Println()
	}

	// Concerns
	if len(resp.Concerns) > 0 {
		fmt.Println("Concerns:")
		for _, c := range resp.Concerns {
			fmt.Printf("  - %s\n", c)
		}
		fmt.Println()
	}

	// Proceed indicator
	if resp.Proceed {
		fmt.Println("Proceeding with full review...")
	} else {
		fmt.Println("Review blocked. Address concerns before continuing.")
	}

	fmt.Println(strings.Repeat("=", 60))
	fmt.Println()

	return nil
}

// resolvePRURL fetches PR metadata and ensures commits are available locally.
func resolvePRURL(ctx context.Context, repo *git.Repository, url string) (*pr.PRMetadata, error) {
	meta, err := pr.Resolve(ctx, url)
	if err != nil {
		return nil, handlePRError(err)
	}

	// Check if we're in the right repository
	if err := validateRepository(ctx, repo, meta); err != nil {
		return nil, err
	}

	// Ensure the PR's commits are fetched
	if err := ensureCommitsFetched(ctx, repo, meta); err != nil {
		return nil, err
	}

	return meta, nil
}

// validateRepository checks that the local repo matches the PR's repo.
func validateRepository(ctx context.Context, repo *git.Repository, meta *pr.PRMetadata) error {
	remoteURL, err := repo.GetRemoteURL(ctx, "origin")
	if err != nil {
		return fmt.Errorf("getting remote URL: %w", err)
	}

	// Parse the remote URL to extract host, owner, and repo
	remoteInfo, err := pr.ParseRemoteURL(remoteURL)
	if err != nil {
		return fmt.Errorf("parsing remote URL %q: %w", remoteURL, err)
	}

	// Compare parsed components
	if !remoteInfo.Matches(&meta.PRInfo) {
		return fmt.Errorf("local repository (%s/%s on %s) doesn't match PR repository (%s/%s on %s)",
			remoteInfo.Owner, remoteInfo.Repo, remoteInfo.Host,
			meta.Owner, meta.Repo, meta.Host)
	}

	return nil
}

// ensureCommitsFetched fetches the PR's commits if not available locally.
func ensureCommitsFetched(ctx context.Context, repo *git.Repository, meta *pr.PRMetadata) error {
	// Check if we have the head commit
	if repo.HasRef(ctx, meta.HeadSHA) {
		Verbose("PR commits already available locally")
		return nil
	}

	// Fetch the PR branch
	Verbose("Fetching PR commits from remote...")

	// For GitHub PRs, we can fetch the PR ref directly with an explicit refspec
	if meta.Platform == pr.PlatformGitHub {
		remoteRef := fmt.Sprintf("refs/pull/%d/head", meta.Number)
		localRef := fmt.Sprintf("refs/remotes/origin/pr/%d", meta.Number)

		if err := repo.FetchRefTo(ctx, "origin", remoteRef, localRef); err != nil {
			// Fall back to fetching the head branch
			Verbose("Could not fetch PR ref, trying head branch...")
			if err := repo.FetchRef(ctx, "origin", meta.HeadRef); err != nil {
				return fmt.Errorf("fetching PR commits: %w", err)
			}
		}

		// Verify we now have the commit
		if !repo.HasRef(ctx, meta.HeadSHA) {
			return fmt.Errorf("PR head commit %s not found after fetch; the branch may have been force-pushed or deleted", truncateSHA(meta.HeadSHA))
		}
		return nil
	}

	// For other platforms, fetch the head branch
	if err := repo.FetchRef(ctx, "origin", meta.HeadRef); err != nil {
		return fmt.Errorf("fetching PR commits: %w", err)
	}

	// Verify we now have the commit
	if !repo.HasRef(ctx, meta.HeadSHA) {
		return fmt.Errorf("PR head commit %s not found after fetch; the branch may have been force-pushed or deleted", truncateSHA(meta.HeadSHA))
	}
	return nil
}

// truncateSHA returns a truncated SHA for display (up to 12 chars).
func truncateSHA(sha string) string {
	if len(sha) > 12 {
		return sha[:12]
	}
	return sha
}

// handlePRError converts PR errors to user-friendly messages.
func handlePRError(err error) error {
	switch e := err.(type) {
	case *pr.ErrCLINotFound:
		return fmt.Errorf(`%w

To review GitHub PRs, install the GitHub CLI:
  brew install gh && gh auth login

Or visit: https://cli.github.com/`, e)

	case *pr.ErrPRNotFound:
		return fmt.Errorf("pull request not found: %s\n\nCheck that the URL is correct and you have access to the repository", e.URL)

	default:
		return err
	}
}
