package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/pelletier/go-toml/v2"
)

// Config represents the application configuration with precedence:
// Environment variables > CLI flags > Config file > Defaults
type Config struct {
	// Shell - preferred shell for command execution (default: auto-detect)
	Shell string `toml:"shell"`

	// DefaultSafetyLevel - default safety level for commands: "safe", "warning", "dangerous" (default: "warning")
	DefaultSafetyLevel string `toml:"default_safety_level"`

	// ShowAlternatives - show alternative commands when available (default: false)
	ShowAlternatives bool `toml:"show_alternatives"`

	// LLMModel - preferred LLM model for explanations (default: auto-detect llamafile/ollama)
	LLMModel string `toml:"llm_model"`

	// AutoConfirmSafe - auto-confirm commands marked as safe (default: false)
	AutoConfirmSafe bool `toml:"auto_confirm_safe"`

	// DryRun - show command but never execute (default: false) - can be overridden by CLI
	DryRun bool `toml:"dry_run"`

	// NoConfirm - skip confirmation for safe commands only (default: false) - can be overridden by CLI
	NoConfirm bool `toml:"no_confirm"`
}

// DefaultConfig returns a Config with sensible defaults
func DefaultConfig() Config {
	return Config{
		Shell:              "", // auto-detect
		DefaultSafetyLevel: "warning",
		ShowAlternatives:   false,
		LLMModel:           "", // auto-detect
		AutoConfirmSafe:    false,
		DryRun:             false,
		NoConfirm:          false,
	}
}

// Load loads configuration from file with environment variable overrides
func Load() (Config, error) {
	cfg := DefaultConfig()

	// Load from config file if it exists
	configPath := ConfigPath()
	if data, err := os.ReadFile(configPath); err == nil {
		if err := toml.Unmarshal(data, &cfg); err != nil {
			return Config{}, fmt.Errorf("parse config file: %w", err)
		}
	}

	// Override with environment variables
	if shell := os.Getenv("SHELLAI_SHELL"); shell != "" {
		cfg.Shell = shell
	}
	if level := os.Getenv("SHELLAI_SAFETY_LEVEL"); level != "" {
		cfg.DefaultSafetyLevel = level
	}
	if alts := os.Getenv("SHELLAI_SHOW_ALTERNATIVES"); alts != "" {
		cfg.ShowAlternatives = strings.ToLower(alts) == "true" || alts == "1"
	}
	if model := os.Getenv("SHELLAI_LLM_MODEL"); model != "" {
		cfg.LLMModel = model
	}
	if autoConf := os.Getenv("SHELLAI_AUTO_CONFIRM_SAFE"); autoConf != "" {
		cfg.AutoConfirmSafe = strings.ToLower(autoConf) == "true" || autoConf == "1"
	}
	if dryRun := os.Getenv("SHELLAI_DRY_RUN"); dryRun != "" {
		cfg.DryRun = strings.ToLower(dryRun) == "true" || dryRun == "1"
	}
	if noConf := os.Getenv("SHELLAI_NO_CONFIRM"); noConf != "" {
		cfg.NoConfirm = strings.ToLower(noConf) == "true" || noConf == "1"
	}

	return cfg, nil
}

// ConfigPath returns the path to the config file
func ConfigPath() string {
	if override := os.Getenv("SHELLAI_CONFIG_PATH"); override != "" {
		return override
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return ".shellai-config.toml" // fallback
	}
	return filepath.Join(home, ".config", "shellai", "config.toml")
}

// EnsureConfigExists creates a default config file if it doesn't exist
func EnsureConfigExists() error {
	configPath := ConfigPath()

	// Check if file already exists
	if _, err := os.Stat(configPath); err == nil {
		return nil // file exists
	}

	// Create directory structure
	if err := os.MkdirAll(filepath.Dir(configPath), 0o755); err != nil {
		return fmt.Errorf("create config directory: %w", err)
	}

	// Generate default config with documentation
	defaultCfg := defaultConfigContent()
	if err := os.WriteFile(configPath, []byte(defaultCfg), 0o644); err != nil {
		return fmt.Errorf("write default config: %w", err)
	}

	return nil
}

// defaultConfigContent returns the default config file with full documentation
func defaultConfigContent() string {
	return `# ShellAI Configuration File
# This file configures ShellAI behavior. Settings can be overridden by:
#   1. Environment variables (highest priority)
#   2. Command-line flags
#   3. Config file values (this file)
#   4. Built-in defaults (lowest priority)

# Shell to use for executing commands. Leave empty to auto-detect.
# Valid values: "bash", "zsh", "sh", "fish", "powershell", "cmd"
# Default: auto-detect based on $SHELL environment variable or system
shell = ""

# Default safety level when analyzing commands before execution
# Valid values: "safe", "warning", "dangerous"
# - "safe": Only allow commands marked as completely safe
# - "warning": Allow warning-level risks with confirmation
# - "dangerous": Allow all commands (use with caution!)
default_safety_level = "warning"

# Show alternative command suggestions when available
# When enabled, ShellAI may suggest safer or more efficient alternatives
# to commands you request
show_alternatives = false

# Preferred LLM model for command explanations. Leave empty to auto-detect.
# The system will try: llamafile -> Ollama -> fallback (no LLM)
# Valid values: "llamafile", "ollama", or a specific model name
llm_model = ""

# Auto-confirm execution for commands marked as "safe"
# When enabled, safe commands execute without requiring confirmation
# Only applies to commands at safety level "safe", not "warning"
auto_confirm_safe = false

# Dry-run mode: show commands but never execute them
# This is primarily set via CLI flag: --dry-run
# Can be overridden by: SHELLAI_DRY_RUN environment variable
dry_run = false

# Skip confirmation prompt for safe commands only
# When enabled, commands below the confirmation threshold execute immediately
# Only applies if they meet the safety criteria
no_confirm = false
`
}

// MergeFlags applies CLI flag values to the config, overriding config file values
// Only values that were explicitly set (non-zero) are applied
func (cfg *Config) MergeFlags(shell string, safetyLevel string, showAlts, dryRun, noConfirm bool, llmModel string) {
	if shell != "" {
		cfg.Shell = shell
	}
	if safetyLevel != "" {
		cfg.DefaultSafetyLevel = safetyLevel
	}
	if showAlts {
		cfg.ShowAlternatives = showAlts
	}
	if dryRun {
		cfg.DryRun = dryRun
	}
	if noConfirm {
		cfg.NoConfirm = noConfirm
	}
	if llmModel != "" {
		cfg.LLMModel = llmModel
	}
}
