#!/bin/sh
# Applies the akmesh Go.Sed fork to the module cache: GNU sed's reading of a
# regular expression instead of the library's Go-flavoured one.
#
# Why a patch and not a copy: AGENTS.md forbids a copy of a foreign project's
# source in this repo. The changes live in docs/design/patches/, the untouched
# source comes from the module cache, and this script joins the two.
#
# What the patch does (measured against GNU sed 4.9 in a Debian container,
# see docs/measurements.md, "sed gegen GNU sed"):
#   - a basic regular expression is translated, so \( \) \+ \? \| \{ work
#   - the replacement understands \1..\9 and & ($1 keeps working)
#   - the leftmost, longest match wins, as POSIX wants it
#   - "." matches a newline, as it does in sed's multi-line pattern space
#   - -E/-r scripts go through NewExtended/NewQuietExtended untranslated
#
# The script is idempotent: it checks whether the patch is already applied and
# says so instead of failing, so a fresh module cache cannot quietly ship the
# Go-flavoured engine.
#
# Run by scripts/build-bb.sh and scripts/check-all.sh, or by hand:
#   ./scripts/apply-go-sed-fork.sh

set -eu
cd "$(dirname "$0")/.."

MOD="github.com/rwtodd/Go.Sed"
PATCH="docs/design/patches/go-sed-gnu-semantics.patch"
MARK=".build/go-sed-fork.applied"

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

# The cache directory of this module is escaped (!go.!sed@... for Go.Sed), so
# ask the toolchain for the path instead of gluing the module path together.
DIR=$(go list -m -f '{{.Dir}}' "$MOD" 2>/dev/null | head -n 1)
[ -n "$DIR" ] && [ -d "$DIR" ] || { echo "fork: $MOD@$VER not in the module cache" >&2; exit 1; }

# Already applied? Ask the module itself, not patch: busybox patch and GNU patch
# disagree about a reverse dry run on an applied patch, and the build calls this
# script through sh. The marker is a file only the fork has.
if grep -q "longestMatcher gives a pattern POSIX" "$DIR/sed/substitution.go" 2>/dev/null; then
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
