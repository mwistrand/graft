// Package git provides git operations for extracting diff and commit information.
package git

import "time"

// FileDiff represents the diff information for a single file.
type FileDiff struct {
	// Path is the file path relative to the repository root.
	Path string

	// OldPath is the original path for renamed files.
	OldPath string

	// Status indicates the type of change: "added", "modified", "deleted", "renamed".
	Status string

	// Additions is the number of lines added.
	Additions int

	// Deletions is the number of lines deleted.
	Deletions int

	// IsBinary indicates whether this is a binary file.
	IsBinary bool

	// Patch contains the actual diff content for this file.
	Patch string
}

// Commit represents a git commit with its metadata.
type Commit struct {
	// Hash is the full commit hash.
	Hash string

	// ShortHash is the abbreviated commit hash.
	ShortHash string

	// Author is the commit author name.
	Author string

	// AuthorEmail is the commit author email.
	AuthorEmail string

	// Date is the commit timestamp.
	Date time.Time

	// Subject is the first line of the commit message.
	Subject string

	// Body is the rest of the commit message (after the first line).
	Body string
}

// Message returns the full commit message (subject + body).
func (c *Commit) Message() string {
	if c.Body == "" {
		return c.Subject
	}
	return c.Subject + "\n\n" + c.Body
}

// DiffResult contains all diff information between two git refs.
type DiffResult struct {
	// BaseRef is the base reference (e.g., "main").
	BaseRef string

	// HeadRef is the head reference (e.g., "HEAD" or branch name).
	HeadRef string

	// Files contains the diff for each changed file.
	Files []FileDiff

	// Commits contains the commits between base and head.
	Commits []Commit

	// Stats contains summary statistics.
	Stats DiffStats
}

// DiffStats contains summary statistics for a diff.
type DiffStats struct {
	// FilesChanged is the total number of files changed.
	FilesChanged int

	// Additions is the total number of lines added.
	Additions int

	// Deletions is the total number of lines deleted.
	Deletions int
}

// FileStatus constants for diff status.
const (
	StatusAdded    = "added"
	StatusModified = "modified"
	StatusDeleted  = "deleted"
	StatusRenamed  = "renamed"
)
