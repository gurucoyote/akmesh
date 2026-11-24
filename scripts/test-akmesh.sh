#!/bin/sh
BIN=${1:-./akmesh}
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

# The repository root, resolved before the script changes directory: checks that
# need a fixture from the repo (apk testdata) would otherwise look in $WORK.
SCRIPT_DIR=$(cd "$(dirname "$0")" && pwd)
REPO_DIR=$(cd "$SCRIPT_DIR/.." && pwd)

# The battery runs against the akmesh binary for the target it was built for:
# every check calls it as "akmesh <command> [args]".
run() {
	"$BIN" "$@"
}

# A profile without networking lacks curl, wget, ssh and scp; those checks are
# skipped rather than failed, so the same battery serves every build.
if "$BIN" curl -h >/dev/null 2>&1; then HAS_NET=1; else HAS_NET=0; fi

WORK=$(mktemp -d "${TMPDIR:-/tmp}/exttest.XXXXXX") || exit 2
trap 'rm -rf "$WORK"' EXIT
cd "$WORK" || exit 2

# Under WSL the .exe is started through /init, and that keeps the image of a
# path: after a build, running ./akmesh.exe again hands back the old one while a
# copy under another name is already the new one (measured 18.11.2026, same md5,
# different behaviour). So the battery runs on a copy with a fresh path, which
# is the same file.
case "$BIN" in
*.exe) cp "$BIN" "$WORK/acmesh-under-test.exe" && BIN="$WORK/acmesh-under-test.exe" && WINDOWS=1 ;;
esac

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
# cut behaviour lives in Go since acmesh-pfm (internal/cut):
#   -d/-f, -c          -> TestFieldsAndCharacters
#   Reihenfolge, Doppel-> TestSelectionComesOutSorted, TestOverlappingRangesAreOne
#   -f-2, -f2-         -> TestOpenEndedRanges
#   --complement, -s   -> TestComplementKeepsTheRest, TestOnlyDelimitedDropsTheRest
#   -b mit Umlauten    -> TestBytesAndOutputDelimiter
#   Fehlbedienung      -> TestRefusalsFollowGNU, TestFileArgumentAndBadList
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
# ls is our own command since acmesh-wx6: an unknown flag is an error, and -t
# sorts, which the old u-root command refused with exit 2.
checkcode ls-unknown-flag 2 "$(run ls -Z >/dev/null 2>&1; echo $?)"
check ls-sort-time "neu,alt" "$(lsd=$(mktemp -d); printf x > "$lsd/alt"; printf x > "$lsd/neu"; touch -d '2020-01-01' "$lsd/alt"; run ls -t "$lsd" | tr -d '\r' | tr '\n' ',' | sed 's/,$//')"
check ls-long-has-year "ja" "$(lsd=$(mktemp -d); printf x > "$lsd/alt"; touch -d '2020-01-01' "$lsd/alt"; run ls -l "$lsd" | grep -q ' 2020 ' && echo ja)"
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
# cp und tar laufen in Go seit acmesh-qiz (internal/tools/files_test.go):
#   -a, verschachtelt        -> TestCpArchiveCopiesTheTree
#   -t                       -> TestCpTargetDirectory
#   -n                       -> TestCpNoClobber
#   -czf, -tzf, -tf          -> TestTarCreateAndList
#   --exclude                -> TestTarExclude
#   -xzf -C --strip-components -> TestTarExtractStripComponents
#   -cf -C                   -> TestTarCreateFromDirectory
#   -cjf abgelehnt           -> TestTarRefusesBzip2
#   diff und patch           -> TestDiffAndPatchRoundTrip
# Was bleibt: die Rechte nach "cp -a" gelten nur auf dem echten Ziel.
mkdir -p cpsrc/sub && printf 'a\n' > cpsrc/a.txt && printf 'b\n' > cpsrc/sub/b.txt
chmod 640 cpsrc/a.txt 2>/dev/null
run cp -a cpsrc cpdst >/dev/null 2>&1
case "$BIN" in
*.exe) ;;
*) check cp-archive-mode "640" "$(run stat -c %a cpdst/a.txt 2>/dev/null | tr -d '\r')" ;;
esac
rm -rf cpsrc cpdst

# sha256sum -c: the check form scripts use to verify a download
printf 'hallo\n' > sumfile.txt
run sha256sum sumfile.txt > sums.txt
check sums-check-ok "sumfile.txt: OK" "$(run sha256sum -c sums.txt 2>/dev/null | tr -d '\r')"
checkcode sums-check-exit 0 "$(run sha256sum -c sums.txt >/dev/null 2>&1; echo $?)"
printf '%s  sumfile.txt\n' "0000000000000000000000000000000000000000000000000000000000000000" > sums-bad.txt
check sums-check-failed "sumfile.txt: FAILED" "$(run sha256sum -c sums-bad.txt 2>/dev/null | tr -d '\r')"
checkcode sums-check-failed-exit 1 "$(run sha256sum -c sums-bad.txt >/dev/null 2>&1; echo $?)"
rm -f sumfile.txt sums.txt sums-bad.txt

# hexdump -C, base64 -w, wc -m, tr -sc/-cd and touch -t are the individual
# switches; each check guards one spelling an agent writes.
printf 'AB' > small.bin
check hexdump-canonical "ja" "$(run hexdump -C small.bin | grep -q '41 42' && echo ja)"
check hexdump-ascii "|AB|" "$(run hexdump -C small.bin | sed -n 's/.*\(|AB|\).*/\1/p' | tr -d '\r')"
check hexdump-final-offset "00000002" "$(run hexdump -C small.bin | tail -1 | tr -d '\r')"
# base64 behaviour lives in Go since acmesh-pfm (internal/forked/base64):
#   -w0, -w4, --wrap=8      -> TestWrapWidth, TestWrapWidths, TestWrapArgumentErrors
#   -d mit kaputter Eingabe -> TestDecodeRejectsGarbage, TestDecodeWithoutPaddingKeepsTheLastBlock
#   -i                      -> TestIgnoreGarbage, TestIgnoreGarbageWithABadLength
#   Datei, "--", Fehlbedienung -> TestFileAndDashArguments, TestRefusalsFollowGNU
printf 'abc\n' > wct.txt
# wc behaviour lives in Go since acmesh-795 (internal/forked/wc):
#   -m gegen -c mit Umlauten -> TestUmlautsAreCharactersNotBytes
#   -l -w -c, Vorgabe         -> TestLinesWordsBytes, TestDefaultCountsLinesWordsBytes
#   Summenzeile               -> TestTotalLineForSeveralFiles
#   --files0-from             -> TestFiles0From, TestFiles0FromWithoutAValueAndAMissingList
#   fehlende Datei            -> TestMissingFileIsNamedAndOne
# tr behaviour lives in Go since acmesh-49y (internal/forked/tr):
#   a-z A-Z, Bereiche        -> TestRanges, TestUnequalSetsRepeatTheLast
#   -cd, -C                  -> TestComplement, TestDeleteAndComplement, TestComplementWithAClassTranslates
#   -s                       -> TestSqueeze, TestSqueezeWithTwoSets, TestSqueezeAloneAndClassErrors
#   Klassen                  -> TestClassesAreASCII (ASCII wie GNU, nicht Unicode)
#   Escapes, Operandenfehler -> TestEscapesInSets, TestOperandErrorsFollowGNU
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
# stat -c %s, -c %n, fehlende Datei und kein Operand sind Go-Tests seit
# acmesh-j4f (TestCountedFieldsAndOperandErrors). stat-mode bleibt: die Rechte
# gelten nur auf dem echten Ziel.
case "$BIN" in
*.exe) ;;
*) check stat-mode "644" "$(run stat -c '%a' statme.txt | tr -d '\r')" ;;
esac
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
# -h is the case the Go test could not fail on: without SI suffixes it equals -n
# (acmesh-k7t). Order checked against GNU coreutils 9.7.
check sort-human "500 1K 2M" "$(printf '2M\n1K\n500\n' | run sort -h | tr -d '\r' | tr '\n' ' ' | sed 's/ $//')"
check sort-unique "1 2" "$(printf '2\n1\n2\n' | run sort -u | tr -d '\r' | tr '\n' ' ' | sed 's/ $//')"
checkcode sort-disorder 1 "$(printf 'b\na\n' | run sort -c >/dev/null 2>&1; echo $?)"
checkcode sort-unknown-flag 2 "$(run sort --bogus >/dev/null 2>&1; echo $?)"
check shuf-count 2 "$(printf 'a\nb\nc\nd\n' | run shuf -n 2 | tr -d '\r' | sed '/^$/d' | wc -l | tr -d ' ')"
check shuf-range "1 2 3" "$(run shuf -i 1-3 | tr -d '\r' | sort | tr '\n' ' ' | sed 's/ $//')"

# diff und patch: der Rundlauf ist TestDiffAndPatchRoundTrip (acmesh-qiz), samt
# Kopfzeile, Anwenden ueber stdin und Inhalt danach.

