// Package tui provides terminal UI components for interactive diff review.
package tui

import (
	"bytes"
	"context"
	"os/exec"
)

// DiffLoader loads diff content with optional Delta enhancement.
type DiffLoader struct {
	repoDir   string
	baseRef   string
	deltaPath string
}

// NewDiffLoader creates a new diff loader.
func NewDiffLoader(repoDir, baseRef, deltaPath string) *DiffLoader {
	return &DiffLoader{
		repoDir:   repoDir,
		baseRef:   baseRef,
		deltaPath: deltaPath,
	}
}

// LoadDiff loads the diff for a file, optionally piping through Delta.
func (d *DiffLoader) LoadDiff(ctx context.Context, filePath string) (string, error) {
	if d.deltaPath != "" {
		return d.loadWithDelta(ctx, filePath)
	}
	return d.loadPlain(ctx, filePath)
}

// loadWithDelta pipes git diff through delta for enhanced syntax highlighting.
func (d *DiffLoader) loadWithDelta(ctx context.Context, filePath string) (string, error) {
	gitCmd := exec.CommandContext(ctx, "git", "-C", d.repoDir, "diff", "--color=always", d.baseRef+"...HEAD", "--", filePath)
	deltaCmd := exec.CommandContext(ctx, d.deltaPath, "--paging=never")

	// Set up pipe from git to delta
	pipe, err := gitCmd.StdoutPipe()
	if err != nil {
		// Fall back to plain diff
		return d.loadPlain(ctx, filePath)
	}
	deltaCmd.Stdin = pipe

	var out bytes.Buffer
	var errBuf bytes.Buffer
	deltaCmd.Stdout = &out
	deltaCmd.Stderr = &errBuf

	if err := gitCmd.Start(); err != nil {
		return d.loadPlain(ctx, filePath)
	}

	if err := deltaCmd.Start(); err != nil {
		gitCmd.Wait()
		return d.loadPlain(ctx, filePath)
	}

	gitCmd.Wait()
	if err := deltaCmd.Wait(); err != nil {
		// Delta failed, fall back to plain
		return d.loadPlain(ctx, filePath)
	}

	return out.String(), nil
}

// loadPlain loads diff with git's native coloring.
func (d *DiffLoader) loadPlain(ctx context.Context, filePath string) (string, error) {
	cmd := exec.CommandContext(ctx, "git", "-C", d.repoDir, "diff", "--color=always", d.baseRef+"...HEAD", "--", filePath)
	out, err := cmd.Output()
	if err != nil {
		return "", err
	}
	return string(out), nil
}

// FindDelta looks for delta binary in PATH.
func FindDelta() string {
	path, err := exec.LookPath("delta")
	if err != nil {
		return ""
	}
	return path
}
