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

echo "== goldmark-pdf fork"
./scripts/apply-goldmark-pdf-fork.sh
echo "== go-sed fork"
./scripts/apply-go-sed-fork.sh
./scripts/apply-sh-fork.sh

echo "== check-licences"
./scripts/check-licences.sh

# -race after the plain run: a data race in the xargs test was invisible without
# it (acmesh-vyc). Keep this script single-threaded - a parallel go run makes
# WSL fail to start the .exe with "accept4 failed 110" and the Windows battery
# reports false errors.
echo "== go test -race ./internal/..."
GOTOOLCHAIN=auto GOCACHE="${GOCACHE:-$PWD/.cache}" go test -race -count=1 ./internal/...

echo "== go test ./..."
GOTOOLCHAIN=auto GOCACHE="${GOCACHE:-$PWD/.cache}" go test ./...

echo "== build linux"
GOTOOLCHAIN=auto ./scripts/build-bb.sh linux

echo "== battery linux"
./scripts/test-akmesh.sh ./akmesh

echo "== help audit linux"
./scripts/help-audit.sh --check ./akmesh

echo "== build windows"
GOTOOLCHAIN=auto ./scripts/build-bb.sh windows

echo "== battery windows"
./scripts/test-akmesh.sh ./akmesh.exe

echo "== build ext (lean profile)"
GOTOOLCHAIN=auto ./scripts/build-bb.sh linux ext

echo "all green"
