// Package components provides reusable TUI components.
// All components wrap github.com/charmbracelet/bubbles primitives.
// Do not implement custom TUI widgets from scratch.
package components

import (
	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/lipgloss"

	"github.com/iainconnor/vpin-plunger/internal/ui/theme"
)

// StatusBar is the pinned bottom bar. Full implementation in milestone 1.
type StatusBar struct {
	State   string
	Message string
}

func (s StatusBar) Init() tea.Cmd { return nil }
func (s StatusBar) Update(msg tea.Msg) (tea.Model, tea.Cmd) { return s, nil }
func (s StatusBar) View() tea.View {
	return tea.NewView(lipgloss.NewStyle().
		Background(theme.ColorStatusBar).
		Foreground(theme.ColorDMD).
		Render(s.State))
}
