// Package app holds the root bubbletea Model and is the only place application
// state mutates. Per CLAUDE.md strict Elm architecture:
//   - Model owns ALL state.
//   - Update is the ONLY place state changes.
//   - View is pure — no I/O, no side effects, no goroutines.
package app

// AppState is the system-state label shown in StatusBar zone 4. Values are
// terse uppercase machine labels per CONTEXT.md decision §Status Bar.
type AppState string

const (
	StateIdle      AppState = "IDLE"
	StateLoading   AppState = "LOADING"
	StateScanning  AppState = "SCANNING"
	StateMatching  AppState = "MATCHING"
	StateExecuting AppState = "EXECUTING"
	StateDone      AppState = "DONE"

	StateConfirming         AppState = "CONFIRMING"           // plan-confirm sub-state (D-07)
	StateCatalogFreshCheck  AppState = "CATALOG_FRESH_CHECK"  // MOD-02: ask before refresh
	StateRehearsalWipeCheck AppState = "REHEARSAL_WIPE_CHECK" // D-16: ask before wipe
)
