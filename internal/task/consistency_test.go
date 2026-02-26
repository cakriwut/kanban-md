package task

import (
	"os"
	"path/filepath"
	"slices"
	"testing"
	"time"

	"github.com/antopolskiy/kanban-md/internal/config"
)

func TestEnsureConsistency_RepairsDuplicateMismatchAndNextID(t *testing.T) {
	cfg := setupConsistencyFixture(t)

	report, err := EnsureConsistency(cfg)
	if err != nil {
		t.Fatalf("EnsureConsistency error: %v", err)
	}
	assertInitialRepairReport(t, report)
	assertConsistencyResult(t, cfg)

	report, err = EnsureConsistency(cfg)
	if err != nil {
		t.Fatalf("EnsureConsistency second run error: %v", err)
	}
	if len(report.Repairs) != 0 {
		t.Fatalf("repairs on second run = %d, want 0", len(report.Repairs))
	}
}

func setupConsistencyFixture(t *testing.T) *config.Config {
	t.Helper()

	kanbanDir := t.TempDir()
	cfg := config.NewDefault("Test")
	cfg.SetDir(kanbanDir)
	cfg.NextID = 2
	if err := os.MkdirAll(cfg.TasksPath(), 0o750); err != nil {
		t.Fatal(err)
	}
	if err := cfg.Save(); err != nil {
		t.Fatal(err)
	}

	now := time.Date(2026, 2, 24, 12, 0, 0, 0, time.UTC)
	mustWriteTask(t, filepath.Join(cfg.TasksPath(), "001-first-task.md"), &Task{
		ID:       1,
		Title:    "First task",
		Status:   "backlog",
		Priority: "medium",
		Created:  now,
		Updated:  now,
	})
	mustWriteTask(t, filepath.Join(cfg.TasksPath(), "002-duplicate-task.md"), &Task{
		ID:       1, // duplicate ID on purpose
		Title:    "Duplicate task",
		Status:   "backlog",
		Priority: "medium",
		Created:  now,
		Updated:  now,
	})
	mustWriteTask(t, filepath.Join(cfg.TasksPath(), "099-mismatch-task.md"), &Task{
		ID:       3,
		Title:    "Mismatch task",
		Status:   "todo",
		Priority: "high",
		Created:  now,
		Updated:  now,
	})
	if err := os.WriteFile(filepath.Join(cfg.TasksPath(), "004-bad.md"), []byte("not frontmatter"), 0o600); err != nil {
		t.Fatal(err)
	}

	return cfg
}

func mustWriteTask(t *testing.T, path string, tk *Task) {
	t.Helper()
	if err := Write(path, tk); err != nil {
		t.Fatal(err)
	}
}

func assertInitialRepairReport(t *testing.T, report ConsistencyReport) {
	t.Helper()
	if len(report.Warnings) != 1 {
		t.Fatalf("warnings = %d, want 1", len(report.Warnings))
	}
	if len(report.Repairs) < 3 {
		t.Fatalf("repairs = %d, want at least 3 (%v)", len(report.Repairs), report.Repairs)
	}
}

func assertConsistencyResult(t *testing.T, cfg *config.Config) {
	t.Helper()

	tasks, warnings, err := ReadAllLenient(cfg.TasksPath())
	if err != nil {
		t.Fatalf("ReadAllLenient error: %v", err)
	}
	if len(warnings) != 1 {
		t.Fatalf("warnings after repair = %d, want 1 malformed warning", len(warnings))
	}
	if len(tasks) != 3 {
		t.Fatalf("task count after repair = %d, want 3", len(tasks))
	}

	gotIDs := make([]int, 0, len(tasks))
	for _, tk := range tasks {
		gotIDs = append(gotIDs, tk.ID)
	}
	slices.Sort(gotIDs)
	wantIDs := []int{1, 3, 4}
	if !slices.Equal(gotIDs, wantIDs) {
		t.Fatalf("IDs after repair = %v, want %v", gotIDs, wantIDs)
	}
	if cfg.NextID != 5 {
		t.Fatalf("cfg.NextID = %d, want 5", cfg.NextID)
	}

	assertTaskFilename(t, cfg, 1, "001-first-task.md")
	assertTaskFilename(t, cfg, 3, "003-mismatch-task.md")
	assertTaskFilename(t, cfg, 4, "004-duplicate-task.md")
}

func assertTaskFilename(t *testing.T, cfg *config.Config, id int, wantBase string) {
	t.Helper()

	path, err := FindByID(cfg.TasksPath(), id)
	if err != nil {
		t.Fatalf("FindByID(%d): %v", id, err)
	}
	if filepath.Base(path) != wantBase {
		t.Fatalf("task #%d filename = %s, want %s", id, filepath.Base(path), wantBase)
	}
}
