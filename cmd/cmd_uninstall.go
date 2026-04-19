package main

import "github.com/spf13/cobra"

var (
	uninstallYes        bool
	uninstallKeepConfig bool
)

var uninstallCmd = &cobra.Command{
	Use:   "uninstall",
	Short: "Remove ShellAI from this machine",
	Long:  "Uninstall ShellAI binary and optionally remove local configuration and command databases.",
	RunE: func(cmd *cobra.Command, args []string) error {
		return runUninstall(uninstallOptions{
			Yes:        uninstallYes,
			KeepConfig: uninstallKeepConfig,
		})
	},
}

func init() {
	uninstallCmd.Flags().BoolVarP(&uninstallYes, "yes", "y", false, "skip confirmation prompt")
	uninstallCmd.Flags().BoolVar(&uninstallKeepConfig, "keep-config", false, "keep ~/.config/shellai data")
}
