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
