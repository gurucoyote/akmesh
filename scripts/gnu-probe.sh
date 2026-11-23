#!/bin/sh
# gnu-probe.sh - vergleicht ein Werkzeug Aufruf fuer Aufruf mit GNU.
#
# Der Abgleich der bisherigen Runde lief je Werkzeug in einem eigenen Skript
# (find-probe.sh, base64-probe.sh, ...). Fuer die naechste Messreihe
# (Textwerkzeuge) gibt es diesen gemeinsamen Rahmen: er bekommt den Werkzeug-
# namen und liest die Aufrufe zeilenweise aus einer Datei (oder von stdin).
#
# Aufruf:
#   ./scripts/gnu-probe.sh awk scripts/probes/awk.probe
#   ./scripts/gnu-probe.sh sort < meine-aufrufe.txt
#
# Eine Zeile ist ein ganzer sh-Aufruf ohne Wrapper, z.B.
#   sort -k2 -t: /tmp/fix/colon.txt
#   awk -F: '{print $2}' /tmp/fix/colon.txt
# '#' am Zeilenanfang ist Kommentar, Leerzeilen werden uebersprungen.
#
# Beide Seiten laufen durch "sh -c", die eigene Seite bekommt dafuer ein
# "NAME" im PATH, das auf akmesh zeigt. stdout und stderr werden getrennt
# verglichen: im gemeinsamen Kanal haengt die Reihenfolge von der Pufferung ab
# und sagt nichts ueber das Verhalten. Der Exitcode gehoert zum Vergleich.
#
# Exitcode: 0 alle gleich, 1 mindestens eine Abweichung, 2 Aufruffehler.
set -u
NAME=${1:-}
PROBE=${2:-/dev/stdin}
BIN=${BIN:-/root/akmesh/akmesh}
IMG=${IMG:-docker.io/library/debian@sha256:cd9f2189b62dc5ce003b22ad8cb109bbc3cef8ddcc8bc13771472a6edeb16980}
FIX=${FIX:-/tmp/gnufix}

if [ -z "$NAME" ]; then
	echo "usage: gnu-probe.sh TOOL [AUFRUFE]" >&2
	exit 2
fi
if [ ! -x "$BIN" ]; then
	echo "gnu-probe: $BIN not found (build first)" >&2
	exit 2
fi
# Pfad absolut machen, weil unten ins Fixture gewechselt wird.
case "$PROBE" in
/dev/*) ;;
*) PROBE=$(cd "$(dirname "$PROBE")" 2>/dev/null && pwd)/$(basename "$PROBE") ;;
esac
if [ ! -f "$PROBE" ]; then
	echo "gnu-probe: $PROBE not found" >&2
	exit 2
fi
if [ ! -d "$FIX" ]; then
	echo "gnu-probe: fixture $FIX missing (create it first)" >&2
	exit 2
fi

WRAP=$(mktemp -d)
printf '#!/bin/sh\nexec %s %s "$@"\n' "$BIN" "$NAME" > "$WRAP/$NAME"
chmod +x "$WRAP/$NAME"
trap 'rm -rf "$WRAP"' EXIT
cd "$FIX" || exit 2

same=0
diff=0
while IFS= read -r line; do
	case "$line" in
	'' | '#'*) continue ;;
	esac
	gout=$(podman run --rm -v "$FIX":/w -w /w "$IMG" sh -c "$line" 2>"$FIX/.g.err")
	grc=$?
	gerr=$(cat "$FIX/.g.err")
	aout=$(PATH="$WRAP:$PATH" sh -c "$line" 2>"$FIX/.a.err")
	arc=$?
	aerr=$(cat "$FIX/.a.err")
	if [ "$gout" = "$aout" ] && [ "$gerr" = "$aerr" ] && [ "$grc" = "$arc" ]; then
		same=$((same + 1))
		echo "gleich   $line"
	else
		diff=$((diff + 1))
		echo "ABWEICH  $line"
		[ "$gout" != "$aout" ] && { echo "  GNU  out: $(printf '%s' "$gout" | head -3)"; echo "  unser out: $(printf '%s' "$aout" | head -3)"; }
		[ "$gerr" != "$aerr" ] && { echo "  GNU  err: $(printf '%s' "$gerr" | head -2)"; echo "  unser err: $(printf '%s' "$aerr" | head -2)"; }
		[ "$grc" != "$arc" ] && echo "  rc: GNU $grc, unser $arc"
	fi
done < "$PROBE"
rm -f "$FIX/.g.err" "$FIX/.a.err"

echo "----"
echo "$same gleich, $diff abweichend ($NAME)"
if [ "$same" -eq 0 ] && [ "$diff" -eq 0 ]; then
	echo "gnu-probe: keine Aufrufe gelesen" >&2
	exit 2
fi
[ "$diff" -eq 0 ] && exit 0
exit 1
