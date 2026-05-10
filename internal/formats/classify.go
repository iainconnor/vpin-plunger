// classify.go — pure classification logic for the formats package.
// No archive I/O: IsROMZip operates on a pre-computed member list produced by
// ZIPHandler.Peek (Plan 02). IsPOVIni reads from a caller-supplied io.Reader
// (the caller opens the file or archive member stream).
package formats

import (
	"errors"
	"io"
	"io/fs"
	"path"
	"path/filepath"
	"regexp"
	"strings"
)

// MemberGroup is the output unit of ClassifyMembers. Bundle groups
// (IsBundle == true) represent one logical asset spanning multiple archive
// members. Dissolved and loose groups have Type == AssetTypeUnknown; callers
// are expected to call ClassifyFile on Representative for those.
type MemberGroup struct {
	Representative string   // canonical path: "dir/" for bundles, full file path for singles
	Members        []string // all archive paths belonging to this group
	Type           AssetType
	IsBundle       bool
}

// povPeekBytes is the number of bytes IsPOVIni reads from the head of a stream
// to scan for the [TableOverride] or [Player] sentinel. 4096 is sufficient per
// RESEARCH.md Pitfall 5.
const povPeekBytes = 4096

var (
	povSentinels = []string{"[TableOverride]", "[Player]"}

	// romExcludedExts (D-01): if any zip member has one of these extensions,
	// the zip is NOT a ROM archive.
	romExcludedExts = map[string]bool{
		".vpx": true, ".directb2s": true, ".ini": true,
		".mp3": true, ".ogg": true, ".wav": true, ".flac": true,
		".png": true, ".jpg": true, ".jpeg": true,
	}

	// romFilenameRe (D-01/D-02): every zip member basename must match.
	romFilenameRe = regexp.MustCompile(`^[a-z0-9._-]+$`)

	musicAudioExts = map[string]bool{
		".mp3": true, ".ogg": true, ".wav": true, ".flac": true,
	}
)

// ClassifyFile returns the AssetType for a single file. For .ini files, f is
// read (up to povPeekBytes) to inspect for the POV sentinels. For .zip, .7z,
// .rar files, this function returns AssetTypeArchive — caller must invoke
// IsROMZip(ZIPHandler.Peek(path)) separately to refine .zip to AssetTypeROM.
// Pass f == nil to skip content inspection (returns AssetTypeUnknown for .ini).
func ClassifyFile(p string, f fs.File) (AssetType, error) {
	ext := strings.ToLower(filepath.Ext(p))
	switch ext {
	case ".vpx":
		return AssetTypeVPX, nil
	case ".directb2s":
		return AssetTypeBackglass, nil
	case ".nv":
		return AssetTypeNVRAM, nil
	case ".ini":
		if f == nil {
			return AssetTypeUnknown, nil
		}
		ok, err := IsPOVIni(f)
		if err != nil {
			return AssetTypeUnknown, err
		}
		if ok {
			return AssetTypePOV, nil
		}
		return AssetTypeUnknown, nil
	case ".zip", ".7z", ".rar":
		return AssetTypeArchive, nil
	}
	return AssetTypeUnknown, nil
}

// IsPOVIni reads up to povPeekBytes from r and returns true iff the bytes
// contain "[TableOverride]" or "[Player]". Tolerates io.EOF and
// io.ErrUnexpectedEOF for short files.
func IsPOVIni(r io.Reader) (bool, error) {
	buf := make([]byte, povPeekBytes)
	n, err := io.ReadFull(r, buf)
	if err != nil && !errors.Is(err, io.ErrUnexpectedEOF) && !errors.Is(err, io.EOF) {
		return false, err
	}
	content := string(buf[:n])
	for _, s := range povSentinels {
		if strings.Contains(content, s) {
			return true, nil
		}
	}
	return false, nil
}

// IsROMZip applies the structural/negative ROM rule from D-01:
// (1) names is non-empty AND
// (2) no name ends with "/" (no directory entries) AND
// (3) every path.Base(name) matches [a-z0-9._-]+ AND
// (4) no member's lowercased extension is in romExcludedExts.
func IsROMZip(names []string) bool {
	if len(names) == 0 {
		return false
	}
	for _, name := range names {
		if strings.HasSuffix(name, "/") {
			return false
		}
		base := path.Base(name)
		if !romFilenameRe.MatchString(base) {
			return false
		}
		if romExcludedExts[strings.ToLower(path.Ext(base))] {
			return false
		}
	}
	return true
}

