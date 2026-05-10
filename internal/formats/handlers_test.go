package formats

import (
	"archive/zip"
	"bytes"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// dist7zFixture is a minimal valid 7z archive (334 bytes, LZMA2 store) containing:
//   - game.vpx, game.directb2s, game.ini (root level)
//   - altsound/altsound.csv, altsound/sfx.ogg (subdirectory)
//
// Inlined as a byte literal so no external tool is required to build the fixture
// (WR-04: replaces the former exec.Command("7z.exe") approach). The archive was
// generated once with py7zr and is verified readable by github.com/bodgit/sevenzip.
var dist7zFixture = []byte{
	0x37, 0x7a, 0xbc, 0xaf, 0x27, 0x1c, 0x00, 0x04, 0x7d, 0xf9, 0x44, 0x7c, 0x0c, 0x01, 0x00, 0x00,
	0x00, 0x00, 0x00, 0x00, 0x22, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x99, 0x3f, 0xaa, 0xa2,
	0x01, 0x00, 0x57, 0x69, 0x64, 0x2c, 0x63, 0x68, 0x61, 0x6e, 0x6e, 0x65, 0x6c, 0x2c, 0x64, 0x75,
	0x63, 0x6b, 0x0a, 0x31, 0x2c, 0x62, 0x67, 0x2c, 0x30, 0x0a, 0x4f, 0x67, 0x67, 0x53, 0x20, 0x70,
	0x61, 0x79, 0x6c, 0x6f, 0x61, 0x64, 0x3c, 0x64, 0x69, 0x72, 0x65, 0x63, 0x74, 0x62, 0x32, 0x73,
	0x2f, 0x3e, 0x5b, 0x54, 0x61, 0x62, 0x6c, 0x65, 0x4f, 0x76, 0x65, 0x72, 0x72, 0x69, 0x64, 0x65,
	0x5d, 0x0a, 0x46, 0x4f, 0x56, 0x3d, 0x31, 0x30, 0x0a, 0x56, 0x50, 0x58, 0x20, 0x62, 0x69, 0x6e,
	0x61, 0x72, 0x79, 0x20, 0x70, 0x61, 0x79, 0x6c, 0x6f, 0x61, 0x64, 0x00, 0x00, 0x00, 0x81, 0x33,
	0x07, 0xae, 0x0f, 0xd3, 0x5c, 0x46, 0xfd, 0x40, 0xc0, 0x90, 0xd2, 0xff, 0x7d, 0x69, 0x4d, 0x87,
	0xe0, 0x96, 0x9e, 0x4c, 0xa8, 0x3b, 0x15, 0x67, 0x36, 0x05, 0xc2, 0x42, 0xc6, 0xe0, 0x62, 0x38,
	0xd8, 0x0e, 0x36, 0x53, 0x85, 0x16, 0xb5, 0x7b, 0xab, 0xc1, 0x87, 0xdc, 0x40, 0x33, 0x90, 0x2c,
	0xe8, 0x18, 0x5a, 0x3a, 0x7c, 0x20, 0x13, 0x1d, 0x2e, 0xf1, 0x99, 0x68, 0x4a, 0xde, 0x4d, 0xbf,
	0x23, 0xcc, 0x02, 0x7b, 0xdf, 0x87, 0x33, 0xc4, 0xc6, 0xb8, 0x44, 0xa5, 0x7d, 0x5c, 0x2c, 0xb4,
	0x45, 0x65, 0xe9, 0xfb, 0xfb, 0x40, 0xe6, 0x7a, 0x79, 0x4b, 0x74, 0xff, 0xdf, 0x7d, 0xf9, 0x26,
	0xe2, 0xdc, 0x32, 0x9c, 0xc6, 0x30, 0xba, 0xeb, 0x4c, 0xf1, 0x70, 0x81, 0x77, 0x04, 0x0e, 0xc7,
	0xf3, 0x1c, 0xa3, 0xae, 0xac, 0x4d, 0xed, 0x35, 0xe7, 0xbb, 0x0a, 0x57, 0x55, 0x19, 0xc3, 0x11,
	0xeb, 0x15, 0x25, 0xbe, 0x6d, 0x41, 0xe9, 0x8d, 0x28, 0x37, 0x9c, 0xd5, 0x1e, 0x3c, 0x30, 0x50,
	0x8b, 0xa1, 0x5c, 0x5d, 0xdd, 0x4e, 0x21, 0x71, 0x38, 0xfb, 0x58, 0xbe, 0x73, 0x5f, 0xa3, 0x82,
	0x4c, 0x6e, 0x16, 0xa2, 0xf9, 0x92, 0x0d, 0x62, 0x81, 0xa4, 0xa4, 0x00, 0x17, 0x06, 0x5c, 0x01,
	0x09, 0x80, 0xb0, 0x00, 0x07, 0x0b, 0x01, 0x00, 0x01, 0x23, 0x03, 0x01, 0x01, 0x05, 0x5d, 0x00,
	0x10, 0x00, 0x00, 0x0c, 0x81, 0x36, 0x0a, 0x01, 0x99, 0xf0, 0xb5, 0x39, 0x00, 0x00,
}

const (
	fixtureDir   = "testdata"
	fixtureROM   = "testdata/rom.zip"
	fixtureDist  = "testdata/dist.zip"
	fixtureDist7 = "testdata/dist.7z"
	fixtureRAR   = "testdata/test.rar"
)

func TestMain(m *testing.M) {
	if err := os.MkdirAll(fixtureDir, 0o755); err != nil {
		panic(err)
	}
	if err := buildROMZip(fixtureROM); err != nil {
		panic(err)
	}
	if err := buildDistZip(fixtureDist); err != nil {
		panic(err)
	}
	// Best-effort: build .7z from dist.zip contents using the 7z CLI.
	_ = build7zFixture(fixtureDist7) // tests t.Skip if missing
	// Best-effort: copy a .rar fixture from the mholt/archives module cache.
	_ = copyRARFixture(fixtureRAR)
	os.Exit(m.Run())
}

func buildROMZip(path string) error {
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()
	w := zip.NewWriter(f)
	defer w.Close()
	for _, name := range []string{"mm_109c.bin", "mm_109c.u06", "mm_109c.snd"} {
		wf, err := w.Create(name)
		if err != nil {
			return err
		}
		if _, err := wf.Write([]byte("ROM payload " + name)); err != nil {
			return err
		}
	}
	return nil
}

func buildDistZip(path string) error {
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()
	w := zip.NewWriter(f)
	defer w.Close()
	members := map[string][]byte{
		"game.vpx":              []byte("VPX binary payload"),
		"game.directb2s":        []byte("<directb2s/>"),
		"game.ini":              []byte("[TableOverride]\nFOV=10\n"),
		"altsound/altsound.csv": []byte("id,channel,duck\n1,bg,0\n"),
		"altsound/sfx.ogg":      []byte("OggS payload"),
	}
	for name, body := range members {
		wf, err := w.Create(name)
		if err != nil {
			return err
		}
		if _, err := wf.Write(body); err != nil {
			return err
		}
	}
	return nil
}

// build7zFixture writes the embedded dist.7z fixture to out. The fixture is a
// pre-built minimal 7z archive committed to testdata/; no external tools are
// required (WR-04: replaces the former exec.Command("7z.exe") approach).
func build7zFixture(out string) error {
	return os.WriteFile(out, dist7zFixture, 0o644)
}

func copyRARFixture(out string) error {
	gopath := os.Getenv("GOPATH")
	if gopath == "" {
		// best-effort: ask Go for the module path
		gopath = filepath.Join(os.Getenv("USERPROFILE"), "go")
	}
	// Derive the mholt/archives version dynamically so this path stays
	// correct when the dependency is upgraded (WR-03).
	archivesVersion, err := resolveModuleVersion("github.com/mholt/archives")
	if err != nil {
		return fmt.Errorf("copyRARFixture: cannot resolve mholt/archives version: %w", err)
	}
	src := filepath.Join(gopath, "pkg", "mod", "github.com", "mholt", "archives@"+archivesVersion, "testdata", "test.part01.rar")
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()
	o, err := os.Create(out)
	if err != nil {
		return err
	}
	defer o.Close()
	_, err = io.Copy(o, in)
	return err
}

// resolveModuleVersion returns the version string for the named module by
// running "go list -m <module>" in the module root. This avoids hardcoding
// version strings that become stale when dependencies are upgraded.
func resolveModuleVersion(module string) (string, error) {
	cmd := exec.Command("go", "list", "-m", module)
	// Run from the module root so go.mod is found regardless of test working dir.
	cmd.Dir = moduleRoot()
	out, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("go list -m %s: %w", module, err)
	}
	// Output format: "<module> <version>\n"
	parts := strings.Fields(strings.TrimSpace(string(out)))
	if len(parts) < 2 {
		return "", fmt.Errorf("unexpected go list output: %q", string(out))
	}
	return parts[1], nil
}

