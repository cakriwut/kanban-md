package e2e_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// ---------------------------------------------------------------------------
// Context command tests
// ---------------------------------------------------------------------------

func TestContextEmptyBoard(t *testing.T) {
	kanbanDir := initBoard(t)

	var ctx struct {
		BoardName string `json:"board_name"`
		Summary   struct {
			TotalTasks int `json:"total_tasks"`
		} `json:"summary"`
		Sections []struct {
			Name string `json:"name"`
		} `json:"sections"`
	}
	runKanbanJSON(t, kanbanDir, &ctx, "context")

	if ctx.Summary.TotalTasks != 0 {
		t.Errorf("TotalTasks = %d, want 0", ctx.Summary.TotalTasks)
	}
	if len(ctx.Sections) != 0 {
		t.Errorf("Sections = %d, want 0", len(ctx.Sections))
	}
}

func TestContextWithTasks(t *testing.T) {
	kanbanDir := initBoard(t)
	mustCreateTask(t, kanbanDir, "Active task")
	mustCreateTask(t, kanbanDir, "Blocked task")
	runKanban(t, kanbanDir, "--json", "move", "1", "in-progress", "--claim", claimTestAgent)
	runKanban(t, kanbanDir, "--json", "edit", "2", "--block", "waiting")

	var ctx struct {
		Summary struct {
			TotalTasks int `json:"total_tasks"`
			Active     int `json:"active"`
			Blocked    int `json:"blocked"`
		} `json:"summary"`
		Sections []struct {
			Name  string `json:"name"`
			Items []struct {
				ID int `json:"id"`
			} `json:"items"`
		} `json:"sections"`
	}
	runKanbanJSON(t, kanbanDir, &ctx, "context")

	if ctx.Summary.TotalTasks != 2 {
		t.Errorf("TotalTasks = %d, want 2", ctx.Summary.TotalTasks)
	}
	if ctx.Summary.Active < 1 {
		t.Errorf("Active = %d, want >= 1", ctx.Summary.Active)
	}
	if ctx.Summary.Blocked != 1 {
		t.Errorf("Blocked = %d, want 1", ctx.Summary.Blocked)
	}

	// Should have at least in-progress and blocked sections.
	sectionNames := make(map[string]bool)
	for _, s := range ctx.Sections {
		sectionNames[s.Name] = true
	}
	if !sectionNames["in-progress"] {
		t.Error("missing in-progress section")
	}
	if !sectionNames["blocked"] {
		t.Error("missing blocked section")
	}
}

func TestContextJSON(t *testing.T) {
	kanbanDir := initBoard(t)
	mustCreateTask(t, kanbanDir, "JSON task")

	var ctx struct {
		BoardName string `json:"board_name"`
		Summary   struct {
			TotalTasks int `json:"total_tasks"`
		} `json:"summary"`
	}
	runKanbanJSON(t, kanbanDir, &ctx, "context")

	if ctx.BoardName == "" {
		t.Error("BoardName is empty")
	}
	if ctx.Summary.TotalTasks != 1 {
		t.Errorf("TotalTasks = %d, want 1", ctx.Summary.TotalTasks)
	}
}

func TestContextWriteToFile_NewFile(t *testing.T) {
	kanbanDir := initBoard(t)
	mustCreateTask(t, kanbanDir, "Written task")
	runKanban(t, kanbanDir, "--json", "move", "1", "in-progress", "--claim", claimTestAgent)

	outFile := filepath.Join(t.TempDir(), "context.md")
	runKanban(t, kanbanDir, "context", "--write-to", outFile)

	data, err := os.ReadFile(outFile) //nolint:gosec // test file path
	if err != nil {
		t.Fatalf("reading output file: %v", err)
	}

	content := string(data)
	if !strings.Contains(content, "<!-- BEGIN kanban-md context -->") {
		t.Error("missing begin marker")
	}
	if !strings.Contains(content, "<!-- END kanban-md context -->") {
		t.Error("missing end marker")
	}
	if !strings.Contains(content, "Written task") {
		t.Error("missing task in context")
	}
}

