package e2e_test

import (
	"strconv"
	"strings"
	"testing"
)

// ---------------------------------------------------------------------------
// Board summary tests
// ---------------------------------------------------------------------------

func TestBoardSummaryEmpty(t *testing.T) {
	kanbanDir := initBoard(t)

	var summary struct {
		BoardName  string `json:"board_name"`
		TotalTasks int    `json:"total_tasks"`
		Statuses   []struct {
			Status  string `json:"status"`
			Count   int    `json:"count"`
			Blocked int    `json:"blocked"`
			Overdue int    `json:"overdue"`
		} `json:"statuses"`
		Priorities []struct {
			Priority string `json:"priority"`
			Count    int    `json:"count"`
		} `json:"priorities"`
	}
	runKanbanJSON(t, kanbanDir, &summary, "board")

	if summary.TotalTasks != 0 {
		t.Errorf("TotalTasks = %d, want 0", summary.TotalTasks)
	}
	if len(summary.Statuses) != 5 {
		t.Errorf("Statuses count = %d, want 5", len(summary.Statuses))
	}
	for _, ss := range summary.Statuses {
		if ss.Count != 0 {
			t.Errorf("status %q count = %d, want 0", ss.Status, ss.Count)
		}
	}
}

func TestBoardSummaryWithTasks(t *testing.T) {
	kanbanDir := initBoard(t)
	mustCreateTask(t, kanbanDir, "Task A", "--priority", "high")
	mustCreateTask(t, kanbanDir, "Task B", "--priority", "high")
	mustCreateTask(t, kanbanDir, "Task C", "--priority", "low")
	runKanban(t, kanbanDir, "--json", "move", "1", statusTodo)
	runKanban(t, kanbanDir, "--json", "move", "3", "done")

	var summary struct {
		TotalTasks int `json:"total_tasks"`
		Statuses   []struct {
			Status string `json:"status"`
			Count  int    `json:"count"`
		} `json:"statuses"`
		Priorities []struct {
			Priority string `json:"priority"`
			Count    int    `json:"count"`
		} `json:"priorities"`
	}
	runKanbanJSON(t, kanbanDir, &summary, "board")

	if summary.TotalTasks != 3 {
		t.Fatalf("TotalTasks = %d, want 3", summary.TotalTasks)
	}

	statusCounts := make(map[string]int)
	for _, ss := range summary.Statuses {
		statusCounts[ss.Status] = ss.Count
	}
	if statusCounts["backlog"] != 1 {
		t.Errorf("backlog = %d, want 1", statusCounts["backlog"])
	}
	if statusCounts[statusTodo] != 1 {
		t.Errorf("todo = %d, want 1", statusCounts[statusTodo])
	}
	if statusCounts["done"] != 1 {
		t.Errorf("done = %d, want 1", statusCounts["done"])
	}

	prioMap := make(map[string]int)
	for _, pc := range summary.Priorities {
		prioMap[pc.Priority] = pc.Count
	}
	if prioMap["high"] != 2 {
		t.Errorf("high = %d, want 2", prioMap["high"])
	}
	if prioMap["low"] != 1 {
		t.Errorf("low = %d, want 1", prioMap["low"])
	}
}

func TestBoardSummaryTableOutput(t *testing.T) {
	kanbanDir := initBoard(t)
	mustCreateTask(t, kanbanDir, "Table test")

	r := runKanban(t, kanbanDir, "--table", "board")
	if r.exitCode != 0 {
		t.Fatalf("exit code = %d, want 0", r.exitCode)
	}
	if !strings.Contains(r.stdout, "STATUS") || !strings.Contains(r.stdout, "COUNT") {
		t.Errorf("table header not found in output:\n%s", r.stdout)
	}
	if !strings.Contains(r.stdout, "PRIORITY") {
		t.Errorf("priority section not found in output:\n%s", r.stdout)
	}
}

func TestBoardSummaryAlias(t *testing.T) {
	kanbanDir := initBoard(t)

	var summary struct {
		TotalTasks int `json:"total_tasks"`
	}
	runKanbanJSON(t, kanbanDir, &summary, "summary")

	if summary.TotalTasks != 0 {
		t.Errorf("TotalTasks via alias = %d, want 0", summary.TotalTasks)
	}
}

// ---------------------------------------------------------------------------
// Metrics tests
// ---------------------------------------------------------------------------

func TestBoardGroupByStatus(t *testing.T) {
	kanbanDir := initBoard(t)
	mustCreateTask(t, kanbanDir, "Task A", "--status", statusTodo)
	taskB := mustCreateTask(t, kanbanDir, "Task B")
	runKanban(t, kanbanDir, "move", strconv.Itoa(taskB.ID), "in-progress", "--claim", claimTestAgent)

	// Table output.
	r := runKanban(t, kanbanDir, "board", "--group-by", "status")
	if r.exitCode != 0 {
		t.Fatalf("board --group-by status failed (exit %d): %s", r.exitCode, r.stderr)
	}

	// JSON output.
	var grouped map[string]interface{}
	r = runKanbanJSON(t, kanbanDir, &grouped, "board", "--group-by", "status")
	if r.exitCode != 0 {
		t.Fatalf("board --group-by --json failed (exit %d): %s", r.exitCode, r.stderr)
	}
}

func TestBoardGroupByInvalid(t *testing.T) {
	kanbanDir := initBoard(t)

	errResp := runKanbanJSONError(t, kanbanDir, "board", "--group-by", "invalid-field")
	if errResp.Code != "INVALID_GROUP_BY" {
		t.Errorf("error code = %q, want INVALID_GROUP_BY", errResp.Code)
	}
}

func TestBoardCompactOutput(t *testing.T) {
	kanbanDir := initBoard(t)
	mustCreateTask(t, kanbanDir, "Board compact task")

	r := runKanban(t, kanbanDir, "board", "--compact")
	if r.exitCode != 0 {
		t.Fatalf("board --compact failed (exit %d): %s", r.exitCode, r.stderr)
	}
}
