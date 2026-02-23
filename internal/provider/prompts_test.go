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

func TestExtractJSON_CodeBlocksInValues(t *testing.T) {
	// AI review responses contain backtick code blocks inside JSON string values.
	// ExtractJSON must not confuse these with markdown code block delimiters.
	input := `{
  "summary": "Test review",
  "comments": [
    {
      "category": "design",
      "severity": "critical",
      "file": "repo.go",
      "line": 18,
      "title": "Invalid query syntax",
      "description": "The query uses CAST which is not valid.",
      "suggestion": "Remove the CAST:\n` + "```" + `java\nAND (:search IS NULL)\n` + "```" + `\nApply the same fix elsewhere."
    }
  ]
}`

	got := ExtractJSON(input)

	// Should return the full JSON, not a fragment extracted from inside the code block
	var parsed StructuredReview
	if err := ParseJSONResponse(got, &parsed); err != nil {
		t.Fatalf("ExtractJSON returned non-parseable result: %v\ngot: %s", err, got)
	}
	if parsed.Summary != "Test review" {
		t.Errorf("Summary = %q, want %q", parsed.Summary, "Test review")
	}
	if len(parsed.Comments) != 1 {
		t.Fatalf("expected 1 comment, got %d", len(parsed.Comments))
	}
	if parsed.Comments[0].Title != "Invalid query syntax" {
		t.Errorf("Title = %q, want %q", parsed.Comments[0].Title, "Invalid query syntax")
	}
}

