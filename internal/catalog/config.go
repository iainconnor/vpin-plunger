// Package catalog downloads, parses, caches, and fuzzy-matches the VPX
// community catalog. The Config struct is standalone (does not import the
// app-level internal/config package) to prevent circular imports; CLI flag
// binding is performed in app/ during Phase 6 wiring.
package catalog

import "time"

// CatalogSheetID is the Google Sheets document ID for the VPX community
// catalog. The full export URL is constructed as:
//
//	https://docs.google.com/spreadsheets/d/{CatalogSheetID}/export?format=xlsx
//
// Override the sheet via Config.SheetURL (D-12).
const CatalogSheetID = "12-Pwub-p4krv17cwOFT4kr4qR_a34B_YbBvHjEcpSI0"

// DefaultStaleness is the maximum age of the cached catalog before
// re-download. Default 7 days per D-13.
const DefaultStaleness = 7 * 24 * time.Hour

// DefaultYearWindow is the +/-N tolerance applied to the same-era
// manufacturer filter (D-04). Default 3.
const DefaultYearWindow = 3

// DefaultTrailingArticle controls whether trailing-article convention is
// applied to canonical filenames and match-side normalization (D-05).
const DefaultTrailingArticle = true

// ThresholdAutoAssign: BestMatch auto-assigns at confidence >= this value.
// Empirically derived from the Python fuzzywuzzy implementation (D-02);
// may need minor tuning once Phase 4's 65 ported tests are validated.
const ThresholdAutoAssign = 92

// ThresholdInteractive: BestMatch returns nil when best confidence is below
// this value. Empirically derived from the Python fuzzywuzzy
// implementation (D-02).
const ThresholdInteractive = 72

// Config holds all parameters required to construct and operate a Catalog.
// Fields are populated from CLI flags by app/ during Phase 6; tests
// construct Config literals directly.
type Config struct {
	// CachePath is the on-disk path to the cached catalog xlsx
	// (default: {downloads_dir}/../cache/catalog.xlsx, D-11).
	CachePath string

	// SheetURL is the full xlsx export URL. Overrides any URL derived
	// from CatalogSheetID. Tests inject httptest.NewServer URLs here (D-18).
	SheetURL string

	// Staleness is the maximum age the on-disk cache may reach before
	// Load() re-downloads (D-13).
	Staleness time.Duration

	// YearWindow is the +/-N tolerance for the same-era manufacturer
	// filter in FindMatch Path A (D-04).
	YearWindow int

	// TrailingArticle toggles trailing-article convention for both
	// canonical filename construction and match-side normalization (D-05).
	TrailingArticle bool
}
