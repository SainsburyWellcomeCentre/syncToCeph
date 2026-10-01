#!/bin/sh
# Remove synctoceph from this computer.
#
#   ./uninstall.sh                remove the program, completions and automatic runs
#   ./uninstall.sh --purge        also remove every config file, the state folders
#                                 (status and logs) and the Go build cache; it lists
#                                 them and asks before deleting anything
#   ./uninstall.sh --purge --yes  the same, without asking (for scripts)
#
# It never touches the archive (including its .syncToCeph/ records and the
# history of replaced files) or the source data.
set -eu

say() { printf '==> %s\n' "$*"; }
note() { printf '    %s\n' "$*"; }
die() {
	printf 'ERROR     %s\n' "$*" >&2
	exit 1
}

usage() {
	echo "Usage: ./uninstall.sh [--purge [--yes]]"
	echo "  --purge   also delete configuration, state (status and logs) and the build cache"
	echo "  --yes     do not ask before deleting (needed with --purge when not run in a terminal)"
}

purge=0
yes=0
for arg in "$@"; do
	case "$arg" in
	--purge) purge=1 ;;
	-y | --yes) yes=1 ;;
	-h | --help)
		usage
		exit 0
		;;
	*)
		usage >&2
		die "unknown option: $arg"
		;;
	esac
done

data_home="${XDG_DATA_HOME:-$HOME/.local/share}"
record="$data_home/synctoceph/installed-files.txt"
config_dir="${XDG_CONFIG_HOME:-$HOME/.config}/synctoceph"
state_dir="${XDG_STATE_HOME:-$HOME/.local/state}/synctoceph"
cache_dir="${XDG_CACHE_HOME:-$HOME/.cache}/synctoceph"

# purge_dirs prints the folders --purge deletes, one per line.
purge_dirs() {
	printf '%s\n' "$config_dir" "$state_dir" "$cache_dir" "$data_home/synctoceph"
}

# The installed program, from the install record (or PATH).
bin=""
if [ -f "$record" ]; then
	bin=$(grep '/bin/synctoceph$' "$record" | head -n 1 || true)
fi
if [ -z "$bin" ] || [ ! -x "$bin" ]; then
	bin=$(command -v synctoceph || true)
fi

# With --purge, show what will be deleted and ask first, before anything is
# changed, so answering "no" leaves everything as it was.
if [ "$purge" -eq 1 ]; then
	say "--purge deletes these folders and everything in them:"
	purge_dirs | while IFS= read -r dir; do
		if [ -e "$dir" ]; then note "$dir"; fi
	done
	for file in "$config_dir"/*.toml; do
		if [ -e "$file" ]; then note "  profile $(basename "$file" .toml): $file"; fi
	done
	note "It does not touch the archive (including its .syncToCeph/ records and the"
	note "history of replaced files) or the source data."
	if [ "$yes" -eq 0 ]; then
		[ -t 0 ] || die "--purge asks for confirmation; run it in a terminal, or add --yes. Nothing was changed."
		printf 'Delete them? Type yes to continue: '
		read -r answer || answer=""
		[ "$answer" = "yes" ] || {
			note "Nothing was changed."
			exit 1
		}
	fi
fi

say "Removing automatic runs (services and scheduled tasks) of every profile"
if [ -n "$bin" ] && [ -x "$bin" ]; then
	# --all (not --all-profiles) also works with programs installed by older versions.
	"$bin" service uninstall --all || note "Could not remove them all; check with: synctoceph service status"
else
	note "synctoceph is not installed, so there are no automatic runs to remove."
fi

# A run started by hand in another terminal still uses the state folder.
if [ "$purge" -eq 1 ] && [ -n "$bin" ] && [ -x "$bin" ] &&
	"$bin" status --all-profiles --json 2>/dev/null | grep -q '"running": true'; then
	"$bin" status --all-profiles 2>/dev/null || true
	die "a synctoceph run is still active (see above). Stop it with: synctoceph --profile NAME stop
          then run ./uninstall.sh --purge again. Only the automatic runs were removed."
fi

say "Removing the installed files"
if [ -f "$record" ]; then
	while IFS= read -r file; do
		# Only remove what install.sh installs, even if the record was edited.
		case "$file" in
		/*/bin/synctoceph | /*/completions/synctoceph | /*/site-functions/_synctoceph) ;;
		*)
			[ -z "$file" ] || note "Skipped unexpected entry: $file"
			continue
			;;
		esac
		if [ -e "$file" ] || [ -L "$file" ]; then
			if [ -w "$(dirname "$file")" ]; then
				rm -f "$file"
			else
				sudo rm -f "$file"
			fi
			note "Removed $file"
		fi
	done <"$record"
	rm -f "$record"
	rmdir "$(dirname "$record")" 2>/dev/null || true
else
	note "No install record at $record; nothing to remove."
fi

if [ "$purge" -eq 1 ]; then
	say "Removing configuration, state, logs and build cache (--purge)"
	purge_dirs | while IFS= read -r dir; do
		if [ -e "$dir" ]; then
			rm -rf "$dir"
			note "Removed $dir"
		fi
	done
fi

say "Done. synctoceph never touches the archive or the source; your data is where it was."
if [ "$purge" -eq 0 ]; then
	note "Kept (delete with ./uninstall.sh --purge):"
	note "  configuration:  $config_dir"
	note "  state and logs: $state_dir"
	note "  build cache:    $cache_dir"
fi
note "Kept: this folder ($(cd "$(dirname "$0")" && pwd)); delete it yourself if you no longer need it."
