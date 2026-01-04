// Package prompt provides interactive terminal prompts for user input.
package prompt

import (
	"fmt"
	"os"

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