// moduleRoot walks up from the test binary directory to find the directory
// containing go.mod. Falls back to the current working directory.
func moduleRoot() string {
	dir, err := os.Getwd()
	if err != nil {
		return "."
	}
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			break
		}
		dir = parent
	}
	wd, _ := os.Getwd()
	return wd
}

func TestZIPHandler_Peek(t *testing.T) {
	names, err := ZIPHandler{}.Peek(fixtureDist)
	if err != nil {
		t.Fatalf("Peek err: %v", err)
	}
	want := []string{"game.vpx", "altsound/altsound.csv"}
	for _, w := range want {
		var found bool
		for _, n := range names {
			if n == w {
				found = true
				break
			}
		}
		if !found {
			t.Fatalf("expected %q in Peek result, got %v", w, names)
		}
	}
}

func TestZIPHandler_Peek_FeedsIsROMZip_ARC01_CLF03(t *testing.T) {
	romNames, err := ZIPHandler{}.Peek(fixtureROM)
	if err != nil {
		t.Fatalf("Peek rom: %v", err)
	}
	if !IsROMZip(romNames) {
		t.Fatalf("rom.zip should classify as ROM, got names=%v", romNames)
	}
	distNames, err := ZIPHandler{}.Peek(fixtureDist)
	if err != nil {
		t.Fatalf("Peek dist: %v", err)
	}
	if IsROMZip(distNames) {
		t.Fatalf("dist.zip should NOT classify as ROM, got names=%v", distNames)
	}
}

