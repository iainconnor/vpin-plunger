package main

import (
	"fmt"
	"os"
	"os/exec"
	"runtime"
	"strings"

	tea "charm.land/bubbletea/v2"
	"github.com/mattn/go-isatty"
	"github.com/spf13/cobra"

	"github.com/iainconnor/vpin-plunger/internal/app"
	"github.com/iainconnor/vpin-plunger/internal/catalog"
	"github.com/iainconnor/vpin-plunger/internal/config"
)

type downloadFlags struct {
	db     string
	dir    string
	dryRun bool
}

func newDownloadCmd() *cobra.Command {
	flags := &downloadFlags{}
	cmd := &cobra.Command{
		Use:   "download",
		Short: "Open VPW/VPS URLs for uninstalled catalog entries, grouped by manufacturer + decade",
		Long: "Loads the catalog and the installed Games table, finds catalog entries " +
			"that are not installed, groups them by manufacturer and decade, and opens " +
			"their VPW Version Link / VPS Link URLs in the default browser. " +
			"With --dry-run, the URLs are printed instead of opened.",
		RunE: func(cmd *cobra.Command, args []string) error {
			return runDownload(flags)
		},
	}
	cmd.Flags().StringVar(&flags.db, "db", "", "path to PUPDatabase.db")
	cmd.Flags().StringVar(&flags.dir, "dir", "", "path to downloads/ directory")
	cmd.Flags().BoolVar(&flags.dryRun, "dry-run", false, "print URLs instead of opening them")
	return cmd
}

// openURL opens url in the default system browser using a runtime.GOOS switch.
// CGO-free per CLAUDE.md (uses only os/exec + runtime).
//
// URL scheme validation is performed first: only http:// and https:// URLs are
// accepted. On Windows, rundll32 url.dll,FileProtocolHandler is used instead of
// cmd.exe /c start, which avoids passing the URL through cmd.exe and eliminates
// shell-special-character injection (& ^ | > etc.) that cmd /c start is
// vulnerable to.
func openURL(url string) error {
	if !strings.HasPrefix(url, "https://") && !strings.HasPrefix(url, "http://") {
		return fmt.Errorf("openURL: unsupported scheme (only http/https allowed): %s", url)
	}
	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "windows":
		// rundll32 url.dll,FileProtocolHandler does not pass the URL through
		// cmd.exe, so shell metacharacters in the URL are not interpreted.
		cmd = exec.Command("rundll32", "url.dll,FileProtocolHandler", url)
	case "darwin":
		cmd = exec.Command("open", url)
	default: // linux, freebsd, etc.
		cmd = exec.Command("xdg-open", url)
	}
	return cmd.Start() // Start, not Run, so we don't block on browser process
}

// openGroups iterates DownloadGroupMsg and opens (or skips) each URL.
// In dry-run mode the TUI's downloadPrintCmd already printed the URLs, so
// openGroups is a no-op. Returns the first openURL error encountered, but
// continues opening remaining URLs so a single failure doesn't abort the batch.
func openGroups(msg app.DownloadGroupMsg, dryRun bool) error {
	var firstErr error
	for _, g := range msg.Groups {
		for _, u := range g.URLs {
			if dryRun {
				continue // app.Update() already prints via downloadPrintCmd
			}
			if err := openURL(u); err != nil && firstErr == nil {
				firstErr = fmt.Errorf("openURL %s: %w", u, err)
			}
		}
	}
	return firstErr
}

func runDownload(flags *downloadFlags) error {
	cf := config.Flags{DBPath: flags.db, DownloadsDir: flags.dir, DryRun: flags.dryRun}

	catCfg, err := config.BuildCatalogConfig(cf)
	if err != nil {
		return fmt.Errorf("download: %w", err)
	}
	cat := catalog.New(catCfg)

	dbPath := flags.db
	if dbPath == "" {
		dbPath = config.DefaultDBPath
	}

	m := app.New(
		app.WithInitCmd(app.StartDownload(cat, dbPath, flags.dryRun)),
		app.WithDBPath(dbPath),
	)

	if !isatty.IsTerminal(os.Stdout.Fd()) {
		p := tea.NewProgram(m, tea.WithoutRenderer(), tea.WithInput(nil))
		if _, err := p.Run(); err != nil {
			return err
		}
	} else {
		p := tea.NewProgram(m)
		if _, err := p.Run(); err != nil {
			return err
		}
	}

	// After the TUI prints the URLs, open them in the system browser (non-dry-run only).
	// We re-derive the groups using catalog + db: catalog.Load() is idempotent so
	// reusing cat costs nothing. app.DownloadBuildOnce shares the same data path as
	// the tea.Cmd-based downloadBuildCmd.
	if !flags.dryRun {
		groups, err := app.DownloadBuildOnce(cat, dbPath)
		if err != nil {
			return fmt.Errorf("download: derive groups for open: %w", err)
		}
		if err := openGroups(app.DownloadGroupMsg{Groups: groups}, false); err != nil {
			return err
		}
	}
	return nil
}
