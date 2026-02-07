package board

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/antopolskiy/kanban-md/internal/config"
	"github.com/antopolskiy/kanban-md/internal/date"
	"github.com/antopolskiy/kanban-md/internal/task"
)

func newTestConfig() *config.Config {
	return config.NewDefault("Test Board")
}

func TestGenerateContextEmpty(t *testing.T) {
	cfg := newTestConfig()
	data := GenerateContext(cfg, nil, ContextOptions{})

	if data.BoardName != "Test Board" {
		t.Errorf("BoardName = %q, want %q", data.BoardName, "Test Board")
	}
	if data.Summary.TotalTasks != 0 {
		t.Errorf("TotalTasks = %d, want 0", data.Summary.TotalTasks)
	}
	if len(data.Sections) != 0 {
		t.Errorf("Sections = %d, want 0 (empty board)", len(data.Sections))
	}
}

func TestGenerateContextWithTasks(t *testing.T) {
	cfg := newTestConfig()
	now := time.Now()
	yesterday := now.Add(-24 * time.Hour)
	past := now.Add(-48 * time.Hour)
	pastDue := date.Date{Time: past}

	tasks := []*task.Task{
		{ID: 1, Title: "In progress task", Status: "in-progress", Priority: "high", Created: yesterday, Updated: now},
		{ID: 2, Title: "Blocked task", Status: "todo", Priority: "medium", Blocked: true, BlockReason: "waiting on API", Created: yesterday, Updated: now},
		{ID: 3, Title: "Ready task", Status: "todo", Priority: "critical", Created: yesterday, Updated: now},
		{ID: 4, Title: "Overdue task", Status: "in-progress", Priority: "high", Due: &pastDue, Created: yesterday, Updated: now},
		{ID: 5, Title: "Done task", Status: "done", Priority: "medium", Created: yesterday, Updated: now, Completed: &now},
	}

	const wantTotal = 5
	const wantInProgress = 4

	data := GenerateContext(cfg, tasks, ContextOptions{Days: defaultDays})

	if data.Summary.TotalTasks != wantTotal {
		t.Errorf("TotalTasks = %d, want %d", data.Summary.TotalTasks, wantTotal)
	}
	if data.Summary.InProgress != wantInProgress {
		t.Errorf("InProgress = %d, want %d", data.Summary.InProgress, wantInProgress)
	}
	if data.Summary.Blocked != 1 {
		t.Errorf("Blocked = %d, want 1", data.Summary.Blocked)
	}
	if data.Summary.Overdue != 1 {
		t.Errorf("Overdue = %d, want 1", data.Summary.Overdue)
	}

	// Should have sections for in-progress, blocked, ready, overdue, recently-completed.
	if len(data.Sections) == 0 {
		t.Fatal("expected non-empty sections")
	}

	sectionNames := make(map[string]bool)
	for _, s := range data.Sections {
		sectionNames[s.Name] = true
	}
	for _, want := range []string{sectionInProgress, sectionBlocked, sectionReady, sectionOverdue, sectionRecentlyCompleted} {
		if !sectionNames[want] {
			t.Errorf("missing section %q", want)
		}
	}
}

func TestGenerateContextSectionFilter(t *testing.T) {
	cfg := newTestConfig()
	now := time.Now()

	tasks := []*task.Task{
		{ID: 1, Title: "Active", Status: "in-progress", Priority: "high", Created: now, Updated: now},
		{ID: 2, Title: "Stuck", Status: "todo", Priority: "medium", Blocked: true, BlockReason: "dep", Created: now, Updated: now},
	}

	data := GenerateContext(cfg, tasks, ContextOptions{Sections: []string{sectionBlocked}})
	if len(data.Sections) != 1 {
		t.Fatalf("Sections = %d, want 1", len(data.Sections))
	}
	if data.Sections[0].Name != sectionBlocked {
		t.Errorf("Section = %q, want %q", data.Sections[0].Name, sectionBlocked)
	}
}

func TestRenderContextMarkdown(t *testing.T) {
	const totalTasks = 3

	data := ContextData{
		BoardName: "My Project",
		Summary: ContextSummary{
			TotalTasks: totalTasks,
			InProgress: 1,
			Blocked:    1,
			Overdue:    0,
		},
		Sections: []ContextSection{
			{
				Name: sectionInProgress,
				Items: []ContextItem{
					{ID: 1, Title: "Build API", Status: "in-progress", Priority: "high", Assignee: "alice"},
				},
			},
		},
	}

	md := RenderContextMarkdown(data)

	if !strings.HasPrefix(md, contextBeginMarker) {
		t.Error("missing begin marker")
	}
	if !strings.Contains(md, contextEndMarker) {
		t.Error("missing end marker")
	}
	if !strings.Contains(md, "## Board: My Project") {
		t.Error("missing board name")
	}
	if !strings.Contains(md, "**3 tasks**") {
		t.Error("missing summary stats")
	}
	if !strings.Contains(md, "### In Progress") {
		t.Error("missing in-progress section")
	}
	if !strings.Contains(md, "#1") {
		t.Error("missing task reference")
	}
	if !strings.Contains(md, "@alice") {
		t.Error("missing assignee")
	}
}

func TestWriteContextToFile_NewFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "context.md")
	content := contextBeginMarker + "\ntest\n" + contextEndMarker + "\n"

	if err := WriteContextToFile(path, content); err != nil {
		t.Fatalf("WriteContextToFile error: %v", err)
	}

	data, err := os.ReadFile(path) //nolint:gosec // test file path
	if err != nil {
		t.Fatalf("ReadFile error: %v", err)
	}
	if string(data) != content {
		t.Errorf("content = %q, want %q", string(data), content)
	}
}

func TestWriteContextToFile_ReplaceExisting(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "AGENTS.md")

	// Write initial file with other content and a context block.
	initial := "# My Agents\n\nSome notes.\n\n" +
		contextBeginMarker + "\nold context\n" + contextEndMarker + "\n\n" +
		"## More stuff\n"
	if err := os.WriteFile(path, []byte(initial), 0o600); err != nil {
		t.Fatal(err)
	}

	newContext := contextBeginMarker + "\nnew context\n" + contextEndMarker + "\n"
	if err := WriteContextToFile(path, newContext); err != nil {
		t.Fatalf("WriteContextToFile error: %v", err)
	}

	data, err := os.ReadFile(path) //nolint:gosec // test file path
	if err != nil {
		t.Fatal(err)
	}

	result := string(data)
	if !strings.Contains(result, "# My Agents") {
		t.Error("lost header content")
	}
	if !strings.Contains(result, "## More stuff") {
		t.Error("lost trailing content")
	}
	if strings.Contains(result, "old context") {
		t.Error("old context not replaced")
	}
	if !strings.Contains(result, "new context") {
		t.Error("new context not inserted")
	}
}

func TestWriteContextToFile_AppendToExisting(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "AGENTS.md")

	existing := "# My Agents\n\nExisting content.\n"
	if err := os.WriteFile(path, []byte(existing), 0o600); err != nil {
		t.Fatal(err)
	}

	newContext := contextBeginMarker + "\nappended\n" + contextEndMarker + "\n"
	if err := WriteContextToFile(path, newContext); err != nil {
		t.Fatalf("WriteContextToFile error: %v", err)
	}

	data, err := os.ReadFile(path) //nolint:gosec // test file path
	if err != nil {
		t.Fatal(err)
	}

	result := string(data)
	if !strings.Contains(result, "Existing content") {
		t.Error("lost existing content")
	}
	if !strings.Contains(result, "appended") {
		t.Error("context not appended")
	}
}
