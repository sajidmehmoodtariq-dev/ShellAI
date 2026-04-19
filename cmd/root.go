package main

import (
	"fmt"
	"os"

	"shellai/internal/config"

	"github.com/spf13/cobra"
)

// Build metadata - set at compile time via ldflags
// Example: go build -ldflags "-X main.version=v0.1.0 -X main.commit=abc123 -X main.buildDate=2024-01-01"
var (
	version   = "dev"
	commit    = "unknown"
	buildDate = "unknown"
)

// Global config and flags
var (
	cfg               config.Config
	globalShell       string
	globalSafetyLevel string
	globalShowAlts    bool
	globalDryRun      bool
	globalNoConfirm   bool
	globalLLMModel    string
)

// rootCmd is the base command for the entire application
var rootCmd = &cobra.Command{
	Use:     "shellai",
	Short:   "AI-powered shell command assistant",
	Long:    "ShellAI helps you understand and execute shell commands safely with AI assistance.",
	Version: version,
	RunE:    runDefault,
	PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
		// Load configuration from file and environment variables
		var err error
		cfg, err = config.Load()
		if err != nil {
			return fmt.Errorf("load config: %w", err)
		}

		// Ensure config file exists for future runs
		if err := config.EnsureConfigExists(); err != nil {
			// Non-fatal: warn but continue
			fmt.Fprintf(os.Stderr, "warning: could not create config file: %v\n", err)
		}

		// Merge CLI flags (they override config file and env vars already in cfg from Load())
		cfg.MergeFlags(globalShell, globalSafetyLevel, globalShowAlts, globalDryRun, globalNoConfirm, globalLLMModel)

		return nil
	},
}

// runDefault runs the default TUI when no subcommand is specified
func runDefault(cmd *cobra.Command, args []string) error {
	// This will be implemented to call the UI
	return runDefaultTUI(cfg)
}

// Execute runs the root command and all subcommands
func Execute() error {
	return rootCmd.Execute()
}

func init() {
	// Global flags that apply to all commands
	rootCmd.PersistentFlags().StringVar(&globalShell, "shell", "", "shell to use for execution (bash, zsh, sh, fish, powershell, cmd)")
	rootCmd.PersistentFlags().StringVar(&globalSafetyLevel, "safety-level", "", "default safety level (safe, warning, dangerous)")
	rootCmd.PersistentFlags().BoolVar(&globalShowAlts, "show-alternatives", false, "show alternative command suggestions")
	rootCmd.PersistentFlags().BoolVar(&globalDryRun, "dry-run", false, "show command but do not execute")
	rootCmd.PersistentFlags().BoolVar(&globalNoConfirm, "no-confirm", false, "skip confirmation for safe commands only")
	rootCmd.PersistentFlags().StringVar(&globalLLMModel, "llm-model", "", "preferred LLM model (llamafile, ollama, or model name)")
	rootCmd.SetVersionTemplate("ShellAI {{.Version}}\n")

	// Add subcommands
	rootCmd.AddCommand(runCmd)
	rootCmd.AddCommand(addCmd)
	rootCmd.AddCommand(shareCmd)
	rootCmd.AddCommand(importCmd)
	rootCmd.AddCommand(explainCmd)
	rootCmd.AddCommand(llmCmd)
}
