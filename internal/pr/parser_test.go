package pr

import (
	"testing"
)

func TestIsPRURL(t *testing.T) {
	tests := []struct {
		input string
		want  bool
	}{
		{"https://github.com/owner/repo/pull/123", true},
		{"http://github.com/owner/repo/pull/123", true},
		{"main", false},
		{"origin/main", false},
		{"HEAD~5", false},
		{"", false},
		{"github.com/owner/repo/pull/123", false}, // missing scheme
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := IsPRURL(tt.input)
			if got != tt.want {
				t.Errorf("IsPRURL(%q) = %v, want %v", tt.input, got, tt.want)
			}
		})
	}
}

func TestParse_GitHub(t *testing.T) {
	tests := []struct {
		name    string
		url     string
		want    *PRInfo
		wantErr bool
	}{
		{
			name: "standard github URL",
			url:  "https://github.com/owner/repo/pull/123",
			want: &PRInfo{
				Platform:    PlatformGitHub,
				Host:        "github.com",
				Owner:       "owner",
				Repo:        "repo",
				Number:      123,
				OriginalURL: "https://github.com/owner/repo/pull/123",
			},
		},
		{
			name: "github URL with trailing slash",
			url:  "https://github.com/owner/repo/pull/456/",
			want: &PRInfo{
				Platform:    PlatformGitHub,
				Host:        "github.com",
				Owner:       "owner",
				Repo:        "repo",
				Number:      456,
				OriginalURL: "https://github.com/owner/repo/pull/456/",
			},
		},
		{
			name: "github enterprise URL",
			url:  "https://github.mycompany.com/team/project/pull/789",
			want: &PRInfo{
				Platform:    PlatformGitHub,
				Host:        "github.mycompany.com",
				Owner:       "team",
				Repo:        "project",
				Number:      789,
				OriginalURL: "https://github.mycompany.com/team/project/pull/789",
			},
		},
		{
			name: "github URL with http",
			url:  "http://github.com/owner/repo/pull/1",
			want: &PRInfo{
				Platform:    PlatformGitHub,
				Host:        "github.com",
				Owner:       "owner",
				Repo:        "repo",
				Number:      1,
				OriginalURL: "http://github.com/owner/repo/pull/1",
			},
		},
		{
			name:    "github URL without PR number",
			url:     "https://github.com/owner/repo/pull",
			wantErr: true,
		},
		{
			name:    "github URL with invalid PR number",
			url:     "https://github.com/owner/repo/pull/abc",
			wantErr: true,
		},
		{
			name:    "github issues URL (not a PR)",
			url:     "https://github.com/owner/repo/issues/123",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := Parse(tt.url)
			if (err != nil) != tt.wantErr {
				t.Errorf("Parse() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if tt.wantErr {
				return
			}

			if got.Platform != tt.want.Platform {
				t.Errorf("Platform = %v, want %v", got.Platform, tt.want.Platform)
			}
			if got.Host != tt.want.Host {
				t.Errorf("Host = %v, want %v", got.Host, tt.want.Host)
			}
			if got.Owner != tt.want.Owner {
				t.Errorf("Owner = %v, want %v", got.Owner, tt.want.Owner)
			}
			if got.Repo != tt.want.Repo {
				t.Errorf("Repo = %v, want %v", got.Repo, tt.want.Repo)
			}
			if got.Number != tt.want.Number {
				t.Errorf("Number = %v, want %v", got.Number, tt.want.Number)
			}
			if got.OriginalURL != tt.want.OriginalURL {
				t.Errorf("OriginalURL = %v, want %v", got.OriginalURL, tt.want.OriginalURL)
			}
		})
	}
}

