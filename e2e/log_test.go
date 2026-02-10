package e2e_test

import (
	"strings"
	"testing"
)

func TestLogInvalidSinceStructuredError(t *testing.T) {
	kanbanDir := initBoard(t)

	errResp := runKanbanJSONError(t, kanbanDir, "log", "--since", "not-a-date")
	if errResp.Code != codeInvalidDate {
		t.Errorf("code = %q, want INVALID_DATE", errResp.Code)
	}
}

// ---------------------------------------------------------------------------
// Activity log tests
// ---------------------------------------------------------------------------

type logEntry struct {
	Action string `json:"action"`
	TaskID int    `json:"task_id"`
	Detail string `json:"detail"`
}

func TestLogEmptyBoard(t *testing.T) {
	kanbanDir := initBoard(t)

	var entries []logEntry
	runKanbanJSON(t, kanbanDir, &entries, "log")

	if len(entries) != 0 {
		t.Errorf("got %d entries, want 0", len(entries))
	}
}

func TestLogAfterCreate(t *testing.T) {
	kanbanDir := initBoard(t)
	mustCreateTask(t, kanbanDir, "Log me")

	var entries []logEntry
	runKanbanJSON(t, kanbanDir, &entries, "log")

	if len(entries) != 1 {
		t.Fatalf("got %d entries, want 1", len(entries))
	}
	if entries[0].Action != "create" {
		t.Errorf("action = %q, want %q", entries[0].Action, "create")
	}
	if entries[0].TaskID != 1 {
		t.Errorf("task_id = %d, want 1", entries[0].TaskID)
	}
}

func TestLogAfterMove(t *testing.T) {
	kanbanDir := initBoard(t)
	mustCreateTask(t, kanbanDir, "Mover")
	runKanban(t, kanbanDir, "--json", "move", "1", statusTodo)

	var entries []logEntry
	runKanbanJSON(t, kanbanDir, &entries, "log")

	if len(entries) != 2 {
		t.Fatalf("got %d entries, want 2", len(entries))
	}
	if entries[1].Action != "move" {
		t.Errorf("action = %q, want %q", entries[1].Action, "move")
	}
	if !strings.Contains(entries[1].Detail, "->") {
		t.Errorf("detail = %q, want to contain '->'", entries[1].Detail)
	}
}

func TestLogAfterEdit(t *testing.T) {
	kanbanDir := initBoard(t)
	mustCreateTask(t, kanbanDir, "Editor")
	runKanban(t, kanbanDir, "--json", "edit", "1", "--priority", "high")

	var entries []logEntry
	runKanbanJSON(t, kanbanDir, &entries, "log")

	if len(entries) != 2 {
		t.Fatalf("got %d entries, want 2", len(entries))
	}
	if entries[1].Action != "edit" {
		t.Errorf("action = %q, want %q", entries[1].Action, "edit")
	}
}

func TestLogAfterDelete(t *testing.T) {
	kanbanDir := initBoard(t)
	mustCreateTask(t, kanbanDir, "Deletable")
	runKanban(t, kanbanDir, "--json", "delete", "1", "--yes")

	var entries []logEntry
	runKanbanJSON(t, kanbanDir, &entries, "log")

	if len(entries) != 2 {
		t.Fatalf("got %d entries, want 2", len(entries))
	}
	if entries[1].Action != "delete" {
		t.Errorf("action = %q, want %q", entries[1].Action, "delete")
	}
}

func TestLogBlockUnblock(t *testing.T) {
	kanbanDir := initBoard(t)
	mustCreateTask(t, kanbanDir, "Blocker")
	runKanban(t, kanbanDir, "--json", "edit", "1", "--block", "waiting")
	runKanban(t, kanbanDir, "--json", "edit", "1", "--unblock")

	var entries []logEntry
	runKanbanJSON(t, kanbanDir, &entries, "log")

	// create, edit+block, edit+unblock
	actions := make(map[string]int)
	for _, e := range entries {
		actions[e.Action]++
	}
	if actions["block"] != 1 {
		t.Errorf("block actions = %d, want 1", actions["block"])
	}
	if actions["unblock"] != 1 {
		t.Errorf("unblock actions = %d, want 1", actions["unblock"])
	}
}

