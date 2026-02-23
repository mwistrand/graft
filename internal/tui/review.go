package tui

import (
	"context"
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/mwistrand/graft/internal/prompt"
	"github.com/mwistrand/graft/internal/provider"
)

// ReviewModel is the main TUI model for reviewing diffs.
type ReviewModel struct {
	session    *prompt.ReviewSession
	viewport   viewport.Model
	picker     PickerModel
	pickerOpen bool
	loader     *DiffLoader
	width      int
	height     int
	ready      bool
	quitting   bool
	message    string // Temporary message to display
}

// NewReviewModel creates a new review TUI.
func NewReviewModel(files []provider.OrderedFile, repoDir, baseRef, deltaPath string, fullCodebase, noGit bool) ReviewModel {
	return ReviewModel{
		session: prompt.NewReviewSession(files),
		loader:  NewDiffLoader(repoDir, baseRef, deltaPath, fullCodebase, noGit),
	}
}

// Init implements tea.Model.
func (m ReviewModel) Init() tea.Cmd {
	return nil
}

// diffLoadedMsg is sent when diff content is loaded.
type diffLoadedMsg struct {
	content string
	err     error
}

// loadDiffCmd creates a command to load diff content.
func (m ReviewModel) loadDiffCmd() tea.Cmd {
	file := m.session.Current()
	return func() tea.Msg {
		content, err := m.loader.LoadDiff(context.Background(), file.Path)
		return diffLoadedMsg{content: content, err: err}
	}
}

// Update implements tea.Model.
func (m ReviewModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmds []tea.Cmd

	switch msg := msg.(type) {
	case tea.KeyMsg:
		// Handle picker if open
		if m.pickerOpen {
			var cmd tea.Cmd
			m.picker, cmd = m.picker.Update(msg)
			if m.picker.Done() {
				m.pickerOpen = false
				if idx := m.picker.Selected(); idx >= 0 {
					m.session.JumpTo(idx)
					return m, m.loadDiffCmd()
				}
			}
			return m, cmd
		}

		// Main view key handling
		switch msg.String() {
		case "ctrl+c", "q":
			m.quitting = true
			return m, tea.Quit

		case "ctrl+j":
			// Open file picker
			m.picker = NewPickerModel(m.session.Files, m.session.Index, m.width, m.height)
			m.pickerOpen = true
			return m, nil

		case "ctrl+b":
			// Jump back
			if m.session.JumpBack() {
				m.message = ""
				return m, m.loadDiffCmd()
			}
			m.message = "No previous position"
			return m, nil

		case "n":
			// Next file
			if m.session.Next() {
				m.message = ""
				return m, m.loadDiffCmd()
			}
			m.message = "Already at last file"
			return m, nil

		case "p":
			// Previous file
			if m.session.Previous() {
				m.message = ""
				return m, m.loadDiffCmd()
			}
			m.message = "Already at first file"
			return m, nil

		default:
			// Pass to viewport for scrolling
			var cmd tea.Cmd
			m.viewport, cmd = m.viewport.Update(msg)
			return m, cmd
		}

	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height

		headerHeight := 2
		statusHeight := 1

		if !m.ready {
			m.viewport = viewport.New(msg.Width, msg.Height-headerHeight-statusHeight)
			m.viewport.YPosition = headerHeight
			m.ready = true
			// Load initial diff
			cmds = append(cmds, m.loadDiffCmd())
		} else {
			m.viewport.Width = msg.Width
			m.viewport.Height = msg.Height - headerHeight - statusHeight
		}

		if m.pickerOpen {
			m.picker, _ = m.picker.Update(msg)
		}

		return m, tea.Batch(cmds...)

	case diffLoadedMsg:
		if msg.err != nil {
			m.viewport.SetContent(fmt.Sprintf("Error loading diff: %v", msg.err))
		} else if msg.content == "" {
			m.viewport.SetContent("No changes in this file")
		} else {
			m.viewport.SetContent(msg.content)
		}
		m.viewport.GotoTop()
		return m, nil
	}

	// Update picker if open
	if m.pickerOpen {
		var cmd tea.Cmd
		m.picker, cmd = m.picker.Update(msg)
		cmds = append(cmds, cmd)
	}

	return m, tea.Batch(cmds...)
}

// View implements tea.Model.
func (m ReviewModel) View() string {
	if m.quitting {
		return "Review ended.\n"
	}

	if !m.ready {
		return "Loading...\n"
	}

	var b strings.Builder

	// Header
	file := m.session.Current()
	position := fmt.Sprintf("[%d/%d]", m.session.Position(), m.session.Total())

	header := positionStyle.Render(position) + " "
	if file.Group != "" {
		header += groupStyle.Render("["+file.Group+"]") + " "
	}
	header += filePathStyle.Render(file.Path)
	if file.Description != "" {
		header += "\n" + descStyle.Render("  "+file.Description)
	}
	b.WriteString(headerStyle.Render(header))
	b.WriteString("\n")

	// Viewport (diff content)
	b.WriteString(m.viewport.View())
	b.WriteString("\n")

	// Status bar
	status := keyStyle.Render("^J") + " jump  "
	if m.session.CanGoBack() {
		status += keyStyle.Render("^B") + " back  "
	}
	status += keyStyle.Render("n") + "/" + keyStyle.Render("p") + " next/prev  "
	status += keyStyle.Render("q") + " quit"

	if m.message != "" {
		status += "  " + noHistoryStyle.Render(m.message)
	}

	// Scroll position
	scrollInfo := fmt.Sprintf("  %d%%", int(m.viewport.ScrollPercent()*100))
	status += lipgloss.NewStyle().Foreground(lipgloss.Color("240")).Render(scrollInfo)

	b.WriteString(statusBarStyle.Render(status))

	// Overlay picker if open
	if m.pickerOpen {
		// Center the picker
		pickerView := m.picker.View()
		lines := strings.Split(pickerView, "\n")
		pickerHeight := len(lines)
		pickerWidth := 0
		for _, line := range lines {
			if len(line) > pickerWidth {
				pickerWidth = len(line)
			}
		}

		// Calculate position to center
		startY := (m.height - pickerHeight) / 2
		startX := (m.width - pickerWidth) / 2
		if startX < 0 {
			startX = 0
		}
		if startY < 0 {
			startY = 0
		}

		// Create overlay effect
		base := b.String()
		baseLines := strings.Split(base, "\n")

		// Pad picker lines with spaces for positioning
		paddedPicker := make([]string, len(lines))
		for i, line := range lines {
			paddedPicker[i] = strings.Repeat(" ", startX) + line
		}

		// Overlay picker on base
		for i, pickerLine := range paddedPicker {
			targetLine := startY + i
			if targetLine >= 0 && targetLine < len(baseLines) {
				baseLines[targetLine] = pickerLine
			}
		}

		return strings.Join(baseLines, "\n")
	}

	return b.String()
}

// Run starts the review TUI.
func Run(files []provider.OrderedFile, repoDir, baseRef string, useDelta, fullCodebase, noGit bool) error {
	deltaPath := ""
	if useDelta {
		deltaPath = FindDelta()
	}

	model := NewReviewModel(files, repoDir, baseRef, deltaPath, fullCodebase, noGit)
	p := tea.NewProgram(model, tea.WithAltScreen())

	_, err := p.Run()
	return err
}
