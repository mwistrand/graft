package cli

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"sort"
	"strings"

	"github.com/spf13/cobra"

	"github.com/mwistrand/graft/internal/analysis"
	"github.com/mwistrand/graft/internal/config"
	"github.com/mwistrand/graft/internal/git"
	"github.com/mwistrand/graft/internal/prompt"
	"github.com/mwistrand/graft/internal/provider"
	"github.com/mwistrand/graft/internal/provider/claude"
	"github.com/mwistrand/graft/internal/provider/copilot"
	"github.com/mwistrand/graft/internal/render"
)

var (
	skipSummary  bool
	skipOrdering bool
	providerName string
	modelName    string
	noDelta      bool
	testsFirst   bool
	refresh      bool
	noAnalyze    bool
)

var reviewCmd = &cobra.Command{
	Use:   "review <base-branch>",
	Short: "Review changes against a base branch",
	Long: `Review changes between the current branch and a base branch.

This command:
1. Summarizes the changes using AI (incorporating commit messages)
2. Determines the optimal file review order based on architectural flow
3. Displays diffs in that order, piped through Delta for beautiful rendering

Example:
  graft review main         Review changes against main
  graft review origin/main  Review changes against remote main
  graft review HEAD~5       Review the last 5 commits`,
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
	reviewCmd.Flags().BoolVar(&refresh, "refresh", false, "Re-analyze repository structure")
	reviewCmd.Flags().BoolVar(&noAnalyze, "no-analyze", false, "Skip repository analysis")

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
	if !skipSummary || !skipOrdering {
		Verbose("Initializing AI provider...")
		aiProvider, cleanup, err = initProvider(ctx, cfg)
		if err != nil {
			fmt.Printf("Warning: %v\n", err)
			fmt.Println("Skipping AI analysis. Use --no-summary --no-order to suppress this warning.")
			fmt.Println()
			skipSummary = true
			skipOrdering = true
		}
		if cleanup != nil {
			defer cleanup()
		}
	}

	// Get full diff for AI analysis
	var fullDiff string
	if aiProvider != nil && !skipSummary {
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
	} else {
		// No ordering requested, send nil immediately
		orderCh <- orderResult{}
	}

	// AI Summary (blocking - user reads this while ordering runs in background)
	var summary *provider.SummarizeResponse
	if aiProvider != nil && !skipSummary {
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

	// Prompt user to continue (only if summary was shown, giving user time to read)
	if summary != nil {
		if !prompt.ConfirmContinue("") {
			fmt.Println("Review cancelled.")
			return nil
		}
	}

	// Wait for ordering to complete
	var orderedFiles *provider.OrderResponse
	result := <-orderCh
	if result.err != nil {
		fmt.Printf("Warning: Failed to determine order: %v\n", result.err)
		fmt.Println("Using default file order.")
		fmt.Println()
	} else if result.files != nil {
		orderedFiles = result.files
		if err := renderer.RenderOrdering(orderedFiles); err != nil {
			return fmt.Errorf("rendering ordering: %w", err)
		}
	}

	// Build file list for display
	var filesToReview []provider.OrderedFile

	// If we have groups, let user select which to review
	if orderedFiles != nil && len(orderedFiles.Groups) > 0 {
		selectedGroups, err := promptGroupSelection(orderedFiles.Groups, orderedFiles.Files)
		if err != nil {
			fmt.Printf("Warning: Group selection failed: %v\n", err)
			filesToReview = buildFileList(diffResult.Files, orderedFiles)
		} else {
			filesToReview = buildGroupedFileList(orderedFiles.Files, selectedGroups)
		}
	} else {
		filesToReview = buildFileList(diffResult.Files, orderedFiles)
	}

	// Display diffs
	for i, file := range filesToReview {
		if err := renderer.RenderFileHeader(&file, i+1, len(filesToReview)); err != nil {
			return fmt.Errorf("rendering file header: %w", err)
		}

		if err := renderer.RenderFileDiff(ctx, repoDir, baseRef, file.Path, i+1, len(filesToReview)); err != nil {
			// Non-fatal: continue with other files
			fmt.Printf("Warning: Failed to render diff for %s: %v\n", file.Path, err)
		}
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
