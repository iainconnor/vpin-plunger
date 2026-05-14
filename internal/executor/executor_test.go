// executor_test.go — smoke test suite for ExecutePlan.
// White-box tests (package executor) exercise the full execution path:
// file moves, archive extraction, vault moves, nested archive extraction,
// review/ignored routing, REVIEW.md creation, failure tolerance, and the
// FailedTwice path. Uses a synthetic temp fixture directory — no real VPX content.
package executor

import (
	"archive/zip"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/iainconnor/vpin-plunger/internal/planner"
)

// ---------------------------------------------------------------------------
// Package-level test state (populated by TestMain)
// ---------------------------------------------------------------------------

var (
	fixtureDownloadsDir string
	fixtureInstallRoot  string
	fixtureCfg          *Config
)

// ---------------------------------------------------------------------------
// TestMain
// ---------------------------------------------------------------------------

func TestMain(m *testing.M) {
	// Create temp downloads dir
	var err error
	fixtureDownloadsDir, err = os.MkdirTemp("", "executor-fixture-*")
	if err != nil {
		fmt.Fprintln(os.Stderr, "TestMain: create downloads dir:", err)
		os.Exit(1)
	}

	// Create synthetic fixture files
	if err := buildFixtureDownloads(fixtureDownloadsDir); err != nil {
		fmt.Fprintln(os.Stderr, "TestMain: buildFixtureDownloads:", err)
		os.RemoveAll(fixtureDownloadsDir)
		os.Exit(1)
	}

	// Create temp install dirs for all Config fields
	fixtureInstallRoot, err = os.MkdirTemp("", "executor-install-*")
	if err != nil {
		fmt.Fprintln(os.Stderr, "TestMain: create install root:", err)
		os.RemoveAll(fixtureDownloadsDir)
		os.Exit(1)
	}

	fixtureCfg = buildFixtureConfig(fixtureInstallRoot)

	code := m.Run()
	os.RemoveAll(fixtureDownloadsDir)
	os.RemoveAll(fixtureInstallRoot)
	os.Exit(code)
}

// ---------------------------------------------------------------------------
// Fixture builders
// ---------------------------------------------------------------------------

// buildFixtureDownloads creates the minimal synthetic downloads directory
// content needed to exercise each action type.
func buildFixtureDownloads(dir string) error {
	// Empty .vpx file
	if err := os.WriteFile(filepath.Join(dir, "Medieval Madness (Williams, 1997).vpx"), []byte(""), 0o644); err != nil {
		return err
	}
	// Empty .directb2s file
	if err := os.WriteFile(filepath.Join(dir, "Funhouse (Williams, 1990).directb2s"), []byte(""), 0o644); err != nil {
		return err
	}
	// Small .zip archive containing a .vpx member (for EXE-01 test)
	zipPath := filepath.Join(dir, "Theatre of Magic (Bally, 1995).zip")
	if err := createZIPFixture(zipPath, []string{"Theatre of Magic (Bally, 1995).vpx"}); err != nil {
		return err
	}
	return nil
}

// buildFixtureConfig creates a Config pointing all paths under root.
// Each directory is created immediately so that the executor can write to it.
func buildFixtureConfig(root string) *Config {
	mkdir := func(name string) string {
		p := filepath.Join(root, name)
		_ = os.MkdirAll(p, 0o755)
		return p
	}
	return &Config{
		VPXDir:          mkdir("Tables"),
		BackglassDir:    mkdir("DirectB2S"),
		ROMDir:          mkdir("ROMs"),
		NVRAMDir:        mkdir("NVRAM"),
		POVDir:          mkdir("POV"),
		DMDDir:          mkdir("UltraDMD"),
		FlexDMDDir:      mkdir("FlexDMD"),
		AudioDir:        mkdir("Altsound"),
		AltcolorDir:     mkdir("Altcolor"),
		MusicDir:        mkdir("Music"),
		PuPDir:          mkdir("PuPPacksVideosMusicExtras"),
		ArchiveVaultDir: mkdir("archive_vault"),
		ReviewDir:       mkdir("review"),
		IgnoredDir:      mkdir("ignored"),
		RehearsalDir:    mkdir("rehearsal"),
		DBPath:          "", // not used in filesystem-only tests
		BackupDir:       "",
	}
}

