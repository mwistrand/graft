// Package tui provides terminal UI components for interactive diff review.
package tui

import (
	"bytes"
	"context"
	"os/exec"
	"strings"

	"github.com/mwistrand/graft/internal/filescan"
)

// DiffLoader loads diff content with optional Delta enhancement.
type DiffLoader struct {
	repoDir      string
	baseRef      string
	deltaPath    string
	fullCodebase bool
	noGit        bool
}

// NewDiffLoader creates a new diff loader.
func NewDiffLoader(repoDir, baseRef, deltaPath string, fullCodebase, noGit bool) *DiffLoader {
	return &DiffLoader{
		repoDir:      repoDir,
		baseRef:      baseRef,
		deltaPath:    deltaPath,
		fullCodebase: fullCodebase,
		noGit:        noGit,
	}
}

// LoadDiff loads the diff for a file, optionally piping through Delta.
func (d *DiffLoader) LoadDiff(ctx context.Context, filePath string) (string, error) {
	if d.noGit {
		return d.loadFromFilesystem(ctx, filePath)
	}
	if d.deltaPath != "" {
		return d.loadWithDelta(ctx, filePath)
	}
	return d.loadPlain(ctx, filePath)
}

// diffArgs returns the git diff arguments for the configured mode.
func (d *DiffLoader) diffArgs(filePath string) []string {
	args := []string{"-C", d.repoDir, "diff", "--color=always"}
	if d.fullCodebase {
		args = append(args, d.baseRef, "HEAD")
	} else {
		args = append(args, d.baseRef+"...HEAD")
	}
	args = append(args, "--", filePath)
	return args
}

// loadFromFilesystem generates a synthetic diff from file contents and
// optionally pipes it through Delta for syntax highlighting.
func (d *DiffLoader) loadFromFilesystem(ctx context.Context, filePath string) (string, error) {
	diff, err := filescan.GenerateFileDiff(d.repoDir, filePath)
	if err != nil {
		return "", err
	}

	if d.deltaPath != "" {
		colored, err := d.pipeThruDelta(ctx, diff)
		if err == nil {
			return colored, nil
		}
		// Fall back to uncolored diff
	}

	return diff, nil
}

// pipeThruDelta sends content through delta for syntax highlighting.
func (d *DiffLoader) pipeThruDelta(ctx context.Context, content string) (string, error) {
	cmd := exec.CommandContext(ctx, d.deltaPath, "--paging=never")
	cmd.Stdin = strings.NewReader(content)

	var out bytes.Buffer
	cmd.Stdout = &out

	if err := cmd.Run(); err != nil {
		return "", err
	}
	return out.String(), nil
}

// loadWithDelta pipes git diff through delta for enhanced syntax highlighting.
func (d *DiffLoader) loadWithDelta(ctx context.Context, filePath string) (string, error) {
	gitCmd := exec.CommandContext(ctx, "git", d.diffArgs(filePath)...)
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
	cmd := exec.CommandContext(ctx, "git", d.diffArgs(filePath)...)
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
