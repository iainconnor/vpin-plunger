// scan.go — two-pass scan algorithm for BuildPlan.
//
// Pass 1 (bundlePrePass): classifies direct-child directories of downloadsDir;
// claims recognised bundle directories so their members are not re-classified
// in Pass 2.
//
// Pass 2 (buildScanActions Walk): recursive Walk over all remaining files.
// Archives produce EXTRACT_ARCHIVE trees with VAULT_ARCHIVE + member actions.
// ROM zips produce MOVE_ROM. Loose files produce a single PlannedAction.
//
// NOTE: formats.Walk delivers paths that are ALWAYS relative to downloadsDir
// (dot-rooted fs.WalkDir paths, e.g. "file.vpx" or "subdir/file.vpx", not
// "/abs/path/file.vpx"). buildScanActions constructs absolute paths via
// filepath.Join(downloadsDir, path) before any filesystem operation.
package planner

import (
	"context"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/iainconnor/vpin-plunger/internal/formats"
)

// listRelative returns a flat list of relative file paths under dir.
// Only files are included; directory entries are not. Uses forward slashes
// for ClassifyDirectory (consistent with archive member paths).
func listRelative(dir string) ([]string, error) {
	var result []string
	err := filepath.WalkDir(dir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return nil // skip unreadable entries
		}
		if d.IsDir() {
			return nil
		}
		rel, err := filepath.Rel(dir, path)
		if err != nil {
			return nil
		}
		// Use forward slashes for ClassifyDirectory (consistent with archive member paths)
		result = append(result, filepath.ToSlash(rel))
		return nil
	})
	return result, err
}

// dirMtime returns the mtime of a directory (falls back to zero time on error).
func dirMtime(path string) (time.Time, error) {
	info, err := os.Stat(path)
	if err != nil {
		return time.Time{}, err
	}
	return info.ModTime(), nil
}

// assetTypeToMoveAction maps a formats.AssetType to the corresponding
// ActionType for a MOVE_* action. Returns ActionTypeUnknown for types
// that are not directly moved (archive, unknown).
func assetTypeToMoveAction(t formats.AssetType) ActionType {
	switch t {
	case formats.AssetTypeVPX:
		return ActionTypeMoveVPX
	case formats.AssetTypeBackglass:
		return ActionTypeMoveBackglass
	case formats.AssetTypeROM:
		return ActionTypeMoveROM
	case formats.AssetTypeNVRAM:
		return ActionTypeMoveNVRAM
	case formats.AssetTypePOV:
		return ActionTypeMovePOV
	case formats.AssetTypeDMD:
		return ActionTypeMoveUltraDMD
	case formats.AssetTypeFlexDMD:
		return ActionTypeMoveFlexDMD
	case formats.AssetTypeAudio:
		return ActionTypeMoveAudio
	case formats.AssetTypeAltcolor:
		return ActionTypeMoveAltcolor
	case formats.AssetTypeMusic:
		return ActionTypeMoveMusic
	case formats.AssetTypePUP:
		return ActionTypeMovePUP
	default:
		return ActionTypeUnknown
	}
}

// destForAssetType returns the absolute destination path for an asset given
// the Config, the asset type, and the asset's base name.
// For file assets: filepath.Join(dir, name).
// For directory assets: filepath.Join(dir, name) where dir is the bundle dir.
func destForAssetType(cfg *Config, t formats.AssetType, name string) string {
	switch t {
	case formats.AssetTypeVPX:
		return filepath.Join(cfg.VPXDir, name)
	case formats.AssetTypeBackglass:
		return filepath.Join(cfg.BackglassDir, name)
	case formats.AssetTypeROM:
		return filepath.Join(cfg.ROMDir, name)
	case formats.AssetTypeNVRAM:
		return filepath.Join(cfg.NVRAMDir, name)
	case formats.AssetTypePOV:
		return filepath.Join(cfg.POVDir, name)
	case formats.AssetTypeDMD:
		return filepath.Join(cfg.DMDDir, name)
	case formats.AssetTypeFlexDMD:
		return filepath.Join(cfg.FlexDMDDir, name)
	case formats.AssetTypeAudio:
		return filepath.Join(cfg.AudioDir, name)
	case formats.AssetTypeAltcolor:
		return filepath.Join(cfg.AltcolorDir, name)
	case formats.AssetTypeMusic:
		return filepath.Join(cfg.MusicDir, name)
	case formats.AssetTypePUP:
		return filepath.Join(cfg.PuPDir, name)
	default:
		return ""
	}
}

