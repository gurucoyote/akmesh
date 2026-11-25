#!/bin/sh
# Lay out the /bin links for every command name the akmesh binary knows.
#
# Why this exists as its own script: the image build lays /bin out once, from the
# name list the binary prints. When a newer binary is copied into a running
# distribution instead of importing a new image, a name it brings along has no
# link yet, and the shell answers "not found" although the command is there
# (measured 25.11.2026: "clear" was unknown after exactly that swap). Run this
# inside the distribution as root, after replacing /bin/akmesh.
#
#   akmesh-wsl-links [/bin/akmesh] [/bin]
set -e
BIN=${1:-/bin/akmesh}
DIR=${2:-/bin}
if [ ! -x "$BIN" ]; then
	echo "akmesh-wsl-links: $BIN is not executable" >&2
	exit 1
fi
BASE=$(basename "$BIN")
COUNT=0
for name in $(set +e; "$BIN" __akmesh_list_commands__ 2>&1 | sed -n 's/^ - //p'); do
	[ "$name" = "$BASE" ] && continue
	target="$DIR/$name"
	if [ -e "$target" ] && [ ! -L "$target" ]; then
		# A real file stays: it is a package's own command (apk pulls in own
		# binaries), and overwriting it would take that away.
		continue
	fi
	if [ -L "$target" ] && [ "$(readlink "$target")" = "$BASE" ]; then
		continue
	fi
	# Two calls, not "-sf": our own ln refuses combined short flags
	# (acmesh-kf91), and this script runs with that shell.
	rm -f "$target"
	ln -s "$BASE" "$target"
	COUNT=$((COUNT + 1))
done
echo "akmesh-wsl-links: $COUNT new links in $DIR (from $BIN)"
