package planner

import (
	"time"

	"github.com/iainconnor/vpin-plunger/internal/catalog"
)

// ActionType identifies the planned operation for a single asset or archive.
type ActionType int

const (
	ActionTypeUnknown      ActionType = iota // zero value — should never appear in a valid plan
	ActionTypeMoveVPX                        // rename + move .vpx → Config.VPXDir
	ActionTypeMoveBackglass                  // rename + move .directb2s → Config.BackglassDir
	ActionTypeMoveROM                        // copy verbatim (no rename) → Config.ROMDir
	ActionTypeMoveNVRAM                      // copy verbatim → Config.NVRAMDir
	ActionTypeMovePOV                        // rename + move .ini → Config.POVDir
	ActionTypeMoveUltraDMD                   // move directory → Config.DMDDir
	ActionTypeMoveFlexDMD                    // move directory → Config.FlexDMDDir
	ActionTypeMoveAudio                      // move directory → Config.AudioDir (Altsound)
	ActionTypeMoveAltcolor                   // move directory → Config.AltcolorDir
	ActionTypeMoveMusic                      // move directory → Config.MusicDir
	ActionTypeMovePUP                        // move directory → Config.PuPDir
	ActionTypeExtractArchive                 // extract distribution archive to temp subdir; children: VAULT_ARCHIVE + member actions
	ActionTypeVaultArchive                   // move original archive → Config.ArchiveVaultDir
	ActionTypeRegisterGame                   // write Games row to PUPDatabase (paired with MOVE_VPX via back-pointer)
	ActionTypeSendToReview                   // move to Config.ReviewDir + REVIEW.md entry
	ActionTypeIgnore                         // move to Config.IgnoredDir
	ActionTypeSkip                           // dedup nullification sentinel: orphaned REGISTER_GAME with a losing MOVE_VPX
)

// String returns the display label for an ActionType (e.g. "MOVE_VPX").
func (a ActionType) String() string {
	switch a {
	case ActionTypeMoveVPX:
		return "MOVE_VPX"
	case ActionTypeMoveBackglass:
		return "MOVE_BACKGLASS"
	case ActionTypeMoveROM:
		return "MOVE_ROM"
	case ActionTypeMoveNVRAM:
		return "MOVE_NVRAM"
	case ActionTypeMovePOV:
		return "MOVE_POV"
	case ActionTypeMoveUltraDMD:
		return "MOVE_ULTRADMD"
	case ActionTypeMoveFlexDMD:
		return "MOVE_FLEXDMD"
	case ActionTypeMoveAudio:
		return "MOVE_AUDIO"
	case ActionTypeMoveAltcolor:
		return "MOVE_ALTCOLOR"
	case ActionTypeMoveMusic:
		return "MOVE_MUSIC"
	case ActionTypeMovePUP:
		return "MOVE_PUP"
	case ActionTypeExtractArchive:
		return "EXTRACT_ARCHIVE"
	case ActionTypeVaultArchive:
		return "VAULT_ARCHIVE"
	case ActionTypeRegisterGame:
		return "REGISTER_GAME"
	case ActionTypeSendToReview:
		return "SEND_TO_REVIEW"
	case ActionTypeIgnore:
		return "IGNORE"
	case ActionTypeSkip:
		return "SKIP"
	default:
		return "UNKNOWN"
	}
}

// MatchChoice is returned by a MatchFn callback. Exactly one field is set
// non-zero by the caller. The planner inspects them in order: Match →
// ForceID → SendToReview → Ignore.
type MatchChoice struct {
	Match        *catalog.MatchResult // user selected a candidate from the picker
	ForceID      string               // user typed a MasterID or IPDBNum (non-empty = force search)
	SendToReview bool                 // user deferred this file to review
	Ignore       bool                 // user chose to ignore this file entirely
}