// bundlePrePass scans direct children of downloadsDir for recognised bundle
// directories (PuP packs, Altcolor, Altsound, Music, UltraDMD, FlexDMD).
// Each recognised bundle becomes a single PlannedAction appended to plan.Actions.
// The directory path is added to claimedDirs so Pass 2 can skip it.
//
// PLN-01: bundle members must not be re-classified as individual files.
func bundlePrePass(downloadsDir string, cfg *Config, plan *ProcessPlan, claimedDirs map[string]struct{}) error {
	entries, err := os.ReadDir(downloadsDir)
	if err != nil {
		return err
	}
	for _, d := range entries {
		if !d.IsDir() {
			continue
		}
		dirPath := filepath.Join(downloadsDir, d.Name())
		members, err := listRelative(dirPath)
		if err != nil {
			continue // unreadable dir — skip silently
		}
		assetType := formats.ClassifyDirectory(d.Name(), members)
		if assetType == formats.AssetTypeUnknown {
			continue // not a recognised bundle
		}
		actionType := assetTypeToMoveAction(assetType)
		if actionType == ActionTypeUnknown {
			continue
		}
		dest := destForAssetType(cfg, assetType, d.Name())
		mtime, _ := dirMtime(dirPath)
		action := &PlannedAction{
			Type:   actionType,
			Source: dirPath,
			Dest:   dest,
			Reason: "bundle directory classified in pre-pass",
			Mtime:  mtime,
		}
		plan.Actions = append(plan.Actions, action)
		claimedDirs[dirPath] = struct{}{}
	}
	return nil
}

// isInsideClaimed returns true if walkPath is the same as or inside any path in claimedDirs.
func isInsideClaimed(walkPath string, claimedDirs map[string]struct{}) bool {
	for claimed := range claimedDirs {
		// filepath.Rel returns a path without ".." if walkPath is inside claimed
		rel, err := filepath.Rel(claimed, walkPath)
		if err == nil && !strings.HasPrefix(rel, "..") {
			return true
		}
	}
	return false
}

// peekArchive calls the appropriate handler's Peek for .zip, .7z, or .rar.
// Returns the member list or an error.
func peekArchive(path string) ([]string, error) {
	lower := strings.ToLower(path)
	switch {
	case strings.HasSuffix(lower, ".zip"):
		return formats.ZIPHandler{}.Peek(path)
	case strings.HasSuffix(lower, ".7z"):
		return formats.SevenZipHandler{}.Peek(path)
	case strings.HasSuffix(lower, ".rar"):
		return formats.RARHandler{}.Peek(path)
	default:
		return nil, nil
	}
}

// isArchiveExt returns true if the file has a .zip, .7z, or .rar extension.
func isArchiveExt(path string) bool {
	lower := strings.ToLower(path)
	return strings.HasSuffix(lower, ".zip") ||
		strings.HasSuffix(lower, ".7z") ||
		strings.HasSuffix(lower, ".rar")
}

// buildArchiveAction builds the EXTRACT_ARCHIVE PlannedAction tree for a
// distribution archive. Children are: [VAULT_ARCHIVE, ...member actions].
// At plan time no matching is performed — matchable actions (MOVE_VPX etc.)
// carry Match=nil; the match pass (04-03) fills these in.
//
// archivePath: absolute path to the .zip/.7z/.rar file.
// mtime: the archive file's mtime, propagated to all member PlannedActions.
// members: the member list from Peek.
func buildArchiveAction(archivePath string, mtime time.Time, members []string, cfg *Config) *PlannedAction {
	archiveName := filepath.Base(archivePath)

	vaultAction := &PlannedAction{
		Type:   ActionTypeVaultArchive,
		Source: archivePath,
		Dest:   filepath.Join(cfg.ArchiveVaultDir, archiveName),
		Reason: "original archive moved to vault after extraction",
		Mtime:  mtime,
	}

	archiveAction := &PlannedAction{
		Type:     ActionTypeExtractArchive,
		Source:   archivePath,
		Mtime:    mtime,
		Children: []*PlannedAction{vaultAction},
	}

	groups := formats.ClassifyMembers(members)
	for _, g := range groups {
		memberActions := planMember(g, archiveName, archivePath, mtime, cfg)
		archiveAction.Children = append(archiveAction.Children, memberActions...)
	}

	return archiveAction
}

