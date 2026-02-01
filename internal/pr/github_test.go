package pr

import (
	"testing"
)

func TestNormalizeState(t *testing.T) {
	tests := []struct {
		name   string
		state  string
		merged bool
		want   string
	}{
		{
			name:   "open state",
			state:  "OPEN",
			merged: false,
			want:   StateOpen,
		},
		{
			name:   "closed state",
			state:  "CLOSED",
			merged: false,
			want:   StateClosed,
		},
		{
			name:   "merged state from merged field",
			state:  "CLOSED",
			merged: true,
			want:   StateMerged,
		},
		{
			name:   "merged state from state field",
			state:  "MERGED",
			merged: false,
			want:   StateMerged,
		},
		{
			name:   "lowercase open",
			state:  "open",
			merged: false,
			want:   StateOpen,
		},
		{
			name:   "mixed case",
			state:  "Open",
			merged: false,
			want:   StateOpen,
		},
		{
			name:   "unknown state passes through",
			state:  "draft",
			merged: false,
			want:   "draft",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := normalizeState(tt.state, tt.merged)
			if got != tt.want {
				t.Errorf("normalizeState(%q, %v) = %q, want %q", tt.state, tt.merged, got, tt.want)
			}
		})
	}
}
