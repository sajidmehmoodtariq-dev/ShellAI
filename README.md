# ShellAI

ShellAI is a CLI and terminal UI assistant that helps you find, understand, and safely run shell commands.

It combines command search, templates, safety checks, optional LLM explanations, and user-defined command packs so you can move faster in the terminal without giving up control.

## Why ShellAI

- Turn plain-English intent into executable shell commands.
- Run commands with a built-in safety layer that flags risky operations.
- Add your own commands and share them with others.
- Import command packs from local files or URLs.
- Use an interactive TUI or subcommands directly.

## Core Features

- Intent search engine for command discovery.
- Template placeholder resolution.
- Safety guard with safe, warning, and dangerous levels.
- Streaming command execution.
- Optional LLM explanation mode.
- Bubble Tea based terminal UI.
- Custom command management:
 	- `shellai add`
 	- `shellai share`
 	- `shellai import`

## Command Surface

- `shellai run` - launch normal interactive usage.
- `shellai add` - add custom commands.
- `shellai share` - export user command entries.
- `shellai import` - import command entries from file or URL.
- `shellai update-db` - refresh installed command database for your platform.
- `shellai uninstall` - remove ShellAI from this machine.
- `shellai stats` - review hit counts and never-matched commands.
- `shellai explain` - force explanation mode.
- `shellai llm install|remove|status` - manage LLM backend status.

Global flags include:

- `--dry-run`
- `--no-confirm`
- `--version`

## Download and Install

ShellAI supports three ways to get started:

1. One-line installer (Linux and macOS)
2. One-line installer (Windows PowerShell)
3. Manual download from GitHub Releases (Linux and Windows)
4. Build from source (Linux, macOS, and Windows)

### Option 1: One-line installer (Linux and macOS)

```bash
curl -fsSL https://raw.githubusercontent.com/sajidmehmoodtariq-dev/ShellAI/main/install.sh | bash
```

Install a specific version:

```bash
curl -fsSL https://raw.githubusercontent.com/sajidmehmoodtariq-dev/ShellAI/main/install.sh | SHELLAI_VERSION=v0.1.0 bash
```

The installer:

- Detects OS and architecture.
- Downloads the matching binary from GitHub Releases.
- Verifies SHA256 checksum.
- Installs to `/usr/local/bin` (or a fallback path if needed).
- Downloads the platform command database and installs it as `~/.config/shellai/commands.json`.
- Stores `platform = "linux"` or `platform = "mac"` in `~/.config/shellai/config.toml`.

Notes:

- The installer supports Linux and macOS style environments.

### Option 2: One-line installer (Windows PowerShell)

Run in PowerShell:

``` powershell
iwr -useb https://raw.githubusercontent.com/sajidmehmoodtariq-dev/ShellAI/main/install.ps1 | iex
```

Install a specific version:

``` powershell
$env:SHELLAI_VERSION='v0.1.4'; iwr -useb https://raw.githubusercontent.com/sajidmehmoodtariq-dev/ShellAI/main/install.ps1 | iex
```

If `shellai` is not recognized immediately, open a new terminal and run:

``` powershell
shellai --version
```

The Windows installer:

- Detects architecture (`amd64` or `arm64`).
- Downloads `shellai-<version>-windows-<arch>.exe` from Releases.
- Verifies against `SHA256SUMS`.
- Installs to `%USERPROFILE%\\bin\\shellai.exe`.
- Adds that directory to user PATH if missing.
- Installs `%USERPROFILE%\\.config\\shellai\\commands.json`.
- Stores `platform = "windows"` in `%USERPROFILE%\\.config\\shellai\\config.toml`.

### Option 3: Manual download from Releases

1. Open the GitHub Releases page for this repository.
2. Download the binary that matches your platform.
3. Download `SHA256SUMS`.
4. Verify:

```bash
sha256sum -c SHA256SUMS
```

1. Install on Linux:

```bash
chmod +x shellai-<version>-linux-<arch>
sudo mv shellai-<version>-linux-<arch> /usr/local/bin/shellai
```

1. Install on Windows (PowerShell):

``` powershell
New-Item -ItemType Directory -Force -Path $env:USERPROFILE\bin | Out-Null
Copy-Item .\shellai-<version>-windows-amd64.exe $env:USERPROFILE\bin\shellai.exe -Force
$env:PATH = "$env:USERPROFILE\bin;$env:PATH"
```

1. Verify installation:

```bash
shellai --version
```

Current release artifacts:

- Linux amd64
- Linux arm64
- Windows amd64

### Option 4: Build from source (Linux/macOS/Windows)

Prerequisites:

- Go 1.26.2+

Build:

```bash
go build -o shellai ./cmd
```

Run tests:

```bash
go test ./...
```

If you use `make`, helper targets are available:

```bash
make build
make test
make install
make clean
```

## Supported Release Platforms

- Linux amd64
- Linux arm64
- Windows amd64

## Quick Start

Launch interactive mode:

```bash
shellai run
```

Add your first custom command:

```bash
shellai add
```

Share your command pack:

```bash
shellai share --format yaml --output commands.yaml
```

Import commands:

```bash
shellai import commands.yaml
```

## Configuration

ShellAI reads config from:

`~/.config/shellai/config.toml`

ShellAI reads the command database from:

`~/.config/shellai/commands.json`

Config precedence:

1. Environment variables
2. CLI flags
3. Config file
4. Built-in defaults

Update command database for your install platform:

```bash
shellai update-db
```

Pin to a specific release tag:

```bash
shellai update-db --version v0.1.4
```

## Accuracy Feedback Loop

After each command run in the TUI, ShellAI asks for one-key feedback:

- `y` = command/result was correct
- `n` = incorrect match (logged as miss)

Misses are stored locally in:

`~/.config/shellai/misses.log`

Each miss records:

- raw query
- returned command

Match hit counts are stored in:

`~/.config/shellai/hits.json`

Use this to review quality trends and zero-hit entries:

```bash
shellai stats
```

## Uninstall

ShellAI includes a built-in uninstall command for all platforms:

```bash
shellai uninstall
```

Skip the confirmation prompt:

```bash
shellai uninstall --yes
```

Keep your local data (commands and config):

```bash
shellai uninstall --keep-config --yes
```

Platform behavior:

- Linux: removes the current ShellAI binary and optionally `~/.config/shellai`.
- macOS: removes the current ShellAI binary and optionally `~/.config/shellai`.
- Windows: schedules removal of the current executable and optionally `%USERPROFILE%\.config\shellai`.

## Release Automation

GitHub Actions is configured to run on tagged releases.

On `v*` tags, it will:

1. Run test and vet checks.
2. Build Linux binaries (amd64 and arm64) and a Windows binary (amd64).
3. Generate checksums.
4. Validate installer flow.
5. Publish release assets.

## Project Status

Active development. The current release line starts at `v0.1.x`.
