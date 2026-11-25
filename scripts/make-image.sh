#!/bin/sh
# Builds the akmesh container image: one static binary, a symlink tree, an OCI
# image tagged after the product name (acmesh-35q).
#
# Why a script and not a Dockerfile: the command list must not be written down a
# second time. The names come from the binary itself - it prints them when it is
# called with a name it does not carry - so a new command is in the tree the next
# time this runs, and nothing here can drift from scripts/cmds.txt plus OURS_ALL.
#
# What lands in the image:
#   /bin/<name>          symlinks to /bin/akmesh (argv[0] is the dispatcher)
#   /bin/akmesh          the static build, stripped
#   /etc/passwd, /etc/group, /etc/hosts, /etc/resolv.conf, /etc/profile, /etc/issue
#   /etc/ssl/certs/ca-certificates.crt   copied from the host, so curl and wget
#                        can speak TLS; skipped with a note when absent
#   /tmp (1777), /proc, /sys, /dev, /root, /home  as empty directories
# /dev and /proc are filled by the container runtime, not by us: there is no mdev
# and no udev in this binary.
#
# Usage:
#   ./scripts/make-image.sh              # build, assemble, import (one binary)
#   ./scripts/make-image.sh --split      # the image with the split binaries
#   TAG=akmesh:test ./scripts/make-image.sh
#   PROFILE=services-off ./scripts/make-image.sh   # any profile from scripts/
#   ./scripts/make-image.sh --verify-only          # only run the checks
#
# Two image variants on purpose (Martin, 25.11.2026): the one-binary image is
# what a container usually wants, because its /bin is one file of 21.5 MiB; the
# split image is what shows the size for a single job - /bin/akmesh-docs is
# 5.8 MiB, and a harness that only reads documents never loads the rest.
#
# Requires podman. Nothing is pulled: the rootfs is tarred here and imported.

set -eu
cd "$(dirname "$0")/.."
ROOT=$PWD

PROFILE=${PROFILE:-full}
SPLIT=0
case "${1:-}" in
--split) SPLIT=1 ;;
esac
SPLIT_BINS="akmesh-docs akmesh-write akmesh-net akmesh-shell"
if [ "$SPLIT" -eq 1 ]; then
	TAG=${TAG:-${AKMESH_NAME:-akmesh}-split:latest}
else
	TAG=${TAG:-${AKMESH_NAME:-akmesh}:latest}
fi
IMAGE_DIR="$ROOT/.build/image"
ROOTFS="$IMAGE_DIR/rootfs"
TAR="$IMAGE_DIR/rootfs.tar"
BIN="$ROOT/akmesh"
VERIFY_ONLY=0
[ "${1:-}" = "--verify-only" ] && VERIFY_ONLY=1
SPLIT_ARG=${SPLIT:-0}

command -v podman >/dev/null 2>&1 || {
	echo "make-image: podman is required" >&2
	exit 2
}

# 1. Build. Static, because a musl loader is not in the image: a dynamic binary
# dies with "not found" and that looks like a missing file.
if [ "$VERIFY_ONLY" -eq 0 ]; then
	echo "make-image: building the static binary (profile $PROFILE)"
	CGO_ENABLED=0 ./scripts/build-bb.sh linux "$PROFILE"
	if [ "$SPLIT" -eq 1 ]; then
		for p in $SPLIT_BINS; do
			echo "make-image: building $p (static)"
			CGO_ENABLED=0 GOTOOLCHAIN=auto go build -trimpath \
				-ldflags "-s -w -buildid=" -o "$ROOT/$p" "./cmd/$p"
		done
	fi
fi

[ -f "$BIN" ] || {
	echo "make-image: $BIN is missing; build it first" >&2
	exit 2
}

# 2. Assemble the rootfs from scratch every time: a leftover symlink for a
# command that no longer exists would ship a broken name.
rm -rf "$ROOTFS" "$TAR"
mkdir -p "$ROOTFS/bin" "$ROOTFS/etc/ssl/certs" "$ROOTFS/tmp" "$ROOTFS/proc" \
	"$ROOTFS/sys" "$ROOTFS/dev" "$ROOTFS/root" "$ROOTFS/home"

cp "$BIN" "$ROOTFS/bin/akmesh"
chmod 0755 "$ROOTFS/bin/akmesh"

