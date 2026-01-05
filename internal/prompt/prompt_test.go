package prompt

import (
	"testing"

	"github.com/mwistrand/graft/internal/provider"
)

func TestSelectModel_EmptyModels(t *testing.T) {
	_, err := SelectModel(nil)
	if err == nil {
		t.Error("expected error for empty models")
	}
	if err.Error() != "no models available" {
		t.Errorf("unexpected error: %v", err)
	}

	_, err = SelectModel([]provider.ModelInfo{})
	if err == nil {
		t.Error("expected error for empty models slice")
	}
}

func TestSelectModel_NonInteractive(t *testing.T) {
	// When running in tests, stdin is not a terminal
	models := []provider.ModelInfo{
		{ID: "model-1", Name: "Model One"},
	}

	_, err := SelectModel(models)
	if err == nil {
		t.Error("expected error for non-interactive terminal")
	}
	if err.Error() != "cannot prompt for model: not running in an interactive terminal" {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestIsInteractive_InTests(t *testing.T) {
	// In test environment, stdin is typically not a terminal
	if IsInteractive() {
		t.Skip("skipping: stdin is a terminal in this test environment")
	}
}

func TestConfirmContinue_NonInteractive(t *testing.T) {
	// In test environment, stdin is not a terminal, so should return true (continue by default)
	if IsInteractive() {
		t.Skip("skipping: stdin is a terminal in this test environment")
	}

	result := ConfirmContinue("")
	if !result {
		t.Error("expected true (continue) in non-interactive mode")
	}

	result = ConfirmContinue("Custom message")
	if !result {
		t.Error("expected true (continue) in non-interactive mode with custom message")
	}
}