func TestContextWriteToFile_UpdateExisting(t *testing.T) {
	kanbanDir := initBoard(t)
	mustCreateTask(t, kanbanDir, "First run")
	runKanban(t, kanbanDir, "--json", "move", "1", "in-progress", "--claim", claimTestAgent)

	outFile := filepath.Join(t.TempDir(), "AGENTS.md")

	// Write initial file with other content.
	initial := "# My Agents\n\nImportant notes here.\n"
	if err := os.WriteFile(outFile, []byte(initial), 0o600); err != nil {
		t.Fatal(err)
	}

	// First write — appends context block.
	runKanban(t, kanbanDir, "context", "--write-to", outFile)

	// Create another task and move it, then write again — should update in place.
	mustCreateTask(t, kanbanDir, "Second run")
	runKanban(t, kanbanDir, "--json", "move", "2", "in-progress", "--claim", claimTestAgent)
	runKanban(t, kanbanDir, "context", "--write-to", outFile)

	data, err := os.ReadFile(outFile) //nolint:gosec // test file path
	if err != nil {
		t.Fatal(err)
	}
	content := string(data)

	if !strings.Contains(content, "# My Agents") {
		t.Error("lost header content")
	}
	if !strings.Contains(content, "Important notes here") {
		t.Error("lost body content")
	}
	if !strings.Contains(content, "Second run") {
		t.Error("missing updated task")
	}

	// Should only have one begin/end marker pair.
	if strings.Count(content, "<!-- BEGIN kanban-md context -->") != 1 {
		t.Error("multiple begin markers found — in-place update failed")
	}
}

func TestContextWriteToFile_AppendToExisting(t *testing.T) {
	kanbanDir := initBoard(t)
	mustCreateTask(t, kanbanDir, "Appended task")

	outFile := filepath.Join(t.TempDir(), "README.md")
	existing := "# README\n\nSome documentation.\n"
	if err := os.WriteFile(outFile, []byte(existing), 0o600); err != nil {
		t.Fatal(err)
	}

	runKanban(t, kanbanDir, "context", "--write-to", outFile)

	data, err := os.ReadFile(outFile) //nolint:gosec // test file path
	if err != nil {
		t.Fatal(err)
	}
	content := string(data)

	if !strings.Contains(content, "# README") {
		t.Error("lost existing content")
	}
	if !strings.Contains(content, "<!-- BEGIN kanban-md context -->") {
		t.Error("context not appended")
	}
}

func TestContextSectionsFilter(t *testing.T) {
	kanbanDir := initBoard(t)
	mustCreateTask(t, kanbanDir, "Filtered task")
	runKanban(t, kanbanDir, "--json", "move", "1", "in-progress", "--claim", claimTestAgent)
	runKanban(t, kanbanDir, "--json", "edit", "1", "--block", "test", "--claim", claimTestAgent)

	var ctx struct {
		Sections []struct {
			Name string `json:"name"`
		} `json:"sections"`
	}
	runKanbanJSON(t, kanbanDir, &ctx, "context", "--sections", "blocked")

	if len(ctx.Sections) != 1 {
		t.Fatalf("Sections = %d, want 1", len(ctx.Sections))
	}
	if ctx.Sections[0].Name != "blocked" {
		t.Errorf("Section = %q, want %q", ctx.Sections[0].Name, "blocked")
	}
}

func TestContextSectionsReadyIsIgnored(t *testing.T) {
	kanbanDir := initBoard(t)
	mustCreateTask(t, kanbanDir, "Accepted-ish task")
	runKanban(t, kanbanDir, "--json", "move", "1", statusTodo)

	var ctx struct {
		Sections []struct {
			Name string `json:"name"`
		} `json:"sections"`
	}
	runKanbanJSON(t, kanbanDir, &ctx, "context", "--sections", "ready")

	if len(ctx.Sections) != 0 {
		t.Fatalf("Sections = %d, want 0", len(ctx.Sections))
	}
}

// ---------------------------------------------------------------------------
// Batch operations tests
// ---------------------------------------------------------------------------

type batchResultJSON struct {
	ID    int    `json:"id"`
	OK    bool   `json:"ok"`
	Error string `json:"error,omitempty"`
	Code  string `json:"code,omitempty"`
}
