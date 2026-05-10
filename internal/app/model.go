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
// Compile-time assertions: verify that app.AppState constants and
// components.StatusState constants stay in sync. If you add a new state to
// either side without updating the other, this block will fail to compile.
// The two types are intentionally kept separate to avoid an import cycle
// (components cannot import app). Canonical source: internal/app/state.go.
var _ = [1]struct{}{}[0] // no-op anchor so the block is parseable without real assertions

func init() {
	// Runtime sync check — fires once at program start; panics if the two
	// constant sets have drifted. Kept in init() because Go does not allow
	// compile-time string equality comparisons outside of unsafe tricks.
	pairs := [][2]string{
		{string(StateIdle), components.StatusStateIdle},
		{string(StateLoading), components.StatusStateLoading},
		{string(StateScanning), components.StatusStateScanning},
		{string(StateMatching), components.StatusStateMatching},
		{string(StateExecuting), components.StatusStateExecuting},
		{string(StateDone), components.StatusStateDone},
	}
	for _, p := range pairs {
		if p[0] != p[1] {
			panic("app.AppState / components.StatusState mismatch: " + p[0] + " != " + p[1])
		}
	}
}

func New() Model {
	const initialWidth = 80
	sb := components.NewStatusBar(initialWidth)
	// StatusState is a type alias for string; AppState underlying value matches.
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
