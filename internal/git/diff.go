package git

import (
	"context"
	"fmt"
	"regexp"
	"strconv"
	"strings"
)

// GetDiff returns the complete diff information between base and HEAD.
func (r *Repository) GetDiff(ctx context.Context, baseRef string) (*DiffResult, error) {
	result := &DiffResult{
		BaseRef: baseRef,
		HeadRef: "HEAD",
	}

	// Get commits
	commits, err := r.GetCommits(ctx, baseRef)
	if err != nil {
		return nil, err
	}
	result.Commits = commits

	// Get file list with stats
	files, stats, err := r.getDiffFiles(ctx, baseRef)
	if err != nil {
		return nil, err
	}
	result.Files = files
	result.Stats = stats

	return result, nil
}

// getDiffFiles parses the diff stat and returns file information.
func (r *Repository) getDiffFiles(ctx context.Context, baseRef string) ([]FileDiff, DiffStats, error) {
	// Get numstat for accurate line counts
	numstatOutput, err := r.run(ctx, "diff", "--numstat", baseRef+"...HEAD")
	if err != nil {
		return nil, DiffStats{}, fmt.Errorf("getting diff numstat: %w", err)
	}

	// Get name-status for detecting renames and status
	nameStatusOutput, err := r.run(ctx, "diff", "--name-status", baseRef+"...HEAD")
	if err != nil {
		return nil, DiffStats{}, fmt.Errorf("getting diff name-status: %w", err)
	}

	// Parse numstat
	numstatMap := parseNumstat(numstatOutput)

	// Parse name-status and build file list
	files, stats := parseNameStatus(nameStatusOutput, numstatMap)

	return files, stats, nil
}

// parseNumstat parses git diff --numstat output.
// Format: additions<tab>deletions<tab>filepath
func parseNumstat(output string) map[string][2]int {
	result := make(map[string][2]int)
	if output == "" {
		return result
	}

	for _, line := range strings.Split(output, "\n") {
		parts := strings.Split(line, "\t")
		if len(parts) < 3 {
			continue
		}

		// Binary files show "-" for additions/deletions
		adds, _ := strconv.Atoi(parts[0])
		dels, _ := strconv.Atoi(parts[1])

		// Handle renames: old_path => new_path or just filepath
		path := parts[2]
		if strings.Contains(path, " => ") {
			// Rename format could be: dir/{old => new}/file or old_path => new_path
			path = extractNewPath(path)
		}

		result[path] = [2]int{adds, dels}
	}

	return result
}

// parseNameStatus parses git diff --name-status output.
// Format: status<tab>filepath (or status<tab>old_path<tab>new_path for renames)
func parseNameStatus(output string, numstat map[string][2]int) ([]FileDiff, DiffStats) {
	var files []FileDiff
	var stats DiffStats

	if output == "" {
		return files, stats
	}

	for _, line := range strings.Split(output, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}

		parts := strings.Split(line, "\t")
		if len(parts) < 2 {
			continue
		}

		statusCode := parts[0]
		file := FileDiff{}

		switch {
		case statusCode == "A":
			file.Status = StatusAdded
			file.Path = parts[1]
		case statusCode == "M":
			file.Status = StatusModified
			file.Path = parts[1]
		case statusCode == "D":
			file.Status = StatusDeleted
			file.Path = parts[1]
		case strings.HasPrefix(statusCode, "R"):
			file.Status = StatusRenamed
			if len(parts) >= 3 {
				file.OldPath = parts[1]
				file.Path = parts[2]
			} else {
				file.Path = parts[1]
			}
		case strings.HasPrefix(statusCode, "C"):
			// Copied file - treat as added
			file.Status = StatusAdded
			if len(parts) >= 3 {
				file.Path = parts[2]
			} else {
				file.Path = parts[1]
			}
		default:
			file.Status = StatusModified
			file.Path = parts[1]
		}

		// Get line counts from numstat
		if counts, ok := numstat[file.Path]; ok {
			file.Additions = counts[0]
			file.Deletions = counts[1]
			file.IsBinary = (counts[0] == 0 && counts[1] == 0 && file.Status != StatusDeleted)
		}

		files = append(files, file)
		stats.FilesChanged++
		stats.Additions += file.Additions
		stats.Deletions += file.Deletions
	}

	return files, stats
}

// extractNewPath extracts the new path from a rename format.
// Handles: "dir/{old => new}/file" and "old_path => new_path"
func extractNewPath(path string) string {
	// Handle brace format: dir/{old => new}/file
	braceRegex := regexp.MustCompile(`(.*)\{[^}]* => ([^}]*)\}(.*)`)
	if matches := braceRegex.FindStringSubmatch(path); len(matches) == 4 {
		return matches[1] + matches[2] + matches[3]
	}

	// Handle simple format: old_path => new_path
	if idx := strings.Index(path, " => "); idx != -1 {
		return path[idx+4:]
	}

	return path
}

// GetFileDiff returns the diff content for a specific file.
func (r *Repository) GetFileDiff(ctx context.Context, baseRef, filePath string) (string, error) {
	output, err := r.run(ctx, "diff", baseRef+"...HEAD", "--", filePath)
	if err != nil {
		return "", fmt.Errorf("getting diff for %s: %w", filePath, err)
	}
	return output, nil
}

// GetFileDiffColored returns the colored diff content for a specific file.
func (r *Repository) GetFileDiffColored(ctx context.Context, baseRef, filePath string) (string, error) {
	output, err := r.run(ctx, "diff", "--color=always", baseRef+"...HEAD", "--", filePath)
	if err != nil {
		return "", fmt.Errorf("getting colored diff for %s: %w", filePath, err)
	}
	return output, nil
}

// GetFullDiff returns the complete diff between base and HEAD.
func (r *Repository) GetFullDiff(ctx context.Context, baseRef string) (string, error) {
	output, err := r.run(ctx, "diff", baseRef+"...HEAD")
	if err != nil {
		return "", fmt.Errorf("getting full diff: %w", err)
	}
	return output, nil
}

// GetDiffStat returns a human-readable diff stat.
func (r *Repository) GetDiffStat(ctx context.Context, baseRef string) (string, error) {
	output, err := r.run(ctx, "diff", "--stat", baseRef+"...HEAD")
	if err != nil {
		return "", fmt.Errorf("getting diff stat: %w", err)
	}
	return output, nil
}
