package app

import (
	"strings"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/lipgloss"
)

// View is pure: no I/O, no state changes, no goroutines.
//
// Layout (top to bottom):
//   - Picker content (candidates, search input, action row) — only when a
//     picker state is active (PendingMatch != nil, Confirming,
//     StateCatalogFreshCheck, or StateRehearsalWipeCheck)
//   - Vertical filler so the StatusBar pins to the bottom regardless of how
//     little content the Picker emits
//   - StatusBar (always 1 line tall, full terminal width)
//
// TUI-01: the StatusBar is ALWAYS the last segment, so it is pinned at the
// bottom. lipgloss.JoinVertical performs the composition.
func (m Model) View() tea.View {
	bar := m.StatusBar.Render()

	showPicker := m.PendingMatch != nil ||
		m.Confirming ||
		m.State == StateCatalogFreshCheck ||
		m.State == StateRehearsalWipeCheck
	var picker string
	if showPicker {
		picker = m.Picker.Render()
	}

	var composed string
	if m.Height <= 0 {
		if showPicker {
			composed = lipgloss.JoinVertical(lipgloss.Left, picker, bar)
		} else {
			composed = bar
		}
	} else {
		pickerLines := 0
		if showPicker {
			pickerLines = strings.Count(strings.TrimRight(picker, "\n"), "\n") + 1
		}
		filler := m.Height - pickerLines - 1
		if filler < 0 {
			filler = 0
		}
		if showPicker {
			composed = lipgloss.JoinVertical(lipgloss.Left, picker,
				strings.Repeat("\n", filler), bar)
		} else {
			composed = lipgloss.JoinVertical(lipgloss.Left,
				strings.Repeat("\n", filler), bar)
		}
	}

	v := tea.NewView(composed)
	v.AltScreen = true
	return v
}
