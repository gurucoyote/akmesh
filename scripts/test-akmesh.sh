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
*.exe) cp "$BIN" "$WORK/acmesh-under-test.exe" && BIN="$WORK/acmesh-under-test.exe" ;;
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
# That curl verified test2.local against the root is the proof that the second
# name got its own leaf: hostname verification fails on a foreign certificate.
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
rm -rf "$CERT_DIR"

printf '%d checks, %d failures\n' "$checks" "$fails"
[ "$fails" -eq 0 ]
