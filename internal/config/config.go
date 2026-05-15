// Package config owns path defaults and Config struct construction for the
// three domain packages (planner, executor, catalog). It is imported only by
// cmd/plunger/*.go and is NOT imported by any domain package (Pitfall 7).
package config

import (
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/iainconnor/vpin-plunger/internal/catalog"
	"github.com/iainconnor/vpin-plunger/internal/executor"
	"github.com/iainconnor/vpin-plunger/internal/planner"
)

// Baller Installer standard layout — defaults locked by D-12 / D-13.
const (
	BallerInstallerRoot = `C:\vPinball\visualpinball\`
	DefaultDBPath       = `C:\vPinball\PinUPSystem\PUPDatabase.db`
	DefaultPuPMediaRoot = `C:\vPinball\PinUPSystem\POPMedia\PuPPacks`
)

// Flags carries every CLI flag value relevant to Config construction.
// Empty string fields mean "use default" — the Build* functions substitute.
type Flags struct {
	// PUPDatabase + downloads roots
	DBPath       string
	DownloadsDir string

	// Operational toggles
	Rehearsal bool
	Auto      bool
	DryRun    bool

	// Per-asset destination overrides (empty = Baller Installer default)
	VPXDir       string
	BackglassDir string
	ROMDir       string
	NVRAMDir     string
	POVDir       string
	DMDDir       string // UltraDMD
	FlexDMDDir   string
	AudioDir     string // Altsound
	AltcolorDir  string
	MusicDir     string
	PuPDir       string

	// Catalog overrides
	CachePath string
	SheetURL  string
	Staleness time.Duration // 0 = use DefaultStaleness
}

// orDefault returns flag if non-empty, otherwise the joined default path.
func orDefault(flag, defaultPath string) string {
	if flag != "" {
		return flag
	}
	return defaultPath
}

// BuildPlannerConfig constructs a *planner.Config from Flags. The 11 install
// paths default to the Baller Installer layout rooted at BallerInstallerRoot
// (D-12). Operational dirs (vault/review/ignored/rehearsal) live under
// DownloadsDir. If DownloadsDir is empty, BuildPlannerConfig returns an error.
func BuildPlannerConfig(f Flags) (*planner.Config, error) {
	if f.DownloadsDir == "" {
		return nil, fmt.Errorf("config: --dir (downloads dir) is required")
	}
	root := BallerInstallerRoot
	return &planner.Config{
		VPXDir:       orDefault(f.VPXDir, filepath.Join(root, `Tables`)),
		BackglassDir: orDefault(f.BackglassDir, filepath.Join(root, `Tables`)),
		ROMDir:       orDefault(f.ROMDir, filepath.Join(root, `VPinMAME`, `roms`)),
		NVRAMDir:     orDefault(f.NVRAMDir, filepath.Join(root, `VPinMAME`, `nvram`)),
		POVDir:       orDefault(f.POVDir, filepath.Join(root, `Tables`)),
		DMDDir:       orDefault(f.DMDDir, filepath.Join(root, `UltraDMD`)),
		FlexDMDDir:   orDefault(f.FlexDMDDir, filepath.Join(root, `FlexDMD`)),
		AudioDir:     orDefault(f.AudioDir, filepath.Join(root, `VPinMAME`, `altsound`)),
		AltcolorDir:  orDefault(f.AltcolorDir, filepath.Join(root, `VPinMAME`, `altcolor`)),
		MusicDir:     orDefault(f.MusicDir, filepath.Join(root, `Music`)),
		PuPDir:       orDefault(f.PuPDir, DefaultPuPMediaRoot),

		ArchiveVaultDir: filepath.Join(f.DownloadsDir, `archive_vault`),
		ReviewDir:       filepath.Join(f.DownloadsDir, `review`),
		IgnoredDir:      filepath.Join(f.DownloadsDir, `ignored`),
		RehearsalDir:    filepath.Join(f.DownloadsDir, `rehearsal`),

		TrailingArticle: catalog.DefaultTrailingArticle,
		YearWindow:      catalog.DefaultYearWindow,
		Rehearsal:       f.Rehearsal,
	}, nil
}

// BuildExecutorConfig mirrors BuildPlannerConfig for executor.Config and
// additionally sets DBPath + BackupDir. dbPath argument allows the rehearsal
// caller to pass a copy path (D-15) instead of the real DB path.
func BuildExecutorConfig(f Flags, dbPath string) (*executor.Config, error) {
	p, err := BuildPlannerConfig(f)
	if err != nil {
		return nil, err
	}
	if dbPath == "" {
		dbPath = orDefault(f.DBPath, DefaultDBPath)
	}
	return &executor.Config{
		VPXDir:          p.VPXDir,
		BackglassDir:    p.BackglassDir,
		ROMDir:          p.ROMDir,
		NVRAMDir:        p.NVRAMDir,
		POVDir:          p.POVDir,
		DMDDir:          p.DMDDir,
		FlexDMDDir:      p.FlexDMDDir,
		AudioDir:        p.AudioDir,
		AltcolorDir:     p.AltcolorDir,
		MusicDir:        p.MusicDir,
		PuPDir:          p.PuPDir,
		ArchiveVaultDir: p.ArchiveVaultDir,
		ReviewDir:       p.ReviewDir,
		IgnoredDir:      p.IgnoredDir,
		RehearsalDir:    p.RehearsalDir,
		DBPath:          dbPath,
		BackupDir:       filepath.Join(filepath.Dir(dbPath), "backup"),
	}, nil
}

// BuildCatalogConfig constructs a *catalog.Config. CachePath defaults to
// {downloads_dir}/../cache/catalog.xlsx (the convention from Phase 3 D-11).
// SheetURL defaults to the Google Sheets export URL for CatalogSheetID.
func BuildCatalogConfig(f Flags) (*catalog.Config, error) {
	if f.DownloadsDir == "" && f.CachePath == "" {
		return nil, fmt.Errorf("config: --dir or explicit cache path required")
	}
	cache := f.CachePath
	if cache == "" {
		cache = filepath.Join(filepath.Dir(f.DownloadsDir), "cache", "catalog.xlsx")
	}
	url := f.SheetURL
	if url == "" {
		url = fmt.Sprintf("https://docs.google.com/spreadsheets/d/%s/export?format=xlsx", catalog.CatalogSheetID)
	}
	stale := f.Staleness
	if stale == 0 {
		stale = catalog.DefaultStaleness
	}
	return &catalog.Config{
		CachePath:       cache,
		SheetURL:        url,
		Staleness:       stale,
		YearWindow:      catalog.DefaultYearWindow,
		TrailingArticle: catalog.DefaultTrailingArticle,
	}, nil
}

// ValidatePaths checks that every required source/destination directory in
// cfg exists. Returns the FIRST missing directory as a named error so the
// operator can fix one path at a time (D-14). Operational dirs (vault,
// review, ignored, rehearsal) are NOT validated here — they are created
// on-demand by the executor.
func ValidatePaths(cfg *planner.Config) error {
	required := []struct{ name, path string }{
		{"VPXDir", cfg.VPXDir},
		{"BackglassDir", cfg.BackglassDir},
		{"ROMDir", cfg.ROMDir},
		{"NVRAMDir", cfg.NVRAMDir},
		{"POVDir", cfg.POVDir},
		{"DMDDir", cfg.DMDDir},
		{"FlexDMDDir", cfg.FlexDMDDir},
		{"AudioDir", cfg.AudioDir},
		{"AltcolorDir", cfg.AltcolorDir},
		{"MusicDir", cfg.MusicDir},
		{"PuPDir", cfg.PuPDir},
	}
	for _, r := range required {
		info, err := os.Stat(r.path)
		if err != nil {
			return fmt.Errorf("required path %s does not exist: %s (%w)", r.name, r.path, err)
		}
		if !info.IsDir() {
			return fmt.Errorf("required path %s is not a directory: %s", r.name, r.path)
		}
	}
	return nil
}
