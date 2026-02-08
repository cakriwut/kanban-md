package cmd

import (
	"errors"
	"testing"

	"github.com/antopolskiy/kanban-md/internal/clierr"
	"github.com/antopolskiy/kanban-md/internal/config"
)

func TestRootCommand(t *testing.T) {
	if rootCmd.Use != "kanban-md" {
		t.Errorf("rootCmd.Use = %v, want kanban-md", rootCmd.Use)
	}
}

func TestParseIDs_Single(t *testing.T) {
	ids, err := parseIDs("42")
	if err != nil {
		t.Fatal(err)
	}
	if len(ids) != 1 || ids[0] != 42 {
		t.Errorf("parseIDs(\"42\") = %v, want [42]", ids)
	}
}

func TestParseIDs_Multiple(t *testing.T) {
	ids, err := parseIDs("1,2,3")
	if err != nil {
		t.Fatal(err)
	}
	want := [3]int{1, 2, 3}
	if len(ids) != 3 {
		t.Fatalf("len = %d, want 3", len(ids))
	}
	if [3]int{ids[0], ids[1], ids[2]} != want {
		t.Errorf("parseIDs(\"1,2,3\") = %v, want %v", ids, want)
	}
}

func TestParseIDs_Deduplicates(t *testing.T) {
	ids, err := parseIDs("1,2,1,3,2")
	if err != nil {
		t.Fatal(err)
	}
	want := [3]int{1, 2, 3}
	if len(ids) != 3 {
		t.Fatalf("len = %d, want 3", len(ids))
	}
	if [3]int{ids[0], ids[1], ids[2]} != want {
		t.Errorf("parseIDs(\"1,2,1,3,2\") = %v, want %v", ids, want)
	}
}

func TestParseIDs_TrimsSpaces(t *testing.T) {
	ids, err := parseIDs(" 1 , 2 ")
	if err != nil {
		t.Fatal(err)
	}
	if len(ids) != 2 || ids[0] != 1 || ids[1] != 2 {
		t.Errorf("parseIDs(\" 1 , 2 \") = %v, want [1, 2]", ids)
	}
}

func TestParseIDs_InvalidID(t *testing.T) {
	_, err := parseIDs("abc")
	if err == nil {
		t.Fatal("expected error for non-numeric ID")
	}
	var cliErr *clierr.Error
	if !errors.As(err, &cliErr) {
		t.Fatalf("expected clierr.Error, got %T", err)
	}
	if cliErr.Code != clierr.InvalidTaskID {
		t.Errorf("code = %q, want %q", cliErr.Code, clierr.InvalidTaskID)
	}
}

func TestParseIDs_EmptyString(t *testing.T) {
	_, err := parseIDs("")
	if err == nil {
		t.Fatal("expected error for empty string")
	}
}

func TestCheckWIPLimit_NoLimit(t *testing.T) {
	cfg := config.NewDefault("Test")
	counts := map[string]int{"in-progress": 5}
	err := checkWIPLimit(cfg, counts, "in-progress", "")
	if err != nil {
		t.Errorf("expected nil when no WIP limit set, got %v", err)
	}
}

func TestCheckWIPLimit_WithinLimit(t *testing.T) {
	cfg := config.NewDefault("Test")
	cfg.WIPLimits = map[string]int{"in-progress": 5}
	counts := map[string]int{"in-progress": 3}
	err := checkWIPLimit(cfg, counts, "in-progress", "")
	if err != nil {
		t.Errorf("expected nil within limit, got %v", err)
	}
}

func TestCheckWIPLimit_AtLimit(t *testing.T) {
	cfg := config.NewDefault("Test")
	cfg.WIPLimits = map[string]int{"in-progress": 3}
	counts := map[string]int{"in-progress": 3}
	err := checkWIPLimit(cfg, counts, "in-progress", "")
	if err == nil {
		t.Fatal("expected error at limit")
	}
	var cliErr *clierr.Error
	if !errors.As(err, &cliErr) {
		t.Fatalf("expected clierr.Error, got %T", err)
	}
	if cliErr.Code != clierr.WIPLimitExceeded {
		t.Errorf("code = %q, want %q", cliErr.Code, clierr.WIPLimitExceeded)
	}
}

func TestCheckWIPLimit_SameStatus(t *testing.T) {
	cfg := config.NewDefault("Test")
	cfg.WIPLimits = map[string]int{"in-progress": 3}
	counts := map[string]int{"in-progress": 3}
	// Task already in target status should not count as adding.
	err := checkWIPLimit(cfg, counts, "in-progress", "in-progress")
	if err != nil {
		t.Errorf("expected nil when task already at target, got %v", err)
	}
}
