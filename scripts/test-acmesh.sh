BIN=${1:-./acmesh}
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

# The battery runs against the acmesh binary for the target it was built for:
# every check calls it as "acmesh <command> [args]".
run() {
	"$BIN" "$@"
}

# A profile without networking lacks curl, wget, ssh and scp; those checks are
# skipped rather than failed, so the same battery serves every build.
if "$BIN" curl -h >/dev/null 2>&1; then HAS_NET=1; else HAS_NET=0; fi

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

# silent success: a bad flag must never end with exit 0
checkcode ls-unknown-flag 2 "$(run ls -Z >/dev/null 2>&1; echo $?)"
checkcode ls-sort-time 2 "$(run ls -t >/dev/null 2>&1; echo $?)"
touch xmode
checkcode chmod-plusx 0 "$(run chmod +x xmode >/dev/null 2>&1; echo $?)"
# Windows has no execute bit, so the mode itself is only checked where it exists.
case "$BIN" in
*.exe) ;;
*) check chmod-plusx-bit "yes" "$([ -x xmode ] && echo yes || echo no)" ;;
esac
checkcode chmod-bad-mode 1 "$(run chmod 999 xmode >/dev/null 2>&1; echo $?)"
# GNU says "invalid mode: '999'" plus the hint line; the fork follows GNU.
check chmod-bad-mode-message "ja" "$(run chmod 999 xmode 2>&1 | grep -q "invalid mode: '999'" && echo ja)"
check chmod-try-hint "ja" "$(run chmod 999 xmode 2>&1 | grep -q "Try 'chmod --help'" && echo ja)"
mkdir -p cdir/sub && printf 'z\n' > cdir/sub/f.txt
checkcode chmod-recursive 0 "$(run chmod -R 700 cdir >/dev/null 2>&1; echo $?)"
# Mode bits are only real on unix; Windows keeps its own ACLs behind them.
case "$BIN" in
*.exe) ;;
*) check chmod-recursive-mode "rwx------" "$(run ls -ld cdir/sub | awk '{print $1}' | cut -c2-10)" ;;
esac
case "$BIN" in
*.exe) ;;
*)
	chmod 644 cdir/sub/f.txt 2>/dev/null
	check chmod-verbose "ja" "$(run chmod -v 755 cdir/sub/f.txt 2>&1 | grep -q "changed from 0644 (rw-r--r--) to 0755 (rwxr-xr-x)" && echo ja)"
	;;
esac
rm -rf cdir
# cp: -a (mode and time), -t and -n are the reasons it got forked
mkdir -p cpsrc/sub && printf 'a\n' > cpsrc/a.txt && printf 'b\n' > cpsrc/sub/b.txt
chmod 640 cpsrc/a.txt 2>/dev/null
checkcode cp-archive 0 "$(run cp -a cpsrc cpdst >/dev/null 2>&1; echo $?)"
case "$BIN" in
*.exe) ;;
*) check cp-archive-mode "640" "$(run stat -c %a cpdst/a.txt 2>/dev/null | tr -d '\r')" ;;
esac
check cp-archive-nested "b" "$(run cat cpdst/sub/b.txt | tr -d '\r\n')"
mkdir -p cptarget
checkcode cp-target-dir 0 "$(run cp -t cptarget cpsrc/a.txt cpsrc/sub/b.txt >/dev/null 2>&1; echo $?)"
check cp-target-files "2" "$(run ls cptarget | wc -l | tr -d ' ')"
printf 'old\n' > cpdst/a.txt
run cp -n cpsrc/sub/b.txt cpdst/a.txt >/dev/null 2>&1
check cp-no-clobber "old" "$(run cat cpdst/a.txt | tr -d '\r\n')"
rm -rf cpsrc cpdst cptarget

