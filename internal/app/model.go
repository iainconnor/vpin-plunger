package app

import (
	tea "charm.land/bubbletea/v2"

	"github.com/iainconnor/vpin-plunger/internal/ui/components"
)

// Model is the root bubbletea Model. It owns all application state.
//
// Sub-components (StatusBar, Picker) are embedded as fields, NOT pointers —
// bubbletea v2 returns updated models by value through Update.
type Model struct {
	State     AppState
	Width     int
	Height    int
	StatusBar components.StatusBar
	Picker    components.Picker
}

// New constructs the initial Model with fixture-populated sub-components so
// the visual chrome is reviewable before real data arrives in later phases.
//
// Initial Width is 80 (a reasonable default; the first WindowSizeMsg replaces
// it before the first render is committed in alt-screen mode).
func New() Model {
	const initialWidth = 80
	sb := components.NewStatusBar(initialWidth)
	sb.State = string(StateIdle)
	return Model{
		State:     StateIdle,
		Width:     initialWidth,
		Height:    24,
		StatusBar: sb,
		Picker:    components.NewPicker(),
	}
}

// Init satisfies tea.Model. It composes the Init commands of all sub-components
// so spinners/blinks start ticking immediately.
func (m Model) Init() tea.Cmd {
	sbCmd := m.StatusBar.Init()
	pCmd := m.Picker.Init()
	return tea.Batch(sbCmd, pCmd)
}
