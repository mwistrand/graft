package git

import (
	"context"
	"fmt"
	"strings"
	"time"
)

// Delimiter used for parsing git log output.
const commitDelimiter = "|||COMMIT|||"

// GetCommits returns commits between the base ref and HEAD.
func (r *Repository) GetCommits(ctx context.Context, baseRef string) ([]Commit, error) {
	// Format: hash|||short_hash|||author|||email|||date|||subject|||body|||COMMIT|||
	format := "%H" + commitDelimiter +
		"%h" + commitDelimiter +
		"%an" + commitDelimiter +
		"%ae" + commitDelimiter +
		"%aI" + commitDelimiter +
		"%s" + commitDelimiter +
		"%b" + commitDelimiter

	output, err := r.run(ctx, "log", baseRef+"..HEAD", "--pretty=format:"+format)
	if err != nil {
		return nil, fmt.Errorf("getting commits: %w", err)
	}

	if output == "" {
		return nil, nil
	}

	return parseCommits(output)
}

// GetCommit returns information about a single commit.
func (r *Repository) GetCommit(ctx context.Context, ref string) (*Commit, error) {
	format := "%H" + commitDelimiter +
		"%h" + commitDelimiter +
		"%an" + commitDelimiter +
		"%ae" + commitDelimiter +
		"%aI" + commitDelimiter +
		"%s" + commitDelimiter +
		"%b" + commitDelimiter

	output, err := r.run(ctx, "log", "-1", "--pretty=format:"+format, ref)
	if err != nil {
		return nil, fmt.Errorf("getting commit %s: %w", ref, err)
	}

	commits, err := parseCommits(output)
	if err != nil {
		return nil, err
	}

	if len(commits) == 0 {
		return nil, fmt.Errorf("commit %s not found", ref)
	}

	return &commits[0], nil
}

// parseCommits parses the git log output into Commit structs.
func parseCommits(output string) ([]Commit, error) {
	var commits []Commit

	// Split by the commit delimiter at the end of each commit
	entries := strings.Split(output, commitDelimiter+"\n")

	for _, entry := range entries {
		entry = strings.TrimSpace(entry)
		if entry == "" {
			continue
		}

		// Handle last entry which may not have trailing newline
		entry = strings.TrimSuffix(entry, commitDelimiter)

		parts := strings.Split(entry, commitDelimiter)
		if len(parts) < 6 {
			continue
		}

		date, err := time.Parse(time.RFC3339, parts[4])
		if err != nil {
			// Try alternate format
			date, _ = time.Parse("2006-01-02 15:04:05 -0700", parts[4])
		}

		commit := Commit{
			Hash:        parts[0],
			ShortHash:   parts[1],
			Author:      parts[2],
			AuthorEmail: parts[3],
			Date:        date,
			Subject:     parts[5],
		}

		// Body is the 7th part if present
		if len(parts) > 6 {
			commit.Body = strings.TrimSpace(parts[6])
		}

		commits = append(commits, commit)
	}

	return commits, nil
}

// GetCommitCount returns the number of commits between base and HEAD.
func (r *Repository) GetCommitCount(ctx context.Context, baseRef string) (int, error) {
	output, err := r.run(ctx, "rev-list", "--count", baseRef+"..HEAD")
	if err != nil {
		return 0, fmt.Errorf("counting commits: %w", err)
	}

	var count int
	fmt.Sscanf(output, "%d", &count)
	return count, nil
}
