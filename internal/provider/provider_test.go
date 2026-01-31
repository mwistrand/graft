package provider

import (
	"testing"
)

func TestDefaultSummarizeOptions(t *testing.T) {
	opts := DefaultSummarizeOptions()

	if opts.MaxTokens != 2048 {
		t.Errorf("MaxTokens = %d, want 2048", opts.MaxTokens)
	}

	if opts.Temperature != 0.3 {
		t.Errorf("Temperature = %f, want 0.3", opts.Temperature)
	}

	if opts.Focus != "" {
		t.Errorf("Focus = %q, want empty", opts.Focus)
	}
}

func TestCategoryConstants(t *testing.T) {
	// Verify category constants are unique and non-empty
	categories := []string{
		CategoryEntryPoint,
		CategoryBusinessLogic,
		CategoryAdapter,
		CategoryModel,
		CategoryConfig,
		CategoryTest,
		CategoryDocs,
		CategoryOther,
	}

	seen := make(map[string]bool)
	for _, c := range categories {
		if c == "" {
			t.Error("category constant should not be empty")
		}
		if seen[c] {
			t.Errorf("duplicate category: %s", c)
		}
		seen[c] = true
	}
}

func TestParseReviewCategories(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected []ReviewCategory
	}{
		{
			name:     "empty string",
			input:    "",
			expected: nil,
		},
		{
			name:     "single category",
			input:    "design",
			expected: []ReviewCategory{CategoryDesign},
		},
		{
			name:     "multiple categories",
			input:    "design,functionality,tests",
			expected: []ReviewCategory{CategoryDesign, CategoryFunctionality, CategoryTests},
		},
		{
			name:     "with whitespace",
			input:    "design , functionality , tests",
			expected: []ReviewCategory{CategoryDesign, CategoryFunctionality, CategoryTests},
		},
		{
			name:     "invalid category ignored",
			input:    "design,invalid,tests",
			expected: []ReviewCategory{CategoryDesign, CategoryTests},
		},
		{
			name:     "all categories",
			input:    "design,functionality,complexity,tests,naming,comments,style,documentation,praise",
			expected: []ReviewCategory{CategoryDesign, CategoryFunctionality, CategoryComplexity, CategoryTests, CategoryNaming, CategoryComments, CategoryStyle, CategoryDocumentation, CategoryPraise},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ParseReviewCategories(tt.input)
			if len(result) != len(tt.expected) {
				t.Errorf("ParseReviewCategories(%q) = %v, want %v", tt.input, result, tt.expected)
				return
			}
			for i, cat := range result {
				if cat != tt.expected[i] {
					t.Errorf("ParseReviewCategories(%q)[%d] = %v, want %v", tt.input, i, cat, tt.expected[i])
				}
			}
		})
	}
}

func TestParseReviewSeverity(t *testing.T) {
	tests := []struct {
		input    string
		expected ReviewSeverity
	}{
		{"critical", SeverityCritical},
		{"suggestion", SeveritySuggestion},
		{"nit", SeverityNit},
		{"", ""},
		{"invalid", ""},
		{"CRITICAL", ""}, // case sensitive
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			result := ParseReviewSeverity(tt.input)
			if result != tt.expected {
				t.Errorf("ParseReviewSeverity(%q) = %q, want %q", tt.input, result, tt.expected)
			}
		})
	}
}

