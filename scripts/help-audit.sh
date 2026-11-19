#!/bin/sh
# Every command the binary carries, asked for --help and -h.
#
#   ./scripts/help-audit.sh ./akmesh             (or ./akmesh.exe)
#   ./scripts/help-audit.sh --check ./akmesh     gate: exit 1 on a regression
#
# Prints one line per call: name, flag, exit code, size, lines and the first
# line. A command that answers with "unknown option" or with an error instead
# of a usage text is the finding this script exists to show (acmesh-py8).
#
# --check is the gate check-all.sh runs: every command must answer --help with
# a usage line, except the u-root commands listed in KNOWN_NO_HELP below. Those
# are a finding in docs/measurements.md, not a silent rebuild: they live in
# u-root and have no place of ours to hook into.
set -u

CHECK=0
if [ "${1:-}" = "--check" ]; then
	CHECK=1
	shift
fi

BIN=${1:-./akmesh}
if [ ! -x "$BIN" ]; then
	echo "usage: $0 [--check] BIN" >&2
	exit 2
fi

# u-root commands that still answer --help with something other than help.
# Listed so that a NEW command without help fails the gate instead of hiding
# in a big number (acmesh-py8).
KNOWN_NO_HELP=" mv rm mkdir gzip uptime printenv man "
# Commands whose help does not end with 0. GNU /bin/false reports its usage and
# still exits 1 (measured 19.11.2026); everything else ends with 0.
KNOWN_NONZERO=" false "
# On Windows tracedeps refuses the whole command ("tracing is built for
# linux/amd64 only", exit 2), so it has no help to give there. The Linux gate in
# check-all.sh still requires its usage text.
case $BIN in
*.exe) KNOWN_NO_HELP="$KNOWN_NO_HELP tracedeps " ;;
esac

# A file per run: two audits in parallel (linux and windows, or a run beside a
# battery) shared one name and interleaved their output, which looked like a
# command answering with another command's help.
SCRATCH=$(mktemp -d "${TMPDIR:-/tmp}/help-audit.XXXXXX") || exit 2
trap 'rm -rf "$SCRATCH"' EXIT
TMP=$SCRATCH/out.txt
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

[ "$CHECK" = 1 ] || exit 0

# The gate reads the same probe once more and judges it instead of printing it.
missing=""
for c in $CMDS; do
	case $KNOWN_NO_HELP in
	*" $c "*) continue ;;
	esac
	timeout 5 "$BIN" "$c" --help </dev/null >"$TMP" 2>&1
	code=$?
	first=$(head -1 "$TMP" | tr -d '\r')
	case $first in
	usage*|Usage*) ;;
	*) missing="$missing $c(keine usage)" ;;
	esac
	if [ "$code" != 0 ]; then
		case $KNOWN_NONZERO in
		*" $c "*) ;;
		*) missing="$missing $c(exit=$code)" ;;
		esac
	fi
done
if [ -n "$missing" ]; then
	echo "help-audit: no usage answer or a non-zero exit from:$missing" >&2
	exit 1
fi
echo "help-audit: every command answers --help except the documented u-root gap"
