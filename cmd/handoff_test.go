package cmd

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/antopolskiy/kanban-md/internal/config"
	"github.com/antopolskiy/kanban-md/internal/task"
)

func TestLogHandoffActivity_MoveAndBlock(t *testing.T) {
	kanbanDir := setupBoard(t)
	cfg, err := config.Load(kanbanDir)
	if err != nil {
		t.Fatal(err)
	}

	tk := &task.Task{ID: 1, Title: "test", Status: "review", Blocked: true, BlockReason: "waiting"}
	logHandoffActivity(cfg, tk, "in-progress")

	logPath := filepath.Join(kanbanDir, "activity.jsonl")
	data, err := os.ReadFile(logPath) //nolint:gosec // test path
	if err != nil {
		t.Fatal(err)
	}
	got := string(data)
	if !containsSubstring(got, "move") {
		t.Errorf("expected 'move' in log, got: %s", got)
	}
	if !containsSubstring(got, "handoff") {
		t.Errorf("expected 'handoff' in log, got: %s", got)
	}
	if !containsSubstring(got, "block") {
		t.Errorf("expected 'block' in log, got: %s", got)
	}
}

func TestLogHandoffActivity_ReleaseOnly(t *testing.T) {
	kanbanDir := setupBoard(t)
	cfg, err := config.Load(kanbanDir)
	if err != nil {
		t.Fatal(err)
	}

	// ClaimedBy="" means claim was released.
	tk := &task.Task{ID: 1, Title: "test", Status: "review", ClaimedBy: ""}
	logHandoffActivity(cfg, tk, "review")

	logPath := filepath.Join(kanbanDir, "activity.jsonl")
	data, err := os.ReadFile(logPath) //nolint:gosec // test path
	if err != nil {
		t.Fatal(err)
	}
	got := string(data)
	if !containsSubstring(got, "release") {
		t.Errorf("expected 'release' in log, got: %s", got)
	}
	// Should not log move (same status).
	if containsSubstring(got, "move") {
		t.Errorf("should not log 'move' when status unchanged, got: %s", got)
	}
}
