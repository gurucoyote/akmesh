#!/bin/sh
# base64: Kodieren mit -w0 und Dekodieren ohne Auffuellung, gegen GNU geprueft.
# Beide Seiten laufen durch "sh -c", damit der Aufruf derselbe ist; die eigene
# Seite bekommt dafuer ein "base64" im PATH, das auf akmesh zeigt.
set -u
BIN=${1:-/root/akmesh/akmesh}
IMG=${IMG:-docker.io/library/debian@sha256:cd9f2189b62dc5ce003b22ad8cb109bbc3cef8ddcc8bc13771472a6edeb16980}
WRAP=$(mktemp -d)
printf '#!/bin/sh\nexec %s base64 "$@"\n' "$BIN" > "$WRAP/base64"
chmod +x "$WRAP/base64"
cd /tmp || exit 1
printf 'hello world\n' > b.txt
t() {
	name=$1
	cmd=$2
	g=$(podman run --rm -v /tmp:/w -w /w "$IMG" sh -c "$cmd" 2>&1 | od -c | tr '\n' ' ')
	a=$(PATH="$WRAP:$PATH" sh -c "$cmd" 2>&1 | od -c | tr '\n' ' ')
	if [ "$g" = "$a" ]; then
		echo "gleich   $name"
	else
		echo "ABWEICH  $name"
		echo "  GNU  : $g"
		echo "  unser: $a"
	fi
}
t "base64 b.txt" 'base64 b.txt'
t "-w0" 'base64 -w0 b.txt'
t "-w 0" 'base64 -w 0 b.txt'
t "--wrap=0" 'base64 --wrap=0 b.txt'
t "-w4" 'base64 -w4 b.txt'
t "-d ohne Auffuellung" 'printf aGVsbG8 > p.txt && base64 -d p.txt'
t "-d mit Auffuellung" 'printf aGVsbG8= > q.txt && base64 -d q.txt'
t "-d Leerzeichen" 'printf "aGVs bG8" > r.txt && base64 -d r.txt'
t "-di Leerzeichen" 'printf "aGVs bG8" > s.txt && base64 -di s.txt'
t "-d Unsinn" 'printf "!!!!" > t.txt && base64 -d t.txt'
t "-d Umbruch im Text" 'printf "aGVs\nbG8" > u.txt && base64 -d u.txt'
rm -rf "$WRAP"
