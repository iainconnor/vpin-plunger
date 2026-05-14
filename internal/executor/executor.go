// Package executor performs the filesystem mutations and database writes described
// in a *planner.ProcessPlan. It is a pure-execution package: no TUI dependency
// (no lipgloss, no bubbletea imports), no raw goroutines (D-12).
// Standalone Config avoids circular imports with internal/config/ (D-04).
// Phase 6 wraps ExecutePlan in a tea.Cmd.
package executor

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"time"

	"github.com/iainconnor/vpin-plunger/internal/db"
	"github.com/iainconnor/vpin-plunger/internal/formats"
	"github.com/iainconnor/vpin-plunger/internal/planner"
)

// Config holds all destination paths for ExecutePlan.
// Standalone — no import of internal/config/ — to avoid circular imports.
// Phase 6 constructs this from CLI flags and path discovery.
// Tests inject a Config pointing at temp directories.
type Config struct {
	// 11 content destination directories (one per asset type)
	VPXDir       string
	BackglassDir  string
	ROMDir        string
	NVRAMDir      string
	POVDir        string
	DMDDir        string
	FlexDMDDir    string
	AudioDir      string
	AltcolorDir   string
	MusicDir      string
	PuPDir        string

	// 4 operational directories
	ArchiveVaultDir string
	ReviewDir       string
	IgnoredDir      string
	RehearsalDir    string

	// DB path — real or rehearsal path; db/ is unaware which (D-04)
	DBPath string

	// BackupDir — directory for PUPDatabase backups (sibling to DBPath)
	BackupDir string
}

// OutcomeStatus describes the result of executing one PlannedAction.
type OutcomeStatus int

const (
	OutcomeSucceeded  OutcomeStatus = iota // file moved to intended destination
	OutcomeFailed                          // primary move failed; file moved to review/ (D-05)
	OutcomeFailedTwice                     // primary + review/ both failed; file left in place (D-06)
	OutcomeSkipped                         // ActionTypeSkip — dedup sentinel, no-op
	OutcomeReviewed                        // SEND_TO_REVIEW — moved to review/
	OutcomeIgnored                         // IGNORE — moved to ignored/
)

// String returns the display label for an OutcomeStatus.
func (s OutcomeStatus) String() string {
	switch s {
	case OutcomeSucceeded:
		return "succeeded"
	case OutcomeFailed:
		return "failed"
	case OutcomeFailedTwice:
		return "failed_twice"
	case OutcomeSkipped:
		return "skipped"
	case OutcomeReviewed:
		return "reviewed"
	case OutcomeIgnored:
		return "ignored"
	default:
		return "unknown"
	}
}

// ActionOutcome records the result of one PlannedAction.
type ActionOutcome struct {
	Action      *planner.PlannedAction
	Status      OutcomeStatus
	Error       error // primary error (nil on success)
	FallbackErr error // review/ fallback error (non-nil only for OutcomeFailedTwice, D-06)
}

// ExecuteResult is the aggregate result of ExecutePlan (D-01).
// Phase 6 renders the summary string from these fields.
type ExecuteResult struct {
	Moved    int
	Failed   int
	Reviewed int
	Ignored  int
	Outcomes []ActionOutcome
}

// ExecutePlan executes all actions in plan, performing filesystem moves,
// archive extractions, DB registrations, and REVIEW.md appending.
// It never aborts on a single action failure — all planned actions run (EXE-09).
// database may be nil if DBPath was not configured (REGISTER_GAME actions fail gracefully).
func ExecutePlan(plan *planner.ProcessPlan, cfg *Config, database *db.DB) (*ExecuteResult, error) {
	result := &ExecuteResult{}
	var reviewEntries []reviewEntry
	runTime := time.Now()

	for _, action := range plan.Actions {
		outcomes := executeAction(action, cfg, database, &reviewEntries)
		for _, o := range outcomes {
			result.Outcomes = append(result.Outcomes, o)
			switch o.Status {
			case OutcomeSucceeded:
				result.Moved++
			case OutcomeFailed, OutcomeFailedTwice:
				result.Failed++
			case OutcomeReviewed:
				result.Reviewed++
			case OutcomeIgnored:
				result.Ignored++
			}
		}
	}

	// Append REVIEW.md after full plan walk (gate: only when entries exist, Pitfall 5)
	if err := appendREVIEW(cfg.ReviewDir, reviewEntries, runTime); err != nil {
		// Non-fatal: log as a warning but do not fail ExecutePlan
		result.Outcomes = append(result.Outcomes, ActionOutcome{
			Status: OutcomeFailed,
			Error:  fmt.Errorf("REVIEW.md append: %w", err),
		})
	}

	return result, nil
}