func TestStructuredReview_FilterBySeverity(t *testing.T) {
	review := &StructuredReview{
		Summary: "Test summary",
		Comments: []ReviewComment{
			{Category: CategoryDesign, Severity: SeverityCritical, Title: "Critical"},
			{Category: CategoryFunctionality, Severity: SeveritySuggestion, Title: "Suggestion"},
			{Category: CategoryStyle, Severity: SeverityNit, Title: "Nit"},
		},
	}

	t.Run("no filter", func(t *testing.T) {
		filtered := review.FilterBySeverity("")
		if len(filtered.Comments) != 3 {
			t.Errorf("expected 3 comments, got %d", len(filtered.Comments))
		}
	})

	t.Run("filter critical", func(t *testing.T) {
		filtered := review.FilterBySeverity(SeverityCritical)
		if len(filtered.Comments) != 1 {
			t.Errorf("expected 1 comment, got %d", len(filtered.Comments))
		}
		if filtered.Comments[0].Title != "Critical" {
			t.Errorf("expected Critical, got %s", filtered.Comments[0].Title)
		}
	})

	t.Run("filter suggestion", func(t *testing.T) {
		filtered := review.FilterBySeverity(SeveritySuggestion)
		if len(filtered.Comments) != 2 {
			t.Errorf("expected 2 comments, got %d", len(filtered.Comments))
		}
	})

	t.Run("filter nit", func(t *testing.T) {
		filtered := review.FilterBySeverity(SeverityNit)
		if len(filtered.Comments) != 3 {
			t.Errorf("expected 3 comments, got %d", len(filtered.Comments))
		}
	})

	t.Run("preserves summary", func(t *testing.T) {
		filtered := review.FilterBySeverity(SeverityCritical)
		if filtered.Summary != "Test summary" {
			t.Errorf("expected summary to be preserved")
		}
	})
}

func TestStructuredReview_CommentsByCategory(t *testing.T) {
	t.Run("groups comments correctly", func(t *testing.T) {
		review := &StructuredReview{
			Comments: []ReviewComment{
				{Category: CategoryDesign, Title: "Design1"},
				{Category: CategoryDesign, Title: "Design2"},
				{Category: CategoryTests, Title: "Test1"},
				{Category: CategoryPraise, Title: "Praise1"},
			},
		}

		byCategory := review.CommentsByCategory()

		if len(byCategory[CategoryDesign]) != 2 {
			t.Errorf("expected 2 design comments, got %d", len(byCategory[CategoryDesign]))
		}
		if len(byCategory[CategoryTests]) != 1 {
			t.Errorf("expected 1 test comment, got %d", len(byCategory[CategoryTests]))
		}
		if len(byCategory[CategoryPraise]) != 1 {
			t.Errorf("expected 1 praise comment, got %d", len(byCategory[CategoryPraise]))
		}
		if len(byCategory[CategoryStyle]) != 0 {
			t.Errorf("expected 0 style comments, got %d", len(byCategory[CategoryStyle]))
		}
	})

	t.Run("empty comments", func(t *testing.T) {
		review := &StructuredReview{Comments: []ReviewComment{}}

		byCategory := review.CommentsByCategory()

		if len(byCategory) != 0 {
			t.Errorf("expected empty map, got %d entries", len(byCategory))
		}
	})
}

func TestStructuredReview_CountBySeverity(t *testing.T) {
	t.Run("counts correctly", func(t *testing.T) {
		review := &StructuredReview{
			Comments: []ReviewComment{
				{Severity: SeverityCritical},
				{Severity: SeverityCritical},
				{Severity: SeveritySuggestion},
				{Severity: SeverityNit},
				{Severity: SeverityNit},
				{Severity: SeverityNit},
			},
		}

		counts := review.CountBySeverity()

		if counts[SeverityCritical] != 2 {
			t.Errorf("expected 2 critical, got %d", counts[SeverityCritical])
		}
		if counts[SeveritySuggestion] != 1 {
			t.Errorf("expected 1 suggestion, got %d", counts[SeveritySuggestion])
		}
		if counts[SeverityNit] != 3 {
			t.Errorf("expected 3 nits, got %d", counts[SeverityNit])
		}
	})

	t.Run("empty comments", func(t *testing.T) {
		review := &StructuredReview{Comments: []ReviewComment{}}

		counts := review.CountBySeverity()

		if len(counts) != 0 {
			t.Errorf("expected empty map, got %d entries", len(counts))
		}
	})
}

func TestAllReviewCategories(t *testing.T) {
	categories := AllReviewCategories()

	if len(categories) != 9 {
		t.Errorf("expected 9 categories, got %d", len(categories))
	}

	// Verify order
	expected := []ReviewCategory{
		CategoryDesign,
		CategoryFunctionality,
		CategoryComplexity,
		CategoryTests,
		CategoryNaming,
		CategoryComments,
		CategoryStyle,
		CategoryDocumentation,
		CategoryPraise,
	}

	for i, cat := range categories {
		if cat != expected[i] {
			t.Errorf("category[%d] = %q, want %q", i, cat, expected[i])
		}
	}
}
