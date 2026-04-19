package main

import "github.com/spf13/cobra"

var (
	upgradeVersion string
	upgradeRepo    string
	upgradeSkipDB  bool
)

var upgradeCmd = &cobra.Command{
	Use:   "upgrade",
	Short: "Upgrade ShellAI to a newer version",
	Long:  "Download and install a newer ShellAI binary from GitHub Releases, then optionally refresh the command database.",
	RunE: func(cmd *cobra.Command, args []string) error {
		return runUpgrade(upgradeOptions{
			Version: upgradeVersion,
			Repo:    upgradeRepo,
			SkipDB:  upgradeSkipDB,
		})
	},
}

func init() {
	upgradeCmd.Flags().StringVar(&upgradeVersion, "version", "latest", "release tag to install (for example v0.1.6 or latest)")
	upgradeCmd.Flags().StringVar(&upgradeRepo, "repo", "sajidmehmoodtariq-dev/ShellAI", "GitHub repository in owner/name format")
	upgradeCmd.Flags().BoolVar(&upgradeSkipDB, "skip-db", false, "skip command database refresh after upgrade")
}
