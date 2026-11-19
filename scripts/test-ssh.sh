#!/bin/sh
# Behaviour test for our ssh and scp against a real sshd on this machine.
# Needs /usr/sbin/sshd and ssh-keygen; without them the script says so and passes,
# so the battery can run anywhere.
set -u

BIN=${1:-./akmesh}
case "$BIN" in
/*) ;;
*) BIN=$(cd "$(dirname "$BIN")" && pwd)/$(basename "$BIN") ;;
esac
[ -f "$BIN" ] || { echo "no binary at $BIN" >&2; exit 2; }

if [ ! -x /usr/sbin/sshd ] || ! command -v ssh-keygen >/dev/null 2>&1; then
	echo "skipped: no sshd or ssh-keygen on this machine"
	exit 0
fi

WORK=$(mktemp -d "${TMPDIR:-/tmp}/sshtest.XXXXXX") || exit 2
PIDFILE="$WORK/sshd.pid"
cleanup() {
	[ -f "$PIDFILE" ] && kill "$(cat "$PIDFILE")" 2>/dev/null
	rm -rf "$WORK"
}
trap cleanup EXIT

fail=0
checks=0
check() {
	name=$1
	want=$2
	got=$3
	checks=$((checks + 1))
	if [ "$want" != "$got" ]; then
		fail=$((fail + 1))
		printf 'FAIL %s\n  want: %s\n  got : %s\n' "$name" "$want" "$got"
	fi
}

PORT=$((2200 + ($$ % 200)))
ssh-keygen -q -t ed25519 -N '' -f "$WORK/hostkey" || exit 2
ssh-keygen -q -t ed25519 -N '' -f "$WORK/clientkey" || exit 2
cp "$WORK/clientkey.pub" "$WORK/authorized_keys"
cat > "$WORK/sshd_config" <<EOF
Port $PORT
ListenAddress 127.0.0.1
HostKey $WORK/hostkey
PidFile $PIDFILE
AuthorizedKeysFile $WORK/authorized_keys
PermitRootLogin yes
PubkeyAuthentication yes
PasswordAuthentication no
UsePAM no
StrictModes no
LogLevel ERROR
EOF
/usr/sbin/sshd -f "$WORK/sshd_config" -E "$WORK/sshd.log" || { echo "sshd did not start"; exit 2; }
sleep 1

# Run from the work directory with a relative key name: a Windows binary cannot
# open the Linux absolute path, but it can open a relative one in its cwd (WSL
# hands it the directory over).
cd "$WORK" || exit 2

# The connection switches every call shares. ssh takes -p for the port, scp takes
# -P, exactly as the original tools do.
SSHOPTS="-o StrictHostKeyChecking=no -i clientkey -p $PORT"
SCPOPTS="-o StrictHostKeyChecking=no -i clientkey -P $PORT"
r() {
	# shellcheck disable=SC2086
	"$BIN" ssh $SSHOPTS "$@"
}
rscp() {
	# shellcheck disable=SC2086
	"$BIN" scp $SCPOPTS "$@"
}

check ssh-output "hallo" "$(r root@127.0.0.1 'echo hallo' </dev/null | tr -d '\r')"
check ssh-multiline "zwei" "$(r root@127.0.0.1 'uname -s >/dev/null; echo zwei' </dev/null | tr -d '\r')"
# The remote status must be our status, that is the whole point for an agent.
r root@127.0.0.1 'exit 7' </dev/null >/dev/null 2>&1
check ssh-exit-status "7" "$?"
r root@127.0.0.1 'test -f /etc/hostname' </dev/null >/dev/null 2>&1
check ssh-test-true "0" "$?"
r root@127.0.0.1 'test -f /gibtsnicht' </dev/null >/dev/null 2>&1
check ssh-test-false "1" "$?"
# Non-interactive: stdin is a pipe, no key prompt, stderr stays quiet.
check ssh-stdin "durchgereicht" "$(echo durchgereicht | r root@127.0.0.1 'cat' 2>&1 | tr -d '\r')"
# An unknown host without the override must fail instead of trusting blindly.
"$BIN" ssh -i clientkey -p "$PORT" root@127.0.0.1 'true' </dev/null >/dev/null 2>&1
check ssh-untrusted-host "255" "$?"

mkdir -p "$WORK/src/sub"
echo eins > "$WORK/src/a.txt"
echo zwei > "$WORK/src/sub/b.txt"
rscp src/a.txt "root@127.0.0.1:$WORK/uploaded.txt" </dev/null >/dev/null 2>&1
check scp-upload-status "0" "$?"
check scp-upload-content "eins" "$(cat "$WORK/uploaded.txt" 2>/dev/null | tr -d '\r')"

rscp -r src "root@127.0.0.1:$WORK/" </dev/null >/dev/null 2>&1
check scp-recursive-status "0" "$?"
check scp-recursive-content "zwei" "$(cat "$WORK/src/sub/b.txt" 2>/dev/null | tr -d '\r')"

mkdir -p "$WORK/back"
rscp "root@127.0.0.1:$WORK/src/a.txt" "$WORK/back/" </dev/null >/dev/null 2>&1
check scp-download-status "0" "$?"
check scp-download-content "eins" "$(cat "$WORK/back/a.txt" 2>/dev/null | tr -d '\r')"

rscp -r "root@127.0.0.1:$WORK/src" "$WORK/back/" </dev/null >/dev/null 2>&1
check scp-download-tree-status "0" "$?"
check scp-download-tree-content "zwei" "$(cat "$WORK/back/src/sub/b.txt" 2>/dev/null | tr -d '\r')"

# -p keeps the modification time in both directions, -C is refused honestly.
touch -d '2020-01-02 03:04:05' stamp.txt 2>/dev/null || touch -t 202001020304.05 stamp.txt
rscp -p stamp.txt "root@127.0.0.1:$WORK/keep.txt" </dev/null >/dev/null 2>&1
check scp-preserve-upload "0" "$?"
rscp -p "root@127.0.0.1:$WORK/keep.txt" "$WORK/keep-back.txt" </dev/null >/dev/null 2>&1
check scp-preserve-download-year "2020" "$(date -d @"$(stat -c %Y "$WORK/keep-back.txt" 2>/dev/null || echo 0)" +%Y 2>/dev/null || echo fehlt)"
rscp -C stamp.txt "root@127.0.0.1:$WORK/x.txt" </dev/null >/dev/null 2>&1
check scp-compression-refused "1" "$?"

printf '%d checks, %d failures\n' "$checks" "$fail"
[ "$fail" -eq 0 ]
