#!/bin/sh
# grep und Binaerdateien gegen GNU grep 3.11: Meldung, -c, -l, -L, -I, -a.
set -u
BIN=${1:-/root/akmesh/akmesh}
IMG=${IMG:-docker.io/library/debian@sha256:cd9f2189b62dc5ce003b22ad8cb109bbc3cef8ddcc8bc13771472a6edeb16980}
WRAP=$(mktemp -d)
printf '#!/bin/sh\nexec %s grep "$@"\n' "$BIN" > "$WRAP/grep"
chmod +x "$WRAP/grep"
mkdir -p /tmp/gb
cd /tmp/gb || exit 1
printf 'X\000ABC\n' > bin1
printf 'ABC\000Z\n' > bin2
printf 'ABC\n' > txt
t() {
	name=$1
	cmd=$2
	# stdout und stderr getrennt, weil die Reihenfolge im gemeinsamen Kanal von
	# der Pufferung abhaengt und nichts ueber das Verhalten sagt.
	g=$(podman run --rm -v /tmp/gb:/w -w /w "$IMG" sh -c "$cmd" 2>/tmp/gb/g.err; rc=$?; echo "rc=$rc"; echo "err:"; cat /tmp/gb/g.err)
	a=$(PATH="$WRAP:$PATH" sh -c "$cmd" 2>/tmp/gb/a.err; rc=$?; echo "rc=$rc"; echo "err:"; cat /tmp/gb/a.err)
	if [ "$g" = "$a" ]; then
		echo "gleich   $name"
	else
		echo "ABWEICH  $name"
		echo "  GNU  : $g"
		echo "  unser: $a"
	fi
}
t "Treffer in Binaerdatei" "grep ABC bin1"
t "kein Treffer" "grep ZZZ bin1"
t "-c" "grep -c ABC bin1"
t "-l" "grep -l ABC bin1 txt"
t "-L" "grep -L ABC bin1"
t "-I" "grep -I ABC bin1"
t "-a" "grep -a ABC bin1"
t "--binary-files=text" "grep --binary-files=text ABC bin1"
t "--binary-files=without-match" "grep --binary-files=without-match ABC bin1"
t "zwei Dateien, eine binaer" "grep ABC bin1 txt"
t "-n in Binaerdatei" "grep -n ABC bin2"
t "-q in Binaerdatei" "grep -q ABC bin1"
t "Textdatei unveraendert" "grep ABC txt"
rm -rf "$WRAP"
