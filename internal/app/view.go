package app

import (
	"strings"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/lipgloss"
)

// View is pure: no I/O, no state changes, no goroutines.
//
// Layout (top to bottom):
//   - Picker content (candidates, search input, action row)
//   - Vertical filler so the StatusBar pins to the bottom regardless of how
//     little content the Picker emits
//   - StatusBar (always 1 line tall, full terminal width)
//
// TUI-01: the StatusBar is ALWAYS the last segment, so it is pinned at the
// bottom. lipgloss.JoinVertical performs the composition.
func (m Model) View() tea.View {
	picker := m.Picker.Render()
	bar := m.StatusBar.Render()

	var composed string
	if m.Height <= 0 {
		composed = lipgloss.JoinVertical(lipgloss.Left, picker, bar)
	} else {
		pickerLines := strings.Count(strings.TrimRight(picker, "\n"), "\n") + 1
		// Reserve 1 line for the bar; remaining vertical space is filler.
		filler := m.Height - pickerLines - 1
		if filler < 0 {
			filler = 0
		}
		composed = lipgloss.JoinVertical(
			lipgloss.Left,
			picker,
			strings.Repeat("\n", filler),
			bar,
		)
	}

	v := tea.NewView(composed)
	v.AltScreen = true
	return v
}