// createZIPFixture creates a valid ZIP archive at path containing the given member names.
// Member files are empty. Copied verbatim from planner_test.go pattern.
func createZIPFixture(path string, members []string) error {
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()
	w := zip.NewWriter(f)
	defer w.Close()
	for _, m := range members {
		fw, err := w.Create(m)
		if err != nil {
			return err
		}
		_ = fw // empty member
	}
	return nil
}

// ---------------------------------------------------------------------------
// Test helpers
// ---------------------------------------------------------------------------

// outcomesWithStatus filters result.Outcomes by the given status.
func outcomesWithStatus(result *ExecuteResult, s OutcomeStatus) []ActionOutcome {
	var out []ActionOutcome
	for _, o := range result.Outcomes {
		if o.Status == s {
			out = append(out, o)
		}
	}
	return out
}

// ---------------------------------------------------------------------------
// Smoke tests
// ---------------------------------------------------------------------------

// TestExecutePlan_MoveVPX verifies that a MOVE_VPX action moves the source
// file to the intended destination and increments result.Moved (EXE-02).
func TestExecutePlan_MoveVPX(t *testing.T) {
	// Copy fixture to a per-test temp file so tests are independent
	src := filepath.Join(t.TempDir(), "Test Table (Williams, 1980).vpx")
	if err := os.WriteFile(src, []byte(""), 0o644); err != nil {
		t.Fatal(err)
	}
	dst := filepath.Join(fixtureCfg.VPXDir, "Test Table (Williams, 1980).vpx")

	plan := &planner.ProcessPlan{
		DownloadsDir: fixtureDownloadsDir,
		Actions: []*planner.PlannedAction{
			{
				Type:   planner.ActionTypeMoveVPX,
				Source: src,
				Dest:   dst,
				Reason: "auto-assigned",
				Mtime:  time.Now(),
			},
		},
	}

	result, err := ExecutePlan(plan, fixtureCfg, nil)
	if err != nil {
		t.Fatalf("ExecutePlan: %v", err)
	}
	if result.Moved != 1 {
		t.Errorf("Moved = %d, want 1", result.Moved)
	}
	if _, statErr := os.Stat(dst); statErr != nil {
		t.Errorf("dest file not found: %v", statErr)
	}
	if _, statErr := os.Stat(src); !os.IsNotExist(statErr) {
		t.Errorf("source file should have been moved (not exist): %v", statErr)
	}
}

// TestExecutePlan_ReviewIgnored verifies that SEND_TO_REVIEW and IGNORE actions
// route files to ReviewDir and IgnoredDir respectively, and that REVIEW.md
// contains the ignored item's source path (EXE-03, D-07).
func TestExecutePlan_ReviewIgnored(t *testing.T) {
	reviewSrc := filepath.Join(t.TempDir(), "Unknown Game.vpx")
	ignoreSrc := filepath.Join(t.TempDir(), "Junk File.bin")
	_ = os.WriteFile(reviewSrc, []byte(""), 0o644)
	_ = os.WriteFile(ignoreSrc, []byte(""), 0o644)

	// Use a per-test review/ignored dir to avoid cross-test interference
	testRoot := t.TempDir()
	cfg := buildFixtureConfig(testRoot)

	reviewDest := filepath.Join(cfg.ReviewDir, "Unknown Game.vpx")
	ignoreDest := filepath.Join(cfg.IgnoredDir, "Junk File.bin")

	plan := &planner.ProcessPlan{
		Actions: []*planner.PlannedAction{
			{Type: planner.ActionTypeSendToReview, Source: reviewSrc, Dest: reviewDest, Reason: "low confidence"},
			{Type: planner.ActionTypeIgnore, Source: ignoreSrc, Dest: ignoreDest, Reason: "user ignored"},
		},
	}

	result, err := ExecutePlan(plan, cfg, nil)
	if err != nil {
		t.Fatalf("ExecutePlan: %v", err)
	}
	if result.Reviewed != 1 {
		t.Errorf("Reviewed = %d, want 1", result.Reviewed)
	}
	if result.Ignored != 1 {
		t.Errorf("Ignored = %d, want 1", result.Ignored)
	}
	if _, statErr := os.Stat(filepath.Join(cfg.ReviewDir, "Unknown Game.vpx")); statErr != nil {
		t.Errorf("review file not in ReviewDir: %v", statErr)
	}
	if _, statErr := os.Stat(filepath.Join(cfg.IgnoredDir, "Junk File.bin")); statErr != nil {
		t.Errorf("ignored file not in IgnoredDir: %v", statErr)
	}

	// D-07: REVIEW.md must contain the ignored item's source path
	reviewMD := filepath.Join(cfg.ReviewDir, "REVIEW.md")
	content, err := os.ReadFile(reviewMD)
	if err != nil {
		t.Fatalf("REVIEW.md not created after IGNORE action: %v", err)
	}
	if !strings.Contains(string(content), ignoreSrc) {
		t.Errorf("REVIEW.md does not contain ignored item source path %q; content:\n%s", ignoreSrc, string(content))
	}
}

