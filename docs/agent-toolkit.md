# Was aus dem coreutils-Angebot fuer einen Agenten dazugehoert

Stand 17.11.2026. Grundlage: 150 Applets in qiangli/coreutils, 183 Namen in
u-root, davon haben wir 57 (48 im Binary, 9 nur Linux). Schnittmenge geprüft mit
comm; 104 coreutils-Namen fehlen uns.

Die Einordnung unten ist eine Bewertung, keine Messung - was laeuft, steht in
docs/luecken.md und docs/ecosystem.md.

## Stufe 1: ohne das ist ein Agent halb blind

| Befehl | warum |
|---|---|
| diff | der Agent muss seine eigene Aenderung sehen, bevor er sie behauptet |
| patch | Diff anwenden. In Go: go-gitdiff (157 Sterne) parst git- und unified-Diffs und wendet sie an, aber nur strikt, ohne Kontext-Fuzz |
| cut | Felder schneiden, in fast jedem Skript |
| stat | Groesse, Rechte, Zeiten, Inode - Grundlage jeder Dateipruefung |
| split | grosse Ausgaben in Stuecke, wenn das Ausgabelimit des Harness greift |
| file | Typ erkennen, bevor man eine Datei blind liest |
| iconv | Kodierung wandeln; ohne das scheitert alles ausserhalb von UTF-8 |
| env | Programm mit geaenderter Umgebung starten |
| sha256sum + Familie | Pruefsummen, Downloads belegen |
| basename, dirname | Pfadzerlegung, in jedem zweiten Skript |
| nl, tac | Zeilennummern, Umkehrung |
| truncate | Datei auf eine Groesse schneiden |

## Stufe 2: nuetzlich, je nach Aufgabe

join, paste, csplit, od, numfmt, expand, unexpand, fmt, fold, shuf, stdbuf,
nohup, nice, renice, install, chown, chgrp, logger, getconf, nproc, tty,
cksum, sum, b2sum, base32, basenc, ed, pr, pathchk, tsort, m4, make, bc, factor

- ed ist der interessante Fall: ein Zeileneditor, der ueber eine Pipe
  funktioniert, also ohne Terminal - fuer einen Agenten oft brauchbarer als vi.
- stdbuf und nohup helfen, wenn ein Agent einen Prozess startet und die Ausgabe
  lesen will.

## Stufe 3: im Angebot, fuer einen Agenten ohne Wert

talk, write, who, users, lp, mailx, pinky, mesg, newgrp, batch, crontab,
runcon, chcon, localedef, gencat, posix-gate, posix-providers, perfbench, pax,
vdir, uudecode, uuencode, arch, dir, dircolors, hostid, logname, tabs, tput, stty

Dazu die Namen, die bei uns schon Shell-Builtins sind: echo, printf, pwd, test,
true, false.

## Die Schwester yoke hat das Wichtigere

yoke (qiangli) traegt 28 eigene Applets, darunter: jq, tar, gzip, tree, which,
hexdump, watch, cal, tz, tokens, winpath, mount, browser, fetch, db, lens,
graph, why, clip, atq, atrm, ast, resources, nft, ntp, foreman, duration.

jq ist fuer einen Agenten fast wichtiger als alles aus Stufe 1: JSON lesen und
schreiben ist die halbe Arbeit. tar, gzip, tree und which haben wir schon aus
u-root. yoke ist bisher NICHT gebaut oder gemessen - steht als offener Punkt.

## Vorschlag

akmesh bekommt Stufe 1 vollstaendig, dazu aus Stufe 2 ed, od, shuf,
stdbuf, nohup und die Pruefsummen-Familie. patch ueber go-gitdiff, nicht selbst
geschrieben. jq vor dem Bau pruefen: ob yoke ohne bashy-Geruest baut, ist offen.
