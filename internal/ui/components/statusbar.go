// Package components provides reusable TUI components.
// All components wrap github.com/charmbracelet/bubbles primitives.
// Do not implement custom TUI widgets from scratch.
package components

import (
	"fmt"
	"strings"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/lipgloss"

	"github.com/iainconnor/vpin-plunger/internal/ui/theme"
)

// StatusBar is the pinned bottom bar. Always visible. 4 zones left-to-right.
// Zone 1: spinner + task description (transient; hidden when State == "IDLE" or "DONE")
// Zone 2: progress bar + count + percentage (transient; shown during SCANNING / EXECUTING)
// Zone 3: type-summary pill badges using per-asset insert colors (transient; shown during SCANNING / EXECUTING)
// Zone 4: system state label (permanent — always amber, always visible)
type StatusBar struct {
	Width      int
	State      string         // one of "IDLE", "LOADING", "SCANNING", "MATCHING", "EXECUTING", "DONE"
	Task       string         // zone 1 description (e.g. "Extracting archive")
	Progress   float64        // zone 2: 0.0–1.0
	Current    int            // zone 2: numerator
	Total      int            // zone 2: denominator
	TypeCounts map[string]int // zone 3: short code -> count, e.g. {"VPX": 3, "ROM": 2, "BkG": 1}
	Spinner    IndeterminateSpinner
	Bar        DeterminateBar
}

// NewStatusBar returns a StatusBar with fixture defaults so the layout is
// reviewable in Phase 1 before real data arrives.
func NewStatusBar(width int) StatusBar {
	return StatusBar{
		Width:    width,
		State:    "IDLE",
		Task:     "Loading...",
		Progress: 0,
		Current:  10,
		Total:    30,
		TypeCounts: map[string]int{
			"VPX": 3,
			"ROM": 2,
			"BkG": 1,
		},
		Spinner: NewIndeterminateSpinner(),
		Bar:     NewDeterminateBar(barWidthFor(width)),
	}
}

var barStyle = lipgloss.NewStyle().
	Background(theme.ColorStatusBar).
	Foreground(theme.ColorDMD)

// pillColor maps the short code (used as pill label) to the type's insert color.
// Order of declaration mirrors the 12 ColorInsert* constants.
var pillColor = map[string]lipgloss.Color{
	"VPX": theme.ColorInsertVPX,
	"BkG": theme.ColorInsertBackglass,
	"ROM": theme.ColorInsertROM,
	"NVR": theme.ColorInsertNVRAM,
	"INI": theme.ColorInsertPOV,
	"DMD": theme.ColorInsertDMD,
	"Aud": theme.ColorInsertAudio,
	"Clr": theme.ColorInsertAltcolor,
	"Snd": theme.ColorInsertAltsound,
	"Msc": theme.ColorInsertMusic,
	"PUP": theme.ColorInsertPUP,
	"Arc": theme.ColorInsertArchive,
}

// renderPill renders one pill badge: short code, the multiplier glyph, and count,
// foreground in the type's insert color, on the status-bar background.
func renderPill(shortCode string, count int) string {
	col, ok := pillColor[shortCode]
	if !ok {
		col = theme.ColorMuted
	}
	return lipgloss.NewStyle().
		Background(theme.ColorStatusBar).
		Foreground(col).
		Render(fmt.Sprintf("%s×%d", shortCode, count))
}

// barWidthFor returns the progress-bar width for a given terminal width.
// Uses width/4 (integer division) with a minimum of 1 to avoid passing zero
// to progress.WithWidth — the bubbles progress library may panic or clamp
// unexpectedly on a zero-width bar.
func barWidthFor(termWidth int) int {
	w := termWidth / 4
	if w < 1 {
		w = 1
	}
	return w
}

// Init starts the spinner ticking via tea.Cmd.
func (s StatusBar) Init() tea.Cmd {
	return s.Spinner.Init()
}

// Update forwards messages to the embedded spinner so animation frames advance,
// and handles WindowSizeMsg so the determinate bar is rebuilt at the new width.
// State transitions (Task, Progress, etc.) are written by the parent Model.
func (s StatusBar) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	if ws, ok := msg.(tea.WindowSizeMsg); ok {
		s.Width = ws.Width
		s.Bar = NewDeterminateBar(barWidthFor(ws.Width))
	}
	updatedSpinner, cmd := s.Spinner.Update(msg)
	if sp, ok := updatedSpinner.(IndeterminateSpinner); ok {
		s.Spinner = sp
	}
	return s, cmd
}

// Render returns the composed 4-zone bar as a plain string for embedding in a
// parent View. The parent's tea.View wraps this along with other components.
func (s StatusBar) Render() string {
	var zones []string

	// Zone 1
	if s.State != "IDLE" && s.State != "DONE" {
		zones = append(zones, s.Spinner.View().Content+" "+barStyle.Render(s.Task))
	}

	// Zone 2
	if s.State == "SCANNING" || s.State == "EXECUTING" {
		zones = append(zones, s.Bar.View(s.Current, s.Total))
	}

	// Zone 3
	if s.State == "SCANNING" || s.State == "EXECUTING" {
		// Stable pill ordering by declaration: VPX BkG ROM NVR INI DMD Aud Clr Snd Msc PUP Arc
		order := []string{"VPX", "BkG", "ROM", "NVR", "INI", "DMD", "Aud", "Clr", "Snd", "Msc", "PUP", "Arc"}
		var pills []string
		for _, code := range order {
			if n, ok := s.TypeCounts[code]; ok && n > 0 {
				pills = append(pills, renderPill(code, n))
			}
		}
		if len(pills) > 0 {
			zones = append(zones, strings.Join(pills, "  "))
		}
	}

	// Zone 4 — always visible
	zones = append(zones, barStyle.Bold(true).Render(s.State))

	return barStyle.Width(s.Width).Render(strings.Join(zones, "  │  "))
}

// View renders 4 zones left-to-right and joins them on the bar background.
// Zone visibility rules:
//
//	Zone 1 (spinner + Task): hidden when State == "IDLE" || State == "DONE"
//	Zone 2 (progress + count): shown when State == "SCANNING" || State == "EXECUTING"
//	Zone 3 (type pills):       shown when State == "SCANNING" || State == "EXECUTING"
//	Zone 4 (system state):     always visible
func (s StatusBar) View() tea.View {
	return tea.NewView(s.Render())
}