// planMember converts a formats.MemberGroup from a distribution archive into
// one or more PlannedActions. The archiveName is the basename of the parent
// archive (e.g. "Flash (Williams 1979).zip") used to construct VirtualPath.
//
// IMPORTANT: Pass nil as fs.File to ClassifyFile for archive members —
// the file does not exist on disk at plan time; content inspection is skipped.
// (RESEARCH Pitfall 1)
//
// Mtime is propagated from the parent archive (RESEARCH Pitfall 2).
func planMember(g formats.MemberGroup, archiveName, archivePath string, mtime time.Time, cfg *Config) []*PlannedAction {
	if g.IsBundle {
		// Bundle directories inside an archive: e.g. an altcolor dir
		// Representative is the directory name (with trailing slash)
		at := g.Type
		actionType := assetTypeToMoveAction(at)
		if actionType == ActionTypeUnknown {
			// Unrecognised bundle — send to review
			return []*PlannedAction{{
				Type:        ActionTypeSendToReview,
				Source:      filepath.Join(archivePath, g.Representative),
				VirtualPath: archiveName + "/" + g.Representative,
				Reason:      "unrecognised bundle type inside archive",
				Mtime:       mtime,
				Dest:        filepath.Join(cfg.ReviewDir, filepath.Base(strings.TrimSuffix(g.Representative, "/"))),
			}}
		}
		bundleName := filepath.Base(strings.TrimSuffix(g.Representative, "/"))
		return []*PlannedAction{{
			Type:        actionType,
			Source:      filepath.Join(archivePath, strings.TrimSuffix(g.Representative, "/")),
			Dest:        destForAssetType(cfg, at, bundleName),
			VirtualPath: archiveName + "/" + g.Representative,
			Reason:      "bundle directory member of archive",
			Mtime:       mtime,
		}}
	}

	// Individual files (g.IsBundle == false)
	var actions []*PlannedAction
	for _, memberPath := range g.Members {
		// Skip directory entries that appear in the member list
		if strings.HasSuffix(memberPath, "/") {
			continue
		}
		virtualPath := archiveName + "/" + memberPath
		memberBase := filepath.Base(memberPath)

		// Nested archives inside distribution archives: stub as EXTRACT_ARCHIVE
		// with no children. Content resolved at execution time (Pitfall 5).
		if isArchiveExt(memberPath) {
			actions = append(actions, &PlannedAction{
				Type:        ActionTypeExtractArchive,
				Source:      filepath.Join(archivePath, memberPath),
				VirtualPath: virtualPath,
				Reason:      "nested archive — content resolved at execution time",
				Mtime:       mtime,
			})
			continue
		}

		// Classify on extension only — pass nil for fs.File (Pitfall 1)
		assetType, err := formats.ClassifyFile(memberPath, nil)
		if err != nil || assetType == formats.AssetTypeUnknown {
			actions = append(actions, &PlannedAction{
				Type:        ActionTypeSendToReview,
				Source:      filepath.Join(archivePath, memberPath),
				Dest:        filepath.Join(cfg.ReviewDir, memberBase),
				VirtualPath: virtualPath,
				Reason:      "unclassified archive member",
				Mtime:       mtime,
			})
			continue
		}

		actionType := assetTypeToMoveAction(assetType)
		dest := destForAssetType(cfg, assetType, memberBase)

		memberAction := &PlannedAction{
			Type:        actionType,
			Source:      filepath.Join(archivePath, memberPath),
			Dest:        dest,
			VirtualPath: virtualPath,
			Reason:      "archive member classified by extension",
			Mtime:       mtime,
		}

		// MOVE_VPX: create paired REGISTER_GAME and set back-pointer (PLN-03)
		if actionType == ActionTypeMoveVPX {
			regGame := &PlannedAction{
				Type:   ActionTypeRegisterGame,
				Source: dest, // destination of the VPX becomes source for registration
				Reason: "paired with MOVE_VPX",
				Mtime:  mtime,
			}
			memberAction.RegisterGame = regGame
			actions = append(actions, memberAction, regGame)
			continue
		}

		actions = append(actions, memberAction)
	}
	return actions
}

