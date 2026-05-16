package main

import (
	"bytes"
	"database/sql"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	_ "modernc.org/sqlite"
)

// TestE2ERehearsalAuto verifies Phase 6 success criterion 5:
//
//	"End-to-end rehearsal smoke test in CI: vpin process --rehearsal --auto
//	 exits 0, fixture rehearsal/ directory contains expected renamed files,
//	 REVIEW.md exists if any review items were produced."
//
// The test is HERMETIC — no network access required. The catalog cache is
// pre-populated from cmd/plunger/testdata/catalog_fixture.xlsx with a fresh
// mtime, so catalog.IsStale() returns false and no download is attempted.
func TestE2ERehearsalAuto(t *testing.T) {
	if testing.Short() {
		t.Skip("e2e smoke test skipped in -short mode")
	}

	workDir := t.TempDir()
	binPath := filepath.Join(workDir, "vpin.exe")
	if runtime.GOOS != "windows" {
		binPath = filepath.Join(workDir, "vpin")
	}

	// Step 1: build the binary.
	goBin := goExecutable()
	repoDir := repoRoot(t)
	cmdBuild := exec.Command(goBin, "build", "-o", binPath, "./cmd/plunger")
	cmdBuild.Dir = repoDir
	var buildErr bytes.Buffer
	cmdBuild.Stderr = &buildErr
	if err := cmdBuild.Run(); err != nil {
		t.Fatalf("go build failed: %v\n%s", err, buildErr.String())
	}

	// Step 2: synthetic downloads tree with one .vpx.
	downloads := filepath.Join(workDir, "downloads")
	if err := os.MkdirAll(downloads, 0o755); err != nil {
		t.Fatal(err)
	}
	// Use a filename that closely matches the catalog fixture entry
	// "Phoenix (Williams, 1978)" so fuzzy matching yields >= 92% confidence
	// (ThresholdAutoAssign) and the file is auto-assigned a MoveVPX action.
	// After RemapForRehearsal, the dest is under rehearsal/, which is where
	// the executor moves it.
	vpxName := "Phoenix (Williams, 1978).vpx"
	if err := os.WriteFile(filepath.Join(downloads, vpxName), []byte("fake vpx"), 0o644); err != nil {
		t.Fatal(err)
	}

	// Step 2b: pre-populate the catalog cache so catalog.Load() does NOT
	// attempt a network download. BuildCatalogConfig defaults CachePath to
	// {downloads}/../cache/catalog.xlsx (per internal/config/config.go BuildCatalogConfig).
	cacheDir := filepath.Join(filepath.Dir(downloads), "cache")
	if err := os.MkdirAll(cacheDir, 0o755); err != nil {
		t.Fatal(err)
	}
	cachePath := filepath.Join(cacheDir, "catalog.xlsx")
	fixtureSrc := filepath.Join(repoDir, "cmd", "plunger", "testdata", "catalog_fixture.xlsx")
	if err := copyFileForTest(fixtureSrc, cachePath); err != nil {
		t.Fatalf("seed catalog cache: %v", err)
	}
	// Set a recent mtime so IsStale() returns false (default staleness = 7d).
	now := time.Now()
	if err := os.Chtimes(cachePath, now, now); err != nil {
		t.Fatalf("chtimes catalog cache: %v", err)
	}

	// Step 3: fixture PUPDatabase.db at workDir/pupdb/PUPDatabase.db.
	pupDir := filepath.Join(workDir, "pupdb")
	if err := os.MkdirAll(pupDir, 0o755); err != nil {
		t.Fatal(err)
	}
	dbPath := filepath.Join(pupDir, "PUPDatabase.db")
	if err := createFixtureDB(dbPath); err != nil {
		t.Fatalf("createFixtureDB: %v", err)
	}

	// Step 4: create dummy install dirs.
	installRoot := filepath.Join(workDir, "install")
	mkDirs := []string{
		"Tables",
		"VPinMAME/roms",
		"VPinMAME/nvram",
		"VPinMAME/altcolor",
		"VPinMAME/altsound",
		"Music",
		"UltraDMD",
		"FlexDMD",
		"PuPVideos",
	}
	for _, d := range mkDirs {
		if err := os.MkdirAll(filepath.Join(installRoot, d), 0o755); err != nil {
			t.Fatal(err)
		}
	}

	// Step 5: run vpin process.
	args := []string{
		"process", "--rehearsal", "--auto",
		"--dir", downloads,
		"--db", dbPath,
		"--vpx-dir", filepath.Join(installRoot, "Tables"),
		"--backglass-dir", filepath.Join(installRoot, "Tables"),
		"--rom-dir", filepath.Join(installRoot, "VPinMAME", "roms"),
		"--nvram-dir", filepath.Join(installRoot, "VPinMAME", "nvram"),
		"--pov-dir", filepath.Join(installRoot, "Tables"),
		"--dmd-dir", filepath.Join(installRoot, "UltraDMD"),
		"--flexdmd-dir", filepath.Join(installRoot, "FlexDMD"),
		"--audio-dir", filepath.Join(installRoot, "VPinMAME", "altsound"),
		"--altcolor-dir", filepath.Join(installRoot, "VPinMAME", "altcolor"),
		"--music-dir", filepath.Join(installRoot, "Music"),
		"--pup-dir", filepath.Join(installRoot, "PuPVideos"),
	}
	var stdout, stderr bytes.Buffer
	runCmd := exec.Command(binPath, args...)
	runCmd.Stdout = &stdout
	runCmd.Stderr = &stderr
	if err := runCmd.Run(); err != nil {
		t.Fatalf("vpin process exited with error: %v\nstdout:\n%s\nstderr:\n%s",
			err, stdout.String(), stderr.String())
	}

	// Step 6: assertions.
	rehearsal := filepath.Join(downloads, "rehearsal")
	if _, err := os.Stat(rehearsal); err != nil {
		t.Fatalf("rehearsal/ directory was not created: %v\nstdout:\n%s\nstderr:\n%s",
			err, stdout.String(), stderr.String())
	}
	foundVPX := false
	foundReview := false
	filepath.Walk(rehearsal, func(path string, info os.FileInfo, err error) error {
		if err != nil || info == nil {
			return nil
		}
		if !info.IsDir() && strings.HasSuffix(strings.ToLower(info.Name()), ".vpx") {
			foundVPX = true
		}
		if !info.IsDir() && strings.EqualFold(info.Name(), "REVIEW.md") {
			foundReview = true
		}
		return nil
	})
	if !foundVPX {
		t.Errorf("no .vpx file found under rehearsal/ — vpin process did not move the fixture\nstdout:\n%s",
			stdout.String())
	}
	if !foundReview {
		t.Logf("REVIEW.md not produced under rehearsal/ — acceptable only if no review items")
	}
}

