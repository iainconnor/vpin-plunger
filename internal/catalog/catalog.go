// Package catalog: the Catalog type owns the in-process entry slice (CAT-09).
// Construct one Catalog per run via New(); call Load() exactly once before
// any match methods. The entry slice is unexported — all access is via
// public methods (D-09). Load() does all I/O; app/ wraps it in a tea.Cmd
// closure (Phase 6).
package catalog

import "errors"

// ErrNotLoaded is returned by match methods when called before Load() has
// populated the entry slice.
var ErrNotLoaded = errors.New("catalog: Load() not called or did not succeed")

// errNotImplemented is a transient sentinel used by Wave 1 skeletons.
// Replaced by real implementations in Wave 2 plans 03-04 and 03-05.
var errNotImplemented = errors.New("catalog: not implemented (Wave 2)")

// Catalog holds the parsed catalog entries for the lifetime of one run.
// The entries slice is private — populated by Load(), read by match methods.
type Catalog struct {
	cfg     *Config
	entries []SheetEntry
}

// New constructs a Catalog with the given configuration. Load() must be
// called before any match methods are used (D-09).
func New(cfg *Config) *Catalog {
	return &Catalog{cfg: cfg}
}

// Entries returns a read-only view of the in-process catalog. Used by tests
// and by Phase 6 for monitor-mode iteration.
func (c *Catalog) Entries() []SheetEntry {
	return c.entries
}

// Load downloads the xlsx (if cache is missing or stale per Config.Staleness),
// parses all data tabs (those whose row 1 contains a "GameName" header), and
// populates the in-process entry slice. Idempotent: safe to call again, but
// in practice called once per run (CAT-01, CAT-02, CAT-09).
//
// Implementation lives in download.go + parse.go and is wired up by plan 03-04.
func (c *Catalog) Load() error {
	return errNotImplemented
}

// FindMatch returns up to limit candidate matches for stem, ordered by
// descending confidence. Path A: same-era filter (manufacturer match +
// |catalog_year - signal_year| <= cfg.YearWindow) -> WRatio scoring.
// Path B (fallback when Path A best < ThresholdInteractive): full-catalog
// WRatio against NormalizeForMatching-stripped names (CAT-05).
//
// Implementation lives in match.go and is wired up by plan 03-05.
func (c *Catalog) FindMatch(stem string, limit int) []MatchResult {
	return nil
}

// BestMatch returns the top FindMatch result when its confidence is at
// least ThresholdInteractive (72). Returns nil when no candidate clears
// the floor. Auto-assignable when the returned result has Confidence >=
// ThresholdAutoAssign (92) (CAT-06).
//
// Implementation lives in match.go and is wired up by plan 03-05.
func (c *Catalog) BestMatch(stem string) *MatchResult {
	return nil
}

// ForceMatch returns a synthetic MatchResult with Confidence=100 when id
// matches an entry's MasterID or IPDBNum (case-insensitive). Returns nil
// when no entry matches. MatchField is "master_id" or "ipdb_num" (CAT-07).
//
// Implementation lives in match.go and is wired up by plan 03-05.
func (c *Catalog) ForceMatch(id string) *MatchResult {
	return nil
}
