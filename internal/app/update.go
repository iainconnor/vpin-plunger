package app

import (
	"fmt"
	"os"

	tea "charm.land/bubbletea/v2"

	"github.com/iainconnor/vpin-plunger/internal/planner"
	"github.com/iainconnor/vpin-plunger/internal/ui/components"
)

// Update is the ONLY place application state mutates. Per CLAUDE.md, no raw
// goroutines are spawned here — long-running work returns a tea.Cmd.
//
// Handling order:
//  1. Domain messages (CatalogLoadedMsg, ScanDoneMsg, MatchRequestMsg, etc.) —
//     processed before sub-component fan-out so state transitions happen first.
//  2. tea.WindowSizeMsg — record dimensions, propagate to sub-components so
//     they can rebuild width-dependent layout (TUI-06: graceful resize).
//  3. tea.KeyMsg — global hotkeys (ctrl+c) before delegating to sub-components.
//  4. All other msgs — fan out to sub-components and batch the returned cmds.
func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmds []tea.Cmd

	switch m2 := msg.(type) {
	case tea.WindowSizeMsg:
		m.Width = m2.Width
		m.Height = m2.Height
		// Width and Bar resize are handled inside StatusBar.Update (WindowSizeMsg
		// case) — no direct field write needed here. Picker likewise owns its own
		// resize. Both components receive the message via the fan-out below.

	case CatalogLoadedMsg:
		if m2.Catalog != nil {
			m.Catalog = m2.Catalog
		}
		m.State = StateScanning
		m.StatusBar.State = string(StateScanning)
		if m.ScanCfg != nil && m.DownloadsDir != "" {
			m.matchReqCh = make(chan MatchRequest, 1)
			return m, StartProcessScan(m.Catalog, m.DownloadsDir, m.ScanCfg, m.matchReqCh, m.AutoMode)
		}
		return m, nil

	case CatalogErrorMsg:
		m.State = StateDone
		m.StatusBar.State = string(StateDone)
		return m, tea.Sequence(Print("ERROR: catalog load failed: %v", m2.Err), tea.Quit)

	case ScanDoneMsg:
		m.Plan = m2.Plan
		if m.AutoMode {
			m.State = StateExecuting
			m.StatusBar.State = string(StateExecuting)
			return m, tea.Sequence(
				Print("%s", planner.FormatPlan(m2.Plan)),
				executePlanCmd(m2.Plan, m.ExecCfg, m.DBPath),
			)
		}
		m.State = StateConfirming
		m.StatusBar.State = string(StateConfirming)
		m.Confirming = true
		m.Picker.SetCandidates([]components.Candidate{
			{Name: "Execute plan", Parenthetical: "proceed", Confidence: 0},
			{Name: "Cancel", Parenthetical: "abort", Confidence: 0},
		})
		m.Picker.Cursor = 0
		return m, Print("%s", planner.FormatPlan(m2.Plan))

	case ScanErrorMsg:
		m.State = StateDone
		m.StatusBar.State = string(StateDone)
		return m, tea.Sequence(Print("ERROR: scan failed: %v", m2.Err), tea.Quit)

	case MatchRequestMsg:
		m.PendingMatch = (*MatchRequest)(&m2)
		m.State = StateMatching
		m.StatusBar.State = string(StateMatching)
		cands := make([]components.Candidate, 0, len(m2.Candidates))
		for _, mr := range m2.Candidates {
			cands = append(cands, components.Candidate{
				Name:          mr.Entry.Name,
				Parenthetical: fmt.Sprintf("%s, %d", mr.Entry.Manufacturer, mr.Entry.Year),
				Confidence:    mr.Confidence,
			})
		}
		m.Picker.SetCandidates(cands)
		m.Picker.Cursor = 0
		return m, waitMatchCmd(m.matchReqCh)

	case ExecuteProgressMsg:
		m.ProgressCounts = m2
		m.StatusBar.State = fmt.Sprintf("EXECUTING %d✓ %d✗ %d?", m2.Moved, m2.Failed, m2.Reviewed)
		return m, nil

	case ExecuteDoneMsg:
		m.State = StateDone
		m.StatusBar.State = string(StateDone)
		if m2.Err != nil {
			return m, tea.Sequence(Print("ERROR: execute failed: %v", m2.Err), tea.Quit)
		}
		r := m2.Result
		return m, tea.Sequence(
			Print("DONE: moved=%d failed=%d reviewed=%d ignored=%d", r.Moved, r.Failed, r.Reviewed, r.Ignored),
			tea.Quit,
		)

	case MonitorReportMsg:
		return m, monitorPrintCmd(m2)

	case DownloadGroupMsg:
		return m, downloadPrintCmd(m2)

	case tea.KeyMsg:
		// Global quit on ctrl+c regardless of focus state.
		if m2.String() == "ctrl+c" {
			return m, tea.Quit
		}

		if m2.String() == "enter" {
			// 1. Match prompt active (D-02, Pitfall 5)
			if m.PendingMatch != nil {
				cands := m.PendingMatch.Candidates
				idx := m.Picker.Cursor
				var choice planner.MatchChoice
				if m.Picker.InputFocus && m.Picker.Input.Value() != "" {
					choice = planner.MatchChoice{ForceID: m.Picker.Input.Value()}
				} else if idx >= 0 && idx < len(cands) {
					sel := cands[idx]
					choice = planner.MatchChoice{Match: &sel}
				} else {
					choice = planner.MatchChoice{SendToReview: true}
				}
				m.PendingMatch.Response <- choice
				m.PendingMatch = nil
				m.Picker.SetCandidates(nil)
				m.Picker.Input.SetValue("")
				m.State = StateScanning
				m.StatusBar.State = string(StateScanning)
				return m, nil
			}
			// 2. Plan-confirm picker (D-07)
			if m.Confirming {
				m.Confirming = false
				if m.Picker.Cursor == 0 && m.Plan != nil {
					m.State = StateExecuting
					m.StatusBar.State = string(StateExecuting)
					m.Picker.SetCandidates(nil)
					return m, executePlanCmd(m.Plan, m.ExecCfg, m.DBPath)
				}
				m.State = StateDone
				m.StatusBar.State = string(StateDone)
				m.Picker.SetCandidates(nil)
				return m, tea.Sequence(Print("CANCELLED"), tea.Quit)
			}
			// 3. Rehearsal wipe picker (D-16, MOD-05)
			if m.State == StateRehearsalWipeCheck {
				m.Picker.SetCandidates(nil)
				if m.Picker.Cursor == 0 && m.RehearsalDir != "" {
					// Yes -> wipe and proceed to next prompt (catalog freshness)
					if err := os.RemoveAll(m.RehearsalDir); err != nil {
						m.State = StateDone
						m.StatusBar.State = string(StateDone)
						return m, tea.Sequence(Print("ERROR: wipe rehearsal/: %v", err), tea.Quit)
					}
					return advanceToCatalogFreshCheck(m)
				}
				// Cancel -> exit
				m.State = StateDone
				m.StatusBar.State = string(StateDone)
				return m, tea.Sequence(Print("CANCELLED"), tea.Quit)
			}
			// 4. Catalog freshness picker (MOD-02)
			if m.State == StateCatalogFreshCheck {
				yes := m.Picker.Cursor == 0
				m.Picker.SetCandidates(nil)
				if !yes && m.Catalog == nil {
					// Picker showed ["Download (recommended)", "Exit"] for the
					// missing-cache case; "Exit" path quits cleanly.
					m.State = StateDone
					m.StatusBar.State = string(StateDone)
					return m, tea.Sequence(Print("CANCELLED"), tea.Quit)
				}
				m.State = StateLoading
				m.StatusBar.State = string(StateLoading)
				if yes {
					// Yes -> download/refresh
					return m, loadCatalogCmd(m.Catalog)
				}
				// No (stale cache) -> use existing cache without refresh.
				// catalog.Load() with a non-stale cache short-circuits; calling
				// it is the simplest way to obtain CatalogLoadedMsg.
				return m, loadCatalogCmd(m.Catalog)
			}
		}
		if m2.String() == "r" && m.PendingMatch != nil {
			m.PendingMatch.Response <- planner.MatchChoice{SendToReview: true}
			m.PendingMatch = nil
			m.Picker.SetCandidates(nil)
			m.State = StateScanning
			return m, nil
		}
		if m2.String() == "i" && m.PendingMatch != nil {
			m.PendingMatch.Response <- planner.MatchChoice{Ignore: true}
			m.PendingMatch = nil
			m.Picker.SetCandidates(nil)
			m.State = StateScanning
			return m, nil
		}
	}

	// Delegate to StatusBar.
	updatedSB, sbCmd := m.StatusBar.Update(msg)
	if sb, ok := updatedSB.(components.StatusBar); ok {
		m.StatusBar = sb
	}
	cmds = append(cmds, sbCmd)

	// Delegate to Picker.
	updatedPicker, pCmd := m.Picker.Update(msg)
	if p, ok := updatedPicker.(components.Picker); ok {
		m.Picker = p
	}
	cmds = append(cmds, pCmd)

	return m, tea.Batch(cmds...)
}

