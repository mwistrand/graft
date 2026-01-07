package provider

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/mwistrand/graft/internal/git"
)

func TestGenerateCacheKey(t *testing.T) {
	commits := []git.Commit{
		{Hash: "abc123def456"},
		{Hash: "789xyz000111"},
	}

	key1 := GenerateCacheKey("main", commits)

	// Key should be 16 hex characters
	if len(key1) != 16 {
		t.Errorf("expected key length 16, got %d", len(key1))
	}

	// Same input should produce same key
	key2 := GenerateCacheKey("main", commits)
	if key1 != key2 {
		t.Errorf("expected same key for same input, got %q and %q", key1, key2)
	}

	// Different order should produce same key (sorted internally)
	commitsReversed := []git.Commit{
		{Hash: "789xyz000111"},
		{Hash: "abc123def456"},
	}
	key3 := GenerateCacheKey("main", commitsReversed)
	if key1 != key3 {
		t.Errorf("expected same key regardless of commit order, got %q and %q", key1, key3)
	}

	// Different base ref should produce different key
	key4 := GenerateCacheKey("develop", commits)
	if key1 == key4 {
		t.Errorf("expected different key for different base ref")
	}

	// Different commits should produce different key
	differentCommits := []git.Commit{
		{Hash: "completely_different"},
	}
	key5 := GenerateCacheKey("main", differentCommits)
	if key1 == key5 {
		t.Errorf("expected different key for different commits")
	}
}

func TestGenerateCacheKey_EmptyCommits(t *testing.T) {
	key := GenerateCacheKey("main", []git.Commit{})

	// Should still produce a valid key
	if len(key) != 16 {
		t.Errorf("expected key length 16, got %d", len(key))
	}
}

func TestReviewCache_SaveAndLoad(t *testing.T) {
	// Create temp directory
	tmpDir := t.TempDir()

	cache := NewReviewCache(tmpDir)
	cacheKey := "test123456789abc"

	// Create test data
	original := &CachedReview{
		CacheKey:     cacheKey,
		BaseRef:      "main",
		CommitHashes: []string{"abc123", "def456"},
		Summary: &SummarizeResponse{
			Overview:   "Test overview",
			KeyChanges: []string{"Change 1", "Change 2"},
		},
		Ordering: &OrderResponse{
			Files: []OrderedFile{
				{Path: "main.go", Category: CategoryEntryPoint, Priority: 1},
			},
			Groups: []OrderGroup{
				{Name: "Feature A", Description: "Test feature", Priority: 1},
			},
			Reasoning: "Test reasoning",
		},
		CachedAt: time.Now(),
	}

	// Save
	if err := cache.Save(original); err != nil {
		t.Fatalf("Save() failed: %v", err)
	}

	// Verify file exists
	if !cache.Exists(cacheKey) {
		t.Error("Exists() returned false after Save()")
	}

	// Load
	loaded, err := cache.Load(cacheKey)
	if err != nil {
		t.Fatalf("Load() failed: %v", err)
	}

	if loaded == nil {
		t.Fatal("Load() returned nil")
	}

	// Verify data
	if loaded.CacheKey != original.CacheKey {
		t.Errorf("CacheKey = %q, want %q", loaded.CacheKey, original.CacheKey)
	}
	if loaded.BaseRef != original.BaseRef {
		t.Errorf("BaseRef = %q, want %q", loaded.BaseRef, original.BaseRef)
	}
	if len(loaded.CommitHashes) != len(original.CommitHashes) {
		t.Errorf("CommitHashes length = %d, want %d", len(loaded.CommitHashes), len(original.CommitHashes))
	}
	if loaded.Summary.Overview != original.Summary.Overview {
		t.Errorf("Summary.Overview = %q, want %q", loaded.Summary.Overview, original.Summary.Overview)
	}
	if len(loaded.Ordering.Files) != len(original.Ordering.Files) {
		t.Errorf("Ordering.Files length = %d, want %d", len(loaded.Ordering.Files), len(original.Ordering.Files))
	}
	if len(loaded.Ordering.Groups) != len(original.Ordering.Groups) {
		t.Errorf("Ordering.Groups length = %d, want %d", len(loaded.Ordering.Groups), len(original.Ordering.Groups))
	}
}