# find and grep are forked u-root commands: the agent spellings are the point.
# The behaviour checks live in Go since acmesh-qz2 (internal/forked/find):
#   optional start path, "./" prefix   -> TestStartPathIsOptional
#   -maxdepth, -mindepth               -> TestMaxDepthReportsTheBoundaryDirectory, TestMindepth
#   -name, -path, -regex, -type, -mode -> TestNameTakesAGlob, TestPathMatchesTheWholePath,
#                                         TestRegexMatchesTheWholePath, TestTypeDirectorySelectsDirectories
#   -not, -print, -print0              -> TestNotAndType, TestExplicitPrintIsTheDefault, TestPrint0EndsEveryNameWithNUL
#   symlink not followed               -> TestSymlinkIsListedButNotFollowed
#   parse errors, walk error           -> TestPredicateErrorsAreUsageErrors, TestWalkErrorIsNotSuccess
# What stays: -exec spawning the built binary itself, which is the per-target case.
mkdir -p fdir/sub && printf 'x\n' > fdir/a.txt && printf 'y\n' > fdir/sub/b.txt
check find-exec "a.txt" "$(cd fdir && run find . -maxdepth 1 -name 'a.txt' -exec "$BIN" echo {} \; | tr -d '\r' | sed 's|^\./||')"
# Ausdrucksbaum, Aktionen und Fehlerfaelle (acmesh-x7f, acmesh-kgi):
#   -o, Klammern, !              -> TestOrAndParentheses, TestNotAndEmpty
#   -printf, -delete             -> TestPrintfAndDelete
#   Exit 2 auf kaputten Ausdruck -> TestBrokenExpressionIsUsageError
# Nur der Aufruf am gebauten Binary bleibt hier: die Klammern sind Shell-Sache.
check find-or "$(cd fdir && run find . -maxdepth 1 -name 'a.txt' -o -name 'zzz' | tr -d '\r' | sort | tr '\n' ' ')" "./a.txt "
check find-not-dirs "$(cd fdir && run find . -maxdepth 1 ! -type d | tr -d '\r' | grep -c 'a.txt')" "1"
check find-delete "$(cd fdir && cp a.txt gone.txt && run find . -maxdepth 1 -name 'gone.txt' -delete; printf '%s' "$?")" "0"
check find-delete-gone "weg" "$(if [ -e fdir/gone.txt ]; then echo da; else echo weg; fi)"
check find-broken-expr "$(cd fdir && run find . -name 2>/dev/null; printf '%s' "$?")" "2"
check find-unknown-predicate "$(cd fdir && run find . -bogus x 2>/dev/null; printf '%s' "$?")" "2"
# df-Kopfzeile (acmesh-jag): GNU nennt die Spalte bei -h "Size", sonst nach der
# Einheit, und "Avail" nur bei -h; die Zahlen stehen rechtsbuendig.
# diff -u setzt den Zeitstempel hinter den Namen (acmesh-5ug), sonst findet
# patch -p0 die Datei nur mit -i.
printf 'a\n' > dgu1.txt && printf 'b\n' > dgu2.txt
check diff-u-header-stamp "$(run diff -u dgu1.txt dgu2.txt | tr -d '\r' | sed -n 1p | grep -c 'dgu1.txt	[0-9]\{4\}-[0-9]\{2\}-[0-9]\{2\} [0-9]\{2\}:[0-9]\{2\}:[0-9]\{2\}\.[0-9]\{9\} [+-][0-9]\{4\}')" "1"
# sed -z (acmesh-d9e): NUL-getrennte Saetze rein und raus.
printf 'a.txt\000b c.txt\000d.txt\000' > z.nul
# tail -z und Fehlermeldungen ohne Zeitstempel (acmesh-bmr).
check tail-zero "$(printf 'a\000b\000c\000d\000' > t4.nul; run tail -z -n2 t4.nul | od -c | head -1 | tr -d '\r')" "0000000   c  \\0   d  \\0"
check tail-zero-in-head "$(printf 'a\000b\000c\000' > t4.nul; run tail -z -n1 t4.nul | od -c | head -1 | tr -d '\r')" "0000000   c  \\0"
check tail-bad-option "$(run tail -Y 2>&1 | tr -d '\r')" "tail: invalid option -- 'Y'"
# cat: GNU's sentence, and the file after a bad one is still copied (acmesh-2rl).
check cat-continues "$(printf 'hi\n' > cm.txt; run cat nofile cm.txt 2>/dev/null | tr -d '\r')" "hi"
check cat-status-after-error "$(printf 'hi\n' > cm.txt; run cat nofile cm.txt >/dev/null 2>&1; echo $?)" "1"
# The text after the colon is the system's own, German on this Windows, so the
# sentence is only compared where GNU's English is the reference.
if [ -z "$WINDOWS" ]; then
check cat-missing "$(run cat nofile 2>&1 | tr -d '\r')" "cat: nofile: No such file or directory"
check tail-missing-file "$(run tail nofile.txt 2>&1 | tr -d '\r')" "tail: cannot open 'nofile.txt' for reading: No such file or directory"
fi
# awk nennt eine unbekannte Funktion wie mawk (acmesh-3xs).
check awk-undefined-function "$(run awk 'BEGIN{x = xor(1,2)}' 2>&1 | tr -d '\r')" "awk: function xor never defined"
# nl, tac und seq gegen GNU (acmesh-efg). The two fixtures are made here because
# the head block further down creates them again.
printf 'a\nb\nc\nd\ne\n' > h1.txt
printf 'b:2:z\nA:1:y\na:3:x\nc:10:w\n' > c1.txt
check nl-default "$(run nl h1.txt | tr -d '\r' | tr '\t\n' '||')" "     1|a|     2|b|     3|c|     4|d|     5|e|"
check nl-all "$(printf 'x\n\ny\n' > h2b.txt; run nl -ba h2b.txt | tr -d '\r' | tr '\t\n' '||')" "     1|x|     2||     3|y|"
check nl-format "$(run nl -n rz -w 3 h1.txt | tr -d '\r' | sed -n 1p | tr '\t' '|')" "001|a"
check nl-sep "$(run nl -s = -w 4 h1.txt | tr -d '\r' | sed -n 1p)" "   1=a"
check nl-start "$(run nl -v 5 -i 2 h1.txt | tr -d '\r' | sed -n 1,2p | tr '\t\n' '||')" "     5|a|     7|b|"
check tac-lines "$(run tac h1.txt | tr -d '\r' | tr '\n' '|')" "e|d|c|b|a|"
# Own fixture: an earlier block writes c1.txt with other content, so a shared
# file would make this check depend on the order of the battery.
check tac-sep "$(printf 'a:b\nc:d\n' > tq.txt; run tac -s : tq.txt | tr -d '\r' | tr '\n' '|')" "d|b|c:a:"
check tac-before "$(printf 'a\nb\nc\n' | run tac -b | od -c | head -1 | tr -d '\r')" "0000000  \\n  \\n   c  \\n   b   a"
check seq-basic "$(run seq 3 | tr -d '\r' | tr '\n' ' ')" "1 2 3 "
check seq-step "$(run seq 1 2 7 | tr -d '\r' | tr '\n' ' ')" "1 3 5 7 "
check seq-decimals "$(run seq 1.0 3 | tr -d '\r' | tr '\n' ' ')" "1.0 2.0 3.0 "
check seq-width "$(run seq -w 8 10 | tr -d '\r' | tr '\n' ' ')" "08 09 10 "
check seq-sep "$(run seq -s, 3 | tr -d '\r')" "1,2,3"
check seq-format "$(run seq -f 'x%02g' 3 | tr -d '\r' | tr '\n' ' ')" "x01 x02 x03 "
# tr und od gegen GNU (acmesh-3qy, acmesh-bly, acmesh-4b7).
check tr-octal-nul "$(printf 'x\000y\n' | run tr '\0' '@' | od -c | head -1)" "0000000   x   @   y  \\n"
check tr-delete-nul "$(printf 'x\000y\n' | run tr -d '\0' | od -c | head -1)" "0000000   x   y  \\n"
check tr-octal-101 "$(printf 'A\n' | run tr '\101' 'Z')" "Z"
check tr-two-classes "$(printf 'A1b\n' | run tr '[:upper:][:digit:]' 'a')" "aab"
check tr-class-range "$(printf 'abc\n' | run tr 'a-c' 'x-z')" "xyz"
check od-c "$(printf 'a\n' > o1.bin; run od -c o1.bin | tr -d '\r' | tr '\n' '|')" "0000000   a  \\n|0000002|"
check od-b "$(printf 'a\n' > o1.bin; run od -b o1.bin | tr -d '\r' | sed -n 1p)" "0000000 141 012"
check od-x "$(printf 'a\n' > o1.bin; run od -x o1.bin | tr -d '\r' | sed -n 1p)" "0000000 0a61"
check od-u1 "$(printf 'a\n' > o1.bin; run od -t u1 o1.bin | tr -d '\r' | sed -n 1p)" "0000000  97  10"
check od-An "$(printf 'a\n' > o1.bin; run od -An -tc o1.bin | tr -d '\r' | sed -n 1p)" "   a  \\n"
check od-Ax "$(printf 'a\n' > o1.bin; run od -A x -c o1.bin | tr -d '\r' | tail -1)" "000002"
check od-N "$(printf 'abc\n' > o2.bin; run od -c -N 2 o2.bin | tr -d '\r' | sed -n 1p)" "0000000   a   b"
# head und tail, Grenzfaelle (acmesh-fqj).
printf 'a\nb\nc\nd\ne\n' > h1.txt
printf 'x\ny' > h2.txt
printf 'a.txt\000b c.txt\000d.txt\000' > h3.nul
check head-last-nonl "$(run head -n 2 h2.txt | od -c | head -1)" "0000000   x  \\n   y"
check head-minus-one "$(run head -n -1 h1.txt | tr -d '\r' | tr '\n' '|')" "a|b|c|d|"
check head-minus-bytes "$(run head -c -3 h1.txt | tr -d '\r' | tr '\n' '|')" "a|b|c|d"
check head-zero "$(run head -n0 h1.txt | wc -c | tr -d ' ')" "0"
check head-old-form "$(run head -2 h1.txt | tr -d '\r' | tr '\n' '|')" "a|b|"
check head-zero-term "$(run head -z -n1 h3.nul | tr '\0' '|')" "a.txt|"
check head-header "$(run head -q -n1 h1.txt h2.txt | tr -d '\r' | tr '\n' '|')" "a|x|"
check head-header-name "$(run head -n1 h1.txt h2.txt | head -1 | tr -d '\r')" "==> h1.txt <=="
check tail-plus-two "$(run tail -n +2 h1.txt | tr -d '\r' | tr '\n' '|')" "b|c|d|e|"
check tail-last-nonl "$(run tail -n 1 h2.txt | od -c | head -1)" "0000000   y"
check tail-minus-bytes "$(run tail -c +3 h1.txt | tr -d '\r' | head -c 2)" "b"
check tail-more-than-lines "$(run tail -n 99 h1.txt | tr -d '\r' | tr '\n' '|')" "a|b|c|d|e|"
# awk gegen mawk 1.3.4 (acmesh-722).
printf 'b:2:z\nA:1:y\na:3:x\nc:10:w\n' > c1.txt
check awk-print "$(run awk '{print $1}' c1.txt | tr -d '\r' | tr '\n' '|')" "b:2:z|A:1:y|a:3:x|c:10:w|"
check awk-field "$(run awk -F: '{print $2}' c1.txt | tr -d '\r' | tr '\n' '|')" "2|1|3|10|"
check awk-nr "$(run awk '{print NR": "$0}' c1.txt | head -2 | tr -d '\r' | tr '\n' '|')" "1: b:2:z|2: A:1:y|"
check awk-end "$(run awk 'END{print NR}' c1.txt | tr -d '\r')" "4"
check awk-gsub "$(printf 'aab\n' | run awk '{n=gsub(/a/, "X"); print n, $0}' | tr -d '\r')" "2 XXb"
check awk-printf "$(run awk 'BEGIN{printf "%05.2f|%s\n", 3.14159, "ok"}' | tr -d '\r')" "03.14|ok"
check awk-array "$(run awk 'BEGIN{a["x"]=1; a["y"]=2; n=0; for (k in a) n++; print n}' | tr -d '\r')" "2"
check awk-v "$(run awk -v n=2 'NR==n' c1.txt | tr -d '\r')" "A:1:y"
# xargs gegen GNU findutils (acmesh-ji5). Auf Windows gibt es kein echo
# als eigenes Programm, also laeuft die Gruppe nur auf dem Linux-Bau.
if [ -z "$WINDOWS" ]; then
check xargs-n1 "$(printf 'a\nb\n' | run xargs -n1 echo | tr '\n' '|')" "a|b|"
check xargs-replace "$(printf 'a\nb\n' | run xargs -I{} echo '[{}]' | tr '\n' '|')" "[a]|[b]|"
check xargs-quotes "$(printf "'a b'\n" | run xargs -n1 echo)" "a b"
check xargs-arg-file "$(printf 'a b\n' > xa.txt; run xargs -a xa.txt -n1 echo | tr '\n' '|')" "a|b|"
check xargs-command-options "$(printf 'a\n' | run xargs sh -c 'echo $0')" "a"
check xargs-exit-123 "$(printf 'a\n' | run xargs sh -c 'exit 5'; printf '%s' "$?")" "123"
check xargs-eof "$(printf 'a\nSTOP\nb\n' | run xargs -E STOP -n1 echo | tr '\n' '|')" "a|"
fi
# cut, comm, split, tee gegen GNU (acmesh-y16).
printf 'b:2:z\nA:1:y\na:3:x\nc:10:w\n' > c1.txt
check cut-field "$(run cut -d: -f2 c1.txt | tr '\n' '|')" "2|1|3|10|"
check cut-range "$(run cut -d: -f1,3 c1.txt | tr '\n' '|')" "b:z|A:y|a:x|c:w|"
check cut-open-range "$(run cut -d: -f2- c1.txt | tr '\n' '|')" "2:z|1:y|3:x|10:w|"
check cut-complement "$(run cut -d: -f2 --complement c1.txt | tr '\n' '|')" "b:z|A:y|a:x|c:w|"
check cut-chars "$(run cut -c1-3 c1.txt | tr '\n' '|')" "b:2|A:1|a:3|c:1|"
check cut-output-delimiter "$(run cut -d: -f2,3 --output-delimiter='|' c1.txt | tr '\n' '/')" "2|z/1|y/3|x/10|w/"
check comm-basic "$(printf 'a\nb\n' > m1.txt; printf 'b\nc\n' > m2.txt; run comm m1.txt m2.txt | cat -A | tr '\n' '/')" "a$/^I^Ib$/^Ic$/"
check comm-suppress "$(printf 'a\nb\n' > m1.txt; printf 'b\nc\n' > m2.txt; run comm -12 m1.txt m2.txt | tr '\n' '|')" "b|"
check comm-total "$(printf 'a\nb\n' > m1.txt; printf 'b\nc\n' > m2.txt; run comm --total m1.txt m2.txt | tail -1)" "1	1	1	total"
check comm-empty-lines "$(printf 'a\n\nb\n' > m1.txt; printf 'a\n\nb\n' > m2.txt; run comm m1.txt m2.txt | wc -l | tr -d ' ')" "3"
check split-lines "$(rm -f x*; run split -l 2 c1.txt && cat xaa xab | wc -l | tr -d ' ')" "4"
check split-suffix "$(rm -f x*; run split -l 2 -d c1.txt && ls x00 x01 | tr '\n' ' ')" "x00 x01 "
check tee-passthrough "$(printf 'a\nb\n' | run tee t9.txt | tr '\n' '|')" "a|b|"
check tee-append "$(printf 'a\n' | run tee t9.txt > /dev/null; printf 'b\n' | run tee -a t9.txt > /dev/null; cat t9.txt | tr '\n' '|')" "a|b|"
# uniq gegen GNU (acmesh-6bq): Zaehlbreite, -d, -u, -f, -s, -w, -z.
printf 'a\na\nb\n' > u1.txt
printf 'x one\ny one\nz two\n' > u2.txt
check uniq-count-width "$(run uniq -c u1.txt | tr -d '\r' | sed 's/$/|/' | tr -d '\n')" "      2 a|      1 b|"
check uniq-repeated "$(run uniq -d u1.txt | tr -d '\r' | tr '\n' '|')" "a|"
check uniq-unique "$(run uniq -u u1.txt | tr -d '\r' | tr '\n' '|')" "b|"
check uniq-skip-fields "$(run uniq -f 1 u2.txt | tr -d '\r' | tr '\n' '|')" "x one|z two|"
check uniq-skip-chars "$(run uniq -s 2 u1.txt | tr -d '\r' | tr '\n' '|')" "a|"
check uniq-check-chars "$(printf 'abcdef\nabcxyz\n' | run uniq -w 3 | tr -d '\r' | tr '\n' '|')" "abcdef|"
check uniq-zero "$(printf 'a\000a\000b\000' | run uniq -z | tr '\0' '|')" "a|b|"
check uniq-bad-option "$(run uniq -Q < u1.txt >/dev/null 2>&1; printf '%s' "$?")" "2"
# grep und Binaerdateien (acmesh-bde): Meldung auf stderr, kein Zeilenausdruck.
printf 'X\000ABC\n' > gbin.bin
check grep-binary-message "$(run grep ABC gbin.bin 2>&1 >/dev/null | tr -d '\r' | sed 's/.*: //')" "binary file matches"
check grep-binary-silent-stdout "$(run grep ABC gbin.bin 2>/dev/null | wc -c | tr -d ' ')" "0"
check grep-binary-rc "$(run grep ABC gbin.bin >/dev/null 2>&1; printf '%s' "$?")" "0"
check grep-binary-a "$(run grep -a ABC gbin.bin | tr '\0' '@')" "X@ABC"
check grep-binary-I "$(run grep -I ABC gbin.bin >/dev/null 2>&1; printf '%s' "$?")" "1"
check grep-binary-count "$(run grep -c ABC gbin.bin | tr -d '\r')" "1"
check grep-binary-L-rc "$(run grep -L ABC gbin.bin >/dev/null 2>&1; printf '%s' "$?")" "0"
check sed-z-prefix "$(run sed -z 's/^/X/' z.nul | tr '\0' '|')" "Xa.txt|Xb c.txt|Xd.txt|"
check sed-z-count "$(run sed -z 's/^/X/' z.nul | tr -cd '\0' | wc -c | tr -d ' ')" "3"
check sed-z-delete "$(run sed -z 2d z.nul | tr '\0' '|')" "a.txt|d.txt|"
check sed-z-glued "$(run sed -zn p z.nul | tr '\0' '|')" "a.txt|b c.txt|d.txt|"
check sed-z-last "$(run sed -z 's/txt$/END/' z.nul | tr '\0' '|')" "a.END|b c.END|d.END|"
check diff-u-header-stamp "$(run diff -u dgu1.txt dgu2.txt | tr -d '\r' | sed -n 1p | grep -c 'dgu1.txt	[0-9]\{4\}-[0-9]\{2\}-[0-9]\{2\} [0-9]\{2\}:[0-9]\{2\}:[0-9]\{2\}\.[0-9]\{9\} [+-][0-9]\{4\}')" "1"
# sed -z (acmesh-d9e): NUL-getrennte Saetze rein und raus.
printf 'a.txt\000b c.txt\000d.txt\000' > z.nul
check diff-u-plus-line "$(run diff -u dgu1.txt dgu2.txt | tr -d '\r' | sed -n 2p | grep -c '^+++ dgu2.txt	')" "1"
check df-h-size-header "$(run df -h | tr -d '\r' | head -1 | tr -s ' ' | cut -d' ' -f2)" "Size"
check df-blocks-header "$(run df | tr -d '\r' | head -1 | tr -s ' ' | cut -d' ' -f2)" "1K-blocks"
check df-posix-header "$(run df -P | tr -d '\r' | head -1 | tr -s ' ' | cut -d' ' -f2)" "1024-blocks"
check df-posix-capacity "$(run df -P | tr -d '\r' | head -1 | tr -s ' ' | cut -d' ' -f5)" "Capacity"
# Ein fester Pfad wie "/" gibt es auf Windows nicht; dort zaehlt die erste Zeile.
case "$BIN" in
*.exe) check df-fields-in-columns "$(run df -h | tr -d '\r' | sed -n 2p | awk '{print NF}')" "6" ;;
*) check df-fields-in-columns "$(run df -h / | tr -d '\r' | tail -1 | awk '{print NF}')" "6" ;;
esac
# ls -t mit Nanosekunden (acmesh-fgu): drei Dateien in derselben Sekunde, die
# Reihenfolge widerspricht dem Namen.
mkdir -p lstdir && (printf x > lstdir/x.tar; sleep 0.3; printf y > lstdir/x.tgz; sleep 0.3; printf z > lstdir/y.txt)
check ls-t-nanoseconds "$(run ls -t lstdir | tr -d '\r' | head -1)" "y.txt"
check ls-tr-nanoseconds "$(run ls -tr lstdir | tr -d '\r' | head -1)" "x.tar"
# rg mit PCRE2 (acmesh-4v0): -P, --engine=pcre2, --engine=auto, und der Fehler
# ohne -P. Die Engine selbst pruefen TestPCREEngine und TestPCREWordAndCase.
printf 'beta alpha\nalpha\n' > fdir/r.txt
check rg-pcre-lookahead "$(cd fdir && run rg -P 'beta(?= alpha)' r.txt | tr -d '\r')" "beta alpha"
check rg-pcre-lookbehind "$(cd fdir && run rg --engine=pcre2 '(?<=beta )alpha' r.txt | tr -d '\r')" "beta alpha"
check rg-pcre-auto "$(cd fdir && run rg --engine=auto 'alpha\K' r.txt | tr -d '\r' | head -1)" "beta alpha"
check rg-re2-rejects "$(cd fdir && run rg 'beta(?= alpha)' r.txt 2>/dev/null; printf '%s' "$?")" "2"
check rg-engine-default-rejects "$(cd fdir && run rg --engine=default 'beta(?= alpha)' r.txt 2>/dev/null; printf '%s' "$?")" "2"
check rg-bad-engine "$(cd fdir && run rg --engine=nope alpha r.txt 2>/dev/null; printf '%s' "$?")" "2"
check rg-pcre-word "$(cd fdir && run rg -Pw 'beta' r.txt | tr -d '\r')" "beta alpha"
# wc: -L und die Spaltenbreite (acmesh-j66, acmesh-154). Fuer die Breite sorgt
# eine Datei mit vielen Bytes und wenigen Zeilen: GNU richtet dann auch die
# Zeilenspalte an deren Breite aus.
printf 'a\nbb\nccc\n' > wdir_a.txt
head -c 5000 /dev/zero | tr '\0' 'z' > wdir_z.txt
check wc-max-line "$(run wc -L wdir_a.txt | tr -d '\r' | awk '{print $1}')" "3"
check wc-longest-from-stdin "$(printf 'abcde' | run wc -L | tr -d '\r')" "5"
check wc-columns "$(run wc -l wdir_a.txt wdir_z.txt | tr -d '\r' | head -1 | cut -c1-4)" "   3"
check wc-total-row "$(run wc -l wdir_a.txt wdir_z.txt | tr -d '\r' | tail -1 | awk '{print $1}')" "3"
check wc-single-no-pad "$(run wc -l wdir_a.txt | tr -d '\r' | cut -c1)" "3"
# base64: -w0 ohne Umbruch und -d ohne Auffuellung (acmesh-449).
check base64-w0-no-newline "$(printf 'hello world' | run base64 -w0 | od -c | head -1 | grep -c '\\n')" "0"
check base64-decode-unpadded "$(printf 'aGVsbG8' > b64_p.txt && run base64 -d b64_p.txt)" "hello"
check base64-decode-unpadded-status "$(printf 'aGVsbG8' > b64_p.txt && run base64 -d b64_p.txt >/dev/null; printf '%s' "$?")" "0"
check base64-decode-space-is-garbage "$(printf 'aGVs bG8' > b64_s.txt && run base64 -d b64_s.txt 2>/dev/null >/dev/null; printf '%s' "$?")" "1"
check base64-decode-ignore-space "$(printf 'aGVs bG8' | run base64 -di)" "hello"
# grep behaviour lives in Go since acmesh-4aj (internal/forked/grep):
#   -E, -w, -i            -> TestExtendedRegexpAndWordMatch, TestIgnoreCaseIsAbsolute
#   -A/-B/-C mit "--"     -> TestContextGroupsAreSeparatedByDashes, TestAdjacentWindowsAreOneGroup
#   -n mit Kontext        -> TestContextAndNumbersUseTheDashSeparator
#   -o, -q, -v, -c        -> TestOnlyMatchAndNumbers, TestQuietPrintsNothingAndKeepsTheStatus, TestInvertSelectsTheRest
#   0/1/2                 -> TestExitStatus, TestMissingFileIsTwoWithGNUsWording, TestBrokenPatternIsTwo
#   -r gegen -R (Symlink) -> TestDashRAndDashRDifferOnSymlinks
#   -l, -h, -F, Langformen-> TestFilesWithMatchesNamesOnly, TestFilesWithMatchesOnAStreamAndOnAMiss, TestFixedStringsAreNotPatterns, TestLongOptionNamesMatchTheirShortSpelling
rm -rf fdir g.txt