# tar: -czf and -tzf are what every agent writes
mkdir -p tdir/sub && printf 'x\n' > tdir/a.txt && printf 'y\n' > tdir/sub/b.txt && printf 'z\n' > tdir/skip.log
checkcode tar-create-gzip 0 "$(run tar -czf t.tgz tdir >/dev/null 2>&1; echo $?)"
# The separator differs per target, so the check looks for the file name only.
check tar-list-gzip "ja" "$(run tar -tzf t.tgz 2>/dev/null | grep -q 'a\.txt' && echo ja)"
check tar-list-without-z "ja" "$(run tar -tf t.tgz 2>/dev/null | grep -q 'a\.txt' && echo ja)"
checkcode tar-exclude 0 "$(run tar -czf te.tgz --exclude='*.log' tdir >/dev/null 2>&1; echo $?)"
check tar-exclude-worked "" "$(run tar -tzf te.tgz 2>/dev/null | grep 'skip.log')"
mkdir -p tout
checkcode tar-extract-strip 0 "$(run tar -xzf t.tgz -C tout --strip-components=1 >/dev/null 2>&1; echo $?)"
check tar-strip-file "x" "$(run cat tout/a.txt 2>/dev/null | tr -d '\r\n')"
checkcode tar-C-create 0 "$(run tar -cf t2.tar -C tdir a.txt >/dev/null 2>&1; echo $?)"
check tar-C-listing "a.txt" "$(run tar -tf t2.tar 2>/dev/null | tr -d '\r')"
checkcode tar-j-refused 2 "$(run tar -cjf t.tbz tdir >/dev/null 2>&1; echo $?)"
rm -rf tdir tout t.tgz te.tgz t2.tar

# hexdump -C, base64 -w, wc -m, tr -sc/-cd and touch -t are the individual
# switches; each check guards one spelling an agent writes.
printf 'AB' > small.bin
check hexdump-canonical "ja" "$(run hexdump -C small.bin | grep -q '41 42' && echo ja)"
check hexdump-ascii "|AB|" "$(run hexdump -C small.bin | sed -n 's/.*\(|AB|\).*/\1/p' | tr -d '\r')"
check hexdump-final-offset "00000002" "$(run hexdump -C small.bin | tail -1 | tr -d '\r')"
check base64-wrap0 "1" "$(printf 'aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa/dummy\n' | run base64 -w0 | wc -l | tr -d ' ')"
awk 'BEGIN{for(i=0;i<100;i++)printf "a"}' > long.txt
check base64-default-wrap "ja" "$(run base64 long.txt 2>/dev/null | head -1 | grep -qE '^.{76}$' && echo ja || echo nein)"
printf 'abc\n' > wct.txt
check wc-chars "4" "$(run wc -m wct.txt | awk '{print $1}')"
check wc-bytes "4" "$(run wc -c wct.txt | awk '{print $1}')"
check tr-range "ABC" "$(printf 'abc' | run tr a-z A-Z | tr -d '\r')"
check tr-complement "1234" "$(printf 'ab12cd34' | run tr -cd 0-9 | tr -d '\r')"
check tr-squeeze "a b" "$(printf 'a   b' | run tr -s ' ' | tr -d '\r')"
printf 'x' > tstamp.txt
checkcode touch-stamp 0 "$(run touch -t 202001020304 tstamp.txt >/dev/null 2>&1; echo $?)"
case "$BIN" in
*.exe) ;;
*) check touch-stamp-year "2020" "$(run stat -c %y tstamp.txt 2>/dev/null | cut -c1-4)" ;;
esac
checkcode touch-date 0 "$(run touch -d '2020-01-02 03:04:05' tstamp.txt >/dev/null 2>&1; echo $?)"
rm -f small.bin wct.txt tstamp.txt long.txt

# xargs starts a real program, so the command is the binary itself: echo and
# false are builtins on Windows and cannot be started.
check xargs-replace "[a]" "$(printf 'a\n' | run xargs -I{} "$BIN" echo '[{}]' | tr -d '\r')"
check xargs-chunk "1 2" "$(printf '1\n2\n' | run xargs -n 2 "$BIN" echo | tr -d '\r')"
check xargs-no-run-empty "" "$(printf '' | run xargs -r "$BIN" echo | tr -d '\r\n')"
checkcode xargs-child-fails 123 "$(printf 'x\n' | run xargs "$BIN" false >/dev/null 2>&1; echo $?)"

