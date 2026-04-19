package main

import (
	"github.com/spf13/cobra"
)

var (
	addFromStdin     bool
	addGeneratePrompt bool
)

var addCmd = &cobra.Command{
	Use:   "add",
	Short: "Add a custom shell command",
	Long:  "Interactively add a new custom command to your personal database. You can provide YAML via stdin or use the editor.",
	RunE: func(cmd *cobra.Command, args []string) error {
		var opts []string
		if addFromStdin {
			opts = append(opts, "--from-stdin")
		}
		if addGeneratePrompt {
			opts = append(opts, "--generate-prompt")
		}
		return runAdd(opts)
	},
}

func init() {
	addCmd.Flags().BoolVar(&addFromStdin, "from-stdin", false, "read command entry from stdin as YAML")
	addCmd.Flags().BoolVar(&addGeneratePrompt, "generate-prompt", false, "use LLM to help generate the command template")
}