# head is a forked u-root command: the short form and -q are the reason
printf '1\n2\n3\n4\n5\n6\n7\n' > h.txt
# head behaviour lives in Go since acmesh-795 (internal/forked/head):
#   -5 Kurzform, -n -2, -c -3 -> TestShortFormAndQuiet, TestNegativeCountKeepsEverythingButTheEnd, TestNegativeBytesKeepTheHead
#   -n 0 / -c 0               -> TestZeroPrintsNothing
#   Kopfzeilen, -q, -v        -> TestHeadersFollowTheLastOption
#   kurze Datei, fehlende Datei, -n gegen -c -> TestShortFileIsNotAnError, TestErrorsAreNotSuccess
rm -f h.txt h1.txt h2.txt

# files
# truncate und split laufen ganz in Go seit acmesh-j4f: das Kuerzen ist
# TestShrinkAndGrow, Anzahl und Groesse der Teile TestLinesSplitIntoParts, der
# Verzicht auf einen dritten Buchstaben TestTooManyPartsIsRefused.

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
# -Z is a real flag since acmesh-730, so the unknown one is another letter.
checkcode curl-unknown-flag 2 "$(run curl -Y http://example.com >/dev/null 2>&1; echo $?)"
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

# tracedeps: only Linux has ptrace, and only amd64 has the register layout it reads.
case "$BIN" in
*.exe) ;;
*)
  printf '#!/bin/sh\ncat td-in.txt\n' > td.sh && printf 'hallo\n' > td-in.txt
  chmod +x td.sh
  check tracedeps-script "1" "$(run tracedeps ./td.sh 2>/dev/null | tr -d '\r' | grep -c 'td-in.txt' | tr -d ' ')"
  check tracedeps-program "1" "$(run tracedeps ./td.sh 2>/dev/null | tr -d '\r' | grep -c 'td.sh' | tr -d ' ')"
  checkcode tracedeps-usage 2 "$(run tracedeps >/dev/null 2>&1; echo $?)"
  rm -f td.sh td-in.txt
  ;;
