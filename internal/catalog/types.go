package catalog

import (
	"github.com/iainconnor/vpin-plunger/internal/naming"
)

// SheetEntry holds one row from a catalog data tab. Field names match the
// sheet column headers (MIGRATION-BRIEF section 9.5). Empty cells are
// stored as empty strings; Year is 0 when the cell was missing or
// unparseable.
type SheetEntry struct {
	GameName     string // raw "GameName" column value (full name)
	Name         string // extracted title only (from naming.ExtractSignal or full GameName)
	Manufacturer string // "Manufact" column
	Year         int    // "GameYear" column, parsed via float fallback
	GameType     string // "GameType" column
	MasterID     string // "MasterID" column
	IPDBNum      string // "IPDBNum" column (string; some rows blank)
	DesignedBy   string // "DesignedBy" column
	Decade       string // "Decade" column
	Tier         string // "Tier" column
	Notes        string // "Notes" column
	VPWLink      string // "VPW Version Link" column
	VPSLink      string // "VPS Link" column
	IPDBUrl      string // "WebLinkURL" column
}

// CanonicalFilename returns the canonical stem
//
//	{Name} ({Manufacturer}, {Year})
//
// applying trailing-article convention when trailingArticle is true (CAT-03).
// Delegates to naming.Canonical (D-08: naming/ owns canonical construction).
func (e SheetEntry) CanonicalFilename(trailingArticle bool) string {
	return naming.Canonical(e.Name, e.Manufacturer, e.Year, trailingArticle)
}

// MatchResult is the unit returned by FindMatch, BestMatch, and ForceMatch.
// Confidence is 0..100; ForceMatch always returns 100.
// MatchField identifies which catalog field produced the match:
//
//	"game_name" — fuzzy match on the normalized game name
//	"master_id" — exact match on MasterID via ForceMatch
//	"ipdb_num"  — exact match on IPDBNum via ForceMatch
type MatchResult struct {
	Entry      SheetEntry
	Confidence int
	MatchField string
}
