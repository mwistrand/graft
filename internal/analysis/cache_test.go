package analysis

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestCache_SaveAndLoad(t *testing.T) {
	dir := t.TempDir()
	cache := NewCache(dir)

	analysis := &Analysis{
		Type:       ProjectTypeBackend,
		Languages:  []string{"Go"},
		Frameworks: []string{},
		Directories: []DirectorySummary{
			{Path: "cmd", FileCount: 1, Description: "Entry points"},
		},
		AnalyzedAt: time.Now(),
	}

	// Save
	if err := cache.Save(analysis); err != nil {
		t.Fatalf("Save() failed: %v", err)
	}

	// Verify file exists
	if !cache.Exists() {
		t.Error("Exists() = false after Save()")
	}

	// Load
	loaded, err := cache.Load()
	if err != nil {
		t.Fatalf("Load() failed: %v", err)
	}

	if loaded == nil {
		t.Fatal("Load() returned nil")
	}

	if loaded.Type != analysis.Type {
		t.Errorf("Type = %q, want %q", loaded.Type, analysis.Type)
	}

	if len(loaded.Languages) != 1 || loaded.Languages[0] != "Go" {
		t.Errorf("Languages = %v, want [Go]", loaded.Languages)
	}

	if len(loaded.Directories) != 1 {
		t.Errorf("Directories count = %d, want 1", len(loaded.Directories))
	}
}

func TestCache_LoadNonExistent(t *testing.T) {
	dir := t.TempDir()
	cache := NewCache(dir)

	loaded, err := cache.Load()
	if err != nil {
		t.Fatalf("Load() failed: %v", err)
	}

	if loaded != nil {
		t.Error("Load() should return nil for non-existent cache")
	}
}

func TestCache_Clear(t *testing.T) {
	dir := t.TempDir()
	cache := NewCache(dir)

	// Create cache
	analysis := &Analysis{Type: ProjectTypeBackend}
	if err := cache.Save(analysis); err != nil {
		t.Fatal(err)
	}

	if !cache.Exists() {
		t.Fatal("Cache should exist after Save()")
	}

	// Clear
	if err := cache.Clear(); err != nil {
		t.Fatalf("Clear() failed: %v", err)
	}

	if cache.Exists() {
		t.Error("Cache should not exist after Clear()")
	}
}

func TestCache_ClearNonExistent(t *testing.T) {
	dir := t.TempDir()
	cache := NewCache(dir)

	// Clear non-existent cache should not error
	if err := cache.Clear(); err != nil {
		t.Fatalf("Clear() failed: %v", err)
	}
}

func TestCache_CachePath(t *testing.T) {
	cache := NewCache("/some/repo")
	expected := filepath.Join("/some/repo", CacheDir, CacheFile)

	if cache.CachePath() != expected {
		t.Errorf("CachePath() = %q, want %q", cache.CachePath(), expected)
	}
}

func TestCache_LoadInvalidJSON(t *testing.T) {
	dir := t.TempDir()
	cache := NewCache(dir)

	// Create cache directory and file with invalid JSON
	cacheDir := filepath.Join(dir, CacheDir)
	if err := os.MkdirAll(cacheDir, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(cache.CachePath(), []byte("invalid json"), 0644); err != nil {
		t.Fatal(err)
	}

	// Should return nil (treat as missing) not error
	loaded, err := cache.Load()
	if err != nil {
		t.Fatalf("Load() with invalid JSON should not error: %v", err)
	}

	if loaded != nil {
		t.Error("Load() with invalid JSON should return nil")
	}
}

func TestGetOrAnalyze(t *testing.T) {
	dir := t.TempDir()

	// Create a simple Go project
	if err := os.WriteFile(filepath.Join(dir, "go.mod"), []byte("module test"), 0644); err != nil {
		t.Fatal(err)
	}
	cmdDir := filepath.Join(dir, "cmd")
	if err := os.MkdirAll(cmdDir, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(cmdDir, "main.go"), []byte("package main"), 0644); err != nil {
		t.Fatal(err)
	}

	// First call should analyze and cache
	result1, isNew1, err := GetOrAnalyze(dir, false)
	if err != nil {
		t.Fatalf("GetOrAnalyze() failed: %v", err)
	}
	if !isNew1 {
		t.Error("First GetOrAnalyze() should return isNew=true")
	}
	if result1 == nil {
		t.Fatal("GetOrAnalyze() returned nil")
	}

	// Second call should use cache
	result2, isNew2, err := GetOrAnalyze(dir, false)
	if err != nil {
		t.Fatalf("GetOrAnalyze() failed: %v", err)
	}
	if isNew2 {
		t.Error("Second GetOrAnalyze() should return isNew=false (cached)")
	}
	if result2.Type != result1.Type {
		t.Error("Cached result should match original")
	}

	// Force refresh should analyze again
	result3, isNew3, err := GetOrAnalyze(dir, true)
	if err != nil {
		t.Fatalf("GetOrAnalyze() with refresh failed: %v", err)
	}
	if !isNew3 {
		t.Error("GetOrAnalyze() with forceRefresh should return isNew=true")
	}
	if result3.Type != result1.Type {
		t.Error("Refreshed result should match original")
	}
}