esac
case "$BIN" in
*.exe)
  check tracedeps-says-no-windows "ja" "$(run tracedeps ls 2>&1 | grep -q 'linux/amd64 only' && echo ja)"
  ;;
esac

# httpd: serve a directory, ask for a file and for a missing one. The root is
# given as an absolute path: the .exe resolves a relative one against its own
# view of the working directory, and that is not what this check is about.
HD_DIR="$PWD/hd-test"
mkdir -p "$HD_DIR" && printf '<h1>hd</h1>' > "$HD_DIR/index.html"
# A port of this run: a fixed one collides with a server left over from an
# earlier run, and then the check answers for the wrong directory.
HTTPD_PORT=$((18100 + ($$ % 400)))
# A port that somebody else already answers on was the cause of false failures
# (acmesh-ert): step on until nothing answers, instead of hoping.
rent=0
while [ $rent -lt 20 ] && curl -s -m 1 -o /dev/null "http://127.0.0.1:$HTTPD_PORT/" 2>/dev/null; do
  HTTPD_PORT=$((HTTPD_PORT + 7))
  rent=$((rent + 1))
done
run httpd -f -p 127.0.0.1:$HTTPD_PORT -h "$HD_DIR" >/dev/null 2>&1 &
HTTPD_PID=$!
# The .exe starts slower through the WSL interop than the Linux binary: wait
# for the port instead of guessing a second (acmesh-m5j).
tries=0
while [ $tries -lt 40 ]; do
  # Our own page, not just any answer: a foreign server would make this pass.
  curl -s -m 1 "http://127.0.0.1:$HTTPD_PORT/index.html" 2>/dev/null | grep -q hd && break
  sleep 0.5
  tries=$((tries+1))
done
if [ $tries -ge 40 ]; then
  printf 'httpd: port %d did not serve the test page in 20 s\n' "$HTTPD_PORT" >&2
fi
if command -v curl >/dev/null 2>&1; then
  check httpd-file "1" "$(curl -s -m 5 http://127.0.0.1:$HTTPD_PORT/index.html 2>/dev/null | grep -c hd)"
  check httpd-404 "404" "$(curl -s -m 5 -o /dev/null -w '%{http_code}' http://127.0.0.1:$HTTPD_PORT/nix 2>/dev/null)"
fi

# -w and -i have to look like curl's, because agents parse them (acmesh-ly7).
# Checked against curl 8 in a Debian container during the same session.
U="http://127.0.0.1:$HTTPD_PORT/index.html"
check curl-writeout-code "200" "$(run curl -s -o /dev/null -w '%{http_code}' "$U" | tr -d '\r')"
check curl-writeout-size "11" "$(run curl -s -o /dev/null -w '%{size_download}' "$U" | tr -d '\r')"
check curl-include-status "HTTP/1.1 200 OK" "$(run curl -s -i "$U" | tr -d '\r' | head -1)"
check curl-include-no-prefix "0" "$(run curl -s -i "$U" | tr -d '\r' | grep -c '^< ')"
check curl-include-has-type "1" "$(run curl -s -i "$U" | tr -d '\r' | grep -c '^content-type:')"
# The default agent must not say Go (acmesh-ijz); -v shows the request headers.
# wget has no such output, its default agent is pinned by a Go test instead.
check curl-default-agent "1" "$(run curl -s -v "$U" 2>&1 | grep -c 'akmesh/0.1 (curl compatible)')"
# A wrong profile must fail loudly instead of quietly using Go's handshake.
checkcode curl-impersonate-unknown 2 "$(run curl -s --impersonate netscape "$U" >/dev/null 2>&1; echo $?)"
kill $HTTPD_PID 2>/dev/null
# The .exe is wrapped by /init, so the shell's $! is not the server: match the
# port as well, or the next run finds a server on a closed directory.
pkill -f "httpd -f -p 127.0.0.1:$HTTPD_PORT" 2>/dev/null

# Without -f the command has to go to the background, name its pid and leave the
# caller free; the page must be there afterwards (acmesh-rds). The pid comes from
# the message, not from $!, because the child detaches itself.
BG_PORT=$((HTTPD_PORT + 1))
# Same rule as above: a port somebody else answers on is not ours (acmesh-ert).
brt=0
while [ $brt -lt 20 ] && curl -s -m 1 -o /dev/null "http://127.0.0.1:$BG_PORT/" 2>/dev/null; do
  BG_PORT=$((BG_PORT + 7))
  brt=$((brt + 1))
done
BG_OUT=$($BIN httpd -p 127.0.0.1:$BG_PORT -h "$HD_DIR" 2>&1)
check httpd-detach-pid "1" "$(printf '%s' "$BG_OUT" | grep -c 'pid [0-9]')"
BG_PID=$(printf '%s' "$BG_OUT" | grep -o 'pid [0-9]*' | cut -d' ' -f2)
tries=0
while [ $tries -lt 20 ]; do
  curl -s -m 1 "http://127.0.0.1:$BG_PORT/index.html" 2>/dev/null | grep -q hd && break
  sleep 0.5
  tries=$((tries+1))
done
if [ $tries -ge 20 ]; then
  printf 'httpd detach: port %d did not serve the test page in 10 s (pid %s)\n' "$BG_PORT" "$BG_PID" >&2
fi
check httpd-detach-serves "1" "$(curl -s -m 5 http://127.0.0.1:$BG_PORT/index.html 2>/dev/null | grep -c hd)"
# A detached server that cannot be stopped is worse than one that never starts:
# it holds the port and the next run finds it. Kill it and prove it is gone.
# Stop it with our own kill: the pid printed by the .exe is a Windows pid, and
# the shell's kill does not reach that from WSL (measured: the server kept
# answering). Our kill knows both worlds.
"$BIN" kill "$BG_PID" 2>/dev/null
kill "$BG_PID" 2>/dev/null
pkill -f "httpd -f -p 127.0.0.1:$BG_PORT" 2>/dev/null
# The .exe takes a moment to disappear through the interop layer, so poll
# instead of looking once (measured: 200 right after kill, 000 a second later).
stop_code=200
stops=0
while [ $stops -lt 20 ]; do
  stop_code=$(curl -s -m 2 -o /dev/null -w '%{http_code}' "http://127.0.0.1:$BG_PORT/" 2>/dev/null)
  [ "$stop_code" = "000" ] && break
  sleep 0.5
  stops=$((stops+1))
done
check httpd-detach-stopped "000" "$stop_code"
rm -rf "$HD_DIR"

# rtf2md: a paragraph, a bold word and a code page byte.
if command -v python3 >/dev/null 2>&1; then
  python3 /root/akmesh/scripts/make-test-rtf.py r-test.rtf
  check rtf2md-paragraph "Eins" "$(run rtf2md r-test.rtf | tr -d '\r' | head -1)"
  check rtf2md-bold "**Zwei**ä" "$(run rtf2md r-test.rtf | tr -d '\r' | tail -1)"
  check rtf2md-not-rtf "1" "$(printf 'kein rtf' | run rtf2md >/dev/null 2>&1; echo $?)"
  rm -f r-test.rtf
fi

