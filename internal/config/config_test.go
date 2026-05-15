package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestBuildPlannerConfig_Defaults(t *testing.T) {
	tmp := t.TempDir()
	f := Flags{DownloadsDir: tmp}
	cfg, err := BuildPlannerConfig(f)
	if err != nil {
		t.Fatalf("BuildPlannerConfig: %v", err)
	}
	if !strings.HasSuffix(cfg.VPXDir, filepath.Join("visualpinball", "Tables")) {
		t.Errorf("VPXDir = %q; want suffix visualpinball\\Tables", cfg.VPXDir)
	}
	if !strings.HasSuffix(cfg.ROMDir, filepath.Join("VPinMAME", "roms")) {
		t.Errorf("ROMDir = %q; want suffix VPinMAME\\roms", cfg.ROMDir)
	}
	if cfg.ArchiveVaultDir != filepath.Join(tmp, "archive_vault") {
		t.Errorf("ArchiveVaultDir = %q; want %q", cfg.ArchiveVaultDir, filepath.Join(tmp, "archive_vault"))
	}
	if cfg.ReviewDir != filepath.Join(tmp, "review") {
		t.Errorf("ReviewDir mismatch")
	}
	if cfg.IgnoredDir != filepath.Join(tmp, "ignored") {
		t.Errorf("IgnoredDir mismatch")
	}
	if cfg.RehearsalDir != filepath.Join(tmp, "rehearsal") {
		t.Errorf("RehearsalDir mismatch")
	}
	if cfg.YearWindow != 3 {
		t.Errorf("YearWindow = %d; want 3", cfg.YearWindow)
	}
}

func TestBuildPlannerConfig_FlagOverride(t *testing.T) {
	tmp := t.TempDir()
	custom := filepath.Join(tmp, "my-tables")
	f := Flags{DownloadsDir: tmp, VPXDir: custom}
	cfg, err := BuildPlannerConfig(f)
	if err != nil {
		t.Fatalf("BuildPlannerConfig: %v", err)
	}
	if cfg.VPXDir != custom {
		t.Errorf("VPXDir = %q; want %q (override)", cfg.VPXDir, custom)
	}
}

func TestBuildPlannerConfig_MissingDownloadsDir(t *testing.T) {
	_, err := BuildPlannerConfig(Flags{})
	if err == nil {
		t.Fatalf("expected error for empty DownloadsDir, got nil")
	}
}

func TestBuildExecutorConfig_RehearsalDBPath(t *testing.T) {
	tmp := t.TempDir()
	rehDB := filepath.Join(tmp, "rehearsal", "PUPDatabase.db")
	cfg, err := BuildExecutorConfig(Flags{DownloadsDir: tmp}, rehDB)
	if err != nil {
		t.Fatalf("BuildExecutorConfig: %v", err)
	}
	if cfg.DBPath != rehDB {
		t.Errorf("DBPath = %q; want rehearsal copy %q", cfg.DBPath, rehDB)
	}
	if !strings.HasSuffix(cfg.BackupDir, "backup") {
		t.Errorf("BackupDir = %q; want suffix 'backup'", cfg.BackupDir)
	}
}

func TestBuildCatalogConfig_DefaultURL(t *testing.T) {
	tmp := t.TempDir()
	cfg, err := BuildCatalogConfig(Flags{DownloadsDir: tmp})
	if err != nil {
		t.Fatalf("BuildCatalogConfig: %v", err)
	}
	if !strings.Contains(cfg.SheetURL, "docs.google.com/spreadsheets") {
		t.Errorf("SheetURL = %q; want Google Sheets export URL", cfg.SheetURL)
	}
	if cfg.Staleness == 0 {
		t.Errorf("Staleness must default to non-zero")
	}
}

func TestValidatePaths_MissingVPXDir(t *testing.T) {
	tmp := t.TempDir()
	cfg, err := BuildPlannerConfig(Flags{
		DownloadsDir: tmp,
		VPXDir:       filepath.Join(tmp, "does-not-exist"),
	})
	if err != nil {
		t.Fatalf("BuildPlannerConfig: %v", err)
	}
	// Create all OTHER required dirs so we isolate the VPXDir failure.
	for _, d := range []string{cfg.BackglassDir, cfg.ROMDir, cfg.NVRAMDir, cfg.POVDir,
		cfg.DMDDir, cfg.FlexDMDDir, cfg.AudioDir, cfg.AltcolorDir, cfg.MusicDir, cfg.PuPDir} {
		_ = os.MkdirAll(d, 0o755)
	}
	err = ValidatePaths(cfg)
	if err == nil {
		t.Fatalf("expected missing-path error, got nil")
	}
	if !strings.Contains(err.Error(), "VPXDir") {
		t.Errorf("error must name the missing path; got %v", err)
	}
}

func TestValidatePaths_AllExist(t *testing.T) {
	tmp := t.TempDir()
	cfg, err := BuildPlannerConfig(Flags{DownloadsDir: tmp})
	if err != nil {
		t.Fatalf("BuildPlannerConfig: %v", err)
	}
	// Override all paths to live under tmp and create them.
	cfg.VPXDir = filepath.Join(tmp, "vpx")
	cfg.BackglassDir = filepath.Join(tmp, "bg")
	cfg.ROMDir = filepath.Join(tmp, "rom")
	cfg.NVRAMDir = filepath.Join(tmp, "nvram")
	cfg.POVDir = filepath.Join(tmp, "pov")
	cfg.DMDDir = filepath.Join(tmp, "dmd")
	cfg.FlexDMDDir = filepath.Join(tmp, "flex")
	cfg.AudioDir = filepath.Join(tmp, "audio")
	cfg.AltcolorDir = filepath.Join(tmp, "altc")
	cfg.MusicDir = filepath.Join(tmp, "music")
	cfg.PuPDir = filepath.Join(tmp, "pup")
	for _, d := range []string{cfg.VPXDir, cfg.BackglassDir, cfg.ROMDir, cfg.NVRAMDir,
		cfg.POVDir, cfg.DMDDir, cfg.FlexDMDDir, cfg.AudioDir, cfg.AltcolorDir,
		cfg.MusicDir, cfg.PuPDir} {
		if err := os.MkdirAll(d, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	if err := ValidatePaths(cfg); err != nil {
		t.Fatalf("ValidatePaths returned error when all dirs exist: %v", err)
	}
}
