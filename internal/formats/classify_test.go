package formats

import (
	"io/fs"
	"strings"
	"testing"
	"time"
)

// fakeFile implements just enough of fs.File for IsPOVIni callers. Not used
// here directly — IsPOVIni takes io.Reader, so tests pass strings.NewReader.

func TestClassifyFile_Extensions(t *testing.T) {
	cases := []struct {
		path string
		want AssetType
	}{
		{"foo.vpx", AssetTypeVPX},
		{"FOO.VPX", AssetTypeVPX}, // case-insensitive
		{"backglass.directb2s", AssetTypeBackglass},
		{"save.nv", AssetTypeNVRAM},
		{"archive.zip", AssetTypeArchive},
		{"archive.7z", AssetTypeArchive},
		{"archive.rar", AssetTypeArchive},
		{"readme.txt", AssetTypeUnknown},
		{"noext", AssetTypeUnknown},
	}
	for _, tc := range cases {
		t.Run(tc.path, func(t *testing.T) {
			got, err := ClassifyFile(tc.path, nil)
			if err != nil {
				t.Fatalf("unexpected err: %v", err)
			}
			if got != tc.want {
				t.Fatalf("ClassifyFile(%q) = %v, want %v", tc.path, got, tc.want)
			}
		})
	}
}

func TestIsPOVIni(t *testing.T) {
	cases := []struct {
		name string
		body string
		want bool
	}{
		{"TableOverride header", "[TableOverride]\nFOV=10\n", true},
		{"Player header", "[Player]\nVolume=80\n", true},
		{"both headers", "[Player]\n[TableOverride]\n", true},
		{"no header", "key=value\nfoo=bar\n", false},
		{"empty", "", false},
		{"short non-POV", "x", false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := IsPOVIni(strings.NewReader(tc.body))
			if err != nil {
				t.Fatalf("unexpected err: %v", err)
			}
			if got != tc.want {
				t.Fatalf("IsPOVIni(%q) = %v, want %v", tc.body, got, tc.want)
			}
		})
	}
}

func TestClassifyFile_INI_RoutesThroughIsPOVIni(t *testing.T) {
	// Use a strings.Reader wrapped to match fs.File minimally — but ClassifyFile
	// only calls IsPOVIni(f), and IsPOVIni only needs io.Reader. We construct
	// a minimal fs.File adapter.
	pov := newReaderFile("[TableOverride]\nFOV=10\n")
	got, err := ClassifyFile("foo.ini", pov)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if got != AssetTypePOV {
		t.Fatalf("POV ini: got %v want AssetTypePOV", got)
	}

	notPov := newReaderFile("key=value\n")
	got, err = ClassifyFile("foo.ini", notPov)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if got != AssetTypeUnknown {
		t.Fatalf("non-POV ini: got %v want AssetTypeUnknown", got)
	}
}

func TestIsROMZip(t *testing.T) {
	cases := []struct {
		name  string
		names []string
		want  bool
	}{
		{"empty list", []string{}, false},
		{"flat lowercase pass", []string{"mm_109c.bin", "mm_109c.u06", "mm_109c.snd"}, true},
		{"flat with dot dir", []string{"mm.cpu", "mm.snd"}, true},
		{"has directory entry", []string{"mm/", "mm/foo.bin"}, false},
		{"has uppercase in basename", []string{"MM_109c.bin"}, false},
		{"has space in basename", []string{"mm 109c.bin"}, false},
		{"has VPX extension", []string{"mm.vpx", "mm.bin"}, false},
		{"has png", []string{"mm.bin", "mm.png"}, false},
		{"txt is allowed", []string{"mm.bin", "readme.txt"}, true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := IsROMZip(tc.names); got != tc.want {
				t.Fatalf("IsROMZip(%v) = %v, want %v", tc.names, got, tc.want)
			}
		})
	}
}