# pdf2md: a PDF without compression is the smallest thing a reader has to
# handle; the text operator is the claim.
if command -v python3 >/dev/null 2>&1; then
  python3 /root/akmesh/scripts/make-test-pdf.py p-test.pdf
  check pdf2md-text "Hallo PDF" "$(run pdf2md p-test.pdf | tr -d '\r' | head -1)"
  python3 /root/akmesh/scripts/make-test-pdf-table.py pt-test.pdf
  check pdf2md-table "| Name: | Anna Beispiel |" "$(run pdf2md pt-test.pdf | tr -d '\r' | head -1)"
  python3 /root/akmesh/scripts/make-test-pdf-labels.py pl-test.pdf
  check pdf2md-labels "| Ort: | Pumpkin Mart |" "$(run pdf2md pl-test.pdf | tr -d '\r' | head -1)"
  check pdf2md-labels-rows "7" "$(run pdf2md pl-test.pdf | tr -d '\r' | grep -c '|')"
  python3 /root/akmesh/scripts/make-test-pdf-form.py pf-test.pdf
  check pdf2md-form-xobject "Feldtext" "$(run pdf2md pf-test.pdf | tr -d '\r' | head -1)"
  check pdf2md-not-pdf "1" "$(printf 'kein pdf' | run pdf2md >/dev/null 2>&1; echo $?)"
  rm -f p-test.pdf pt-test.pdf pf-test.pdf pl-test.pdf
fi

# md2epub / epub2md: the book and its reader, both in the binary.
printf '# Erstes\n\nEin Absatz.\n\n| A | B |\n| --- | --- |\n| 1 | 2 |\n\n# Zweites\n\nEnde.\n' > b.md
run md2epub -o b.epub --title "Probe" b.md >/dev/null 2>&1
check md2epub-is-zip "PK" "$(head -c 2 b.epub)"
# The first entry of an EPUB is the mimetype, stored, and a reader checks it
# before anything else.
check md2epub-mimetype "application/epub+zip" "$(unzip -p b.epub mimetype 2>/dev/null | tr -d '\r')"
check md2epub-mimetype-first "mimetype" "$(unzip -l b.epub 2>/dev/null | sed -n '4p' | awk '{print $4}')"
check md2epub-roundtrip "# Erstes" "$(run epub2md b.epub | tr -d '\r' | grep -m1 '^#' )"
check epub2md-table "| A | B |" "$(run epub2md b.epub | tr -d '\r' | grep -o '| A | B |' | head -1)"
check epub2md-chapter-count "2" "$(run epub2md b.epub | tr -d '\r' | grep -c '^# ')"
check epub2md-refuses-zip "1" "$(printf 'kein epub' | run epub2md >/dev/null 2>&1; echo $?)"
rm -f b.md b.epub

# md2docx: Markdown to a Word document, written by our own package.
printf '# Titel\n\nEin **fetter** Satz mit Umlauten: \303\244\303\266\303\274\303\237.\n\n- eins\n- zwei\n\n| Name | Ort |\n| --- | --- |\n| Anna | Bremen |\n' > m.docx.md
run md2docx -o m-out.docx m.docx.md >/dev/null 2>&1
check md2docx-is-zip "PK" "$(head -c 2 m-out.docx)"
# The round trip through our own reader: the document has to come back as the
# Markdown it was written from.
check md2docx-heading "# Titel" "$(run docx2md m-out.docx | tr -d '\r' | head -1)"
check md2docx-bold "**fetter**" "$(run docx2md m-out.docx | tr -d '\r' | grep -o '\*\*fetter\*\*' | head -1)"
check md2docx-list "- eins" "$(run docx2md m-out.docx | tr -d '\r' | grep -o -- '- eins' | head -1)"
check md2docx-table "| Anna | Bremen |" "$(run docx2md m-out.docx | tr -d '\r' | grep -o '| Anna | Bremen |' | head -1)"
check md2docx-unknown-option "2" "$(run md2docx --frobnicate >/dev/null 2>&1; echo $?)"
rm -f m.docx.md m-out.docx

# md2pdf: Markdown to PDF, written by the slimmed goldmark-pdf fork.
printf '# Titel\n\nEin Absatz mit Umlauten: äöüß.\n' > m.pdf.md
run md2pdf -o m-out.pdf m.pdf.md >/dev/null 2>&1
check md2pdf-is-pdf "%PDF-1." "$(head -c 8 m-out.pdf | cut -c1-7)"
check md2pdf-has-eof "1" "$(grep -c '%%EOF' m-out.pdf | tr -d ' ')"
# The round trip is the real proof: our own reader must find the text again.
check md2pdf-roundtrip "Titel" "$(run pdf2md m-out.pdf | tr -d '\r' | grep -o 'Titel' | head -1)"
check md2pdf-umlaut "äöüß" "$(run pdf2md m-out.pdf | tr -d '\r' | grep -o 'äöüß' | head -1)"
check md2pdf-unknown-option "2" "$(run md2pdf --frobnicate >/dev/null 2>&1; echo $?)"
rm -f m.pdf.md m-out.pdf

# md2txt-l: the plain text of Markdown (the lightweight variant, the way pandoc -t plain writes it.
printf '# Titel\n\nEin **fetter** [Link](http://x/y).\n' > m-test.md
check md2txt-l-heading "Titel" "$(run md2txt-l m-test.md | tr -d '\r' | head -1)"
check md2txt-l-inline "Ein fetter Link." "$(run md2txt-l m-test.md | tr -d '\r' | sed -n '3p')"
printf '| a | b |\n| --- | --- |\n| 1 | 2 |\n' > m-tab.md
check md2txt-l-table "  a   b" "$(run md2txt-l m-tab.md | tr -d '\r' | head -1)"
printf '%s\n' "Ein Absatz, der bewusst viel laenger als zweiundsiebzig Zeichen ist, damit der Umbruch greift und die Zeile danach kuerzer bleibt als das Limit." > m-wrap.md
check md2txt-l-wrap "0" "$(run md2txt-l m-wrap.md | awk 'length($0)>72' | wc -l | tr -d ' ')"
rm -f m-test.md m-tab.md m-wrap.md

# docx2md: a minimal docx (a zip with word/document.xml) in, Markdown out.
if command -v python3 >/dev/null 2>&1; then
  python3 /root/akmesh/scripts/make-test-docx.py dx-test.docx
  check docx2md-heading "## Kapitel" "$(run docx2md dx-test.docx | tr -d '\r' | head -1)"
  check docx2md-text "Text" "$(run docx2md dx-test.docx | tr -d '\r' | tail -1)"
  check docx2md-not-a-docx "1" "$(printf 'kein zip' | run docx2md >/dev/null 2>&1; echo $?)"
  rm -f dx-test.docx
fi

# html2md: a fragment in, Markdown out, on both targets.
check html2md-heading "### Titel" "$(printf '<h3>Titel</h3>' | run html2md | tr -d '\r')"
check html2md-link "1" "$(printf '<p><a href="http://x/y">L</a></p>' | run html2md | tr -d '\r' | grep -c '(http://x/y)')"
check html2md-escape "5" "$(printf '<p>5*3</p>' | run html2md | wc -c | tr -d ' ')"
check html2md-stdin-empty "0" "$(printf '' | run html2md | wc -c | tr -d ' ')"

# ps, kill, killall
check ps-fields "PID COMM" "$(run ps -eo pid,comm | head -1 | tr -s ' ' | tr -d '\r' | sed 's/^ //;s/ $//')"
check ps-ef-ppid "PPID" "$(run ps -ef | head -1 | tr -d '\r' | grep -o PPID | head -1)"
check ps-has-rows "ja" "$([ "$(run ps -eo pid | wc -l)" -gt 1 ] && echo ja)"
check ps-unknown-field "2" "$(run ps -eo bogus >/dev/null 2>&1; echo $?)"
checkcode kill-bad-signal 2 "$(run kill -NOSUCHSIGNAL 1 >/dev/null 2>&1; echo $?)"
checkcode kill-bad-pid 2 "$(run kill keinpid >/dev/null 2>&1; echo $?)"
checkcode killall-no-match 1 "$(run killall gibtsnicht 2>/dev/null; echo $?)"
check killall-list-term "1" "$(run killall -l 2>&1 | grep -c '^TERM$')"
checkcode killall-list-exit 0 "$(run killall -l >/dev/null 2>&1; echo $?)"

# pgrep and pkill (acmesh-2qr): the pair every script uses to look before killing
checkcode pgrep-no-match 1 "$(run pgrep -x akmesh-gibtsnicht 2>/dev/null; echo $?)"
checkcode pgrep-no-pattern 2 "$(run pgrep >/dev/null 2>&1; echo $?)"
check pgrep-help-exit "0" "$(run pgrep --help >/dev/null 2>&1; echo $?)"
check pkill-help-exit "0" "$(run pkill --help >/dev/null 2>&1; echo $?)"
# pgrep sees the process it runs in. The name comes from the binary, not from
# "akmesh": on Windows the battery runs a copy called acmesh-under-test.exe.
PGNAME=$(basename "$BIN")
PGNAME=${PGNAME%.exe}
check pgrep-finds-own-name "ja" "$(run pgrep -x "$PGNAME" 2>/dev/null | tr -d '\r' | head -1 | grep -q '^[0-9][0-9]*$' && echo ja)"
check pgrep-count "1" "$(run pgrep -c -x "$PGNAME" 2>/dev/null | tr -d '\r' | tr -dc '0-9' | grep -c .)"

# tar: -O, Verzeichnisse mit Schraegstrich, Archiv nicht in sich selbst (acmesh-vc3)
TDIR=$(mktemp -d 2>/dev/null || echo /tmp/akmesh-tar)
mkdir -p "$TDIR/d"
printf 'aaa\n' > "$TDIR/d/a.txt"
( cd "$TDIR" && run tar -cf x.tar d >/dev/null 2>&1 )
check tar-dir-slash "d/" "$(run tar -tf "$TDIR/x.tar" 2>/dev/null | head -n 1 | tr -d '\r')"
check tar-to-stdout "aaa" "$(run tar -xOf "$TDIR/x.tar" d/a.txt 2>/dev/null | head -n 1 | tr -d '\r')"
check tar-to-stdout-all "aaa" "$(run tar -xOf "$TDIR/x.tar" 2>/dev/null | head -n 1 | tr -d '\r')"
checkcode tar-self-archive-rc 0 "$(run tar -cf "$TDIR/d/self.tar" "$TDIR/d" >/dev/null 2>&1; echo $?)"
check tar-self-archive-skip "ja" "$(run tar -tf "$TDIR/d/self.tar" 2>/dev/null | grep -q self.tar || echo ja)"
case "$BIN" in
*.exe) ;; # a WSL path like /tmp/x is not absolute for the Windows binary
*) ABS=$(cd "$TDIR/d" && pwd)
   check tar-absolute-strips-slash "ja" "$(run tar -cf "$TDIR/abs.tar" "$ABS" 2>/dev/null; run tar -tf "$TDIR/abs.tar" 2>/dev/null | grep -qvE '^(/|[A-Za-z]:)' && echo ja)" ;;
