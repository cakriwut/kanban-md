package e2e_test

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestBatchMoveAll(t *testing.T) {
	kanbanDir := initBoard(t)
	mustCreateTask(t, kanbanDir, "Task A")
	mustCreateTask(t, kanbanDir, "Task B")
	mustCreateTask(t, kanbanDir, "Task C")

	var results []batchResultJSON
	runKanbanJSON(t, kanbanDir, &results, "move", "1,2,3", statusTodo)

	if len(results) != 3 {
		t.Fatalf("results = %d, want 3", len(results))
	}
	for _, r := range results {
		if !r.OK {
			t.Errorf("task #%d failed: %s", r.ID, r.Error)
		}
	}

	// Verify tasks actually moved.
	var tasks []taskJSON
	runKanbanJSON(t, kanbanDir, &tasks, "list", "--status", statusTodo)
	if len(tasks) != 3 {
		t.Errorf("todo tasks = %d, want 3", len(tasks))
	}
}

func TestBatchMovePartialFailure(t *testing.T) {
	kanbanDir := initBoardWithWIP(t, 1)
	mustCreateTask(t, kanbanDir, "Task A")
	mustCreateTask(t, kanbanDir, "Task B")
	mustCreateTask(t, kanbanDir, "Task C")

	// Move all 3 to in-progress with WIP limit of 1. First succeeds, rest fail.
	r := runKanban(t, kanbanDir, "--json", "move", "1,2,3", "in-progress", "--claim", claimTestAgent)
	if r.exitCode == 0 {
		t.Error("expected non-zero exit code for partial failure")
	}

	var results []batchResultJSON
	if err := json.Unmarshal([]byte(r.stdout), &results); err != nil {
		t.Fatalf("parsing batch results: %v\nstdout: %s", err, r.stdout)
	}

	if len(results) != 3 {
		t.Fatalf("results = %d, want 3", len(results))
	}
	if !results[0].OK {
		t.Error("first task should succeed")
	}
	if results[1].OK || results[2].OK {
		t.Error("remaining tasks should fail due to WIP limit")
	}
}

func TestBatchEditMultiple(t *testing.T) {
	kanbanDir := initBoard(t)
	mustCreateTask(t, kanbanDir, "Task A")
	mustCreateTask(t, kanbanDir, "Task B")
	mustCreateTask(t, kanbanDir, "Task C")

	var results []batchResultJSON
	runKanbanJSON(t, kanbanDir, &results, "edit", "1,2,3", "--priority", priorityHigh)

	if len(results) != 3 {
		t.Fatalf("results = %d, want 3", len(results))
	}
	for _, r := range results {
		if !r.OK {
			t.Errorf("task #%d failed: %s", r.ID, r.Error)
		}
	}

	// Verify tasks actually updated.
	var tasks []taskJSON
	runKanbanJSON(t, kanbanDir, &tasks, "list", "--priority", priorityHigh)
	if len(tasks) != 3 {
		t.Errorf("high priority tasks = %d, want 3", len(tasks))
	}
}

func TestBatchDeleteMultiple(t *testing.T) {
	kanbanDir := initBoard(t)
	mustCreateTask(t, kanbanDir, "Task A")
	mustCreateTask(t, kanbanDir, "Task B")
	mustCreateTask(t, kanbanDir, "Task C")

	var results []batchResultJSON
	runKanbanJSON(t, kanbanDir, &results, "delete", "1,2,3", "--yes")

	if len(results) != 3 {
		t.Fatalf("results = %d, want 3", len(results))
	}
	for _, r := range results {
		if !r.OK {
			t.Errorf("task #%d failed: %s", r.ID, r.Error)
		}
	}

	// Verify tasks are gone.
	var tasks []taskJSON
	runKanbanJSON(t, kanbanDir, &tasks, "list")
	if len(tasks) != 0 {
		t.Errorf("remaining tasks = %d, want 0", len(tasks))
	}
}

func TestBatchExitCode(t *testing.T) {
	kanbanDir := initBoard(t)
	mustCreateTask(t, kanbanDir, "Task A")

	// Move existing + nonexistent task. Partial failure → exit code 1.
	r := runKanban(t, kanbanDir, "--json", "move", "1,99", statusTodo)
	if r.exitCode != 1 {
		t.Errorf("exit code = %d, want 1", r.exitCode)
	}
}

func TestBatchSingleIDBackcompat(t *testing.T) {
	kanbanDir := initBoard(t)
	mustCreateTask(t, kanbanDir, "Solo task")

	// Single ID should return a task object (not batch results array).
	var result struct {
		ID      int    `json:"id"`
		Status  string `json:"status"`
		Changed bool   `json:"changed"`
	}
	runKanbanJSON(t, kanbanDir, &result, "move", "1", statusTodo)

	if result.ID != 1 {
		t.Errorf("ID = %d, want 1", result.ID)
	}
	if result.Status != statusTodo {
		t.Errorf("Status = %q, want %q", result.Status, statusTodo)
	}
	if !result.Changed {
		t.Error("Changed = false, want true")
	}
}

func TestBatchDeleteWarnsDependents(t *testing.T) {
	kanbanDir := initBoard(t)
	mustCreateTask(t, kanbanDir, "Task A")
	mustCreateTask(t, kanbanDir, "Task B")
	mustCreateTask(t, kanbanDir, "Task C")

	// Make B depend on A.
	runKanban(t, kanbanDir, "edit", "2", "--add-dep", "1")

	// Batch delete A and C — should warn about B depending on A.
	r := runKanban(t, kanbanDir, "delete", "1,3", "--yes")
	if r.exitCode != 0 {
		t.Fatalf("exit code = %d, want 0\nstderr: %s", r.exitCode, r.stderr)
	}
	if !strings.Contains(r.stderr, "Warning") {
		t.Errorf("stderr should contain dependent-task warning, got: %q", r.stderr)
	}

	// Dependent task B should still be unblocked after deleting A.
	var tasks []taskJSON
	runKanbanJSON(t, kanbanDir, &tasks, "list", "--unblocked")
	if len(tasks) != 1 || tasks[0].ID != 2 {
		t.Fatalf("unblocked tasks after batch delete = %+v, want only task #2", tasks)
	}
}

func TestBatchDeleteRequiresYes(t *testing.T) {
	kanbanDir := initBoard(t)
	mustCreateTask(t, kanbanDir, "Task A")
	mustCreateTask(t, kanbanDir, "Task B")

	errResp := runKanbanJSONError(t, kanbanDir, "delete", "1,2")
	if errResp.Code != "CONFIRMATION_REQUIRED" {
		t.Errorf("code = %q, want CONFIRMATION_REQUIRED", errResp.Code)
	}
	if !strings.Contains(errResp.Error, "batch delete requires --yes") {
		t.Errorf("error = %q, want 'batch delete requires --yes'", errResp.Error)
	}
}

func TestBatchMoveJSON(t *testing.T) {
	kanbanDir := initBoard(t)
	mustCreateTask(t, kanbanDir, "Task A")
	mustCreateTask(t, kanbanDir, "Task B")

	var results []batchResultJSON
	runKanbanJSON(t, kanbanDir, &results, "move", "1,2", statusTodo)

	if len(results) != 2 {
		t.Fatalf("results = %d, want 2", len(results))
	}
	if results[0].ID != 1 || results[1].ID != 2 {
		t.Errorf("IDs = [%d,%d], want [1,2]", results[0].ID, results[1].ID)
	}
}

// ---------------------------------------------------------------------------
// Default output format tests (table is always the default)
// ---------------------------------------------------------------------------
