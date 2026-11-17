#!/bin/sh
# Build the acmesh ext binary (the small companion to the u-root busybox).
#
#   ./scripts/build-ext.sh linux    -> ext
#   ./scripts/build-ext.sh windows  -> ext.exe
#
# The binary is stripped: the size budget in docs/groesse.md counts on it.
set -eu

ROOT=$(cd "$(dirname "$0")/.." && pwd)
PLATFORM=${1:-linux}

case "$PLATFORM" in
windows)
	export GOOS=windows GOARCH=amd64
	OUT=ext.exe
	;;
linux)
	export GOOS=linux GOARCH=amd64
	OUT=ext
	;;
*)
	echo "usage: $0 [windows|linux]" >&2
	exit 2
	;;
esac

export GOCACHE="$ROOT/.cache"
export GOPATH="$ROOT/.gopath"
export GOTOOLCHAIN=auto
export GOFLAGS=

cd "$ROOT"
go build -trimpath -ldflags="-s -w" -o "$OUT" ./cmd/ext
SIZE=$(wc -c < "$OUT")
printf '%s: %s bytes (%s MiB)\n' "$OUT" "$SIZE" "$(awk "BEGIN{printf \"%.2f\", $SIZE/1048576}")"
