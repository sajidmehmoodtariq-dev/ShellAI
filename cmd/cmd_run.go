package main

import (
	"shellai/ui"

	"github.com/spf13/cobra"
)

var runCmd = &cobra.Command{
	Use:   "run [query]",
	Short: "Run the interactive shell command assistant (default)",
	Long:  "Launch the interactive TUI for searching and executing shell commands. Can also accept a query to search directly.",
	RunE: func(cmd *cobra.Command, args []string) error {
		return runDefault(cmd, args)
	},
}

// runDefaultTUI runs the main interactive UI
func runDefaultTUI(cfg interface{}) error {
	return ui.Run()
}