// TestExecutePlan_FailureTolerance verifies that a single-action failure does not
// abort remaining actions, and that result.Failed > 0 with at least one Failed or
// FailedTwice outcome (EXE-09, D-05, D-06).
func TestExecutePlan_FailureTolerance(t *testing.T) {
	testRoot := t.TempDir()
	cfg := buildFixtureConfig(testRoot)

	// Action 1: will succeed
	succSrc := filepath.Join(t.TempDir(), "Good Table (Williams, 1985).vpx")
	_ = os.WriteFile(succSrc, []byte(""), 0o644)
	succDst := filepath.Join(cfg.VPXDir, "Good Table (Williams, 1985).vpx")

	// Action 2: will fail — use file-as-directory-parent pattern for reliable failure
	// on both Windows and Linux. os.MkdirAll(filepath.Dir(failDst)) tries to create a
	// directory where a regular file exists — fails with ENOTDIR/syscall.ENOTDIR.
	failSrc := filepath.Join(t.TempDir(), "Bad Table (Williams, 1997).vpx")
	_ = os.WriteFile(failSrc, []byte(""), 0o644)
	failParent := filepath.Join(cfg.VPXDir, "blocking_file")
	if err := os.WriteFile(failParent, []byte{}, 0o644); err != nil {
		t.Fatal(err)
	}
	failDst := filepath.Join(failParent, "Bad Table (Williams, 1997).vpx")

	plan := &planner.ProcessPlan{
		Actions: []*planner.PlannedAction{
			{Type: planner.ActionTypeMoveVPX, Source: succSrc, Dest: succDst, Reason: "good"},
			{Type: planner.ActionTypeMoveVPX, Source: failSrc, Dest: failDst, Reason: "will fail"},
		},
	}

	result, err := ExecutePlan(plan, cfg, nil)
	if err != nil {
		t.Fatalf("ExecutePlan returned error, want nil (errors are per-action): %v", err)
	}

	// Execution must continue — both outcomes present
	if len(result.Outcomes) < 2 {
		t.Fatalf("expected at least 2 outcomes, got %d", len(result.Outcomes))
	}

	// First action succeeded
	succeeded := outcomesWithStatus(result, OutcomeSucceeded)
	if len(succeeded) == 0 {
		t.Error("expected at least 1 succeeded outcome")
	}

	// Second action failed (primary) — moved to review/ or left in place (FailedTwice)
	// Either OutcomeFailed (file in review/) or OutcomeFailedTwice (file left in place)
	failures := outcomesWithStatus(result, OutcomeFailed)
	failedTwice := outcomesWithStatus(result, OutcomeFailedTwice)
	if len(failures)+len(failedTwice) == 0 {
		t.Error("expected at least 1 failed or failed_twice outcome for the bad action")
	}
	if result.Failed == 0 {
		t.Error("result.Failed should be > 0")
	}

	// Verify successful file moved to VPXDir
	if _, statErr := os.Stat(succDst); statErr != nil {
		t.Errorf("succeeded action dest not found: %v", statErr)
	}
}

