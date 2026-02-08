package tui_test

import (
	"os"
	"path/filepath"
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/antopolskiy/kanban-md/internal/config"
	"github.com/antopolskiy/kanban-md/internal/task"
	"github.com/antopolskiy/kanban-md/internal/tui"
)

// setupTestBoard creates a temp kanban directory with a config and tasks,
// then returns a Board model ready for testing.
func setupTestBoard(t *testing.T) (*tui.Board, *config.Config) {
	t.Helper()

	dir := t.TempDir()
	kanbanDir := filepath.Join(dir, "kanban")
	tasksDir := filepath.Join(kanbanDir, "tasks")

	if err := os.MkdirAll(tasksDir, 0o750); err != nil {
		t.Fatalf("creating dirs: %v", err)
	}

	cfg := config.NewDefault("Test Board")
	cfg.SetDir(kanbanDir)
	if err := cfg.Save(); err != nil {
		t.Fatalf("saving config: %v", err)
	}

	// Create test tasks.
	tasks := []struct {
		id       int
		title    string
		status   string
		priority string
	}{
		{1, "Task A", "backlog", "high"},
		{2, "Task B", "backlog", "medium"},
		{3, "Task C", "in-progress", "high"},
		{4, "Task D", "done", "low"},
	}

	for _, tt := range tasks {
		tk := &task.Task{
			ID:       tt.id,
			Title:    tt.title,
			Status:   tt.status,
			Priority: tt.priority,
		}
		path := filepath.Join(tasksDir, task.GenerateFilename(tt.id, tt.title))
		if err := task.Write(path, tk); err != nil {
			t.Fatalf("writing task: %v", err)
		}
	}

	b := tui.NewBoard(cfg)
	// Simulate window size.
	b.Update(tea.WindowSizeMsg{Width: 120, Height: 40})

	return b, cfg
}

func sendKey(b *tui.Board, k string) *tui.Board {
	m, _ := b.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(k)})
	return m.(*tui.Board)
}

func sendSpecialKey(b *tui.Board, k tea.KeyType) *tui.Board {
	m, _ := b.Update(tea.KeyMsg{Type: k})
	return m.(*tui.Board)
}

func TestBoard_InitialState(t *testing.T) {
	b, _ := setupTestBoard(t)
	v := b.View()

	// Should show all status columns.
	if v == "" || v == "Loading..." {
		t.Error("expected board view, got empty or loading")
	}

	// Board should contain task titles.
	if !containsStr(v, "Task A") {
		t.Error("expected Task A in view")
	}
	if !containsStr(v, "Task C") {
		t.Error("expected Task C in view")
	}
}

func TestBoard_NavigateColumns(t *testing.T) {
	b, _ := setupTestBoard(t)

	// Move right twice, then left twice — should not panic.
	b = sendKey(b, "l")
	b = sendKey(b, "l")
	b = sendKey(b, "h")
	b = sendKey(b, "h")

	// Moving left past column 0 should not panic.
	b = sendKey(b, "h")

	// View should render without issues.
	v := b.View()
	if v == "" || v == "Loading..." {
		t.Error("expected valid board view after navigation")
	}
}

func TestBoard_NavigateRows(t *testing.T) {
	b, _ := setupTestBoard(t)

	// Column 0 (backlog) has 2 tasks. Move down.
	b = sendKey(b, "j")

	// Move down again should not crash (already at last).
	b = sendKey(b, "j")

	// Move up back.
	b = sendKey(b, "k")

	// Should not panic.
	_ = b.View()
}

func TestBoard_EnterDetail(t *testing.T) {
	b, _ := setupTestBoard(t)

	// Press enter to see detail.
	b = sendKey(b, "enter")
	v := b.View()

	// Detail view should show task fields.
	if !containsStr(v, "Status:") {
		t.Error("expected detail view with Status field")
	}

	// Press esc to go back.
	b = sendSpecialKey(b, tea.KeyEsc)
	v = b.View()

	// Should be back to board.
	if containsStr(v, "Press q/esc to go back") {
		t.Error("expected to return to board view")
	}
}

func TestBoard_MoveDialog(t *testing.T) {
	b, _ := setupTestBoard(t)

	// Press m to open move dialog.
	b = sendKey(b, "m")
	v := b.View()

	if !containsStr(v, "Move #") {
		t.Error("expected move dialog")
	}
	if !containsStr(v, "(current)") {
		t.Error("expected current status marker in move dialog")
	}

	// Press esc to cancel.
	b = sendSpecialKey(b, tea.KeyEsc)
	_ = b.View()
}

