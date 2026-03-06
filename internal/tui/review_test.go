package tui

import (
	"fmt"
	"testing"

	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/mwistrand/graft/internal/prompt"
	"github.com/mwistrand/graft/internal/provider"
)

func newTestModel(files []provider.OrderedFile) ReviewModel {
	return ReviewModel{
		session:  prompt.NewReviewSession(files),
		loader:   NewDiffLoader("/repo", "main", "", false, false),
		viewport: viewport.New(80, 20),
		ready:    true,
	}
}

var testFiles = []provider.OrderedFile{
	{Path: "a.go", Group: "core", Description: "first"},
	{Path: "b.go", Group: "core", Description: "second"},
	{Path: "c.go", Group: "tests", Description: "third"},
}

func sendKey(m ReviewModel, key string) ReviewModel {
	updated, _ := m.Update(tea.KeyMsg(tea.Key{Type: tea.KeyRunes, Runes: []rune(key)}))
	return updated.(ReviewModel)
}

func sendSpecialKey(m ReviewModel, keyType tea.KeyType) ReviewModel {
	updated, _ := m.Update(tea.KeyMsg(tea.Key{Type: keyType}))
	return updated.(ReviewModel)
}

func TestUpdate_Quit(t *testing.T) {
	m := newTestModel(testFiles)

	tests := []struct {
		name string
		key  string
	}{
		{"q key", "q"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := sendKey(m, tt.key)
			if !result.quitting {
				t.Error("expected quitting to be true")
			}
		})
	}
}

func TestUpdate_CtrlC(t *testing.T) {
	m := newTestModel(testFiles)
	updated, cmd := m.Update(tea.KeyMsg(tea.Key{Type: tea.KeyCtrlC}))
	result := updated.(ReviewModel)

	if !result.quitting {
		t.Error("expected quitting to be true")
	}
	if cmd == nil {
		t.Error("expected quit command")
	}
}

func TestUpdate_NextFile(t *testing.T) {
	m := newTestModel(testFiles)

	result := sendKey(m, "n")
	if result.session.Index != 1 {
		t.Errorf("expected index 1, got %d", result.session.Index)
	}
	if result.message != "" {
		t.Errorf("expected empty message, got %q", result.message)
	}
}

func TestUpdate_NextFileAtEnd(t *testing.T) {
	m := newTestModel(testFiles)
	m.session.Index = 2

	result := sendKey(m, "n")
	if result.session.Index != 2 {
		t.Errorf("expected index 2, got %d", result.session.Index)
	}
	if result.message != "Already at last file" {
		t.Errorf("expected boundary message, got %q", result.message)
	}
}

func TestUpdate_PreviousFile(t *testing.T) {
	m := newTestModel(testFiles)
	m.session.Index = 2

	result := sendKey(m, "p")
	if result.session.Index != 1 {
		t.Errorf("expected index 1, got %d", result.session.Index)
	}
}

func TestUpdate_PreviousFileAtStart(t *testing.T) {
	m := newTestModel(testFiles)

	result := sendKey(m, "p")
	if result.session.Index != 0 {
		t.Errorf("expected index 0, got %d", result.session.Index)
	}
	if result.message != "Already at first file" {
		t.Errorf("expected boundary message, got %q", result.message)
	}
}

func TestUpdate_GotoTop(t *testing.T) {
	m := newTestModel(testFiles)
	// Set content tall enough to scroll
	content := ""
	for i := range 100 {
		content += fmt.Sprintf("line %d\n", i)
	}
	m.viewport.SetContent(content)
	m.viewport.GotoBottom()
	if m.viewport.YOffset == 0 {
		t.Fatal("expected viewport to be scrolled down after GotoBottom")
	}

	tests := []struct {
		name string
		send func(ReviewModel) ReviewModel
	}{
		{"g key", func(m ReviewModel) ReviewModel { return sendKey(m, "g") }},
		{"home key", func(m ReviewModel) ReviewModel { return sendSpecialKey(m, tea.KeyHome) }},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			scrolled := m
			scrolled.viewport.GotoBottom()
			result := tt.send(scrolled)
			if result.viewport.YOffset != 0 {
				t.Errorf("expected YOffset 0, got %d", result.viewport.YOffset)
			}
		})
	}
}

