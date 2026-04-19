package main

import (
	"crypto/sha256"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
)

type upgradeOptions struct {
	Version string
	Repo    string
	SkipDB  bool
}

func runUpgrade(opts upgradeOptions) error {
	repo := strings.TrimSpace(opts.Repo)
	if repo == "" {
		return fmt.Errorf("repo cannot be empty")
	}

	version := strings.TrimSpace(opts.Version)
	if version == "" || version == "latest" {
		resolved, err := latestVersion(repo)
		if err != nil {
			return err
		}
		version = resolved
	}

	artifact, err := upgradeArtifactName(version)
	if err != nil {
		return err
	}

	exePath, err := os.Executable()
	if err != nil {
		return fmt.Errorf("resolve executable path: %w", err)
	}
	exePath, _ = filepath.Abs(exePath)

	baseURL := fmt.Sprintf("https://github.com/%s/releases/download/%s", repo, version)
	binaryURL := fmt.Sprintf("%s/%s", baseURL, artifact)
	checksumsURL := fmt.Sprintf("%s/SHA256SUMS", baseURL)

	tmpDir, err := os.MkdirTemp("", "shellai-upgrade-*")
	if err != nil {
		return fmt.Errorf("create temp directory: %w", err)
	}
	defer os.RemoveAll(tmpDir)

	tmpArtifact := filepath.Join(tmpDir, artifact)
	tmpChecksums := filepath.Join(tmpDir, "SHA256SUMS")

	binData, err := download(binaryURL)
	if err != nil {
		return err
	}
	if err := os.WriteFile(tmpArtifact, binData, 0o755); err != nil {
		return fmt.Errorf("write downloaded binary: %w", err)
	}

	checksumsData, err := download(checksumsURL)
	if err != nil {
		return err
	}
	if err := os.WriteFile(tmpChecksums, checksumsData, 0o644); err != nil {
		return fmt.Errorf("write checksum file: %w", err)
	}

	if err := verifyReleaseChecksum(tmpArtifact, string(checksumsData), artifact); err != nil {
		return err
	}

	switched := false
	if runtime.GOOS == "windows" {
		if err := replaceWindowsExecutableDeferred(exePath, tmpArtifact); err != nil {
			return err
		}
		switched = true
		fmt.Printf("Upgrade scheduled to %s. Open a new terminal in a few seconds to use the new version.\n", version)
	} else {
		if err := os.Rename(tmpArtifact, exePath); err != nil {
			return fmt.Errorf("replace executable: %w", err)
		}
		if err := os.Chmod(exePath, 0o755); err != nil {
			return fmt.Errorf("set executable mode: %w", err)
		}
		switched = true
		fmt.Printf("Upgraded ShellAI binary to %s\n", version)
	}

	if !opts.SkipDB {
		if err := runUpdateDB(updateDBOptions{Version: version, Repo: repo}); err != nil {
			fmt.Printf("warning: binary upgraded but database refresh failed: %v\n", err)
		} else {
			fmt.Println("Command database refreshed.")
		}
	}

	if switched {
		fmt.Println("Upgrade complete.")
	}
	return nil
}

func upgradeArtifactName(version string) (string, error) {
	osPart := runtime.GOOS
	switch osPart {
	case "darwin", "linux", "windows":
	default:
		return "", fmt.Errorf("unsupported operating system %q", osPart)
	}

	archPart := runtime.GOARCH
	switch archPart {
	case "amd64", "arm64":
	default:
		return "", fmt.Errorf("unsupported architecture %q", archPart)
	}

	if osPart == "windows" {
		return fmt.Sprintf("shellai-%s-%s-%s.exe", version, osPart, archPart), nil
	}
	return fmt.Sprintf("shellai-%s-%s-%s", version, osPart, archPart), nil
}

func verifyReleaseChecksum(filePath, checksums, artifact string) error {
	var expected string
	for _, line := range strings.Split(checksums, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		parts := strings.Fields(line)
		if len(parts) < 2 {
			continue
		}
		name := strings.TrimPrefix(parts[len(parts)-1], "*")
		if name == artifact {
			expected = strings.ToLower(parts[0])
			break
		}
	}
	if expected == "" {
		return fmt.Errorf("checksum entry for %s not found", artifact)
	}

	data, err := os.ReadFile(filePath)
	if err != nil {
		return fmt.Errorf("read downloaded binary for checksum: %w", err)
	}
	actual := fmt.Sprintf("%x", sha256.Sum256(data))
	if actual != expected {
		return fmt.Errorf("checksum mismatch for %s", artifact)
	}
	return nil
}

func replaceWindowsExecutableDeferred(targetExe, downloadedExe string) error {
	scriptPath := filepath.Join(os.TempDir(), "shellai-upgrade.cmd")
	script := fmt.Sprintf("@echo off\r\n"+
		"ping 127.0.0.1 -n 3 > nul\r\n"+
		"move /y \"%s\" \"%s\" > nul\r\n"+
		"del /f /q %%~f0\r\n", downloadedExe, targetExe)

	if err := os.WriteFile(scriptPath, []byte(script), 0o600); err != nil {
		return fmt.Errorf("write upgrade helper script: %w", err)
	}

	cmd := exec.Command("cmd", "/C", "start", "", "/B", scriptPath)
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("start upgrade helper script: %w", err)
	}
	return nil
}
