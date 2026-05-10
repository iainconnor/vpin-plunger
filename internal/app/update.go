package app

import (
	tea "charm.land/bubbletea/v2"

	"github.com/iainconnor/vpin-plunger/internal/ui/components"
)

// Update is the ONLY place application state mutates. Per CLAUDE.md, no raw
// goroutines are spawned here — long-running work returns a tea.Cmd.
//
// Handling order:
//  1. tea.WindowSizeMsg — record dimensions, propagate to sub-components so
//     they can rebuild width-dependent layout (TUI-06: graceful resize).
//  2. tea.KeyMsg — global hotkeys (ctrl+c) before delegating to sub-components.
//  3. All other msgs — fan out to sub-components and batch the returned cmds.
func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmds []tea.Cmd

	switch m2 := msg.(type) {
	case tea.WindowSizeMsg:
		m.Width = m2.Width
		m.Height = m2.Height
		// Width and Bar resize are handled inside StatusBar.Update (WindowSizeMsg
		// case) — no direct field write needed here. Picker likewise owns its own
		// resize. Both components receive the message via the fan-out below.
	case tea.KeyMsg:
		// Global quit on ctrl+c regardless of focus state. The Picker handles
		// its own 'q' shortcut (only when not focused on the input) so we do
		// NOT intercept 'q' here.
		if m2.String() == "ctrl+c" {
			return m, tea.Quit
		}
	}

	// Delegate to StatusBar.
	updatedSB, sbCmd := m.StatusBar.Update(msg)
	if sb, ok := updatedSB.(components.StatusBar); ok {
		m.StatusBar = sb
	}
	cmds = append(cmds, sbCmd)

	// Delegate to Picker.
	updatedPicker, pCmd := m.Picker.Update(msg)
	if p, ok := updatedPicker.(components.Picker); ok {
		m.Picker = p
	}
	cmds = append(cmds, pCmd)

	return m, tea.Batch(cmds...)
}
