package pr

import (
	"net/url"
	"regexp"
	"strconv"
	"strings"
)

// URL patterns for each platform.
var (
	// GitHub: https://github.com/owner/repo/pull/123
	githubPattern = regexp.MustCompile(`^/([^/]+)/([^/]+)/pull/(\d+)/?$`)

	// GitLab: https://gitlab.com/owner/repo/-/merge_requests/123
	// Also supports nested groups: https://gitlab.com/group/subgroup/repo/-/merge_requests/123
	gitlabPattern = regexp.MustCompile(`^/(.+)/([^/]+)/-/merge_requests/(\d+)/?$`)

	// BitBucket: https://bitbucket.org/owner/repo/pull-requests/123
	bitbucketPattern = regexp.MustCompile(`^/([^/]+)/([^/]+)/pull-requests/(\d+)/?$`)
)

// IsPRURL returns true if the input looks like a PR URL.
func IsPRURL(input string) bool {
	return strings.HasPrefix(input, "https://") || strings.HasPrefix(input, "http://")
}

// Parse attempts to parse a URL as a pull request URL.
// Returns ErrNotPRURL if the URL doesn't match any known pattern.
func Parse(rawURL string) (*PRInfo, error) {
	u, err := url.Parse(rawURL)
	if err != nil {
		return nil, ErrNotPRURL
	}

	if u.Scheme != "https" && u.Scheme != "http" {
		return nil, ErrNotPRURL
	}

	host := strings.ToLower(u.Host)
	path := u.Path

	// Try GitHub (github.com or GitHub Enterprise)
	if strings.Contains(host, "github") {
		if matches := githubPattern.FindStringSubmatch(path); matches != nil {
			num, _ := strconv.Atoi(matches[3])
			return &PRInfo{
				Platform:    PlatformGitHub,
				Host:        host,
				Owner:       matches[1],
				Repo:        matches[2],
				Number:      num,
				OriginalURL: rawURL,
			}, nil
		}
	}

	// Try GitLab (gitlab.com or self-hosted)
	if strings.Contains(host, "gitlab") {
		if matches := gitlabPattern.FindStringSubmatch(path); matches != nil {
			num, _ := strconv.Atoi(matches[3])
			return &PRInfo{
				Platform:    PlatformGitLab,
				Host:        host,
				Owner:       matches[1],
				Repo:        matches[2],
				Number:      num,
				OriginalURL: rawURL,
			}, nil
		}
	}

	// Try BitBucket (bitbucket.org or self-hosted)
	if strings.Contains(host, "bitbucket") {
		if matches := bitbucketPattern.FindStringSubmatch(path); matches != nil {
			num, _ := strconv.Atoi(matches[3])
			return &PRInfo{
				Platform:    PlatformBitBucket,
				Host:        host,
				Owner:       matches[1],
				Repo:        matches[2],
				Number:      num,
				OriginalURL: rawURL,
			}, nil
		}
	}

	return nil, ErrNotPRURL
}
