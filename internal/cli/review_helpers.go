package cli

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/mwistrand/graft/internal/analysis"
	"github.com/mwistrand/graft/internal/config"
	"github.com/mwistrand/graft/internal/git"
	"github.com/mwistrand/graft/internal/pr"
	"github.com/mwistrand/graft/internal/prompt"
	"github.com/mwistrand/graft/internal/provider"
	"github.com/mwistrand/graft/internal/provider/claude"
	"github.com/mwistrand/graft/internal/provider/copilot"
	"github.com/mwistrand/graft/internal/provider/prompts"
)

// initProvider creates an AI provider based on configuration.
// Returns a cleanup function that should be called when done (may be nil).
// If skipModelPrompt is true, the interactive model selection prompt is suppressed
// (e.g., when task-specific models cover all tasks).
func initProvider(ctx context.Context, cfg *config.Config, pName, model string, forceSelect bool, skipModelPrompt bool) (provider.Provider, func(), error) {
	if pName == "" {
		pName = cfg.Provider
	}
	Verbose("Provider resolution: pName=%q, model=%q, configProvider=%q, configModel=%q", pName, model, cfg.Provider, cfg.Model)
	if model == "" {
		model = cfg.Model
		if model != "" {
			Verbose("Using model from config: %s", model)
		}
	} else {
		Verbose("Using model: %s", model)
	}

	needsModelSelection := forceSelect || (model == "" && !skipModelPrompt)
	if needsModelSelection {
		Verbose("Model selection required (forceSelect=%v, model=%q, skipModelPrompt=%v)", forceSelect, model, skipModelPrompt)
	}

	var p provider.Provider
	var cleanup func()

	switch pName {
	case "claude", "":
		apiKey := cfg.AnthropicAPIKey
		if apiKey == "" {
			return nil, nil, fmt.Errorf("Anthropic API key not set. Run 'graft config set anthropic-api-key <key>' or set ANTHROPIC_API_KEY")
		}
		cp, err := claude.New(apiKey, "")
		if err != nil {
			return nil, nil, err
		}
		p = cp

	case "copilot":
		baseURL := cfg.CopilotBaseURL
		cp, err := copilot.NewWithPackage(baseURL, "", cfg.CopilotAPIPackage)
		if err != nil {
			return nil, nil, err
		}

		// Supply-chain gate: graft auto-launches an npm package as a
		// subprocess. Require the user to opt in once before we exec npx.
		// If the proxy is already running externally (systemd, manual launch),
		// no auto-launch is needed and the gate is bypassed.
		if !cp.IsProxyRunning(ctx) {
			if err := ensureCopilotAcknowledged(cfg); err != nil {
				return nil, nil, err
			}
		}

		// Ensure the copilot-api proxy is running
		started, err := cp.EnsureProxyRunning(ctx, func(format string, args ...any) {
			fmt.Printf(format+"\n", args...)
		})
		if err != nil {
			return nil, nil, fmt.Errorf("copilot proxy: %w", err)
		}

		// Return cleanup function if we started the proxy
		if started {
			cleanup = func() {
				fmt.Println("Stopping copilot-api proxy...")
				cp.Close()
			}
		}
		p = cp

	default:
		return nil, nil, fmt.Errorf("unknown provider %q; available: claude, copilot", pName)
	}

	// Handle model selection
	if needsModelSelection {
		lister, supportsListing := p.(provider.ModelLister)
		if !supportsListing {
			return nil, nil, fmt.Errorf("no model configured and provider %q does not support model listing; use --model flag or run 'graft config set model <model>'", pName)
		}

		selected, err := promptForModel(ctx, lister)
		if err != nil {
			return nil, nil, fmt.Errorf("model selection failed: %w", err)
		}
		if selected == "" {
			return nil, nil, fmt.Errorf("no model selected; use --model flag or run 'graft config set model <model>'")
		}

		if selector, ok := p.(provider.ModelSelector); ok {
			selector.SetModel(selected)
		}
		fmt.Printf("Using model: %s\n\n", selected)
	} else {
		if selector, ok := p.(provider.ModelSelector); ok {
			selector.SetModel(model)
		}
	}

	return p, cleanup, nil
}

