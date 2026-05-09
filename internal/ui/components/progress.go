package components

import (
	"github.com/charmbracelet/bubbles/progress"
	"github.com/charmbracelet/bubbles/spinner"
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
