package prompt

import (
	"testing"

	"github.com/mwistrand/graft/internal/provider"
)

func TestNavigationStack_PushPop(t *testing.T) {
	stack := NewNavigationStack(3)

	if !stack.IsEmpty() {
		t.Error("new stack should be empty")
	}

	// Pop from empty stack
	if _, ok := stack.Pop(); ok {
		t.Error("pop from empty stack should return false")
	}

	// Push and pop
	stack.Push(1)
	stack.Push(2)
	stack.Push(3)

	if stack.IsEmpty() {
		t.Error("stack with items should not be empty")
	}

	val, ok := stack.Pop()
	if !ok || val != 3 {
		t.Errorf("expected 3, got %d", val)
	}

	val, ok = stack.Pop()
	if !ok || val != 2 {
		t.Errorf("expected 2, got %d", val)
	}

	val, ok = stack.Pop()
	if !ok || val != 1 {
		t.Errorf("expected 1, got %d", val)
	}

	if !stack.IsEmpty() {
		t.Error("stack should be empty after popping all items")
	}
}

func TestNavigationStack_MaxSize(t *testing.T) {
	stack := NewNavigationStack(3)

	// Push more than max size
	stack.Push(1)
	stack.Push(2)
	stack.Push(3)
	stack.Push(4) // Should drop 1

	// Verify oldest item was dropped
	val, ok := stack.Pop()
	if !ok || val != 4 {
		t.Errorf("expected 4, got %d", val)
	}

	val, ok = stack.Pop()
	if !ok || val != 3 {
		t.Errorf("expected 3, got %d", val)
	}

	val, ok = stack.Pop()
	if !ok || val != 2 {
		t.Errorf("expected 2, got %d", val)
	}

	// 1 should have been dropped
	if _, ok := stack.Pop(); ok {
		t.Error("stack should be empty, 1 should have been dropped")
	}
}

func TestReviewSession_Navigation(t *testing.T) {
	files := []provider.OrderedFile{
		{Path: "file1.go"},
		{Path: "file2.go"},
		{Path: "file3.go"},
	}

	session := NewReviewSession(files)

	if session.Position() != 1 {
		t.Errorf("expected position 1, got %d", session.Position())
	}

	if session.Total() != 3 {
		t.Errorf("expected total 3, got %d", session.Total())
	}

	if !session.HasNext() {
		t.Error("session should have next")
	}

	if session.Current().Path != "file1.go" {
		t.Errorf("expected file1.go, got %s", session.Current().Path)
	}

	// Move to next
	if !session.Next() {
		t.Error("Next should return true when not at end")
	}
	if session.Position() != 2 {
		t.Errorf("expected position 2, got %d", session.Position())
	}

	// Jump to file 3
	session.JumpTo(2)
	if session.Position() != 3 {
		t.Errorf("expected position 3, got %d", session.Position())
	}

	if !session.CanGoBack() {
		t.Error("should be able to go back after jumping")
	}

	// Jump back
	if !session.JumpBack() {
		t.Error("jump back should succeed")
	}

	if session.Position() != 2 {
		t.Errorf("expected position 2 after jump back, got %d", session.Position())
	}

	if session.CanGoBack() {
		t.Error("should not be able to go back after using history")
	}
}

func TestReviewSession_JumpToSameFile(t *testing.T) {
	files := []provider.OrderedFile{
		{Path: "file1.go"},
		{Path: "file2.go"},
	}

	session := NewReviewSession(files)

	// Jump to same file should not add to history
	session.JumpTo(0)

	if session.CanGoBack() {
		t.Error("jumping to same file should not add to history")
	}
}

func TestReviewSession_JumpToInvalidIndex(t *testing.T) {
	files := []provider.OrderedFile{
		{Path: "file1.go"},
	}

	session := NewReviewSession(files)

	// Jump to invalid index should do nothing
	session.JumpTo(-1)
	if session.Position() != 1 {
		t.Errorf("expected position 1, got %d", session.Position())
	}

	session.JumpTo(100)
	if session.Position() != 1 {
		t.Errorf("expected position 1, got %d", session.Position())
	}
}
