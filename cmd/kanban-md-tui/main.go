// kanban-md-tui is a backward-compatible wrapper that launches the
// interactive terminal UI. Prefer 'kanban-md tui' instead.
package main

import (
	"fmt"
	"os"

	"github.com/antopolskiy/kanban-md/cmd"
)

func main() {
	// Parse --dir flag for backward compatibility.
	dir := parseDir()
	if err := cmd.RunTUI(dir); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}

func parseDir() string {
	for i, arg := range os.Args[1:] {
		if arg == "--dir" && i+1 < len(os.Args[1:])-1 {
			return os.Args[i+2]
		}
	}
	return ""
}