esac
rm -rf "$TDIR"

# du: Verzeichnisbloecke, -b (scheinbare Groesse), -x (acmesh-yd7)
TDIR=$(mktemp -d 2>/dev/null || echo /tmp/akmesh-du)
mkdir -p "$TDIR/sub"
printf 'aaaa\n' > "$TDIR/a.txt"
printf 'c\n' > "$TDIR/sub/c.txt"
check du-apparent-total "7" "$(run du -b -s "$TDIR" 2>/dev/null | cut -f1 | tr -d '\r')"
check du-apparent-parent "7" "$(run du -b -s "$TDIR/sub/.." 2>/dev/null | cut -f1 | tr -d '\r')"
check du-child-apparent "2" "$(run du -b -s "$TDIR/sub/c.txt" 2>/dev/null | cut -f1 | tr -d '\r')"
check du-dot-prefix "ja" "$(cd "$TDIR" && run du . 2>/dev/null | grep -qE '^[0-9]+[[:space:]]+\.[\\/]sub' && echo ja)"
checkcode du-one-file-system 0 "$(run du -x -s "$TDIR" >/dev/null 2>&1; echo $?)"
checkcode du-bad-depth 2 "$(run du -d nope "$TDIR" >/dev/null 2>&1; echo $?)"
rm -rf "$TDIR"

# grep -m -x -H -a -z --include --exclude (acmesh-v6r)
TDIR=$(mktemp -d 2>/dev/null || echo /tmp/akmesh-grep7)
mkdir -p "$TDIR/sub"
printf 'AB\nAC\n' > "$TDIR/a.txt"
printf 'delta\n' > "$TDIR/sub/c.txt"
printf 'needle\n' > "$TDIR/b.log"
printf 'needle\n' > "$TDIR/d.txt"
check grep-m1 "AB" "$(run grep -m1 A "$TDIR/a.txt" 2>/dev/null | tr -d '\r')"
check grep-x "AC" "$(run grep -x AC "$TDIR/a.txt" 2>/dev/null | tr -d '\r')"
check grep-x-nomatch "ja" "$(run grep -x A "$TDIR/a.txt" >/dev/null 2>&1 || echo ja)"
check grep-H "ja" "$(run grep -H AB "$TDIR/a.txt" 2>/dev/null | grep -q 'a.txt:AB' && echo ja)"
check grep-z "ja" "$(printf 'a\0b\0' | run grep -z b 2>/dev/null | od -An -c | grep -q 'b  \\0' && echo ja)"
check grep-include "ja" "$(run grep -r --include=*.log needle "$TDIR" 2>/dev/null | grep -q 'b.log' && echo ja)"
check grep-exclude "ja" "$(o=$(run grep -r --exclude=*.log needle "$TDIR" 2>/dev/null); echo "$o" | grep -q 'd.txt' && ! echo "$o" | grep -q 'b.log' && echo ja)"
rm -rf "$TDIR"

# grep ohne -E ist BRE wie bei GNU (acmesh-4gi)
TDIR=$(mktemp -d 2>/dev/null || echo /tmp/akmesh-bre)
mkdir -p "$TDIR"
printf 'aab\naxb\n' > "$TDIR/b.txt"
check grep-bre-literal-plus "ja" "$(run grep 'a+b' "$TDIR/b.txt" >/dev/null 2>&1 || echo ja)"
check grep-bre-quantified "aab" "$(run grep 'a\+b' "$TDIR/b.txt" 2>/dev/null | head -n 1 | tr -d '\r')"
check grep-bre-group "aab" "$(run grep 'a\(a\)b' "$TDIR/b.txt" 2>/dev/null | head -n 1 | tr -d '\r')"
check grep-bre-alternation "2" "$(run grep -c 'a\|b' "$TDIR/b.txt" 2>/dev/null | tr -d '\r')"
check grep-bre-interval "aab" "$(run grep 'a\{2\}' "$TDIR/b.txt" 2>/dev/null | head -n 1 | tr -d '\r')"
checkcode grep-bre-unclosed-bracket 2 "$(run grep '[' "$TDIR/b.txt" >/dev/null 2>&1; echo $?)"
check grep-ere-still-ere "aab" "$(run grep -E 'a+b' "$TDIR/b.txt" 2>/dev/null | head -n 1 | tr -d '\r')"
rm -rf "$TDIR"

# grep -P: PCRE2 hinter dem Schalter (acmesh-b87)
TDIR=$(mktemp -d 2>/dev/null || echo /tmp/akmesh-pcre)
mkdir -p "$TDIR"
printf 'foobar\nfoobaz\n' > "$TDIR/p.txt"
check grep-pcre-lookahead "foobar" "$(run grep -P 'foo(?=bar)' "$TDIR/p.txt" 2>/dev/null | tr -d '\r')"
printf 'xab\n' > "$TDIR/k.txt"
check grep-pcre-k "b" "$(run grep -P -o 'a\Kb' "$TDIR/k.txt" 2>/dev/null | head -n 1 | tr -d '\r')"
printf 'aaab\n' > "$TDIR/poss.txt"
check grep-pcre-possessive "ja" "$(run grep -P 'a++b' "$TDIR/poss.txt" 2>/dev/null | grep -q aaab && echo ja)"
checkcode grep-pcre-bad-pattern 2 "$(run grep -P 'a(b' "$TDIR/p.txt" >/dev/null 2>&1; echo $?)"
check grep-re2-rejects-lookahead "ja" "$(run grep '(?=x)' "$TDIR/p.txt" >/dev/null 2>&1 || echo ja)"
rm -rf "$TDIR"

# date in den GNU-Formen (acmesh-xli)
check date-epoch-zero "0" "$(run date -u -d @0 +%s 2>/dev/null | tr -d '\r')"
check date-fixed "1577923200" "$(run date -u -d 2020-01-02 +%s 2>/dev/null | tr -d '\r')"
check date-iso-date "2023-11-14" "$(run date -u -d @1699999999 +%F 2>/dev/null | tr -d '\r')"
check date-rel-last-week "ja" "$(run date -u -d "last week" +%s 2>/dev/null | grep -qE '^[0-9]+$' && echo ja)"
check date-rfc2822 "ja" "$(run date -u -R 2>/dev/null | grep -q '+0000' && echo ja)"
check date-now-is-epoch "ja" "$(run date -u +%s 2>/dev/null | grep -qE '^[0-9]{10}$' && echo ja)"
checkcode date-bad-string 1 "$(run date -d nonsense +%F >/dev/null 2>&1; echo $?)"

# tail in den Formen, die Agents schreiben (acmesh-uzd)
TDIR=$(mktemp -d 2>/dev/null || echo /tmp/akmesh-tail)
mkdir -p "$TDIR"
printf 'alpha\nbeta\ngamma\n' > "$TDIR/a.txt"
printf 'x\ny\n' > "$TDIR/b.txt"
check tail-dash3 "gamma" "$(run tail -3 "$TDIR/a.txt" 2>/dev/null | tail -n 1 | tr -d '\r')"
check tail-c "amma" "$(run tail -c 5 "$TDIR/a.txt" 2>/dev/null | tr -d '\r')"
check tail-cplus "gamma" "$(run tail -c +12 "$TDIR/a.txt" 2>/dev/null | tr -d '\r')"
check tail-nplus "beta" "$(run tail -n +2 "$TDIR/a.txt" 2>/dev/null | head -n 1 | tr -d '\r')"
check tail-quiet "beta" "$(run tail -q -2 "$TDIR/a.txt" "$TDIR/b.txt" 2>/dev/null | head -n 1 | tr -d '\r')"
check tail-header "==> " "$(run tail -v -1 "$TDIR/a.txt" 2>/dev/null | head -n 1 | cut -c1-4 | tr -d '\r')"
rm -rf "$TDIR"
# chown, od, sum, unzip, setsid, su, passwd, mkpasswd (acmesh-6p8, 2yc, vgr, f1e, a4d)
checkcode chown-help 0 "$(run chown --help >/dev/null 2>&1; echo $?)"
check od-hex-bytes " 41 42" "$(printf 'AB' | run od -An -tx1 | tr -d '\r')"
check od-address "0000000" "$(printf 'AB' | run od | tr -d '\r' | head -1 | cut -c1-7)"
check od-end-address "0000002" "$(printf 'AB' | run od | tr -d '\r' | tail -1)"
check sum-bsd "36979     1 -" "$(printf 'hello\n' | run sum | tr -d '\r' | sed 's/ *$//')"
check sum-sysv "542 1 -" "$(printf 'hello\n' | run sum -s | tr -d '\r' | sed 's/ *$//')"
ZIP_DIR=$(mktemp -d 2>/dev/null || echo /tmp/akmesh-zip)
mkdir -p "$ZIP_DIR"
mkdir -p "$ZIP_DIR/out"
printf '%s' 'UEsDBBQAAAAIALuGd116em/tBQAAAAMAAAAGAAAAaGkudHh0y8jkAgBQSwECFAMUAAAACAC7hnddenpv7QUAAAADAAAABgAAAAAAAAAAAAAAgAEAAAAAaGkudHh0UEsFBgAAAAABAAEANAAAACkAAAAAAA==' | run base64 -d > "$ZIP_DIR/t.zip" 2>/dev/null
check unzip-extracts "hi" "$(run unzip -q -d "$ZIP_DIR/out" "$ZIP_DIR/t.zip" >/dev/null 2>&1; cat "$ZIP_DIR/out/hi.txt" 2>/dev/null | tr -d '\r' | tr -d '\n')"
check unzip-list "1" "$(run unzip -l "$ZIP_DIR/t.zip" 2>/dev/null | tr -d '\r' | grep -c hi.txt)"
checkcode setsid-help 0 "$(run setsid --help >/dev/null 2>&1; echo $?)"
checkcode su-help 0 "$(run su --help >/dev/null 2>&1; echo $?)"
checkcode passwd-help 0 "$(run passwd --help >/dev/null 2>&1; echo $?)"
check mkpasswd-sha512 "ja" "$(run mkpasswd -m sha-512 -s salz geheim 2>/dev/null | grep -q '^\$6\$salz\$' && echo ja)"
check mkpasswd-sha256 "ja" "$(run mkpasswd -m sha-256 -s salz geheim 2>/dev/null | grep -q '^\$5\$salz\$' && echo ja)"
rm -rf "$ZIP_DIR"

# unknown command: the dispatcher names akmesh, not u-root's busybox
checkcode unknown-command-exit 1 "$(run gibtsnicht >/dev/null 2>&1; echo $?)"
check unknown-command-message "ja" "$(run gibtsnicht 2>&1 | grep -q 'not present in akmesh' && echo ja)"
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
	checkcode policy-refused 1 "$(AKMESH_POLICY="$POL" "$BIN" policy >/dev/null 2>&1; echo $?)"
	;;
