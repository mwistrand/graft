package tui

import (
	"fmt"

	"github.com/charmbracelet/bubbles/list"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/mwistrand/graft/internal/provider"
)

// FileItem wraps an OrderedFile to implement list.DefaultItem interface.
type FileItem struct {
	File  provider.OrderedFile
	Index int
}

// Title returns the file path for display.
func (f FileItem) Title() string {
	return f.File.Path
}

// Description returns the file description with group.
func (f FileItem) Description() string {
	desc := f.File.Description
	if f.File.Group != "" && desc != "" {
		return fmt.Sprintf("[%s] %s", f.File.Group, desc)
	}
	if f.File.Group != "" {
		return fmt.Sprintf("[%s]", f.File.Group)
	}
	return desc
}

// FilterValue returns the string used for fuzzy filtering.
func (f FileItem) FilterValue() string {
	return f.File.Path
}

// PickerModel is the file picker overlay component.
type PickerModel struct {
	list     list.Model
	width    int
	height   int
	selected int
	done     bool
}

// NewPickerModel creates a new file picker.
func NewPickerModel(files []provider.OrderedFile, currentIndex, width, height int) PickerModel {
	items := make([]list.Item, len(files))
	for i, f := range files {
		items[i] = FileItem{File: f, Index: i}
	}

	// Calculate picker dimensions (centered overlay)
	pickerWidth := min(width-10, 80)
	pickerHeight := min(height-6, 20)

	delegate := list.NewDefaultDelegate()
	delegate.Styles.SelectedTitle = lipgloss.NewStyle().
		Foreground(lipgloss.Color("170")).
		Bold(true)
	delegate.Styles.SelectedDesc = lipgloss.NewStyle().
		Foreground(lipgloss.Color("240"))

	l := list.New(items, delegate, pickerWidth, pickerHeight)
	l.Title = "Jump to File"
	l.SetShowStatusBar(true)
	l.SetFilteringEnabled(true)
	l.Styles.Title = pickerTitleStyle

	// Pre-select current file
	if currentIndex >= 0 && currentIndex < len(items) {
		l.Select(currentIndex)
	}

	return PickerModel{
		list:     l,
		width:    width,
		height:   height,
		selected: -1,
	}
}

// Init implements tea.Model.
func (m PickerModel) Init() tea.Cmd {
	return nil
}

// Update implements tea.Model.
func (m PickerModel) Update(msg tea.Msg) (PickerModel, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		// Handle picker-specific keys before passing to list
		switch msg.String() {
		case "esc":
			m.done = true
			m.selected = -1
			return m, nil
		case "enter":
			if item, ok := m.list.SelectedItem().(FileItem); ok {
				m.selected = item.Index
			}
			m.done = true
			return m, nil
		}
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		pickerWidth := min(msg.Width-10, 80)
		pickerHeight := min(msg.Height-6, 20)
		m.list.SetSize(pickerWidth, pickerHeight)
		return m, nil
	}

	var cmd tea.Cmd
	m.list, cmd = m.list.Update(msg)
	return m, cmd
}

// View implements tea.Model.
func (m PickerModel) View() string {
	content := m.list.View()
	return pickerBorderStyle.Render(content)
}

// Selected returns the selected file index, or -1 if cancelled.
func (m PickerModel) Selected() int {
	return m.selected
}

// Done returns true if the picker interaction is complete.
func (m PickerModel) Done() bool {
	return m.done
}
