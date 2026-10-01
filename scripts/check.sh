#!/bin/sh
# The one command that checks everything. A change is not done until this
# passes. GitHub Actions runs the same script.
#
# It runs: gofmt, goimports, go vet, staticcheck, the tests (with the race
# detector), shellcheck on every script, a check that docs/cli/ and the
# completion scripts are up to date, and a Markdown link check.
#
# staticcheck and goimports are run at pinned versions with `go run`, so they
# need no installation. shellcheck is used from PATH if present; otherwise a
# pinned release is downloaded once, checked against its SHA-256, and kept in
# ~/.cache/synctoceph/tools/.
set -eu
cd "$(dirname "$0")/.."

STATICCHECK="honnef.co/go/tools/cmd/staticcheck@v0.8.1"
GOIMPORTS="golang.org/x/tools/cmd/goimports@v0.50.0"
SHELLCHECK_VERSION="v0.11.0"
SHELLCHECK_SHA256_X86_64="b7af85e41cc99489dcc21d66c6d5f3685138f06d34651e6d34b42ec6d54fe6f6"
SHELLCHECK_SHA256_AARCH64="68a8133197a50beb8803f8d42f9908d1af1c5540d4bb05fdfca8c1fa47decefc"
TOOLS_DIR="${SYNCTOCEPH_TOOLS_DIR:-${XDG_CACHE_HOME:-$HOME/.cache}/synctoceph/tools}"

failed=""

step() {
	printf '\n==> %s\n' "$1"
}

fail() {
	echo "FAILED: $1"
	failed="$failed
  - $1"
}

# find_shellcheck prints the path of a shellcheck program, downloading the
# pinned release if none is installed.
find_shellcheck() {
	if command -v shellcheck >/dev/null 2>&1; then
		command -v shellcheck
		return
	fi
	case "$(uname -m)" in
	x86_64) arch=x86_64 sum=$SHELLCHECK_SHA256_X86_64 ;;
	aarch64 | arm64) arch=aarch64 sum=$SHELLCHECK_SHA256_AARCH64 ;;
	*)
		echo "no shellcheck for $(uname -m); install it with your package manager" >&2
		return 1
		;;
	esac
	dir="$TOOLS_DIR/shellcheck-$SHELLCHECK_VERSION"
	if [ ! -x "$dir/shellcheck" ]; then
		echo "Downloading shellcheck $SHELLCHECK_VERSION into $dir ..." >&2
		mkdir -p "$dir"
		archive="$dir/shellcheck.tar.gz"
		url="https://github.com/koalaman/shellcheck/releases/download/$SHELLCHECK_VERSION/shellcheck-$SHELLCHECK_VERSION.linux.$arch.tar.gz"
		curl -fsSL -o "$archive" "$url"
		echo "$sum  $archive" | sha256sum -c - >/dev/null
		tar -xzf "$archive" -C "$dir" --strip-components=1
		rm -f "$archive"
	fi
	echo "$dir/shellcheck"
}

step "gofmt"
out=$(gofmt -l cmd internal)
if [ -n "$out" ]; then
	echo "$out"
	fail "gofmt (fix with: gofmt -w cmd internal)"
fi

step "goimports"
out=$(go run "$GOIMPORTS" -l cmd internal) || fail "goimports could not run"
if [ -n "$out" ]; then
	echo "$out"
	fail "goimports (fix with: go run $GOIMPORTS -w cmd internal)"
fi

step "go vet"
go vet ./... || fail "go vet"

step "staticcheck"
go run "$STATICCHECK" ./... || fail "staticcheck"

step "go test -race"
go test -race -count=1 ./... || fail "tests"

step "shellcheck"
if sc=$(find_shellcheck); then
	"$sc" install.sh update.sh uninstall.sh scripts/*.sh || fail "shellcheck"
else
	fail "shellcheck is not available"
fi

step "docs/cli and completions are up to date"
tmp=$(mktemp -d)
trap 'rm -rf "$tmp"' EXIT
go run ./internal/tools/gendocs "$tmp/cli" "$tmp/completions"
if ! diff -r docs/cli "$tmp/cli" || ! diff -r completions "$tmp/completions"; then
	fail "generated docs are out of date (fix with: ./scripts/gen-docs.sh)"
fi

step "Markdown links"
go run ./internal/tools/mdlinks . || fail "broken Markdown links"

if [ -n "$failed" ]; then
	printf '\nRESULT    FAILED:%s\n' "$failed"
	exit 1
fi
printf '\nRESULT    OK: all checks passed.\n'