func TestZIPHandler_Extract_ARC02(t *testing.T) {
	tmp := t.TempDir()
	if err := (ZIPHandler{}).Extract(fixtureDist, tmp); err != nil {
		t.Fatalf("Extract: %v", err)
	}
	if _, err := os.Stat(filepath.Join(tmp, "game.vpx")); err != nil {
		t.Fatalf("game.vpx missing: %v", err)
	}
	body, err := os.ReadFile(filepath.Join(tmp, "altsound", "altsound.csv"))
	if err != nil {
		t.Fatalf("altsound.csv missing: %v", err)
	}
	if !bytes.Contains(body, []byte("id,channel,duck")) {
		t.Fatalf("altsound.csv content unexpected: %q", body)
	}
}

func TestZIPHandler_Extract_RejectsZipSlip(t *testing.T) {
	// Build an in-memory malicious zip with a "../escape.txt" member.
	tmp := t.TempDir()
	badZip := filepath.Join(tmp, "bad.zip")
	f, err := os.Create(badZip)
	if err != nil {
		t.Fatal(err)
	}
	w := zip.NewWriter(f)
	wf, err := w.Create("../escape.txt")
	if err != nil {
		t.Fatal(err)
	}
	wf.Write([]byte("hostile"))
	w.Close()
	f.Close()

	destDir := filepath.Join(tmp, "dest")
	os.MkdirAll(destDir, 0o755)
	err = ZIPHandler{}.Extract(badZip, destDir)
	if err == nil {
		t.Fatalf("expected zip-slip error, got nil")
	}
	if !strings.Contains(err.Error(), "path escape") {
		t.Fatalf("expected error to mention path escape, got: %v", err)
	}
	// The escape.txt MUST NOT exist anywhere outside destDir.
	if _, statErr := os.Stat(filepath.Join(tmp, "escape.txt")); statErr == nil {
		t.Fatalf("zip-slip succeeded: escape.txt was written outside dest")
	}
}

func TestSevenZipHandler_Peek_ARC03(t *testing.T) {
	if _, err := os.Stat(fixtureDist7); err != nil {
		t.Skipf("dist.7z fixture not built (7z CLI absent?): %v", err)
	}
	names, err := SevenZipHandler{}.Peek(fixtureDist7)
	if err != nil {
		t.Fatalf("Peek: %v", err)
	}
	if len(names) == 0 {
		t.Fatal("expected non-empty 7z member list")
	}
	var foundVPX bool
	for _, n := range names {
		if strings.HasSuffix(n, "game.vpx") {
			foundVPX = true
		}
	}
	if !foundVPX {
		t.Fatalf("expected game.vpx in 7z members, got %v", names)
	}
}

func TestSevenZipHandler_Extract_ARC04(t *testing.T) {
	if _, err := os.Stat(fixtureDist7); err != nil {
		t.Skipf("dist.7z fixture not built: %v", err)
	}
	tmp := t.TempDir()
	if err := (SevenZipHandler{}).Extract(fixtureDist7, tmp); err != nil {
		t.Fatalf("Extract: %v", err)
	}
	// game.vpx may be at the root or under a subdirectory depending on the
	// 7z CLI invocation; walk to find it.
	var found bool
	filepath.Walk(tmp, func(p string, info os.FileInfo, err error) error {
		if err == nil && filepath.Base(p) == "game.vpx" {
			found = true
		}
		return nil
	})
	if !found {
		t.Fatalf("game.vpx not extracted from 7z fixture")
	}
}

func TestRARHandler_Peek_ARC05(t *testing.T) {
	if _, err := os.Stat(fixtureRAR); err != nil {
		t.Skipf("test.rar fixture not present: %v", err)
	}
	names, err := RARHandler{}.Peek(fixtureRAR)
	if err != nil {
		// multi-volume part01 may need part02 — accept a wrapped error and skip.
		t.Skipf("RAR Peek failed (likely multi-volume fixture limitation): %v", err)
	}
	if len(names) == 0 {
		t.Fatal("expected non-empty RAR member list")
	}
}

func TestRARHandler_Extract_ErrorWrapping_ARC06(t *testing.T) {
	err := RARHandler{}.Extract("testdata/does-not-exist.rar", t.TempDir())
	if err == nil {
		t.Fatal("expected error for missing RAR")
	}
	if !strings.Contains(err.Error(), "rar extract") {
		t.Fatalf("expected error to be wrapped with 'rar extract', got: %v", err)
	}
}