func TestClassifyDirectory(t *testing.T) {
	cases := []struct {
		name    string
		dirName string
		members []string
		want    AssetType
	}{
		{"UltraDMD case-sensitive match", "Twilight Zone.UltraDMD", []string{"dmd.fseq"}, AssetTypeDMD},
		{"UltraDMD lowercase rejected", "tz.ultradmd", []string{"dmd.fseq"}, AssetTypeUnknown},
		{"FlexDMD case-sensitive match", "AFM.FlexDMD", []string{"video.mp4"}, AssetTypeFlexDMD},
		{"PUP wins over music", "PuPPack", []string{"screens.pup", "video.mp3"}, AssetTypePUP},
		{"Audio altsound.csv", "altsound", []string{"altsound.csv", "sfx.ogg"}, AssetTypeAudio},
		{"Audio g-sound.csv", "altsound", []string{"g-sound.csv", "sfx.ogg"}, AssetTypeAudio},
		{"Altcolor cRZ", "color", []string{"foo.crz"}, AssetTypeAltcolor},
		{"Altcolor vni+pal both required", "color", []string{"foo.vni", "foo.pal"}, AssetTypeAltcolor},
		{"Altcolor vni alone is not", "color", []string{"foo.vni"}, AssetTypeUnknown},
		{"Altcolor pac", "color", []string{"foo.pac"}, AssetTypeAltcolor},
		{"Music audio + no screens.pup", "music", []string{"track1.mp3", "track2.flac"}, AssetTypeMusic},
		{"Unknown dissolves", "misc", []string{"readme.txt"}, AssetTypeUnknown},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := ClassifyDirectory(tc.dirName, tc.members)
			if got != tc.want {
				t.Fatalf("ClassifyDirectory(%q, %v) = %v, want %v", tc.dirName, tc.members, got, tc.want)
			}
		})
	}
}

func TestClassifyMembers_BundleAndDissolved(t *testing.T) {
	members := []string{
		"PuPPack/screens.pup",
		"PuPPack/video.mp4",
		"misc/readme.txt",
		"misc/notes.md",
		"loose.vpx",
	}
	groups := ClassifyMembers(members)

	var bundleCount, dissolvedCount, looseCount int
	for _, g := range groups {
		if g.IsBundle && g.Type == AssetTypePUP {
			bundleCount++
			if len(g.Members) != 2 {
				t.Fatalf("PUP bundle expected 2 members, got %d (%v)", len(g.Members), g.Members)
			}
		} else if !g.IsBundle && g.Type == AssetTypeUnknown {
			// Could be dissolved-from-misc or loose
			if strings.HasPrefix(g.Representative, "misc/") {
				dissolvedCount++
			} else {
				looseCount++
			}
		}
	}
	if bundleCount != 1 {
		t.Fatalf("expected 1 PUP bundle, got %d", bundleCount)
	}
	if dissolvedCount != 2 {
		t.Fatalf("expected 2 dissolved misc files, got %d", dissolvedCount)
	}
	if looseCount != 1 {
		t.Fatalf("expected 1 loose file, got %d", looseCount)
	}
}

func TestClassifyMembers_BundleMembersNotIndividuallyClassified_CLF11(t *testing.T) {
	// .ini inside a PuP pack must NOT come back as a separate POV/Unknown group.
	members := []string{
		"PuPPack/screens.pup",
		"PuPPack/triggers.pup.ini",
		"PuPPack/video.mp4",
	}
	groups := ClassifyMembers(members)
	if len(groups) != 1 {
		t.Fatalf("expected exactly 1 bundle group, got %d (%+v)", len(groups), groups)
	}
	if !groups[0].IsBundle || groups[0].Type != AssetTypePUP {
		t.Fatalf("expected PUP bundle, got IsBundle=%v Type=%v", groups[0].IsBundle, groups[0].Type)
	}
	if len(groups[0].Members) != 3 {
		t.Fatalf("expected all 3 members in bundle, got %d", len(groups[0].Members))
	}
}

// readerFile is a minimal fs.File adapter backed by a strings.Reader.
// Used to pass string content through ClassifyFile for .ini tests.
type readerFile struct {
	r    *strings.Reader
	name string
}

func newReaderFile(body string) *readerFile {
	return &readerFile{r: strings.NewReader(body), name: "test.ini"}
}

func (rf *readerFile) Read(p []byte) (int, error) { return rf.r.Read(p) }
func (rf *readerFile) Close() error               { return nil }
func (rf *readerFile) Stat() (fs.FileInfo, error) {
	return readerFileInfo{name: rf.name, size: int64(rf.r.Len())}, nil
}

type readerFileInfo struct {
	name string
	size int64
}

func (i readerFileInfo) Name() string       { return i.name }
func (i readerFileInfo) Size() int64        { return i.size }
func (i readerFileInfo) Mode() fs.FileMode  { return 0644 }
func (i readerFileInfo) ModTime() time.Time { return time.Time{} }
func (i readerFileInfo) IsDir() bool        { return false }
func (i readerFileInfo) Sys() any           { return nil }
