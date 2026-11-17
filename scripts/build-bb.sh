#!/bin/sh
# Build the acmesh binary for one platform.
#
#   ./scripts/build-bb.sh windows   -> bb.exe
#   ./scripts/build-bb.sh linux     -> bb
#
# The u-root checkout is pinned, so a build is reproducible. Only the u-root
# clone is fetched here; this script never touches the acmesh repository's git.
set -eu

PLATFORM=${1:-linux}
case "$PLATFORM" in
windows)
	export GOOS=windows GOARCH=amd64
	OUT=bb.exe
	EXTRA=""
	;;
linux)
	export GOOS=linux GOARCH=amd64
	OUT=bb
	EXTRA=$(cat "$(dirname "$0")/cmds-linux-only.txt" | grep -v '^#' | sed '/^$/d')
	;;
*)
	echo "usage: $0 [windows|linux]" >&2
	exit 2
	;;
esac

ROOT=$(cd "$(dirname "$0")/.." && pwd)
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

NAMES=$(grep -v '^#' "$ROOT/scripts/cmds.txt" | sed '/^$/d')
if [ -n "$EXTRA" ]; then
	NAMES="$NAMES
$EXTRA"
fi

PKGS=""
for c in $NAMES; do
	PKGS="$PKGS ./cmds/core/$c"
done

cd "$UROOT_DIR"
"$MAKEBB" -o "$ROOT/$OUT" $PKGS
ls -l "$ROOT/$OUT"
