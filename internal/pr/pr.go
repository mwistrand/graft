// Package pr provides pull request URL parsing and metadata fetching.
//
// Currently supports GitHub via the gh CLI. GitLab and BitBucket URL parsing
// is implemented, but resolvers return ErrUnsupportedPlatform.
//
// The Resolver interface defines the contract for fetching PR metadata.
// Each platform implementation handles authentication and API interaction.
// Errors are typed (ErrCLINotFound, ErrPRNotFound) for caller handling.
package pr

import (
	"context"
	"errors"
	"fmt"
)

// Platform represents a supported git hosting platform.
type Platform string

const (
	PlatformGitHub    Platform = "github"
	PlatformGitLab    Platform = "gitlab"
	PlatformBitBucket Platform = "bitbucket"
)

// PR state constants for consistent state handling.
const (
	StateOpen   = "open"
	StateClosed = "closed"
	StateMerged = "merged"
)

// PRInfo contains the parsed information from a PR URL.
type PRInfo struct {
	// Platform is the hosting platform (github, gitlab, bitbucket).
	Platform Platform

	// Host is the full hostname (e.g., "github.com", "gitlab.mycompany.com").
	Host string

	// Owner is the repository owner/organization.
	Owner string

	// Repo is the repository name.
	Repo string

	// Number is the PR/MR number.
	Number int

	// OriginalURL is the original URL provided.
	OriginalURL string
}

// PRMetadata contains the fetched PR details needed for review.
type PRMetadata struct {
	PRInfo

	// Title is the PR title.
	Title string

	// BaseRef is the target branch (e.g., "main").
	BaseRef string

	// HeadRef is the source branch (e.g., "feature/foo").
	HeadRef string

	// HeadSHA is the latest commit SHA on the head branch.
	HeadSHA string

	// State is the PR state (open, closed, merged).
	State string

	// IsMerged indicates if the PR has been merged.
	IsMerged bool
}

// Resolver fetches PR metadata from a hosting platform.
type Resolver interface {
	// Platform returns the platform this resolver handles.
	Platform() Platform

	// Resolve fetches the PR metadata for the given PRInfo.
	Resolve(ctx context.Context, info *PRInfo) (*PRMetadata, error)

	// IsAvailable checks if the required CLI/credentials are available.
	IsAvailable(ctx context.Context) bool
}

// ErrCLINotFound is returned when the platform CLI is not installed.
type ErrCLINotFound struct {
	Platform Platform
	CLI      string
}

func (e *ErrCLINotFound) Error() string {
	return fmt.Sprintf("%s CLI (%s) not found or not authenticated", e.Platform, e.CLI)
}

// ErrPRNotFound is returned when the PR doesn't exist.
type ErrPRNotFound struct {
	URL string
}

func (e *ErrPRNotFound) Error() string {
	return fmt.Sprintf("pull request not found: %s", e.URL)
}

// ErrNotPRURL is returned when the input doesn't match any known PR URL pattern.
var ErrNotPRURL = errors.New("not a recognized pull request URL")

// ErrUnsupportedPlatform is returned when the platform is not yet supported.
var ErrUnsupportedPlatform = errors.New("platform not yet supported")
