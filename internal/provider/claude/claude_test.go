package claude

import (
	"testing"
)

func TestNew(t *testing.T) {
	// Test with valid API key and empty model (can be set later via SetModel)
	p, err := New("test-api-key", "")
	if err != nil {
		t.Fatalf("New() failed: %v", err)
	}

	if p.Name() != "claude" {
		t.Errorf("Name() = %q, want %q", p.Name(), "claude")
	}

	// Model should be empty initially when not specified
	if p.Model() != "" {
		t.Errorf("Model() = %q, want empty string", p.Model())
	}
}

func TestNew_CustomModel(t *testing.T) {
	p, err := New("test-api-key", "claude-opus-4-20250514")
	if err != nil {
		t.Fatalf("New() failed: %v", err)
	}

	if p.Model() != "claude-opus-4-20250514" {
		t.Errorf("Model() = %q, want %q", p.Model(), "claude-opus-4-20250514")
	}
}

func TestNew_NoAPIKey(t *testing.T) {
	_, err := New("", "")
	if err == nil {
		t.Error("expected error for empty API key")
	}
}

func TestListModels(t *testing.T) {
	p, err := New("test-api-key", "")
	if err != nil {
		t.Fatalf("New() failed: %v", err)
	}

	models, err := p.ListModels(nil)
	if err != nil {
		t.Fatalf("ListModels() failed: %v", err)
	}

	if len(models) == 0 {
		t.Error("ListModels() returned empty list")
	}

	// Verify the models have the expected structure
	for _, m := range models {
		if m.ID == "" {
			t.Error("model ID is empty")
		}
		if m.Name == "" {
			t.Error("model Name is empty")
		}
	}
}

func TestSetModel(t *testing.T) {
	p, err := New("test-api-key", "")
	if err != nil {
		t.Fatalf("New() failed: %v", err)
	}

	// Initially empty
	if p.Model() != "" {
		t.Errorf("initial Model() = %q, want empty", p.Model())
	}

	// Set model
	p.SetModel("claude-opus-4-5-20250514")
	if p.Model() != "claude-opus-4-5-20250514" {
		t.Errorf("Model() = %q, want %q", p.Model(), "claude-opus-4-5-20250514")
	}
}

func TestEffectiveModel(t *testing.T) {
	p, err := New("test-api-key", "default-model")
	if err != nil {
		t.Fatalf("New() failed: %v", err)
	}

	// Empty request model returns provider default
	got := p.effectiveModel("")
	if string(got) != "default-model" {
		t.Errorf("effectiveModel(\"\") = %q, want %q", got, "default-model")
	}

	// Non-empty request model overrides provider default
	got = p.effectiveModel("override-model")
	if string(got) != "override-model" {
		t.Errorf("effectiveModel(\"override-model\") = %q, want %q", got, "override-model")
	}
}