func TestReviewCache_LoadMissing(t *testing.T) {
	tmpDir := t.TempDir()
	cache := NewReviewCache(tmpDir)

	// Load non-existent cache
	loaded, err := cache.Load("nonexistent")
	if err != nil {
		t.Fatalf("Load() failed: %v", err)
	}
	if loaded != nil {
		t.Error("Load() should return nil for missing cache")
	}
}

func TestReviewCache_LoadInvalidJSON(t *testing.T) {
	tmpDir := t.TempDir()
	cache := NewReviewCache(tmpDir)

	// Create directory and invalid JSON file
	cacheDir := cache.CacheDirectory()
	if err := os.MkdirAll(cacheDir, 0755); err != nil {
		t.Fatal(err)
	}

	invalidPath := cache.CachePath("invalid")
	if err := os.WriteFile(invalidPath, []byte("not valid json"), 0644); err != nil {
		t.Fatal(err)
	}

	// Should return nil, nil (treat as missing)
	loaded, err := cache.Load("invalid")
	if err != nil {
		t.Fatalf("Load() failed: %v", err)
	}
	if loaded != nil {
		t.Error("Load() should return nil for invalid JSON")
	}
}

func TestReviewCache_Exists(t *testing.T) {
	tmpDir := t.TempDir()
	cache := NewReviewCache(tmpDir)

	// Non-existent
	if cache.Exists("missing") {
		t.Error("Exists() should return false for missing cache")
	}

	// Create a cache entry
	review := &CachedReview{
		CacheKey: "exists",
		BaseRef:  "main",
	}
	if err := cache.Save(review); err != nil {
		t.Fatal(err)
	}

	// Should exist now
	if !cache.Exists("exists") {
		t.Error("Exists() should return true after Save()")
	}
}

func TestReviewCache_Clear(t *testing.T) {
	tmpDir := t.TempDir()
	cache := NewReviewCache(tmpDir)

	// Create a cache entry
	review := &CachedReview{
		CacheKey: "toclear",
		BaseRef:  "main",
	}
	if err := cache.Save(review); err != nil {
		t.Fatal(err)
	}

	if !cache.Exists("toclear") {
		t.Fatal("Cache should exist before Clear()")
	}

	// Clear
	if err := cache.Clear("toclear"); err != nil {
		t.Fatalf("Clear() failed: %v", err)
	}

	if cache.Exists("toclear") {
		t.Error("Cache should not exist after Clear()")
	}

	// Clear again should not error
	if err := cache.Clear("toclear"); err != nil {
		t.Errorf("Clear() on missing cache should not error: %v", err)
	}
}

func TestReviewCache_ClearAll(t *testing.T) {
	tmpDir := t.TempDir()
	cache := NewReviewCache(tmpDir)

	// Create multiple cache entries
	for i := 0; i < 3; i++ {
		review := &CachedReview{
			CacheKey: "entry" + string(rune('a'+i)),
			BaseRef:  "main",
		}
		if err := cache.Save(review); err != nil {
			t.Fatal(err)
		}
	}

	// Clear all
	if err := cache.ClearAll(); err != nil {
		t.Fatalf("ClearAll() failed: %v", err)
	}

	// Verify directory is gone
	if _, err := os.Stat(cache.CacheDirectory()); !os.IsNotExist(err) {
		t.Error("Cache directory should be removed after ClearAll()")
	}

	// ClearAll again should not error
	if err := cache.ClearAll(); err != nil {
		t.Errorf("ClearAll() on missing directory should not error: %v", err)
	}
}

func TestReviewCache_CachePath(t *testing.T) {
	cache := NewReviewCache("/repo")

	path := cache.CachePath("abc123")
	expected := filepath.Join("/repo", ".graft", "reviews", "abc123.json")
	if path != expected {
		t.Errorf("CachePath() = %q, want %q", path, expected)
	}
}

func TestReviewCache_CacheDirectory(t *testing.T) {
	cache := NewReviewCache("/repo")

	dir := cache.CacheDirectory()
	expected := filepath.Join("/repo", ".graft", "reviews")
	if dir != expected {
		t.Errorf("CacheDirectory() = %q, want %q", dir, expected)
	}
}

