package main

import (
	"fmt"
	"os"

	tea "charm.land/bubbletea/v2"
	"github.com/mattn/go-isatty"
	"github.com/spf13/cobra"

	"github.com/iainconnor/vpin-plunger/internal/app"
)

// processFlags collects the flag values for `vpin process`. Phase 1 only
// reads these to confirm the CLI surface compiles and parses; downstream
// phases (Phase 4+) consume them.
type processFlags struct {
	db        string
	dir       string
	auto      bool
	dryRun    bool
	rehearsal bool
}

// newProcessCmd builds the `vpin process` subcommand per TUI-08:
//
//	vpin process [--db PATH] [--dir PATH] [--auto] [--dry-run] [--rehearsal]
//
// The runtime decision between TUI mode and plain-stdout mode is made by
// inspecting whether os.Stdout is a terminal (TUI-05). When stdout is
// redirected (e.g. `vpin process > out.log`), tea.NewProgram is NEVER
// called — that is the only way to guarantee no TUI escape codes leak.
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

	return cmd
}

// runProcess is the entry point after cobra parses flags. It branches on
// TTY detection (TUI-05) to either launch the bubbletea TUI or emit plain
// stdout output. main.go remains free of TUI logic per CLAUDE.md.
func runProcess(flags *processFlags) error {
	if !isatty.IsTerminal(os.Stdout.Fd()) {
		// TUI-05: non-TTY mode. NEVER call tea.NewProgram — emit plain stdout.
		fmt.Println("vpin process (non-TTY mode)")
		fmt.Printf("  db=%q dir=%q auto=%t dry-run=%t rehearsal=%t\n",
			flags.db, flags.dir, flags.auto, flags.dryRun, flags.rehearsal)
		return nil
	}

	p := tea.NewProgram(app.New())
	if _, err := p.Run(); err != nil {
		return err
	}
	return nil
}
