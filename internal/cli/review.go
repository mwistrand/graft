package cli

import (
	"context"
	"fmt"
	"strings"

	"github.com/mwistrand/graft/internal/config"
	"github.com/mwistrand/graft/internal/git"
	"github.com/mwistrand/graft/internal/provider"
	"github.com/mwistrand/graft/internal/provider/claude"
	"github.com/mwistrand/graft/internal/render"
	"github.com/spf13/cobra"
)

var (
	skipSummary  bool
	skipOrdering bool
	providerName string
	modelName    string
	noDelta      bool
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
	if !skipSummary || !skipOrdering {
		Verbose("Initializing AI provider...")
		aiProvider, err = initProvider(cfg)
		if err != nil {
			fmt.Printf("Warning: %v\n", err)
			fmt.Println("Skipping AI analysis. Use --no-summary --no-order to suppress this warning.")
			fmt.Println()
			skipSummary = true
			skipOrdering = true
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

	// AI Summary
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

	// AI File Ordering
	var orderedFiles *provider.OrderResponse
	if aiProvider != nil && !skipOrdering {
		Verbose("Determining file review order...")
		fmt.Println("Determining review order...")

		orderedFiles, err = aiProvider.OrderFiles(ctx, &provider.OrderRequest{
			Files:   diffResult.Files,
			Commits: diffResult.Commits,
		})
		if err != nil {
			fmt.Printf("Warning: Failed to determine order: %v\n", err)
			fmt.Println("Using default file order.")
			fmt.Println()
		} else {
			if err := renderer.RenderOrdering(orderedFiles); err != nil {
				return fmt.Errorf("rendering ordering: %w", err)
			}
		}
	}

	// Build file list for display
	filesToReview := buildFileList(diffResult.Files, orderedFiles)

	// Display diffs
	repoDir, _ := repo.GetRootDir(ctx)

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
func initProvider(cfg *config.Config) (provider.Provider, error) {
	// Determine which provider to use
	pName := providerName
	if pName == "" {
		pName = cfg.Provider
	}

	// Determine model
	model := modelName
	if model == "" {
		model = cfg.Model
	}

	switch pName {
	case "claude", "":
		apiKey := cfg.AnthropicAPIKey
		if apiKey == "" {
			return nil, fmt.Errorf("Anthropic API key not set. Run 'graft config set anthropic-api-key <key>' or set ANTHROPIC_API_KEY")
		}
		return claude.New(apiKey, model)

	default:
		return nil, fmt.Errorf("unknown provider %q; available: claude", pName)
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
