#!/bin/sh
# Remove synctoceph from this computer.
#
#   ./uninstall.sh           remove the program, completions and automatic runs
#   ./uninstall.sh --purge   also remove the configuration, state (logs) and build cache
#
# It never touches the archive or the source data.
set -eu

say() { printf '==> %s\n' "$*"; }
note() { printf '    %s\n' "$*"; }

purge=0
for arg in "$@"; do
	case "$arg" in
	--purge) purge=1 ;;
	-h | --help)
		echo "Usage: ./uninstall.sh [--purge]"
		exit 0
		;;
	*)
		printf 'ERROR     unknown option: %s\n' "$arg" >&2
		exit 1
		;;
	esac
done

data_home="${XDG_DATA_HOME:-$HOME/.local/share}"
record="$data_home/synctoceph/installed-files.txt"
config_dir="${XDG_CONFIG_HOME:-$HOME/.config}/synctoceph"
state_dir="${XDG_STATE_HOME:-$HOME/.local/state}/synctoceph"
cache_dir="${XDG_CACHE_HOME:-$HOME/.cache}/synctoceph"

# The installed program, from the install record (or PATH).
bin=""
if [ -f "$record" ]; then
	bin=$(grep '/bin/synctoceph$' "$record" | head -n 1 || true)
fi
if [ -z "$bin" ] || [ ! -x "$bin" ]; then
	bin=$(command -v synctoceph || true)
fi

say "Removing automatic runs (services and scheduled tasks) of every profile"
if [ -n "$bin" ] && [ -x "$bin" ]; then
	"$bin" service uninstall --all || note "Could not remove them all; check with: synctoceph service status"
else
	note "synctoceph is not installed, so there are no automatic runs to remove."
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
	say "Removing configuration, state and build cache (--purge)"
	for dir in "$config_dir" "$state_dir" "$cache_dir"; do
		if [ -e "$dir" ]; then
			rm -rf "$dir"
			note "Removed $dir"
		fi
	done
fi

say "Done. synctoceph never touches the archive or the source; your data is where it was."
if [ "$purge" -eq 0 ]; then
	note "Kept (remove with --purge):"
	note "  configuration: $config_dir"
	note "  state and logs: $state_dir"
	note "  build cache:   $cache_dir"
fi
note "Kept: this folder ($(cd "$(dirname "$0")" && pwd)); delete it yourself if you no longer need it."
