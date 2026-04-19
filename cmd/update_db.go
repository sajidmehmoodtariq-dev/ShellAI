package main

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"shellai/internal/config"
)

type updateDBOptions struct {
	Version string
	Repo    string
}

type latestReleaseResponse struct {
	TagName string `json:"tag_name"`
}

func runUpdateDB(opts updateDBOptions) error {
	repo := strings.TrimSpace(opts.Repo)
	if repo == "" {
		return fmt.Errorf("repo cannot be empty")
	}

	platform := normalizePlatform(cfg.Platform)
	if platform == "" {
		return fmt.Errorf("config platform is not set; reinstall with install script or set platform in %s", config.ConfigPath())
	}

	version := strings.TrimSpace(opts.Version)
	if version == "" {
		version = "latest"
	}
	if version == "latest" {
		resolved, err := latestVersion(repo)
		if err != nil {
			return err
		}
		version = resolved
	}

	dbFile, err := platformDBFile(platform)
	if err != nil {
		return err
	}

	url := fmt.Sprintf("https://raw.githubusercontent.com/%s/%s/db/%s", repo, version, dbFile)
	data, err := download(url)
	if err != nil {
		return err
	}

	if err := os.MkdirAll(filepath.Dir(config.CommandsPath()), 0o755); err != nil {
		return fmt.Errorf("create config directory: %w", err)
	}
	if err := os.WriteFile(config.CommandsPath(), data, 0o644); err != nil {
		return fmt.Errorf("write commands database: %w", err)
	}

	fmt.Printf("Updated commands database for platform %s from %s\n", platform, version)
	fmt.Printf("Saved to %s\n", config.CommandsPath())
	return nil
}

func normalizePlatform(value string) string {
	v := strings.ToLower(strings.TrimSpace(value))
	switch v {
	case "darwin":
		return "mac"
	default:
		return v
	}
}

func platformDBFile(platform string) (string, error) {
	switch platform {
	case "linux":
		return "commands_linux.json", nil
	case "mac":
		return "commands_mac.json", nil
	case "windows":
		return "commands_windows.json", nil
	default:
		return "", fmt.Errorf("unsupported platform %q in config; expected linux, mac, or windows", platform)
	}
}

func latestVersion(repo string) (string, error) {
	url := fmt.Sprintf("https://api.github.com/repos/%s/releases/latest", repo)
	data, err := download(url)
	if err != nil {
		return "", err
	}

	var out latestReleaseResponse
	if err := json.Unmarshal(data, &out); err != nil {
		return "", fmt.Errorf("parse latest release response: %w", err)
	}
	if strings.TrimSpace(out.TagName) == "" {
		return "", fmt.Errorf("latest release response did not include tag_name")
	}
	return strings.TrimSpace(out.TagName), nil
}

func download(url string) ([]byte, error) {
	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Get(url)
	if err != nil {
		return nil, fmt.Errorf("download %s: %w", url, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("download %s returned status %d", url, resp.StatusCode)
	}

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read response body from %s: %w", url, err)
	}
	if len(strings.TrimSpace(string(data))) == 0 {
		return nil, fmt.Errorf("downloaded empty response from %s", url)
	}
	return data, nil
}
