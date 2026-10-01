#!/bin/sh
# Update synctoceph to the latest version from GitHub and reinstall it.
#
#   ./update.sh
#
# It stops running systemd services first (a copy in progress is stopped
# gracefully and resumes on the next run), reinstalls with ./install.sh into
# the same place, and starts the services again.
set -eu

say() { printf '==> %s\n' "$*"; }
note() { printf '    %s\n' "$*"; }
die() {
	printf 'ERROR     %s\n' "$*" >&2
	exit 1
}

cd "$(dirname "$0")"

say "Checking this folder for local changes"
if [ -n "$(git status --porcelain)" ]; then
	git status --short
	cat >&2 <<'EOF'
ERROR     This folder has local changes (listed above), so it was not updated.
          Nothing was changed. Choose one:
            keep them for later:  git stash       then ./update.sh, then git stash pop
            keep them for good:   git commit -am "my changes"   then ./update.sh
            throw them away:      git checkout -- .   (this cannot be undone)
EOF
	exit 1
fi

say "Downloading the latest version (git pull --ff-only)"
git pull --ff-only || die "git pull failed; the folder was not changed. If your branch has its own commits, see git status."

# Reinstall into the same place as before.
data_home="${XDG_DATA_HOME:-$HOME/.local/share}"
record="$data_home/synctoceph/installed-files.txt"
prefix="$HOME/.local"
if [ -f "$record" ]; then
	bin=$(grep '/bin/synctoceph$' "$record" | head -n 1 || true)
	if [ -n "$bin" ]; then
		prefix=$(dirname "$(dirname "$bin")")
	fi
fi

# Stop running systemd services (Task Scheduler tasks start the new program
# by themselves at their next run).
unit_dir="${XDG_CONFIG_HOME:-$HOME/.config}/systemd/user"
restart=""
if command -v systemctl >/dev/null 2>&1; then
	for unit_file in "$unit_dir"/synctoceph-*.service; do
		[ -e "$unit_file" ] || continue
		unit=$(basename "$unit_file")
		if systemctl --user is-active --quiet "$unit"; then
			say "Stopping $unit (a copy in progress resumes after the update)"
			systemctl --user stop "$unit"
			restart="$restart $unit"
		fi
	done
fi

./install.sh --prefix "$prefix"

if command -v systemctl >/dev/null 2>&1 && [ -d "$unit_dir" ]; then
	systemctl --user daemon-reload
fi
for unit in $restart; do
	say "Starting $unit again"
	systemctl --user start "$unit"
done
say "Update finished."