func TestBoard_MoveTask(t *testing.T) {
	b, cfg := setupTestBoard(t)

	// Select task in backlog, move to todo.
	b = sendKey(b, "m")

	// Move cursor down to "todo".
	b = sendKey(b, "j")

	// Press enter to confirm.
	_ = sendKey(b, "enter")

	// Verify the task was actually moved.
	path, err := task.FindByID(cfg.TasksPath(), 1)
	if err != nil {
		t.Fatalf("finding task: %v", err)
	}
	tk, err := task.Read(path)
	if err != nil {
		t.Fatalf("reading task: %v", err)
	}
	if tk.Status != "todo" {
		t.Errorf("expected status 'todo', got %q", tk.Status)
	}
}

func TestBoard_MoveNext(t *testing.T) {
	b, cfg := setupTestBoard(t)

	// Press M (shift-m) to move to next status.
	b = sendKey(b, "M")

	// Task 1 was in backlog, should now be in todo.
	path, err := task.FindByID(cfg.TasksPath(), 1)
	if err != nil {
		t.Fatalf("finding task: %v", err)
	}
	tk, err := task.Read(path)
	if err != nil {
		t.Fatalf("reading task: %v", err)
	}
	if tk.Status != "todo" {
		t.Errorf("expected status 'todo', got %q", tk.Status)
	}

	_ = b.View()
}

func TestBoard_DeleteTask(t *testing.T) {
	b, cfg := setupTestBoard(t)

	// Press d to start delete.
	b = sendKey(b, "d")
	v := b.View()

	if !containsStr(v, "Delete task?") {
		t.Error("expected delete confirmation dialog")
	}

	// Press y to confirm.
	b = sendKey(b, "y")

	// Task 1 should be gone.
	_, err := task.FindByID(cfg.TasksPath(), 1)
	if err == nil {
		t.Error("expected task 1 to be deleted")
	}

	_ = b.View()
}

func TestBoard_DeleteCancel(t *testing.T) {
	b, cfg := setupTestBoard(t)

	b = sendKey(b, "d")
	b = sendKey(b, "n")

	// Task should still exist.
	_, err := task.FindByID(cfg.TasksPath(), 1)
	if err != nil {
		t.Error("expected task 1 to still exist after cancel")
	}

	_ = b.View()
}

func TestBoard_HelpView(t *testing.T) {
	b, _ := setupTestBoard(t)

	b = sendKey(b, "?")
	v := b.View()

	if !containsStr(v, "Keyboard Shortcuts") {
		t.Error("expected help view")
	}

	// Any key should close help.
	b = sendKey(b, "q")
	v = b.View()

	if containsStr(v, "Keyboard Shortcuts") {
		t.Error("expected help view to close")
	}
}

func TestBoard_Refresh(t *testing.T) {
	b, cfg := setupTestBoard(t)

	// Create a new task externally.
	tk := &task.Task{
		ID:       5,
		Title:    "External Task",
		Status:   "todo",
		Priority: "medium",
	}
	path := filepath.Join(cfg.TasksPath(), task.GenerateFilename(5, "External Task"))
	if err := task.Write(path, tk); err != nil {
		t.Fatalf("writing task: %v", err)
	}

	// Press r to refresh.
	b = sendKey(b, "r")
	v := b.View()

	if !containsStr(v, "External Task") {
		t.Error("expected External Task in view after refresh")
	}
}

func TestBoard_EmptyBoard(t *testing.T) {
	dir := t.TempDir()
	kanbanDir := filepath.Join(dir, "kanban")
	tasksDir := filepath.Join(kanbanDir, "tasks")

	if err := os.MkdirAll(tasksDir, 0o750); err != nil {
		t.Fatalf("creating dirs: %v", err)
	}

	cfg := config.NewDefault("Empty Board")
	cfg.SetDir(kanbanDir)
	if err := cfg.Save(); err != nil {
		t.Fatalf("saving config: %v", err)
	}

	b := tui.NewBoard(cfg)
	b.Update(tea.WindowSizeMsg{Width: 120, Height: 40})

	v := b.View()
	if !containsStr(v, "(empty)") {
		t.Error("expected empty column indicator")
	}
}

func containsStr(haystack, needle string) bool {
	return len(haystack) > 0 && len(needle) > 0 &&
		haystack != needle && // avoid trivial match
		findSubstring(haystack, needle)
}

func findSubstring(s, sub string) bool {
	for i := 0; i <= len(s)-len(sub); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}