# The names are what the binary answers to, not a list kept next to it.
NAMES=$(set +e; "$BIN" __akmesh_list_commands__ 2>&1 | sed -n 's/^ - //p')
COUNT=$(printf '%s\n' "$NAMES" | grep -c . || true)
[ "$COUNT" -gt 50 ] || {
	echo "make-image: only $COUNT command names read from the binary; the list changed shape" >&2
	exit 2
}
for name in $NAMES; do
	ln -sf akmesh "$ROOTFS/bin/$name"
done
echo "make-image: $COUNT names linked into /bin"

# The split layout: each binary has its own name set, and the symlinks for a
# split binary replace the ones from the full binary, so the same name is
# answered by the binary that owns it.
if [ "$SPLIT" -eq 1 ]; then
	for p in $SPLIT_BINS; do
		[ -f "$ROOT/$p" ] || { echo "make-image: $p is missing; build it first" >&2; exit 2; }
		cp "$ROOT/$p" "$ROOTFS/bin/$p"
		chmod 0755 "$ROOTFS/bin/$p"
		# The names come from the binary too: it prints them when started
		# without a command.
		snames=$(set +e; "$ROOT/$p" 2>&1 | sed -n 's/^commands: //p')
		[ -n "$snames" ] || {
			echo "make-image: $p names no commands; the dispatcher changed shape" >&2
			exit 2
		}
		for name in $snames; do
			ln -sf "$p" "$ROOTFS/bin/$name"
		done
		echo "make-image: $p with $(printf '%s\n' $snames | grep -c .) names linked"
	done
fi

# 3. The few files a shell session expects to find.
cat > "$ROOTFS/etc/passwd" <<'EOF'
root:x:0:0:root:/root:/bin/sh
nobody:x:65534:65534:nobody:/nonexistent:/bin/sh
EOF

# /etc/shadow: without it login cannot check a password and passwd cannot write
# one - the file is not optional for a system that offers a console login. Root is
# locked ("!") unless AKMESH_ROOT_HASH carries a hash, e.g. from
# "akmesh mkpasswd -m sha-512"; a locked root is honest, login says so instead of
# failing on a missing file (acmesh-a4d).
cat > "$ROOTFS/etc/shadow" <<SHADOW
root:${AKMESH_ROOT_HASH:-!}:19000:0:99999:7:::
SHADOW
chmod 0400 "$ROOTFS/etc/shadow"
cat > "$ROOTFS/etc/group" <<'EOF'
root:x:0:
nobody:x:65534:
EOF
# The console banner getty prints before the login prompt: \n is the host name,
# \l the terminal. Without this file a login prompt would show nothing about
# where one is (acmesh-bu1).
cat > "$ROOTFS/etc/issue" <<'EOF'
\n (akmesh) \l

EOF
cat > "$ROOTFS/etc/hosts" <<'EOF'
127.0.0.1	localhost
::1		localhost
EOF
cat > "$ROOTFS/etc/resolv.conf" <<'EOF'
# The container runtime replaces this file when it hands out DNS servers.
nameserver 1.1.1.1
EOF
cat > "$ROOTFS/etc/profile" <<'EOF'
# Login shell start file; only read by "sh -l" (acmesh-7tx).
PATH=/bin
export PATH
EOF

CA=""
for candidate in /etc/ssl/certs/ca-certificates.crt /etc/pki/tls/certs/ca-bundle.crt \
	/etc/ssl/cert.pem /etc/ca-certificates/extracted/tls-ca-bundle.pem; do
	if [ -f "$candidate" ]; then
		CA=$candidate
		break
	fi
done
if [ -n "$CA" ]; then
	cp "$CA" "$ROOTFS/etc/ssl/certs/ca-certificates.crt"
	echo "make-image: CA bundle from $CA ($(wc -c < "$ROOTFS/etc/ssl/certs/ca-certificates.crt") bytes)"
else
	echo "make-image: no CA bundle on this host; curl and wget will fail on https" >&2
fi

chmod 1777 "$ROOTFS/tmp"

# 4. Import. tar with root ownership, then the few Dockerfile instructions the
# image needs: PATH, a default command and the product name as a label.
if [ "$VERIFY_ONLY" -eq 0 ]; then
	tar -C "$ROOTFS" --owner=0 --group=0 -cf "$TAR" .
	echo "make-image: importing $TAR as $TAG"
	podman import \
		--change "ENV PATH=/bin" \
		--change "CMD [\"/bin/sh\"]" \
		--change "LABEL org.opencontainers.image.title=${TAG%%:*}" \
		--change "LABEL org.opencontainers.image.description=akmesh as a minimal userland" \
		"$TAR" "$TAG" > /dev/null
