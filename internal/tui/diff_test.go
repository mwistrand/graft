package tui

import (
	"testing"
)

func TestDiffArgs_ThreeDot(t *testing.T) {
	d := NewDiffLoader("/repo", "main", "", false, false)
	args := d.diffArgs("file.go")

	want := []string{"-C", "/repo", "diff", "--color=always", "main...HEAD", "--", "file.go"}
	if len(args) != len(want) {
		t.Fatalf("got %d args, want %d: %v", len(args), len(want), args)
	}
	for i := range want {
		if args[i] != want[i] {
			t.Errorf("arg[%d] = %q, want %q", i, args[i], want[i])
		}
	}
}

func TestDiffArgs_TwoTree(t *testing.T) {
	emptyTree := "4b825dc642cb6eb9a060e54bf899d15f3f857a08"
	d := NewDiffLoader("/repo", emptyTree, "", true, false)
	args := d.diffArgs("file.go")

	want := []string{"-C", "/repo", "diff", "--color=always", emptyTree, "HEAD", "--", "file.go"}
	if len(args) != len(want) {
		t.Fatalf("got %d args, want %d: %v", len(args), len(want), args)
	}
	for i := range want {
		if args[i] != want[i] {
			t.Errorf("arg[%d] = %q, want %q", i, args[i], want[i])
		}
	}
}
