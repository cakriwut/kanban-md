package e2e_test

import (
	"strings"
	"testing"
)

// ---------------------------------------------------------------------------
// Metrics tests
// ---------------------------------------------------------------------------

func TestMetricsEmpty(t *testing.T) {
	kanbanDir := initBoard(t)

	var m struct {
		Throughput7d  int `json:"throughput_7d"`
		Throughput30d int `json:"throughput_30d"`
	}
	runKanbanJSON(t, kanbanDir, &m, "metrics")

	if m.Throughput7d != 0 {
		t.Errorf("Throughput7d = %d, want 0", m.Throughput7d)
	}
	if m.Throughput30d != 0 {
		t.Errorf("Throughput30d = %d, want 0", m.Throughput30d)
	}
}

func TestMetricsWithCompletedTasks(t *testing.T) {
	kanbanDir := initBoard(t)
	mustCreateTask(t, kanbanDir, "Task A")
	mustCreateTask(t, kanbanDir, "Task B")

	// Move tasks through the workflow to get timestamps.
	runKanban(t, kanbanDir, "--json", "move", "1", "in-progress", "--claim", claimTestAgent)
	runKanban(t, kanbanDir, "--json", "move", "1", "done", "--claim", claimTestAgent)
	runKanban(t, kanbanDir, "--json", "move", "2", "in-progress", "--claim", claimTestAgent)

	var m struct {
		Throughput7d  int `json:"throughput_7d"`
		Throughput30d int `json:"throughput_30d"`
		AgingItems    []struct {
			ID     int    `json:"id"`
			Status string `json:"status"`
		} `json:"aging_items"`
	}
	runKanbanJSON(t, kanbanDir, &m, "metrics")

	if m.Throughput7d != 1 {
		t.Errorf("Throughput7d = %d, want 1", m.Throughput7d)
	}
	if m.Throughput30d != 1 {
		t.Errorf("Throughput30d = %d, want 1", m.Throughput30d)
	}
	if len(m.AgingItems) != 1 {
		t.Fatalf("AgingItems = %d, want 1", len(m.AgingItems))
	}
	if m.AgingItems[0].ID != 2 {
		t.Errorf("AgingItems[0].ID = %d, want 2", m.AgingItems[0].ID)
	}
}

func TestMetricsSinceFilter(t *testing.T) {
	kanbanDir := initBoard(t)
	mustCreateTask(t, kanbanDir, "Task A")
	runKanban(t, kanbanDir, "--json", "move", "1", "in-progress", "--claim", claimTestAgent)
	runKanban(t, kanbanDir, "--json", "move", "1", "done", "--claim", claimTestAgent)

	// Filter with a future date — completed task should be excluded from throughput.
	var m struct {
		Throughput7d  int `json:"throughput_7d"`
		Throughput30d int `json:"throughput_30d"`
	}
	runKanbanJSON(t, kanbanDir, &m, "metrics", "--since", "2099-01-01")

	if m.Throughput7d != 0 {
		t.Errorf("Throughput7d = %d, want 0 (filtered out)", m.Throughput7d)
	}
}

func TestMetricsTableOutput(t *testing.T) {
	kanbanDir := initBoard(t)

	r := runKanban(t, kanbanDir, "--table", "metrics")
	if r.exitCode != 0 {
		t.Fatalf("exit code = %d, want 0", r.exitCode)
	}
	if !strings.Contains(r.stdout, "Throughput") {
		t.Errorf("table output missing throughput:\n%s", r.stdout)
	}
	if !strings.Contains(r.stdout, "lead time") || !strings.Contains(r.stdout, "cycle time") {
		t.Errorf("table output missing time fields:\n%s", r.stdout)
	}
}

func TestMetricsInvalidSinceStructuredError(t *testing.T) {
	kanbanDir := initBoard(t)

	errResp := runKanbanJSONError(t, kanbanDir, "metrics", "--since", "not-a-date")
	if errResp.Code != codeInvalidDate {
		t.Errorf("code = %q, want INVALID_DATE", errResp.Code)
	}
}

func TestMetricsCompactOutput(t *testing.T) {
	kanbanDir := initBoard(t)
	mustCreateTask(t, kanbanDir, "Metrics test task")

	r := runKanban(t, kanbanDir, "metrics", "--compact")
	if r.exitCode != 0 {
		t.Fatalf("metrics --compact failed (exit %d): %s", r.exitCode, r.stderr)
	}
}

func TestMetricsWithSince(t *testing.T) {
	kanbanDir := initBoard(t)
	mustCreateTask(t, kanbanDir, "Old task")

	r := runKanban(t, kanbanDir, "metrics", "--since", "2020-01-01", "--json")
	if r.exitCode != 0 {
		t.Fatalf("metrics --since failed (exit %d): %s", r.exitCode, r.stderr)
	}
}

func TestMetricsWithBadSince(t *testing.T) {
	kanbanDir := initBoard(t)

	errResp := runKanbanJSONError(t, kanbanDir, "metrics", "--since", "not-a-date")
	if errResp.Code != codeInvalidDate {
		t.Errorf("error code = %q, want %q", errResp.Code, codeInvalidDate)
	}
}
