package app

import (
	"fmt"

	tea "charm.land/bubbletea/v2"
)

// Print emits a line ABOVE the pinned status bar without disturbing the bar.
// Implements TUI-04: rich output from anywhere in the program, dispatched
// through the bubbletea command queue so the alt-screen renderer composites
// it correctly.
//
// Returns a tea.Cmd that callers send back through the Update -> tea.Batch
// pipeline. This is a tea.Cmd factory — never spawn goroutines to "print
// asynchronously".
//
// Usage from inside Update:
//
//	return m, app.Print("classified file: %s", path)
func Print(format string, a ...any) tea.Cmd {
	return tea.Println(fmt.Sprintf(format, a...))
}
