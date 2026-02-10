package e2e_test

import (
	"strings"
	"testing"
)

// ---------------------------------------------------------------------------
// Handoff command tests
// ---------------------------------------------------------------------------

func TestHandoffBasic(t *testing.T) {
	kanbanDir := initBoard(t)
	mustCreateTask(t, kanbanDir, "Handoff target")
	runKanban(t, kanbanDir, "move", "1", "in-progress", "--claim", claimTestAgent)

	var task taskJSON
	r := runKanbanJSON(t, kanbanDir, &task, "handoff", "1", "--claim", claimTestAgent)
	if r.exitCode != 0 {
		t.Fatalf("handoff failed: %s", r.stderr)
	}
	if task.Status != statusReview {
		t.Errorf("Status = %q, want %q", task.Status, "review")
	}
}

func TestHandoffWithNote(t *testing.T) {
	kanbanDir := initBoard(t)
	mustCreateTask(t, kanbanDir, "Note target")
	runKanban(t, kanbanDir, "move", "1", "in-progress", "--claim", claimTestAgent)

	var task taskJSON
	r := runKanbanJSON(t, kanbanDir, &task, "handoff", "1",
		"--claim", claimTestAgent, "--note", "Ready to merge branch X")
	if r.exitCode != 0 {
		t.Fatalf("handoff failed: %s", r.stderr)
	}
	if !strings.Contains(task.Body, "Ready to merge branch X") {
		t.Errorf("Body = %q, want note text", task.Body)
	}
}

func TestHandoffWithTimestamp(t *testing.T) {
	kanbanDir := initBoard(t)
	mustCreateTask(t, kanbanDir, "TS target")
	runKanban(t, kanbanDir, "move", "1", "in-progress", "--claim", claimTestAgent)

	var task taskJSON
	r := runKanbanJSON(t, kanbanDir, &task, "handoff", "1",
		"--claim", claimTestAgent, "--note", "progress", "-t")
	if r.exitCode != 0 {
		t.Fatalf("handoff failed: %s", r.stderr)
	}
	if !strings.Contains(task.Body, "[[") || !strings.Contains(task.Body, "]]") {
		t.Errorf("Body should contain timestamp markers, got %q", task.Body)
	}
	if !strings.Contains(task.Body, "progress") {
		t.Errorf("Body should contain note text, got %q", task.Body)
	}
}

func TestHandoffWithBlock(t *testing.T) {
	kanbanDir := initBoard(t)
	mustCreateTask(t, kanbanDir, "Block target")
	runKanban(t, kanbanDir, "move", "1", "in-progress", "--claim", claimTestAgent)

	var task taskJSON
	r := runKanbanJSON(t, kanbanDir, &task, "handoff", "1",
		"--claim", claimTestAgent, "--block", "Waiting on user: which approach?")
	if r.exitCode != 0 {
		t.Fatalf("handoff failed: %s", r.stderr)
	}
	if task.Status != statusReview {
		t.Errorf("Status = %q, want %q", task.Status, "review")
	}
	if !task.Blocked {
		t.Error("expected task to be blocked")
	}
	if task.BlockReason != "Waiting on user: which approach?" {
		t.Errorf("BlockReason = %q, want %q", task.BlockReason, "Waiting on user: which approach?")
	}
}

func TestHandoffWithRelease(t *testing.T) {
	kanbanDir := initBoard(t)
	mustCreateTask(t, kanbanDir, "Release target")
	runKanban(t, kanbanDir, "move", "1", "in-progress", "--claim", claimTestAgent)

	var task taskJSON
	r := runKanbanJSON(t, kanbanDir, &task, "handoff", "1",
		"--claim", claimTestAgent, "--release")
	if r.exitCode != 0 {
		t.Fatalf("handoff failed: %s", r.stderr)
	}
	if task.ClaimedBy != "" {
		t.Errorf("ClaimedBy = %q, want empty (released)", task.ClaimedBy)
	}
}

func TestHandoffFullWorkflow(t *testing.T) {
	kanbanDir := initBoard(t)
	mustCreateTask(t, kanbanDir, "Full handoff", "--body", "existing context")
	runKanban(t, kanbanDir, "move", "1", "in-progress", "--claim", claimTestAgent)

	var task taskJSON
	r := runKanbanJSON(t, kanbanDir, &task, "handoff", "1",
		"--claim", claimTestAgent,
		"--block", "Waiting on user: deploy creds",
		"--note", "Branch: task/1-full-handoff\nNext step: push after creds",
		"-t", "--release")
	if r.exitCode != 0 {
		t.Fatalf("handoff failed: %s", r.stderr)
	}
	if task.Status != statusReview {
		t.Errorf("Status = %q, want review", task.Status)
	}
	if !task.Blocked {
		t.Error("expected blocked")
	}
	if task.ClaimedBy != "" {
		t.Errorf("expected released, got ClaimedBy=%q", task.ClaimedBy)
	}
	if !strings.Contains(task.Body, "existing context") {
		t.Errorf("Body should preserve existing content, got %q", task.Body)
	}
	if !strings.Contains(task.Body, "Branch: task/1-full-handoff") {
		t.Errorf("Body should contain note, got %q", task.Body)
	}
}

func TestHandoffRequiresClaim(t *testing.T) {
	kanbanDir := initBoard(t)
	mustCreateTask(t, kanbanDir, "No claim target")

	errResp := runKanbanJSONError(t, kanbanDir, "handoff", "1")
	if errResp.Code != codeInvalidInput {
		t.Errorf("code = %q, want %q", errResp.Code, codeInvalidInput)
	}
}

func TestHandoffEmptyBlockReason(t *testing.T) {
	kanbanDir := initBoard(t)
	mustCreateTask(t, kanbanDir, "Empty block")

	errResp := runKanbanJSONError(t, kanbanDir, "handoff", "1",
		"--claim", claimTestAgent, "--block", "")
	if errResp.Code != codeInvalidInput {
		t.Errorf("code = %q, want %q", errResp.Code, codeInvalidInput)
	}
}

// ---------------------------------------------------------------------------
// Read command coverage tests (list, show, board, log, metrics)
// ---------------------------------------------------------------------------