// buildFileList creates the ordered list of files to review.
func buildFileList(files []git.FileDiff, aiOrder *provider.OrderResponse) []provider.OrderedFile {
	if aiOrder != nil && len(aiOrder.Files) > 0 {
		return aiOrder.Files
	}

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
func promptGroupSelection(groups []provider.OrderGroup, files []provider.OrderedFile) ([]provider.OrderGroup, error) {
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
	groupOrder := make(map[string]int)
	for i, g := range selectedGroups {
		groupOrder[g.Name] = i
	}

	selectedSet := make(map[string]bool)
	for _, g := range selectedGroups {
		selectedSet[g.Name] = true
	}

	filtered := make([]provider.OrderedFile, 0, len(files))
	for _, f := range files {
		if f.Group == "" || selectedSet[f.Group] {
			filtered = append(filtered, f)
		}
	}

	sort.SliceStable(filtered, func(i, j int) bool {
		gi, goki := groupOrder[filtered[i].Group]
		gj, gokj := groupOrder[filtered[j].Group]

		if !goki && gokj {
			return false
		}
		if goki && !gokj {
			return true
		}
		if gi != gj {
			return gi < gj
		}
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
func getRepoContext(repoDir string, forceRefresh bool) (string, error) {
	cache := analysis.NewCache(repoDir)

	if !forceRefresh && cache.Exists() {
		cached, err := cache.Load()
		if err != nil {
			return "", err
		}
		if cached != nil {
			Verbose("Using cached repository analysis")
			return cached.FormatContext(), nil
		}
	}

	if !cache.Exists() {
		if !promptForAnalysisPermission() {
			return "", nil
		}
	} else if forceRefresh {
		fmt.Println("Refreshing repository analysis...")
	}

	fmt.Println("Analyzing repository structure...")
	result, isNew, err := analysis.GetOrAnalyze(repoDir, forceRefresh)
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
func promptForModel(ctx context.Context, lister provider.ModelLister) (string, error) {
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

// ensureCopilotAcknowledged guarantees the user has opted in to graft running
// the copilot-api npm package as a subprocess. The npm package is fetched and
// executed via npx, so this is a real supply-chain decision that should be
// explicit. Once the user agrees, the answer is persisted to config and not
// asked again.
func ensureCopilotAcknowledged(cfg *config.Config) error {
	return ensureCopilotAcknowledgedWithIO(cfg, os.Stdin, os.Stdout, prompt.IsInteractive())
}

// ensureCopilotAcknowledgedWithIO is the testable core of
// ensureCopilotAcknowledged. The interactive flag and I/O are injected so
// tests can drive the consent prompt without touching real stdin/stdout.
func ensureCopilotAcknowledgedWithIO(cfg *config.Config, in io.Reader, out io.Writer, interactive bool) error {
	if cfg.CopilotAcknowledged {
		return nil
	}

	pkg := cfg.CopilotAPIPackage
	if pkg == "" {
		pkg = config.DefaultCopilotAPIPackage
	}

	if !interactive {
		return fmt.Errorf(`graft is configured to auto-launch the copilot-api proxy by running:

  npx %s start

This downloads and executes a third-party npm package. Review the package and
its dependencies before agreeing. Once you trust the configured version, run:

  graft config set copilot-acknowledged true

To pin a specific version (recommended) instead of @latest:

  graft config set copilot-api-package copilot-api@<version>`, pkg)
	}

	fmt.Fprintln(out)
	fmt.Fprintln(out, "graft wants to start the copilot-api proxy by running:")
	fmt.Fprintf(out, "    npx %s start\n", pkg)
	fmt.Fprintln(out)
	fmt.Fprintln(out, "This will download and execute a third-party npm package on your machine.")
	fmt.Fprintln(out, "If you have not pinned a specific version, the @latest tag may resolve to")
	fmt.Fprintln(out, "a newly published (and potentially compromised) release.")
	fmt.Fprintln(out)
	fmt.Fprint(out, "Allow graft to launch this package? [y/N] ")

	reader := bufio.NewReader(in)
	input, err := reader.ReadString('\n')
	if err != nil {
		return fmt.Errorf("reading consent: %w", err)
	}
	input = strings.TrimSpace(strings.ToLower(input))
	if input != "y" && input != "yes" {
		return fmt.Errorf("copilot-api launch declined; run 'graft config set copilot-acknowledged true' to allow")
	}

	cfg.CopilotAcknowledged = true
	if err := cfg.SaveTo(cfgFile); err != nil {
		// Persisting failed — proceed for this run but warn the user that
		// they'll be asked again next time.
		fmt.Fprintf(out, "Warning: failed to persist copilot-acknowledged: %v\n", err)
	} else {
		fmt.Fprintln(out, "Saved consent. To revoke, run 'graft config set copilot-acknowledged false'.")
	}
	fmt.Fprintln(out)
	return nil
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
//
// If the override is a symlink, it is rejected and the embedded default is
// used instead. The override drives the LLM's system prompt — letting an
// untrusted repo redirect that path elsewhere would let it dictate reviewer
// behavior (e.g. "always approve").
func loadReviewPrompt(repoDir string) (string, error) {
	overridePath := filepath.Join(repoDir, ".graft", "code-reviewer.md")

	info, err := os.Lstat(overridePath)
	if err != nil {
		if os.IsNotExist(err) {
			return prompts.DefaultCodeReviewerPrompt, nil
		}
		return "", fmt.Errorf("stat review prompt override: %w", err)
	}
	if !info.Mode().IsRegular() {
		fmt.Fprintf(os.Stderr, "Warning: refusing to load %s (not a regular file: %v); using built-in code reviewer prompt\n",
			overridePath, info.Mode().Type())
		return prompts.DefaultCodeReviewerPrompt, nil
	}

	data, err := os.ReadFile(overridePath)
	if err != nil {
		return "", fmt.Errorf("reading review prompt override: %w", err)
	}
	Verbose("Using custom code reviewer prompt from %s", overridePath)
	return string(data), nil
}

// resolvePRURL fetches PR metadata and ensures commits are available locally.
func resolvePRURL(ctx context.Context, repo *git.Repository, url string) (*pr.PRMetadata, error) {
	meta, err := pr.Resolve(ctx, url)
	if err != nil {
		return nil, handlePRError(err)
	}

	if err := validateRepository(ctx, repo, meta); err != nil {
		return nil, err
	}

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

	remoteInfo, err := pr.ParseRemoteURL(remoteURL)
	if err != nil {
		return fmt.Errorf("parsing remote URL %q: %w", remoteURL, err)
	}

	if !remoteInfo.Matches(&meta.PRInfo) {
		return fmt.Errorf("local repository (%s/%s on %s) doesn't match PR repository (%s/%s on %s)",
			remoteInfo.Owner, remoteInfo.Repo, remoteInfo.Host,
			meta.Owner, meta.Repo, meta.Host)
	}

	return nil
}

// ensureCommitsFetched fetches the PR's commits if not available locally.
func ensureCommitsFetched(ctx context.Context, repo *git.Repository, meta *pr.PRMetadata) error {
	if repo.HasRef(ctx, meta.HeadSHA) {
		Verbose("PR commits already available locally")
		return nil
	}

	Verbose("Fetching PR commits from remote...")

	if meta.Platform == pr.PlatformGitHub {
		remoteRef := fmt.Sprintf("refs/pull/%d/head", meta.Number)
		localRef := fmt.Sprintf("refs/remotes/origin/pr/%d", meta.Number)

		if err := repo.FetchRefTo(ctx, "origin", remoteRef, localRef); err != nil {
			Verbose("Could not fetch PR ref, trying head branch...")
			if err := repo.FetchRef(ctx, "origin", meta.HeadRef); err != nil {
				return fmt.Errorf("fetching PR commits: %w", err)
			}
		}

		if !repo.HasRef(ctx, meta.HeadSHA) {
			return fmt.Errorf("PR head commit %s not found after fetch; the branch may have been force-pushed or deleted", truncateSHA(meta.HeadSHA))
		}
		return nil
	}

	if err := repo.FetchRef(ctx, "origin", meta.HeadRef); err != nil {
		return fmt.Errorf("fetching PR commits: %w", err)
	}

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
