package app

import (
	"fmt"
	"os"
	"sort"
	"strings"
	"time"

	tea "charm.land/bubbletea/v2"

	"github.com/iainconnor/vpin-plunger/internal/catalog"
	"github.com/iainconnor/vpin-plunger/internal/db"
	"github.com/iainconnor/vpin-plunger/internal/executor"
	"github.com/iainconnor/vpin-plunger/internal/planner"
)

// loadCatalogCmd: runs catalog.Load() and returns CatalogLoadedMsg or CatalogErrorMsg.
func loadCatalogCmd(cat *catalog.Catalog) tea.Cmd {
	return func() tea.Msg {
		if err := cat.Load(); err != nil {
			return CatalogErrorMsg{Err: err}
		}
		return CatalogLoadedMsg{Catalog: cat}
	}
}

// scanCmd launches planner.BuildPlan in a goroutine. matchFn sends each
// request on reqCh and blocks on a buffered respCh (D-01, RESEARCH Pattern 1).
// In auto mode, pass a nil reqCh — the cmd uses planner.AutoSelectMatchFn.
func scanCmd(downloadsDir string, cat *catalog.Catalog, cfg *planner.Config,
	reqCh chan MatchRequest, auto bool) tea.Cmd {
	return func() tea.Msg {
		var matchFn planner.MatchFn
		if auto {
			matchFn = planner.AutoSelectMatchFn
		} else {
			matchFn = func(stem string, cands []catalog.MatchResult) planner.MatchChoice {
				respCh := make(chan planner.MatchChoice, 1)
				reqCh <- MatchRequest{Stem: stem, Candidates: cands, Response: respCh}
				return <-respCh
			}
		}
		plan, err := planner.BuildPlan(downloadsDir, cat, cfg, matchFn)
		if err != nil {
			return ScanErrorMsg{Err: err}
		}
		return ScanDoneMsg{Plan: plan}
	}
}

// waitMatchCmd blocks on reqCh and returns the next MatchRequest as a Msg.
func waitMatchCmd(reqCh chan MatchRequest) tea.Cmd {
	return func() tea.Msg {
		req, ok := <-reqCh
		if !ok {
			return nil
		}
		return MatchRequestMsg(req)
	}
}

// executePlanCmd opens the DB, calls executor.ExecutePlan, and returns ExecuteDoneMsg.
func executePlanCmd(plan *planner.ProcessPlan, cfg *executor.Config, dbPath string) tea.Cmd {
	return func() tea.Msg {
		database, err := db.Open(dbPath)
		if err != nil {
			return ExecuteDoneMsg{Err: fmt.Errorf("db open %s: %w", dbPath, err)}
		}
		defer database.Close()
		res, err := executor.ExecutePlan(plan, cfg, database)
		return ExecuteDoneMsg{Result: res, Err: err}
	}
}

// monitorPrintCmd prints the three monitor sections via tea.Println.
func monitorPrintCmd(report MonitorReportMsg) tea.Cmd {
	var cmds []tea.Cmd
	cmds = append(cmds, Print("=== Not Installed (%d entries) ===", len(report.NotInstalled)))
	for _, e := range report.NotInstalled {
		cmds = append(cmds, Print("  %s — %s (%d)", e.Name, e.Manufacturer, e.Year))
	}
	cmds = append(cmds, Print("=== Not in Catalog (%d entries) ===", len(report.NotInCatalog)))
	for _, g := range report.NotInCatalog {
		cmds = append(cmds, Print("  %s — %s", g.GameFileName, g.GameName))
	}
	cmds = append(cmds, Print("=== Name Mismatch (%d entries) ===", len(report.NameMismatch)))
	for _, g := range report.NameMismatch {
		cmds = append(cmds, Print("  %s: installed %q vs catalog %q", g.GameFileName, g.GameName, g.Canonical))
	}
	cmds = append(cmds, tea.Quit)
	return tea.Sequence(cmds...)
}

// downloadPrintCmd prints each group's URLs.
func downloadPrintCmd(msg DownloadGroupMsg) tea.Cmd {
	groups := msg.Groups
	sort.Slice(groups, func(i, j int) bool {
		if groups[i].Manufacturer != groups[j].Manufacturer {
			return groups[i].Manufacturer < groups[j].Manufacturer
		}
		return groups[i].Decade < groups[j].Decade
	})
	var cmds []tea.Cmd
	for _, g := range groups {
		cmds = append(cmds, Print("=== %s, %ds (%d URLs) ===", g.Manufacturer, g.Decade, len(g.URLs)))
		for _, u := range g.URLs {
			cmds = append(cmds, Print("  %s", u))
		}
	}
	cmds = append(cmds, tea.Quit)
	return tea.Sequence(cmds...)
}

