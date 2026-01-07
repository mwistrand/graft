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

func TestSelectGroups_EmptyGroups(t *testing.T) {
	_, err := SelectGroups(nil, nil)
	if err == nil {
		t.Error("expected error for empty groups")
	}
	if err.Error() != "no groups available" {
		t.Errorf("unexpected error: %v", err)
	}

	_, err = SelectGroups([]provider.OrderGroup{}, nil)
	if err == nil {
		t.Error("expected error for empty groups slice")
	}
}

func TestSelectGroups_NonInteractive(t *testing.T) {
	// When running in tests, stdin is not a terminal
	// Non-interactive mode should return all groups in original order
	if IsInteractive() {
		t.Skip("skipping: stdin is a terminal in this test environment")
	}

	groups := []provider.OrderGroup{
		{Name: "Feature A", Description: "First feature", Priority: 1},
		{Name: "Feature B", Description: "Second feature", Priority: 2},
	}
	fileCounts := map[string]int{
		"Feature A": 3,
		"Feature B": 2,
	}

	result, err := SelectGroups(groups, fileCounts)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Should return all groups in original order
	if len(result) != 2 {
		t.Errorf("expected 2 groups, got %d", len(result))
	}
	if result[0].Name != "Feature A" {
		t.Errorf("expected first group to be 'Feature A', got %q", result[0].Name)
	}
	if result[1].Name != "Feature B" {
		t.Errorf("expected second group to be 'Feature B', got %q", result[1].Name)
	}
}

func TestSelectGroups_NonInteractive_SingleGroup(t *testing.T) {
	if IsInteractive() {
		t.Skip("skipping: stdin is a terminal in this test environment")
	}

	groups := []provider.OrderGroup{
		{Name: "Only Group", Description: "The only group", Priority: 1},
	}
	fileCounts := map[string]int{
		"Only Group": 5,
	}

	result, err := SelectGroups(groups, fileCounts)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(result) != 1 {
		t.Errorf("expected 1 group, got %d", len(result))
	}
	if result[0].Name != "Only Group" {
		t.Errorf("expected group name 'Only Group', got %q", result[0].Name)
	}
}
