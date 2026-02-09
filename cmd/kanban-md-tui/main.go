// kanban-md-tui is an interactive terminal UI for kanban-md boards.
package main

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/antopolskiy/kanban-md/internal/clierr"
	"github.com/antopolskiy/kanban-md/internal/config"
	"github.com/antopolskiy/kanban-md/internal/tui"
	"github.com/antopolskiy/kanban-md/internal/watcher"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}

func run() error {
	cfg, err := findConfig()
	if err != nil {
		// If no board found, offer to create one.
		var cliErr *clierr.Error
		if errors.As(err, &cliErr) && cliErr.Code == clierr.BoardNotFound {
			cfg, err = offerInit()
			if err != nil {
				return err
			}
		} else {
			return err
		}
	}

	model := tui.NewBoard(cfg)
	p := tea.NewProgram(model, tea.WithAltScreen())

	// Start file watcher in background.
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go startWatcher(ctx, model, p)

	_, err = p.Run()
	return err
}

func offerInit() (*config.Config, error) {
	cwd, err := os.Getwd()
	if err != nil {
		return nil, fmt.Errorf("getting working directory: %w", err)
	}
	name := filepath.Base(cwd)
	kanbanDir := filepath.Join(cwd, config.DefaultDir)

	fmt.Printf("No kanban board found. Create one in %s? [Y/n] ", kanbanDir)
	reader := bufio.NewReader(os.Stdin)
	answer, _ := reader.ReadString('\n')
	answer = strings.TrimSpace(strings.ToLower(answer))

	if answer != "" && answer != "y" && answer != "yes" {
		return nil, errors.New("no board found — run 'kanban-md init' to create one")
	}

	cfg, err := config.Init(kanbanDir, name)
	if err != nil {
		return nil, fmt.Errorf("initializing board: %w", err)
	}

	fmt.Printf("Board %q created in %s\n", name, kanbanDir)
	return cfg, nil
}

func startWatcher(ctx context.Context, model *tui.Board, p *tea.Program) {
	paths := model.WatchPaths()
	w, err := watcher.New(paths, func() {
		p.Send(tui.ReloadMsg{})
	})
	if err != nil {
		return // non-fatal: TUI works without live refresh
	}
	defer w.Close()
	w.Run(ctx, nil)
}

func findConfig() (*config.Config, error) {
	// Check for --dir flag (simple flag parsing).
	dir := ""
	for i, arg := range os.Args[1:] {
		if arg == "--dir" && i+1 < len(os.Args)-2 {
			dir = os.Args[i+2]
			break
		}
	}

	if dir != "" {
		return config.Load(dir)
	}

	cwd, err := os.Getwd()
	if err != nil {
		return nil, fmt.Errorf("getting working directory: %w", err)
	}

	kanbanDir, err := config.FindDir(cwd)
	if err != nil {
		return nil, err
	}

	return config.Load(kanbanDir)
}
