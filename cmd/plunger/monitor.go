package main

import (
	"fmt"
	"os"

	tea "charm.land/bubbletea/v2"
	"github.com/mattn/go-isatty"
	"github.com/spf13/cobra"

	"github.com/iainconnor/vpin-plunger/internal/app"
	"github.com/iainconnor/vpin-plunger/internal/catalog"
	"github.com/iainconnor/vpin-plunger/internal/config"
)

type monitorFlags struct {
	db  string
	dir string
}

func newMonitorCmd() *cobra.Command {
	flags := &monitorFlags{}
	cmd := &cobra.Command{
		Use:   "monitor",
		Short: "Compare installed Games table against the community catalog",
		Long: "Loads the catalog and the PUPDatabase Games table, then prints three " +
			"sections: Not Installed (in catalog but not installed), Not in Catalog " +
			"(installed but unknown to catalog), and Name Mismatch (installed GameName " +
			"differs from catalog canonical).",
		RunE: func(cmd *cobra.Command, args []string) error {
			return runMonitor(flags)
		},
	}
	cmd.Flags().StringVar(&flags.db, "db", "", "path to PUPDatabase.db")
	cmd.Flags().StringVar(&flags.dir, "dir", "", "path to downloads/ directory (used to locate catalog cache)")
	return cmd
}

func runMonitor(flags *monitorFlags) error {
	cf := config.Flags{DBPath: flags.db, DownloadsDir: flags.dir}

	catCfg, err := config.BuildCatalogConfig(cf)
	if err != nil {
		return fmt.Errorf("monitor: %w", err)
	}
	cat := catalog.New(catCfg)

	dbPath := flags.db
	if dbPath == "" {
		dbPath = config.DefaultDBPath
	}

	m := app.New(
		app.WithInitCmd(app.StartMonitor(cat, dbPath)),
		app.WithDBPath(dbPath),
	)

	if !isatty.IsTerminal(os.Stdout.Fd()) {
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
