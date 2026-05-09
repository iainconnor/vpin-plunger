// Package main is the vpin-plunger entry point. Wiring only — no domain
// logic per CLAUDE.md. Subcommand handlers live in their own files.
package main

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

func main() {
	root := &cobra.Command{
		Use:   "vpin",
		Short: "vpin-plunger — virtual pinball asset manager",
		Long:  "Scan downloads, plan every move, show the plan — then execute only when the user says go.",
		// Silence cobra's automatic usage/error printing on subcommand errors;
		// subcommands handle their own error reporting.
		SilenceUsage:  true,
		SilenceErrors: false,
	}

	root.AddCommand(newProcessCmd())

	if err := root.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
