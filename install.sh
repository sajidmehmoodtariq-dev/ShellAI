#!/usr/bin/env bash

set -euo pipefail

REPO="${SHELLAI_REPO:-shellai/shellai}"
VERSION_INPUT="${SHELLAI_VERSION:-latest}"
BASE_URL="${SHELLAI_BASE_URL:-https://github.com/${REPO}/releases/download}"
API_URL="${SHELLAI_API_URL:-https://api.github.com/repos/${REPO}/releases/latest}"
INSTALL_DIR_OVERRIDE="${SHELLAI_INSTALL_DIR:-}"
BINARY_NAME="shellai"

log_info() {
	printf '%s\n' "$*"
}

log_warn() {
	printf 'warning: %s\n' "$*" >&2
}

log_error() {
	printf 'error: %s\n' "$*" >&2
}

have() {
	command -v "$1" >/dev/null 2>&1
}

detect_os() {
	case "$(uname -s)" in
	Linux*) echo linux ;;
	Darwin*) echo darwin ;;
	*) echo "" ;;
	esac
}

detect_arch() {
	case "$(uname -m)" in
	x86_64|amd64) echo amd64 ;;
	aarch64|arm64) echo arm64 ;;
	*) echo "" ;;
	esac
}

latest_version() {
	if have jq; then
		curl -fsSL "$API_URL" | jq -r '.tag_name'
		return
	fi

	curl -fsSL "$API_URL" | sed -n 's/.*"tag_name"[[:space:]]*:[[:space:]]*"\([^"]*\)".*/\1/p' | head -n 1
}

choose_install_dir() {
	if [ -n "$INSTALL_DIR_OVERRIDE" ]; then
		printf '%s\n' "$INSTALL_DIR_OVERRIDE"
		return
	fi

	if [ -w /usr/local/bin ] || have sudo; then
		printf '%s\n' /usr/local/bin
		return
	fi

	printf '%s\n' "$HOME/.local/bin"
}

verify_checksum() {
	if have sha256sum; then
		sha256sum -c SHA256SUMS --ignore-missing --status
		return
	fi

	if have shasum; then
		shasum -a 256 -c SHA256SUMS --ignore-missing --status
		return
	fi

	log_warn "sha256sum or shasum not found; skipping checksum verification"
}

install_binary() {
	local source_file="$1"
	local destination_file="$2"

	if [ -w "$(dirname "$destination_file")" ] || [ "$(dirname "$destination_file")" = "$HOME/.local/bin" ]; then
		mkdir -p "$(dirname "$destination_file")"
		if have install; then
			install -m 755 "$source_file" "$destination_file"
		else
			cp "$source_file" "$destination_file"
			chmod 755 "$destination_file"
		fi
		return
	fi

	if have sudo; then
		sudo install -m 755 "$source_file" "$destination_file"
		return
	fi

	log_error "cannot install to $(dirname "$destination_file") without write access or sudo"
	exit 1
}

main() {
	OS="$(detect_os)"
	ARCH="$(detect_arch)"
	if [ -z "$OS" ] || [ -z "$ARCH" ]; then
		log_error "unsupported platform: $(uname -s)/$(uname -m)"
		log_error "ShellAI release binaries are currently published for linux amd64 and arm64."
		exit 1
	fi

	if [ "$VERSION_INPUT" = "latest" ]; then
		VERSION="$(latest_version)"
	else
		VERSION="$VERSION_INPUT"
	fi

	if [ -z "$VERSION" ]; then
		log_error "unable to determine release version"
		exit 1
	fi

	INSTALL_DIR="$(choose_install_dir)"
	mkdir -p "$INSTALL_DIR"

	ARTIFACT="${BINARY_NAME}-${VERSION}-${OS}-${ARCH}"
	BINARY_URL="${BASE_URL}/${VERSION}/${ARTIFACT}"
	CHECKSUM_URL="${BASE_URL}/${VERSION}/SHA256SUMS"

	TMP_DIR="$(mktemp -d)"
	trap 'rm -rf "$TMP_DIR"' EXIT

	log_info "Downloading ShellAI ${VERSION} for ${OS}/${ARCH}"
	log_info "Binary: ${BINARY_URL}"

	cd "$TMP_DIR"
	curl -fsSLO "$BINARY_URL"
	curl -fsSLO "$CHECKSUM_URL"

	verify_checksum

	install_binary "$ARTIFACT" "$INSTALL_DIR/$BINARY_NAME"

	log_info "Installed to $INSTALL_DIR/$BINARY_NAME"
	"$INSTALL_DIR/$BINARY_NAME" --version
}

main "$@"
