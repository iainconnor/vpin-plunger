package app

import (
	"github.com/iainconnor/vpin-plunger/internal/catalog"
	"github.com/iainconnor/vpin-plunger/internal/executor"
	"github.com/iainconnor/vpin-plunger/internal/planner"
)

// MatchRequest carries one interactive match prompt. Stored on Model.PendingMatch.
// Response is buffered(1) so writers never block when receiver is gone (RESEARCH Pitfall 3).
type MatchRequest struct {
	Stem       string
	Candidates []catalog.MatchResult
	Response   chan planner.MatchChoice
}

// CatalogLoadedMsg: catalog.Load() completed (D-04). LOADING -> SCANNING.
type CatalogLoadedMsg struct {
	Catalog *catalog.Catalog
}

// CatalogErrorMsg: catalog.Load() failed.
type CatalogErrorMsg struct {
	Err error
}

// ScanDoneMsg: BuildPlan completed (D-04). Carries the built plan.
type ScanDoneMsg struct {
	Plan *planner.ProcessPlan
}

// ScanErrorMsg: BuildPlan returned an error.
type ScanErrorMsg struct {
	Err error
}

// MatchRequestMsg: a MatchRequest from BuildPlan goroutine (D-04).
type MatchRequestMsg MatchRequest

// ExecuteProgressMsg: running outcome counts during ExecutePlan (D-04).
type ExecuteProgressMsg struct {
	Moved    int
	Failed   int
	Reviewed int
	Ignored  int
}

// ExecuteDoneMsg: ExecutePlan completion (D-04).
type ExecuteDoneMsg struct {
	Result *executor.ExecuteResult
	Err    error
}

// MonitorReportMsg: monitor command produced its three-section report.
type MonitorReportMsg struct {
	NotInstalled []catalog.SheetEntry
	NotInCatalog []DBGameRef // installed games missing from catalog
	NameMismatch []DBGameRef // installed games with name mismatch
}

// DBGameRef is a lightweight reference to one Games row, used by monitor.
// Defined here (not in db) because it carries display fields, not schema.
type DBGameRef struct {
	GameFileName string
	GameName     string
	Canonical    string // catalog canonical name (empty for NotInCatalog rows)
}

// DownloadGroupMsg: download command produced URL groups.
type DownloadGroupMsg struct {
	Groups []DownloadGroup
}

// DownloadGroup is one manufacturer+decade bucket of catalog entries to open.
type DownloadGroup struct {
	Manufacturer string
	Decade       int      // e.g. 1990
	URLs         []string // VPW + VPS URLs in order, deduped, non-empty
}
