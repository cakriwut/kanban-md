package tui_test

import (
	"flag"
	"os"
	"path/filepath"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/muesli/termenv"

	"github.com/antopolskiy/kanban-md/internal/config"
	"github.com/antopolskiy/kanban-md/internal/tui"
)

var update = flag.Bool("update", false, "update golden files")

func init() {
	// Strip all ANSI codes so golden files are plain text.
	lipgloss.SetColorProfile(termenv.Ascii)
}

func assertGolden(t *testing.T, name, got string) {
	t.Helper()
	path := filepath.Join("testdata", name+".golden")

	if *update {
		if err := os.MkdirAll("testdata", 0o750); err != nil {
			t.Fatalf("creating testdata dir: %v", err)
		}
		if err := os.WriteFile(path, []byte(got), 0o600); err != nil {
			t.Fatalf("writing golden file: %v", err)
		}
		return
	}

	want, err := os.ReadFile(path) //nolint:gosec // test golden file path is safe
	if err != nil {
		t.Fatalf("reading golden file %s (run with -update to create): %v", path, err)
	}

	if got != string(want) {
		// Write "got" file for easy comparison.
		gotPath := path + ".got"
		_ = os.WriteFile(gotPath, []byte(got), 0o600)
		t.Errorf("output does not match golden file %s\n  got file: %s\n  run with -update to accept changes", path, gotPath)
	}
}

func TestSnapshot_BoardView(t *testing.T) {
	b, _ := setupTestBoard(t)
	assertGolden(t, "board_view", b.View())
}

func TestSnapshot_DetailView(t *testing.T) {
	b, _ := setupTestBoard(t)
	b = sendKey(b, "enter") // open detail
	assertGolden(t, "detail_view", b.View())
}

func TestSnapshot_MoveDialog(t *testing.T) {
	b, _ := setupTestBoard(t)
	b = sendKey(b, "m") // open move dialog
	assertGolden(t, "move_dialog", b.View())
}

func TestSnapshot_DeleteConfirm(t *testing.T) {
	b, _ := setupTestBoard(t)
	b = sendKey(b, "d") // open delete confirm
	assertGolden(t, "delete_confirm", b.View())
}

func TestSnapshot_HelpView(t *testing.T) {
	b, _ := setupTestBoard(t)
	b = sendKey(b, "?") // open help
	assertGolden(t, "help_view", b.View())
}

func TestSnapshot_BoardView80(t *testing.T) {
	b, _ := setupTestBoard80(t)
	assertGolden(t, "board_view_80", b.View())
}

func setupTestBoard80(t *testing.T) (*tui.Board, *config.Config) {
	t.Helper()
	b, cfg := setupTestBoard(t)
	b.Update(tea.WindowSizeMsg{Width: 80, Height: 24})
	return b, cfg
}

func TestSnapshot_BoardView60(t *testing.T) {
	b, _ := setupTestBoard(t)
	b.Update(tea.WindowSizeMsg{Width: 60, Height: 24})
	assertGolden(t, "board_view_60", b.View())
}

func TestSnapshot_EmptyBoard(t *testing.T) {
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
	assertGolden(t, "empty_board", b.View())
}
