#!/bin/sh
# timeweft installer for Linux and macOS.
#
#   curl -fsSL https://raw.githubusercontent.com/leue21/timeweft/main/install.sh | sh
#   curl -fsSL .../install.sh | sh -s -- --version v0.1.0 --dir /usr/local/bin
#
# Downloads the release archive for this OS/CPU, verifies its SHA-256 against
# the release's checksums.txt and installs the binary (default ~/.local/bin).
#
# Environment: TIMEWEFT_VERSION, TIMEWEFT_INSTALL_DIR, and
# TIMEWEFT_DOWNLOAD_URL (base URL holding the archives, e.g. a mirror).
set -eu

REPO="leue21/timeweft"

say() { printf 'timeweft-install: %s\n' "$*" >&2; }
err() { say "error: $*"; exit 1; }

usage() {
	cat >&2 <<EOF
Usage: install.sh [--version vX.Y.Z] [--dir DIR]

  --version  release to install (default: latest)
  --dir      install directory (default: ~/.local/bin)
EOF
}

download() { # url dest
	if command -v curl >/dev/null 2>&1; then
		curl -fsSL --retry 3 -o "$2" "$1"
	elif command -v wget >/dev/null 2>&1; then
		wget -q -O "$2" "$1"
	else
		err "curl or wget is required"
	fi
}

# latest_version follows GitHub's /releases/latest redirect (no API rate limit).
latest_version() {
	url="https://github.com/$REPO/releases/latest"
	if command -v curl >/dev/null 2>&1; then
		final=$(curl -fsSLI -o /dev/null -w '%{url_effective}' "$url")
	else
		final=$(wget -S --spider --max-redirect=0 "$url" 2>&1 | sed -n 's/^ *[Ll]ocation: *//p' | tr -d '\r' | tail -n 1)
	fi
	case "$final" in
	*/tag/*) printf '%s\n' "${final##*/}" ;;
	*) err "no published release found at $url" ;;
	esac
}

detect_os() {
	case "$(uname -s)" in
	Linux) echo linux ;;
	Darwin) echo darwin ;;
	MINGW* | MSYS* | CYGWIN*) err "on Windows use install.ps1 (see README)" ;;
	*) err "unsupported OS: $(uname -s)" ;;
	esac
}

detect_arch() {
	m=$(uname -m)
	# An x86_64 shell under Rosetta on Apple Silicon: install the native binary.
	if [ "$m" = x86_64 ] && [ "$(uname -s)" = Darwin ] &&
		[ "$(sysctl -n sysctl.proc_translated 2>/dev/null || echo 0)" = 1 ]; then
		m=arm64
	fi
	case "$m" in
	x86_64 | amd64) echo amd64 ;;
	aarch64 | arm64) echo arm64 ;;
	armv7* | armv8l) echo armv7 ;;
	*) err "unsupported CPU architecture: $m" ;;
	esac
}

sha256() {
	if command -v sha256sum >/dev/null 2>&1; then
		sha256sum "$1" | awk '{print $1}'
	elif command -v shasum >/dev/null 2>&1; then
		shasum -a 256 "$1" | awk '{print $1}'
	else
		err "sha256sum or shasum is required to verify the download"
	fi
}

main() {
	version="${TIMEWEFT_VERSION:-}"
	bin_dir="${TIMEWEFT_INSTALL_DIR:-$HOME/.local/bin}"
	base_url="${TIMEWEFT_DOWNLOAD_URL:-}"

	while [ $# -gt 0 ]; do
		case "$1" in
		--version | -v) [ $# -ge 2 ] || err "$1 needs a value"; version="$2"; shift 2 ;;
		--dir | -d) [ $# -ge 2 ] || err "$1 needs a value"; bin_dir="$2"; shift 2 ;;
		-h | --help) usage; exit 0 ;;
		*) usage; err "unknown option: $1" ;;
		esac
	done

	os=$(detect_os)
	arch=$(detect_arch)
	[ -n "$version" ] || version=$(latest_version)
	num="${version#v}"
	[ -n "$base_url" ] || base_url="https://github.com/$REPO/releases/download/v$num"
	archive="timeweft_${num}_${os}_${arch}.tar.gz"

	tmp=$(mktemp -d)
	trap 'rm -rf "$tmp"' EXIT INT TERM

	say "downloading timeweft $version ($os/$arch)"
	download "$base_url/$archive" "$tmp/$archive" || err "download failed: $base_url/$archive"
	download "$base_url/checksums.txt" "$tmp/checksums.txt" || err "download failed: $base_url/checksums.txt"

	expected=$(awk -v f="$archive" '$2 == f {print $1}' "$tmp/checksums.txt")
	[ -n "$expected" ] || err "$archive is not listed in checksums.txt"
	actual=$(sha256 "$tmp/$archive")
	[ "$expected" = "$actual" ] || err "checksum mismatch for $archive (expected $expected, got $actual)"

	tar -xzf "$tmp/$archive" -C "$tmp" timeweft
	mkdir -p "$bin_dir"
	cp "$tmp/timeweft" "$bin_dir/.timeweft.new"
	chmod 0755 "$bin_dir/.timeweft.new"
	mv -f "$bin_dir/.timeweft.new" "$bin_dir/timeweft"

	say "installed $("$bin_dir/timeweft" version) to $bin_dir/timeweft"
	case ":$PATH:" in
	*":$bin_dir:"*) ;;
	*) say "note: $bin_dir is not in your PATH; add:  export PATH=\"$bin_dir:\$PATH\"" ;;
	esac
	say "next: timeweft setup claude"
}

# Everything runs from main, so a partially downloaded script does nothing.
main "$@"
