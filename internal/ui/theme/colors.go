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

// Per-asset-type insert colors. One per type; no two share a hex value.
// Short codes used in pill badges: VPX BkG ROM NVR INI DMD Aud Clr Snd Msc PUP Arc.
var (
	ColorInsertVPX       = lipgloss.Color("#FF3C00")
	ColorInsertBackglass = lipgloss.Color("#FF00FF")
	ColorInsertROM       = lipgloss.Color("#FFE600")
	ColorInsertNVRAM     = lipgloss.Color("#7FFF00")
	ColorInsertPOV       = lipgloss.Color("#00FFD4")
	ColorInsertDMD       = lipgloss.Color("#FF6EC7")
	ColorInsertAudio     = lipgloss.Color("#BF5FFF")
	ColorInsertAltcolor  = lipgloss.Color("#00FF41")
	ColorInsertAltsound  = lipgloss.Color("#FF9500")
	ColorInsertMusic     = lipgloss.Color("#00C8FF")
	ColorInsertPUP       = lipgloss.Color("#FF4081")
	ColorInsertArchive   = lipgloss.Color("#A0A0A0")
)