func TestUpdate_GotoBottom(t *testing.T) {
	m := newTestModel(testFiles)
	content := ""
	for i := range 100 {
		content += fmt.Sprintf("line %d\n", i)
	}
	m.viewport.SetContent(content)
	m.viewport.GotoTop()

	tests := []struct {
		name string
		send func(ReviewModel) ReviewModel
	}{
		{"G key", func(m ReviewModel) ReviewModel { return sendKey(m, "G") }},
		{"end key", func(m ReviewModel) ReviewModel { return sendSpecialKey(m, tea.KeyEnd) }},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			atTop := m
			atTop.viewport.GotoTop()
			result := tt.send(atTop)
			if result.viewport.YOffset == 0 {
				t.Errorf("expected viewport to scroll down, YOffset is still 0")
			}
		})
	}
}

func TestUpdate_OpenPicker(t *testing.T) {
	m := newTestModel(testFiles)
	m.width = 80
	m.height = 24

	updated, _ := m.Update(tea.KeyMsg(tea.Key{Type: tea.KeyCtrlJ}))
	result := updated.(ReviewModel)

	if !result.pickerOpen {
		t.Error("expected picker to be open")
	}
}

func TestUpdate_JumpBack(t *testing.T) {
	m := newTestModel(testFiles)
	m.width = 80
	m.height = 24

	// Jump to file 2 to create history
	m.session.JumpTo(2)

	updated, _ := m.Update(tea.KeyMsg(tea.Key{Type: tea.KeyCtrlB}))
	result := updated.(ReviewModel)

	if result.session.Index != 0 {
		t.Errorf("expected index 0 after jump back, got %d", result.session.Index)
	}
}

func TestUpdate_JumpBackNoHistory(t *testing.T) {
	m := newTestModel(testFiles)

	updated, _ := m.Update(tea.KeyMsg(tea.Key{Type: tea.KeyCtrlB}))
	result := updated.(ReviewModel)

	if result.message != "No previous position" {
		t.Errorf("expected no history message, got %q", result.message)
	}
}

func TestUpdate_NextClearsMessage(t *testing.T) {
	m := newTestModel(testFiles)
	m.message = "some message"

	result := sendKey(m, "n")
	if result.message != "" {
		t.Errorf("expected message cleared, got %q", result.message)
	}
}

func TestUpdate_DiffLoaded(t *testing.T) {
	m := newTestModel(testFiles)

	updated, _ := m.Update(diffLoadedMsg{content: "diff content"})
	result := updated.(ReviewModel)

	if result.viewport.YOffset != 0 {
		t.Errorf("expected viewport at top after load, got YOffset %d", result.viewport.YOffset)
	}
}

func TestUpdate_DiffLoadedEmpty(t *testing.T) {
	m := newTestModel(testFiles)

	updated, _ := m.Update(diffLoadedMsg{content: ""})
	result := updated.(ReviewModel)

	view := result.viewport.View()
	if view == "" {
		t.Error("expected fallback content for empty diff")
	}
}

func TestUpdate_DiffLoadedError(t *testing.T) {
	m := newTestModel(testFiles)

	updated, _ := m.Update(diffLoadedMsg{err: fmt.Errorf("test error")})
	result := updated.(ReviewModel)

	view := result.viewport.View()
	if view == "" {
		t.Error("expected error content in viewport")
	}
}

func TestUpdate_WindowSize(t *testing.T) {
	m := newTestModel(testFiles)
	m.ready = false

	updated, _ := m.Update(tea.WindowSizeMsg{Width: 120, Height: 40})
	result := updated.(ReviewModel)

	if !result.ready {
		t.Error("expected ready to be true after window size")
	}
	if result.width != 120 {
		t.Errorf("expected width 120, got %d", result.width)
	}
	if result.height != 40 {
		t.Errorf("expected height 40, got %d", result.height)
	}
}

func TestUpdate_PickerKeysIgnoredWhenClosed(t *testing.T) {
	m := newTestModel(testFiles)
	// Pressing enter with picker closed should pass to viewport, not panic
	updated, _ := m.Update(tea.KeyMsg(tea.Key{Type: tea.KeyEnter}))
	result := updated.(ReviewModel)
	if result.pickerOpen {
		t.Error("picker should not be open")
	}
}

func TestView_Quitting(t *testing.T) {
	m := newTestModel(testFiles)
	m.quitting = true

	view := m.View()
	if view != "Review ended.\n" {
		t.Errorf("expected quit message, got %q", view)
	}
}

func TestView_Loading(t *testing.T) {
	m := newTestModel(testFiles)
	m.ready = false

	view := m.View()
	if view != "Loading...\n" {
		t.Errorf("expected loading message, got %q", view)
	}
}
