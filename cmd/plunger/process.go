package main

import (
	"fmt"
	"io"
	"os"
	"path/filepath"

	tea "charm.land/bubbletea/v2"
	"github.com/mattn/go-isatty"
	"github.com/spf13/cobra"

	"github.com/iainconnor/vpin-plunger/internal/app"
	"github.com/iainconnor/vpin-plunger/internal/catalog"
	"github.com/iainconnor/vpin-plunger/internal/config"
	"github.com/iainconnor/vpin-plunger/internal/ui/components"
)

// processFlags collects the flag values for `vpin process`. Phase 1 only
// reads these to confirm the CLI surface compiles and parses; Phase 6 wires
// them into config.Build* and the TUI model options.
type processFlags struct {
	db        string
	dir       string
	auto      bool
	dryRun    bool
	rehearsal bool

	vpxDir, backglassDir, romDir, nvramDir, povDir, dmdDir,
		flexDMDDir, audioDir, altcolorDir, musicDir, pupDir string
}

// newProcessCmd builds the `vpin process` subcommand.
func newProcessCmd() *cobra.Command {
	flags := &processFlags{}
	cmd := &cobra.Command{
		Use:   "process",
		Short: "Scan downloads, plan, and (with confirmation) execute moves",
		RunE: func(cmd *cobra.Command, args []string) error {
			return runProcess(flags)
		},
	}
	cmd.Flags().StringVar(&flags.db, "db", "", "path to PUPDatabase.db")
	cmd.Flags().StringVar(&flags.dir, "dir", "", "path to downloads/ directory")
	cmd.Flags().BoolVar(&flags.auto, "auto", false, "non-interactive: send unmatched files to review")
	cmd.Flags().BoolVar(&flags.dryRun, "dry-run", false, "build and display the plan without executing")
	cmd.Flags().BoolVar(&flags.rehearsal, "rehearsal", false, "execute against a sandboxed rehearsal/ subdirectory")
	cmd.Flags().StringVar(&flags.vpxDir, "vpx-dir", "", "override default VPX install dir")
	cmd.Flags().StringVar(&flags.backglassDir, "backglass-dir", "", "override default backglass dir")
	cmd.Flags().StringVar(&flags.romDir, "rom-dir", "", "override default ROM dir")
	cmd.Flags().StringVar(&flags.nvramDir, "nvram-dir", "", "override default NVRAM dir")
	cmd.Flags().StringVar(&flags.povDir, "pov-dir", "", "override default POV dir")
	cmd.Flags().StringVar(&flags.dmdDir, "dmd-dir", "", "override default UltraDMD dir")
	cmd.Flags().StringVar(&flags.flexDMDDir, "flexdmd-dir", "", "override default FlexDMD dir")
	cmd.Flags().StringVar(&flags.audioDir, "audio-dir", "", "override default Altsound dir")
	cmd.Flags().StringVar(&flags.altcolorDir, "altcolor-dir", "", "override default Altcolor dir")
	cmd.Flags().StringVar(&flags.musicDir, "music-dir", "", "override default Music dir")
	cmd.Flags().StringVar(&flags.pupDir, "pup-dir", "", "override default PuP packs dir")
	return cmd
}

// flagsToConfigFlags maps processFlags to the config.Flags type consumed by
// the config.Build* functions. Empty strings mean "use default".
func flagsToConfigFlags(f *processFlags) config.Flags {
	return config.Flags{
		DBPath:       f.db,
		DownloadsDir: f.dir,
		Rehearsal:    f.rehearsal,
		Auto:         f.auto,
		DryRun:       f.dryRun,
		VPXDir:       f.vpxDir,
		BackglassDir: f.backglassDir,
		ROMDir:       f.romDir,
		NVRAMDir:     f.nvramDir,
		POVDir:       f.povDir,
		DMDDir:       f.dmdDir,
		FlexDMDDir:   f.flexDMDDir,
		AudioDir:     f.audioDir,
		AltcolorDir:  f.altcolorDir,
		MusicDir:     f.musicDir,
		PuPDir:       f.pupDir,
	}
}

