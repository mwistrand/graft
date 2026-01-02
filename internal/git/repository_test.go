package git

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

// setupTestRepo creates a temporary git repository for testing.
func setupTestRepo(t *testing.T) string {
	t.Helper()

	dir := t.TempDir()

	// Initialize git repo
	runGit(t, dir, "init")
	runGit(t, dir, "config", "user.email", "test@example.com")
	runGit(t, dir, "config", "user.name", "Test User")

	// Create initial commit
	writeFile(t, dir, "README.md", "# Test Repo\n")
	runGit(t, dir, "add", "README.md")
	runGit(t, dir, "commit", "-m", "Initial commit")

	return dir
}

// runGit runs a git command in the given directory.
func runGit(t *testing.T, dir string, args ...string) string {
	t.Helper()

	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %v failed: %s\n%s", args, err, output)
	}
	return string(output)
}

// writeFile creates a file with the given content.
func writeFile(t *testing.T, dir, name, content string) {
	t.Helper()

	path := filepath.Join(dir, name)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestNewRepository(t *testing.T) {
	dir := setupTestRepo(t)

	repo, err := NewRepository(dir)
	if err != nil {
		t.Fatalf("NewRepository() failed: %v", err)
	}

	if repo.Dir() != dir {
		t.Errorf("Dir() = %q, want %q", repo.Dir(), dir)
	}
}

func TestNewRepository_NotARepo(t *testing.T) {
	dir := t.TempDir()

	_, err := NewRepository(dir)
	if err != ErrNotARepository {
		t.Errorf("expected ErrNotARepository, got %v", err)
	}
}

func TestGetCurrentBranch(t *testing.T) {
	dir := setupTestRepo(t)
	repo, _ := NewRepository(dir)

	branch, err := repo.GetCurrentBranch(context.Background())
	if err != nil {
		t.Fatalf("GetCurrentBranch() failed: %v", err)
	}

	// Default branch is usually main or master
	if branch != "main" && branch != "master" {
		t.Errorf("GetCurrentBranch() = %q, expected main or master", branch)
	}
}

func TestValidateBranch(t *testing.T) {
	dir := setupTestRepo(t)
	repo, _ := NewRepository(dir)
	ctx := context.Background()

	// Get the current branch name
	branch, _ := repo.GetCurrentBranch(ctx)

	// Valid branch should not error
	if err := repo.ValidateBranch(ctx, branch); err != nil {
		t.Errorf("ValidateBranch(%q) failed: %v", branch, err)
	}

	// Invalid branch should error
	if err := repo.ValidateBranch(ctx, "nonexistent-branch"); err == nil {
		t.Error("ValidateBranch(nonexistent) should have failed")
	}
}

func TestIsClean(t *testing.T) {
	dir := setupTestRepo(t)
	repo, _ := NewRepository(dir)
	ctx := context.Background()

	// Initially clean
	clean, err := repo.IsClean(ctx)
	if err != nil {
		t.Fatalf("IsClean() failed: %v", err)
	}
	if !clean {
		t.Error("expected repository to be clean")
	}

	// Add untracked file
	writeFile(t, dir, "new.txt", "content")

	clean, err = repo.IsClean(ctx)
	if err != nil {
		t.Fatalf("IsClean() failed: %v", err)
	}
	if clean {
		t.Error("expected repository to be dirty")
	}
}

func TestListBranches(t *testing.T) {
	dir := setupTestRepo(t)
	repo, _ := NewRepository(dir)
	ctx := context.Background()

	// Create a new branch
	runGit(t, dir, "branch", "feature-test")

	branches, err := repo.ListBranches(ctx)
	if err != nil {
		t.Fatalf("ListBranches() failed: %v", err)
	}

	if len(branches) < 2 {
		t.Errorf("expected at least 2 branches, got %d", len(branches))
	}

	found := false
	for _, b := range branches {
		if b == "feature-test" {
			found = true
			break
		}
	}
	if !found {
		t.Error("feature-test branch not found in branch list")
	}
}

func TestGetRootDir(t *testing.T) {
	dir := setupTestRepo(t)

	// Create a subdirectory
	subdir := filepath.Join(dir, "sub", "dir")
	if err := os.MkdirAll(subdir, 0o755); err != nil {
		t.Fatal(err)
	}

	// Create repo from subdirectory
	repo, err := NewRepository(subdir)
	if err != nil {
		t.Fatalf("NewRepository() failed: %v", err)
	}

	root, err := repo.GetRootDir(context.Background())
	if err != nil {
		t.Fatalf("GetRootDir() failed: %v", err)
	}

	// Root should be the original dir (need to resolve symlinks for comparison on macOS)
	expectedRoot, _ := filepath.EvalSymlinks(dir)
	actualRoot, _ := filepath.EvalSymlinks(root)

	if actualRoot != expectedRoot {
		t.Errorf("GetRootDir() = %q, want %q", actualRoot, expectedRoot)
	}
}
