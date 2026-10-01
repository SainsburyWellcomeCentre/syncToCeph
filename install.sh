#!/bin/sh
# Install synctoceph from this folder (a git clone of the project).
#
#   ./install.sh                 install into ~/.local (no administrator rights needed)
#   ./install.sh --prefix DIR    install into DIR/bin (sudo is used only if DIR is not writable)
#
# Running it again reinstalls. Remove everything with ./uninstall.sh.
set -eu

# The Go version pinned in go.mod, and the SHA-256 of the official downloads
# from https://go.dev/dl/. Update all three together.
GO_VERSION="1.27.1"
GO_SHA256_AMD64="63d339f0da5ab53635a56f2490a7984dfe12dfcff22ad749f63edaf590168445"
GO_SHA256_ARM64="3450b45a3f9ee8568792736a5c5e70a1f2e9b36c35a8f74958c03e51d7d92bec"
# Oldest rsync with every option synctoceph uses (--fsync).
MIN_RSYNC="3.2.4"
PKG="github.com/SainsburyWellcomeCentre/syncToCeph/internal/cli"

say() { printf '==> %s\n' "$*"; }
note() { printf '    %s\n' "$*"; }
die() {
	printf 'ERROR     %s\n' "$*" >&2
	exit 1
}

usage() {
	echo "Usage: ./install.sh [--prefix DIR]"
	echo "Installs synctoceph into DIR/bin (default: ~/.local/bin)."
}

prefix="$HOME/.local"
while [ $# -gt 0 ]; do
	case "$1" in
	--prefix)
		[ $# -ge 2 ] || die "--prefix needs a folder"
		prefix=$2
		shift 2
		;;
	--prefix=*)
		prefix=${1#*=}
		shift
		;;
	-h | --help)
		usage
		exit 0
		;;
	*)
		usage >&2
		die "unknown option: $1"
		;;
	esac
