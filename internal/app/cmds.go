package app

import (
	"fmt"
	"sort"

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
