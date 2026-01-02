package git

import (
	"context"
	"strings"
	"testing"
)

func TestGetCommits(t *testing.T) {
	dir := setupTestRepo(t)
	repo, _ := NewRepository(dir)
	ctx := context.Background()

	// Get the current branch (initial commit base)
	branch, _ := repo.GetCurrentBranch(ctx)

	// Create a new branch from the initial commit
	runGit(t, dir, "checkout", "-b", "feature")

	// Add some commits
	writeFile(t, dir, "file1.go", "package main")
	runGit(t, dir, "add", "file1.go")
	runGit(t, dir, "commit", "-m", "Add file1")

	writeFile(t, dir, "file2.go", "package main")
	runGit(t, dir, "add", "file2.go")
	runGit(t, dir, "commit", "-m", "Add file2\n\nThis is the body of the commit message.")

	// Get commits since base branch
	commits, err := repo.GetCommits(ctx, branch)
	if err != nil {
		t.Fatalf("GetCommits() failed: %v", err)
	}

	if len(commits) != 2 {
		t.Fatalf("expected 2 commits, got %d", len(commits))
	}

	// Commits are in reverse chronological order (newest first)
	if !strings.Contains(commits[0].Subject, "file2") {
		t.Errorf("expected first commit to mention file2, got: %s", commits[0].Subject)
	}
	if !strings.Contains(commits[1].Subject, "file1") {
		t.Errorf("expected second commit to mention file1, got: %s", commits[1].Subject)
	}

	// Verify commit metadata
	if commits[0].Author != "Test User" {
		t.Errorf("Author = %q, want %q", commits[0].Author, "Test User")
	}
	if commits[0].AuthorEmail != "test@example.com" {
		t.Errorf("AuthorEmail = %q, want %q", commits[0].AuthorEmail, "test@example.com")
	}
	if commits[0].Hash == "" {
		t.Error("Hash should not be empty")
	}
	if commits[0].ShortHash == "" {
		t.Error("ShortHash should not be empty")
	}
	if commits[0].Date.IsZero() {
		t.Error("Date should not be zero")
	}
}

func TestGetCommit(t *testing.T) {
	dir := setupTestRepo(t)
	repo, _ := NewRepository(dir)
	ctx := context.Background()

	// Get HEAD commit
	commit, err := repo.GetCommit(ctx, "HEAD")
	if err != nil {
		t.Fatalf("GetCommit(HEAD) failed: %v", err)
	}

	if commit.Subject != "Initial commit" {
		t.Errorf("Subject = %q, want %q", commit.Subject, "Initial commit")
	}
}

func TestGetCommitCount(t *testing.T) {
	dir := setupTestRepo(t)
	repo, _ := NewRepository(dir)
	ctx := context.Background()

	branch, _ := repo.GetCurrentBranch(ctx)
	runGit(t, dir, "checkout", "-b", "counting")

	// Add 3 commits
	for i := 0; i < 3; i++ {
		writeFile(t, dir, "file"+string(rune('a'+i))+".txt", "content")
		runGit(t, dir, "add", ".")
		runGit(t, dir, "commit", "-m", "Commit "+string(rune('a'+i)))
	}

	count, err := repo.GetCommitCount(ctx, branch)
	if err != nil {
		t.Fatalf("GetCommitCount() failed: %v", err)
	}

	if count != 3 {
		t.Errorf("GetCommitCount() = %d, want 3", count)
	}
}

func TestCommitMessage(t *testing.T) {
	commit := &Commit{
		Subject: "Fix bug",
		Body:    "This is the detailed explanation.",
	}

	msg := commit.Message()
	expected := "Fix bug\n\nThis is the detailed explanation."
	if msg != expected {
		t.Errorf("Message() = %q, want %q", msg, expected)
	}

	// Without body
	commit.Body = ""
	msg = commit.Message()
	if msg != "Fix bug" {
		t.Errorf("Message() = %q, want %q", msg, "Fix bug")
	}
}

func TestParseCommits(t *testing.T) {
	// Test the parseCommits function directly with known input
	// Format matches what git log produces with our format string
	input := "abc123" + commitDelimiter +
		"abc" + commitDelimiter +
		"John Doe" + commitDelimiter +
		"john@example.com" + commitDelimiter +
		"2024-01-15T10:30:00Z" + commitDelimiter +
		"Initial commit" + commitDelimiter +
		"" + commitDelimiter + "\n" +
		"def456" + commitDelimiter +
		"def" + commitDelimiter +
		"Jane Doe" + commitDelimiter +
		"jane@example.com" + commitDelimiter +
		"2024-01-15T11:00:00Z" + commitDelimiter +
		"Add feature" + commitDelimiter +
		"This is the body" + commitDelimiter

	commits, err := parseCommits(input)
	if err != nil {
		t.Fatalf("parseCommits() failed: %v", err)
	}

	if len(commits) != 2 {
		t.Fatalf("expected 2 commits, got %d", len(commits))
	}

	if commits[0].Hash != "abc123" {
		t.Errorf("Hash = %q, want %q", commits[0].Hash, "abc123")
	}
	if commits[0].Author != "John Doe" {
		t.Errorf("Author = %q, want %q", commits[0].Author, "John Doe")
	}

	if commits[1].Hash != "def456" {
		t.Errorf("Hash = %q, want %q", commits[1].Hash, "def456")
	}
	if commits[1].Body != "This is the body" {
		t.Errorf("Body = %q, want %q", commits[1].Body, "This is the body")
	}
}