// ---------------------------------------------------------------------------
// Phase 6 Plan 04 additions: exported entry points + freshness detection
// ---------------------------------------------------------------------------

// CatalogFreshness is the result of DetectCatalogFreshness.
// It drives which picker candidates are shown in StateCatalogFreshCheck,
// or whether the freshness prompt is skipped entirely (CatalogFresh).
type CatalogFreshness int

const (
	// CatalogMissing means the cache file does not exist. Prompt: "Download (recommended) / Exit".
	CatalogMissing CatalogFreshness = iota
	// CatalogStale means the cache exists but is older than the staleness threshold.
	// Prompt: "Use cached / Download fresh".
	CatalogStale
	// CatalogFresh means the cache is present and recent — skip the prompt.
	CatalogFresh
)

// DetectCatalogFreshness inspects the catalog cache state WITHOUT downloading.
// Used by cmd/plunger/process.go BEFORE tea.NewProgram so it can seed the
// initial picker with the correct candidates.
//
// Uses os.Stat directly to distinguish missing from stale — avoids the fragile
// two-pass heuristic that relied on an undocumented contract of catalog.IsStale
// returning (true, nil) for a missing file at any threshold.
//   - File absent (os.IsNotExist or other stat error) → CatalogMissing
//   - Exists but older than configured staleness threshold → CatalogStale
//   - Exists and within threshold → CatalogFresh
func DetectCatalogFreshness(cat *catalog.Catalog) CatalogFreshness {
	path := cat.CachePath()
	info, err := os.Stat(path)
	if os.IsNotExist(err) {
		return CatalogMissing
	}
	if err != nil {
		return CatalogMissing // treat unexpected stat errors as missing
	}
	if time.Since(info.ModTime()) > cat.Staleness() {
		return CatalogStale
	}
	return CatalogFresh
}

// StartProcess returns loadCatalogCmd for the process command flow.
// Called when the freshness check already authorized a load (Fresh path or
// after the user confirmed via the StateCatalogFreshCheck picker).
func StartProcess(cat *catalog.Catalog) tea.Cmd {
	return loadCatalogCmd(cat)
}

// StartProcessScan returns the scan-launch Cmd batch. In interactive mode,
// it batches scanCmd with the first waitMatchCmd. In auto mode, only scanCmd
// is returned (no channel needed; AutoSelectMatchFn is used).
func StartProcessScan(cat *catalog.Catalog, downloadsDir string, cfg *planner.Config,
	reqCh chan MatchRequest, auto bool) tea.Cmd {
	if auto {
		return scanCmd(downloadsDir, cat, cfg, nil, true)
	}
	return tea.Batch(
		scanCmd(downloadsDir, cat, cfg, reqCh, false),
		waitMatchCmd(reqCh),
	)
}

// StartMonitor returns the initial Cmd chain for `vpin monitor`.
func StartMonitor(cat *catalog.Catalog, dbPath string) tea.Cmd {
	return tea.Sequence(loadCatalogCmd(cat), monitorBuildCmd(cat, dbPath))
}

// StartDownload returns the initial Cmd chain for `vpin download`.
func StartDownload(cat *catalog.Catalog, dbPath string, dryRun bool) tea.Cmd {
	return tea.Sequence(loadCatalogCmd(cat), downloadBuildCmd(cat, dbPath, dryRun))
}

