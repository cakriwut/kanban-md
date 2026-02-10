package e2e_test

import (
	"strings"
	"testing"
)

// ---------------------------------------------------------------------------
// Show tests
// ---------------------------------------------------------------------------

func TestShow(t *testing.T) {
	kanbanDir := initBoard(t)
	mustCreateTask(t, kanbanDir, "Show me", "--body", "Detailed description",
		"--assignee", assigneeAlice, "--tags", "test")

	var task taskJSON
	r := runKanbanJSON(t, kanbanDir, &task, "show", "1")

	if r.exitCode != 0 {
		t.Fatalf("show failed: %s", r.stderr)
	}
	if task.ID != 1 {
		t.Errorf("ID = %d, want 1", task.ID)
	}
	if task.Title != "Show me" {
		t.Errorf("Title = %q, want %q", task.Title, "Show me")
	}
	if !strings.Contains(task.Body, "Detailed description") {
		t.Errorf("Body = %q, want to contain %q", task.Body, "Detailed description")
	}
	if task.Assignee != assigneeAlice {
		t.Errorf("Assignee = %q, want %q", task.Assignee, assigneeAlice)
	}
}

func TestShowNotFound(t *testing.T) {
	kanbanDir := initBoard(t)

	errResp := runKanbanJSONError(t, kanbanDir, "show", "999")
	if errResp.Code != "TASK_NOT_FOUND" {
		t.Errorf("code = %q, want TASK_NOT_FOUND", errResp.Code)
	}
}

func TestShowInvalidID(t *testing.T) {
	kanbanDir := initBoard(t)

	errResp := runKanbanJSONError(t, kanbanDir, "show", "abc")
	if errResp.Code != "INVALID_TASK_ID" {
		t.Errorf("code = %q, want INVALID_TASK_ID", errResp.Code)
	}
}

// ---------------------------------------------------------------------------
// Edit tests
// ---------------------------------------------------------------------------

func TestShowDisplaysLeadCycleTime(t *testing.T) {
	kanbanDir := initBoard(t)
	mustCreateTask(t, kanbanDir, "Task A")
	runKanban(t, kanbanDir, "--json", "move", "1", statusTodo)
	runKanban(t, kanbanDir, "--json", "move", "1", "done")

	r := runKanban(t, kanbanDir, "--table", "show", "1")
	if r.exitCode != 0 {
		t.Fatalf("show failed: %s", r.stderr)
	}
	if !strings.Contains(r.stdout, "Lead time") {
		t.Errorf("show output missing 'Lead time', got: %s", r.stdout)
	}
	if !strings.Contains(r.stdout, "Cycle time") {
		t.Errorf("show output missing 'Cycle time', got: %s", r.stdout)
	}
}

// ---------------------------------------------------------------------------
// Dependency tests
// ---------------------------------------------------------------------------

func TestShowCompactOutput(t *testing.T) {
	kanbanDir := initBoard(t)
	mustCreateTask(t, kanbanDir, "Compact show test")

	r := runKanban(t, kanbanDir, "show", "1", "--compact")
	if r.exitCode != 0 {
		t.Fatalf("show --compact failed (exit %d): %s", r.exitCode, r.stderr)
	}
	if !strings.Contains(r.stdout, "Compact show test") {
		t.Error("compact show output should contain task title")
	}
}