*)
	check policy-active "0" "$(AKMESH_POLICY="$POL" "$BIN" policy >/dev/null 2>&1; echo $?)"
	check policy-shows-source "ja" "$(AKMESH_POLICY="$POL" "$BIN" policy 2>/dev/null | grep -q "$POL" && echo ja)"
	check policy-read-inside "drin" "$(AKMESH_POLICY="$POL" "$BIN" sh -c "cat $WORK/work/in.txt" 2>/dev/null | tr -d '\r')"
	check policy-blocks-read "ja" "$(AKMESH_POLICY="$POL" "$BIN" sh -c 'cat /etc/hostname' >/dev/null 2>&1 || echo ja)"
	check policy-blocks-write "ja" "$(AKMESH_POLICY="$POL" "$BIN" sh -c "echo x > $WORK/aussen.txt" >/dev/null 2>&1 || echo ja)"
	check policy-no-outside-file "ja" "$([ ! -f "$WORK/aussen.txt" ] && echo ja)"
	check policy-write-inside "ok" "$(AKMESH_POLICY="$POL" "$BIN" sh -c "echo ok > $WORK/work/neu.txt; cat $WORK/work/neu.txt" 2>/dev/null | tr -d '\r')"
	check policy-check-allowed "0" "$(AKMESH_POLICY="$POL" "$BIN" policy --check "$WORK/work/in.txt" >/dev/null 2>&1; echo $?)"
	check policy-check-denied "1" "$(AKMESH_POLICY="$POL" "$BIN" policy --check /etc/hostname >/dev/null 2>&1; echo $?)"
	# "Write implies read": a policy that names only the write path must still
	# allow reading it, because the write rule carries the read rights. Two runs,
	# one inside and one outside, so a policy that blocks everything cannot pass
	# this as a success.
	printf '{"version":1,"write":["%s"]}' "$WORK/work" > "$WORK/write-only.json"
	check policy-write-only-inside-allowed "0" "$(AKMESH_POLICY="$WORK/write-only.json" "$BIN" policy --check "$WORK/work/in.txt" >/dev/null 2>&1; echo $?)"
	check policy-write-only-outside-denied "1" "$(AKMESH_POLICY="$WORK/write-only.json" "$BIN" policy --check /etc/hostname >/dev/null 2>&1; echo $?)"
	if [ "$HAS_NET" = 1 ]; then
		check policy-blocks-network "ja" "$(AKMESH_POLICY="$POL" "$BIN" curl -s https://example.com >/dev/null 2>&1 || echo ja)"
		check policy-network-message "ja" "$(AKMESH_POLICY="$POL" "$BIN" curl -s https://example.com 2>&1 | grep -q 'does not allow network access' && echo ja)"
	fi
	# A broken policy must stop the process instead of running unconfined.
	printf '{"version":9}' > "$WORK/broken.json"
	checkcode policy-broken-refused 1 "$(AKMESH_POLICY="$WORK/broken.json" "$BIN" echo x >/dev/null 2>&1; echo $?)"
	;;
esac

# --help before the parser (acmesh-py8): ten commands, of which five used to
# execute the word (hostname set it as the name, sync opened it as a file) and
# two read only its first letter. A command that answers with anything other
# than a usage line, or with a non-zero exit, is a regression.
for c in base64 wc touch hostname sync grep sed find cp tar; do
	check "help-$c-usage" "ja" "$(run "$c" --help 2>&1 | grep -qi '^usage:' && echo ja)"
	checkcode "help-$c-exit" 0 "$(run "$c" --help >/dev/null 2>&1; echo $?)"
done
check help-echo-usage "ja" "$(run echo --help 2>&1 | grep -qi '^usage:' && echo ja)"
check echo-help-literal "--help foo" "$(run echo --help foo | tr -d '\r')"

# The three certificate commands together, end to end (acmesh-pav, acmesh-vcl,
# acmesh-0kh): mkcert signs a leaf with its own CA, httpd serves it with TLS and
# certinfo reads the chain back. The Go tests cover each command's own behaviour
# with httptest; what only the real binary can show is that the CA file mkcert
# wrote is the one httpd hands out and certinfo reads.
CERT_DIR=$(mktemp -d)
tls_port=$((19200 + ($$ % 300)))
tls_try=0
while [ $tls_try -lt 20 ] && curl -sk -m 1 -o /dev/null "https://127.0.0.1:$tls_port/" 2>/dev/null; do
	tls_port=$((tls_port + 7))
	tls_try=$((tls_try + 1))
done
run mkcert -host localhost,127.0.0.1 -ca "$CERT_DIR/tca" -out "$CERT_DIR/tleaf" >/dev/null 2>&1
checkcode mkcert-exit 0 "$?"
check verify-ca-ok "OK" "$(run verify -ca "$CERT_DIR/tca-cert.pem" -host localhost "$CERT_DIR/tleaf-cert.pem" 2>&1 | cut -d: -f1)"
checkcode verify-wrong-host 1 "$(run verify -ca "$CERT_DIR/tca-cert.pem" -host falsch.test "$CERT_DIR/tleaf-cert.pem" >/dev/null 2>&1; echo $?)"
# Started directly, not through run(): with a shell function $! is the subshell,
# and killing it left the server running and holding the port (measured: seven
# orphans after four runs).
"$BIN" httpd -f -p 127.0.0.1:$tls_port -h "$CERT_DIR" -S "$CERT_DIR/tleaf-cert.pem" -K "$CERT_DIR/tleaf-key.pem" >/dev/null 2>&1 &
TLS_PID=$!
sleep 1
check certinfo-httpd-tls "localhost" "$(run certinfo 127.0.0.1:$tls_port 2>&1 | sed -n 's/^Subject: CN=\([^,]*\).*/\1/p' | head -1)"
kill $TLS_PID 2>/dev/null
wait $TLS_PID 2>/dev/null
pkill -f "httpd -f -p 127.0.0.1:$tls_port" 2>/dev/null
stop_code=200
stops=0
while [ $stops -lt 20 ]; do
  stop_code=$(curl -sk -m 2 -o /dev/null -w '%{http_code}' "https://127.0.0.1:$tls_port/" 2>/dev/null)
  [ "$stop_code" = "000" ] && break
  sleep 0.5
  stops=$((stops+1))
done
check tls-httpd-stopped "000" "$stop_code"

# A local CA: httpd signs a leaf per SNI name, so a name no public CA would ever
# certify gets a certificate the client can verify against that one root
# (acmesh-816). No -install here: the battery must not touch the system's trust
# store, so curl gets the root with --cacert.
CA_DIR=$(mktemp -d "${TMPDIR:-/tmp}/catest.XXXXXX")
run mkcert -host test1.local -ca "$CA_DIR/local" -ca-domains .local -out "$CA_DIR/leaf" >/dev/null 2>&1
CA_PORT=$((tls_port + 1))
crt=0
while [ $crt -lt 20 ] && curl -sk -m 1 -o /dev/null "https://127.0.0.1:$CA_PORT/" 2>/dev/null; do
  CA_PORT=$((CA_PORT + 7))
  crt=$((crt + 1))
done
printf 'sni-ok\n' > "$CA_DIR/index.html"
"$BIN" httpd -f -p 127.0.0.1:$CA_PORT -h "$CA_DIR" --tls-ca "$CA_DIR/local-cert.pem" -K "$CA_DIR/local-key.pem" >"$CA_DIR/srv.log" 2>&1 &
CA_PID=$!
tries=0
while [ $tries -lt 40 ]; do
  run curl -s -m 1 --cacert "$CA_DIR/local-cert.pem" --resolve "test1.local:$CA_PORT:127.0.0.1" "https://test1.local:$CA_PORT/index.html" 2>/dev/null | grep -q sni-ok && break
  sleep 0.5
  tries=$((tries+1))
done
check ca-sni-test1 "sni-ok" "$(run curl -s -m 5 --cacert "$CA_DIR/local-cert.pem" --resolve "test1.local:$CA_PORT:127.0.0.1" "https://test1.local:$CA_PORT/index.html" | tr -d '\r')"
check ca-sni-test2 "sni-ok" "$(run curl -s -m 5 --cacert "$CA_DIR/local-cert.pem" --resolve "test2.local:$CA_PORT:127.0.0.1" "https://test2.local:$CA_PORT/index.html" | tr -d '\r')"
# Both names are served by one process, so each leaf has to be for its own name.
# certinfo reaches the name with --resolve, without an /etc/hosts entry.
check ca-sni-two-leaves "test2.local" "$(run certinfo --resolve "test2.local:$CA_PORT:127.0.0.1" "test2.local:$CA_PORT" 2>/dev/null | grep -m1 '^DNS:' | grep -o 'test2\.local' | head -1)"
# A name outside the CA's constraints is refused in the handshake, not signed.
checkcode ca-sni-foreign 35 "$(run curl -s -m 5 --cacert "$CA_DIR/local-cert.pem" --resolve "example.com:$CA_PORT:127.0.0.1" "https://example.com:$CA_PORT/" >/dev/null 2>&1; echo $?)"
# The server logs the refusal on its own goroutine, so give it a moment: on
# Windows the line was not there yet when it was looked for once.
notice=0
nt=0
while [ $nt -lt 20 ]; do
  notice=$(grep -c 'not covered by the CA' "$CA_DIR/srv.log" 2>/dev/null)
  [ "$notice" = "0" ] || break
  sleep 0.5
  nt=$((nt+1))
done
check ca-sni-notice "1" "$notice"
kill $CA_PID 2>/dev/null
pkill -f "httpd -f -p 127.0.0.1:$CA_PORT" 2>/dev/null
rm -rf "$CA_DIR"

# A flood against a server of our own: the count is what matters, and the
# summary has to name it (acmesh-730).
FL_DIR=$(mktemp -d "${TMPDIR:-/tmp}/flood.XXXXXX")
printf 'flood\n' > "$FL_DIR/index.html"
FL_PORT=$((CA_PORT + 1))
frt=0
while [ $frt -lt 20 ] && curl -s -m 1 -o /dev/null "http://127.0.0.1:$FL_PORT/" 2>/dev/null; do
  FL_PORT=$((FL_PORT + 7))
  frt=$((frt + 1))
done
"$BIN" httpd -f -p 127.0.0.1:$FL_PORT -h "$FL_DIR" >/dev/null 2>&1 &
FL_PID=$!
tries=0
while [ $tries -lt 40 ]; do
  curl -s -m 1 "http://127.0.0.1:$FL_PORT/index.html" 2>/dev/null | grep -q flood && break
  sleep 0.5
  tries=$((tries+1))
