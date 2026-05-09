// Package theme defines every color and style value in vpin-plunger.
// All UI code references named constants from this package only.
// Per-asset-type insert colors are added here during GSD milestone 1.
package theme

import "github.com/charmbracelet/lipgloss"

var (
	ColorBackground = lipgloss.Color("#0A0A0F")
	ColorInactive   = lipgloss.Color("#1A1A2E")
	ColorStatusBar  = lipgloss.Color("#12122A")
	ColorDMD        = lipgloss.Color("#FF8C00")
	ColorAccent     = lipgloss.Color("#00BFFF")
	ColorSuccess    = lipgloss.Color("#39FF14")
	ColorWarning    = lipgloss.Color("#FF006E")
	ColorMuted      = lipgloss.Color("#6B7FA3")
)