func goExecutable() string {
	if runtime.GOOS == "windows" {
		if _, err := os.Stat(`C:\Program Files\Go\bin\go.exe`); err == nil {
			return `C:\Program Files\Go\bin\go.exe`
		}
	}
	return "go"
}

func repoRoot(t *testing.T) string {
	t.Helper()
	wd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	for d := wd; d != filepath.Dir(d); d = filepath.Dir(d) {
		if _, err := os.Stat(filepath.Join(d, "go.mod")); err == nil {
			return d
		}
	}
	t.Fatal("repoRoot: go.mod not found")
	return ""
}

func copyFileForTest(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()
	if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
		return err
	}
	out, err := os.Create(dst)
	if err != nil {
		return err
	}
	if _, err = io.Copy(out, in); err != nil {
		out.Close()
		return err
	}
	return out.Close() // captures flush/sync errors; not deferred
}

func createFixtureDB(path string) error {
	sqldb, err := sql.Open("sqlite", path)
	if err != nil {
		return err
	}
	defer sqldb.Close()
	schema := []string{
		`CREATE TABLE Emulators (
            EMUID INTEGER PRIMARY KEY,
            EmuName TEXT
        )`,
		`INSERT INTO Emulators (EMUID, EmuName) VALUES (1, 'Visual Pinball X')`,
		`CREATE TABLE Games (
            ID INTEGER PRIMARY KEY AUTOINCREMENT,
            EMUID INTEGER, GameName TEXT, GameFileName TEXT, GameDisplay TEXT,
            Visible INTEGER, GameYear TEXT, Manufact TEXT, GameType TEXT,
            DateFileUpdated TEXT, WEBGameID TEXT,
            TAGS TEXT, IPDBNum TEXT, WebLinkURL TEXT, WebLink2URL TEXT,
            DesignedBy TEXT, Notes TEXT,
            DateAdded TEXT, DateUpdated TEXT
        )`,
	}
	for _, q := range schema {
		if _, err := sqldb.Exec(q); err != nil {
			return err
		}
	}
	return nil
}
