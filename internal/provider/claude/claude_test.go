package claude

import (
	"testing"

	"github.com/mwistrand/graft/internal/git"
	"github.com/mwistrand/graft/internal/provider"
)

func TestNew(t *testing.T) {
	// Test with valid API key
	p, err := New("test-api-key", "")
	if err != nil {
		t.Fatalf("New() failed: %v", err)
	}

	if p.Name() != "claude" {
		t.Errorf("Name() = %q, want %q", p.Name(), "claude")
	}

	if string(p.model) != DefaultModel {
		t.Errorf("model = %q, want %q", p.model, DefaultModel)
	}
}

func TestNew_CustomModel(t *testing.T) {
	p, err := New("test-api-key", "claude-opus-4-20250514")
	if err != nil {
		t.Fatalf("New() failed: %v", err)
	}

	if string(p.model) != "claude-opus-4-20250514" {
		t.Errorf("model = %q, want %q", p.model, "claude-opus-4-20250514")
	}
}

func TestNew_NoAPIKey(t *testing.T) {
	_, err := New("", "")
	if err == nil {
		t.Error("expected error for empty API key")
	}
}

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
			got := extractJSON(tt.input)
			if got != tt.want {
				t.Errorf("extractJSON() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestParseJSONResponse(t *testing.T) {
	input := `{"overview": "Test summary", "key_changes": ["Change 1"]}`

	var resp provider.SummarizeResponse
	err := parseJSONResponse(input, &resp)
	if err != nil {
		t.Fatalf("parseJSONResponse() failed: %v", err)
	}

	if resp.Overview != "Test summary" {
		t.Errorf("Overview = %q, want %q", resp.Overview, "Test summary")
	}

	if len(resp.KeyChanges) != 1 || resp.KeyChanges[0] != "Change 1" {
		t.Errorf("KeyChanges = %v, want [\"Change 1\"]", resp.KeyChanges)
	}
}

func TestParseJSONResponse_Invalid(t *testing.T) {
	var resp provider.SummarizeResponse
	err := parseJSONResponse("not valid json", &resp)
	if err == nil {
		t.Error("expected error for invalid JSON")
	}
}

func TestBuildSummaryPrompt(t *testing.T) {
	req := &provider.SummarizeRequest{
		Files: []git.FileDiff{
			{Path: "main.go", Status: git.StatusModified, Additions: 10, Deletions: 5},
			{Path: "helper.go", Status: git.StatusAdded, Additions: 20, Deletions: 0},
		},
		Commits: []git.Commit{
			{ShortHash: "abc123", Author: "Test User", Subject: "Add feature"},
		},
		FullDiff: "+line1\n-line2",
	}

	prompt := buildSummaryPrompt(req)

	// Check that key elements are present
	if !containsString(prompt, "main.go") {
		t.Error("prompt should contain main.go")
	}
	if !containsString(prompt, "helper.go") {
		t.Error("prompt should contain helper.go")
	}
	if !containsString(prompt, "abc123") {
		t.Error("prompt should contain commit hash")
	}
	if !containsString(prompt, "Add feature") {
		t.Error("prompt should contain commit message")
	}
	if !containsString(prompt, "+line1") {
		t.Error("prompt should contain diff content")
	}
	if !containsString(prompt, "JSON") {
		t.Error("prompt should mention JSON format")
	}
}

func TestBuildSummaryPrompt_WithFocus(t *testing.T) {
	req := &provider.SummarizeRequest{
		Files: []git.FileDiff{
			{Path: "main.go", Status: git.StatusModified},
		},
		Options: provider.SummarizeOptions{
			Focus: "security",
		},
	}

	prompt := buildSummaryPrompt(req)

	if !containsString(prompt, "security") {
		t.Error("prompt should contain focus area")
	}
}

func TestBuildOrderPrompt(t *testing.T) {
	req := &provider.OrderRequest{
		Files: []git.FileDiff{
			{Path: "cmd/main.go", Status: git.StatusModified, Additions: 5, Deletions: 2},
			{Path: "internal/service.go", Status: git.StatusAdded, Additions: 50, Deletions: 0},
		},
		Commits: []git.Commit{
			{Subject: "Implement feature X"},
		},
	}

	prompt := buildOrderPrompt(req)

	// Check that key elements are present
	if !containsString(prompt, "cmd/main.go") {
		t.Error("prompt should contain cmd/main.go")
	}
	if !containsString(prompt, "internal/service.go") {
		t.Error("prompt should contain internal/service.go")
	}
	if !containsString(prompt, "Implement feature X") {
		t.Error("prompt should contain commit message")
	}
	if !containsString(prompt, "entry_point") {
		t.Error("prompt should mention category options")
	}
	if !containsString(prompt, "priority") {
		t.Error("prompt should mention priority")
	}
}

func containsString(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