// monitorBuildCmd opens the DB, calls AllGames(), compares against catalog
// entries, and returns MonitorReportMsg. CanonicalFilename(true) is the
// correct call form per internal/catalog/types.go (one bool argument required).
func monitorBuildCmd(cat *catalog.Catalog, dbPath string) tea.Cmd {
	return func() tea.Msg {
		database, err := db.Open(dbPath)
		if err != nil {
			return CatalogErrorMsg{Err: err}
		}
		defer database.Close()
		rows, err := database.AllGames()
		if err != nil {
			return CatalogErrorMsg{Err: err}
		}

		catEntries := cat.Entries()
		installedFilenames := make(map[string]bool, len(rows))
		for _, r := range rows {
			installedFilenames[strings.ToLower(r.GameFileName)] = true
		}

		var notInstalled []catalog.SheetEntry
		for _, e := range catEntries {
			canonStem := strings.ToLower(e.CanonicalFilename(true)) + ".vpx"
			if !installedFilenames[canonStem] {
				notInstalled = append(notInstalled, e)
			}
		}

		// Build a reverse map from catalog canonical filename (lower-cased, no
		// extension) to *SheetEntry so the per-row lookup below is O(1) rather
		// than O(m) for each of the n installed games.
		catByFilename := make(map[string]*catalog.SheetEntry, len(catEntries))
		for i := range catEntries {
			key := strings.ToLower(catEntries[i].CanonicalFilename(true))
			catByFilename[key] = &catEntries[i]
		}

		var notInCatalog, nameMismatch []DBGameRef
		for _, r := range rows {
			key := strings.TrimSuffix(strings.ToLower(r.GameFileName), ".vpx")
			match := catByFilename[key]
			if match == nil {
				notInCatalog = append(notInCatalog, DBGameRef{
					GameFileName: r.GameFileName,
					GameName:     r.GameName,
				})
				continue
			}
			if !strings.EqualFold(r.GameName, match.Name) {
				nameMismatch = append(nameMismatch, DBGameRef{
					GameFileName: r.GameFileName,
					GameName:     r.GameName,
					Canonical:    match.Name,
				})
			}
		}
		return MonitorReportMsg{
			NotInstalled: notInstalled,
			NotInCatalog: notInCatalog,
			NameMismatch: nameMismatch,
		}
	}
}

// downloadBuildCmd opens the DB, finds uninstalled catalog entries, groups by
// manufacturer + decade, and returns DownloadGroupMsg with VPW+VPS URLs.
// Delegates to buildDownloadGroups so the same logic is reusable outside the
// tea.Cmd pipeline (see DownloadBuildOnce).
func downloadBuildCmd(cat *catalog.Catalog, dbPath string, dryRun bool) tea.Cmd {
	return func() tea.Msg {
		groups, err := buildDownloadGroups(cat, dbPath)
		if err != nil {
			return CatalogErrorMsg{Err: err}
		}
		_ = dryRun
		return DownloadGroupMsg{Groups: groups}
	}
}

// buildDownloadGroups is the synchronous data-only core of downloadBuildCmd.
// It opens the DB, calls AllGames(), groups uninstalled catalog entries by
// manufacturer + decade, and returns the slice. Called by both downloadBuildCmd
// (inside a tea.Cmd) and DownloadBuildOnce (outside the tea.Cmd pipeline).
func buildDownloadGroups(cat *catalog.Catalog, dbPath string) ([]DownloadGroup, error) {
	database, err := db.Open(dbPath)
	if err != nil {
		return nil, err
	}
	defer database.Close()
	rows, err := database.AllGames()
	if err != nil {
		return nil, err
	}

	installed := make(map[string]bool, len(rows))
	for _, r := range rows {
		installed[strings.ToLower(strings.TrimSuffix(r.GameFileName, ".vpx"))] = true
	}

	type key struct {
		Manufacturer string
		Decade       int
	}
	groups := make(map[key][]string)
	for _, e := range cat.Entries() {
		if installed[strings.ToLower(e.CanonicalFilename(true))] {
			continue
		}
		if e.Year == 0 {
			continue
		}
		k := key{Manufacturer: e.Manufacturer, Decade: (e.Year / 10) * 10}
		for _, u := range []string{e.VPWLink, e.VPSLink} {
			if u != "" {
				groups[k] = append(groups[k], u)
			}
		}
	}
	var out []DownloadGroup
	for k, urls := range groups {
		out = append(out, DownloadGroup{
			Manufacturer: k.Manufacturer,
			Decade:       k.Decade,
			URLs:         dedup(urls),
		})
	}
	return out, nil
}

// DownloadBuildOnce is the synchronous data-only counterpart of downloadBuildCmd.
// It opens the DB, calls AllGames(), groups uninstalled catalog entries by
// manufacturer + decade, and returns the slice. Used by cmd/plunger/download.go
// after the TUI exits to perform openURL calls outside the tea.Cmd pipeline.
func DownloadBuildOnce(cat *catalog.Catalog, dbPath string) ([]DownloadGroup, error) {
	return buildDownloadGroups(cat, dbPath)
}

// dedup removes duplicate strings from in, preserving first-occurrence order.
func dedup(in []string) []string {
	seen := map[string]bool{}
	out := make([]string, 0, len(in))
	for _, s := range in {
		if seen[s] {
			continue
		}
		seen[s] = true
		out = append(out, s)
	}
	return out
}
