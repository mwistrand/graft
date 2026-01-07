package provider

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"time"

	"github.com/mwistrand/graft/internal/git"
)

const (
	// CacheDir is the directory name for graft cache files.
	CacheDir = ".graft"

	// ReviewCacheDir is the subdirectory for review caches.
	ReviewCacheDir = "reviews"
)

// CachedReview contains cached AI responses for a review.
type CachedReview struct {
	// CacheKey is the unique identifier for this review.
	CacheKey string `json:"cache_key"`

	// BaseRef is the base reference used for the review.
	BaseRef string `json:"base_ref"`

	// CommitHashes are the commit hashes that were reviewed.
	CommitHashes []string `json:"commit_hashes"`

	// Summary contains the cached summarization response.
	Summary *SummarizeResponse `json:"summary,omitempty"`

	// Ordering contains the cached ordering response.
	Ordering *OrderResponse `json:"ordering,omitempty"`

	// CachedAt is when this cache entry was created.
	CachedAt time.Time `json:"cached_at"`
}

// ReviewCache handles loading and saving AI review responses.
type ReviewCache struct {
	repoRoot string
}

// NewReviewCache creates a cache manager for the given repository root.
func NewReviewCache(repoRoot string) *ReviewCache {
	return &ReviewCache{repoRoot: repoRoot}
}

// GenerateCacheKey creates a deterministic cache key from commits.
// The key is based on the sorted commit hashes to ensure consistency.
func GenerateCacheKey(baseRef string, commits []git.Commit) string {
	// Extract and sort commit hashes for deterministic ordering
	hashes := make([]string, len(commits))
	for i, c := range commits {
		hashes[i] = c.Hash
	}
	sort.Strings(hashes)

	// Create a hash of baseRef + sorted commit hashes
	h := sha256.New()
	h.Write([]byte(baseRef))
	h.Write([]byte{0}) // separator
	for _, hash := range hashes {
		h.Write([]byte(hash))
		h.Write([]byte{0}) // separator
	}

	// Return first 16 chars of hex-encoded hash (64 bits)
	return hex.EncodeToString(h.Sum(nil))[:16]
}

// CacheDirectory returns the full path to the review cache directory.
func (c *ReviewCache) CacheDirectory() string {
	return filepath.Join(c.repoRoot, CacheDir, ReviewCacheDir)
}

// CachePath returns the full path to a specific cache file.
func (c *ReviewCache) CachePath(cacheKey string) string {
	return filepath.Join(c.CacheDirectory(), cacheKey+".json")
}

// Load reads a cached review from disk.
// Returns nil if cache doesn't exist or is invalid.
func (c *ReviewCache) Load(cacheKey string) (*CachedReview, error) {
	data, err := os.ReadFile(c.CachePath(cacheKey))
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("reading review cache: %w", err)
	}

	var cached CachedReview
	if err := json.Unmarshal(data, &cached); err != nil {
		// Invalid cache, treat as missing
		return nil, nil
	}

	return &cached, nil
}

// Save writes a cached review to disk.
func (c *ReviewCache) Save(cached *CachedReview) error {
	// Ensure cache directory exists
	cacheDir := c.CacheDirectory()
	if err := os.MkdirAll(cacheDir, 0755); err != nil {
		return fmt.Errorf("creating review cache directory: %w", err)
	}

	data, err := json.MarshalIndent(cached, "", "  ")
	if err != nil {
		return fmt.Errorf("marshaling review cache: %w", err)
	}

	if err := os.WriteFile(c.CachePath(cached.CacheKey), data, 0644); err != nil {
		return fmt.Errorf("writing review cache: %w", err)
	}

	return nil
}

// Exists returns true if a cache file exists for the given key.
func (c *ReviewCache) Exists(cacheKey string) bool {
	_, err := os.Stat(c.CachePath(cacheKey))
	return err == nil
}

// Clear removes the cached review for the given key.
func (c *ReviewCache) Clear(cacheKey string) error {
	err := os.Remove(c.CachePath(cacheKey))
	if os.IsNotExist(err) {
		return nil
	}
	return err
}

// ClearAll removes all cached reviews.
func (c *ReviewCache) ClearAll() error {
	err := os.RemoveAll(c.CacheDirectory())
	if os.IsNotExist(err) {
		return nil
	}
	return err
}

// List returns all cached reviews.
func (c *ReviewCache) List() ([]*CachedReview, error) {
	cacheDir := c.CacheDirectory()
	entries, err := os.ReadDir(cacheDir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("reading cache directory: %w", err)
	}

	var reviews []*CachedReview
	for _, entry := range entries {
		if entry.IsDir() || filepath.Ext(entry.Name()) != ".json" {
			continue
		}

		// Extract cache key from filename (remove .json extension)
		cacheKey := entry.Name()[:len(entry.Name())-5]

		review, err := c.Load(cacheKey)
		if err != nil {
			continue // Skip invalid entries
		}
		if review != nil {
			reviews = append(reviews, review)
		}
	}

	return reviews, nil
}

// ClearStale removes cached reviews older than the specified duration.
// Returns the number of entries cleared.
func (c *ReviewCache) ClearStale(maxAge time.Duration) (int, error) {
	reviews, err := c.List()
	if err != nil {
		return 0, err
	}

	cutoff := time.Now().Add(-maxAge)
	cleared := 0

	for _, review := range reviews {
		if review.CachedAt.Before(cutoff) {
			if err := c.Clear(review.CacheKey); err != nil {
				return cleared, fmt.Errorf("clearing %s: %w", review.CacheKey, err)
			}
			cleared++
		}
	}

	return cleared, nil
}

// Count returns the number of cached reviews.
func (c *ReviewCache) Count() (int, error) {
	reviews, err := c.List()
	if err != nil {
		return 0, err
	}
	return len(reviews), nil
}
