#!/bin/sh
# Build the acmesh busybox for one platform: one binary that carries the u-root
# commands, the ssh client and the tools from internal/tools.
#
#   ./scripts/build-bb.sh windows   -> bb.exe
#   ./scripts/build-bb.sh linux     -> bb
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
case "$PLATFORM" in
windows)
	export GOOS=windows GOARCH=amd64
	OUT=acmesh.exe
	# ssh builds for Windows. The four names u-root only builds for Linux
	# (hostname, sync, timeout, uname) come from our own tools there.
	UROOT_EXTRA="ssh"
	;;
linux)
	export GOOS=linux GOARCH=amd64
	OUT=acmesh
	# ssh and strace are Linux-only here. sync, timeout, uname and hostname
	# come from u-root, so our own versions of those four stay out.
	# hostname, sync, timeout and uname come from our own tools: u-root's
	# timeout takes a different command line, and its uname/sync add nothing.
	UROOT_EXTRA="ssh strace df du free kill ps"
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
OURS_ALL="awk basename cut diff dirname env file hostname iconv md5sum nl patch sed sha1sum sha256sum sha512sum split stat sync tac timeout truncate uname"

NAMES=$(grep -v '^#' "$ROOT/scripts/cmds.txt" | sed '/^$/d')
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
UROOT_NAMES=" $NAMES $UROOT_EXTRA "
OURS=""
for c in $OURS_ALL; do
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
"$MAKEBB" -o "$ROOT/$OUT" $PKGS
ls -l "$ROOT/$OUT"
