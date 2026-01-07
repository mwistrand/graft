package cli

import (
	"bufio"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/mwistrand/graft/internal/git"
	"github.com/mwistrand/graft/internal/provider"
)

var (
	staleOnly bool
)

var cacheCmd = &cobra.Command{
	Use:   "cache",
	Short: "Manage the review cache",
	Long:  `Manage cached AI responses for code reviews.`,
}

var cacheClearCmd = &cobra.Command{
	Use:   "clear",
	Short: "Clear cached review responses",
	Long: `Clear cached AI responses for code reviews.

By default, clears all cached reviews after confirmation.
Use --stale to only remove entries older than one week.`,
	RunE: runCacheClear,
}

func init() {
	cacheClearCmd.Flags().BoolVar(&staleOnly, "stale", false, "Only remove cache entries older than one week")

	cacheCmd.AddCommand(cacheClearCmd)
	rootCmd.AddCommand(cacheCmd)
}

func runCacheClear(cmd *cobra.Command, args []string) error {
	// Find repository root
	repo, err := git.NewRepository("")
	if err != nil {
		if err == git.ErrNotARepository {
			return fmt.Errorf("not in a git repository")
		}
		return fmt.Errorf("opening repository: %w", err)
	}

	repoDir, err := repo.GetRootDir(cmd.Context())
	if err != nil {
		return fmt.Errorf("getting repo root: %w", err)
	}

	cache := provider.NewReviewCache(repoDir)

	// Get current cache count
	count, err := cache.Count()
	if err != nil {
		return fmt.Errorf("counting cache entries: %w", err)
	}

	if count == 0 {
		fmt.Println("No cached reviews found.")
		return nil
	}

	if staleOnly {
		return clearStaleCache(cache, count)
	}

	return clearAllCache(cache, count)
}

func clearAllCache(cache *provider.ReviewCache, count int) error {
	// Confirm with user
	fmt.Printf("This will remove %d cached review(s).\n", count)
	fmt.Print("Are you sure? [y/N] ")

	reader := bufio.NewReader(os.Stdin)
	input, err := reader.ReadString('\n')
	if err != nil {
		return fmt.Errorf("reading input: %w", err)
	}

	input = strings.TrimSpace(strings.ToLower(input))
	if input != "y" && input != "yes" {
		fmt.Println("Cancelled.")
		return nil
	}

	if err := cache.ClearAll(); err != nil {
		return fmt.Errorf("clearing cache: %w", err)
	}

	fmt.Printf("Cleared %d cached review(s).\n", count)
	return nil
}

func clearStaleCache(cache *provider.ReviewCache, totalCount int) error {
	const staleAge = 7 * 24 * time.Hour // One week

	// Count stale entries first
	reviews, err := cache.List()
	if err != nil {
		return fmt.Errorf("listing cache: %w", err)
	}

	cutoff := time.Now().Add(-staleAge)
	staleCount := 0
	for _, review := range reviews {
		if review.CachedAt.Before(cutoff) {
			staleCount++
		}
	}

	if staleCount == 0 {
		fmt.Printf("No stale cache entries found (all %d entries are less than one week old).\n", totalCount)
		return nil
	}

	// Confirm with user
	fmt.Printf("Found %d stale cache entry/entries (older than one week) out of %d total.\n", staleCount, totalCount)
	fmt.Print("Remove stale entries? [y/N] ")

	reader := bufio.NewReader(os.Stdin)
	input, err := reader.ReadString('\n')
	if err != nil {
		return fmt.Errorf("reading input: %w", err)
	}

	input = strings.TrimSpace(strings.ToLower(input))
	if input != "y" && input != "yes" {
		fmt.Println("Cancelled.")
		return nil
	}

	cleared, err := cache.ClearStale(staleAge)
	if err != nil {
		return fmt.Errorf("clearing stale cache: %w", err)
	}

	fmt.Printf("Cleared %d stale cached review(s).\n", cleared)
	return nil
}