# stat, sync, hostname, policy: the four that had no test at all (acmesh-nyu)
printf 'sechs\n' > statme.txt
chmod 644 statme.txt
check stat-size "6" "$(run stat -c '%s' statme.txt | tr -d '\r')"
check stat-name "statme.txt" "$(run stat -c '%n' statme.txt | tr -d '\r' | sed 's|.*[/\\]||')"
case "$BIN" in
*.exe) ;;
*) check stat-mode "644" "$(run stat -c '%a' statme.txt | tr -d '\r')" ;;
esac
checkcode stat-missing 1 "$(run stat /nicht/vorhanden >/dev/null 2>&1; echo $?)"
checkcode stat-no-operand 2 "$(run stat >/dev/null 2>&1; echo $?)"
checkcode sync-file 0 "$(run sync statme.txt >/dev/null 2>&1; echo $?)"
checkcode sync-missing 1 "$(run sync /nicht/vorhanden >/dev/null 2>&1; echo $?)"
check hostname-present "ja" "$(run hostname 2>/dev/null | grep -q . && echo ja)"
checkcode hostname-set-refused 2 "$(run hostname neuer-name >/dev/null 2>&1; echo $?)"
checkcode policy-unrestricted 1 "$(run policy >/dev/null 2>&1; echo $?)"
check policy-check-allowed "ja" "$(run policy --check statme.txt 2>/dev/null | grep -q '^allowed: ' && echo ja)"
checkcode policy-unknown 2 "$(run policy --quatsch >/dev/null 2>&1; echo $?)"

# echo, pwd, true, false: the GNU echo semantics, not u-root's
check echo-plain "a b" "$(run echo a b | tr -d '\r')"
check echo-backslash 'a\nb' "$(run echo 'a\nb' | tr -d '\r')"
check echo-escape-lines 2 "$(run echo -e 'a\nb' | tr -d '\r' | wc -l | tr -d ' ')"
check echo-no-newline "x" "$(run echo -n x | tr -d '\r')"
# pwd: the Windows binary prints a UNC path, so compare the last component.
check pwd-here "$(basename "$PWD")" "$(run pwd | tr -d '\r' | sed 's|.*[/\\]||')"
checkcode true-exit 0 "$(run true; echo $?)"
checkcode false-exit 1 "$(run false; echo $?)"

# sort and shuf: our own code since 2026-11-18, checked against GNU's order
check sort-default "a b c" "$(printf 'c\na\nb\n' | run sort | tr -d '\r' | tr '\n' ' ' | sed 's/ $//')"
check sort-numeric "1 2 10" "$(printf '10\n2\n1\n' | run sort -n | tr -d '\r' | tr '\n' ' ' | sed 's/ $//')"
check sort-key "c 1|b 2|a 10" "$(printf 'b 2\na 10\nc 1\n' | run sort -t' ' -k2,2n | tr -d '\r' | tr '\n' '|' | sed 's/ $//;s/|$//')"
check sort-version "v1.2.3 v1.9 v1.10" "$(printf 'v1.10\nv1.9\nv1.2.3\n' | run sort -V | tr -d '\r' | tr '\n' ' ' | sed 's/ $//')"
check sort-unique "1 2" "$(printf '2\n1\n2\n' | run sort -u | tr -d '\r' | tr '\n' ' ' | sed 's/ $//')"
checkcode sort-disorder 1 "$(printf 'b\na\n' | run sort -c >/dev/null 2>&1; echo $?)"
checkcode sort-unknown-flag 2 "$(run sort --bogus >/dev/null 2>&1; echo $?)"
check shuf-count 2 "$(printf 'a\nb\nc\nd\n' | run shuf -n 2 | tr -d '\r' | sed '/^$/d' | wc -l | tr -d ' ')"
check shuf-range "1 2 3" "$(run shuf -i 1-3 | tr -d '\r' | sort | tr '\n' ' ' | sed 's/ $//')"

# diff and patch: the round trip is the point
printf 'eins\nzwei\ndrei\nvier\n' > a.txt
printf 'eins\nZWEI\ndrei\nvier\nfuenf\n' > b.txt
run diff a.txt b.txt > p.diff
check diff-header "--- a/a.txt" "$(head -1 p.diff | tr -d '\r')"
# GNU form: the diff comes on stdin, a positional name would be the file to patch
checkcode patch-apply 0 "$(run patch < p.diff >/dev/null 2>&1; echo $?)"
check patch-result "eins ZWEI drei vier fuenf" "$(tr '\n' ' ' < a.txt | tr -d '\r' | sed 's/ $//')"
rm -f p.diff a.txt b.txt