func TestLogActionFilter(t *testing.T) {
	kanbanDir := initBoard(t)
	mustCreateTask(t, kanbanDir, "Task A")
	runKanban(t, kanbanDir, "--json", "move", "1", statusTodo)

	var entries []logEntry
	runKanbanJSON(t, kanbanDir, &entries, "log", "--action", "move")

	if len(entries) != 1 {
		t.Fatalf("got %d entries, want 1", len(entries))
	}
	if entries[0].Action != "move" {
		t.Errorf("action = %q, want %q", entries[0].Action, "move")
	}
}

func TestLogTaskFilter(t *testing.T) {
	kanbanDir := initBoard(t)
	mustCreateTask(t, kanbanDir, "Task A")
	mustCreateTask(t, kanbanDir, "Task B")

	var entries []logEntry
	runKanbanJSON(t, kanbanDir, &entries, "log", "--task", "2")

	if len(entries) != 1 {
		t.Fatalf("got %d entries, want 1", len(entries))
	}
	if entries[0].TaskID != 2 {
		t.Errorf("task_id = %d, want 2", entries[0].TaskID)
	}
}

func TestLogLimit(t *testing.T) {
	kanbanDir := initBoard(t)
	mustCreateTask(t, kanbanDir, "Task A")
	mustCreateTask(t, kanbanDir, "Task B")
	mustCreateTask(t, kanbanDir, "Task C")

	var entries []logEntry
	runKanbanJSON(t, kanbanDir, &entries, "log", "--limit", "2")

	if len(entries) != 2 {
		t.Fatalf("got %d entries, want 2", len(entries))
	}
}

func TestLogTableOutput(t *testing.T) {
	kanbanDir := initBoard(t)
	mustCreateTask(t, kanbanDir, "Table entry")

	r := runKanban(t, kanbanDir, "--table", "log")
	if r.exitCode != 0 {
		t.Fatalf("exit code = %d, want 0", r.exitCode)
	}
	if !strings.Contains(r.stdout, "TIMESTAMP") || !strings.Contains(r.stdout, "ACTION") {
		t.Errorf("table header not found in output:\n%s", r.stdout)
	}
}

// ---------------------------------------------------------------------------
// Slug / output tests
// ---------------------------------------------------------------------------

func TestLogCompactOutput(t *testing.T) {
	kanbanDir := initBoard(t)
	mustCreateTask(t, kanbanDir, "Log test task")

	r := runKanban(t, kanbanDir, "log", "--compact")
	if r.exitCode != 0 {
		t.Fatalf("log --compact failed (exit %d): %s", r.exitCode, r.stderr)
	}
	// Activity log should have the create entry.
	if !strings.Contains(r.stdout, "create") {
		t.Error("compact log output should contain 'create' action")
	}
}

func TestLogWithFilters(t *testing.T) {
	kanbanDir := initBoard(t)
	mustCreateTask(t, kanbanDir, "First task")
	mustCreateTask(t, kanbanDir, "Second task")

	// Filter by action.
	r := runKanban(t, kanbanDir, "log", "--action", "create", "--json")
	if r.exitCode != 0 {
		t.Fatalf("log --action create failed (exit %d): %s", r.exitCode, r.stderr)
	}

	// Filter by task ID.
	r = runKanban(t, kanbanDir, "log", "--task", "1", "--json")
	if r.exitCode != 0 {
		t.Fatalf("log --task 1 failed (exit %d): %s", r.exitCode, r.stderr)
	}

	// Limit.
	r = runKanban(t, kanbanDir, "log", "--limit", "1", "--json")
	if r.exitCode != 0 {
		t.Fatalf("log --limit 1 failed (exit %d): %s", r.exitCode, r.stderr)
	}
}

func TestLogWithBadSince(t *testing.T) {
	kanbanDir := initBoard(t)

	errResp := runKanbanJSONError(t, kanbanDir, "log", "--since", "not-a-date")
	if errResp.Code != codeInvalidDate {
		t.Errorf("error code = %q, want %q", errResp.Code, codeInvalidDate)
	}
}

// ---------------------------------------------------------------------------
// Pick command tests
// ---------------------------------------------------------------------------
