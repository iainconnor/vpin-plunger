package components

import (
	"fmt"
	"strings"

	tea "charm.land/bubbletea/v2"
	"charm.land/bubbles/v2/textinput"
	"github.com/charmbracelet/lipgloss"

	"github.com/iainconnor/vpin-plunger/internal/ui/theme"
)

// Candidate is one match candidate displayed in the Picker. Fields map to
// CONTEXT.md row format: name in ColorDMD, parenthetical in ColorMuted,
// confidence in ColorAccent.
type Candidate struct {
	Name          string
	Parenthetical string
	Confidence    int
}

// Picker displays match candidates with arrow-key navigation and a search input.
// Phase 1: 3 hardcoded fixture candidates so the layout is reviewable.
//
// Key bindings (per CONTEXT.md):
//
//	1-9        instant select by number
//	up/k       move cursor up
//	down/j     move cursor down
//	enter      confirm highlighted candidate
//	s          focus the search input (blur via esc)
//	r          send to review
//	i          ignore
//	q          quit
//
// Action row format: [S]earch  [R]eview  [I]gnore  [Q]uit
type Picker struct {
	Candidates []Candidate
	Cursor     int
	Input      textinput.Model
	InputFocus bool
	Width      int
	Height     int
}

// NewPicker returns a Picker pre-populated with 3 fixture candidates so the
// layout can be reviewed before real catalog data exists.
func NewPicker() Picker {
	ti := textinput.New()
	ti.Placeholder = "search by name, MasterID, or IPDB#"
	ti.CharLimit = 64

	return Picker{
		Candidates: []Candidate{
			{Name: "Addams Family, The", Parenthetical: "Williams, 1992", Confidence: 92},
			{Name: "Addams Family", Parenthetical: "Williams, 1992", Confidence: 85},
			{Name: "Addams Family Pinball", Parenthetical: "Bally, 1991", Confidence: 71},
		},
		Cursor:     0,
		Input:      ti,
		InputFocus: false,
	}
}

// SetCandidates replaces the candidate slice and clamps the cursor so it
// always points at a valid row. Call this whenever catalog results arrive.
// Without the clamp, a stale cursor past the new slice end causes the
// highlighted row to silently disappear (Render iterates with range — no
// panic, but no highlight either).
func (p *Picker) SetCandidates(candidates []Candidate) {
	p.Candidates = candidates
	if len(p.Candidates) == 0 {
		p.Cursor = 0
		return
	}
	if p.Cursor >= len(p.Candidates) {
		p.Cursor = len(p.Candidates) - 1
	}
}

// Init starts the input cursor blink. The blink is a tea.Cmd — bubbletea
// schedules it; no goroutines.
func (p Picker) Init() tea.Cmd {
	return textinput.Blink
}

// Update handles keyboard input. State changes happen ONLY here.
//
// Focus rules:
//   - When InputFocus is true: all key messages are forwarded to the text
//     input, EXCEPT esc (which blurs the input and returns control to the
//     candidate list). Letters typed into the input are not interpreted as
//     action shortcuts — preventing the "typing corrupts the bar" failure
//     mode that TUI-03 explicitly forbids.
//   - When InputFocus is false: key messages drive cursor movement, instant
//     select (1-9), action shortcuts (s/r/i/q), and enter to confirm.
func (p Picker) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch m := msg.(type) {
	case tea.WindowSizeMsg:
		p.Width = m.Width
		p.Height = m.Height
	case tea.KeyMsg:
		key := m.String()

		if p.InputFocus {
			if key == "esc" {
				p.InputFocus = false
				p.Input.Blur()
				return p, nil
			}
			var cmd tea.Cmd
			p.Input, cmd = p.Input.Update(msg)
			return p, cmd
		}

		switch key {
		case "up", "k":
			if p.Cursor > 0 {
				p.Cursor--
			}
		case "down", "j":
			if p.Cursor < len(p.Candidates)-1 {
				p.Cursor++
			}
		case "1", "2", "3", "4", "5", "6", "7", "8", "9":
			idx := int(key[0] - '1')
			if idx >= 0 && idx < len(p.Candidates) {
				p.Cursor = idx
			}
		case "enter":
			// confirmed — wired in Phase 4
			return p, nil
		case "s":
			p.InputFocus = true
			cmd := p.Input.Focus()
			return p, cmd
		case "r":
			// review — wired in Phase 4; placeholder output keeps the tea.Cmd
			// contract explicit so future callers adding a real cmd cannot
			// accidentally drop it by falling through to the final return.
			return p, tea.Println("[picker] review")
		case "i":
			// ignore — wired in Phase 4; same rationale as "r" above.
			return p, tea.Println("[picker] ignore")
		case "q":
			return p, tea.Quit
		}
	}
	return p, nil
}

// Render returns the picker content as a plain string for embedding in a
// parent View.
func (p Picker) Render() string {
	var b strings.Builder

	nameStyle := lipgloss.NewStyle().Foreground(theme.ColorDMD)
	parenStyle := lipgloss.NewStyle().Foreground(theme.ColorMuted)
	confStyle := lipgloss.NewStyle().Foreground(theme.ColorAccent)
	highlight := lipgloss.NewStyle().
		Foreground(theme.ColorDMD).
		Background(theme.ColorInactive).
		Bold(true)

	for i, c := range p.Candidates {
		marker := "  "
		row := fmt.Sprintf("%d  %s   %s   %s",
			i+1,
			nameStyle.Render(c.Name),
			parenStyle.Render("("+c.Parenthetical+")"),
			confStyle.Render(fmt.Sprintf("%d%%", c.Confidence)),
		)
		if i == p.Cursor && !p.InputFocus {
			b.WriteString(highlight.Render(marker + row))
		} else {
			b.WriteString(marker + row)
		}
		b.WriteString("\n")
	}

	b.WriteString("\n")
	b.WriteString(p.Input.View())
	b.WriteString("\n\n")

	accent := lipgloss.NewStyle().Foreground(theme.ColorAccent)
	actions := []string{
		accent.Render("[S]earch"),
		accent.Render("[R]eview"),
		accent.Render("[I]gnore"),
		accent.Render("[Q]uit"),
	}
	b.WriteString(strings.Join(actions, "  "))
	b.WriteString("\n")

	return b.String()
}

// View renders candidate rows, the search input, and the action row.
// Pure — no I/O or state changes.
func (p Picker) View() tea.View {
	return tea.NewView(p.Render())
}
