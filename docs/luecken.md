# Luecken fuellen: coreutils selektiv oder selbst bauen

Stand 17.11.2026, alles gebaut und gemessen.

## Frage

Kann man aus qiangli/coreutils nur die Befehle nehmen, die uns fehlen
(awk, sed, uname, hostname, df, du, sync, timeout, ps), statt der ganzen Liste?

## Antwort: technisch ja, gewichtsmassig nein

Jeder Befehl ist ein eigenes Paket mit `tool.Register` in `init`. Ein eigenes
main, das nur bestimmte Pakete importiert, enthaelt nur diese - gebaut und
gemessen, qcore HEAD 6dd7a3e, Ziel windows/amd64:

    uname + hostname            24,2 MB
    sed allein                  24,9 MB
    uname hostname df du sync timeout   24,4 MB
    awk allein                  27,4 MB
    awk + sed                   27,7 MB
    die acht Luecken zusammen   27,9 MB
    alle 153 Befehle            40,9 MB

Der Befund: die Last ist das gemeinsame Geruest (multicall, tool, locale,
collate, x/text, dazu mvdan/sh interp und der goawk-Fork), nicht die einzelnen
Befehle. Selektiv spart 13 MB gegen die volle Liste - aber als zweites Binary
neben unserer bb (8,9 MB) ergibt das 37 MB statt 41 MB, fuer doppelte Bauart
und zwei Verteilungen.

## Zwei Nebenbefunde, die wichtiger sind als die Groesse

1. Wer auch nur ein Applet nimmt, zieht die Shell-Gabel mit. coreutils
   deklariert im go.mod:

       replace mvdan.cc/sh/v3 => github.com/qiangli/sh/v3 v3.0.0-...

   Weil `tool` und `pkg/nudge` die Pakete winmode und polyglot brauchen, die es
   nur in der Gabel gibt. Die Applet-Entscheidung entscheidet also indirekt
   auch die Shell-Frage - das widerspricht dem Gedanken, beides getrennt zu
   waehlen.

2. makebb kann unsere eigenen Befehle nicht aufnehmen. Getestet:

       makebb -o spike.exe ./cmds/core/ls /tmp/acspike/cmd/whoami2
       -> finding packages failed: package ... outside main module or its
          selected dependencies

   Auch `-go-mod` hilft nicht, es setzt nur -mod (vendor, mod, readonly).
   Heisst: ein Binary aus u-root-Befehlen UND eigenen Befehlen gibt es mit
   makebb nicht.

## Der billigere Weg fuer die Luecken

- awk: github.com/benhoyt/goawk direkt, eigenes Binary fuer Windows
  4,4 MB (gemessen), statt 27,4 MB im coreutils-Geruest.
- sed: das CLI von Go.Sed baut fuer Windows NICHT ("undefined:
  syscall.Stat_t"). Entweder die Bibliothek mit eigenem kleinen CLI nutzen
  oder weglassen und die Aufgabe mit awk erledigen.
- ps, kill, killall, uname, hostname, free, df, du, sync, timeout: selbst,
  in einem eigenen kleinen Multicall.

## Vorschlag

Ein zweites, kleines Binary `acmesh-ext` (eigener Unterbefehl-Verteiler, ohne
Fremdgeruest), das genau die Luecken traegt: ps, kill, killall, uname,
hostname, free, df, du, sync, timeout, spaeter awk und vielleicht sed.
Geschaetzt 6 bis 12 MB, abhaengig von awk.

Damit bleibt: bb (8,9 MB, 48 u-root-Befehle) plus acmesh-ext (Luecken), beide
ohne Fremd-Shell-Gabel, und keine 40-MB-Lieferung.

Die Alternative bleibt offen und ist eine Zeile Arbeit: die volle
coreutils-Lieferung als ein 40,9-MB-Binary, dafuer mit bashy-Gabel und 153
Befehlen.
