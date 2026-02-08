package cmd

import (
	"strconv"
	"testing"
)

func TestAppendUnique(t *testing.T) {
	tests := []struct {
		name  string
		slice []string
		items []string
		want  []string
	}{
		{"adds new items", []string{"a", "b"}, []string{"c"}, []string{"a", "b", "c"}},
		{"skips duplicates", []string{"a", "b"}, []string{"b", "c"}, []string{"a", "b", "c"}},
		{"empty slice", nil, []string{"a"}, []string{"a"}},
		{"no items", []string{"a"}, nil, []string{"a"}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := appendUnique(tt.slice, tt.items...)
			assertStrings(t, got, tt.want)
		})
	}
}

func TestRemoveAll(t *testing.T) {
	tests := []struct {
		name  string
		slice []string
		items []string
		want  []string
	}{
		{"removes matching", []string{"a", "b", "c"}, []string{"b"}, []string{"a", "c"}},
		{"removes multiple", []string{"a", "b", "c"}, []string{"a", "c"}, []string{"b"}},
		{"no match", []string{"a", "b"}, []string{"x"}, []string{"a", "b"}},
		{"empty slice", nil, []string{"a"}, []string{}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := removeAll(tt.slice, tt.items...)
			assertStrings(t, got, tt.want)
		})
	}
}

func TestAppendUniqueInts(t *testing.T) {
	tests := []struct {
		name  string
		slice []int
		items []int
		want  []int
	}{
		{"adds new", []int{1, 2}, []int{3}, []int{1, 2, 3}},
		{"skips dupes", []int{1, 2}, []int{2, 3}, []int{1, 2, 3}},
		{"empty slice", nil, []int{1}, []int{1}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := appendUniqueInts(tt.slice, tt.items...)
			assertInts(t, got, tt.want)
		})
	}
}

func TestRemoveInts(t *testing.T) {
	tests := []struct {
		name  string
		slice []int
		items []int
		want  []int
	}{
		{"removes matching", []int{1, 2, 3}, []int{2}, []int{1, 3}},
		{"removes multiple", []int{1, 2, 3}, []int{1, 3}, []int{2}},
		{"no match", []int{1, 2}, []int{5}, []int{1, 2}},
		{"empty slice", nil, []int{1}, []int{}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := removeInts(tt.slice, tt.items...)
			assertInts(t, got, tt.want)
		})
	}
}

func assertStrings(t *testing.T, got, want []string) {
	t.Helper()
	if len(got) != len(want) {
		t.Fatalf("len = %d, want %d: got %v", len(got), len(want), got)
	}
	for i, v := range got {
		if v != want[i] {
			t.Errorf("got[%d] = %q, want %q", i, v, want[i])
		}
	}
}

func assertInts(t *testing.T, got, want []int) {
	t.Helper()
	if len(got) != len(want) {
		t.Fatalf("len = %d, want %d: got %v", len(got), len(want), got)
	}
	for i, v := range got {
		if v != want[i] {
			t.Errorf("got[%d] = %s, want %s", i, strconv.Itoa(v), strconv.Itoa(want[i]))
		}
	}
}