// TestExecutePlan_REVIEWmd verifies that REVIEW.md is created in ReviewDir
// and contains the source filename when a SEND_TO_REVIEW action is executed (EXE-08).
func TestExecutePlan_REVIEWmd(t *testing.T) {
	testRoot := t.TempDir()
	cfg := buildFixtureConfig(testRoot)

	src := filepath.Join(t.TempDir(), "Weird Game.vpx")
	_ = os.WriteFile(src, []byte(""), 0o644)

	plan := &planner.ProcessPlan{
		Actions: []*planner.PlannedAction{
			{
				Type:   planner.ActionTypeSendToReview,
				Source: src,
				Dest:   filepath.Join(cfg.ReviewDir, "Weird Game.vpx"),
				Reason: "below confidence threshold",
			},
		},
	}

	_, err := ExecutePlan(plan, cfg, nil)
	if err != nil {
		t.Fatalf("ExecutePlan: %v", err)
	}

	reviewMD := filepath.Join(cfg.ReviewDir, "REVIEW.md")
	content, err := os.ReadFile(reviewMD)
	if err != nil {
		t.Fatalf("REVIEW.md not created: %v", err)
	}
	if !strings.Contains(string(content), "Weird Game.vpx") {
		t.Errorf("REVIEW.md does not contain source filename; content:\n%s", string(content))
	}
	if !strings.Contains(string(content), "## Run ") {
		t.Errorf("REVIEW.md missing '## Run' section header; content:\n%s", string(content))
	}
}

// TestExecutePlan_REVIEWmd_NoWrite verifies that REVIEW.md is NOT created
// when no review items are present (EXE-08 negative path).
func TestExecutePlan_REVIEWmd_NoWrite(t *testing.T) {
	testRoot := t.TempDir()
	cfg := buildFixtureConfig(testRoot)

	src := filepath.Join(t.TempDir(), "Clean Table.vpx")
	_ = os.WriteFile(src, []byte(""), 0o644)
	dst := filepath.Join(cfg.VPXDir, "Clean Table.vpx")

	plan := &planner.ProcessPlan{
		Actions: []*planner.PlannedAction{
			{Type: planner.ActionTypeMoveVPX, Source: src, Dest: dst},
		},
	}

	_, err := ExecutePlan(plan, cfg, nil)
	if err != nil {
		t.Fatalf("ExecutePlan: %v", err)
	}

	reviewMD := filepath.Join(cfg.ReviewDir, "REVIEW.md")
	if _, statErr := os.Stat(reviewMD); !os.IsNotExist(statErr) {
		t.Error("REVIEW.md should NOT exist when no review items present")
	}
}

// TestExecutePlan_ExtractArchive verifies that an EXTRACT_ARCHIVE action extracts
// the archive and the VAULT_ARCHIVE child moves the original to ArchiveVaultDir (EXE-01).
func TestExecutePlan_ExtractArchive(t *testing.T) {
	testRoot := t.TempDir()
	cfg := buildFixtureConfig(testRoot)

	// Copy fixture zip to per-test temp dir
	zipSrc := filepath.Join(t.TempDir(), "Theatre of Magic (Bally, 1995).zip")
	if err := createZIPFixture(zipSrc, []string{"Theatre of Magic (Bally, 1995).vpx"}); err != nil {
		t.Fatal(err)
	}

	vaultDest := filepath.Join(cfg.ArchiveVaultDir, "Theatre of Magic (Bally, 1995).zip")

	// executeExtractArchive ignores action.Dest and computes its own extractDir
	// from filepath.Dir(action.Source) + stem. We set Dest to match what it will compute.
	extractDir := filepath.Join(filepath.Dir(zipSrc), "Theatre of Magic (Bally, 1995)")

	plan := &planner.ProcessPlan{
		Actions: []*planner.PlannedAction{
			{
				Type:   planner.ActionTypeExtractArchive,
				Source: zipSrc,
				Dest:   extractDir,
				Children: []*planner.PlannedAction{
					{
						Type:   planner.ActionTypeVaultArchive,
						Source: zipSrc,
						Dest:   vaultDest,
					},
				},
			},
		},
	}

	_, err := ExecutePlan(plan, cfg, nil)
	if err != nil {
		t.Fatalf("ExecutePlan: %v", err)
	}

	// Original zip should be in archive_vault
	if _, statErr := os.Stat(vaultDest); statErr != nil {
		t.Errorf("original zip not in ArchiveVaultDir: %v", statErr)
	}

	// Extraction dir should exist
	entries, err := os.ReadDir(cfg.ArchiveVaultDir)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) == 0 {
		t.Error("ArchiveVaultDir is empty; expected vaulted archive")
	}
}