// buildScanActions executes the two-pass scan and populates plan.Actions.
// Pass 1: bundle pre-pass (direct-child directories).
// Pass 2: recursive Walk over all remaining files.
//
// This function does NOT perform catalog matching. All matchable actions
// (MOVE_VPX, MOVE_BACKGLASS, MOVE_POV) carry Match=nil and use raw filenames
// as destination placeholders. The match pass in 04-03 fills in Match and
// rewrites Dest to the canonical filename.
//
// NOTE on path handling: formats.Walk delivers paths that are ALWAYS relative
// to downloadsDir (dot-rooted fs.WalkDir paths, e.g. "file.vpx" not
// "/abs/path/file.vpx"). buildScanActions constructs absolute paths via
// filepath.Join(downloadsDir, path) before any filesystem operation.
//
// Called by BuildPlan after Catalog has been loaded.
func buildScanActions(ctx context.Context, downloadsDir string, cfg *Config, plan *ProcessPlan) error {
	claimedDirs := make(map[string]struct{})

	// Pass 1: bundle pre-pass
	if err := bundlePrePass(downloadsDir, cfg, plan, claimedDirs); err != nil {
		return err
	}

	// Pass 2: recursive Walk
	// path argument from Walk is always relative to downloadsDir (dot-rooted).
	walkErr := formats.Walk(ctx, downloadsDir, func(path string, d fs.DirEntry, f fs.File) error {
		// Construct absolute path from the dot-rooted relative path Walk delivers.
		absPath := filepath.Join(downloadsDir, path)

		if d.IsDir() {
			if absPath == downloadsDir {
				return nil // root dir — continue
			}
			// If this directory was claimed in Pass 1, skip its contents entirely (Pitfall 3)
			if _, claimed := claimedDirs[absPath]; claimed {
				return fs.SkipDir
			}
			return nil
		}

		// Skip files inside claimed dirs (handles files at depth > 1 inside bundle)
		if isInsideClaimed(filepath.Dir(absPath), claimedDirs) {
			return nil
		}

		// Get file mtime
		var mtime time.Time
		if info, err := d.Info(); err == nil {
			mtime = info.ModTime()
		}

		// Handle archive files
		if isArchiveExt(absPath) {
			members, err := peekArchive(absPath)
			if err != nil {
				// Unpeekable archive — send to review
				plan.Actions = append(plan.Actions, &PlannedAction{
					Type:   ActionTypeSendToReview,
					Source: absPath,
					Dest:   filepath.Join(cfg.ReviewDir, filepath.Base(absPath)),
					Reason: "archive peek failed: " + err.Error(),
					Mtime:  mtime,
				})
				return nil
			}

			// ROM zip? (only .zip can be ROM; .7z and .rar are always distribution archives)
			if strings.HasSuffix(strings.ToLower(absPath), ".zip") && formats.IsROMZip(members) {
				plan.Actions = append(plan.Actions, &PlannedAction{
					Type:   ActionTypeMoveROM,
					Source: absPath,
					Dest:   filepath.Join(cfg.ROMDir, filepath.Base(absPath)),
					Reason: "ROM zip (chip extensions only, no VPX asset extensions)",
					Mtime:  mtime,
				})
				return nil
			}

			// Distribution archive: build EXTRACT_ARCHIVE tree
			archiveAction := buildArchiveAction(absPath, mtime, members, cfg)
			plan.Actions = append(plan.Actions, archiveAction)
			return nil
		}

		// Non-archive file: classify with content inspection if fs.File is available
		var assetType formats.AssetType
		var classErr error
		if f != nil {
			assetType, classErr = formats.ClassifyFile(absPath, f)
		} else {
			assetType, classErr = formats.ClassifyFile(absPath, nil)
		}

		if classErr != nil || assetType == formats.AssetTypeUnknown {
			plan.Actions = append(plan.Actions, &PlannedAction{
				Type:   ActionTypeSendToReview,
				Source: absPath,
				Dest:   filepath.Join(cfg.ReviewDir, filepath.Base(absPath)),
				Reason: "unclassified loose file",
				Mtime:  mtime,
			})
			return nil
		}

		actionType := assetTypeToMoveAction(assetType)
		action := &PlannedAction{
			Type:   actionType,
			Source: absPath,
			Dest:   destForAssetType(cfg, assetType, filepath.Base(absPath)),
			Reason: "loose file classified by extension",
			Mtime:  mtime,
		}

		if actionType == ActionTypeMoveVPX {
			regGame := &PlannedAction{
				Type:   ActionTypeRegisterGame,
				Source: action.Dest,
				Reason: "paired with MOVE_VPX",
				Mtime:  mtime,
			}
			action.RegisterGame = regGame
			plan.Actions = append(plan.Actions, action, regGame)
			return nil
		}

		plan.Actions = append(plan.Actions, action)
		return nil
	})

	return walkErr
}
