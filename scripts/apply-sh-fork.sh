#!/bin/sh
# Applies the akmesh fork of mvdan.cc/sh to the module cache.
#
# Why patches and not a copy: AGENTS.md forbids a copy of a foreign project's
# source in this repo. The changes live in docs/design/patches/, the untouched
# source comes from the module cache, and this script joins the two.
#
# The patches, in order:
#   sh-dollar-zero.patch  $0 is the name the shell was called with, not "gosh"
#                         (acmesh-51b): it is read from the environment "0".
#   sh-bg-pid.patch       $! is the process id of the last background command,
#                         not a job number "g1", so `cmd & p=$!; kill $p` works
#                         (acmesh-tms).
#
# The script is idempotent: a patch already in the module is recognised by its
# reverse applying cleanly, so a fresh module cache cannot quietly ship the old
# behaviour and a second run does not fail.
#
# Run by scripts/build-bb.sh and scripts/check-all.sh, or by hand:
#   ./scripts/apply-sh-fork.sh

set -eu
cd "$(dirname "$0")/.."

MOD="mvdan.cc/sh/v3"
PATCHES="docs/design/patches/sh-dollar-zero.patch docs/design/patches/sh-bg-pid.patch"
MARK=".build/sh-fork.applied"

export GOTOOLCHAIN=auto
export GOCACHE="${GOCACHE:-$PWD/.cache}"
export GOFLAGS="${GOFLAGS:--mod=mod}"

if ! grep -q "$MOD" go.mod 2>/dev/null; then
	exit 0
fi

VER=$(go list -m -f '{{.Version}}' "$MOD" 2>/dev/null | head -n 1)
[ -n "$VER" ] || { echo "fork: $MOD not resolved (run go mod download)" >&2; exit 1; }

DIR=$(go list -m -f '{{.Dir}}' "$MOD" 2>/dev/null | head -n 1)
[ -n "$DIR" ] && [ -d "$DIR" ] || { echo "fork: $MOD@$VER not in the module cache" >&2; exit 1; }

SUM=""
for PATCH in $PATCHES; do
	[ -f "$PATCH" ] || { echo "fork: patch $PATCH missing" >&2; exit 1; }
	if patch -p1 --dry-run --silent -R -d "$DIR" < "$PATCH" >/dev/null 2>&1; then
		# Already in the module: the reverse patch applies.
		SUM="$SUM $(sha256sum "$PATCH" | cut -d' ' -f1)"
		continue
	fi
	if ! patch -p1 --dry-run --silent -d "$DIR" < "$PATCH" >/dev/null 2>&1; then
		echo "fork: $PATCH does not apply to $MOD@$VER - the module was updated, the fork has to be rebased" >&2
		exit 1
	fi
	chmod -R u+w "$DIR"
	patch -p1 --silent -d "$DIR" < "$PATCH"
	SUM="$SUM $(sha256sum "$PATCH" | cut -d' ' -f1)"
	echo "fork: applied $PATCH to $MOD@$VER"
done

mkdir -p .build
printf '%s %s\n' "$VER" "$SUM" > "$MARK"
