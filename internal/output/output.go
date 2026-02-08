// Package output handles formatting CLI output as table or JSON.
package output

import (
	"os"

	"golang.org/x/term"
)

// Format represents an output format.
type Format int

const (
	// FormatAuto detects based on TTY.
	FormatAuto Format = iota
	// FormatJSON outputs JSON.
	FormatJSON
	// FormatTable outputs a human-readable table.
	FormatTable
)

// isTerminalFn checks whether stdout is a terminal. Replaceable in tests.
var isTerminalFn = func() bool {
	return term.IsTerminal(int(os.Stdout.Fd()))
}

// Detect returns the appropriate format based on flags, environment, and TTY.
// When no explicit format is set: TTY → table, piped → JSON.
func Detect(jsonFlag, tableFlag bool) Format {
	if jsonFlag {
		return FormatJSON
	}
	if tableFlag {
		return FormatTable
	}

	// Check environment variable.
	switch os.Getenv("KANBAN_OUTPUT") {
	case "json":
		return FormatJSON
	case "table":
		return FormatTable
	}

	// Auto-detect: TTY gets table, piped gets JSON.
	if isTerminalFn() {
		return FormatTable
	}
	return FormatJSON
}