// executeAction executes a single PlannedAction and returns all resulting outcomes.
// For ActionTypeExtractArchive, it recurses into action.Children.
// The tree is walked by action.Type only — RegisterGame back-pointer is never followed
// for execution (RESEARCH Pitfall 6).
func executeAction(action *planner.PlannedAction, cfg *Config, database *db.DB, reviewEntries *[]reviewEntry) []ActionOutcome {
	switch action.Type {
	// ── File move actions ──────────────────────────────────────────────────────
	case planner.ActionTypeMoveVPX,
		planner.ActionTypeMoveBackglass,
		planner.ActionTypeMoveROM,
		planner.ActionTypeMoveNVRAM,
		planner.ActionTypeMovePOV:
		o := executeMoveFile(action, cfg, reviewEntries)
		return []ActionOutcome{o}

	// ── Directory move actions ─────────────────────────────────────────────────
	case planner.ActionTypeMoveUltraDMD,
		planner.ActionTypeMoveFlexDMD,
		planner.ActionTypeMoveAudio,
		planner.ActionTypeMoveAltcolor,
		planner.ActionTypeMoveMusic,
		planner.ActionTypeMovePUP:
		o := executeMoveDir(action, cfg, reviewEntries)
		return []ActionOutcome{o}

	// ── Archive extraction ─────────────────────────────────────────────────────
	case planner.ActionTypeExtractArchive:
		return executeExtractArchive(action, cfg, database, reviewEntries)

	// ── Vault archive ──────────────────────────────────────────────────────────
	case planner.ActionTypeVaultArchive:
		if err := moveFile(action.Source, action.Dest); err != nil {
			return []ActionOutcome{{Action: action, Status: OutcomeFailed, Error: err}}
		}
		return []ActionOutcome{{Action: action, Status: OutcomeSucceeded}}

	// ── Register game in PUP database ──────────────────────────────────────────
	case planner.ActionTypeRegisterGame:
		return executeRegisterGame(action, database)

	// ── Send to review ─────────────────────────────────────────────────────────
	case planner.ActionTypeSendToReview:
		reviewDest := filepath.Join(cfg.ReviewDir, filepath.Base(action.Source))
		if err := moveFile(action.Source, reviewDest); err != nil {
			*reviewEntries = append(*reviewEntries, reviewEntry{
				Source:    action.Source,
				Dest:      reviewDest,
				Action:    "SEND_TO_REVIEW",
				Reason:    fmt.Sprintf("move to review failed: %v", err),
				Timestamp: time.Now(),
			})
			return []ActionOutcome{{Action: action, Status: OutcomeFailed, Error: err}}
		}
		*reviewEntries = append(*reviewEntries, reviewEntry{
			Source:    action.Source,
			Dest:      reviewDest,
			Action:    "SEND_TO_REVIEW",
			Reason:    action.Reason,
			Timestamp: time.Now(),
		})
		return []ActionOutcome{{Action: action, Status: OutcomeReviewed}}

	// ── Ignore — always appends reviewEntry (D-07) ────────────────────────────
	case planner.ActionTypeIgnore:
		ignoreDest := filepath.Join(cfg.IgnoredDir, filepath.Base(action.Source))
		moveErr := moveFile(action.Source, ignoreDest)
		*reviewEntries = append(*reviewEntries, reviewEntry{
			Source:    action.Source,
			Dest:      ignoreDest,
			Action:    "IGNORE",
			Reason:    action.Reason,
			Timestamp: time.Now(),
		})
		if moveErr != nil {
			return []ActionOutcome{{Action: action, Status: OutcomeFailed, Error: moveErr}}
		}
		return []ActionOutcome{{Action: action, Status: OutcomeIgnored}}

	// ── Skip — dedup nullification sentinel, no-op ────────────────────────────
	case planner.ActionTypeSkip:
		return []ActionOutcome{{Action: action, Status: OutcomeSkipped}}

	// ── Unknown — should never appear in a valid plan ─────────────────────────
	default: // ActionTypeUnknown
		return []ActionOutcome{{Action: action, Status: OutcomeSkipped,
			Error: fmt.Errorf("unknown action type: %d", action.Type)}}
	}
}

