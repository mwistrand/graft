// Package prompt provides interactive terminal prompts for user input.
package prompt

import (
	"bufio"
	"fmt"
	"os"
	"strings"

	"github.com/charmbracelet/huh"
	"github.com/mwistrand/graft/internal/provider"
	"golang.org/x/term"
)

// IsInteractive returns true if stdin is connected to a terminal.
func IsInteractive() bool {
	return term.IsTerminal(int(os.Stdin.Fd()))
}

// SelectModel displays an interactive list of models and returns the selected model ID.
// If models is empty or stdin is not a terminal, returns an error.
func SelectModel(models []provider.ModelInfo) (string, error) {
	if len(models) == 0 {
		return "", fmt.Errorf("no models available")
	}

	if !IsInteractive() {
		return "", fmt.Errorf("cannot prompt for model: not running in an interactive terminal")
	}

	// Build options for the select prompt
	options := make([]huh.Option[string], len(models))
	for i, m := range models {
		displayName := m.Name
		if displayName == "" {
			displayName = m.ID
		}
		if m.Description != "" {
			displayName = fmt.Sprintf("%s - %s", displayName, m.Description)
		}
		options[i] = huh.NewOption(displayName, m.ID)
	}

	var selected string
	form := huh.NewForm(
		huh.NewGroup(
			huh.NewSelect[string]().
				Title("Select a model").
				Description("Use arrow keys to navigate, enter to select").
				Options(options...).
				Value(&selected),
		),
	).WithAccessible(false) // Require interactive mode, don't fall back to accessible mode

	if err := form.Run(); err != nil {
		return "", fmt.Errorf("model selection: %w", err)
	}

	return selected, nil
}

// ConfirmContinue prompts the user to continue or quit.
// Returns true if the user wants to continue, false to quit.
// If not running in an interactive terminal, returns true (continue by default).
func ConfirmContinue(message string) bool {
	if !IsInteractive() {
		return true
	}

	if message == "" {
		message = "Continue reviewing diffs?"
	}

	fmt.Printf("\n%s [Y/n] ", message)

	reader := bufio.NewReader(os.Stdin)
	input, err := reader.ReadString('\n')
	if err != nil {
		return true // On error, continue by default
	}

	input = strings.TrimSpace(strings.ToLower(input))
	// Default to yes if empty, or explicit yes
	if input == "" || input == "y" || input == "yes" {
		fmt.Println()
		return true
	}

	return false
}

// SelectGroups displays an interactive multi-select for choosing which groups to review.
// Returns the selected groups in their original priority order.
// If not interactive or user selects nothing, returns all groups.
func SelectGroups(groups []provider.OrderGroup, fileCounts map[string]int) ([]provider.OrderGroup, error) {
	if len(groups) == 0 {
		return nil, fmt.Errorf("no groups available")
	}

	if !IsInteractive() {
		// Non-interactive: return groups in original order
		return groups, nil
	}

	// Build options with file counts
	options := make([]huh.Option[string], len(groups))
	for i, g := range groups {
		count := fileCounts[g.Name]
		displayName := fmt.Sprintf("%s (%d files)", g.Name, count)
		if g.Description != "" {
			displayName = fmt.Sprintf("%s - %s (%d files)", g.Name, g.Description, count)
		}
		options[i] = huh.NewOption(displayName, g.Name).Selected(true)
	}

	var selected []string
	form := huh.NewForm(
		huh.NewGroup(
			huh.NewMultiSelect[string]().
				Title("Select groups to review").
				Description("Space to toggle, Enter to confirm. All selected by default.").
				Options(options...).
				Value(&selected),
		),
	).WithAccessible(false)

	if err := form.Run(); err != nil {
		return nil, fmt.Errorf("group selection: %w", err)
	}

	// If user deselected everything, return all groups
	if len(selected) == 0 {
		return groups, nil
	}

	// Build a set of selected group names
	selectedSet := make(map[string]bool)
	for _, name := range selected {
		selectedSet[name] = true
	}

	// Return groups in original order, filtered to selected only
	result := make([]provider.OrderGroup, 0, len(selected))
	for _, g := range groups {
		if selectedSet[g.Name] {
			result = append(result, g)
		}
	}

	return result, nil
}
