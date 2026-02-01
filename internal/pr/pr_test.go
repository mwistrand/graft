package pr

import (
	"strings"
	"testing"
)

func TestErrCLINotFound_Error(t *testing.T) {
	err := &ErrCLINotFound{
		Platform: PlatformGitHub,
		CLI:      "gh",
	}

	msg := err.Error()

	if !strings.Contains(msg, "gh") {
		t.Errorf("error message should contain CLI name 'gh', got: %s", msg)
	}
	if !strings.Contains(msg, string(PlatformGitHub)) {
		t.Errorf("error message should contain platform 'github', got: %s", msg)
	}
}

func TestErrPRNotFound_Error(t *testing.T) {
	url := "https://github.com/owner/repo/pull/123"
	err := &ErrPRNotFound{URL: url}

	msg := err.Error()

	if !strings.Contains(msg, url) {
		t.Errorf("error message should contain URL %q, got: %s", url, msg)
	}
}
