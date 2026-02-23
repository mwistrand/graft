// Package filescan provides filesystem-based code scanning for non-git directories.
package filescan

import (
	"bufio"
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/mwistrand/graft/internal/git"
)

// defaultIgnoreDirs contains directory names to skip during scanning.
var defaultIgnoreDirs = map[string]bool{
	".git":            true,
	".svn":            true,
	".hg":             true,
	"node_modules":    true,
	"vendor":          true,
	"__pycache__":     true,
	".tox":            true,
	".venv":           true,
	"venv":            true,
	".mypy_cache":     true,
	".pytest_cache":   true,
	".next":           true,
	".nuxt":           true,
	"dist":            true,
	"build":           true,
	".gradle":         true,
	".idea":           true,
	".vscode":         true,
	"target":          true,
	".terraform":      true,
	".graft":          true,
}

// defaultIgnoreFiles contains file patterns to skip.
var defaultIgnoreFiles = map[string]bool{
	".DS_Store":  true,
	"Thumbs.db":  true,
}

// isBinary reports whether data contains a null byte in the first 8KB,
// which is a simple heuristic for detecting binary files.
func isBinary(data []byte) bool {
	limit := 8192
	if len(data) < limit {
		limit = len(data)
	}
	return bytes.Contains(data[:limit], []byte{0})
}

// ScanDirectory walks a directory tree and returns file information
// in the same format as git diff against the empty tree.
func ScanDirectory(dir string) (*git.DiffResult, error) {
	dir, err := filepath.Abs(dir)
	if err != nil {
		return nil, fmt.Errorf("resolving directory: %w", err)
	}

	var files []git.FileDiff
	var stats git.DiffStats

	err = filepath.WalkDir(dir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return nil // skip inaccessible entries
		}

		name := d.Name()

		// Skip ignored directories
		if d.IsDir() {
			if defaultIgnoreDirs[name] {
				return filepath.SkipDir
			}
			// Skip hidden directories (except the root)
			if strings.HasPrefix(name, ".") && path != dir {
				return filepath.SkipDir
			}
			return nil
		}

		// Skip ignored files
		if defaultIgnoreFiles[name] {
			return nil
		}
		// Skip hidden files
		if strings.HasPrefix(name, ".") {
			return nil
		}

		relPath, err := filepath.Rel(dir, path)
		if err != nil {
			return nil // skip
		}

		info, err := d.Info()
		if err != nil {
			return nil // skip
		}

		// Skip very large files (>1MB)
		if info.Size() > 1<<20 {
			return nil
		}

		// Read file to count lines and check for binary
		data, err := os.ReadFile(path)
		if err != nil {
			return nil // skip unreadable files
		}

		if isBinary(data) {
			files = append(files, git.FileDiff{
				Path:     relPath,
				Status:   git.StatusAdded,
				IsBinary: true,
			})
			stats.FilesChanged++
			return nil
		}

		lineCount := countLines(data)
		files = append(files, git.FileDiff{
			Path:      relPath,
			Status:    git.StatusAdded,
			Additions: lineCount,
		})
		stats.FilesChanged++
		stats.Additions += lineCount

		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("scanning directory: %w", err)
	}

	return &git.DiffResult{
		HeadRef: dir,
		Files:   files,
		Stats:   stats,
	}, nil
}

// countLines returns the number of lines in data.
func countLines(data []byte) int {
	if len(data) == 0 {
		return 0
	}
	count := bytes.Count(data, []byte("\n"))
	// If the file doesn't end with a newline, count the last line
	if data[len(data)-1] != '\n' {
		count++
	}
	return count
}

// GenerateFileDiff reads a file and produces synthetic unified diff output
// showing all lines as added (as if diffing against /dev/null).
func GenerateFileDiff(dir, filePath string) (string, error) {
	fullPath := filepath.Join(dir, filePath)
	data, err := os.ReadFile(fullPath)
	if err != nil {
		return "", fmt.Errorf("reading file: %w", err)
	}

	if isBinary(data) {
		return fmt.Sprintf("Binary file %s\n", filePath), nil
	}

	var b strings.Builder
	b.WriteString(fmt.Sprintf("diff --git a/%s b/%s\n", filePath, filePath))
	b.WriteString("new file mode 100644\n")
	b.WriteString("--- /dev/null\n")
	b.WriteString(fmt.Sprintf("+++ b/%s\n", filePath))

	lineCount := countLines(data)
	b.WriteString(fmt.Sprintf("@@ -0,0 +1,%d @@\n", lineCount))

	scanner := bufio.NewScanner(bytes.NewReader(data))
	for scanner.Scan() {
		b.WriteString("+" + scanner.Text() + "\n")
	}

	return b.String(), nil
}

// ContentFingerprint returns a hash derived from file paths and line counts,
// so the cache key changes when files are added, removed, or modified.
func ContentFingerprint(files []git.FileDiff) string {
	h := sha256.New()
	for _, f := range files {
		fmt.Fprintf(h, "%s\t%d\t%v\n", f.Path, f.Additions, f.IsBinary)
	}
	return hex.EncodeToString(h.Sum(nil))
}

// GenerateFullDiff produces a synthetic unified diff for all files in the scan result.
func GenerateFullDiff(dir string, files []git.FileDiff) (string, error) {
	var b strings.Builder
	for _, f := range files {
		if f.IsBinary {
			b.WriteString(fmt.Sprintf("Binary file %s\n\n", f.Path))
			continue
		}
		diff, err := GenerateFileDiff(dir, f.Path)
		if err != nil {
			continue // skip files we can't read
		}
		b.WriteString(diff)
		b.WriteString("\n")
	}
	return b.String(), nil
}
