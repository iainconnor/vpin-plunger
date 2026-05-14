// Package planner provides the BuildPlan function and associated types for
// planning virtual pinball asset moves. It is a pure-computation package:
// no filesystem writes, no TUI dependency (no lipgloss, no bubbletea imports).
// Standalone Config avoids circular imports with internal/config/.
package planner

// Config holds all destination paths and settings for BuildPlan.
// It is standalone — no import of internal/config/ — to avoid circular imports.
// The same pattern as catalog.Config (Phase 3 D-03).
// In production, Phase 6 constructs Config from CLI flags and PUPDatabase
// path discovery. In tests, inject a Config pointing at temp directories.
type Config struct {
	// 11 content destination directories (one per asset type)
	VPXDir      string // .vpx table files → Tables\
	BackglassDir string // .directb2s backglass files
	ROMDir      string // ROM zip verbatim copies
	NVRAMDir    string // .nv NVRAM save files
	POVDir      string // .ini POV override files
	DMDDir      string // UltraDMD bundle directories (*.UltraDMD)
	FlexDMDDir  string // FlexDMD bundle directories (*.FlexDMD)
	AudioDir    string // Altsound bundle directories
	AltcolorDir string // Altcolor bundle directories
	MusicDir    string // Music bundle directories
	PuPDir      string // PuP pack directories

	// 4 operational directories
	ArchiveVaultDir string // where original archives are moved after extraction
	ReviewDir       string // items below confidence threshold or dedup losers
	IgnoredDir      string // user-ignored items
	RehearsalDir    string // root for rehearsal path remapping (cfg.Rehearsal=true)

	// 3 settings
	TrailingArticle bool // apply trailing-article convention in canonical filenames
	YearWindow      int  // ±N years for same-era matching filter (default 3)
	Rehearsal       bool // if true, BuildPlan calls RemapForRehearsal before returning
}
