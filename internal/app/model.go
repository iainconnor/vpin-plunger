package app

import (
	tea "charm.land/bubbletea/v2"

	"github.com/iainconnor/vpin-plunger/internal/catalog"
	"github.com/iainconnor/vpin-plunger/internal/executor"
	"github.com/iainconnor/vpin-plunger/internal/planner"
	"github.com/iainconnor/vpin-plunger/internal/ui/components"
)

// Model is the root bubbletea Model. It owns all application state.
//
// Sub-components (StatusBar, Picker) are embedded as fields, NOT pointers —
// bubbletea v2 returns updated models by value through Update.
type Model struct {
	State     AppState
	Width     int
	Height    int
	StatusBar components.StatusBar
	Picker    components.Picker

	// Phase 6 additions:
	PendingMatch   *MatchRequest      // non-nil while a match prompt is open (D-02)
	Confirming     bool               // true while plan-confirm picker is shown (D-07)
	ProgressCounts ExecuteProgressMsg // last counts for status bar display (MOD-04)
	initCmd        tea.Cmd            // returned from Init(); set via WithInitCmd

	// Per-command runtime context (populated by initial Cmds; nil for irrelevant commands):
	Catalog      *catalog.Catalog
	Plan         *planner.ProcessPlan
	ScanCfg      *planner.Config
	ExecCfg      *executor.Config
	DownloadsDir string
	AutoMode     bool
	DBPath       string
	RehearsalDir string // set when --rehearsal so StateRehearsalWipeCheck knows the path to wipe
	NeedCatalogDL bool  // set when initial freshness check picked Yes — drives loadCatalogCmd after picker

	// Channel-pair handles (set by scanCmd; nil before scanning starts).
	matchReqCh chan MatchRequest
}

// Option is a functional option for New().
type Option func(*Model)

// WithInitCmd sets the initial tea.Cmd returned from Init().
func WithInitCmd(cmd tea.Cmd) Option { return func(m *Model) { m.initCmd = cmd } }

// WithAutoMode sets auto mode (no interactive prompts).
func WithAutoMode(auto bool) Option { return func(m *Model) { m.AutoMode = auto } }

// WithDownloadsDir sets the downloads directory.
func WithDownloadsDir(dir string) Option { return func(m *Model) { m.DownloadsDir = dir } }

// WithScanConfig sets the planner config.
func WithScanConfig(cfg *planner.Config) Option { return func(m *Model) { m.ScanCfg = cfg } }

// WithExecConfig sets the executor config.
func WithExecConfig(cfg *executor.Config) Option { return func(m *Model) { m.ExecCfg = cfg } }

// WithDBPath sets the database path.
func WithDBPath(p string) Option { return func(m *Model) { m.DBPath = p } }

// WithRehearsalDir sets the rehearsal directory path.
func WithRehearsalDir(p string) Option { return func(m *Model) { m.RehearsalDir = p } }

// WithInitialState sets the initial AppState and mirrors it to the StatusBar.
func WithInitialState(s AppState) Option {
	return func(m *Model) { m.State = s; m.StatusBar.State = string(s) }
}

// WithPickerCandidates pre-seeds the Picker with candidates.
func WithPickerCandidates(c []components.Candidate) Option {
	return func(m *Model) { m.Picker.SetCandidates(c); m.Picker.Cursor = 0 }
}

// WithCatalog sets the catalog on the model.
func WithCatalog(c *catalog.Catalog) Option { return func(m *Model) { m.Catalog = c } }

// New constructs the initial Model with fixture-populated sub-components so
// the visual chrome is reviewable before real data arrives in later phases.
//
// Initial Width is 80 (a reasonable default; the first WindowSizeMsg replaces
// it before the first render is committed in alt-screen mode).
// Compile-time assertions: verify that app.AppState constants and
// components.StatusState constants stay in sync. If you add a new state to
// either side without updating the other, this block will fail to compile.
// The two types are intentionally kept separate to avoid an import cycle
// (components cannot import app). Canonical source: internal/app/state.go.
var _ = [1]struct{}{}[0] // no-op anchor so the block is parseable without real assertions

func init() {
	// Runtime sync check — fires once at program start; panics if the two
	// constant sets have drifted. Kept in init() because Go does not allow
	// compile-time string equality comparisons outside of unsafe tricks.
	pairs := [][2]string{
		{string(StateIdle), components.StatusStateIdle},
		{string(StateLoading), components.StatusStateLoading},
		{string(StateScanning), components.StatusStateScanning},
		{string(StateMatching), components.StatusStateMatching},
		{string(StateExecuting), components.StatusStateExecuting},
		{string(StateDone), components.StatusStateDone},
		{string(StateConfirming), components.StatusStateConfirming},
		{string(StateCatalogFreshCheck), components.StatusStateCatalogFreshCheck},
		{string(StateRehearsalWipeCheck), components.StatusStateRehearsalWipeCheck},
	}
	for _, p := range pairs {
		if p[0] != p[1] {
			panic("app.AppState / components.StatusState mismatch: " + p[0] + " != " + p[1])
		}
	}
}

// New constructs the initial Model. Accepts variadic Option for per-command
// customization (backward-compatible: zero-arg New() still works).
func New(opts ...Option) Model {
	const initialWidth = 80
	sb := components.NewStatusBar(initialWidth)
	// StatusState is a type alias for string; AppState underlying value matches.
	sb.State = string(StateIdle)
	m := Model{
		State:     StateIdle,
		Width:     initialWidth,
		Height:    24,
		StatusBar: sb,
		Picker:    components.NewPicker(),
	}
	for _, opt := range opts {
		opt(&m)
	}
	return m
}

// Init satisfies tea.Model. It composes the Init commands of all sub-components
// so spinners/blinks start ticking immediately. If an initCmd was set via
// WithInitCmd, it is batched with the sub-component commands.
func (m Model) Init() tea.Cmd {
	sbCmd := m.StatusBar.Init()
	pCmd := m.Picker.Init()
	if m.initCmd != nil {
		return tea.Batch(sbCmd, pCmd, m.initCmd)
	}
	return tea.Batch(sbCmd, pCmd)
}
