package git

import (
	"context"
	"fmt"
	"strings"
)

// BranchInfo contains information about a git branch.
type BranchInfo struct {
	// Name is the branch name.
	Name string

	// IsRemote indicates if this is a remote-tracking branch.
	IsRemote bool

	// Upstream is the upstream branch name, if configured.
	Upstream string

	// AheadBy is the number of commits ahead of upstream.
	AheadBy int

	// BehindBy is the number of commits behind upstream.
	BehindBy int
}

// GetBranchInfo returns information about the specified branch.
func (r *Repository) GetBranchInfo(ctx context.Context, branch string) (*BranchInfo, error) {
	info := &BranchInfo{
		Name: branch,
	}

	// Check if remote
	if strings.HasPrefix(branch, "remotes/") || strings.HasPrefix(branch, "origin/") {
		info.IsRemote = true
		return info, nil
	}

	// Get upstream
	upstream, err := r.run(ctx, "rev-parse", "--abbrev-ref", branch+"@{upstream}")
	if err == nil {
		info.Upstream = upstream

		// Get ahead/behind counts
		counts, err := r.run(ctx, "rev-list", "--left-right", "--count", branch+"..."+upstream)
		if err == nil {
			parts := strings.Fields(counts)
			if len(parts) == 2 {
				fmt.Sscanf(parts[0], "%d", &info.AheadBy)
				fmt.Sscanf(parts[1], "%d", &info.BehindBy)
			}
		}
	}

	return info, nil
}

// ListBranches returns all local branches.
func (r *Repository) ListBranches(ctx context.Context) ([]string, error) {
	return r.listBranches(ctx)
}

// ListRemoteBranches returns all remote branches.
func (r *Repository) ListRemoteBranches(ctx context.Context) ([]string, error) {
	output, err := r.run(ctx, "branch", "-r", "--format=%(refname:short)")
	if err != nil {
		return nil, err
	}
	if output == "" {
		return nil, nil
	}
	return strings.Split(output, "\n"), nil
}

// GetDefaultBranch attempts to determine the default branch (main/master).
func (r *Repository) GetDefaultBranch(ctx context.Context) (string, error) {
	// Try to get from remote HEAD
	ref, err := r.run(ctx, "symbolic-ref", "refs/remotes/origin/HEAD")
	if err == nil {
		// Extract branch name from refs/remotes/origin/main
		parts := strings.Split(ref, "/")
		if len(parts) > 0 {
			return parts[len(parts)-1], nil
		}
	}

	// Fall back to checking for common default branch names
	branches, err := r.listBranches(ctx)
	if err != nil {
		return "", err
	}

	for _, candidate := range []string{"main", "master", "develop"} {
		for _, b := range branches {
			if b == candidate {
				return candidate, nil
			}
		}
	}

	// Return first branch if nothing else matches
	if len(branches) > 0 {
		return branches[0], nil
	}

	return "", fmt.Errorf("could not determine default branch")
}
