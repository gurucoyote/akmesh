#!/bin/sh
# Vergleicht ripgrep-Aufrufe zwischen echtem rg (falls vorhanden) und akmesh.
# Ohne rg im Image werden nur die akmesh-Ausgaben gezeigt; PCRE-Muster haben
# dann keinen GNU-Gegenwert, weil ripgrep -P nur mit dem pcre2-Feature kennt.
set -u
DIR=${1:-/tmp/ffx}
BIN=${2:-/root/akmesh/akmesh}
cd "$DIR" || exit 1
run() {
	echo "--- rg $*"
	"$BIN" rg "$@" 2>&1
	echo "rc=$?"
}
run -P 'a(?=a)' .
run -P 'a\Ka' .
run -P 'a++' .
run --engine=pcre2 '(?<=b)b' .
run --engine=auto 'a(?=a)' .
run -Pw 'aa' .
run --engine=bogus 'x' .
run 'a(?=a)' .
