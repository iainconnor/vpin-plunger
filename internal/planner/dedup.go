package planner

import (
	"fmt"
	"path/filepath"
	"sort"
)

// flattenActions returns all PlannedActions in the plan tree, depth-first.
// Top-level actions plus all Children recursively.
func flattenActions(plan *ProcessPlan) []*PlannedAction {
	var result []*PlannedAction
	for _, a := range plan.Actions {
		result = append(result, flattenAction(a)...)
	}
	return result
}

func flattenAction(a *PlannedAction) []*PlannedAction {
	result := []*PlannedAction{a}
	for _, child := range a.Children {
		result = append(result, flattenAction(child)...)
	}
	return result
}

// dedup performs the deduplication post-pass over the complete plan tree.
// It groups all matchable MOVE_* actions by their canonical (Type, Dest) key.
// For each group with more than one action, it:
//  1. Ranks by (Confidence DESC, Mtime DESC).
//  2. Declares the first as winner.
//  3. Routes each loser to SEND_TO_REVIEW with SupersededBy set.
//  4. Kills each loser's paired REGISTER_GAME via the RegisterGame back-pointer.
//
// reviewDir is cfg.ReviewDir from BuildPlan; losers' Dest is rewritten to
// filepath.Join(reviewDir, filepath.Base(loser.Source)).
//
// PLN-04: dedup is a pure in-memory computation — no filesystem access.
// PLN-03: the RegisterGame back-pointer enables correct REGISTER_GAME nullification.
//
// Dedup applies to all matchable action types: MOVE_VPX, MOVE_BACKGLASS, MOVE_POV.
// The dedup key is (Type, full Dest string including extension) — VPX and Backglass
// for the same game have different extensions and are never conflated (Pitfall 6).
func dedup(plan *ProcessPlan, reviewDir string) {
	all := flattenActions(plan)

	// Group by (Type, Dest) — only matchable types with non-empty Dest
	type key struct {
		t    ActionType
		dest string
	}
	groups := make(map[key][]*PlannedAction)
	for _, a := range all {
		if !dedupEligible(a) {
			continue
		}
		k := key{t: a.Type, dest: filepath.Clean(a.Dest)}
		groups[k] = append(groups[k], a)
	}

	for _, actions := range groups {
		if len(actions) <= 1 {
			continue
		}
		// Sort: highest confidence first; break ties by latest mtime (DESC)
		sort.Slice(actions, func(i, j int) bool {
			ci := confidenceOf(actions[i])
			cj := confidenceOf(actions[j])
			if ci != cj {
				return ci > cj
			}
			return actions[i].Mtime.After(actions[j].Mtime)
		})

		winner := actions[0]
		for _, loser := range actions[1:] {
			loser.Type = ActionTypeSendToReview
			loser.SupersededBy = winner.Source
			loser.Dest = filepath.Join(reviewDir, filepath.Base(loser.Source))
			loser.Reason = fmt.Sprintf(
				"duplicate destination — superseded by %s (confidence %d%%)",
				winner.Source, confidenceOf(winner),
			)
			// Kill orphaned REGISTER_GAME via back-pointer (PLN-03, PLN-04)
			if loser.RegisterGame != nil {
				loser.RegisterGame.Type = ActionTypeSkip
				loser.RegisterGame.Reason = "superseded by dedup (paired MOVE_VPX is a loser)"
			}
		}
	}
}

// dedupEligible returns true for action types that can produce duplicate
// canonical destinations. Only matchable rename-and-move types qualify.
func dedupEligible(a *PlannedAction) bool {
	switch a.Type {
	case ActionTypeMoveVPX, ActionTypeMoveBackglass, ActionTypeMovePOV:
		return a.Dest != ""
	default:
		return false
	}
}

// confidenceOf returns the match confidence for an action, or 0 if no match.
func confidenceOf(a *PlannedAction) int {
	if a.Match == nil {
		return 0
	}
	return a.Match.Confidence
}
