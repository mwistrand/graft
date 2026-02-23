package filescan

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mwistrand/graft/internal/git"
)

func setupTestDir(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()

	// Create a simple project structure
	writeFile(t, dir, "main.go", "package main\n\nfunc main() {\n\tprintln(\"hello\")\n}\n")
	writeFile(t, dir, "README.md", "# Test Project\n\nA test project.\n")
	writeFile(t, dir, "lib/helper.go", "package lib\n\nfunc Helper() string {\n\treturn \"help\"\n}\n")
	writeFile(t, dir, "lib/helper_test.go", "package lib\n\nimport \"testing\"\n\nfunc TestHelper(t *testing.T) {}\n")

	return dir
}

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

func TestScanDirectory(t *testing.T) {
	dir := setupTestDir(t)

	result, err := ScanDirectory(dir)
	if err != nil {
		t.Fatalf("ScanDirectory() failed: %v", err)
	}

	if len(result.Files) != 4 {
		t.Errorf("expected 4 files, got %d", len(result.Files))
		for _, f := range result.Files {
			t.Logf("  %s", f.Path)
		}
	}

	// All files should be "added"
	for _, f := range result.Files {
		if f.Status != git.StatusAdded {
			t.Errorf("file %q status = %q, want %q", f.Path, f.Status, git.StatusAdded)
		}
	}

	// Stats should be consistent
	if result.Stats.FilesChanged != len(result.Files) {
		t.Errorf("FilesChanged = %d, want %d", result.Stats.FilesChanged, len(result.Files))
	}
	if result.Stats.Additions == 0 {
		t.Error("expected non-zero additions")
	}
}

func TestScanDirectory_IgnoresCommonDirs(t *testing.T) {
	dir := setupTestDir(t)

	// Create files in ignored directories
	writeFile(t, dir, "node_modules/pkg/index.js", "module.exports = {}\n")
	writeFile(t, dir, ".git/config", "[core]\n")
	writeFile(t, dir, "vendor/lib/lib.go", "package lib\n")

	result, err := ScanDirectory(dir)
	if err != nil {
		t.Fatalf("ScanDirectory() failed: %v", err)
	}

	// Should only have the 4 original files, not the ones in ignored dirs
	for _, f := range result.Files {
		if strings.HasPrefix(f.Path, "node_modules/") ||
			strings.HasPrefix(f.Path, ".git/") ||
			strings.HasPrefix(f.Path, "vendor/") {
			t.Errorf("found file in ignored directory: %s", f.Path)
		}
	}
}

func TestScanDirectory_IgnoresHiddenFiles(t *testing.T) {
	dir := setupTestDir(t)

	writeFile(t, dir, ".DS_Store", "binary content")
	writeFile(t, dir, ".env", "SECRET=value\n")

	result, err := ScanDirectory(dir)
	if err != nil {
		t.Fatalf("ScanDirectory() failed: %v", err)
	}

	for _, f := range result.Files {
		if strings.HasPrefix(f.Path, ".") {
			t.Errorf("found hidden file: %s", f.Path)
		}
	}
}

func TestScanDirectory_BinaryFiles(t *testing.T) {
	dir := setupTestDir(t)

	// Create a binary file (contains null byte)
	binaryContent := []byte{0x89, 0x50, 0x4E, 0x47, 0x00, 0x0D, 0x0A, 0x1A}
	if err := os.WriteFile(filepath.Join(dir, "image.png"), binaryContent, 0o644); err != nil {
		t.Fatal(err)
	}

	result, err := ScanDirectory(dir)
	if err != nil {
		t.Fatalf("ScanDirectory() failed: %v", err)
	}

	for _, f := range result.Files {
		if f.Path == "image.png" {
			if !f.IsBinary {
				t.Error("image.png should be detected as binary")
			}
			if f.Additions != 0 {
				t.Errorf("binary file should have 0 additions, got %d", f.Additions)
			}
			return
		}
	}
	t.Error("image.png not found in results")
}

func TestScanDirectory_Empty(t *testing.T) {
	dir := t.TempDir()

	result, err := ScanDirectory(dir)
	if err != nil {
		t.Fatalf("ScanDirectory() failed: %v", err)
	}

	if len(result.Files) != 0 {
		t.Errorf("expected 0 files, got %d", len(result.Files))
	}
}

