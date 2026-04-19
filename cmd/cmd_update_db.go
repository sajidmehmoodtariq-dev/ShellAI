package main

import "github.com/spf13/cobra"

var (
	updateDBVersion string
	updateDBRepo    string
)

var updateDBCmd = &cobra.Command{
	Use:   "update-db",
	Short: "Update installed command database",
	Long:  "Download the platform-specific command database selected at installation time and replace commands.json.",
	RunE: func(cmd *cobra.Command, args []string) error {
		return runUpdateDB(updateDBOptions{
			Version: updateDBVersion,
			Repo:    updateDBRepo,
		})
	},
}

func init() {
	updateDBCmd.Flags().StringVar(&updateDBVersion, "version", "latest", "release tag to pull database from (for example v0.1.4 or latest)")
	updateDBCmd.Flags().StringVar(&updateDBRepo, "repo", "sajidmehmoodtariq-dev/ShellAI", "GitHub repository in owner/name format")
}
