package e2e_test

import (
	"strings"
	"testing"
)

// ---------------------------------------------------------------------------
// Create tests
// ---------------------------------------------------------------------------

func TestCreateBasic(t *testing.T) {
	kanbanDir := initBoard(t)

	var task taskJSON
	r := runKanbanJSON(t, kanbanDir, &task, "create", "My first task")

	if r.exitCode != 0 {
		t.Fatalf("create failed: %s", r.stderr)
	}
	if task.ID != 1 {
		t.Errorf("ID = %d, want 1", task.ID)
	}
	if task.Title != "My first task" {
		t.Errorf("Title = %q, want %q", task.Title, "My first task")
	}
	if task.Status != statusBacklog {
		t.Errorf("Status = %q, want %q (default)", task.Status, statusBacklog)
	}
	if task.Priority != "medium" {
		t.Errorf("Priority = %q, want %q (default)", task.Priority, "medium")
	}
}

func TestCreateWithAllFlags(t *testing.T) {
	kanbanDir := initBoard(t)

	var task taskJSON
	runKanbanJSON(t, kanbanDir, &task, "create", "Full task",
		"--status", statusTodo,
		"--priority", "high",
		"--assignee", assigneeAlice,
		"--tags", "backend,api",
		"--due", "2026-03-15",
		"--estimate", "4h",
		"--body", "Task description here",
	)

	if task.Status != statusTodo {
		t.Errorf("Status = %q, want %q", task.Status, statusTodo)
	}
	if task.Priority != priorityHigh {
		t.Errorf("Priority = %q, want %q", task.Priority, priorityHigh)
	}
	if task.Assignee != assigneeAlice {
		t.Errorf("Assignee = %q, want %q", task.Assignee, assigneeAlice)
	}
	if len(task.Tags) != 2 || task.Tags[0] != "backend" || task.Tags[1] != "api" {
		t.Errorf("Tags = %v, want [backend api]", task.Tags)
	}
	if task.Due != "2026-03-15" {
		t.Errorf("Due = %q, want %q", task.Due, "2026-03-15")
	}
	if task.Estimate != "4h" {
		t.Errorf("Estimate = %q, want %q", task.Estimate, "4h")
	}
	if task.Body != "Task description here" {
		t.Errorf("Body = %q, want %q", task.Body, "Task description here")
	}
}

func TestCreateTagAlias(t *testing.T) {
	kanbanDir := initBoard(t)

	var task taskJSON
	runKanbanJSON(t, kanbanDir, &task, "create", "Tag alias test", "--tag", "bug,feature")

	if len(task.Tags) != 2 || task.Tags[0] != "bug" || task.Tags[1] != "feature" {
		t.Errorf("Tags = %v, want [bug feature]", task.Tags)
	}
}

func TestCreateIncrementID(t *testing.T) {
	kanbanDir := initBoard(t)

	t1 := mustCreateTask(t, kanbanDir, "First")
	t2 := mustCreateTask(t, kanbanDir, "Second")
	t3 := mustCreateTask(t, kanbanDir, "Third")

	if t1.ID != 1 || t2.ID != 2 || t3.ID != 3 {
		t.Errorf("IDs = [%d, %d, %d], want [1, 2, 3]", t1.ID, t2.ID, t3.ID)
	}
}

func TestCreateInvalidStatus(t *testing.T) {
	kanbanDir := initBoard(t)

	errResp := runKanbanJSONError(t, kanbanDir, "create", "Bad task", "--status", "nonexistent")
	if errResp.Code != codeInvalidStatus {
		t.Errorf("code = %q, want INVALID_STATUS", errResp.Code)
	}
	if !strings.Contains(errResp.Error, "invalid status") {
		t.Errorf("error = %q, want 'invalid status'", errResp.Error)
	}
}

func TestCreateBadDateFormat(t *testing.T) {
	kanbanDir := initBoard(t)

	errResp := runKanbanJSONError(t, kanbanDir, "create", "Bad date", "--due", "02-15-2026")
	if errResp.Code != codeInvalidDate {
		t.Errorf("code = %q, want INVALID_DATE", errResp.Code)
	}
	if !strings.Contains(errResp.Error, "invalid") {
		t.Errorf("error = %q, want 'invalid'", errResp.Error)
	}
}

// ---------------------------------------------------------------------------
// List tests
// ---------------------------------------------------------------------------

func TestCreateRespectsWIPLimit(t *testing.T) {
	kanbanDir := initBoardWithWIP(t, 2)

	// Fill in-progress to the limit.
	mustCreateTask(t, kanbanDir, "Task A", "--status", "in-progress")
	mustCreateTask(t, kanbanDir, "Task B", "--status", "in-progress")

	// Third task to in-progress should fail.
	errResp := runKanbanJSONError(t, kanbanDir, "create", "Task C", "--status", "in-progress")
	if errResp.Code != codeWIPLimitExceeded {
		t.Errorf("code = %q, want WIP_LIMIT_EXCEEDED", errResp.Code)
	}
}

func TestCreateExpediteClassWIPCheck(t *testing.T) {
	kanbanDir := initBoard(t)
	// Create an expedite task.
	mustCreateTask(t, kanbanDir, "Expedite 1", "--class", "expedite")

	// Second expedite task should be blocked by the board-wide expedite limit (1).
	errResp := runKanbanJSONError(t, kanbanDir, "create", "Expedite 2", "--class", "expedite")
	if errResp.Code != "CLASS_WIP_EXCEEDED" {
		t.Errorf("error code = %q, want CLASS_WIP_EXCEEDED", errResp.Code)
	}
}

// ---------------------------------------------------------------------------
// Move command: claim during move, compact output
// ---------------------------------------------------------------------------
