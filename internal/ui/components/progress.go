package components

import (
	"fmt"

	tea "charm.land/bubbletea/v2"
	"charm.land/bubbles/v2/progress"
	"charm.land/bubbles/v2/spinner"
	"github.com/charmbracelet/lipgloss"

	"github.com/iainconnor/vpin-plunger/internal/ui/theme"
)

// DeterminateBar wraps bubbles/progress. Full implementation in milestone 1.
type DeterminateBar struct{ bar progress.Model }

// IndeterminateSpinner wraps bubbles/spinner. Full implementation in milestone 1.
type IndeterminateSpinner struct{ sp spinner.Model }

func NewIndeterminateSpinner() IndeterminateSpinner {
	s := spinner.New()
	s.Spinner = spinner.Dot
	return IndeterminateSpinner{sp: s}
}

// NewDeterminateBar creates a playfield lane-fill style progress bar.
// width is the rendered width in terminal cells.
func NewDeterminateBar(width int) DeterminateBar {
	p := progress.New(progress.WithDefaultBlend(), progress.WithWidth(width))
	return DeterminateBar{bar: p}
}

// View renders the bar at current/total proportion plus an amber count label.
// total == 0 is treated as 0% to avoid divide-by-zero.
func (b DeterminateBar) View(current, total int) string {
	pct := 0.0
	if total > 0 {
		pct = float64(current) / float64(total)
	}
	bar := b.bar.ViewAs(pct)
	label := lipgloss.NewStyle().
		Foreground(theme.ColorDMD).
		Render(fmt.Sprintf("%d / %d  %d%%", current, total, int(pct*100)))
	return lipgloss.JoinHorizontal(lipgloss.Top, bar, "  ", label)
}

// Init starts the spinner ticking. bubbletea v2 signature: returns tea.Cmd.
func (s IndeterminateSpinner) Init() tea.Cmd {
	return s.sp.Tick
}

// Update propagates messages to the underlying bubbles spinner. State changes
// only happen here.
func (s IndeterminateSpinner) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd
	s.sp, cmd = s.sp.Update(msg)
	return s, cmd
}

// View renders the current spinner frame. Pure — no I/O.
func (s IndeterminateSpinner) View() tea.View {
	return tea.NewView(s.sp.View())
}
