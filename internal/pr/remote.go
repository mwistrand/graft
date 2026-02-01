package pr

import (
	"regexp"
	"strings"
)

// RemoteInfo contains parsed information from a git remote URL.
type RemoteInfo struct {
	Host  string
	Owner string
	Repo  string
}

// Patterns for parsing git remote URLs.
var (
	// SSH: git@github.com:owner/repo.git or git@github.com:owner/repo
	sshPattern = regexp.MustCompile(`^git@([^:]+):(.+)/([^/]+?)(?:\.git)?$`)

	// HTTPS: https://github.com/owner/repo.git or https://github.com/owner/repo
	httpsPattern = regexp.MustCompile(`^https?://([^/]+)/(.+)/([^/]+?)(?:\.git)?$`)

	// Git protocol: git://github.com/owner/repo.git
	gitPattern = regexp.MustCompile(`^git://([^/]+)/(.+)/([^/]+?)(?:\.git)?$`)
)

// ParseRemoteURL parses a git remote URL and extracts host, owner, and repo.
// Supports SSH (git@host:owner/repo), HTTPS, and git:// protocols.
func ParseRemoteURL(remoteURL string) (*RemoteInfo, error) {
	remoteURL = strings.TrimSpace(remoteURL)

	// Try SSH format: git@github.com:owner/repo.git
	if matches := sshPattern.FindStringSubmatch(remoteURL); matches != nil {
		return &RemoteInfo{
			Host:  strings.ToLower(matches[1]),
			Owner: matches[2],
			Repo:  matches[3],
		}, nil
	}

	// Try HTTPS format: https://github.com/owner/repo.git
	if matches := httpsPattern.FindStringSubmatch(remoteURL); matches != nil {
		return &RemoteInfo{
			Host:  strings.ToLower(matches[1]),
			Owner: matches[2],
			Repo:  matches[3],
		}, nil
	}

	// Try git:// format: git://github.com/owner/repo.git
	if matches := gitPattern.FindStringSubmatch(remoteURL); matches != nil {
		return &RemoteInfo{
			Host:  strings.ToLower(matches[1]),
			Owner: matches[2],
			Repo:  matches[3],
		}, nil
	}

	return nil, ErrNotPRURL
}

// Matches checks if this remote info matches the given PR info.
// Compares host (normalized), owner, and repo (case-insensitive).
func (r *RemoteInfo) Matches(info *PRInfo) bool {
	// Normalize host comparison (e.g., "github.com" matches "github.com")
	if !hostMatches(r.Host, info.Host) {
		return false
	}

	// Compare owner and repo (case-insensitive)
	if !strings.EqualFold(r.Owner, info.Owner) {
		return false
	}

	if !strings.EqualFold(r.Repo, info.Repo) {
		return false
	}

	return true
}

// hostMatches checks if two hosts are equivalent.
func hostMatches(h1, h2 string) bool {
	h1 = strings.ToLower(h1)
	h2 = strings.ToLower(h2)

	// Direct match
	if h1 == h2 {
		return true
	}

	// Handle www prefix variations
	h1 = strings.TrimPrefix(h1, "www.")
	h2 = strings.TrimPrefix(h2, "www.")

	return h1 == h2
}
