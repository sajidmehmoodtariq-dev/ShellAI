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
- `shellai explain` - force explanation mode.
- `shellai llm install|remove|status` - manage LLM backend status.

Global flags include:

- `--dry-run`
- `--no-confirm`
- `--version`

## Download and Install

ShellAI supports three ways to get started:

1. One-line installer (best for Linux users)
2. Manual download from GitHub Releases
3. Build from source (works for Linux, macOS, and Windows)

### Option 1: One-line installer

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

Note:

- Current published release binaries are Linux-only (`amd64`, `arm64`).
- On macOS and Windows, use Option 3 (build from source) unless Linux/macOS release assets are added later.

### Option 2: Manual download from Releases

1. Open the GitHub Releases page for this repository.
2. Download the binary that matches your platform.
3. Download `SHA256SUMS`.
4. Verify:

```bash
sha256sum -c SHA256SUMS
```

5. Install:

```bash
chmod +x shellai-<version>-linux-<arch>
sudo mv shellai-<version>-linux-<arch> /usr/local/bin/shellai
```

6. Verify installation:

```bash
shellai --version
```

For current releases, manual binaries are available for Linux `amd64` and `arm64`.

### Option 3: Build from source (Linux/macOS/Windows)

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

Config precedence:

1. Environment variables
2. CLI flags
3. Config file
4. Built-in defaults

## Release Automation

GitHub Actions is configured to run on tagged releases.

On `v*` tags, it will:

1. Run test and vet checks.
2. Build Linux binaries (amd64 and arm64).
3. Generate checksums.
4. Validate installer flow.
5. Publish release assets.

## Project Status

Active development. The current release line starts at `v0.1.x`.