func TestExtractJSON_TrailingText(t *testing.T) {
	// AI sometimes adds text after the JSON
	input := `{"summary": "Test", "comments": []}

I hope this review is helpful! Let me know if you have questions.`

	got := ExtractJSON(input)

	var parsed StructuredReview
	if err := ParseJSONResponse(got, &parsed); err != nil {
		t.Fatalf("ExtractJSON should handle trailing text: %v\ngot: %s", err, got)
	}
	if parsed.Summary != "Test" {
		t.Errorf("Summary = %q, want %q", parsed.Summary, "Test")
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

func TestBuildOrderPrompt_GroupingInstructions(t *testing.T) {
	req := &OrderRequest{
		Files: []git.FileDiff{
			{Path: "handler.go", Status: git.StatusModified},
			{Path: "service.go", Status: git.StatusModified},
		},
	}

	prompt := BuildOrderPrompt(req)

	// Check that grouping instructions are present
	if !strings.Contains(prompt, "groups") {
		t.Error("prompt should mention groups in JSON schema")
	}
	if !strings.Contains(prompt, "Identify features") {
		t.Error("prompt should contain grouping strategy")
	}
	if !strings.Contains(prompt, "group") {
		t.Error("prompt should mention group field for files")
	}
	if !strings.Contains(prompt, "Every file MUST have a group assigned") {
		t.Error("prompt should require group assignment")
	}
}

func TestBuildOrderPrompt_GroupJSONSchema(t *testing.T) {
	req := &OrderRequest{
		Files: []git.FileDiff{{Path: "main.go"}},
	}

	prompt := BuildOrderPrompt(req)

	// Check JSON schema contains group-related fields
	if !strings.Contains(prompt, `"groups"`) {
		t.Error("prompt JSON schema should contain groups array")
	}
	if !strings.Contains(prompt, `"name"`) {
		t.Error("prompt JSON schema should contain name field")
	}
	if !strings.Contains(prompt, `"description"`) {
		t.Error("prompt JSON schema should contain description field")
	}
}

func TestBuildOrderPrompt_SignificanceClassification(t *testing.T) {
	req := &OrderRequest{
		Files: []git.FileDiff{{Path: "main.go"}},
	}

	prompt := BuildOrderPrompt(req)

	// Check that significance is in the JSON schema
	if !strings.Contains(prompt, `"significance"`) {
		t.Error("prompt JSON schema should contain significance field")
	}
	if !strings.Contains(prompt, "core|supporting|minor") {
		t.Error("prompt should show significance options")
	}

	// Check significance classification guidance
	if !strings.Contains(prompt, "Significance Classification") {
		t.Error("prompt should contain Significance Classification section")
	}
	if !strings.Contains(prompt, "Major logic changes") {
		t.Error("prompt should describe core significance")
	}
	if !strings.Contains(prompt, "support or validate") {
		t.Error("prompt should describe supporting significance")
	}
	if !strings.Contains(prompt, "Configuration files") {
		t.Error("prompt should describe minor significance")
	}

	// Check significance is required
	if !strings.Contains(prompt, "Every group MUST have a significance level") {
		t.Error("prompt should require significance for every group")
	}
}

func TestBuildReviewPrompt(t *testing.T) {
	req := &ReviewRequest{
		Files: []git.FileDiff{
			{Path: "main.go", Status: git.StatusModified, Additions: 10, Deletions: 5},
			{Path: "helper.go", Status: git.StatusAdded, Additions: 20, Deletions: 0},
		},
		Commits: []git.Commit{
			{ShortHash: "abc123", Author: "Test User", Subject: "Add feature"},
		},
		FullDiff: "+line1\n-line2",
	}

	prompt := BuildReviewPrompt(req)

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
	if !strings.Contains(prompt, "category") {
		t.Error("prompt should mention category")
	}
	if !strings.Contains(prompt, "severity") {
		t.Error("prompt should mention severity")
	}
	if !strings.Contains(prompt, "design") {
		t.Error("prompt should mention design category")
	}
	if !strings.Contains(prompt, "functionality") {
		t.Error("prompt should mention functionality category")
	}
}

func TestBuildReviewPrompt_LargeDiffTruncation(t *testing.T) {
	largeDiff := strings.Repeat("x", 100000)
	req := &ReviewRequest{
		Files:    []git.FileDiff{{Path: "huge.go"}},
		FullDiff: largeDiff,
	}
	prompt := BuildReviewPrompt(req)

	if !strings.Contains(prompt, "... [diff truncated for length] ...") {
		t.Error("large diff should be truncated")
	}
	if strings.Contains(prompt, strings.Repeat("x", 80001)) {
		t.Error("prompt should not contain more than 80000 chars of diff")
	}
}

func TestBuildReviewPrompt_EdgeCases(t *testing.T) {
	t.Run("empty commits", func(t *testing.T) {
		req := &ReviewRequest{
			Files: []git.FileDiff{{Path: "main.go"}},
		}
		prompt := BuildReviewPrompt(req)
		if strings.Contains(prompt, "## Commits") {
			t.Error("prompt should not have Commits section when commits are empty")
		}
	})

	t.Run("renamed file", func(t *testing.T) {
		req := &ReviewRequest{
			Files: []git.FileDiff{{Path: "new.go", OldPath: "old.go", Status: git.StatusRenamed}},
		}
		prompt := BuildReviewPrompt(req)
		if !strings.Contains(prompt, "old.go") {
			t.Error("prompt should include old path for renamed files")
		}
	})

	t.Run("empty diff", func(t *testing.T) {
		req := &ReviewRequest{
			Files: []git.FileDiff{{Path: "main.go"}},
		}
		prompt := BuildReviewPrompt(req)
		if strings.Contains(prompt, "## Diff Content") {
			t.Error("prompt should not have Diff Content section when diff is empty")
		}
	})
}

func TestBuildReviewPrompt_WithCategories(t *testing.T) {
	req := &ReviewRequest{
		Files: []git.FileDiff{{Path: "main.go"}},
		Options: ReviewOptions{
			Categories: []ReviewCategory{CategoryDesign, CategoryTests},
		},
	}

	prompt := BuildReviewPrompt(req)

	if !strings.Contains(prompt, "FOCUS") {
		t.Error("prompt should contain FOCUS instruction when categories are set")
	}
	if !strings.Contains(prompt, "design") {
		t.Error("prompt should mention design category in focus")
	}
	if !strings.Contains(prompt, "tests") {
		t.Error("prompt should mention tests category in focus")
	}
}

func TestParseStructuredReview(t *testing.T) {
	t.Run("valid JSON", func(t *testing.T) {
		input := `{
			"summary": "Test summary",
			"comments": [
				{"category": "design", "severity": "critical", "title": "Issue", "description": "Details"}
			]
		}`

		resp := ParseStructuredReview(input)

		if resp.Structured == nil {
			t.Fatal("expected Structured to be set")
		}
		if resp.Structured.Summary != "Test summary" {
			t.Errorf("Summary = %q, want 'Test summary'", resp.Structured.Summary)
		}
		if len(resp.Structured.Comments) != 1 {
			t.Errorf("expected 1 comment, got %d", len(resp.Structured.Comments))
		}
		if resp.Content == "" {
			t.Error("Content should be generated for backwards compat")
		}
	})

	t.Run("invalid JSON fallback", func(t *testing.T) {
		input := "This is not valid JSON, just plain text review"

		resp := ParseStructuredReview(input)

		if resp.Structured != nil {
			t.Error("Structured should be nil for invalid JSON")
		}
		if resp.Content != input {
			t.Errorf("Content = %q, want raw input", resp.Content)
		}
	})

	t.Run("JSON in code block", func(t *testing.T) {
		input := "```json\n{\"summary\": \"Test\", \"comments\": []}\n```"

		resp := ParseStructuredReview(input)

		if resp.Structured == nil {
			t.Fatal("expected Structured to be set for code block JSON")
		}
		if resp.Structured.Summary != "Test" {
			t.Errorf("Summary = %q, want 'Test'", resp.Structured.Summary)
		}
	})
}

func TestGenerateMarkdownFromReview(t *testing.T) {
	t.Run("full review", func(t *testing.T) {
		review := &StructuredReview{
			Summary: "This is a test review.",
			Comments: []ReviewComment{
				{Category: CategoryDesign, Severity: SeverityCritical, Title: "Critical Issue", File: "main.go", Line: 42, Description: "Details here"},
				{Category: CategoryDesign, Severity: SeverityNit, Title: "Minor Thing", Description: "Nit details"},
				{Category: CategoryPraise, Severity: SeveritySuggestion, Title: "Good Job", Description: "Well done"},
			},
		}

		md := GenerateMarkdownFromReview(review)

		if !strings.Contains(md, "## Summary") {
			t.Error("should contain Summary heading")
		}
		if !strings.Contains(md, "This is a test review.") {
			t.Error("should contain summary text")
		}
		if !strings.Contains(md, "## Design") {
			t.Error("should contain Design heading")
		}
		if !strings.Contains(md, "**[CRITICAL]**") {
			t.Error("should contain critical indicator")
		}
		if !strings.Contains(md, "*[Nit]*") {
			t.Error("should contain nit indicator")
		}
		if !strings.Contains(md, "main.go:42") {
			t.Error("should contain file:line reference")
		}
		if !strings.Contains(md, "## Praise") {
			t.Error("should contain Praise heading")
		}
	})

	t.Run("with suggestion", func(t *testing.T) {
		review := &StructuredReview{
			Comments: []ReviewComment{
				{Category: CategoryDesign, Severity: SeveritySuggestion, Title: "Refactor", Suggestion: "func better() {}"},
			},
		}

		md := GenerateMarkdownFromReview(review)

		if !strings.Contains(md, "**Suggestion:**") {
			t.Error("should contain suggestion label")
		}
		if !strings.Contains(md, "func better()") {
			t.Error("should contain suggestion code")
		}
	})

	t.Run("file without line", func(t *testing.T) {
		review := &StructuredReview{
			Comments: []ReviewComment{
				{Category: CategoryDesign, Severity: SeveritySuggestion, Title: "General", File: "main.go"},
			},
		}

		md := GenerateMarkdownFromReview(review)

		if !strings.Contains(md, "`main.go`") {
			t.Error("should contain file reference without line")
		}
		if strings.Contains(md, "main.go:0") {
			t.Error("should not contain :0 for missing line")
		}
	})

	t.Run("empty summary", func(t *testing.T) {
		review := &StructuredReview{
			Comments: []ReviewComment{
				{Category: CategoryDesign, Severity: SeveritySuggestion, Title: "Issue"},
			},
		}

		md := GenerateMarkdownFromReview(review)

		if strings.Contains(md, "## Summary") {
			t.Error("should not contain Summary heading when empty")
		}
	})
}

func TestCategoryDisplayName(t *testing.T) {
	tests := []struct {
		cat  ReviewCategory
		want string
	}{
		{CategoryDesign, "Design"},
		{CategoryFunctionality, "Functionality"},
		{CategoryComplexity, "Complexity"},
		{CategoryTests, "Tests"},
		{CategoryNaming, "Naming"},
		{CategoryComments, "Comments"},
		{CategoryStyle, "Style"},
		{CategoryDocumentation, "Documentation"},
		{CategoryPraise, "Praise"},
		{ReviewCategory("unknown"), "unknown"},
	}

	for _, tt := range tests {
		t.Run(string(tt.cat), func(t *testing.T) {
			got := CategoryDisplayName(tt.cat)
			if got != tt.want {
				t.Errorf("CategoryDisplayName(%q) = %q, want %q", tt.cat, got, tt.want)
			}
		})
	}
}

func TestBuildQuickReviewPrompt(t *testing.T) {
	req := &QuickReviewRequest{
		Files: []git.FileDiff{
			{Path: "main.go", Status: git.StatusModified, Additions: 10, Deletions: 5},
			{Path: "helper.go", Status: git.StatusAdded, Additions: 20, Deletions: 0},
		},
		Commits: []git.Commit{
			{ShortHash: "abc123", Subject: "Add feature"},
		},
	}

	prompt := BuildQuickReviewPrompt(req)

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
	if !strings.Contains(prompt, "verdict") {
		t.Error("prompt should mention verdict")
	}
	if !strings.Contains(prompt, "approve") {
		t.Error("prompt should mention approve verdict")
	}
	if !strings.Contains(prompt, "blocker") {
		t.Error("prompt should mention blocker verdict")
	}
}

func TestBuildQuickReviewPrompt_EdgeCases(t *testing.T) {
	t.Run("empty commits", func(t *testing.T) {
		req := &QuickReviewRequest{
			Files: []git.FileDiff{{Path: "main.go"}},
		}
		prompt := BuildQuickReviewPrompt(req)
		if strings.Contains(prompt, "## Commits") {
			t.Error("prompt should not have Commits section when commits are empty")
		}
	})

	t.Run("renamed file", func(t *testing.T) {
		req := &QuickReviewRequest{
			Files: []git.FileDiff{{Path: "new.go", OldPath: "old.go", Status: git.StatusRenamed}},
		}
		prompt := BuildQuickReviewPrompt(req)
		if !strings.Contains(prompt, "old.go") {
			t.Error("prompt should include old path for renamed files")
		}
	})

	t.Run("totals calculated", func(t *testing.T) {
		req := &QuickReviewRequest{
			Files: []git.FileDiff{
				{Path: "a.go", Additions: 10, Deletions: 5},
				{Path: "b.go", Additions: 20, Deletions: 3},
			},
		}
		prompt := BuildQuickReviewPrompt(req)
		if !strings.Contains(prompt, "+30/-8") {
			t.Error("prompt should contain total additions/deletions")
		}
	})
}

func TestParseQuickReviewResponse(t *testing.T) {
	t.Run("valid approve", func(t *testing.T) {
		input := `{"verdict": "approve", "summary": "Looks good", "concerns": [], "proceed": true}`
		resp, err := ParseQuickReviewResponse(input)
		if err != nil {
			t.Fatalf("ParseQuickReviewResponse() failed: %v", err)
		}
		if resp.Verdict != VerdictApprove {
			t.Errorf("Verdict = %q, want %q", resp.Verdict, VerdictApprove)
		}
		if resp.Summary != "Looks good" {
			t.Errorf("Summary = %q, want 'Looks good'", resp.Summary)
		}
		if !resp.Proceed {
			t.Error("Proceed should be true for approve")
		}
	})

	t.Run("valid blocker", func(t *testing.T) {
		input := `{"verdict": "blocker", "summary": "Security issue", "concerns": ["SQL injection"], "proceed": false}`
		resp, err := ParseQuickReviewResponse(input)
		if err != nil {
			t.Fatalf("ParseQuickReviewResponse() failed: %v", err)
		}
		if resp.Verdict != VerdictBlocker {
			t.Errorf("Verdict = %q, want %q", resp.Verdict, VerdictBlocker)
		}
		if resp.Proceed {
			t.Error("Proceed should be false for blocker")
		}
		if len(resp.Concerns) != 1 || resp.Concerns[0] != "SQL injection" {
			t.Errorf("Concerns = %v, want [SQL injection]", resp.Concerns)
		}
	})

	t.Run("verdict normalization", func(t *testing.T) {
		input := `{"verdict": "APPROVE", "summary": "Test"}`
		resp, err := ParseQuickReviewResponse(input)
		if err != nil {
			t.Fatalf("ParseQuickReviewResponse() failed: %v", err)
		}
		if resp.Verdict != VerdictApprove {
			t.Errorf("Verdict = %q, want %q (should normalize case)", resp.Verdict, VerdictApprove)
		}
	})

	t.Run("unknown verdict defaults to concerns", func(t *testing.T) {
		input := `{"verdict": "unknown", "summary": "Test"}`
		resp, err := ParseQuickReviewResponse(input)
		if err != nil {
			t.Fatalf("ParseQuickReviewResponse() failed: %v", err)
		}
		if resp.Verdict != VerdictConcerns {
			t.Errorf("Verdict = %q, want %q (should default to concerns)", resp.Verdict, VerdictConcerns)
		}
	})

	t.Run("invalid JSON", func(t *testing.T) {
		_, err := ParseQuickReviewResponse("not valid json")
		if err == nil {
			t.Error("expected error for invalid JSON")
		}
	})

	t.Run("JSON in code block", func(t *testing.T) {
		input := "```json\n{\"verdict\": \"concerns\", \"summary\": \"Needs review\"}\n```"
		resp, err := ParseQuickReviewResponse(input)
		if err != nil {
			t.Fatalf("ParseQuickReviewResponse() failed: %v", err)
		}
		if resp.Verdict != VerdictConcerns {
			t.Errorf("Verdict = %q, want %q", resp.Verdict, VerdictConcerns)
		}
	})
}
