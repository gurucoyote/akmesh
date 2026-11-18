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
neben unserem acmesh (8,9 MB) ergibt das 37 MB statt 41 MB, fuer doppelte Bauart
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

## Vorschlag (ueberholt)

Frueher stand hier ein zweites, kleines Binary `acmesh-ext` (eigener
Unterbefehl-Verteiler, ohne Fremdgeruest), das genau die Luecken traegt: ps,
kill, killall, uname, hostname, free, df, du, sync, timeout, spaeter awk und
vielleicht sed.

Seit dem 18.11.2026 gibt es dieses Binary nicht mehr. Die Luecken sind
integriert: jeder eigene Befehl ist ein eigenes main-Paket unter cmd/<name> und
wird von scripts/build-bb.sh in dasselbe Binary gebaut, das schon die
u-root-Befehle traegt.

Die Alternative bleibt offen und ist eine Zeile Arbeit: die volle
coreutils-Lieferung als ein 40,9-MB-Binary, dafuer mit bashy-Gabel und 153
Befehlen.

## Nachtrag: sed und awk konkret (gemessen)

Beides muss nicht aus der grossen coreutils kommen, es sind zwei kleine
Bibliotheken:

- awk: github.com/benhoyt/goawk, MIT, POSIX-kompatibel. Als eigenes Programm
  fuer Windows 4,4 MB; als Bibliothek in einem eigenen Binary eingebettet
  deutlich weniger (siehe unten). `printf "%.2f"` funktioniert.
- sed: github.com/rwtodd/Go.Sed, MIT. Eigenes CLI darauf gebaut, windows/amd64:
  3,0 MB. Getestet: `s/a/b/`, `-n` mit `p`, `y/abc/xyz/`, Dateien und stdin.

Ein Binary mit sed UND awk als Bibliotheken, eigener Verteiler, ohne
Fremdgeruest:

    mb.exe         4,1 MB
    mb.exe -s -w   2,8 MB

Beides unter Windows gelaufen: awk summiert, sed ersetzt, printf mit
Praezision stimmt.

Die eine Falle: Go.Sed nutzt Gos regexp. `s/a \(bc\)/\1!\1/` (GNU-Schreibweise
mit Rueckverweis) trifft NICHT, `s/a (bc)/$1!$1/` (Go-Schreibweise) schon.
Wer GNU-Skripte unveraendert laufen lassen will, braucht die
Uebersetzungsschicht: qianglis cmds/sed/internal/gosed (MIT, 2589 Zeilen,
haengt nur an deren pkg/bre) plus pkg/bre. Das ist uebertragbar, aber ein
Port, kein Import.

Nebenbefund: goawk schreibt unter Windows CRLF, weil es die
Plattform-Zeilenenden nachbildet. Wer Byte-Gleichheit will, setzt den
Newline-Modus.

## Was bliebe der grossen coreutils vorbehalten

150 Namen hat coreutils, 183 hat u-root. 88 davon gibt es nur in coreutils,
darunter diese, die in Skripten wirklich vorkommen:

    cut paste join nl od split stat diff patch csplit
    file iconv sha1sum sha256sum sha512sum cksum
    m4 make bc expr expand unexpand fmt fold numfmt pr shuf shred
    stdbuf tput tabs truncate install chroot runcon

Keiner davon ist technisch unerreichbar: die meisten sind klein und in einem
Nachmittag geschrieben, existieren als Go-Bibliothek oder als einzelnes
Fremdprojekt. Die grosse coreutils ist nicht noetig, weil ein Befehl fehlt,
sondern wenn man viele davon GNU-genau und ohne Eigenpflege will.
