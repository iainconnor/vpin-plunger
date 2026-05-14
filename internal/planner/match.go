// match.go — catalog matching post-pass for BuildPlan.
//
// applyMatches walks the ProcessPlan tree produced by buildScanActions and
// resolves Match=nil actions by calling catalog.BestMatch (auto-assign),
// matchFn (interactive range), or routing to review (below threshold).
//
// Only matchable ActionTypes (MOVE_VPX, MOVE_BACKGLASS, MOVE_POV) are
// processed; all other action types are left unchanged.
package planner

import (
	"path/filepath"
	"strings"

	"github.com/iainconnor/vpin-plunger/internal/catalog"
)

// matchableTypes are the ActionTypes that require a catalog match to determine
// canonical destination filename. Bundle types (MOVE_ALTCOLOR, MOVE_MUSIC etc.)
// are not matchable — they are moved verbatim by directory name.
var matchableTypes = map[ActionType]bool{
	ActionTypeMoveVPX:       true,
	ActionTypeMoveBackglass: true,
	ActionTypeMovePOV:       true,
}

// stemFromSource returns the bare filename stem from a PlannedAction Source.
// For loose files: filepath.Base minus extension.
// For archive members (VirtualPath set): the member basename minus extension.
func stemFromSource(action *PlannedAction) string {
	base := filepath.Base(action.Source)
	// Strip extension(s): handle ".vpx.zip" double extension by only stripping last.
	ext := filepath.Ext(base)
	stem := strings.TrimSuffix(base, ext)
	return stem
}

// resolveMatch performs catalog matching for a single matchable PlannedAction.
// It uses BestMatch for auto-assign, matchFn for the interactive range,
// and routes to SEND_TO_REVIEW below the interactive threshold.
//
// After a successful match, it rewrites action.Dest to the canonical filename
// and updates RegisterGame.Source if a back-pointer exists.
//
// D-08: Thresholds are 92 (auto) and 72 (interactive). These may need tuning.
func resolveMatch(action *PlannedAction, cat *catalog.Catalog, cfg *Config, matchFn MatchFn) error {
	stem := stemFromSource(action)

	// Attempt BestMatch: returns non-nil if >= ThresholdInteractive (72)
	best, err := cat.BestMatch(stem)
	if err != nil {
		return err
	}

	var chosen *catalog.MatchResult

	if best == nil {
		// Below interactive threshold — always review
		action.Type = ActionTypeSendToReview
		action.Dest = filepath.Join(cfg.ReviewDir, filepath.Base(action.Source))
		action.Reason = "no catalog match above review threshold (< 72%)"
		action.Match = nil
		// Kill paired REGISTER_GAME if any
		if action.RegisterGame != nil {
			action.RegisterGame.Type = ActionTypeSkip
			action.RegisterGame.Reason = "no catalog match for paired MOVE_VPX"
		}
		return nil
	}

	if best.Confidence >= catalog.ThresholdAutoAssign {
		// Auto-assign: confidence >= 92
		chosen = best
		action.Reason = "auto-assigned at " + confidenceStr(best.Confidence) + "%"
	} else {
		// Interactive range: 72 <= confidence < 92 — call matchFn
		candidates, err := cat.FindMatch(stem, 5)
		if err != nil {
			return err
		}
		choice := matchFn(stem, candidates)

		switch {
		case choice.Match != nil:
			chosen = choice.Match
			action.Reason = "user selected match at " + confidenceStr(choice.Match.Confidence) + "%"

		case choice.ForceID != "":
			forced, err := cat.ForceMatch(choice.ForceID)
			if err != nil {
				return err
			}
			if forced == nil {
				// ForceMatch returned no result — send to review
				action.Type = ActionTypeSendToReview
				action.Dest = filepath.Join(cfg.ReviewDir, filepath.Base(action.Source))
				action.Reason = "force-match ID '" + choice.ForceID + "' not found in catalog"
				if action.RegisterGame != nil {
					action.RegisterGame.Type = ActionTypeSkip
					action.RegisterGame.Reason = "force-match not found for paired MOVE_VPX"
				}
				return nil
			}
			chosen = forced
			action.Reason = "force-matched by ID '" + choice.ForceID + "' (confidence=100)"

		case choice.SendToReview:
			action.Type = ActionTypeSendToReview
			action.Dest = filepath.Join(cfg.ReviewDir, filepath.Base(action.Source))
			action.Reason = "user sent to review"
			if action.RegisterGame != nil {
				action.RegisterGame.Type = ActionTypeSkip
				action.RegisterGame.Reason = "user sent paired MOVE_VPX to review"
			}
			return nil

		case choice.Ignore:
			action.Type = ActionTypeIgnore
			action.Dest = filepath.Join(cfg.IgnoredDir, filepath.Base(action.Source))
			action.Reason = "user ignored"
			if action.RegisterGame != nil {
				action.RegisterGame.Type = ActionTypeSkip
				action.RegisterGame.Reason = "user ignored paired MOVE_VPX"
			}
			return nil

		default:
			// No arm set — treat as review
			action.Type = ActionTypeSendToReview
			action.Dest = filepath.Join(cfg.ReviewDir, filepath.Base(action.Source))
			action.Reason = "matchFn returned empty MatchChoice"
			if action.RegisterGame != nil {
				action.RegisterGame.Type = ActionTypeSkip
				action.RegisterGame.Reason = "matchFn returned empty MatchChoice for paired MOVE_VPX"
			}
			return nil
		}
	}

	// Apply the chosen match: rewrite Dest to canonical filename
	action.Match = chosen
	canonicalStem := chosen.Entry.CanonicalFilename(cfg.TrailingArticle)
	ext := filepath.Ext(action.Source)
	// For archive members, ext comes from the virtual member path
	if action.VirtualPath != "" {
		ext = filepath.Ext(action.VirtualPath)
	}
	canonicalName := canonicalStem + ext
	action.Dest = rewriteDestCanonical(action.Dest, canonicalName, cfg)

	// Update paired REGISTER_GAME source to match new Dest
	if action.RegisterGame != nil {
		action.RegisterGame.Source = action.Dest
	}

	return nil
}