// ClassifyDirectory returns the AssetType for a directory given its name and
// the relative paths of its members. Precedence (highest first):
// UltraDMD suffix > FlexDMD suffix > PUP (screens.pup) > Audio
// (altsound.csv|g-sound.csv) > Altcolor (.cRZ | .vni+.pal | .pac) > Music
// (audio exts present, no screens.pup) > Unknown.
func ClassifyDirectory(name string, members []string) AssetType {
	// Suffix rules (case-sensitive per Pitfall 7)
	if strings.HasSuffix(name, ".UltraDMD") {
		return AssetTypeDMD
	}
	if strings.HasSuffix(name, ".FlexDMD") {
		return AssetTypeFlexDMD
	}

	// Build root-level basename set and a lowercased-extension set across all members.
	rootBasenames := map[string]bool{}
	extPresent := map[string]bool{}
	var hasScreensPup bool
	for _, m := range members {
		// Skip directory-entry sentinels.
		if strings.HasSuffix(m, "/") {
			continue
		}
		base := path.Base(m)
		// Treat as root member if no "/" in m (i.e., directly under name)
		if !strings.Contains(m, "/") {
			rootBasenames[base] = true
		}
		if base == "screens.pup" {
			hasScreensPup = true
		}
		extPresent[strings.ToLower(path.Ext(base))] = true
	}

	if hasScreensPup {
		return AssetTypePUP
	}
	if rootBasenames["altsound.csv"] || rootBasenames["g-sound.csv"] {
		return AssetTypeAudio
	}
	// Altcolor: .cRZ present (case-sensitive in extension map? rule says ".cRZ" — check both forms)
	// RESEARCH.md Pattern: extensions are checked case-insensitively for portability.
	if extPresent[".crz"] || extPresent[".pac"] || (extPresent[".vni"] && extPresent[".pal"]) {
		return AssetTypeAltcolor
	}
	for ext := range musicAudioExts {
		if extPresent[ext] {
			return AssetTypeMusic
		}
	}
	return AssetTypeUnknown
}

// ClassifyMembers groups archive members. Per CLF-10/CLF-11, recognised
// directories become bundles (one MemberGroup per directory, IsBundle=true);
// unrecognised directories dissolve into per-file groups; loose files (no
// directory component) become per-file groups. Dissolved and loose groups
// carry Type=AssetTypeUnknown — callers must call ClassifyFile on the
// Representative path.
func ClassifyMembers(members []string) []MemberGroup {
	dirGroups := map[string][]string{} // dirName -> relative member paths
	var loose []string

	for _, m := range members {
		if strings.HasSuffix(m, "/") {
			continue
		}
		parts := strings.SplitN(m, "/", 2)
		if len(parts) == 2 {
			dirGroups[parts[0]] = append(dirGroups[parts[0]], parts[1])
		} else {
			loose = append(loose, m)
		}
	}

	var groups []MemberGroup

	// Iterate dirGroups in stable order (sort dir names) for deterministic output.
	dirNames := make([]string, 0, len(dirGroups))
	for d := range dirGroups {
		dirNames = append(dirNames, d)
	}
	sortStrings(dirNames)

	for _, dirName := range dirNames {
		relMembers := dirGroups[dirName]
		t := ClassifyDirectory(dirName, relMembers)
		if t != AssetTypeUnknown {
			full := make([]string, len(relMembers))
			for i, r := range relMembers {
				full[i] = dirName + "/" + r
			}
			groups = append(groups, MemberGroup{
				Representative: dirName + "/",
				Members:        full,
				Type:           t,
				IsBundle:       true,
			})
		} else {
			for _, r := range relMembers {
				full := dirName + "/" + r
				groups = append(groups, MemberGroup{
					Representative: full,
					Members:        []string{full},
					Type:           AssetTypeUnknown,
					IsBundle:       false,
				})
			}
		}
	}

	for _, lp := range loose {
		groups = append(groups, MemberGroup{
			Representative: lp,
			Members:        []string{lp},
			Type:           AssetTypeUnknown,
			IsBundle:       false,
		})
	}

	return groups
}

// sortStrings is a tiny dep-free sort helper (avoid importing "sort" only here).
func sortStrings(a []string) {
	for i := 1; i < len(a); i++ {
		for j := i; j > 0 && a[j-1] > a[j]; j-- {
			a[j-1], a[j] = a[j], a[j-1]
		}
	}
}