done
case "$prefix" in
/*) ;;
*) prefix="$(pwd)/$prefix" ;;
esac

cd "$(dirname "$0")"
data_home="${XDG_DATA_HOME:-$HOME/.local/share}"
cache_dir="${XDG_CACHE_HOME:-$HOME/.cache}/synctoceph"
record="$data_home/synctoceph/installed-files.txt"
bindir="$prefix/bin"

# download URL FILE: fetch a file with curl or wget.
download() {
	if command -v curl >/dev/null 2>&1; then
		curl -fsSL -o "$2" "$1"
	elif command -v wget >/dev/null 2>&1; then
		wget -q -O "$2" "$1"
	else
		die "neither curl nor wget is installed; install one (e.g. sudo apt install curl) and run ./install.sh again"
	fi
}

# 1. A Go toolchain of exactly the pinned version, used only for building.
say "Looking for Go $GO_VERSION (needed only to build synctoceph)"
GO=""
if command -v go >/dev/null 2>&1 && [ "$(GOTOOLCHAIN=local go env GOVERSION 2>/dev/null)" = "go$GO_VERSION" ]; then
	GO=$(command -v go)
	note "Using $GO"
else
	case "$(uname -m)" in
	x86_64 | amd64) arch=amd64 sum=$GO_SHA256_AMD64 ;;
	aarch64 | arm64) arch=arm64 sum=$GO_SHA256_ARM64 ;;
	*) die "no Go download for this processor ($(uname -m))" ;;
	esac
	toolchain="$cache_dir/go$GO_VERSION"
	if [ ! -x "$toolchain/bin/go" ]; then
		say "Downloading Go $GO_VERSION from go.dev into $toolchain"
		mkdir -p "$cache_dir"
		archive="$cache_dir/go$GO_VERSION.linux-$arch.tar.gz"
		download "https://go.dev/dl/go$GO_VERSION.linux-$arch.tar.gz" "$archive"
		say "Checking the download against the SHA-256 pinned in install.sh"
		if ! echo "$sum  $archive" | sha256sum -c - >/dev/null 2>&1; then
			rm -f "$archive"
			die "the Go download does not match its pinned SHA-256; nothing was installed. Try again later."
		fi
		rm -rf "$toolchain.partial"
		mkdir -p "$toolchain.partial"
		tar -xzf "$archive" -C "$toolchain.partial" --strip-components=1
		rm -f "$archive"
		mv "$toolchain.partial" "$toolchain"
	fi
	GO="$toolchain/bin/go"
	note "Using $GO"
fi

# 2. Build, with the version stamped in.
version="${SYNCTOCEPH_VERSION:-}"
commit="${SYNCTOCEPH_COMMIT:-}"
if [ -z "$version" ]; then
	version=$(git describe --tags --always --dirty 2>/dev/null || echo dev)
fi
if [ -z "$commit" ]; then
	commit=$(git rev-parse --short HEAD 2>/dev/null || echo unknown)
fi
build_date=$(date -u +%Y-%m-%dT%H:%M:%SZ)
say "Building synctoceph $version"
build_dir=$(mktemp -d)
trap 'rm -rf "$build_dir"' EXIT
CGO_ENABLED=0 GOTOOLCHAIN=local "$GO" build -trimpath \
	-ldflags "-s -w -X $PKG.Version=$version -X $PKG.Commit=$commit -X $PKG.BuildDate=$build_date" \
	-o "$build_dir/synctoceph" ./cmd/synctoceph

# 3. Install the program and the shell completions.
SUDO=""
if ! mkdir -p "$bindir" 2>/dev/null || [ ! -w "$bindir" ]; then
	[ "$prefix" != "$HOME/.local" ] || die "cannot write to $bindir; check its permissions"
	say "$bindir is not writable by you; using sudo to copy files there"
	SUDO="sudo"
fi
mkdir -p "$(dirname "$record")"
new_record="$record.new"
: >"$new_record"

# put SOURCE DEST MODE: copy a file into place (atomically) and record it.
put() {
	$SUDO mkdir -p "$(dirname "$2")"
	$SUDO cp "$1" "$2.new"
	$SUDO chmod "$3" "$2.new"
	$SUDO mv -f "$2.new" "$2"
	echo "$2" >>"$new_record"
	note "Installed $2"
}

say "Installing into $prefix"
put "$build_dir/synctoceph" "$bindir/synctoceph" 755
put completions/synctoceph.bash "$prefix/share/bash-completion/completions/synctoceph" 644
put completions/_synctoceph "$prefix/share/zsh/site-functions/_synctoceph" 644

# Keep earlier entries too, so uninstall.sh removes files from older installs.
if [ -f "$record" ]; then
	cat "$record" >>"$new_record"
fi
sort -u "$new_record" >"$record"
rm -f "$new_record"
note "Recorded the installed files in $record"

# 4. Is the program on PATH?
case ":$PATH:" in
*":$bindir:"*) ;;
*)
	say "Note: $bindir is not on your PATH"
	note "Add this line to ~/.bashrc (or ~/.zshrc), then open a new terminal:"
	note "  export PATH=\"$bindir:\$PATH\""
	;;
esac
case "$prefix" in
"$HOME/.local") ;;
*) note "For zsh completion, add $prefix/share/zsh/site-functions to fpath." ;;
esac

# 5. Is rsync new enough?
say "Checking rsync"
rsync_version=$(rsync --version 2>/dev/null | sed -n 's/^rsync *version v*\([0-9.]*\).*/\1/p' | head -n 1)
if [ -z "$rsync_version" ] || [ "$(printf '%s\n%s\n' "$MIN_RSYNC" "$rsync_version" | sort -t. -k1,1n -k2,2n -k3,3n | head -n 1)" != "$MIN_RSYNC" ]; then
	note "rsync $MIN_RSYNC or newer is needed (found: ${rsync_version:-none})."
	distro=$(sed -n 's/^ID=//p' /etc/os-release 2>/dev/null | tr -d '"')
	case "$distro" in
	ubuntu | debian) note "Install it with: sudo apt update && sudo apt install rsync" ;;
	fedora | rhel | centos | rocky | almalinux) note "Install it with: sudo dnf install rsync" ;;
	arch | manjaro) note "Install it with: sudo pacman -S rsync" ;;
	opensuse* | sles) note "Install it with: sudo zypper install rsync" ;;
	*) note "Install it with your distribution's package manager." ;;
	esac
else
	note "rsync $rsync_version is fine"
fi

# 6. Check the setup.
say "Running synctoceph doctor"
"$bindir/synctoceph" doctor || note "doctor found problems (see above); they can be fixed after installing."

say "Installed synctoceph $version"
cat <<EOF
Next steps:
  synctoceph init              set up this computer (machine name, folders, schedule)
  synctoceph doctor            check the setup
  synctoceph run --dry-run     see what would be copied
  synctoceph run               copy and verify
  synctoceph service install   run automatically on the schedule
EOF
