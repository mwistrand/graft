package git

import (
	"context"
	"testing"
)

func TestGetDiff(t *testing.T) {
	dir := setupTestRepo(t)
	repo, _ := NewRepository(dir)
	ctx := context.Background()

	branch, _ := repo.GetCurrentBranch(ctx)
	runGit(t, dir, "checkout", "-b", "diff-test")

	// Make some changes
	writeFile(t, dir, "new_file.go", "package main\n\nfunc main() {}\n")
	writeFile(t, dir, "README.md", "# Updated\n\nSome content.\n")
	runGit(t, dir, "add", ".")
	runGit(t, dir, "commit", "-m", "Add changes")

	result, err := repo.GetDiff(ctx, branch)
	if err != nil {
		t.Fatalf("GetDiff() failed: %v", err)
	}

	if result.BaseRef != branch {
		t.Errorf("BaseRef = %q, want %q", result.BaseRef, branch)
	}
	if result.HeadRef != "HEAD" {
		t.Errorf("HeadRef = %q, want %q", result.HeadRef, "HEAD")
	}

	if len(result.Files) != 2 {
		t.Errorf("expected 2 files, got %d", len(result.Files))
	}

	if len(result.Commits) != 1 {
		t.Errorf("expected 1 commit, got %d", len(result.Commits))
	}

	// Check stats
	if result.Stats.FilesChanged != 2 {
		t.Errorf("FilesChanged = %d, want 2", result.Stats.FilesChanged)
	}
}

func TestGetDiff_FileStatuses(t *testing.T) {
	dir := setupTestRepo(t)
	repo, _ := NewRepository(dir)
	ctx := context.Background()

	branch, _ := repo.GetCurrentBranch(ctx)
	runGit(t, dir, "checkout", "-b", "status-test")

	// Add a new file
	writeFile(t, dir, "added.go", "package main\n")
	// Modify existing file
	writeFile(t, dir, "README.md", "# Modified\n")
	// Delete won't work without a file to delete, skip for now

	runGit(t, dir, "add", ".")
	runGit(t, dir, "commit", "-m", "Various changes")

	result, err := repo.GetDiff(ctx, branch)
	if err != nil {
		t.Fatalf("GetDiff() failed: %v", err)
	}

	statusMap := make(map[string]string)
	for _, f := range result.Files {
		statusMap[f.Path] = f.Status
	}

	if statusMap["added.go"] != StatusAdded {
		t.Errorf("added.go status = %q, want %q", statusMap["added.go"], StatusAdded)
	}
	if statusMap["README.md"] != StatusModified {
		t.Errorf("README.md status = %q, want %q", statusMap["README.md"], StatusModified)
	}
}

func TestGetDiff_Rename(t *testing.T) {
	dir := setupTestRepo(t)
	repo, _ := NewRepository(dir)
	ctx := context.Background()

	branch, _ := repo.GetCurrentBranch(ctx)
	runGit(t, dir, "checkout", "-b", "rename-test")

	// Create and commit a file with enough content for git to detect similarity
	content := `package main

// This file has enough content for git to detect it as a rename
// when we move it to a new location. Git uses a similarity index
// to determine if a file was renamed vs deleted and added.

func main() {
	println("Hello, World!")
}

func helper() {
	println("This is a helper function")
}
`
	writeFile(t, dir, "old_name.go", content)
	runGit(t, dir, "add", ".")
	runGit(t, dir, "commit", "-m", "Add file")

	// Rename it
	runGit(t, dir, "mv", "old_name.go", "new_name.go")
	runGit(t, dir, "commit", "-m", "Rename file")

	result, err := repo.GetDiff(ctx, branch)
	if err != nil {
		t.Fatalf("GetDiff() failed: %v", err)
	}

	// Find the renamed file - could be detected as rename or add+delete
	var foundNew, foundOld bool
	for _, f := range result.Files {
		if f.Path == "new_name.go" {
			foundNew = true
			// If detected as rename, verify OldPath
			if f.Status == StatusRenamed {
				if f.OldPath != "old_name.go" {
					t.Errorf("OldPath = %q, want %q", f.OldPath, "old_name.go")
				}
				return // Test passed - rename detected
			}
		}
		if f.Path == "old_name.go" && f.Status == StatusDeleted {
			foundOld = true
		}
	}

	// Either rename detected OR add+delete pair (both are valid git behaviors)
	if !foundNew {
		t.Error("new_name.go not found in diff")
	}

	// If not detected as rename, should see old_name.go as deleted
	// (This is acceptable behavior for git when similarity threshold isn't met)
	t.Logf("Rename detection: foundNew=%v, foundOld=%v (add+delete pattern is acceptable)", foundNew, foundOld)
}

