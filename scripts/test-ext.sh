#!/bin/sh
# Behaviour check for the acmesh ext binary.
#
#   ./scripts/test-ext.sh /path/to/ext        (Linux)
#   ./scripts/test-ext.sh /path/to/ext.exe    (Windows, run from WSL)
#
# Every case states what it expects; a failure prints the case and the output.
# This proves behaviour, not existence: a build that compiles and does nothing
# fails here.
set -u

BIN=${1:-./ext}
# The checks run inside a scratch directory, so the binary path must be
# absolute before we change directory.
case "$BIN" in
/*) ;;
*) BIN=$(cd "$(dirname "$BIN")" 2>/dev/null && pwd)/$(basename "$BIN") ;;
esac
if [ ! -f "$BIN" ]; then
	echo "no binary at $BIN" >&2
	exit 2
fi

WORK=$(mktemp -d "${TMPDIR:-/tmp}/exttest.XXXXXX") || exit 2
trap 'rm -rf "$WORK"' EXIT
cd "$WORK" || exit 2

fails=0
checks=0

check() {
	name=$1
	want=$2
	got=$3
	checks=$((checks + 1))
	if [ "$want" != "$got" ]; then
		fails=$((fails + 1))
		printf 'FAIL %s\n  want: %s\n  got : %s\n' "$name" "$want" "$got"
	fi
}

checkcode() {
	name=$1
	want=$2
	got=$3
	checks=$((checks + 1))
	if [ "$want" != "$got" ]; then
		fails=$((fails + 1))
		printf 'FAIL %s\n  want exit %s, got %s\n' "$name" "$want" "$got"
	fi
}

# text tools
check basename "$(printf 'foo')" "$("$BIN" basename /usr/local/bin/foo.txt .txt | tr -d '\r')"
check dirname "/usr/local/bin" "$("$BIN" dirname /usr/local/bin/foo.txt | tr -d '\r')"
check cut-fields "b" "$(printf 'a:b:c\n' | "$BIN" cut -d: -f2 | tr -d '\r')"
check cut-chars "bcd" "$(printf 'abcdef\n' | "$BIN" cut -c2-4 | tr -d '\r')"
check tac "3 2 1" "$(printf '1\n2\n3\n' | "$BIN" tac | tr -d '\r' | tr '\n' ' ' | sed 's/ $//')"
check nl-numbered "     1	a" "$(printf 'a\n' | "$BIN" nl | tr -d '\r')"

# hashing against the system tool when there is one
printf 'hallo\n' > h.txt
if command -v sha256sum >/dev/null 2>&1; then
	ours=$("$BIN" sha256sum h.txt | tr -d '\r' | cut -d' ' -f1)
	sys=$(sha256sum h.txt | cut -d' ' -f1)
	check sha256 "$sys" "$ours"
fi

# sed and awk
check sed-sub "erde" "$(printf 'hallo\n' | "$BIN" sed 's/hallo/erde/' | tr -d '\r')"
check sed-quiet "b" "$(printf 'a\nb\nc\n' | "$BIN" sed -n '2p' | tr -d '\r')"
check awk-sum "summe 5" "$(printf 'x 2\ny 3\n' | "$BIN" awk '{s+=$2} END {print "summe",s}' | tr -d '\r')"
check awk-fs "y" "$(printf 'x:y\n' | "$BIN" awk -F: '{print $2}' | tr -d '\r')"

# diff and patch: the round trip is the point
printf 'eins\nzwei\ndrei\nvier\n' > a.txt
printf 'eins\nZWEI\ndrei\nvier\nfuenf\n' > b.txt
"$BIN" diff a.txt b.txt > p.diff
check diff-header "--- a/a.txt" "$(head -1 p.diff | tr -d '\r')"
checkcode patch-apply 0 "$("$BIN" patch p.diff >/dev/null 2>&1; echo $?)"
check patch-result "eins ZWEI drei vier fuenf" "$(tr '\n' ' ' < a.txt | tr -d '\r' | sed 's/ $//')"
rm -f p.diff a.txt b.txt

# files
printf 'abcdefgh' > t.txt
"$BIN" truncate -s 4 t.txt
check truncate "abcd" "$(cat t.txt | tr -d '\r')"
rm -f t.txt
printf '1\n2\n3\n4\n5\n' > big.txt
rm -f part*
"$BIN" split -l 2 big.txt part
check split-count "3" "$(ls part* 2>/dev/null | wc -l | tr -d ' ')"
check split-lines "2 2 1" "$(wc -l partaa partab partac 2>/dev/null | head -3 | awk '{printf "%s ", $1}' | sed 's/ $//')"
rm -f big.txt part*

# system
check uname-machine "x86_64" "$("$BIN" uname -m | tr -d '\r')"
# The child command depends on the target: a Windows binary sees the Windows
# PATH, so a Linux sleep is not available to it.
case "$BIN" in
*.exe) HANG="ping -n 10 127.0.0.1" ;;
*) HANG="sleep 5" ;;
esac
# shellcheck disable=SC2086
"$BIN" timeout 1 $HANG >/dev/null 2>&1
checkcode timeout-expires 124 "$?"

printf '%d checks, %d failures\n' "$checks" "$fails"
[ "$fails" -eq 0 ]
