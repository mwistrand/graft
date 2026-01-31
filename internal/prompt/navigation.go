// Package prompt provides interactive terminal prompts for user input.
package prompt

import (
	"github.com/mwistrand/graft/internal/provider"
)

// NavigationStack tracks jump history for "back" functionality.
type NavigationStack struct {
	positions []int
	maxSize   int
}

// NewNavigationStack creates a new navigation stack with the given maximum size.
func NewNavigationStack(maxSize int) *NavigationStack {
	if maxSize <= 0 {
		maxSize = 10
	}
	return &NavigationStack{
		positions: make([]int, 0, maxSize),
		maxSize:   maxSize,
	}
}

// Push adds a position to the stack. If the stack is full, the oldest position is removed.
func (s *NavigationStack) Push(index int) {
	if len(s.positions) >= s.maxSize {
		// Remove oldest (first) element
		s.positions = s.positions[1:]
	}
	s.positions = append(s.positions, index)
}

// Pop removes and returns the most recent position from the stack.
// Returns -1 and false if the stack is empty.
func (s *NavigationStack) Pop() (int, bool) {
	if len(s.positions) == 0 {
		return -1, false
	}
	index := s.positions[len(s.positions)-1]
	s.positions = s.positions[:len(s.positions)-1]
	return index, true
}

// IsEmpty returns true if the stack has no positions.
func (s *NavigationStack) IsEmpty() bool {
	return len(s.positions) == 0
}

// ReviewSession maintains navigation state during a review.
type ReviewSession struct {
	Files   []provider.OrderedFile
	Index   int
	History *NavigationStack
}

// NewReviewSession creates a new review session for the given files.
func NewReviewSession(files []provider.OrderedFile) *ReviewSession {
	return &ReviewSession{
		Files:   files,
		Index:   0,
		History: NewNavigationStack(10),
	}
}

// Current returns the current file being reviewed.
func (s *ReviewSession) Current() provider.OrderedFile {
	if s.Index < 0 || s.Index >= len(s.Files) {
		return provider.OrderedFile{}
	}
	return s.Files[s.Index]
}

// Position returns the 1-indexed position of the current file.
func (s *ReviewSession) Position() int {
	return s.Index + 1
}

// Total returns the total number of files in the review.
func (s *ReviewSession) Total() int {
	return len(s.Files)
}

// HasNext returns true if there are more files to review.
func (s *ReviewSession) HasNext() bool {
	return s.Index < len(s.Files)
}

// Next advances to the next file. Returns true if the position changed.
func (s *ReviewSession) Next() bool {
	if s.Index < len(s.Files)-1 {
		s.Index++
		return true
	}
	return false
}

// Previous moves to the previous file. Returns true if the position changed.
func (s *ReviewSession) Previous() bool {
	if s.Index > 0 {
		s.Index--
		return true
	}
	return false
}

// JumpTo jumps to the specified file index, saving the current position to history.
func (s *ReviewSession) JumpTo(index int) {
	if index < 0 || index >= len(s.Files) {
		return
	}
	if index != s.Index {
		s.History.Push(s.Index)
		s.Index = index
	}
}

// JumpBack returns to the previous position from the navigation history.
// Returns true if a jump back occurred, false if there was no history.
func (s *ReviewSession) JumpBack() bool {
	if prev, ok := s.History.Pop(); ok {
		s.Index = prev
		return true
	}
	return false
}

// CanGoBack returns true if there is navigation history to go back to.
func (s *ReviewSession) CanGoBack() bool {
	return !s.History.IsEmpty()
}
