# ShellAI

ShellAI is an AI-assisted shell command helper with a terminal UI, safety checks, custom command support, import/export, and release packaging.

## Current Distribution Status

Yes, this is distributable.

The repo now includes:

- A Makefile for build, test, install, clean, and checksums.
- An installer script that supports curl-pipe-bash installs.
- A GitHub Actions release workflow that:
	- runs tests before build,
	- builds Linux binaries for amd64 and arm64,
	- generates SHA256SUMS,
	- tests the installer in a fresh container,
	- publishes release assets on version tags.

Current published binary targets are Linux amd64 and Linux arm64.

## Requirements

- Go 1.26.2+
- bash and curl for installer usage
- sha256sum or shasum for checksum verification

## Build Locally

Build for your current platform:

```bash
make build
```

Build release binaries (Linux amd64 and arm64):

```bash
make build-all
```

Run tests:

```bash
make test
```

Generate checksums:

```bash
make checksums
```

## Install Locally

Install the built binary to /usr/local/bin:

```bash
make install
```

## Install from GitHub Releases

If this project is hosted in OWNER/REPO, install with:

```bash
curl -fsSL https://raw.githubusercontent.com/OWNER/REPO/main/install.sh | SHELLAI_REPO=OWNER/REPO bash
```

Optional installer environment variables:

- SHELLAI_REPO: GitHub repo in owner/name format.
- SHELLAI_VERSION: release tag, for example v0.1.0 (default is latest).
- SHELLAI_INSTALL_DIR: override install target directory.
- SHELLAI_BASE_URL: override release download base URL.
- SHELLAI_API_URL: override latest-release API URL.

Example pinned install:

```bash
curl -fsSL https://raw.githubusercontent.com/OWNER/REPO/main/install.sh | SHELLAI_REPO=OWNER/REPO SHELLAI_VERSION=v0.1.0 bash
```

## Release Process

1. Ensure tests are green locally.
2. Commit changes.
3. Create and push a release tag.

```bash
git tag -a v0.1.0 -m "ShellAI v0.1.0"
git push origin main --tags
```

On tag push, GitHub Actions will:

1. Run test and vet checks.
2. Build Linux amd64 and arm64 binaries.
3. Generate SHA256SUMS.
4. Validate the installer in a clean Ubuntu container.
5. Publish GitHub Release assets.

## Verify Downloads Manually

After downloading release assets:

```bash
sha256sum -c SHA256SUMS
```

## Notes

- The Makefile is intended for Unix-like environments.
- On Windows, use go build directly or run make from WSL/Git Bash.
- The install script currently targets Linux and macOS style environments.