// copyFile copies src to dst atomically (via temp file in the same dir).
func copyFile(src, dst string) error {
	if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
		return err
	}
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()
	out, err := os.Create(dst)
	if err != nil {
		return err
	}
	if _, err := io.Copy(out, in); err != nil {
		out.Close()
		return err
	}
	return out.Close() // captures flush/sync errors; not deferred
}

// determineInitialState computes which pre-scan picker (if any) should be shown
// at program start. Precedence (per CONTEXT.md BLOCKER 1+2):
//
//  1. StateRehearsalWipeCheck — if --rehearsal AND rehearsal/ dir already exists
//  2. StateCatalogFreshCheck  — if catalog missing OR stale
//  3. StateLoading            — cache is fresh; skip prompts, load directly
//
// Returns (state, picker candidates, cursor, initCmd). When initCmd is non-nil,
// the picker is skipped and the cmd fires immediately on Init().
func determineInitialState(rehearsal bool, rehearsalDir string, cat *catalog.Catalog) (
	app.AppState, []components.Candidate, int, tea.Cmd,
) {
	if rehearsal && rehearsalDir != "" {
		if _, err := os.Stat(rehearsalDir); err == nil {
			return app.StateRehearsalWipeCheck,
				[]components.Candidate{
					{Name: "Wipe rehearsal/ and rebuild", Parenthetical: "destructive", Confidence: 0},
					{Name: "Cancel", Parenthetical: "exit", Confidence: 0},
				}, 0, nil
		}
	}
	switch app.DetectCatalogFreshness(cat) {
	case app.CatalogMissing:
		return app.StateCatalogFreshCheck,
			[]components.Candidate{
				{Name: "Download catalog (recommended)", Parenthetical: "fetch fresh xlsx", Confidence: 0},
				{Name: "Exit", Parenthetical: "abort", Confidence: 0},
			}, 0, nil
	case app.CatalogStale:
		return app.StateCatalogFreshCheck,
			[]components.Candidate{
				{Name: "Use cached catalog", Parenthetical: "skip refresh", Confidence: 0},
				{Name: "Download fresh catalog", Parenthetical: "refresh from sheet", Confidence: 0},
			}, 0, nil
	default:
		// Fresh cache: skip the prompt, load directly.
		return app.StateLoading, nil, 0, app.StartProcess(cat)
	}
}

