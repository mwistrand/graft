// Package prompt provides interactive terminal prompts for user input.
package prompt

import (
	"bufio"
	"fmt"
	"os"
	"sort"
	"strings"
	"time"

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

// ConfirmContinueResult represents the result of a confirmation prompt.
type ConfirmContinueResult struct {
	Continue  bool // Whether the user wants to continue
	TimedOut  bool // Whether the prompt timed out
	Cancelled bool // Whether the user explicitly cancelled
}

// ConfirmContinue prompts the user to continue or quit with an optional timeout.
// The timeout parameter specifies the maximum wait time. Use 0 to disable timeout.
// Returns a result indicating whether to continue, if it timed out, or was cancelled.
// If not running in an interactive terminal, returns Continue=true immediately.
func ConfirmContinue(message string, timeout time.Duration) ConfirmContinueResult {
	if !IsInteractive() {
		return ConfirmContinueResult{Continue: true}
	}

	if message == "" {
		message = "Continue reviewing diffs?"
	}

	// Show timeout info if enabled
	if timeout > 0 {
		fmt.Printf("\n%s [Y/n] (timeout in %v) ", message, timeout.Round(time.Minute))
	} else {
		fmt.Printf("\n%s [Y/n] ", message)
	}

	// Read input in a goroutine to allow timeout.
	//
	// Note: if the timeout fires, this goroutine will remain blocked on
	// ReadString until the process exits. This is an intentional trade-off:
	// os.Stdin reads cannot be cancelled via context, and the alternatives
	// (SetReadDeadline on a dup'd fd, or closing stdin) introduce platform-
	// specific complexity for no practical benefit — the process exits
	// shortly after a timeout.
	type readResult struct {
		input string
		err   error
	}
	resultCh := make(chan readResult, 1)

	go func() {
		reader := bufio.NewReader(os.Stdin)
		input, err := reader.ReadString('\n')
		resultCh <- readResult{input: input, err: err}
	}()

	// Wait for input or timeout
	if timeout > 0 {
		select {
		case result := <-resultCh:
			if result.err != nil {
				return ConfirmContinueResult{Continue: true} // On error, continue by default
			}
			input := strings.TrimSpace(strings.ToLower(result.input))
			if input == "" || input == "y" || input == "yes" {
				fmt.Println()
				return ConfirmContinueResult{Continue: true}
			}
			return ConfirmContinueResult{Cancelled: true}
		case <-time.After(timeout):
			fmt.Println("\n\nPrompt timed out. Exiting review.")
			return ConfirmContinueResult{TimedOut: true}
		}
	}

	// No timeout - wait indefinitely
	result := <-resultCh
	if result.err != nil {
		return ConfirmContinueResult{Continue: true} // On error, continue by default
	}
	input := strings.TrimSpace(strings.ToLower(result.input))
	if input == "" || input == "y" || input == "yes" {
		fmt.Println()
		return ConfirmContinueResult{Continue: true}
	}
	return ConfirmContinueResult{Cancelled: true}
}

// SelectGroups displays an interactive multi-select for choosing which groups to review.
// Groups are organized by significance tier (core, supporting, minor).
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

	// Sort groups by significance tier, then by priority within tier
	sortedGroups := make([]provider.OrderGroup, len(groups))
	copy(sortedGroups, groups)
	sortGroupsBySignificance(sortedGroups)

	// Build options with file counts and tier prefixes
	options := make([]huh.Option[string], len(sortedGroups))
	for i, g := range sortedGroups {
		count := fileCounts[g.Name]
		sig := provider.NormalizeSignificance(g.Significance)

		// Add tier prefix for visual organization
		tierPrefix := significanceTierPrefix(sig)

		displayName := fmt.Sprintf("%s %s (%d files)", tierPrefix, g.Name, count)
		if g.Description != "" {
			displayName = fmt.Sprintf("%s %s - %s (%d files)", tierPrefix, g.Name, g.Description, count)
		}

		// Default: core and supporting selected, minor deselected
		selected := sig != provider.SignificanceMinor
		options[i] = huh.NewOption(displayName, g.Name).Selected(selected)
	}

	var selected []string
	form := huh.NewForm(
		huh.NewGroup(
			huh.NewMultiSelect[string]().
				Title("Select groups to review").
				Description("Space to toggle, Enter to confirm. Core/supporting selected by default.").
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

// sortGroupsBySignificance sorts groups by significance tier, then by priority within tier.
func sortGroupsBySignificance(groups []provider.OrderGroup) {
	sort.Slice(groups, func(i, j int) bool {
		sigI := provider.SignificancePriority(provider.NormalizeSignificance(groups[i].Significance))
		sigJ := provider.SignificancePriority(provider.NormalizeSignificance(groups[j].Significance))
		if sigI != sigJ {
			return sigI < sigJ
		}
		return groups[i].Priority < groups[j].Priority
	})
}

// significanceTierPrefix returns a visual prefix for a significance tier.
func significanceTierPrefix(sig provider.Significance) string {
	switch sig {
	case provider.SignificanceCore:
		return "[core]"
	case provider.SignificanceSupporting:
		return "[supporting]"
	case provider.SignificanceMinor:
		return "[minor]"
	default:
		return "[core]"
	}
}
