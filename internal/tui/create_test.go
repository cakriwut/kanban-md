package tui_test

import (
	"os"
	"path/filepath"
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/antopolskiy/kanban-md/internal/config"
	"github.com/antopolskiy/kanban-md/internal/tui"
)

func TestCreate_DialogOpensAndCloses(t *testing.T) {
	b, _ := setupTestBoard(t)

	// Press 'c' to open create dialog.
	b = sendKey(b, "c")
	v := b.View()
	if !containsStr(v, "Create task in") {
		t.Error("expected create dialog, got:", v[:min(len(v), 200)])
	}

	// Press esc to cancel.
	b = sendSpecialKey(b, tea.KeyEscape)
	v = b.View()
	if containsStr(v, "Create task in") {
		t.Error("expected board view after esc, but create dialog is still showing")
	}
}

func TestCreate_TypesTitle(t *testing.T) {
	b, _ := setupTestBoard(t)

	// Open create dialog.
	b = sendKey(b, "c")

	// Type a title.
	for _, ch := range "New task" {
		m, _ := b.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{ch}})
		b = m.(*tui.Board)
	}

	v := b.View()
	if !containsStr(v, "New task") {
		t.Error("expected typed title in dialog")
	}
}

func TestCreate_BackspaceDeletesCharacter(t *testing.T) {
	b, _ := setupTestBoard(t)

	b = sendKey(b, "c")
	// Type "AB" then backspace.
	for _, ch := range "AB" {
		m, _ := b.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{ch}})
		b = m.(*tui.Board)
	}
	b = sendSpecialKey(b, tea.KeyBackspace)

	v := b.View()
	// Should show "A" not "AB".
	if containsStr(v, "AB") {
		t.Error("backspace should have deleted the last character")
	}
}

func TestCreate_EnterCreatesTask(t *testing.T) {
	b, cfg := setupTestBoard(t)

	// Navigate to todo column (l once from backlog).
	b = sendKey(b, "l")

	// Open create dialog.
	b = sendKey(b, "c")

	// Type task title.
	for _, ch := range "My new task" {
		m, _ := b.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{ch}})
		b = m.(*tui.Board)
	}

	// Press enter to create.
	b = sendSpecialKey(b, tea.KeyEnter)

	// Board should be back to normal view with the new task visible.
	v := b.View()
	if !containsStr(v, "My new task") {
		t.Error("expected new task in board view")
	}

	// Verify the task file was created.
	entries, err := os.ReadDir(cfg.TasksPath())
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, e := range entries {
		if containsStr(e.Name(), "my-new-task") {
			found = true
			break
		}
	}
	if !found {
		t.Error("expected task file to be created in tasks directory")
	}
}

func TestCreate_EmptyTitleCancels(t *testing.T) {
	b, cfg := setupTestBoard(t)

	initialCount := countTaskFiles(t, cfg.TasksPath())

	// Open and immediately press enter (empty title).
	b = sendKey(b, "c")
	b = sendSpecialKey(b, tea.KeyEnter)

	// Should be back at board with no new task.
	v := b.View()
	if containsStr(v, "Create task in") {
		t.Error("expected board view after empty enter")
	}

	if got := countTaskFiles(t, cfg.TasksPath()); got != initialCount {
		t.Errorf("task count changed: %d -> %d, empty title should not create", initialCount, got)
	}
}

func TestCreate_UsesCurrentColumnStatus(t *testing.T) {
	b, _ := setupTestBoard(t)

	// Navigate to in-progress column (l twice from backlog).
	b = sendKey(b, "l") // todo
	b = sendKey(b, "l") // in-progress

	// Open create dialog — should show "in-progress" in the prompt.
	b = sendKey(b, "c")
	v := b.View()
	if !containsStr(v, "in-progress") {
		t.Error("expected 'in-progress' in create dialog")
	}
}

func TestCreate_NextIDIncrements(t *testing.T) {
	b, cfg := setupTestBoard(t)

	initialNextID := cfg.NextID

	// Create a task.
	b = sendKey(b, "c")
	for _, ch := range "Increment test" {
		m, _ := b.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{ch}})
		b = m.(*tui.Board)
	}
	_ = sendSpecialKey(b, tea.KeyEnter)

	// Reload config and check NextID incremented.
	reloaded, err := config.Load(cfg.Dir())
	if err != nil {
		t.Fatal(err)
	}
	if reloaded.NextID != initialNextID+1 {
		t.Errorf("NextID = %d, want %d", reloaded.NextID, initialNextID+1)
	}
}

func TestCreate_SpaceInTitle(t *testing.T) {
	b, _ := setupTestBoard(t)

	b = sendKey(b, "c")

	// Type with explicit space key.
	for _, ch := range "hello" {
		m, _ := b.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{ch}})
		b = m.(*tui.Board)
	}
	m, _ := b.Update(tea.KeyMsg{Type: tea.KeySpace})
	b = m.(*tui.Board)
	for _, ch := range "world" {
		m, _ = b.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{ch}})
		b = m.(*tui.Board)
	}

	v := b.View()
	if !containsStr(v, "hello world") {
		t.Error("expected 'hello world' in dialog")
	}
}

func TestCreate_StatusBarShowsCreateHint(t *testing.T) {
	b, _ := setupTestBoard(t)
	v := b.View()
	if !containsStr(v, "c:create") {
		t.Error("expected 'c:create' in status bar")
	}
}

func countTaskFiles(t *testing.T, dir string) int {
	t.Helper()
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	count := 0
	for _, e := range entries {
		if filepath.Ext(e.Name()) == ".md" {
			count++
		}
	}
	return count
}