// MatchFn is the interactive callback BuildPlan calls when a file falls in
// the interactive confidence range (ThresholdInteractive ≤ score < ThresholdAutoAssign).
//
// Parameters:
//
//	stem       — the bare filename stem (no extension, no path) of the asset
//	candidates — scored match candidates from catalog.FindMatch, highest score first
//
// In Phase 4 tests, MatchFn is always a synchronous auto-select function.
// In Phase 6, the app layer wires a channel pair inside a tea.Cmd to drive
// the bubbletea picker through this same callback. BuildPlan is unaware of
// the TUI and completes in a single synchronous pass.
//
// MatchFn must not be nil when BuildPlan is called. Use AutoSelectMatchFn
// for non-interactive (--auto) mode.
type MatchFn func(stem string, candidates []catalog.MatchResult) MatchChoice

// AutoSelectMatchFn is a MatchFn implementation for non-interactive (--auto)
// mode. It auto-selects the best candidate if its confidence is at or above
// ThresholdAutoAssign; otherwise sends to review.
// Tests inject this to exercise BuildPlan without a TUI.
func AutoSelectMatchFn(stem string, candidates []catalog.MatchResult) MatchChoice {
	if len(candidates) > 0 && candidates[0].Confidence >= catalog.ThresholdAutoAssign {
		return MatchChoice{Match: &candidates[0]}
	}
	return MatchChoice{SendToReview: true}
}

// PlannedAction is one node in the ProcessPlan tree. Top-level actions are
// loose files or archive roots; Children are populated only for
// ActionTypeExtractArchive.
//
// Back-pointer: for ActionTypeMoveVPX, RegisterGame points to the paired
// ActionTypeRegisterGame action. The dedup post-pass kills orphaned
// REGISTER_GAME actions via this pointer (PLN-03, PLN-04).
type PlannedAction struct {
	Type         ActionType
	Source       string               // absolute path on disk (or virtual for archive members)
	Dest         string               // absolute destination path; empty for REGISTER_GAME and SKIP
	VirtualPath  string               // display path: "archive.zip/member.vpx"; empty for loose files (PLN-05)
	Match        *catalog.MatchResult // nil for unmatched, review, ROM, and bundle actions
	Reason       string               // human-readable explanation of routing decision
	Mtime        time.Time            // file mtime; archive members use parent archive mtime (RESEARCH Pitfall 2)
	Children     []*PlannedAction     // non-nil only for ActionTypeExtractArchive
	RegisterGame *PlannedAction       // back-pointer: set on MOVE_VPX → its paired REGISTER_GAME (PLN-03)
	SupersededBy string               // set by dedup on losers; contains winner's Source path (PLN-04)
}

// ProcessPlan is the complete output of BuildPlan. Top-level Actions contains
// only root-level items; archive member actions are embedded in their parent's
// Children slice.
type ProcessPlan struct {
	Actions      []*PlannedAction
	DownloadsDir string // the root directory that was scanned
}

// BuildPlan performs a two-pass scan of downloadsDir and returns a complete
// ProcessPlan tree. It is a pure computation: no filesystem writes, no prints,
// no goroutines (D-07, D-10).
//
// Pass 1 (bundle pre-pass): classifies direct-child directories of downloadsDir;
// claims recognised bundles so their members are not re-classified in Pass 2.
//
// Pass 2 (recursive walk): calls matchFn for each matchable file in the
// interactive confidence range. matchFn must not be nil.
//
// If cfg.Rehearsal is true, BuildPlan calls RemapForRehearsal before returning.
//
// Implemented in scan.go (Pass 1+2) and match.go (match integration).
// This stub satisfies the compiler for Wave 1.
func BuildPlan(
	downloadsDir string,
	cat *catalog.Catalog,
	cfg *Config,
	matchFn MatchFn,
) (*ProcessPlan, error) {
	// Stub: full implementation assembled across 04-02 (scan.go) and 04-03 (match.go).
	return &ProcessPlan{DownloadsDir: downloadsDir}, nil
}
