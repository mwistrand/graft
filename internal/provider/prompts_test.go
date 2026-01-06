package provider

import (
	"strings"
	"testing"

	"github.com/mwistrand/graft/internal/git"
)

func TestExtractJSON(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{
			name:  "raw JSON",
			input: `{"key": "value"}`,
			want:  `{"key": "value"}`,
		},
		{
			name:  "JSON in code block",
			input: "```json\n{\"key\": \"value\"}\n```",
			want:  `{"key": "value"}`,
		},
		{
			name:  "JSON in generic code block",
			input: "```\n{\"key\": \"value\"}\n```",
			want:  `{"key": "value"}`,
		},
		{
			name:  "JSON with leading text",
			input: "Here is the response:\n{\"key\": \"value\"}",
			want:  `{"key": "value"}`,
		},
		{
			name:  "JSON array",
			input: `[1, 2, 3]`,
			want:  `[1, 2, 3]`,
		},
		{
			name:  "whitespace padded",
			input: "  \n  {\"key\": \"value\"}  \n  ",
			want:  `{"key": "value"}`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ExtractJSON(tt.input)
			if got != tt.want {
				t.Errorf("ExtractJSON() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestParseJSONResponse(t *testing.T) {
	input := `{"overview": "Test summary", "key_changes": ["Change 1"]}`

	var resp SummarizeResponse
	err := ParseJSONResponse(input, &resp)
	if err != nil {
		t.Fatalf("ParseJSONResponse() failed: %v", err)
	}

	if resp.Overview != "Test summary" {
		t.Errorf("Overview = %q, want %q", resp.Overview, "Test summary")
	}

	if len(resp.KeyChanges) != 1 || resp.KeyChanges[0] != "Change 1" {
		t.Errorf("KeyChanges = %v, want [\"Change 1\"]", resp.KeyChanges)
	}
}

func TestParseJSONResponse_Invalid(t *testing.T) {
	var resp SummarizeResponse
	err := ParseJSONResponse("not valid json", &resp)
	if err == nil {
		t.Error("expected error for invalid JSON")
	}
}

func TestBuildSummaryPrompt(t *testing.T) {
	req := &SummarizeRequest{
		Files: []git.FileDiff{
			{Path: "main.go", Status: git.StatusModified, Additions: 10, Deletions: 5},
			{Path: "helper.go", Status: git.StatusAdded, Additions: 20, Deletions: 0},
		},
		Commits: []git.Commit{
			{ShortHash: "abc123", Author: "Test User", Subject: "Add feature"},
		},
		FullDiff: "+line1\n-line2",
	}

	prompt := BuildSummaryPrompt(req)

	// Check that key elements are present
	if !strings.Contains(prompt, "main.go") {
		t.Error("prompt should contain main.go")
	}
	if !strings.Contains(prompt, "helper.go") {
		t.Error("prompt should contain helper.go")
	}
	if !strings.Contains(prompt, "abc123") {
		t.Error("prompt should contain commit hash")
	}
	if !strings.Contains(prompt, "Add feature") {
		t.Error("prompt should contain commit message")
	}
	if !strings.Contains(prompt, "+line1") {
		t.Error("prompt should contain diff content")
	}
	if !strings.Contains(prompt, "JSON") {
		t.Error("prompt should mention JSON format")
	}
}

func TestBuildSummaryPrompt_WithFocus(t *testing.T) {
	req := &SummarizeRequest{
		Files: []git.FileDiff{
			{Path: "main.go", Status: git.StatusModified},
		},
		Options: SummarizeOptions{
			Focus: "security",
		},
	}

	prompt := BuildSummaryPrompt(req)

	if !strings.Contains(prompt, "security") {
		t.Error("prompt should contain focus area")
	}
}

func TestBuildSummaryPrompt_EdgeCases(t *testing.T) {
	t.Run("empty commits", func(t *testing.T) {
		req := &SummarizeRequest{
			Files: []git.FileDiff{{Path: "main.go"}},
		}
		prompt := BuildSummaryPrompt(req)
		if strings.Contains(prompt, "## Commits") {
			t.Error("prompt should not have Commits section when commits are empty")
		}
	})

	t.Run("commit with body", func(t *testing.T) {
		req := &SummarizeRequest{
			Files: []git.FileDiff{{Path: "main.go"}},
			Commits: []git.Commit{
				{ShortHash: "abc", Author: "Test", Subject: "Subject", Body: "Detailed body"},
			},
		}
		prompt := BuildSummaryPrompt(req)
		if !strings.Contains(prompt, "Detailed body") {
			t.Error("prompt should include commit body")
		}
	})

	t.Run("renamed file", func(t *testing.T) {
		req := &SummarizeRequest{
			Files: []git.FileDiff{{Path: "new.go", OldPath: "old.go", Status: git.StatusRenamed}},
		}
		prompt := BuildSummaryPrompt(req)
		if !strings.Contains(prompt, "old.go") {
			t.Error("prompt should include old path for renamed files")
		}
	})

	t.Run("with focus", func(t *testing.T) {
		req := &SummarizeRequest{
			Files:   []git.FileDiff{{Path: "main.go"}},
			Options: SummarizeOptions{Focus: "security implications"},
		}
		prompt := BuildSummaryPrompt(req)
		if !strings.Contains(prompt, "security implications") {
			t.Error("prompt should include focus area")
		}
	})

	t.Run("large diff truncation", func(t *testing.T) {
		largeDiff := strings.Repeat("x", 60000)
		req := &SummarizeRequest{
			Files:    []git.FileDiff{{Path: "huge.go"}},
			FullDiff: largeDiff,
		}
		prompt := BuildSummaryPrompt(req)
		if !strings.Contains(prompt, "... [diff truncated for length] ...") {
			t.Error("large diff should be truncated")
		}
		if strings.Contains(prompt, strings.Repeat("x", 50001)) {
			t.Error("prompt should not contain more than 50000 chars of diff")
		}
	})
}

func TestBuildOrderPrompt(t *testing.T) {
	req := &OrderRequest{
		Files: []git.FileDiff{
			{Path: "cmd/main.go", Status: git.StatusModified, Additions: 5, Deletions: 2},
			{Path: "internal/service.go", Status: git.StatusAdded, Additions: 50, Deletions: 0},
		},
		Commits: []git.Commit{
			{Subject: "Implement feature X"},
		},
	}

	prompt := BuildOrderPrompt(req)

	// Check that key elements are present
	if !strings.Contains(prompt, "cmd/main.go") {
		t.Error("prompt should contain cmd/main.go")
	}
	if !strings.Contains(prompt, "internal/service.go") {
		t.Error("prompt should contain internal/service.go")
	}
	if !strings.Contains(prompt, "Implement feature X") {
		t.Error("prompt should contain commit message")
	}
	if !strings.Contains(prompt, "entry_point") {
		t.Error("prompt should mention category options")
	}
	if !strings.Contains(prompt, "priority") {
		t.Error("prompt should mention priority")
	}
}

func TestBuildOrderPrompt_WithRepoContext(t *testing.T) {
	req := &OrderRequest{
		Files: []git.FileDiff{
			{Path: "main.go", Status: git.StatusModified},
		},
		RepoContext: "Project Type: backend\nFrameworks: gin, gorm",
	}

	prompt := BuildOrderPrompt(req)

	if !strings.Contains(prompt, "Repository Context") {
		t.Error("prompt should contain Repository Context header")
	}
	if !strings.Contains(prompt, "gin, gorm") {
		t.Error("prompt should contain repo context content")
	}
}

func TestBuildOrderPrompt_TestsFirst(t *testing.T) {
	req := &OrderRequest{
		Files: []git.FileDiff{
			{Path: "main.go", Status: git.StatusModified},
		},
		TestsFirst: true,
	}

	prompt := BuildOrderPrompt(req)

	if !strings.Contains(prompt, "tests-first ordering") {
		t.Error("prompt should mention tests-first ordering")
	}
	if !strings.Contains(prompt, "BEGINNING") {
		t.Error("prompt should emphasize placing tests at beginning")
	}
}

func TestBuildOrderPrompt_EdgeCases(t *testing.T) {
	t.Run("empty commits", func(t *testing.T) {
		req := &OrderRequest{
			Files: []git.FileDiff{{Path: "main.go"}},
		}
		prompt := BuildOrderPrompt(req)
		if strings.Contains(prompt, "Brief Context from Commits") {
			t.Error("prompt should not have Commits section when commits are empty")
		}
	})

	t.Run("renamed file", func(t *testing.T) {
		req := &OrderRequest{
			Files: []git.FileDiff{{Path: "new.go", OldPath: "old.go", Status: git.StatusRenamed}},
		}
		prompt := BuildOrderPrompt(req)
		if !strings.Contains(prompt, "old.go") {
			t.Error("prompt should include old path for renamed files")
		}
	})
}
