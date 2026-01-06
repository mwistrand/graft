package claude

import (
	"testing"
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
