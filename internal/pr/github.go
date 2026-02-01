package pr

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
	"strings"
	"time"
)

const (
	// defaultGitHubHost is the standard GitHub hostname.
	defaultGitHubHost = "github.com"

	// authCheckTimeout is the maximum time to wait for gh auth status.
	authCheckTimeout = 5 * time.Second
)

// GitHubResolver fetches PR metadata using the GitHub CLI (gh).
type GitHubResolver struct {
	host string
}

// NewGitHubResolver creates a resolver for GitHub.
func NewGitHubResolver(host string) *GitHubResolver {
	if host == "" {
		host = defaultGitHubHost
	}
	return &GitHubResolver{host: host}
}

// Platform returns PlatformGitHub, identifying this as a GitHub resolver.
func (r *GitHubResolver) Platform() Platform {
	return PlatformGitHub
}

// IsAvailable checks if gh CLI is installed and authenticated for this host.
func (r *GitHubResolver) IsAvailable(ctx context.Context) bool {
	// Check if gh is installed
	if _, err := exec.LookPath("gh"); err != nil {
		return false
	}

	// Check authentication with a timeout to avoid hanging
	checkCtx, cancel := context.WithTimeout(ctx, authCheckTimeout)
	defer cancel()

	// Use --hostname for enterprise GitHub instances
	args := []string{"auth", "status"}
	if r.host != defaultGitHubHost {
		args = append(args, "--hostname", r.host)
	}

	cmd := exec.CommandContext(checkCtx, "gh", args...)
	return cmd.Run() == nil
}

// Resolve fetches the PR metadata using gh CLI.
func (r *GitHubResolver) Resolve(ctx context.Context, info *PRInfo) (*PRMetadata, error) {
	// Build args with hostname support for enterprise
	args := []string{
		"pr", "view",
		fmt.Sprintf("%d", info.Number),
		"--repo", fmt.Sprintf("%s/%s", info.Owner, info.Repo),
		"--json", "baseRefName,headRefName,headRefOid,state,title,merged",
	}

	// Add hostname for enterprise GitHub instances
	if r.host != defaultGitHubHost {
		args = append(args, "--hostname", r.host)
	}

	cmd := exec.CommandContext(ctx, "gh", args...)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		return nil, r.handleError(err, stderr.String(), info)
	}

	var ghResp struct {
		BaseRefName string `json:"baseRefName"`
		HeadRefName string `json:"headRefName"`
		HeadRefOid  string `json:"headRefOid"`
		State       string `json:"state"`
		Title       string `json:"title"`
		Merged      bool   `json:"merged"`
	}

	if err := json.Unmarshal(stdout.Bytes(), &ghResp); err != nil {
		return nil, fmt.Errorf("parsing gh response: %w", err)
	}

	return &PRMetadata{
		PRInfo:   *info,
		Title:    ghResp.Title,
		BaseRef:  ghResp.BaseRefName,
		HeadRef:  ghResp.HeadRefName,
		HeadSHA:  ghResp.HeadRefOid,
		State:    normalizeState(ghResp.State, ghResp.Merged),
		IsMerged: ghResp.Merged,
	}, nil
}

// handleError converts gh CLI errors to appropriate typed errors.
func (r *GitHubResolver) handleError(err error, stderr string, info *PRInfo) error {
	stderrLower := strings.ToLower(stderr)

	// Check for authentication errors
	if strings.Contains(stderrLower, "authentication") ||
		strings.Contains(stderrLower, "not logged") ||
		strings.Contains(stderrLower, "gh auth login") {
		return &ErrCLINotFound{Platform: PlatformGitHub, CLI: "gh"}
	}

	// Check for not found errors
	if strings.Contains(stderrLower, "could not resolve") ||
		strings.Contains(stderrLower, "not found") ||
		strings.Contains(stderrLower, "no pull requests") {
		return &ErrPRNotFound{URL: info.OriginalURL}
	}

	// Check for permission errors
	if strings.Contains(stderrLower, "permission") ||
		strings.Contains(stderrLower, "forbidden") ||
		strings.Contains(stderrLower, "403") {
		return fmt.Errorf("access denied to %s: %s", info.OriginalURL, strings.TrimSpace(stderr))
	}

	// Check if gh CLI is not installed (command not found)
	if exitErr, ok := err.(*exec.ExitError); ok {
		if exitErr.ExitCode() == 127 {
			return &ErrCLINotFound{Platform: PlatformGitHub, CLI: "gh"}
		}
	}

	// Generic error
	errMsg := strings.TrimSpace(stderr)
	if errMsg == "" {
		errMsg = err.Error()
	}
	return fmt.Errorf("gh pr view failed: %s", errMsg)
}

// normalizeState returns a consistent state value.
func normalizeState(state string, merged bool) string {
	if merged {
		return StateMerged
	}
	state = strings.ToLower(state)
	switch state {
	case "open":
		return StateOpen
	case "closed":
		return StateClosed
	case "merged":
		return StateMerged
	default:
		return state
	}
}
