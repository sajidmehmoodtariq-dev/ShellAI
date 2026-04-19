package main

import (
	"strings"

	"github.com/spf13/cobra"
)

var chainCmd = &cobra.Command{
	Use:   "chain [workflow]",
	Short: "Run a multi-step workflow",
	Long:  "Parse a natural-language multi-step workflow and execute each step with per-step confirmation.",
	Args:  cobra.MinimumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		workflow := strings.TrimSpace(strings.Join(args, " "))
		return runChain(workflow)
	},
}
