#!/bin/sh
# Vergleicht find-Aufrufe zwischen dem Referenz-Image und akmesh.
# Beide Seiten laufen durch "sh -c", damit Anfuehrungszeichen und Klammern so
# ankommen, wie ein Agent sie schreibt. Verglichen wird sortiert, weil die
# Reihenfolge bei GNU von readdir abhaengt.
set -u
IMG=${IMG:-docker.io/library/debian@sha256:cd9f2189b62dc5ce003b22ad8cb109bbc3cef8ddcc8bc13771472a6edeb16980}
DIR=${1:-/tmp/ffx}
BIN=${2:-/root/akmesh/akmesh}
WRAP=$(mktemp -d)
printf '#!/bin/sh\nexec %s find "$@"\n' "$BIN" > "$WRAP/find"
chmod +x "$WRAP/find"
same=0
diff=0
while IFS= read -r c; do
	[ -z "$c" ] && continue
	g=$(podman run --rm -v "$DIR:/w" -w /w "$IMG" sh -c "$c" 2>&1 | sort)
	a=$(cd "$DIR" && PATH="$WRAP:$PATH" sh -c "$c" 2>&1 | sort)
	if [ "$g" = "$a" ]; then
		echo "gleich   $c"
		same=$((same + 1))
	else
		echo "ABWEICH  $c"
		diff <(echo "$g") <(echo "$a") | head -6
		diff=$((diff + 1))
	fi
done <<'EOF'
find . -name "*.log" -o -name "*.txt"
find . \( -name "*.txt" -o -name "*.log" \) -size -5c
find . -size +1c
find . -size 0
find . -empty
find . -iname "A*"
find . -type f -newer b.log
find . -perm -u+x
find . -perm 644
find . -user root
find . -group root
find . -mtime -1
find . -mmin -1
find . -maxdepth 0
find . -maxdepth 1 -type d
find . -mindepth 1 -maxdepth 1
find . -type f -name "*.txt" -print0
find . -not -name "*.txt"
find . ! -type d
find . -type f -exec echo HIT {} \;
find . -type f -name "a.txt" -exec echo {} \;
find . -name "*.txt" -a -size -3c
find . -path "./sub/*"
find . -ipath "./SUB/*"
find . -regex ".*\.log"
find . -maxdepth 1 -name "*.log" -o -name "*.txt"
find . -name "*.txt" -o -name "*.log" -print
find . -type f -printf "%p %s\n"
find sub -name "*.txt"
find sub sub/deep -type d
find . -type d -empty
EOF
rm -rf "$WRAP"
echo "--- $same gleich, $diff abweichend ---"
