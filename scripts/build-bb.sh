#!/bin/sh
# Build acmesh for one platform: one binary that carries the u-root commands,
# the ssh client and the tools from internal/tools.
#
#   ./scripts/build-bb.sh windows   -> acmesh.exe
#   ./scripts/build-bb.sh linux     -> acmesh
#
# A second argument picks a profile from scripts/profiles: full (default) or fs
# (no networking at all: the code is excluded, not switched off at runtime).
#
# Two things make the single binary possible:
#   - a go.work that lists both the pinned u-root checkout and this repository,
#     because makebb refuses packages from outside the main module;
#   - one small main package per tool under cmd/<name>.
#
# The u-root checkout is pinned, so a build is reproducible. This script never
# touches the acmesh repository's git.
set -eu

PLATFORM=${1:-linux}
PROFILE=${2:-full}
case "$PLATFORM" in
windows)
	export GOOS=windows GOARCH=amd64
	OUT=acmesh.exe
	# ssh and scp come from internal/tools now, for both targets.
	UROOT_EXTRA=""
	;;
linux)
	export GOOS=linux GOARCH=amd64
	OUT=acmesh
	# strace is Linux only. ssh and scp come from internal/tools now.
	UROOT_EXTRA="strace"
	;;
*)
	echo "usage: $0 [windows|linux]" >&2
	exit 2
	;;
esac

ROOT=$(cd "$(dirname "$0")/.." && pwd)
# A profile other than full gets its own file name, so both builds can sit next
# to each other without one overwriting the other.
case "$PROFILE" in
full) ;;
*)
	case "$OUT" in
	*.exe) OUT="${OUT%.exe}-$PROFILE.exe" ;;
	*) OUT="$OUT-$PROFILE" ;;
	esac
	;;
esac
UROOT_COMMIT=869f00f6b224ce02b1db51806159d6cd9c246f76
UROOT_DIR="$ROOT/.build/uroot"

export GOCACHE="$ROOT/.cache"
export GOPATH="$ROOT/.gopath"
export GOTOOLCHAIN=auto
export GOFLAGS=

if [ ! -d "$UROOT_DIR/.git" ]; then
	mkdir -p "$ROOT/.build"
	git clone -q https://github.com/u-root/u-root.git "$UROOT_DIR"
fi
git -C "$UROOT_DIR" fetch -q origin
git -C "$UROOT_DIR" checkout -q "$UROOT_COMMIT"

if [ ! -x "$GOPATH/bin/makebb" ]; then
	# GOOS/GOARCH are set for the target above; the tool itself must be built
	# for this host, otherwise it lands in bin/<goos>_<goarch>/.
	env -u GOOS -u GOARCH GOBIN="$GOPATH/bin" go install github.com/u-root/gobusybox/src/cmd/makebb@latest
fi
MAKEBB="$GOPATH/bin/makebb"
if [ ! -x "$MAKEBB" ]; then
	MAKEBB="$GOPATH/bin/makebb.exe"
fi

# The workspace is what lets makebb see our own packages.
WORKFILE="$ROOT/.build/go.work"
cat > "$WORKFILE" <<EOF
go 1.27.0

use (
	$UROOT_DIR
	$ROOT
)
EOF
export GOWORK="$WORKFILE"

# Every name our own tools provide. Where u-root also builds the same name for
# the target, u-root wins: it is already in the list and needs no extra bytes.
OURS_ALL="awk basename curl cut df diff dirname du env file free hostname gosh iconv jq iconv kill killall md5sum nl patch ps sed sha1sum sha256sum sha512sum split stat sync tac timeout truncate uname wget sh policy ssh scp"

PROFILE_FILE="$ROOT/scripts/profiles/$PROFILE.txt"
if [ ! -f "$PROFILE_FILE" ]; then
	echo "unknown profile $PROFILE; available:" >&2
	ls "$ROOT/scripts/profiles" | sed 's/\.txt$//' >&2
	exit 2
fi
TAGS=""
PROFILE_EXCLUDE=""
while read -r key value; do
	case "$key" in
	''|'#'*) continue ;;
	tags) TAGS="$value" ;;
	exclude) PROFILE_EXCLUDE="$PROFILE_EXCLUDE $value" ;;
	*) echo "profile $PROFILE: unknown line '$key'" >&2; exit 2 ;;
	esac
done < "$PROFILE_FILE"

# One name per line: cmds.txt groups names on a line, and the profile excludes
# compare whole lines. Without this, "exclude ping" never matched "ping netcat".
NAMES=$(grep -v '^#' "$ROOT/scripts/cmds.txt" | tr ' ' '\n' | sed '/^$/d')
for c in $PROFILE_EXCLUDE; do
	NAMES=$(echo "$NAMES" | grep -v "^$c$" || true)
done
PKGS=""
for c in $NAMES; do
	PKGS="$PKGS ./cmds/core/$c"
done
for c in $UROOT_EXTRA; do
	if [ -d "$UROOT_DIR/cmds/core/$c" ]; then
		PKGS="$PKGS ./cmds/core/$c"
	else
		PKGS="$PKGS ./cmds/exp/$c"
	fi
done
# Names back on one line for the membership test below.
NAMES_SPACED=$(echo $NAMES)
UROOT_NAMES=" $NAMES_SPACED $UROOT_EXTRA "
OURS=""
for c in $OURS_ALL; do
	for x in $PROFILE_EXCLUDE; do
		[ "$c" = "$x" ] && continue 2
	done
	case "$UROOT_NAMES" in
	*" $c "*) ;;
	*) OURS="$OURS $c" ;;
	esac
done
echo "own commands in this build:$OURS"
for c in $OURS; do
	PKGS="$PKGS $ROOT/cmd/$c"
done

cd "$UROOT_DIR"
# shellcheck disable=SC2086
# makebb has no -tags flag; the go tool it calls does read GOFLAGS.
if [ -n "$TAGS" ]; then
	export GOFLAGS="-tags=$TAGS"
	echo "build tags: $TAGS"
fi
# A generator-only run is what a non-Go compiler needs: makebb writes the
# multicall source and the build is done by the caller.
if [ -n "${ACMESH_GEN_DIR:-}" ]; then
	"$MAKEBB" -g -gen-dir "$ACMESH_GEN_DIR" -o "$ROOT/$OUT" $PKGS
	echo "generated: $ACMESH_GEN_DIR"
	exit 0
fi
# shellcheck disable=SC2086
"$MAKEBB" -o "$ROOT/$OUT" $PKGS
ls -l "$ROOT/$OUT"
