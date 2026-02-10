package e2e_test

import (
	"strings"
	"testing"
)

// ---------------------------------------------------------------------------
// Archive command tests
// ---------------------------------------------------------------------------

func TestArchiveCommand(t *testing.T) {
	kanbanDir := initBoard(t)
	mustCreateTask(t, kanbanDir, "Archive me")

	// Archive the task.
	r := runKanban(t, kanbanDir, "archive", "1")
	if r.exitCode != 0 {
		t.Fatalf("archive failed (exit %d): %s", r.exitCode, r.stderr)
	}
	if !strings.Contains(r.stdout, "Archived task #1") {
		t.Error("expected 'Archived task #1' in output")
	}

	// Verify the task is archived.
	var task taskJSON
	runKanbanJSON(t, kanbanDir, &task, "show", "1")
	if task.Status != "archived" {
		t.Errorf("status = %q, want %q", task.Status, "archived")
	}
}

func TestArchiveIdempotent(t *testing.T) {
	kanbanDir := initBoard(t)
	mustCreateTask(t, kanbanDir, "Already archived")

	// Archive the task.
	runKanban(t, kanbanDir, "archive", "1")
	// Archive again — should be idempotent.
	r := runKanban(t, kanbanDir, "archive", "1")
	if r.exitCode != 0 {
		t.Fatalf("archive (idempotent) failed (exit %d): %s", r.exitCode, r.stderr)
	}
	if !strings.Contains(r.stdout, "already archived") {
		t.Error("expected 'already archived' in idempotent output")
	}
}

func TestArchiveExcludedFromList(t *testing.T) {
	kanbanDir := initBoard(t)
	mustCreateTask(t, kanbanDir, "Visible task")
	mustCreateTask(t, kanbanDir, "Will archive")

	// Archive task 2.
	runKanban(t, kanbanDir, "archive", "2")

	// Default list should NOT include archived task.
	var tasks []taskJSON
	runKanbanJSON(t, kanbanDir, &tasks, "list")
	if len(tasks) != 1 {
		t.Fatalf("list returned %d tasks, want 1", len(tasks))
	}
	if tasks[0].ID != 1 {
		t.Errorf("task ID = %d, want 1", tasks[0].ID)
	}

	// --archived should show only archived tasks.
	var archived []taskJSON
	runKanbanJSON(t, kanbanDir, &archived, "list", "--archived")
	if len(archived) != 1 {
		t.Fatalf("list --archived returned %d tasks, want 1", len(archived))
	}
	if archived[0].ID != 2 {
		t.Errorf("archived task ID = %d, want 2", archived[0].ID)
	}

	// Explicit --status archived should also work.
	var explicit []taskJSON
	runKanbanJSON(t, kanbanDir, &explicit, "list", "--status", "archived")
	if len(explicit) != 1 {
		t.Fatalf("list --status archived returned %d tasks, want 1", len(explicit))
	}
}

func TestArchiveExcludedFromBoard(t *testing.T) {
	kanbanDir := initBoard(t)
	mustCreateTask(t, kanbanDir, "Active task")
	mustCreateTask(t, kanbanDir, "Will archive")

	runKanban(t, kanbanDir, "archive", "2")

	// Board should show 1 task total, not 2.
	r := runKanban(t, kanbanDir, "--compact", "board")
	if r.exitCode != 0 {
		t.Fatalf("board failed: %s", r.stderr)
	}
	if strings.Contains(r.stdout, "archived") {
		t.Error("board compact should not contain 'archived' column")
	}
}

func TestArchiveBatch(t *testing.T) {
	kanbanDir := initBoard(t)
	mustCreateTask(t, kanbanDir, "Batch 1")
	mustCreateTask(t, kanbanDir, "Batch 2")
	mustCreateTask(t, kanbanDir, "Batch 3")

	r := runKanban(t, kanbanDir, "archive", "1,2,3")
	if r.exitCode != 0 {
		t.Fatalf("batch archive failed (exit %d): %s", r.exitCode, r.stderr)
	}
	if !strings.Contains(r.stdout, "3/3") {
		t.Error("expected '3/3' in batch output")
	}

	// All tasks should be archived.
	var tasks []taskJSON
	runKanbanJSON(t, kanbanDir, &tasks, "list", "--archived")
	if len(tasks) != 3 {
		t.Errorf("archived tasks = %d, want 3", len(tasks))
	}
}

func TestArchiveSatisfiesDependencies(t *testing.T) {
	kanbanDir := initBoard(t)
	mustCreateTask(t, kanbanDir, "Dep task")
	runKanban(t, kanbanDir, "--json", "create", "Blocked task", "--depends-on", "1")

	// Archive task 1 (dep).
	runKanban(t, kanbanDir, "archive", "1")

	// Task 2 should now show as unblocked.
	var tasks []taskJSON
	runKanbanJSON(t, kanbanDir, &tasks, "list", "--unblocked")
	found := false
	for _, tk := range tasks {
		if tk.ID == 2 {
			found = true
			break
		}
	}
	if !found {
		t.Error("task 2 should be unblocked after archiving task 1")
	}
}

func TestArchiveJSON(t *testing.T) {
	kanbanDir := initBoard(t)
	mustCreateTask(t, kanbanDir, "JSON archive")

	var result struct {
		taskJSON
		Changed bool `json:"changed"`
	}
	runKanbanJSON(t, kanbanDir, &result, "archive", "1")
	if !result.Changed {
		t.Error("changed should be true")
	}
	if result.Status != "archived" {
		t.Errorf("status = %q, want archived", result.Status)
	}
}

// ---------------------------------------------------------------------------
// Require-claim enforcement tests
// ---------------------------------------------------------------------------