func TestReviewCache_PartialCache(t *testing.T) {
	tmpDir := t.TempDir()
	cache := NewReviewCache(tmpDir)

	// Save with only summary
	review := &CachedReview{
		CacheKey: "partial",
		BaseRef:  "main",
		Summary: &SummarizeResponse{
			Overview: "Test",
		},
		// Ordering is nil
	}
	if err := cache.Save(review); err != nil {
		t.Fatal(err)
	}

	loaded, err := cache.Load("partial")
	if err != nil {
		t.Fatal(err)
	}

	if loaded.Summary == nil {
		t.Error("Summary should be loaded")
	}
	if loaded.Ordering != nil {
		t.Error("Ordering should be nil")
	}
}

func TestReviewCache_List(t *testing.T) {
	tmpDir := t.TempDir()
	cache := NewReviewCache(tmpDir)

	// Empty directory should return empty list
	reviews, err := cache.List()
	if err != nil {
		t.Fatalf("List() failed: %v", err)
	}
	if len(reviews) != 0 {
		t.Errorf("expected empty list, got %d items", len(reviews))
	}

	// Create multiple entries
	for i := 0; i < 3; i++ {
		review := &CachedReview{
			CacheKey: fmt.Sprintf("entry%d", i),
			BaseRef:  "main",
			CachedAt: time.Now(),
		}
		if err := cache.Save(review); err != nil {
			t.Fatal(err)
		}
	}

	// Should list all 3
	reviews, err = cache.List()
	if err != nil {
		t.Fatalf("List() failed: %v", err)
	}
	if len(reviews) != 3 {
		t.Errorf("expected 3 items, got %d", len(reviews))
	}
}

func TestReviewCache_List_SkipsInvalidJSON(t *testing.T) {
	tmpDir := t.TempDir()
	cache := NewReviewCache(tmpDir)

	// Create cache directory
	cacheDir := cache.CacheDirectory()
	if err := os.MkdirAll(cacheDir, 0755); err != nil {
		t.Fatal(err)
	}

	// Create a valid entry
	validReview := &CachedReview{
		CacheKey: "valid",
		BaseRef:  "main",
		CachedAt: time.Now(),
	}
	if err := cache.Save(validReview); err != nil {
		t.Fatal(err)
	}

	// Create an invalid JSON file
	invalidPath := filepath.Join(cacheDir, "invalid.json")
	if err := os.WriteFile(invalidPath, []byte("not valid json"), 0644); err != nil {
		t.Fatal(err)
	}

	// List should skip invalid and return only valid
	reviews, err := cache.List()
	if err != nil {
		t.Fatalf("List() failed: %v", err)
	}
	if len(reviews) != 1 {
		t.Errorf("expected 1 item, got %d", len(reviews))
	}
	if reviews[0].CacheKey != "valid" {
		t.Errorf("expected 'valid' entry, got %q", reviews[0].CacheKey)
	}
}

func TestReviewCache_Count(t *testing.T) {
	tmpDir := t.TempDir()
	cache := NewReviewCache(tmpDir)

	// Empty should be 0
	count, err := cache.Count()
	if err != nil {
		t.Fatalf("Count() failed: %v", err)
	}
	if count != 0 {
		t.Errorf("expected 0, got %d", count)
	}

	// Add entries
	for i := 0; i < 5; i++ {
		review := &CachedReview{
			CacheKey: fmt.Sprintf("entry%d", i),
			BaseRef:  "main",
		}
		if err := cache.Save(review); err != nil {
			t.Fatal(err)
		}
	}

	count, err = cache.Count()
	if err != nil {
		t.Fatalf("Count() failed: %v", err)
	}
	if count != 5 {
		t.Errorf("expected 5, got %d", count)
	}
}