fi

# 5. Prove that it answers. No bind mounts, no PATH juggling: what the image
# carries is all it has.
echo "make-image: verifying $TAG"
fail=0
check() {
	label=$1
	shift
	got=$(podman run --rm "$TAG" "$@" 2>&1) || true
	printf '  %-28s %s\n' "$label" "$(printf '%s' "$got" | tr '\n' ' ' | cut -c1-70)"
}
got=$(podman run --rm "$TAG" sh -c 'ls / | tr "\n" " "; echo; printf "b\na\n" | sort' 2>&1) || {
	echo "make-image: the image does not run: $got" >&2
	exit 1
}
echo "  ls / + sort:"
printf '%s\n' "$got" | sed 's/^/    /'
printf '%s\n' "$got" | grep -qx 'a' || { echo "make-image: sort did not answer" >&2; fail=1; }
printf '%s\n' "$got" | grep -qx 'b' || { echo "make-image: sort did not answer" >&2; fail=1; }

check "sh --version path" sh -c 'echo $PATH'
check "uname" uname -s
check "cat /etc/passwd" sh -c 'head -1 /etc/passwd'
check "date" date -u +%Y-%m-%d
if [ "$SPLIT" -eq 1 ]; then
	# Each name must reach the binary that owns it; a symlink to the wrong
	# binary answers "unknown command" and that is the failure this proves.
	for pair in any2md=akmesh-docs md2pdf=akmesh-write curl=akmesh-net sh=akmesh-shell cat=akmesh; do
		name=${pair%%=*}
		want=${pair#*=}
		got_link=$(podman run --rm "$TAG" sh -c "readlink /bin/$name" 2>&1) || true
		printf '  %-28s %s\n' "/bin/$name ->" "$got_link"
		[ "$got_link" = "$want" ] || { echo "make-image: /bin/$name points at $got_link, want $want" >&2; fail=1; }
	done
	check "any2md via symlink" any2md -h
	check "md2pdf via symlink" md2pdf -h
fi
check "awk" sh -c 'echo "x 2" | awk "{print \$2}"'
check "grep" sh -c 'echo hallo | grep -c hallo'
# ps must see this container's own process, not nothing: the header alone is a
# failure (u-root's tail has no -2, so no line arithmetic here).
pslines=$(podman run --rm "$TAG" sh -c 'ps -e | wc -l' 2>&1) || true
printf '  %-28s %s\n' "ps -e lines" "$pslines"
[ "${pslines:-0}" -gt 1 ] || { echo "make-image: ps -e saw no process" >&2; fail=1; }

[ "$fail" -eq 0 ] || exit 1

# 6. Numbers. The binary reports its own size; the image size and the two times
# are measured here, and the command that produced them goes into
# docs/measurements.md. Two different numbers, because they answer two different
# questions: what the runtime costs per container, and what one akmesh invocation
# costs inside it.
echo "make-image: image size $(podman images --format '{{.Size}}' "$TAG" | head -1)"
if command -v bash >/dev/null 2>&1; then
	read -r container_ms exec_ms <<EOF
$(bash -c '
		t0=$EPOCHREALTIME
		for _ in 1 2 3 4 5; do podman run --rm "'"$TAG"'" true >/dev/null 2>&1; done
		t1=$EPOCHREALTIME
		podman run --rm "'"$TAG"'" sh -c '"'"'for i in $(seq 50); do /bin/true; done'"'"' >/dev/null 2>&1
		t2=$EPOCHREALTIME
		awk -v a="$t0" -v b="$t1" -v c="$t2" '"'"'BEGIN{
			printf "%.0f %.2f\n", (b-a)*1000/5, (c-b)*1000/50
		}'"'"'
')
EOF
	echo "make-image: 5 container starts, ${container_ms:-?} ms each (runtime included)"
	echo "make-image: 50 execs of /bin/true inside one container, ${exec_ms:-?} ms each"
else
	echo "make-image: bash is absent, timing skipped (no sub-second clock in sh)" >&2
fi
echo "make-image: done, tag $TAG"