done
FLOOD=$(run curl -Z --repeat 300 --parallel-max 50 "http://127.0.0.1:$FL_PORT/index.html" 2>&1 | tr -d '\r')
check curl-parallel-count "1" "$(printf '%s' "$FLOOD" | grep -c '300 requests, 0 failed')"
check curl-parallel-status "1" "$(printf '%s' "$FLOOD" | grep -c '200: 300')"
checkcode curl-parallel-unknownflags 2 "$(run curl -Z --repeat 0 "http://127.0.0.1:$FL_PORT/" >/dev/null 2>&1; echo $?)"
kill $FL_PID 2>/dev/null
pkill -f "httpd -f -p 127.0.0.1:$FL_PORT" 2>/dev/null
rm -rf "$FL_DIR"
rm -rf "$CERT_DIR"


# --- Benutzerwerkzeuge und rmdir (acmesh-uxf9). Die Konten laufen gegen eine
# Wegwerf-/etc unter AKMESH_ROOT: die Batterie darf das echte /etc nie anfassen.
if [ -z "$WINDOWS" ]; then
  UE="$WORK/urootetc"; mkdir -p "$UE/etc"
  printf 'root:x:0:0:root:/root:/bin/sh\n' > "$UE/etc/passwd"
  printf 'root:x:0:\nwheel:x:100:\n' > "$UE/etc/group"
  printf 'root:!:20781:0:99999:7:::\n' > "$UE/etc/shadow"
  AKMESH_ROOT="$UE" run addgroup -S apkgrp >/dev/null 2>&1
  check addgroup-system 'apkgrp:x:101:' "$(grep '^apkgrp:' "$UE/etc/group" | tr -d '\r')"
  check addgroup-dup-in-use '1' "$(AKMESH_ROOT="$UE" run addgroup -S apkgrp 2>&1 | tr -d '\r' | grep -c "group 'apkgrp' in use")"
  AKMESH_ROOT="$UE" run adduser -S -D -H -h /dev/null -s /sbin/nologin -G apkgrp svc >/dev/null 2>&1
  check adduser-script-line 'svc:x:100:101::/dev/null:/sbin/nologin' "$(grep '^svc:' "$UE/etc/passwd" | tr -d '\r')"
  check adduser-shadow-locked '1' "$(grep -c '^svc:!:' "$UE/etc/shadow" | tr -d '\r')"
  check delgroup-primary '1' "$(AKMESH_ROOT="$UE" run delgroup apkgrp 2>&1 | tr -d '\r' | grep -c "still has 'apkgrp' as their primary group")"
  checkcode delgroup-member 0 "$(AKMESH_ROOT="$UE" run delgroup svc apkgrp >/dev/null 2>&1; echo $?)"
  checkcode deluser 0 "$(AKMESH_ROOT="$UE" run deluser svc >/dev/null 2>&1; echo $?)"
  check deluser-shadow-gone '0' "$(grep -c '^svc:' "$UE/etc/shadow" | tr -d '\r')"

  mkdir -p rd/a/b
  run rmdir rd/a/b >/dev/null 2>&1
  check rmdir-empty '0' "$([ -d rd/a/b ] && echo 1 || echo 0)"
  mkdir -p pp/a/b; touch pp/keep
  check rmdir-parents '1' "$(run rmdir -p pp/a/b 2>&1 | tr -d '\r' | grep -c "failed to remove directory 'pp': Directory not empty")"
  check rmdir-parents-gone '0' "$([ -d pp/a ] && echo 1 || echo 0)"
  mkdir -p rne; touch rne/x
  check rmdir-nonempty-message '1' "$(run rmdir rne 2>&1 | tr -d '\r' | grep -c "failed to remove 'rne': Directory not empty")"
  checkcode rmdir-nonempty-status 1 "$(run rmdir rne >/dev/null 2>&1; echo $?)"
  checkcode rmdir-ignore-nonempty 0 "$(run rmdir --ignore-fail-on-non-empty rne >/dev/null 2>&1; echo $?)"
  check rmdir-missing-message '1' "$(run rmdir nix 2>&1 | tr -d '\r' | grep -c "failed to remove 'nix': No such file or directory")"
  check rmdir-missing-operand '1' "$(run rmdir 2>&1 | tr -d '\r' | grep -c 'rmdir: missing operand')"
fi


# --- apk (acmesh-xwcm): ein echtes Repo aus abuild liegt in
# internal/apkcmd/testdata/real. Die Pruefung installiert daraus in eine
# Wegwerf-Wurzel, mit und ohne Schluessel.
if [ -z "$WINDOWS" ]; then
  APKDIR="$WORK/apk"; mkdir -p "$APKDIR/repo/x86_64" "$APKDIR/root/etc/apk/keys"
  cp "$REPO_DIR"/internal/apkcmd/testdata/real/*.apk "$APKDIR/repo/x86_64/"
  cp "$REPO_DIR"/internal/apkcmd/testdata/real/APKINDEX.tar.gz "$APKDIR/repo/x86_64/"
  cp "$REPO_DIR"/internal/apkcmd/testdata/real/*.pub "$APKDIR/root/etc/apk/keys/"
  check apk-add-line '1' "$(run apk -p "$APKDIR/root" -X "$APKDIR/repo" add akmesh-hello 2>/dev/null | tr -d '\r' | grep -c "(1/1) Installing akmesh-hello (0.0.1-r0)")"
  check apk-file '1' "$(grep -c 'hallo aus dem eigenen paket' "$APKDIR/root/usr/bin/akmesh-hello" 2>/dev/null | tr -d '\r')"
  check apk-world 'akmesh-hello' "$(tr -d '\r' < "$APKDIR/root/etc/apk/world" 2>/dev/null)"
  check apk-installed-db '1' "$(grep -c '^P:akmesh-hello$' "$APKDIR/root/lib/apk/db/installed" 2>/dev/null | tr -d '\r')"
  check apk-info '1' "$(run apk -p "$APKDIR/root" info akmesh-hello 2>/dev/null | tr -d '\r' | grep -c 'hello from our own apk repository')"
  rm -f "$APKDIR/root/etc/apk/keys/"*.pub
  check apk-unsigned-refused '1' "$(run apk -p "$APKDIR/root" -X "$APKDIR/repo" add foo 2>&1 | tr -d '\r' | grep -c 'UNTRUSTED signature')"
  cp "$REPO_DIR"/internal/apkcmd/testdata/real/*.pub "$APKDIR/root/etc/apk/keys/"
  check apk-del '1' "$(run apk -p "$APKDIR/root" del akmesh-hello 2>/dev/null | tr -d '\r' | grep -c 'Purging akmesh-hello')"
  check apk-del-gone '0' "$([ -f "$APKDIR/root/usr/bin/akmesh-hello" ] && echo 1 || echo 0)"
fi


# --- apkbuild (acmesh-oolz): Paket und Index selbst bauen, dann mit unserem
# apk installieren. Kein podman, damit die Batterie ohne Container laeuft.
if [ -z "$WINDOWS" ]; then
  BD="$WORK/ab"; mkdir -p "$BD/src/usr/bin" "$BD/repo/x86_64" "$BD/root/etc/apk/keys"
  printf '#!/bin/sh\necho aus apkbuild\n' > "$BD/src/usr/bin/abhi"; chmod 755 "$BD/src/usr/bin/abhi"
  printf 'pkgname = abhi\npkgver = 1.2.3\npkgrel = 0\npkgdesc = gebaut in der Batterie\narch = x86_64\nlicense = MIT\nsource_date_epoch = 1764000000\n' > "$BD/manifest"
  run apkbuild keygen -o "$BD/bkey.rsa" >/dev/null 2>&1
  check apkbuild-keygen '1' "$([ -f "$BD/bkey.rsa" ] && [ -f "$BD/bkey.rsa.pub" ] && echo 1 || echo 0)"
  check apkbuild-build '1' "$(run apkbuild build --manifest "$BD/manifest" --source "$BD/src" -o "$BD/repo/x86_64" --key "$BD/bkey.rsa" 2>&1 | tr -d '\r' | grep -c 'abhi-1.2.3-r0.apk')"
  check apkbuild-index '1' "$(run apkbuild index -o "$BD/repo/x86_64/APKINDEX.tar.gz" "$BD"/repo/x86_64/*.apk --key "$BD/bkey.rsa" 2>&1 | tr -d '\r' | grep -c '1 packages')"
  cp "$BD/bkey.rsa.pub" "$BD/root/etc/apk/keys/"
  check apkbuild-install '1' "$(run apk -p "$BD/root" -X "$BD/repo" add abhi 2>&1 | tr -d '\r' | grep -c '(1/1) Installing abhi (1.2.3-r0)')"
  check apkbuild-run 'aus apkbuild' "$("$BD/root/usr/bin/abhi" 2>&1 | tr -d '\r')"
  mkdir -p "$BD/root2/etc/apk/keys" && cp "$BD/bkey.rsa.pub" "$BD/root2/etc/apk/keys/"
  printf 'x' | dd of="$BD/repo/x86_64/abhi-1.2.3-r0.apk" bs=1 seek=200 conv=notrunc 2>/dev/null
  run apk -p "$BD/root2" -X "$BD/repo" add abhi >/dev/null 2>&1
  check apkbuild-tamper-exit '1' "$([ $? -ne 0 ] && echo 1 || echo 0)"
fi


# --- apkreport (acmesh-slyv): Bericht je Rechner, eine JSON-Zeile pro Paket.
if [ -z "$WINDOWS" ]; then
  RP="$WORK/rep"; mkdir -p "$RP/root/lib/apk/db" "$RP/root/etc/apk"
  printf 'P:rotest\nV:2.0.0-r0\nA:x86_64\nS:966\nI:28\n\nP:anderes\nV:1.0.0-r0\nA:x86_64\nS:100\nI:10\n\n' > "$RP/root/lib/apk/db/installed"
  printf 'rotest@stable\n' > "$RP/root/etc/apk/world"
  check apkreport-lines '2' "$(run apkreport -p "$RP/root" -q 2>/dev/null | wc -l | tr -d ' \r')"
  check apkreport-json '2' "$(run apkreport -p "$RP/root" -q 2>/dev/null | grep -c '^{.*"paket":"[a-z]*".*}$')"
  check apkreport-kanal '1' "$(run apkreport -p "$RP/root" -q 2>/dev/null | grep '"paket":"rotest"' | grep -c '"kanal":"stable"')"
  check apkreport-writes-file '1' "$(run apkreport -p "$RP/root" -o "$RP/b.jsonl" -q 2>/dev/null; [ -s "$RP/b.jsonl" ] && echo 1 || echo 0)"
  check apkreport-vorher '2.0.0-r0' "$(printf 'P:rotest\nV:3.0.0-r0\nA:x86_64\nS:966\nI:28\n\n' > "$RP/root/lib/apk/db/installed"; run apkreport -p "$RP/root" -o "$RP/b.jsonl" --health ok -q 2>/dev/null; tail -1 "$RP/b.jsonl" | sed -n 's/.*"vorher":"\([^"]*\)".*/\1/p')"
fi

printf '%d checks, %d failures\n' "$checks" "$fails"
[ "$fails" -eq 0 ]
