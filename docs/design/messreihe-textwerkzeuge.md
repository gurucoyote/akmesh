# Messreihe Textwerkzeuge: der naechste GNU-Abgleich

Stand 23.11.2026. Die erste Reihe (179 Aufrufe, 67 Abweichungen, 11 Tickets) ist
abgearbeitet: tail, date, grep (BRE, -P, Schalter, Binaererkennung), du, tar,
find, rg, wc, base64, ls -t, df, diff -u, sed -z. Diese Datei beschreibt die
naechste Reihe, nicht ihre Ergebnisse.

## Warum diese Werkzeuge

Ein Agent verbringt seine Zeit mit Text: sortieren, zaehlen, schneiden, Felder
lesen, Argumente verteilen. Die erste Reihe hat die Werkzeuge geprueft, an denen
er typischerweise haengen bleibt (find, grep, sed, tar). Diese Reihe prueft die,
deren Abweichungen leise sind: falsche Sortierreihenfolge, falsche Feldnummer,
falsches Trennzeichen. Ein falsches sort faellt niemandem auf, es bleibt nur ein
falsches Ergebnis.

Umfang im Binärstand vom 23.11.2026 (134 Namen): awk, sort, uniq, cut, xargs,
head, tail, tr, comm, split, tee, nl, tac, od, seq. Nicht enthalten und daher
eigene Frage: join, paste, expand, unexpand, fmt, pr, fold - dort waere erst zu
entscheiden, ob sie ins Binary gehoeren (Bedarfsmessung acmesh-y2o hat sie nicht
als gebraucht gemeldet).

Bereits im Image gefunden, ohne Messreihe: `od -c` fehlt, `tr '\0'` fehlt. Beide
haben Tickets und sind der Beweis, dass die Reihe noetig ist.

## Verfahren

Das Verfahren bleibt, wie es war, nur der Rahmen ist jetzt gemeinsam:

    scripts/gnu-probe.sh TOOL AUFRUF-DATEI

Der Rahmen startet beide Seiten durch "sh -c", gibt der eigenen Seite ein
"TOOL" im PATH, das auf akmesh zeigt, und vergleicht stdout, stderr und
Exitcode getrennt. getrennt, weil die Reihenfolge im gemeinsamen Kanal von der
Pufferung abhaengt und nichts ueber das Verhalten sagt.

Die Aufrufe stehen als Datei unter scripts/probes/<tool>.probe, eine Zeile pro
Aufruf, '#' als Kommentar. Das Fixture liegt unter /tmp/gnufix und wird von
scripts/gnufix.sh angelegt (noch zu schreiben, siehe Ticket).

Referenz bleibt das gepinnte Debian-Image aus docs/reference-image.txt
(coreutils 9.7, sed 4.9, grep 3.11, gawk).

## Regeln (wie in der ersten Reihe)

- Fehlender oder falscher Schalter ist ein Defekt. Zusaetzliche Schalter sind ok.
- Kosmetik bekommt trotzdem ein Ticket, damit die Entscheidung dokumentiert ist.
- Wo GNU und BusyBox sich unterscheiden, gilt GNU: der Agent kennt GNU.
- Jede Behauptung nennt den Aufruf, der sie belegt.
- Zahlen nach docs/measurements.md, Befund ins Ticket, Ergebnis eine Zeile in
  progress.txt.

## Zuschnitt der Tickets

| Ticket | Werkzeuge | Aufrufe (Ziel) |
|---|---|---|
| Messreihe (Dach) | alle | Verfahren, Fixture, Bericht |
| sort, uniq | sort, uniq | 20 |
| cut, comm, split, tee | diese vier | 20 |
| xargs | xargs | 12 |
| awk | awk (goawk) | 25 |
| head, tail Grenzfaelle | head, tail | 12 |
| tr, od | tr, od | 15 |

Die Grenzfaelle bei head und tail sind kein Misstrauen gegen die frueheren
Patches: dort ging es um -c/-n Schreibweisen, hier um Zahlen groesser als die
Datei, negative Zahlen, mehrere Dateien und die Kopfzeilen, die GNU dann setzt.

awk bekommt die meisten Aufrufe, weil goawk eine andere Engine ist als gawk:
Felder, RS, FS als Regex, printf-Formate, Arrays in der Ausgabe.

## Was nach der Reihe dasteht

Entweder eine Liste gebauter Angleichungen oder eine begruendete Liste bewusster
Abweichungen. Beides ist ein Ergebnis; was nicht geht, steht im Dokument.
