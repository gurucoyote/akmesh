#!/bin/sh
# Everything a change has to survive before it is called done.
#
# go test alone says nothing about the shipped binary: the battery runs the
# built binary on both targets, which is where the u-root commands and our own
# per-target code actually get exercised (acmesh-93y).
#
# Run from the repository root: ./scripts/check-all.sh
set -e
cd "$(dirname "$0")/.."

echo "== go test ./..."
GOTOOLCHAIN=auto GOCACHE="${GOCACHE:-$PWD/.cache}" go test ./...

echo "== build linux"
GOTOOLCHAIN=auto ./scripts/build-bb.sh linux

echo "== battery linux"
./scripts/test-acmesh.sh ./acmesh

echo "== build windows"
GOTOOLCHAIN=auto ./scripts/build-bb.sh windows

echo "== battery windows"
./scripts/test-acmesh.sh ./acmesh.exe

echo "== build ext (lean profile)"
GOTOOLCHAIN=auto ./scripts/build-bb.sh linux ext

echo "all green"
