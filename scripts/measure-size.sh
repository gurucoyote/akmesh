#!/bin/sh
# Measures what a build actually weighs, so the numbers in docs/measurements.md
# come from a command and not from memory (acmesh-94e.15, acmesh-mq1).
#
#   ./scripts/measure-size.sh                 # the repository binaries
#   ./scripts/measure-size.sh probe base64 wc # a throwaway binary with only those
#
# A probe builds into /tmp and never touches akmesh or akmesh.exe.
set -eu
ROOT=$(cd "$(dirname "$0")/.." && pwd)
export GOCACHE="$ROOT/.cache" GOPATH="$ROOT/.gopath" GOTOOLCHAIN=auto
export GOWORK="$ROOT/.build/go.work"
export CGO_ENABLED=0

mode=${1:-binaries}
case "$mode" in
binaries)
	for f in "$ROOT/akmesh" "$ROOT/akmesh.exe"; do
		[ -f "$f" ] || continue
		printf '%s %s bytes\n' "$(basename "$f")" "$(stat -c %s "$f")"
	done
	;;
probe)
	shift
	[ $# -gt 0 ] || { echo "probe needs at least one command name" >&2; exit 2; }
	pkgs=""
	for name in "$@"; do
		if [ -d "$ROOT/cmd/$name" ]; then
			pkgs="$pkgs $ROOT/cmd/$name"
		elif [ -d "$ROOT/.build/uroot/cmds/core/$name" ]; then
			pkgs="$pkgs ./cmds/core/$name"
		else
			echo "no package for $name" >&2
			exit 2
		fi
	done
	export GOOS=linux GOARCH=amd64
	out=/tmp/akmesh-probe-$$
	cd "$ROOT/.build/uroot"
	# shellcheck disable=SC2086
	"$GOPATH/bin/makebb" -o "$out" $pkgs >/dev/null
	printf 'probe (%s) %s bytes\n' "$*" "$(stat -c %s "$out")"
	rm -f "$out"
	;;
*)
	echo "usage: $0 [binaries|probe NAME...]" >&2
	exit 2
	;;
esac
