package main

import (
	"github.com/spf13/cobra"
)

var (
	shareFormat string
	shareOutput string
)

var shareCmd = &cobra.Command{
	Use:   "share",
	Short: "Export your custom commands",
	Long:  "Export your custom commands to a file or stdout in YAML or JSON format. Share with others or backup your commands.",
	RunE: func(cmd *cobra.Command, args []string) error {
		opts := []string{}
		if shareFormat != "" {
			opts = append(opts, "--format", shareFormat)
		}
		if shareOutput != "" {
			opts = append(opts, "--output", shareOutput)
		}
		return runShare(opts)
	},
}

var (
	importSource string
)

var importCmd = &cobra.Command{
	Use:   "import [file or URL]",
	Short: "Import commands from a file or URL",
	Long:  "Import commands from a local YAML/JSON file or a remote URL. Handles conflicts with per-intent prompts.",
	Args:  cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		opts := []string{}
		if len(args) > 0 {
			opts = append(opts, args[0])
		}
		if importSource != "" {
			opts = append(opts, "--source", importSource)
		}
		return runImport(opts)
	},
}

func init() {
	shareCmd.Flags().StringVar(&shareFormat, "format", "yaml", "output format (yaml or json)")
	shareCmd.Flags().StringVarP(&shareOutput, "output", "o", "", "output file (default: stdout)")

	importCmd.Flags().StringVar(&importSource, "source", "", "import source (file path or URL)")
}
