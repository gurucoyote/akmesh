#!/bin/sh
# gnufix.sh - baut das Fixture fuer die Messreihe Textwerkzeuge.
#
# Alle Probe-Dateien unter scripts/probes/ laufen in /tmp/gnufix. Das Fixture
# wird hier angelegt, damit ein Messlauf reproduzierbar ist und nicht von dem
# abhaengt, was zufaellig in /tmp liegt.
#
#   ./scripts/gnufix.sh            # nach /tmp/gnufix
#   FIX=/tmp/meinfix ./scripts/gnufix.sh
#
# Inhalt und wofuer er da ist:
#   words.txt     vier Woerter, zwei gleich      -> sort -u, uniq, comm
#   colon.txt     Felder mit Doppelpunkt         -> cut -d:, sort -k2 -t:, awk -F:
#   tsv.txt       Felder mit Tabulator           -> cut -f, sort -t"$(printf \\t)"
#   mixed.txt     Gross-/Kleinschreibung gemischt-> sort ohne/mit -f, -d
#   num.txt       Zahlen 1..12 unsortiert        -> sort -n, -nr, -k
#   umlaut.txt    Umlaute und ss                 -> Byte- gegen Locale-Ordnung
#   blank.txt     Leerzeilen und Leerzeichen     -> uniq -c, cut -f mit fehlenden Feldern
#   long.txt      eine Zeile ueber 8 KiB         -> head -c, cut -b, awk ohne RS
#   empty.txt     leer                           -> wc, head, sort auf leer
#   bin.bin       0x00 und Text                   -> grep -a, od, tr
#   NoNewline.txt Text ohne letztes Zeilenende    -> head/tail letzte Zeile
set -eu
FIX=${FIX:-/tmp/gnufix}
rm -rf "$FIX"
mkdir -p "$FIX"
cd "$FIX"

printf 'b\nA\na\nc\n' > words.txt
printf 'b:2:z\nA:1:y\na:3:x\nc:10:w\n' > colon.txt
printf 'b\t2\tz\nA\t1\ty\na\t3\tx\n' > tsv.txt
printf 'Banane\napfel\nApfel\nbirne\nzebra\nZebra\n' > mixed.txt
printf '10\n2\n1\n12\n11\n3\n' > num.txt
printf 'zuerst\nÄpfel\nueber\nüber\nStraße\nStrasse\n' > umlaut.txt
printf 'a\n\na\nb\n\n b\n' > blank.txt
awk 'BEGIN{for(i=0;i<400;i++) printf "x"; printf "\n"}' > long.txt
: > empty.txt
printf 'X\000ABC\n' > bin.bin
printf 'ohne Umbruch' > NoNewline.txt

ls -l | awk 'NR>1{print $5, $9}'
echo "gnufix: $FIX ready"
