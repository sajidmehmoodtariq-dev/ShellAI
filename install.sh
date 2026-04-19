#!/usr/bin/env bash

set -euo pipefail

REPO="${SHELLAI_REPO:-sajidmehmoodtariq-dev/ShellAI}"
VERSION_INPUT="${SHELLAI_VERSION:-latest}"
BASE_URL="${SHELLAI_BASE_URL:-https://github.com/${REPO}/releases/download}"
API_URL="${SHELLAI_API_URL:-https://api.github.com/repos/${REPO}/releases/latest}"
RAW_BASE_URL="${SHELLAI_RAW_BASE_URL:-https://raw.githubusercontent.com/${REPO}}"
INSTALL_DIR_OVERRIDE="${SHELLAI_INSTALL_DIR:-}"
BINARY_NAME="shellai"
CONFIG_DIR="${SHELLAI_CONFIG_DIR:-$HOME/.config/shellai}"

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

detect_platform() {
	case "$(uname -s)" in
	Linux*) echo linux ;;
	Darwin*) echo mac ;;
	*) echo "" ;;
	esac
}

binary_os() {
	case "$1" in
	linux) echo linux ;;
	mac) echo darwin ;;
	*) echo "" ;;
	esac
}

platform_db_file() {
	case "$1" in
	linux) echo commands_linux.json ;;
	mac) echo commands_mac.json ;;
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

set_config_platform() {
	local config_file="$1"
	local platform="$2"

	mkdir -p "$(dirname "$config_file")"
	if [ ! -f "$config_file" ]; then
		cat >"$config_file" <<EOF
platform = "${platform}"
EOF
		return
	fi

	if grep -q '^platform[[:space:]]*=' "$config_file"; then
		sed -i.bak "s/^platform[[:space:]]*=.*/platform = \"${platform}\"/" "$config_file"
		rm -f "${config_file}.bak"
	else
		printf '\nplatform = "%s"\n' "$platform" >>"$config_file"
	fi
}

main() {
	PLATFORM="$(detect_platform)"
	ARCH="$(detect_arch)"
	if [ -z "$PLATFORM" ] || [ -z "$ARCH" ]; then
		log_error "unsupported platform: $(uname -s)/$(uname -m)"
		log_error "ShellAI installer supports linux and mac platforms."
		exit 1
	fi

	OS="$(binary_os "$PLATFORM")"
	DB_FILE="$(platform_db_file "$PLATFORM")"
	if [ -z "$OS" ] || [ -z "$DB_FILE" ]; then
		log_error "unsupported platform mapping for ${PLATFORM}"
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
	DB_URL="${RAW_BASE_URL}/${VERSION}/db/${DB_FILE}"

	TMP_DIR="$(mktemp -d)"
	trap 'rm -rf "$TMP_DIR"' EXIT

	log_info "Downloading ShellAI ${VERSION} for ${PLATFORM}/${ARCH}"
	log_info "Binary: ${BINARY_URL}"

	cd "$TMP_DIR"
	curl -fsSLO "$BINARY_URL"
	curl -fsSLO "$CHECKSUM_URL"
	curl -fsSLo "$DB_FILE" "$DB_URL"

	verify_checksum

	install_binary "$ARTIFACT" "$INSTALL_DIR/$BINARY_NAME"
	mkdir -p "$CONFIG_DIR"
	cp "$DB_FILE" "$CONFIG_DIR/commands.json"
	chmod 644 "$CONFIG_DIR/commands.json"
	set_config_platform "$CONFIG_DIR/config.toml" "$PLATFORM"

	log_info "Installed to $INSTALL_DIR/$BINARY_NAME"
	log_info "Installed command database to $CONFIG_DIR/commands.json"
	"$INSTALL_DIR/$BINARY_NAME" --version
}

main "$@"
