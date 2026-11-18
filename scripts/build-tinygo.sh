#!/bin/sh
# Build the fs profile with TinyGo instead of Go, for a size comparison.
#
# Two things stand in the way and both are handled here:
#   - makebb drives "go build" itself, so the source is generated first
#     (ACMESH_GEN_DIR) and compiled with tinygo afterwards;
#   - our module and its dependencies declare go 1.27, TinyGo ships Go 1.25.7,
#     so the generated go.mod and vendor/modules.txt are lowered to the Go
#     version TinyGo actually carries. Nothing in the repository changes.
#
#   ./scripts/build-tinygo.sh [linux|windows] [profile]
set -eu

PLATFORM=${1:-linux}
PROFILE=${2:-fs}
TINYGO=${TINYGO:-/root/3rd/tinygo44/bin/tinygo}
ROOT=$(cd "$(dirname "$0")/.." && pwd)
GEN="$ROOT/.build/tinygo-$PROFILE"

if [ ! -x "$TINYGO" ]; then
	echo "no tinygo at $TINYGO" >&2
	exit 2
fi

# The Go version TinyGo carries, e.g. "go1.25.7".
GOVER=$("$TINYGO" version | sed -n 's/.*using go version \(go[0-9.]*\).*/\1/p')
if [ -z "$GOVER" ]; then
	GOVER=go1.25.7
fi
echo "tinygo: $TINYGO"
echo "go inside tinygo: $GOVER"

rm -rf "$GEN"
echo "--- generating the multicall source for profile $PROFILE"
ACMESH_GEN_DIR="$GEN" sh "$ROOT/scripts/build-bb.sh" "$PLATFORM" "$PROFILE"

# Lower the version requirements to what TinyGo carries.
SHORT=${GOVER#go}
python3 - "$GEN" "$SHORT" <<'PY'
import os
import re
import sys

gen, short = sys.argv[1], sys.argv[2]

gomod = os.path.join(gen, "go.mod")
text = open(gomod, encoding="utf-8").read()
text = re.sub(r"(?m)^go \d+\.\d+(\.\d+)?$", "go " + short, text)
open(gomod, "w", encoding="utf-8").write(text)

mods = os.path.join(gen, "vendor", "modules.txt")
if os.path.exists(mods):
    text = open(mods, encoding="utf-8").read()
    text = re.sub(r"^(## explicit; go )\d+\.\d+(\.\d+)?$", r"\g<1>" + short, text, flags=re.M)
    open(mods, "w", encoding="utf-8").write(text)
print("lowered to go", short)
PY

echo "--- compiling with tinygo"
cd "$GEN"
export GOFLAGS=
export GOWORK=off
export GOTOOLCHAIN=local
case "$PLATFORM" in
windows) export GOOS=windows GOARCH=amd64; OUT="$ROOT/acmesh-$PROFILE-tinygo.exe" ;;
*) export GOOS=linux GOARCH=amd64; OUT="$ROOT/acmesh-$PROFILE-tinygo" ;;
esac

"$TINYGO" build -no-debug -panic=trap -o "$OUT" .
ls -l "$OUT"