// confidenceStr formats an integer confidence as a string without importing fmt.
func confidenceStr(c int) string {
	if c == 100 {
		return "100"
	}
	// Use strconv-free approach for the three-digit range we care about
	s := []byte{'0', '0', '0'}
	s[2] = byte('0' + c%10)
	s[1] = byte('0' + (c/10)%10)
	s[0] = byte('0' + (c/100)%10)
	// Trim leading zeros (keep at least one digit)
	i := 0
	for i < 2 && s[i] == '0' {
		i++
	}
	return string(s[i:])
}

// rewriteDestCanonical replaces the filename component of the current dest path
// with the new canonical filename, preserving the directory.
func rewriteDestCanonical(currentDest, canonicalName string, _ *Config) string {
	dir := filepath.Dir(currentDest)
	return filepath.Join(dir, canonicalName)
}

// applyMatches walks all PlannedActions in the plan (including archive children)
// and calls resolveMatch for each matchable action (MOVE_VPX, MOVE_BACKGLASS,
// MOVE_POV). Non-matchable actions are left unchanged.
//
// matchFn must not be nil. Use AutoSelectMatchFn for --auto mode.
func applyMatches(plan *ProcessPlan, cat *catalog.Catalog, cfg *Config, matchFn MatchFn) error {
	for _, action := range plan.Actions {
		if err := applyMatchesAction(action, cat, cfg, matchFn); err != nil {
			return err
		}
	}
	return nil
}

func applyMatchesAction(action *PlannedAction, cat *catalog.Catalog, cfg *Config, matchFn MatchFn) error {
	if matchableTypes[action.Type] {
		if err := resolveMatch(action, cat, cfg, matchFn); err != nil {
			return err
		}
	}
	// Recurse into children (EXTRACT_ARCHIVE subtrees)
	for _, child := range action.Children {
		if err := applyMatchesAction(child, cat, cfg, matchFn); err != nil {
			return err
		}
	}
	return nil
}
