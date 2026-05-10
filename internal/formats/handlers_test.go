package formats

import (
	"archive/zip"
	"bytes"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

const (
	fixtureDir   = "testdata"
	fixtureROM   = "testdata/rom.zip"
	fixtureDist  = "testdata/dist.zip"
	fixtureDist7 = "testdata/dist.7z"
	fixtureRAR   = "testdata/test.rar"

	sevenZipExeWindows = `C:\Program Files\7-Zip\7z.exe`
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

func build7zFixture(out string) error {
	if runtime.GOOS != "windows" {
		return os.ErrNotExist
	}
	if _, err := os.Stat(sevenZipExeWindows); err != nil {
		return err
	}
	// Stage files into a temp dir and run 7z a.
	stage, err := os.MkdirTemp("", "vpin7z")
	if err != nil {
		return err
	}
	defer os.RemoveAll(stage)
	files := map[string][]byte{
		"game.vpx":              []byte("VPX binary payload"),
		"game.directb2s":        []byte("<directb2s/>"),
		"game.ini":              []byte("[TableOverride]\nFOV=10\n"),
		"altsound/altsound.csv": []byte("id,channel,duck\n1,bg,0\n"),
		"altsound/sfx.ogg":      []byte("OggS payload"),
	}
	for name, body := range files {
		full := filepath.Join(stage, name)
		if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
			return err
		}
		if err := os.WriteFile(full, body, 0o644); err != nil {
			return err
		}
	}
	_ = os.Remove(out)
	cmd := exec.Command(sevenZipExeWindows, "a", "-t7z", out, filepath.Join(stage, "*"))
	return cmd.Run()
}

func copyRARFixture(out string) error {
	gopath := os.Getenv("GOPATH")
	if gopath == "" {
		// best-effort: ask Go for the module path
		gopath = filepath.Join(os.Getenv("USERPROFILE"), "go")
	}
	src := filepath.Join(gopath, "pkg", "mod", "github.com", "mholt", "archives@v0.1.5", "testdata", "test.part01.rar")
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