// advanceToCatalogFreshCheck transitions the Model into StateCatalogFreshCheck
// with the appropriate picker items seeded. Used by both StartProcess (Wave 2)
// and the rehearsal-wipe Yes handler above.
func advanceToCatalogFreshCheck(m Model) (tea.Model, tea.Cmd) {
	m.State = StateCatalogFreshCheck
	m.StatusBar.State = string(StateCatalogFreshCheck)
	// The exact picker items + cursor (missing vs stale) are seeded by the
	// caller (Plan 04 Task 3 sets them via WithPickerCandidates before
	// tea.NewProgram, or by re-seeding here from m.Catalog.CacheState()).
	// For the chained case we re-seed defensively from m.Catalog metadata.
	if m.Catalog == nil {
		m.Picker.SetCandidates([]components.Candidate{
			{Name: "Download catalog (recommended)", Parenthetical: "fetch fresh xlsx", Confidence: 0},
			{Name: "Exit", Parenthetical: "abort", Confidence: 0},
		})
	} else {
		m.Picker.SetCandidates([]components.Candidate{
			{Name: "Use cached catalog", Parenthetical: "skip refresh", Confidence: 0},
			{Name: "Download fresh", Parenthetical: "refresh from sheet", Confidence: 0},
		})
	}
	m.Picker.Cursor = 0
	return m, nil
}
