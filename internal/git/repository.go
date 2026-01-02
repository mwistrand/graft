package git

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// ErrNotARepository is returned when the path is not a git repository.
var ErrNotARepository = errors.New("not a git repository")

// Repository provides operations on a git repository.
type Repository struct {
	// dir is the working directory of the repository.
	dir string
}

// NewRepository creates a new Repository for the given directory.
// If dir is empty, the current working directory is used.
// Returns ErrNotARepository if the directory is not within a git repository.
func NewRepository(dir string) (*Repository, error) {
	if dir == "" {
		var err error
		dir, err = os.Getwd()
		if err != nil {
			return nil, fmt.Errorf("getting current directory: %w", err)
		}
	}

	// Verify this is a git repository
	r := &Repository{dir: dir}
	if _, err := r.run(context.Background(), "rev-parse", "--git-dir"); err != nil {
		return nil, ErrNotARepository
	}

	return r, nil
}

// Dir returns the repository working directory.
func (r *Repository) Dir() string {
	return r.dir
}

// run executes a git command and returns its output.
func (r *Repository) run(ctx context.Context, args ...string) (string, error) {
	cmd := exec.CommandContext(ctx, "git", args...)
	cmd.Dir = r.dir

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		errMsg := strings.TrimSpace(stderr.String())
		if errMsg == "" {
			errMsg = err.Error()
		}
		return "", fmt.Errorf("git %s: %s", args[0], errMsg)
	}

	return strings.TrimSpace(stdout.String()), nil
}

// runWithInput executes a git command with stdin and returns its output.
func (r *Repository) runWithInput(ctx context.Context, input string, args ...string) (string, error) {
	cmd := exec.CommandContext(ctx, "git", args...)
	cmd.Dir = r.dir
	cmd.Stdin = strings.NewReader(input)

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		errMsg := strings.TrimSpace(stderr.String())
		if errMsg == "" {
			errMsg = err.Error()
		}
		return "", fmt.Errorf("git %s: %s", args[0], errMsg)
	}

	return strings.TrimSpace(stdout.String()), nil
}

// GetCurrentBranch returns the name of the current branch.
func (r *Repository) GetCurrentBranch(ctx context.Context) (string, error) {
	branch, err := r.run(ctx, "rev-parse", "--abbrev-ref", "HEAD")
	if err != nil {
		return "", fmt.Errorf("getting current branch: %w", err)
	}
	return branch, nil
}

// ValidateBranch checks if a branch or ref exists.
func (r *Repository) ValidateBranch(ctx context.Context, ref string) error {
	_, err := r.run(ctx, "rev-parse", "--verify", ref)
	if err != nil {
		// Try to suggest similar branches
		branches, _ := r.listBranches(ctx)
		suggestions := findSimilar(ref, branches)
		if len(suggestions) > 0 {
			return fmt.Errorf("branch %q not found; did you mean: %s", ref, strings.Join(suggestions, ", "))
		}
		return fmt.Errorf("branch %q not found", ref)
	}
	return nil
}

// listBranches returns all local branch names.
func (r *Repository) listBranches(ctx context.Context) ([]string, error) {
	output, err := r.run(ctx, "branch", "--format=%(refname:short)")
	if err != nil {
		return nil, err
	}
	if output == "" {
		return nil, nil
	}
	return strings.Split(output, "\n"), nil
}

// GetMergeBase returns the merge base between two refs.
func (r *Repository) GetMergeBase(ctx context.Context, ref1, ref2 string) (string, error) {
	base, err := r.run(ctx, "merge-base", ref1, ref2)
	if err != nil {
		return "", fmt.Errorf("getting merge base: %w", err)
	}
	return base, nil
}

// GetRootDir returns the repository root directory.
func (r *Repository) GetRootDir(ctx context.Context) (string, error) {
	root, err := r.run(ctx, "rev-parse", "--show-toplevel")
	if err != nil {
		return "", fmt.Errorf("getting repository root: %w", err)
	}
	return root, nil
}

// findSimilar finds branch names similar to the target.
func findSimilar(target string, candidates []string) []string {
	target = strings.ToLower(target)
	var similar []string

	for _, c := range candidates {
		lower := strings.ToLower(c)
		// Check if one contains the other
		if strings.Contains(lower, target) || strings.Contains(target, lower) {
			similar = append(similar, c)
		}
	}

	// Limit to 3 suggestions
	if len(similar) > 3 {
		similar = similar[:3]
	}

	return similar
}

// IsClean returns true if the working directory has no uncommitted changes.
func (r *Repository) IsClean(ctx context.Context) (bool, error) {
	status, err := r.run(ctx, "status", "--porcelain")
	if err != nil {
		return false, err
	}
	return status == "", nil
}

// HasUnpushedCommits returns true if there are commits not pushed to the remote.
func (r *Repository) HasUnpushedCommits(ctx context.Context, branch string) (bool, error) {
	// Get the upstream branch
	upstream, err := r.run(ctx, "rev-parse", "--abbrev-ref", branch+"@{upstream}")
	if err != nil {
		// No upstream configured
		return false, nil
	}

	// Check if there are commits between upstream and branch
	count, err := r.run(ctx, "rev-list", "--count", upstream+".."+branch)
	if err != nil {
		return false, err
	}

	return count != "0", nil
}

// ResolvePath resolves a relative path to an absolute path within the repository.
func (r *Repository) ResolvePath(path string) string {
	if filepath.IsAbs(path) {
		return path
	}
	return filepath.Join(r.dir, path)
}
