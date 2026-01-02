package analysis

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

const (
	// CacheDir is the directory name for graft cache files.
	CacheDir = ".graft"

	// CacheFile is the filename for cached analysis.
	CacheFile = "analysis.json"
)

// Cache handles loading and saving analysis results.
type Cache struct {
	repoRoot string
}

// NewCache creates a cache manager for the given repository root.
func NewCache(repoRoot string) *Cache {
	return &Cache{repoRoot: repoRoot}
}

// CachePath returns the full path to the cache file.
func (c *Cache) CachePath() string {
	return filepath.Join(c.repoRoot, CacheDir, CacheFile)
}

// CacheDir returns the full path to the cache directory.
func (c *Cache) CacheDirectory() string {
	return filepath.Join(c.repoRoot, CacheDir)
}

// Load reads cached analysis from disk.
// Returns nil if cache doesn't exist or is invalid.
func (c *Cache) Load() (*Analysis, error) {
	data, err := os.ReadFile(c.CachePath())
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("reading cache: %w", err)
	}

	var analysis Analysis
	if err := json.Unmarshal(data, &analysis); err != nil {
		// Invalid cache, treat as missing
		return nil, nil
	}

	analysis.RepoRoot = c.repoRoot
	return &analysis, nil
}

// Save writes analysis results to disk.
func (c *Cache) Save(analysis *Analysis) error {
	// Ensure cache directory exists
	cacheDir := c.CacheDirectory()
	if err := os.MkdirAll(cacheDir, 0755); err != nil {
		return fmt.Errorf("creating cache directory: %w", err)
	}

	data, err := json.MarshalIndent(analysis, "", "  ")
	if err != nil {
		return fmt.Errorf("marshaling analysis: %w", err)
	}

	if err := os.WriteFile(c.CachePath(), data, 0644); err != nil {
		return fmt.Errorf("writing cache: %w", err)
	}

	return nil
}

// Exists returns true if a cache file exists.
func (c *Cache) Exists() bool {
	_, err := os.Stat(c.CachePath())
	return err == nil
}

// Clear removes the cached analysis.
func (c *Cache) Clear() error {
	err := os.Remove(c.CachePath())
	if os.IsNotExist(err) {
		return nil
	}
	return err
}

// GetOrAnalyze returns cached analysis if available, otherwise runs analysis.
// If forceRefresh is true, always runs fresh analysis.
func GetOrAnalyze(repoRoot string, forceRefresh bool) (*Analysis, bool, error) {
	cache := NewCache(repoRoot)

	// Check for cached analysis
	if !forceRefresh {
		if analysis, err := cache.Load(); err != nil {
			return nil, false, err
		} else if analysis != nil {
			return analysis, false, nil
		}
	}

	// Run fresh analysis
	analyzer := NewAnalyzer(repoRoot)
	analysis, err := analyzer.Analyze()
	if err != nil {
		return nil, false, fmt.Errorf("analyzing repository: %w", err)
	}

	// Cache the results
	if err := cache.Save(analysis); err != nil {
		// Non-fatal: log but continue
		fmt.Fprintf(os.Stderr, "Warning: failed to cache analysis: %v\n", err)
	}

	return analysis, true, nil
}