# find and grep are forked u-root commands: the agent spellings are the point
mkdir -p fdir/sub && printf 'x\n' > fdir/a.txt && printf 'y\n' > fdir/sub/b.txt
check find-optional-path "1" "$(cd fdir && run find -name 'a.txt' | wc -l | tr -d ' ')"
check find-dot-prefix "./a.txt" "$(cd fdir && run find . -maxdepth 1 -name 'a.txt' | tr -d '\r')"
check find-maxdepth "0" "$(cd fdir && run find . -maxdepth 1 -name 'b.txt' | wc -l | tr -d ' ')"
# -exec runs a real program, so the executable has to exist on the target too: the
# binary itself is the one command both platforms have.
check find-exec "a.txt" "$(cd fdir && run find . -maxdepth 1 -name 'a.txt' -exec "$BIN" echo {} \; | tr -d '\r' | sed 's|^\./||')"
checkcode find-missing 1 "$(run find /nicht/vorhanden >/dev/null 2>&1; echo $?)"
printf 'alpha\nBravo\ncharlie\ndelta alpha\n' > g.txt
check grep-extended "2" "$(run grep -E 'a.ph' g.txt | wc -l | tr -d ' ')"
check grep-ignore-case "Bravo" "$(run grep -i bravo g.txt | tr -d '\r')"
check grep-word "2" "$(run grep -w alpha g.txt | wc -l | tr -d ' ')"
check grep-context "4" "$(run grep -A 1 alpha g.txt | wc -l | tr -d ' ')"
check grep-only-match "alp" "$(run grep -o alp g.txt | head -1 | tr -d '\r')"
checkcode grep-no-match 1 "$(run grep zenix g.txt >/dev/null 2>&1; echo $?)"
check grep-recursive "ja" "$(run grep -r alpha . 2>/dev/null | grep -q 'g\.txt' && echo ja)"
rm -rf fdir g.txt

# head is a forked u-root command: the short form and -q are the reason
printf '1\n2\n3\n4\n5\n6\n7\n' > h.txt
check head-short "1 2 3" "$(run head -5 h.txt | tr -d '\r' | head -3 | tr '\n' ' ' | sed 's/ $//')"
check head-quiet "abc" "$(printf 'a\nb\n' > h1.txt; printf 'c\n' > h2.txt; run head -q h1.txt h2.txt | tr -d '\r' | tr -d '\n')"
checkcode head-missing 1 "$(run head /nicht/vorhanden >/dev/null 2>&1; echo $?)"
rm -f h.txt h1.txt h2.txt

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

# jq
check jq-field "1" "$(echo '{"a":1}' | run jq -r .a | tr -d '\r')"
check jq-compact '{"x":1}' "$(echo '{"a":{"x":1}}' | run jq -c .a | tr -d '\r')"
check jq-raw '"nein"' "$(echo '{"a":"nein"}' | run jq '.a' | tr -d '\r')"
check jq-slurp "3" "$(echo '1 2 3' | run jq -s length | tr -d '\r')"
check jq-arg "Hallo Martin" "$(echo '{"x":"Hallo "}' | run jq -r --arg n Martin '.x + $n' | tr -d '\r')"
checkcode jq-bad-filter 3 "$(echo '{}' | run jq '.a |' >/dev/null 2>&1; echo $?)"
checkcode jq-bad-json 2 "$(echo '{"a":}' | run jq .a >/dev/null 2>&1; echo $?)"
checkcode jq-exit-false 1 "$(echo '{"a":false}' | run jq -e .a >/dev/null 2>&1; echo $?)"

if [ "$HAS_NET" = 1 ]; then
# curl and wget: the checks that need no network. The transfer itself is covered
# by the Go tests against a local server.
check curl-help "1" "$(run curl -h | grep -c 'Only http and https' | tr -d '\r')"
checkcode curl-unknown-flag 2 "$(run curl -Z http://example.com >/dev/null 2>&1; echo $?)"
checkcode curl-file-scheme 3 "$(run curl -s file:///etc/passwd >/dev/null 2>&1; echo $?)"
check wget-help "1" "$(run wget -h | grep -c 'Not supported' | tr -d '\r')"
checkcode wget-recursive-refused 2 "$(run wget -r http://example.com >/dev/null 2>&1; echo $?)"
checkcode wget-unknown-flag 2 "$(run wget -N http://example.com >/dev/null 2>&1; echo $?)"
fi

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

# unknown command: the dispatcher names acmesh, not u-root's busybox
checkcode unknown-command-exit 1 "$(run gibtsnicht >/dev/null 2>&1; echo $?)"
check unknown-command-message "ja" "$(run gibtsnicht 2>&1 | grep -q 'not present in acmesh' && echo ja)"
check unknown-command-no-busybox "0" "$(run gibtsnicht 2>&1 | grep -c busybox | tr -d '\r')"

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