func TestGenerateFileDiff(t *testing.T) {
	dir := setupTestDir(t)

	diff, err := GenerateFileDiff(dir, "main.go")
	if err != nil {
		t.Fatalf("GenerateFileDiff() failed: %v", err)
	}

	// Should be valid unified diff format
	if !strings.HasPrefix(diff, "diff --git a/main.go b/main.go") {
		t.Error("diff should start with diff header")
	}
	if !strings.Contains(diff, "--- /dev/null") {
		t.Error("diff should show /dev/null as old file")
	}
	if !strings.Contains(diff, "+++ b/main.go") {
		t.Error("diff should show file as new")
	}
	if !strings.Contains(diff, "+package main") {
		t.Error("diff should contain file content with + prefix")
	}
}

func TestGenerateFileDiff_Binary(t *testing.T) {
	dir := t.TempDir()
	binaryContent := []byte{0x89, 0x50, 0x4E, 0x47, 0x00, 0x0D, 0x0A, 0x1A}
	if err := os.WriteFile(filepath.Join(dir, "image.png"), binaryContent, 0o644); err != nil {
		t.Fatal(err)
	}

	diff, err := GenerateFileDiff(dir, "image.png")
	if err != nil {
		t.Fatalf("GenerateFileDiff() failed: %v", err)
	}

	if !strings.Contains(diff, "Binary file") {
		t.Error("binary file diff should indicate binary content")
	}
}

func TestGenerateFullDiff(t *testing.T) {
	dir := setupTestDir(t)

	result, err := ScanDirectory(dir)
	if err != nil {
		t.Fatalf("ScanDirectory() failed: %v", err)
	}

	fullDiff, err := GenerateFullDiff(dir, result.Files)
	if err != nil {
		t.Fatalf("GenerateFullDiff() failed: %v", err)
	}

	if fullDiff == "" {
		t.Error("expected non-empty full diff")
	}

	// Should contain diffs for all text files
	if !strings.Contains(fullDiff, "main.go") {
		t.Error("full diff should contain main.go")
	}
	if !strings.Contains(fullDiff, "README.md") {
		t.Error("full diff should contain README.md")
	}
}

func TestScanDirectory_SkipsLargeFiles(t *testing.T) {
	dir := setupTestDir(t)

	// Create a file just over 1MB
	large := make([]byte, 1<<20+1)
	for i := range large {
		large[i] = 'x'
	}
	if err := os.WriteFile(filepath.Join(dir, "huge.bin"), large, 0o644); err != nil {
		t.Fatal(err)
	}

	result, err := ScanDirectory(dir)
	if err != nil {
		t.Fatalf("ScanDirectory() failed: %v", err)
	}

	for _, f := range result.Files {
		if f.Path == "huge.bin" {
			t.Error("files over 1MB should be skipped")
		}
	}
}

func TestContentFingerprint(t *testing.T) {
	files := []git.FileDiff{
		{Path: "a.go", Additions: 10},
		{Path: "b.go", Additions: 20},
	}

	fp1 := ContentFingerprint(files)
	fp2 := ContentFingerprint(files)
	if fp1 != fp2 {
		t.Error("same input should produce same fingerprint")
	}

	// Changing line count should change the fingerprint
	files2 := []git.FileDiff{
		{Path: "a.go", Additions: 11},
		{Path: "b.go", Additions: 20},
	}
	fp3 := ContentFingerprint(files2)
	if fp1 == fp3 {
		t.Error("different line counts should produce different fingerprint")
	}

	// Adding a file should change the fingerprint
	files3 := append(files, git.FileDiff{Path: "c.go", Additions: 5})
	fp4 := ContentFingerprint(files3)
	if fp1 == fp4 {
		t.Error("adding a file should produce different fingerprint")
	}
}

func TestCountLines(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  int
	}{
		{"empty", "", 0},
		{"one line with newline", "hello\n", 1},
		{"one line without newline", "hello", 1},
		{"two lines", "hello\nworld\n", 2},
		{"two lines no trailing newline", "hello\nworld", 2},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := countLines([]byte(tt.input))
			if got != tt.want {
				t.Errorf("countLines(%q) = %d, want %d", tt.input, got, tt.want)
			}
		})
	}
}

func TestIsBinary(t *testing.T) {
	tests := []struct {
		name  string
		input []byte
		want  bool
	}{
		{"text", []byte("hello world\n"), false},
		{"empty", []byte{}, false},
		{"binary with null", []byte{0x48, 0x65, 0x00, 0x6C}, true},
		{"png header", []byte{0x89, 0x50, 0x4E, 0x47, 0x00}, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := isBinary(tt.input)
			if got != tt.want {
				t.Errorf("isBinary() = %v, want %v", got, tt.want)
			}
		})
	}
}