func TestParse_GitLab(t *testing.T) {
	tests := []struct {
		name    string
		url     string
		want    *PRInfo
		wantErr bool
	}{
		{
			name: "standard gitlab URL",
			url:  "https://gitlab.com/owner/repo/-/merge_requests/123",
			want: &PRInfo{
				Platform:    PlatformGitLab,
				Host:        "gitlab.com",
				Owner:       "owner",
				Repo:        "repo",
				Number:      123,
				OriginalURL: "https://gitlab.com/owner/repo/-/merge_requests/123",
			},
		},
		{
			name: "gitlab URL with nested groups",
			url:  "https://gitlab.com/group/subgroup/repo/-/merge_requests/456",
			want: &PRInfo{
				Platform:    PlatformGitLab,
				Host:        "gitlab.com",
				Owner:       "group/subgroup",
				Repo:        "repo",
				Number:      456,
				OriginalURL: "https://gitlab.com/group/subgroup/repo/-/merge_requests/456",
			},
		},
		{
			name: "self-hosted gitlab URL",
			url:  "https://gitlab.mycompany.com/team/project/-/merge_requests/789",
			want: &PRInfo{
				Platform:    PlatformGitLab,
				Host:        "gitlab.mycompany.com",
				Owner:       "team",
				Repo:        "project",
				Number:      789,
				OriginalURL: "https://gitlab.mycompany.com/team/project/-/merge_requests/789",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := Parse(tt.url)
			if (err != nil) != tt.wantErr {
				t.Errorf("Parse() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if tt.wantErr {
				return
			}

			if got.Platform != tt.want.Platform {
				t.Errorf("Platform = %v, want %v", got.Platform, tt.want.Platform)
			}
			if got.Host != tt.want.Host {
				t.Errorf("Host = %v, want %v", got.Host, tt.want.Host)
			}
			if got.Owner != tt.want.Owner {
				t.Errorf("Owner = %v, want %v", got.Owner, tt.want.Owner)
			}
			if got.Repo != tt.want.Repo {
				t.Errorf("Repo = %v, want %v", got.Repo, tt.want.Repo)
			}
			if got.Number != tt.want.Number {
				t.Errorf("Number = %v, want %v", got.Number, tt.want.Number)
			}
		})
	}
}

func TestParse_BitBucket(t *testing.T) {
	tests := []struct {
		name    string
		url     string
		want    *PRInfo
		wantErr bool
	}{
		{
			name: "standard bitbucket URL",
			url:  "https://bitbucket.org/owner/repo/pull-requests/123",
			want: &PRInfo{
				Platform:    PlatformBitBucket,
				Host:        "bitbucket.org",
				Owner:       "owner",
				Repo:        "repo",
				Number:      123,
				OriginalURL: "https://bitbucket.org/owner/repo/pull-requests/123",
			},
		},
		{
			name: "bitbucket URL with trailing slash",
			url:  "https://bitbucket.org/owner/repo/pull-requests/456/",
			want: &PRInfo{
				Platform:    PlatformBitBucket,
				Host:        "bitbucket.org",
				Owner:       "owner",
				Repo:        "repo",
				Number:      456,
				OriginalURL: "https://bitbucket.org/owner/repo/pull-requests/456/",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := Parse(tt.url)
			if (err != nil) != tt.wantErr {
				t.Errorf("Parse() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if tt.wantErr {
				return
			}

			if got.Platform != tt.want.Platform {
				t.Errorf("Platform = %v, want %v", got.Platform, tt.want.Platform)
			}
			if got.Host != tt.want.Host {
				t.Errorf("Host = %v, want %v", got.Host, tt.want.Host)
			}
			if got.Owner != tt.want.Owner {
				t.Errorf("Owner = %v, want %v", got.Owner, tt.want.Owner)
			}
			if got.Repo != tt.want.Repo {
				t.Errorf("Repo = %v, want %v", got.Repo, tt.want.Repo)
			}
			if got.Number != tt.want.Number {
				t.Errorf("Number = %v, want %v", got.Number, tt.want.Number)
			}
		})
	}
}

func TestParse_NotPRURL(t *testing.T) {
	notPRURLs := []string{
		"main",
		"origin/main",
		"HEAD~5",
		"abc123def",
		"",
		"https://google.com",
		"https://github.com/owner/repo",           // no PR path
		"https://github.com/owner/repo/issues/1",  // issues, not PR
		"https://github.com/owner/repo/commits/x", // commits, not PR
	}

	for _, url := range notPRURLs {
		t.Run(url, func(t *testing.T) {
			_, err := Parse(url)
			if err != ErrNotPRURL {
				t.Errorf("Parse(%q) = %v, want ErrNotPRURL", url, err)
			}
		})
	}
}

func TestParse_EnterpriseNonPRPaths(t *testing.T) {
	// Enterprise hosts with non-PR paths should return ErrNotPRURL
	notPRURLs := []string{
		"https://github.mycompany.com/owner/repo",              // no PR path
		"https://github.mycompany.com/owner/repo/issues/123",   // issues, not PR
		"https://github.mycompany.com/owner/repo/commits/abc",  // commits, not PR
		"https://github.mycompany.com/owner/repo/tree/main",    // tree view
		"https://github.mycompany.com/owner/repo/blob/main/f",  // blob view
		"https://gitlab.mycompany.com/owner/repo/issues/123",   // GitLab issues
		"https://gitlab.mycompany.com/owner/repo/commits/abc",  // GitLab commits
		"https://gitlab.mycompany.com/owner/repo/pipelines/1",  // GitLab pipelines
		"https://bitbucket.mycompany.com/owner/repo/commits",   // BitBucket commits
		"https://bitbucket.mycompany.com/owner/repo/branches",  // BitBucket branches
	}

	for _, url := range notPRURLs {
		t.Run(url, func(t *testing.T) {
			_, err := Parse(url)
			if err != ErrNotPRURL {
				t.Errorf("Parse(%q) = %v, want ErrNotPRURL", url, err)
			}
		})
	}
}

func TestParseRemoteURL(t *testing.T) {
	tests := []struct {
		name    string
		url     string
		want    *RemoteInfo
		wantErr bool
	}{
		{
			name: "SSH format",
			url:  "git@github.com:owner/repo.git",
			want: &RemoteInfo{Host: "github.com", Owner: "owner", Repo: "repo"},
		},
		{
			name: "SSH format without .git",
			url:  "git@github.com:owner/repo",
			want: &RemoteInfo{Host: "github.com", Owner: "owner", Repo: "repo"},
		},
		{
			name: "HTTPS format",
			url:  "https://github.com/owner/repo.git",
			want: &RemoteInfo{Host: "github.com", Owner: "owner", Repo: "repo"},
		},
		{
			name: "HTTPS format without .git",
			url:  "https://github.com/owner/repo",
			want: &RemoteInfo{Host: "github.com", Owner: "owner", Repo: "repo"},
		},
		{
			name: "enterprise SSH",
			url:  "git@github.mycompany.com:team/project.git",
			want: &RemoteInfo{Host: "github.mycompany.com", Owner: "team", Repo: "project"},
		},
		{
			name: "enterprise HTTPS",
			url:  "https://github.mycompany.com/team/project.git",
			want: &RemoteInfo{Host: "github.mycompany.com", Owner: "team", Repo: "project"},
		},
		{
			name: "GitLab nested groups SSH",
			url:  "git@gitlab.com:group/subgroup/repo.git",
			want: &RemoteInfo{Host: "gitlab.com", Owner: "group/subgroup", Repo: "repo"},
		},
		{
			name: "GitLab nested groups HTTPS",
			url:  "https://gitlab.com/group/subgroup/repo.git",
			want: &RemoteInfo{Host: "gitlab.com", Owner: "group/subgroup", Repo: "repo"},
		},
		{
			name: "git protocol",
			url:  "git://github.com/owner/repo.git",
			want: &RemoteInfo{Host: "github.com", Owner: "owner", Repo: "repo"},
		},
		{
			name:    "invalid URL",
			url:     "not-a-url",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ParseRemoteURL(tt.url)
			if (err != nil) != tt.wantErr {
				t.Errorf("ParseRemoteURL() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if tt.wantErr {
				return
			}

			if got.Host != tt.want.Host {
				t.Errorf("Host = %v, want %v", got.Host, tt.want.Host)
			}
			if got.Owner != tt.want.Owner {
				t.Errorf("Owner = %v, want %v", got.Owner, tt.want.Owner)
			}
			if got.Repo != tt.want.Repo {
				t.Errorf("Repo = %v, want %v", got.Repo, tt.want.Repo)
			}
		})
	}
}

func TestRemoteInfo_Matches(t *testing.T) {
	tests := []struct {
		name   string
		remote *RemoteInfo
		prInfo *PRInfo
		want   bool
	}{
		{
			name:   "exact match",
			remote: &RemoteInfo{Host: "github.com", Owner: "owner", Repo: "repo"},
			prInfo: &PRInfo{Host: "github.com", Owner: "owner", Repo: "repo"},
			want:   true,
		},
		{
			name:   "case insensitive match",
			remote: &RemoteInfo{Host: "github.com", Owner: "Owner", Repo: "Repo"},
			prInfo: &PRInfo{Host: "github.com", Owner: "owner", Repo: "repo"},
			want:   true,
		},
		{
			name:   "different repo",
			remote: &RemoteInfo{Host: "github.com", Owner: "owner", Repo: "other"},
			prInfo: &PRInfo{Host: "github.com", Owner: "owner", Repo: "repo"},
			want:   false,
		},
		{
			name:   "different owner",
			remote: &RemoteInfo{Host: "github.com", Owner: "other", Repo: "repo"},
			prInfo: &PRInfo{Host: "github.com", Owner: "owner", Repo: "repo"},
			want:   false,
		},
		{
			name:   "different host",
			remote: &RemoteInfo{Host: "github.mycompany.com", Owner: "owner", Repo: "repo"},
			prInfo: &PRInfo{Host: "github.com", Owner: "owner", Repo: "repo"},
			want:   false,
		},
		{
			name:   "www prefix handling",
			remote: &RemoteInfo{Host: "www.github.com", Owner: "owner", Repo: "repo"},
			prInfo: &PRInfo{Host: "github.com", Owner: "owner", Repo: "repo"},
			want:   true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.remote.Matches(tt.prInfo)
			if got != tt.want {
				t.Errorf("Matches() = %v, want %v", got, tt.want)
			}
		})
	}
}
