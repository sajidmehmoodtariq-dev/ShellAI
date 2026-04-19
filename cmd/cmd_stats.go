package main

import "github.com/spf13/cobra"

var (
	statsTop int
)

var statsCmd = &cobra.Command{
	Use:   "stats",
	Short: "Show search accuracy feedback stats",
	Long:  "Show matched command hit counts and commands that have never been matched.",
	RunE: func(cmd *cobra.Command, args []string) error {
		return runStats(statsTop)
	},
}

func init() {
	statsCmd.Flags().IntVar(&statsTop, "top", 10, "number of top matched commands to display")
}
