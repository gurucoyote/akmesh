#!/bin/sh
# sed -z gegen GNU sed: NUL-getrennte Saetze lesen und schreiben.
# Beide Seiten laufen durch "sh -c"; die eigene Seite bekommt ein "sed" im PATH,
# das auf akmesh zeigt.
set -u
BIN=${1:-/root/akmesh/akmesh}
IMG=${IMG:-docker.io/library/debian@sha256:cd9f2189b62dc5ce003b22ad8cb109bbc3cef8ddcc8bc13771472a6edeb16980}
WRAP=$(mktemp -d)
printf '#!/bin/sh\nexec %s sed "$@"\n' "$BIN" > "$WRAP/sed"
chmod +x "$WRAP/sed"
cd /tmp/zfix || exit 1
t() {
	name=$1
	cmd=$2
	g=$(podman run --rm -v /tmp/zfix:/w -w /w "$IMG" sh -c "$cmd" 2>&1 | od -c | tr '\n' ' ')
	a=$(PATH="$WRAP:$PATH" sh -c "$cmd" 2>&1 | od -c | tr '\n' ' ')
	if [ "$g" = "$a" ]; then
		echo "gleich   $name"
	else
		echo "ABWEICH  $name"
		echo "  GNU  : $g"
		echo "  unser: $a"
	fi
}
t "s/^/X/ je Satz" "sed -z 's/^/X/' list.nul"
t "s///g je Satz" "sed -z 's/txt/TXT/g' list.nul"
t "zwei Befehle" "sed -z 's/a/A/;s/d/D/' list.nul"
t "-zn p" "sed -zn p list.nul"
t "Leerzeichen ersetzen" "sed -z 's/ /_/' list.nul"
t "2d" "sed -z 2d list.nul"
t "\$ = letzter Satz" "sed -z '\$s/txt/END/' list.nul"
t "ohne -z, NUL als Text" "sed 's/txt/TXT/' list.nul"
t "Zeilenumbruch im Satz" "printf 'a\\nb\\0c\\0' > nl.nul && sed -z 's/^/X/' nl.nul"
t "Byte 0x01 im Satz" "printf 'a\\001b\\0c\\0' > soh.nul && sed -z 's/^/X/' soh.nul"
rm -rf "$WRAP"
