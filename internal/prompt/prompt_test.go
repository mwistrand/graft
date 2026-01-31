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
	// In test environment, stdin is not a terminal, so should return Continue=true
	if IsInteractive() {
		t.Skip("skipping: stdin is a terminal in this test environment")
	}

	result := ConfirmContinue("", 0)
	if !result.Continue {
		t.Error("expected Continue=true in non-interactive mode")
	}
	if result.TimedOut {
		t.Error("expected TimedOut=false in non-interactive mode")
	}

	result = ConfirmContinue("Custom message", 0)
	if !result.Continue {
		t.Error("expected Continue=true in non-interactive mode with custom message")
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

func TestSortGroupsBySignificance(t *testing.T) {
	groups := []provider.OrderGroup{
		{Name: "Minor 1", Significance: provider.SignificanceMinor, Priority: 1},
		{Name: "Core 2", Significance: provider.SignificanceCore, Priority: 2},
		{Name: "Supporting 1", Significance: provider.SignificanceSupporting, Priority: 1},
		{Name: "Core 1", Significance: provider.SignificanceCore, Priority: 1},
		{Name: "Minor 2", Significance: provider.SignificanceMinor, Priority: 2},
	}

	sortGroupsBySignificance(groups)

	// Expected order: Core 1, Core 2, Supporting 1, Minor 1, Minor 2
	expected := []string{"Core 1", "Core 2", "Supporting 1", "Minor 1", "Minor 2"}
	for i, g := range groups {
		if g.Name != expected[i] {
			t.Errorf("groups[%d] = %q, want %q", i, g.Name, expected[i])
		}
	}
}

func TestSortGroupsBySignificance_EmptySignificance(t *testing.T) {
	// Groups without significance should be treated as "core" (default)
	groups := []provider.OrderGroup{
		{Name: "Minor", Significance: provider.SignificanceMinor, Priority: 1},
		{Name: "Unspecified", Significance: "", Priority: 1}, // Should default to core
	}

	sortGroupsBySignificance(groups)

	// Unspecified (treated as core) should come before minor
	if groups[0].Name != "Unspecified" {
		t.Errorf("expected 'Unspecified' first (defaults to core), got %q", groups[0].Name)
	}
}

func TestSignificanceTierPrefix(t *testing.T) {
	tests := []struct {
		sig  provider.Significance
		want string
	}{
		{provider.SignificanceCore, "[core]"},
		{provider.SignificanceSupporting, "[supporting]"},
		{provider.SignificanceMinor, "[minor]"},
		{provider.Significance(""), "[core]"}, // Empty defaults to core
	}

	for _, tt := range tests {
		got := significanceTierPrefix(tt.sig)
		if got != tt.want {
			t.Errorf("significanceTierPrefix(%q) = %q, want %q", tt.sig, got, tt.want)
		}
	}
}

func TestSortGroupsBySignificance_EmptySlice(t *testing.T) {
	var groups []provider.OrderGroup
	sortGroupsBySignificance(groups)

	if len(groups) != 0 {
		t.Errorf("expected empty slice, got %d elements", len(groups))
	}

	groups = []provider.OrderGroup{}
	sortGroupsBySignificance(groups)

	if len(groups) != 0 {
		t.Errorf("expected empty slice, got %d elements", len(groups))
	}
}
