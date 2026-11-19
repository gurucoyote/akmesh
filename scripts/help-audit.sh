#!/bin/sh
# Every command the binary carries, asked for --help and -h.
#
#   ./scripts/help-audit.sh ./akmesh        (or ./akmesh.exe)
#
# Prints one line per call: name, flag, exit code, size, lines and the first
# line. A command that answers with "unknown option" or with an error instead
# of a usage text is the finding this script exists to show (acmesh-py8).
set -u

BIN=${1:-./akmesh}
if [ ! -x "$BIN" ]; then
	echo "usage: $0 BIN" >&2
	exit 2
fi

TMP=${TMPDIR:-/tmp}/help-audit.txt
# Two sources: what we build ourselves (OURS_ALL in build-bb.sh) and the u-root
# commands we keep (cmds.txt). The dispatcher does not print a list of its own.
HERE=$(dirname "$0")
OURS=$(sed -n 's/^OURS_ALL="\(.*\)"$/\1/p' "$HERE/build-bb.sh" 2>/dev/null)
UROOT=$(awk 'NF && $1 !~ /^#/ {print}' "$HERE/cmds.txt" 2>/dev/null | tr ' ' '\n')
CMDS=$(printf '%s\n%s\n' "$OURS" "$UROOT" | tr ' ' '\n' | awk 'NF && !seen[$0]++' | tr '\n' ' ')
if [ -z "$CMDS" ]; then
	echo "no command list found in scripts/cmds.txt and scripts/build-bb.sh" >&2
	exit 2
fi

probe() {
	cmd=$1
	flag=$2
	timeout 5 "$BIN" "$cmd" "$flag" </dev/null >"$TMP" 2>&1
	code=$?
	bytes=$(wc -c <"$TMP")
	lines=$(wc -l <"$TMP")
	first=$(head -1 "$TMP" | tr -d '\r' | cut -c1-56)
	printf '%-14s %-7s code=%-3s %6sB %3sZ | %s\n' "$cmd" "$flag" "$code" "$bytes" "$lines" "${first:-<leer>}"
}

echo "# help-audit for $BIN, $(date -u +%Y-%m-%dT%H:%M:%SZ)"
for c in $CMDS; do
	probe "$c" --help
	probe "$c" -h
done
