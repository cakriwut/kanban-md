package tui

import (
	"testing"
)

func TestWrapTitle(t *testing.T) {
	tests := []struct {
		name     string
		title    string
		maxWidth int
		maxLines int
		want     []string
	}{
		{
			name:     "short title fits one line",
			title:    "Fix bug",
			maxWidth: 20,
			maxLines: 2,
			want:     []string{"Fix bug"},
		},
		{
			name:     "single line mode truncates",
			title:    "This is a very long title that exceeds width",
			maxWidth: 15,
			maxLines: 1,
			want:     []string{"This is a ve..."},
		},
		{
			name:     "wraps at word boundary",
			title:    "Implement user authentication",
			maxWidth: 15,
			maxLines: 2,
			want:     []string{"Implement user", "authentication"},
		},
		{
			name:     "three lines",
			title:    "Add comprehensive integration test suite for the API",
			maxWidth: 15,
			maxLines: 3,
			want:     []string{"Add", "comprehensive", "integration ..."},
		},
		{
			name:     "long single word truncated",
			title:    "Supercalifragilisticexpialidocious task",
			maxWidth: 10,
			maxLines: 2,
			want:     []string{"Superca...", "task"},
		},
		{
			name:     "exact fit",
			title:    "Exact fit here",
			maxWidth: 14,
			maxLines: 2,
			want:     []string{"Exact fit here"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := wrapTitle(tt.title, tt.maxWidth, tt.maxLines)
			if len(got) != len(tt.want) {
				t.Fatalf("wrapTitle() returned %d lines, want %d: %v", len(got), len(tt.want), got)
			}
			for i := range got {
				if got[i] != tt.want[i] {
					t.Errorf("line %d = %q, want %q", i, got[i], tt.want[i])
				}
			}
		})
	}
}