// executeExtractArchive handles ActionTypeExtractArchive.
// 1. Detects the archive handler by extension.
// 2. Creates an extraction subdirectory next to the archive.
// 3. Extracts the archive.
// 4. Recurses into action.Children (which include VAULT_ARCHIVE and member actions).
// 5. Scans extractDir for nested archives and extracts them too (EXE-04).
func executeExtractArchive(action *planner.PlannedAction, cfg *Config, database *db.DB, reviewEntries *[]reviewEntry) []ActionOutcome {
	var outcomes []ActionOutcome

	handler := detectHandler(action.Source)
	if handler == nil {
		outcomes = append(outcomes, ActionOutcome{
			Action: action,
			Status: OutcomeFailed,
			Error:  fmt.Errorf("executeExtractArchive: no handler for %s", action.Source),
		})
		return outcomes
	}

	// Extract to a subdir named after the archive stem
	base := filepath.Base(action.Source)
	stem := base[:len(base)-len(filepath.Ext(base))]
	extractDir := filepath.Join(filepath.Dir(action.Source), stem)

	if err := handler.Extract(action.Source, extractDir); err != nil {
		outcomes = append(outcomes, ActionOutcome{
			Action: action,
			Status: OutcomeFailed,
			Error:  fmt.Errorf("executeExtractArchive extract %s: %w", action.Source, err),
		})
		return outcomes
	}

	// Process children (VAULT_ARCHIVE + member actions)
	for _, child := range action.Children {
		childOutcomes := executeAction(child, cfg, database, reviewEntries)
		outcomes = append(outcomes, childOutcomes...)
	}

	// Scan extractDir for nested archives (EXE-04)
	entries, err := os.ReadDir(extractDir)
	if err == nil {
		for _, entry := range entries {
			if entry.IsDir() {
				continue
			}
			nestedPath := filepath.Join(extractDir, entry.Name())
			f, openErr := os.Open(nestedPath)
			if openErr != nil {
				continue
			}
			assetType, classErr := formats.ClassifyFile(nestedPath, f)
			f.Close()
			if classErr != nil {
				continue
			}
			if assetType == formats.AssetTypeArchive {
				nestedHandler := detectHandler(nestedPath)
				if nestedHandler != nil {
					nestedStem := entry.Name()[:len(entry.Name())-len(filepath.Ext(entry.Name()))]
					nestedDir := filepath.Join(extractDir, nestedStem)
					if extractErr := nestedHandler.Extract(nestedPath, nestedDir); extractErr == nil {
						os.Remove(nestedPath)
					}
				}
			}
		}
	}

	return outcomes
}

// executeRegisterGame handles ActionTypeRegisterGame.
// Builds a db.GameRecord from the action's fields and calls database.UpsertGame.
// Failure is non-fatal (execution continues).
func executeRegisterGame(action *planner.PlannedAction, database *db.DB) []ActionOutcome {
	if database == nil {
		return []ActionOutcome{{Action: action, Status: OutcomeFailed,
			Error: fmt.Errorf("no database configured")}}
	}

	gameName := ""
	gameYear := ""
	manufact := ""
	gameType := "EM"
	ipdbNum := ""
	webLinkURL := ""
	tags := ""

	if action.Match != nil {
		gameName = action.Match.Entry.Name
		gameYear = strconv.Itoa(action.Match.Entry.Year)
		manufact = action.Match.Entry.Manufacturer
		ipdbNum = action.Match.Entry.IPDBNum
		webLinkURL = action.Match.Entry.IPDBUrl
		if action.Match.Entry.VPWLink != "" {
			tags = `"VPW"`
		}
	}
	if gameName == "" {
		gameName = filepath.Base(action.Source)
	}

	rec := db.GameRecord{
		GameFileName: filepath.Base(action.Source),
		GameName:     gameName,
		GameYear:     gameYear,
		Manufact:     manufact,
		GameType:     gameType,
		IPDBNum:      ipdbNum,
		WebLinkURL:   webLinkURL,
		Tags:         tags,
	}
	if err := database.UpsertGame(rec, action.Mtime); err != nil {
		return []ActionOutcome{{Action: action, Status: OutcomeFailed, Error: err}}
	}
	return []ActionOutcome{{Action: action, Status: OutcomeSucceeded}}
}

