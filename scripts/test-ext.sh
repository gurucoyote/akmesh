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

# The same checks serve both shapes: the combined busybox and the standalone
# ext binary dispatch on the first argument alike, so no prefix is needed.
run() {
	"$BIN" "$@"
}

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
check basename "$(printf 'foo')" "$(run basename /usr/local/bin/foo.txt .txt | tr -d '\r')"
check dirname "/usr/local/bin" "$(run dirname /usr/local/bin/foo.txt | tr -d '\r')"
check cut-fields "b" "$(printf 'a:b:c\n' | run cut -d: -f2 | tr -d '\r')"
check cut-chars "bcd" "$(printf 'abcdef\n' | run cut -c2-4 | tr -d '\r')"
check tac "3 2 1" "$(printf '1\n2\n3\n' | run tac | tr -d '\r' | tr '\n' ' ' | sed 's/ $//')"
check nl-numbered "     1	a" "$(printf 'a\n' | run nl | tr -d '\r')"

# hashing against the system tool when there is one
printf 'hallo\n' > h.txt
if command -v sha256sum >/dev/null 2>&1; then
	ours=$(run sha256sum h.txt | tr -d '\r' | cut -d' ' -f1)
	sys=$(sha256sum h.txt | cut -d' ' -f1)
	check sha256 "$sys" "$ours"
fi

# sed and awk
check sed-sub "erde" "$(printf 'hallo\n' | run sed 's/hallo/erde/' | tr -d '\r')"
check sed-quiet "b" "$(printf 'a\nb\nc\n' | run sed -n '2p' | tr -d '\r')"
check awk-sum "summe 5" "$(printf 'x 2\ny 3\n' | run awk '{s+=$2} END {print "summe",s}' | tr -d '\r')"
check awk-fs "y" "$(printf 'x:y\n' | run awk -F: '{print $2}' | tr -d '\r')"

# diff and patch: the round trip is the point
printf 'eins\nzwei\ndrei\nvier\n' > a.txt
printf 'eins\nZWEI\ndrei\nvier\nfuenf\n' > b.txt
run diff a.txt b.txt > p.diff
check diff-header "--- a/a.txt" "$(head -1 p.diff | tr -d '\r')"
checkcode patch-apply 0 "$(run patch p.diff >/dev/null 2>&1; echo $?)"
check patch-result "eins ZWEI drei vier fuenf" "$(tr '\n' ' ' < a.txt | tr -d '\r' | sed 's/ $//')"
rm -f p.diff a.txt b.txt

# files
printf 'abcdefgh' > t.txt
run truncate -s 4 t.txt
check truncate "abcd" "$(cat t.txt | tr -d '\r')"
rm -f t.txt
printf '1\n2\n3\n4\n5\n' > big.txt
rm -f part*
run split -l 2 big.txt part
check split-count "3" "$(ls part* 2>/dev/null | wc -l | tr -d ' ')"
check split-lines "2 2 1" "$(wc -l partaa partab partac 2>/dev/null | head -3 | awk '{printf "%s ", $1}' | sed 's/ $//')"
rm -f big.txt part*

# file, iconv, env
printf 'hallo welt\n' > t.txt
check file-text "ASCII text" "$(run file -b t.txt | tr -d '\r')"
check file-dir "directory" "$(run file -b . | tr -d '\r')"
: > e.txt
check file-empty "empty" "$(run file -b e.txt | tr -d '\r')"
check iconv-latin1 "Grüße" "$(printf 'Gr\374\337e\n' | run iconv -f ISO-8859-1 -t UTF-8 | tr -d '\r')"
check iconv-cp1252 "€" "$(printf '\200' | run iconv -f CP1252 -t UTF-8 | tr -d '\r')"
check iconv-to-latin1 "$(printf 'Gr\374\337e' | od -An -tx1 | tr -d ' \n')" "$(printf 'Grüße' | run iconv -f UTF-8 -t ISO-8859-1 | od -An -tx1 | tr -d ' \n')"
check env-assign "1" "$(run env MARKER=ja "$BIN" env | grep -c '^MARKER=ja$' | tr -d '\r')"
check env-unset "0" "$(run env -u HOME "$BIN" env | grep -c '^HOME=' | tr -d '\r')"
rm -f t.txt e.txt