// TestExecutePlan_NestedArchive verifies that a nested ZIP inside an outer ZIP
// is extracted in-place after the outer ZIP is extracted, and that the inner ZIP
// itself is removed from the extraction directory (EXE-04).
func TestExecutePlan_NestedArchive(t *testing.T) {
	testRoot := t.TempDir()
	cfg := buildFixtureConfig(testRoot)

	workDir := t.TempDir()

	// Create inner.zip containing a single member file
	innerZipPath := filepath.Join(workDir, "inner.zip")
	if err := createZIPFixture(innerZipPath, []string{"inner_member.vpx"}); err != nil {
		t.Fatal("create inner.zip:", err)
	}

	// Create outer.zip containing inner.zip
	// We need to embed the actual bytes of inner.zip into outer.zip
	innerBytes, err := os.ReadFile(innerZipPath)
	if err != nil {
		t.Fatal("read inner.zip:", err)
	}

	outerZipPath := filepath.Join(workDir, "outer.zip")
	outerF, err := os.Create(outerZipPath)
	if err != nil {
		t.Fatal("create outer.zip:", err)
	}
	outerW := zip.NewWriter(outerF)
	innerEntry, err := outerW.Create("inner.zip")
	if err != nil {
		outerF.Close()
		t.Fatal("create inner.zip entry in outer.zip:", err)
	}
	if _, err := innerEntry.Write(innerBytes); err != nil {
		outerF.Close()
		t.Fatal("write inner.zip bytes into outer.zip:", err)
	}
	if err := outerW.Close(); err != nil {
		outerF.Close()
		t.Fatal("close outer zip writer:", err)
	}
	if err := outerF.Close(); err != nil {
		t.Fatal("close outer zip file:", err)
	}
	// Remove the standalone inner.zip — only outer.zip should remain in workDir
	_ = os.Remove(innerZipPath)

	vaultDest := filepath.Join(cfg.ArchiveVaultDir, "outer.zip")
	// executeExtractArchive computes extractDir from filepath.Dir(outerZipPath) + "outer"
	extractDir := filepath.Join(workDir, "outer")

	plan := &planner.ProcessPlan{
		Actions: []*planner.PlannedAction{
			{
				Type:   planner.ActionTypeExtractArchive,
				Source: outerZipPath,
				Dest:   extractDir,
				Children: []*planner.PlannedAction{
					{
						Type:   planner.ActionTypeVaultArchive,
						Source: outerZipPath,
						Dest:   vaultDest,
					},
				},
			},
		},
	}

	_, err = ExecutePlan(plan, cfg, nil)
	if err != nil {
		t.Fatalf("ExecutePlan: %v", err)
	}

	// Outer ZIP must be in vault
	if _, statErr := os.Stat(vaultDest); statErr != nil {
		t.Errorf("outer.zip not in ArchiveVaultDir: %v", statErr)
	}

	// inner.zip must have been extracted: its member inner_member.vpx should exist
	// somewhere inside extractDir, and inner.zip itself must not exist in extractDir
	innerMemberFound := false
	_ = filepath.WalkDir(extractDir, func(p string, d os.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		if d.Name() == "inner_member.vpx" {
			innerMemberFound = true
		}
		return nil
	})
	if !innerMemberFound {
		t.Error("inner_member.vpx not found after nested archive extraction; inner.zip was not recursively extracted")
	}

	// inner.zip itself must have been removed (extracted in-place, not vaulted)
	innerZipInExtractDir := filepath.Join(extractDir, "inner.zip")
	if _, statErr := os.Stat(innerZipInExtractDir); !os.IsNotExist(statErr) {
		t.Error("inner.zip still exists in extractDir after recursive extraction; it should have been removed")
	}
}