// executeMoveFile moves a single file to action.Dest with review/ fallback (D-05, D-06).
func executeMoveFile(action *planner.PlannedAction, cfg *Config, reviewEntries *[]reviewEntry) ActionOutcome {
	if err := moveFile(action.Source, action.Dest); err == nil {
		return ActionOutcome{Action: action, Status: OutcomeSucceeded}
	} else {
		// Fallback: attempt move to review/ (D-05)
		reviewDest := filepath.Join(cfg.ReviewDir, filepath.Base(action.Source))
		if err2 := moveFile(action.Source, reviewDest); err2 == nil {
			*reviewEntries = append(*reviewEntries, reviewEntry{
				Source:    action.Source,
				Dest:      action.Dest,
				Action:    action.Type.String(),
				Reason:    fmt.Sprintf("move failed: %v", err),
				Timestamp: time.Now(),
			})
			return ActionOutcome{Action: action, Status: OutcomeFailed, Error: err}
		} else {
			// FailedTwice: file left in place (D-06)
			*reviewEntries = append(*reviewEntries, reviewEntry{
				Source:    action.Source,
				Dest:      action.Dest,
				Action:    action.Type.String(),
				Reason:    fmt.Sprintf("move failed: %v; review fallback also failed: %v", err, err2),
				Timestamp: time.Now(),
			})
			return ActionOutcome{Action: action, Status: OutcomeFailedTwice, Error: err, FallbackErr: err2}
		}
	}
}

// executeMoveDir moves a directory to action.Dest with review/ fallback (D-05, D-06).
func executeMoveDir(action *planner.PlannedAction, cfg *Config, reviewEntries *[]reviewEntry) ActionOutcome {
	if err := moveDir(action.Source, action.Dest); err == nil {
		return ActionOutcome{Action: action, Status: OutcomeSucceeded}
	} else {
		// Fallback: attempt move to review/ (D-05)
		reviewDest := filepath.Join(cfg.ReviewDir, filepath.Base(action.Source))
		if err2 := moveDir(action.Source, reviewDest); err2 == nil {
			*reviewEntries = append(*reviewEntries, reviewEntry{
				Source:    action.Source,
				Dest:      action.Dest,
				Action:    action.Type.String(),
				Reason:    fmt.Sprintf("move failed: %v", err),
				Timestamp: time.Now(),
			})
			return ActionOutcome{Action: action, Status: OutcomeFailed, Error: err}
		} else {
			// FailedTwice: file left in place (D-06)
			*reviewEntries = append(*reviewEntries, reviewEntry{
				Source:    action.Source,
				Dest:      action.Dest,
				Action:    action.Type.String(),
				Reason:    fmt.Sprintf("move failed: %v; review fallback also failed: %v", err, err2),
				Timestamp: time.Now(),
			})
			return ActionOutcome{Action: action, Status: OutcomeFailedTwice, Error: err, FallbackErr: err2}
		}
	}
}

// detectHandler returns the appropriate formats.Handler for an archive file.
// Returns nil if no handler matches (file is not a known archive type).
// Uses a fixed inline slice of the three known handlers — ZIPHandler,
// SevenZipHandler, RARHandler — rather than a registry function that does not exist.
func detectHandler(path string) formats.Handler {
	ctx := context.Background()
	f, err := os.Open(path)
	if err != nil {
		return nil
	}
	defer f.Close()
	// Inline slice of all known archive handlers (no RegisteredHandlers() function exists).
	handlers := []formats.Handler{
		formats.ZIPHandler{},
		formats.SevenZipHandler{},
		formats.RARHandler{},
	}
	for _, h := range handlers {
		if h.Detect(ctx, path, f) {
			return h
		}
	}
	return nil
}