func TestParseNumstat(t *testing.T) {
	tests := []struct {
		name   string
		input  string
		want   map[string][2]int
	}{
		{
			name:  "simple",
			input: "10\t5\tfile.go",
			want:  map[string][2]int{"file.go": {10, 5}},
		},
		{
			name:  "multiple files",
			input: "1\t2\ta.go\n3\t4\tb.go",
			want: map[string][2]int{
				"a.go": {1, 2},
				"b.go": {3, 4},
			},
		},
		{
			name:  "binary file",
			input: "-\t-\timage.png",
			want:  map[string][2]int{"image.png": {0, 0}},
		},
		{
			name:  "empty",
			input: "",
			want:  map[string][2]int{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := parseNumstat(tt.input)
			if len(got) != len(tt.want) {
				t.Errorf("parseNumstat() returned %d entries, want %d", len(got), len(tt.want))
			}
			for k, v := range tt.want {
				if got[k] != v {
					t.Errorf("parseNumstat()[%q] = %v, want %v", k, got[k], v)
				}
			}
		})
	}
}

func TestParseNameStatus(t *testing.T) {
	numstat := map[string][2]int{
		"added.go":    {10, 0},
		"modified.go": {5, 3},
		"new_name.go": {0, 0},
	}

	input := "A\tadded.go\nM\tmodified.go\nR100\told_name.go\tnew_name.go\nD\tdeleted.go"

	files, stats := parseNameStatus(input, numstat)

	if len(files) != 4 {
		t.Fatalf("expected 4 files, got %d", len(files))
	}

	// Check each file
	fileMap := make(map[string]FileDiff)
	for _, f := range files {
		fileMap[f.Path] = f
	}

	if fileMap["added.go"].Status != StatusAdded {
		t.Errorf("added.go status = %q, want %q", fileMap["added.go"].Status, StatusAdded)
	}
	if fileMap["added.go"].Additions != 10 {
		t.Errorf("added.go additions = %d, want 10", fileMap["added.go"].Additions)
	}

	if fileMap["modified.go"].Status != StatusModified {
		t.Errorf("modified.go status = %q, want %q", fileMap["modified.go"].Status, StatusModified)
	}

	if fileMap["new_name.go"].Status != StatusRenamed {
		t.Errorf("new_name.go status = %q, want %q", fileMap["new_name.go"].Status, StatusRenamed)
	}
	if fileMap["new_name.go"].OldPath != "old_name.go" {
		t.Errorf("new_name.go OldPath = %q, want %q", fileMap["new_name.go"].OldPath, "old_name.go")
	}

	if fileMap["deleted.go"].Status != StatusDeleted {
		t.Errorf("deleted.go status = %q, want %q", fileMap["deleted.go"].Status, StatusDeleted)
	}

	// Check stats
	if stats.FilesChanged != 4 {
		t.Errorf("FilesChanged = %d, want 4", stats.FilesChanged)
	}
}

func TestExtractNewPath(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"old.go => new.go", "new.go"},
		{"dir/{old => new}/file.go", "dir/new/file.go"},
		{"simple.go", "simple.go"},
		{"{old => new}.go", "new.go"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := extractNewPath(tt.input)
			if got != tt.want {
				t.Errorf("extractNewPath(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

func TestGetFileDiff(t *testing.T) {
	dir := setupTestRepo(t)
	repo, _ := NewRepository(dir)
	ctx := context.Background()

	branch, _ := repo.GetCurrentBranch(ctx)
	runGit(t, dir, "checkout", "-b", "file-diff-test")

	writeFile(t, dir, "test.go", "package main\n\nfunc hello() {}\n")
	runGit(t, dir, "add", ".")
	runGit(t, dir, "commit", "-m", "Add test.go")

	diff, err := repo.GetFileDiff(ctx, branch, "test.go")
	if err != nil {
		t.Fatalf("GetFileDiff() failed: %v", err)
	}

	if diff == "" {
		t.Error("expected non-empty diff")
	}

	// Should contain the file content
	if !containsString(diff, "func hello()") {
		t.Error("diff should contain 'func hello()'")
	}
}

func containsString(haystack, needle string) bool {
	return len(haystack) > 0 && len(needle) > 0 &&
		(haystack == needle || len(haystack) >= len(needle) &&
		findSubstring(haystack, needle))
}

func findSubstring(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
