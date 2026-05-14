package planner

import "path/filepath"

// RemapForRehearsal rewrites all PlannedAction.Dest fields in the plan tree
// to be under cfg.RehearsalDir. This enables a safe "rehearsal run" that
// moves files to a sandbox directory instead of the real install paths.
//
// PLN-07: all destination paths are rewritten; source paths are unchanged.
// The function walks depth-first through all Actions and their Children.
//
// Remapping strategy (Phase 4): keep the filename only, place under RehearsalDir.
// Example:
//
//	original: C:\vPinball\visualpinball\Tables\Flash (Williams, 1979).vpx
//	remapped: {RehearsalDir}\Flash (Williams, 1979).vpx
//
// Phase 6 may refine to preserve the relative subdirectory structure
// (e.g., Tables\, POVMedia\, etc.) using the Config content dirs as prefixes.
func RemapForRehearsal(plan *ProcessPlan, cfg *Config) {
	for _, action := range plan.Actions {
		remapAction(action, cfg.RehearsalDir)
	}
}

// remapAction rewrites Dest for a single PlannedAction and recurses into Children.
func remapAction(a *PlannedAction, rehearsalDir string) {
	if a.Dest != "" {
		a.Dest = filepath.Join(rehearsalDir, filepath.Base(a.Dest))
	}
	// Keep RegisterGame.Source in sync with the remapped Dest.
	if a.RegisterGame != nil && a.RegisterGame.Source != "" {
		a.RegisterGame.Source = a.Dest
	}
	for _, child := range a.Children {
		remapAction(child, rehearsalDir)
	}
}
