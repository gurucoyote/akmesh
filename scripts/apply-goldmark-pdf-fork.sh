#!/bin/sh
# Applies the mager goldmark-pdf fork to the module cache.
#
# Why a patch and not a copy: AGENTS.md forbids a copy of a foreign project's
# source in this repo. The changes live in docs/design/patches/, the untouched
# source comes from the module cache, and this script joins the two.
#
# Why not third_party/vendor: the module would then be a copy again, and a
# vendor tree drifts silently from the version in go.mod.
#
# The script is idempotent: it checks whether the patch is already applied and
# says so instead of failing. The build calls it before compiling, so a fresh
# cache cannot quietly ship the unlite version with chroma (12,2 MB instead of
# 9,1 MB, which the size check alone would not notice at 21 MB total).
#
# Run by scripts/build-bb.sh and scripts/check-all.sh, or by hand:
#   ./scripts/apply-goldmark-pdf-fork.sh

set -eu
cd "$(dirname "$0")/.."

MOD="github.com/stephenafamo/goldmark-pdf"
PATCH="docs/design/patches/goldmark-pdf-0.4.2-no-chroma-no-google-fonts.patch"
MARK=".build/goldmark-pdf-fork.applied"

export GOTOOLCHAIN=auto
export GOCACHE="${GOCACHE:-$PWD/.cache}"
export GOFLAGS="${GOFLAGS:--mod=mod}"

# Nothing to do when the module is not part of the build yet.
if ! grep -q "$MOD" go.mod 2>/dev/null; then
	exit 0
fi

VER=$(go list -m -f '{{.Version}}' "$MOD" 2>/dev/null | head -n 1)
[ -n "$VER" ] || { echo "fork: $MOD not resolved (run go mod download)" >&2; exit 1; }
[ -f "$PATCH" ] || { echo "fork: patch $PATCH missing" >&2; exit 1; }

MODCACHE=$(go env GOMODCACHE)
DIR="$MODCACHE/$MOD@$VER"
[ -d "$DIR" ] || { echo "fork: $DIR not in the module cache" >&2; exit 1; }

# Already applied? Ask the module itself, not patch: busybox patch and GNU patch
# disagree about a reverse dry run on an applied patch, and the build calls this
# script through sh. The marker below is a line only the fork's fonts.go has.
if grep -q "google fonts are not built in" "$DIR/fonts.go" 2>/dev/null; then
	# Record it, so the build report can say the fork is in force.
	mkdir -p .build
	printf '%s %s\n' "$VER" "$(sha256sum "$PATCH" | cut -d' ' -f1)" > "$MARK"
	exit 0
fi

# Applying needs write access; the cache is read-only by design.
if ! patch -p1 --dry-run --silent -d "$DIR" < "$PATCH" >/dev/null 2>&1; then
	echo "fork: patch does not apply to $MOD@$VER - the module was updated, the fork has to be rebased" >&2
	exit 1
fi
chmod -R u+w "$DIR"
patch -p1 --silent -d "$DIR" < "$PATCH"
mkdir -p .build
printf '%s %s\n' "$VER" "$(sha256sum "$PATCH" | cut -d' ' -f1)" > "$MARK"
echo "fork: applied $PATCH to $MOD@$VER"