# shell and policy
check sh-echo "hallo" "$(run sh -c 'echo hallo' | tr -d '\r')"
check sh-var "7" "$(run sh -c 'x=3; y=4; echo $((x+y))' | tr -d '\r')"
check sh-status "3" "$(run sh -c 'exit 3'; echo $?)"
# Pipes need Unix tools on PATH; a Windows binary would call the Windows ones.
case "$BIN" in
*.exe) ;;
*) check sh-pipe "2" "$(printf 'a\nb\na\n' | run sh -c 'sort | uniq -c | wc -l' | tr -d ' \r')" ;;
esac

POL="$WORK/policy.json"
mkdir -p "$WORK/work"
printf 'drin' > "$WORK/work/in.txt"
cat > "$POL" <<JSON
{"version":1,"read":["$WORK/work"],"write":["$WORK/work"],"network":[]}
JSON

case "$BIN" in
*.exe)
	# A policy cannot be enforced on Windows, and the binary refuses instead of
	# running unconfined.
	checkcode policy-refused 1 "$(ACMESH_POLICY="$POL" "$BIN" policy >/dev/null 2>&1; echo $?)"
	;;
*)
	check policy-active "0" "$(ACMESH_POLICY="$POL" "$BIN" policy >/dev/null 2>&1; echo $?)"
	check policy-shows-source "ja" "$(ACMESH_POLICY="$POL" "$BIN" policy 2>/dev/null | grep -q "$POL" && echo ja)"
	check policy-read-inside "drin" "$(ACMESH_POLICY="$POL" "$BIN" sh -c "cat $WORK/work/in.txt" 2>/dev/null | tr -d '\r')"
	check policy-blocks-read "ja" "$(ACMESH_POLICY="$POL" "$BIN" sh -c 'cat /etc/hostname' >/dev/null 2>&1 || echo ja)"
	check policy-blocks-write "ja" "$(ACMESH_POLICY="$POL" "$BIN" sh -c "echo x > $WORK/aussen.txt" >/dev/null 2>&1 || echo ja)"
	check policy-no-outside-file "ja" "$([ ! -f "$WORK/aussen.txt" ] && echo ja)"
	check policy-write-inside "ok" "$(ACMESH_POLICY="$POL" "$BIN" sh -c "echo ok > $WORK/work/neu.txt; cat $WORK/work/neu.txt" 2>/dev/null | tr -d '\r')"
	check policy-check-allowed "0" "$(ACMESH_POLICY="$POL" "$BIN" policy --check "$WORK/work/in.txt" >/dev/null 2>&1; echo $?)"
	check policy-check-denied "1" "$(ACMESH_POLICY="$POL" "$BIN" policy --check /etc/hostname >/dev/null 2>&1; echo $?)"
	# "Write implies read": a policy that names only the write path must still
	# allow reading it, because the write rule carries the read rights. Two runs,
	# one inside and one outside, so a policy that blocks everything cannot pass
	# this as a success.
	printf '{"version":1,"write":["%s"]}' "$WORK/work" > "$WORK/write-only.json"
	check policy-write-only-inside-allowed "0" "$(ACMESH_POLICY="$WORK/write-only.json" "$BIN" policy --check "$WORK/work/in.txt" >/dev/null 2>&1; echo $?)"
	check policy-write-only-outside-denied "1" "$(ACMESH_POLICY="$WORK/write-only.json" "$BIN" policy --check /etc/hostname >/dev/null 2>&1; echo $?)"
	if [ "$HAS_NET" = 1 ]; then
		check policy-blocks-network "ja" "$(ACMESH_POLICY="$POL" "$BIN" curl -s https://example.com >/dev/null 2>&1 || echo ja)"
		check policy-network-message "ja" "$(ACMESH_POLICY="$POL" "$BIN" curl -s https://example.com 2>&1 | grep -q 'does not allow network access' && echo ja)"
	fi
	# A broken policy must stop the process instead of running unconfined.
	printf '{"version":9}' > "$WORK/broken.json"
	checkcode policy-broken-refused 1 "$(ACMESH_POLICY="$WORK/broken.json" "$BIN" echo x >/dev/null 2>&1; echo $?)"
	;;
esac

printf '%d checks, %d failures\n' "$checks" "$fails"
[ "$fails" -eq 0 ]
