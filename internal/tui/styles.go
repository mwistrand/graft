package tui

import "github.com/charmbracelet/lipgloss"

var (
	// Header bar styling
	headerStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("170")).
			Padding(0, 1)

	// File path in header
	filePathStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("252"))

	// Group tag in header
	groupStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("39")).
			Bold(true)

	// Position counter [1/50]
	positionStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("240"))

	// Status bar at bottom
	statusBarStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("240")).
			Padding(0, 1)

	// Key hints in status bar
	keyStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("170")).
			Bold(true)

	// Description text
	descStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("245")).
			Italic(true)

	// Picker overlay border
	pickerBorderStyle = lipgloss.NewStyle().
				Border(lipgloss.RoundedBorder()).
				BorderForeground(lipgloss.Color("170")).
				Padding(1, 2)

	// Picker title
	pickerTitleStyle = lipgloss.NewStyle().
				Bold(true).
				Foreground(lipgloss.Color("170"))

	// No history message
	noHistoryStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("240")).
			Italic(true)
)
