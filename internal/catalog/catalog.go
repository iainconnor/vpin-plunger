// Package catalog: the Catalog type owns the in-process entry slice (CAT-09).
// Construct one Catalog per run via New(); call Load() exactly once before
// any match methods. The entry slice is unexported — all access is via
// public methods (D-09). Load() does all I/O; app/ wraps it in a tea.Cmd
// closure (Phase 6).
package catalog

import (
	"errors"
	"fmt"
	"os"
	"time"
)

// ErrNotLoaded is returned by match methods when called before Load() has
// populated the entry slice.
var ErrNotLoaded = errors.New("catalog: Load() not called or did not succeed")

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

// CachePath returns the on-disk path to the cached catalog xlsx.
// Used by app/ to detect freshness before tea.NewProgram starts (Phase 6).
func (c *Catalog) CachePath() string { return c.cfg.CachePath }

// Staleness returns the configured staleness threshold.
// Used by app/ alongside catalog.IsStale to detect freshness (Phase 6).
func (c *Catalog) Staleness() time.Duration { return c.cfg.Staleness }

// Load downloads the xlsx (if cache is missing or stale per Config.Staleness),
// parses all data tabs (those whose row 1 contains a "GameName" header), and
// populates the in-process entry slice. Idempotent: safe to call again, but
// in practice called once per run (CAT-01, CAT-02, CAT-09).
func (c *Catalog) Load() error {
	stale, err := IsStale(c.cfg.CachePath, c.cfg.Staleness)
	if err != nil {
		return fmt.Errorf("catalog stale check: %w", err)
	}
	if stale {
		if err := Download(c.cfg.SheetURL, c.cfg.CachePath); err != nil {
			return fmt.Errorf("catalog download: %w", err)
		}
	}
	return c.loadFromDisk()
}

// LoadCached reads the xlsx from the on-disk cache WITHOUT performing a
// freshness check or network download. Used when the user explicitly chooses
// "Use cached catalog" in the StateCatalogFreshCheck picker — the catalog may
// be stale but the user has opted to skip the refresh.
//
// Returns an error if the cache file does not exist or cannot be parsed.
func (c *Catalog) LoadCached() error {
	return c.loadFromDisk()
}

// loadFromDisk opens and parses the cached xlsx file. Called by both Load()
// (after an optional download) and LoadCached() (skipping the download).
func (c *Catalog) loadFromDisk() error {
	f, err := os.Open(c.cfg.CachePath)
	if err != nil {
		return fmt.Errorf("catalog open %s: %w", c.cfg.CachePath, err)
	}
	defer f.Close()

	entries, err := ParseXLSX(f, c.cfg.TrailingArticle)
	if err != nil {
		return fmt.Errorf("catalog parse: %w", err)
	}
	c.entries = entries
	return nil
}
