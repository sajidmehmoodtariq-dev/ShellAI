package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadConfigWithUTF8BOM(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("SHELLAI_CONFIG_DIR", dir)

	configPath := filepath.Join(dir, "config.toml")
	data := []byte{0xEF, 0xBB, 0xBF}
	data = append(data, []byte("platform = \"windows\"\nno_confirm = true\n")...)
	if err := os.WriteFile(configPath, data, 0o644); err != nil {
		t.Fatalf("write config: %v", err)
	}

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load returned error: %v", err)
	}

	if cfg.Platform != "windows" {
		t.Fatalf("expected platform windows, got %q", cfg.Platform)
	}
	if !cfg.NoConfirm {
		t.Fatalf("expected no_confirm true")
	}
}

func TestLoadConfigWithMojibakeBOMPrefix(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("SHELLAI_CONFIG_DIR", dir)

	configPath := filepath.Join(dir, "config.toml")
	data := []byte("ï»¿platform = \"linux\"\nno_confirm = true\n")
	if err := os.WriteFile(configPath, data, 0o644); err != nil {
		t.Fatalf("write config: %v", err)
	}

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load returned error: %v", err)
	}

	if cfg.Platform != "linux" {
		t.Fatalf("expected platform linux, got %q", cfg.Platform)
	}
	if !cfg.NoConfirm {
		t.Fatalf("expected no_confirm true")
	}
}