// runProcess is the entry point after cobra parses flags. It performs:
//  1. Flag → config mapping and path validation (D-14)
//  2. Rehearsal DB copy (D-15)
//  3. Freshness detection and initial picker seeding (MOD-02, D-16)
//  4. Auto-mode short-circuit for both pre-scan pickers (MOD-06)
//  5. TUI construction and launch
func runProcess(flags *processFlags) error {
	cf := flagsToConfigFlags(flags)

	plannerCfg, err := config.BuildPlannerConfig(cf)
	if err != nil {
		return err
	}

	// D-14: path validation BEFORE any TUI/state launch.
	if err := config.ValidatePaths(plannerCfg); err != nil {
		return fmt.Errorf("path validation failed: %w", err)
	}

	// D-15: rehearsal DB copy. Copies the real PUPDatabase.db to
	// rehearsal/PUPDatabase.db so the executor operates on a safe sandbox.
	dbPath := flags.db
	if dbPath == "" {
		dbPath = config.DefaultDBPath
	}
	if flags.rehearsal {
		rehDB := filepath.Join(plannerCfg.RehearsalDir, "PUPDatabase.db")
		// Pre-create rehearsal/ if absent so the DB copy has a directory.
		if _, statErr := os.Stat(plannerCfg.RehearsalDir); os.IsNotExist(statErr) {
			if err := os.MkdirAll(plannerCfg.RehearsalDir, 0o755); err != nil {
				return fmt.Errorf("create rehearsal/: %w", err)
			}
		}
		if _, statErr := os.Stat(dbPath); statErr == nil {
			if err := copyFile(dbPath, rehDB); err != nil {
				return fmt.Errorf("copy PUPDatabase to rehearsal: %w", err)
			}
		}
		dbPath = rehDB
	}

	execCfg, err := config.BuildExecutorConfig(cf, dbPath)
	if err != nil {
		return err
	}

	catCfg, err := config.BuildCatalogConfig(cf)
	if err != nil {
		return err
	}
	cat := catalog.New(catCfg)

	autoMode := flags.auto || flags.dryRun

	// Compute initial picker state BEFORE constructing the Model.
	initState, initCands, initCursor, initCmd := determineInitialState(
		flags.rehearsal, plannerCfg.RehearsalDir, cat,
	)

	// In --auto mode no picker should ever be shown. Collapse to the
	// corresponding non-interactive path for each pre-scan prompt.
	if autoMode {
		switch initState {
		case app.StateRehearsalWipeCheck:
			// Auto-wipe rehearsal/ and re-evaluate.
			if err := os.RemoveAll(plannerCfg.RehearsalDir); err != nil {
				return fmt.Errorf("auto wipe rehearsal/: %w", err)
			}
			// Re-seed the rehearsal DB after the wipe: the DB copy made above was
			// inside the dir we just deleted, so dbPath points to a non-existent
			// file. Re-create rehearsal/ and re-copy the source DB so the executor
			// opens a valid schema. (Rule 1 fix: auto-wipe left dbPath dangling.)
			if flags.rehearsal {
				rehDB := filepath.Join(plannerCfg.RehearsalDir, "PUPDatabase.db")
				origDB := flags.db
				if origDB == "" {
					origDB = config.DefaultDBPath
				}
				if err := os.MkdirAll(plannerCfg.RehearsalDir, 0o755); err != nil {
					return fmt.Errorf("re-create rehearsal/ after wipe: %w", err)
				}
				if _, statErr := os.Stat(origDB); statErr == nil {
					if err := copyFile(origDB, rehDB); err != nil {
						return fmt.Errorf("re-copy PUPDatabase after rehearsal wipe: %w", err)
					}
				}
				dbPath = rehDB
			}
			// After wipe, re-evaluate (rehearsal dir is gone; no wipe prompt needed).
			initState, initCands, initCursor, initCmd = determineInitialState(false, "", cat)
		case app.StateCatalogFreshCheck:
			// Auto-mode: always load (download if missing; use cache if stale).
			initState = app.StateLoading
			initCands = nil
			initCursor = 0
			initCmd = app.StartProcess(cat)
		}
	}

	opts := []app.Option{
		app.WithAutoMode(autoMode),
		app.WithDownloadsDir(flags.dir),
		app.WithScanConfig(plannerCfg),
		app.WithExecConfig(execCfg),
		app.WithDBPath(dbPath),
		app.WithRehearsalDir(plannerCfg.RehearsalDir),
		app.WithCatalog(cat),
		app.WithInitialState(initState),
	}
	if initCands != nil {
		opts = append(opts, app.WithPickerCandidates(initCands))
		_ = initCursor // cursor is always 0; Picker default is already 0
	}
	if initCmd != nil {
		opts = append(opts, app.WithInitCmd(initCmd))
	}

	m := app.New(opts...)

	if !isatty.IsTerminal(os.Stdout.Fd()) {
		// Non-TTY: no picker prompts possible. Require --auto to keep the
		// run unattended; return a clear error if a picker prompt would be shown.
		if initState == app.StateRehearsalWipeCheck || initState == app.StateCatalogFreshCheck {
			return fmt.Errorf("non-TTY mode requires --auto to handle pre-scan prompts")
		}
		p := tea.NewProgram(m, tea.WithoutRenderer(), tea.WithInput(nil))
		_, runErr := p.Run()
		return runErr
	}

	p := tea.NewProgram(m)
	if _, err := p.Run(); err != nil {
		return err
	}
	return nil
}