# curl and wget: the checks that need no network. The transfer itself is covered
# by the Go tests against a local server.
check curl-help "1" "$(run curl -h | grep -c 'Only http and https' | tr -d '\r')"
checkcode curl-unknown-flag 2 "$(run curl -Z http://example.com >/dev/null 2>&1; echo $?)"
checkcode curl-file-scheme 3 "$(run curl -s file:///etc/passwd >/dev/null 2>&1; echo $?)"
check wget-help "1" "$(run wget -h | grep -c 'Not supported' | tr -d '\r')"
checkcode wget-recursive-refused 2 "$(run wget -r http://example.com >/dev/null 2>&1; echo $?)"
checkcode wget-unknown-flag 2 "$(run wget -N http://example.com >/dev/null 2>&1; echo $?)"

# free, df, du
check free-mem-row "1" "$(run free | grep -c '^Mem:' | tr -d '\r')"
check free-human "ja" "$(run free -h | grep -qE '[0-9.]+[KMGT]' && echo ja)"
checkcode free-unknown 2 "$(run free -Z >/dev/null 2>&1; echo $?)"
check df-header "Mounted on" "$(run df -h | head -1 | tr -d '\r' | grep -o 'Mounted on')"
check df-human "ja" "$(run df -h | tail -n +2 | head -1 | grep -qE '[0-9.]+[KMGT]' && echo ja)"
# The path filter needs a real mount point: a Windows binary started from WSL
# sees a UNC working directory, which belongs to no drive.
case "$BIN" in
*.exe) DFPATH="C:/" ;;
*) DFPATH="." ;;
esac
check df-one-row "1" "$(run df -h "$DFPATH" | tail -n +2 | grep -c . | tr -d '\r')"
mkdir -p du1 && printf 'xxxx' > du1/f.txt && head -c 2048 /dev/zero >> du1/f.txt
check du-summary-lines "1" "$(run du -s du1 | wc -l | tr -d ' \r')"
check du-human "ja" "$(run du -h -s du1 | cut -f1 | tr -d '\r' | grep -qE '^[0-9.]+K$' && echo ja)"
check du-total "1" "$(run du -s -c du1 | grep -c total | tr -d '\r')"
checkcode du-unknown 2 "$(run du -Z >/dev/null 2>&1; echo $?)"
rm -rf du1

# ps, kill, killall
check ps-fields "PID COMM" "$(run ps -eo pid,comm | head -1 | tr -s ' ' | tr -d '\r' | sed 's/^ //;s/ $//')"
check ps-ef-ppid "PPID" "$(run ps -ef | head -1 | tr -d '\r' | grep -o PPID | head -1)"
check ps-has-rows "ja" "$([ "$(run ps -eo pid | wc -l)" -gt 1 ] && echo ja)"
check ps-unknown-field "2" "$(run ps -eo bogus >/dev/null 2>&1; echo $?)"
checkcode kill-bad-signal 2 "$(run kill -NOSUCHSIGNAL 1 >/dev/null 2>&1; echo $?)"
checkcode kill-bad-pid 2 "$(run kill keinpid >/dev/null 2>&1; echo $?)"
checkcode killall-no-match 1 "$(run killall gibtsnicht 2>/dev/null; echo $?)"

# system
check uname-machine "x86_64" "$(run uname -m | tr -d '\r')"
# The child command depends on the target: a Windows binary sees the Windows
# PATH, so a Linux sleep is not available to it.
case "$BIN" in
*.exe) HANG="ping -n 10 127.0.0.1" ;;
*) HANG="sleep 5" ;;
esac
# shellcheck disable=SC2086
run timeout 1 $HANG >/dev/null 2>&1
checkcode timeout-expires 124 "$?"

printf '%d checks, %d failures\n' "$checks" "$fails"
[ "$fails" -eq 0 ]
