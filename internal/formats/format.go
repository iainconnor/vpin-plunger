// Package formats contains all asset detection and format handling logic.
// Zero dependency on the TUI or UI layer.
// Archive traversal via github.com/mholt/archives exclusively.
package formats

import (
	"context"
	"io/fs"
)

// Format is the interface all asset handlers must implement.
// Full interface contract defined during GSD milestone 2 per
// MIGRATION-BRIEF.md Section 9.
type Format interface {
	Detect(ctx context.Context, path string, f fs.File) bool
	Name() string
}

// AssetType identifies what kind of pinball asset a file or directory represents.
// Zero value is AssetTypeUnknown — unclassified or sent to review.
// Used by all downstream packages; do not add TUI imports to this file.
type AssetType int

const (
	AssetTypeUnknown   AssetType = iota // zero value — unclassified or sent to review
	AssetTypeVPX                        // .vpx table file
	AssetTypeBackglass                  // .directb2s backglass
	AssetTypeROM                        // flat zip of VPinMAME ROM chips
	AssetTypeNVRAM                      // .nv NVRAM save file
	AssetTypePOV                        // .ini with [TableOverride] or [Player]
	AssetTypeDMD                        // directory ending in .UltraDMD (case-sensitive)
	AssetTypeFlexDMD                    // directory ending in .FlexDMD (case-sensitive)
	AssetTypeAudio                      // Altsound directory (altsound.csv or g-sound.csv)
	AssetTypeAltcolor                   // Altcolor directory (.cRZ or .vni+.pal or .pac)
	AssetTypeMusic                      // Music directory (audio files, no screens.pup)
	AssetTypePUP                        // PuP pack directory (screens.pup at root)
	AssetTypeArchive                    // distribution archive (ZIP/7z/RAR not classified as ROM)
)

// Handler extends Format with archive-specific capabilities.
// Implementations live in handlers.go (Plan 02). Peek returns the flat member
// name list without extracting; member paths use '/' as separator regardless
// of platform. Extract writes all archive members to dest with zip-slip
// protection (any member path that resolves outside dest is rejected).
type Handler interface {
	Detect(ctx context.Context, path string, f fs.File) bool
	Name() string
	Peek(path string) ([]string, error)
	Extract(src, dest string) error
}