func TestReviewCache_ClearStale(t *testing.T) {
	tmpDir := t.TempDir()
	cache := NewReviewCache(tmpDir)

	now := time.Now()
	staleAge := 7 * 24 * time.Hour

	// Create fresh entries (within the last week)
	for i := 0; i < 2; i++ {
		review := &CachedReview{
			CacheKey: fmt.Sprintf("fresh%d", i),
			BaseRef:  "main",
			CachedAt: now.Add(-time.Hour * time.Duration(i+1)), // 1-2 hours ago
		}
		if err := cache.Save(review); err != nil {
			t.Fatal(err)
		}
	}

	// Create stale entries (older than a week)
	for i := 0; i < 3; i++ {
		review := &CachedReview{
			CacheKey: fmt.Sprintf("stale%d", i),
			BaseRef:  "main",
			CachedAt: now.Add(-staleAge - time.Hour*time.Duration(i+1)), // More than a week ago
		}
		if err := cache.Save(review); err != nil {
			t.Fatal(err)
		}
	}

	// Verify we have 5 total
	count, _ := cache.Count()
	if count != 5 {
		t.Fatalf("expected 5 entries, got %d", count)
	}

	// Clear stale entries
	cleared, err := cache.ClearStale(staleAge)
	if err != nil {
		t.Fatalf("ClearStale() failed: %v", err)
	}
	if cleared != 3 {
		t.Errorf("expected 3 cleared, got %d", cleared)
	}

	// Verify only fresh remain
	count, _ = cache.Count()
	if count != 2 {
		t.Errorf("expected 2 remaining, got %d", count)
	}

	// Verify the right entries remain
	for i := 0; i < 2; i++ {
		if !cache.Exists(fmt.Sprintf("fresh%d", i)) {
			t.Errorf("fresh%d should still exist", i)
		}
	}
	for i := 0; i < 3; i++ {
		if cache.Exists(fmt.Sprintf("stale%d", i)) {
			t.Errorf("stale%d should be removed", i)
		}
	}
}

func TestReviewCache_ClearStale_NoStaleEntries(t *testing.T) {
	tmpDir := t.TempDir()
	cache := NewReviewCache(tmpDir)

	// Create only fresh entries
	for i := 0; i < 3; i++ {
		review := &CachedReview{
			CacheKey: fmt.Sprintf("fresh%d", i),
			BaseRef:  "main",
			CachedAt: time.Now(),
		}
		if err := cache.Save(review); err != nil {
			t.Fatal(err)
		}
	}

	// Clear stale should remove nothing
	cleared, err := cache.ClearStale(7 * 24 * time.Hour)
	if err != nil {
		t.Fatalf("ClearStale() failed: %v", err)
	}
	if cleared != 0 {
		t.Errorf("expected 0 cleared, got %d", cleared)
	}

	// All entries should remain
	count, _ := cache.Count()
	if count != 3 {
		t.Errorf("expected 3 remaining, got %d", count)
	}
}

func TestReviewCache_WithReview(t *testing.T) {
	tmpDir := t.TempDir()
	cache := NewReviewCache(tmpDir)
	cacheKey := "review123456789"

	// Create test data with all fields including Review
	original := &CachedReview{
		CacheKey:     cacheKey,
		BaseRef:      "main",
		CommitHashes: []string{"abc123", "def456"},
		Summary: &SummarizeResponse{
			Overview:   "Test overview",
			KeyChanges: []string{"Change 1", "Change 2"},
		},
		Ordering: &OrderResponse{
			Files: []OrderedFile{
				{Path: "main.go", Category: CategoryEntryPoint, Priority: 1},
			},
			Reasoning: "Test reasoning",
		},
		Review: &ReviewResponse{
			Content: "# Code Review\n\nThis is a test review.",
		},
		CachedAt: time.Now(),
	}

	// Save
	if err := cache.Save(original); err != nil {
		t.Fatalf("Save() failed: %v", err)
	}

	// Load
	loaded, err := cache.Load(cacheKey)
	if err != nil {
		t.Fatalf("Load() failed: %v", err)
	}

	if loaded == nil {
		t.Fatal("Load() returned nil")
	}

	// Verify Review data
	if loaded.Review == nil {
		t.Fatal("Review should be loaded")
	}
	if loaded.Review.Content != original.Review.Content {
		t.Errorf("Review.Content = %q, want %q", loaded.Review.Content, original.Review.Content)
	}
}

func TestReviewCache_PartialCache_NoReview(t *testing.T) {
	tmpDir := t.TempDir()
	cache := NewReviewCache(tmpDir)

	// Save with only summary and ordering (no review)
	review := &CachedReview{
		CacheKey: "noreview",
		BaseRef:  "main",
		Summary: &SummarizeResponse{
			Overview: "Test",
		},
		Ordering: &OrderResponse{
			Reasoning: "Test reasoning",
		},
		// Review is nil
	}
	if err := cache.Save(review); err != nil {
		t.Fatal(err)
	}

	loaded, err := cache.Load("noreview")
	if err != nil {
		t.Fatal(err)
	}

	if loaded.Summary == nil {
		t.Error("Summary should be loaded")
	}
	if loaded.Ordering == nil {
		t.Error("Ordering should be loaded")
	}
	if loaded.Review != nil {
		t.Error("Review should be nil")
	}
}
