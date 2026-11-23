#!/bin/sh
# Applies the akmesh fork of mvdan.cc/sh to the module cache: $0 learns the name
# the shell was called with.
#
# Why a patch and not a copy: AGENTS.md forbids a copy of a foreign project's
# source in this repo. The change lives in docs/design/patches/, the untouched
# source comes from the module cache, and this script joins the two.
#
# What the patch does (acmesh-51b): mvdan/sh answered $0 with "gosh" whenever it
# had no filename, so every akmesh shell said "gosh" whatever it was called and
# ash's "-sh" for a login shell was out of reach. The patch reads $0 from the
# environment variable "0" when it is set; without that variable the old
# behaviour stays.
#
# The script is idempotent: it checks whether the patch is already applied and
# says so instead of failing, so a fresh module cache cannot quietly ship the
# old $0.
#
# Run by scripts/build-bb.sh and scripts/check-all.sh, or by hand:
#   ./scripts/apply-sh-fork.sh

set -eu
cd "$(dirname "$0")/.."

MOD="mvdan.cc/sh/v3"
PATCH="docs/design/patches/sh-dollar-zero.patch"
MARK=".build/sh-fork.applied"

export GOTOOLCHAIN=auto
export GOCACHE="${GOCACHE:-$PWD/.cache}"
export GOFLAGS="${GOFLAGS:--mod=mod}"

if ! grep -q "$MOD" go.mod 2>/dev/null; then
	exit 0
fi

VER=$(go list -m -f '{{.Version}}' "$MOD" 2>/dev/null | head -n 1)
[ -n "$VER" ] || { echo "fork: $MOD not resolved (run go mod download)" >&2; exit 1; }
[ -f "$PATCH" ] || { echo "fork: patch $PATCH missing" >&2; exit 1; }

DIR=$(go list -m -f '{{.Dir}}' "$MOD" 2>/dev/null | head -n 1)
[ -n "$DIR" ] && [ -d "$DIR" ] || { echo "fork: $MOD@$VER not in the module cache" >&2; exit 1; }

# Already applied? Ask the module itself: the fork is the only source that reads
# $0 from the environment.
if grep -q 'r.writeEnv.Get("0")' "$DIR/interp/vars.go" 2>/dev/null; then
	mkdir -p .build
	printf '%s %s\n' "$VER" "$(sha256sum "$PATCH" | cut -d' ' -f1)" > "$MARK"
	exit 0
fi

if ! patch -p1 --dry-run --silent -d "$DIR" < "$PATCH" >/dev/null 2>&1; then
	echo "fork: patch does not apply to $MOD@$VER - the module was updated, the fork has to be rebased" >&2
	exit 1
fi
chmod -R u+w "$DIR"
patch -p1 --silent -d "$DIR" < "$PATCH"
mkdir -p .build
printf '%s %s\n' "$VER" "$(sha256sum "$PATCH" | cut -d' ' -f1)" > "$MARK"
echo "fork: applied $PATCH to $MOD@$VER"
