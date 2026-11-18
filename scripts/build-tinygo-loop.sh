#!/bin/sh
# Compiles the generated multicall with TinyGo and drops the u-root commands that
# TinyGo's standard library cannot carry. Every drop is logged, so the report says
# exactly which commands the TinyGo build is missing.
set -u
GEN=/root/acmesh/.build/tgfs
OUT=/root/acmesh/acmesh-fs-tinygo
TG=/root/3rd/tinygo44/bin/tinygo
LOG=/root/acmesh/.build/tgfs-dropped.txt
: > "$LOG"

cd "$GEN" || exit 2
export GOWORK=off GOTOOLCHAIN=local GOFLAGS=

i=0
while [ $i -lt 25 ]; do
	i=$((i + 1))
	err=$("$TG" build -no-debug -panic=trap -tags=nonet,tinygo.enable -o "$OUT" . 2>&1)
	if [ -z "$err" ]; then
		echo "=== built after $((i - 1)) drops"
		break
	fi
	# Only a failing u-root command is dropped; anything else stops the loop.
	cmd=$(echo "$err" | sed -n 's|^vendor/github.com/u-root/u-root/cmds/core/\([a-z0-9_-]*\)/.*|\1|p' | head -1)
	if [ -z "$cmd" ]; then
		echo "=== stopped on an error that is not a command:"
		echo "$err" | head -8
		break
	fi
	echo "drop $cmd" | tee -a "$LOG"
	sed -i "/cmds\/core\/$cmd\"/d" main.go
done

echo "=== dropped commands:"
cat "$LOG" 2>/dev/null || echo "(none)"
ls -l "$OUT" 2>/dev/null | awk '{print $5" Bytes"}'
