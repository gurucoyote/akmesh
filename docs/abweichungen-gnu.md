# Abweichungen zu GNU: gemessener Stand und was daraus wird (23.11.2026)

Anlass: Martin, 23.11.2026 - "es ist ok, wenn unsere tools zusaetzliche oder andre
schalter haben, es ist nicht ok, wenn agenten ueber fehlende oder falsche schalter
stolpern."

Regel, nach der hier entschieden wird:

- Fehlt ein Schalter, den ein Agent tippt, oder liefert er ein falsches Ergebnis,
  ist das ein Defekt und bekommt ein Ticket.
- Andere oder zusaetzliche Schalter sind kein Defekt.
- Reine Reihenfolge- oder Spaltenkosmetik ohne falsches Ergebnis ist kein Ticket,
  sondern steht hier als Notiz.

Methode: dieselben Aufrufe einmal gegen das gepinnte Referenz-Image
(debian@sha256:cd9f2189...169 80, coreutils 9.7, sed 4.9, grep 3.11, tar 1.35) und
einmal gegen `./akmesh`, im gleichen Verzeichnis mit gleichen Testdateien.
Verglichen wurden Exitcode, stdout und ob stderr leer ist. 179 Aufrufe, 67
Abweichungen; die Sonde liegt als `scripts/deviation-probe.py` bei, Testdaten
erzeugt sie selbst.

Zeitstempel-Unterschiede (Container in UTC, Host in CET) sind abgezogen und
zaehlen nicht als Abweichung.

## A. Fehlende Schalter, ueber die ein Agent stolpert

### find (u-root, cmds/core/find)

    find . -name '*.log' -o -name '*.txt'   find: unknown predicate "-o"
    find . -size +1c                        find: unknown predicate "-size"
    find . -empty                           find: unknown predicate "-empty"
    find . -iname 'A*'                      find: unknown predicate "-iname"
    find . -type f -newer b.log             find: unknown predicate "-newer"
    find . -perm -u+x                       find: -perm: invalid mode "-u+x"
    find . -user root                       find: unknown predicate "-user"
    find . -mtime -1                        find: unknown predicate "-mtime"

`-maxdepth` gibt es inzwischen (die alte Doku-Zeile ist ueberholt), aber
`-maxdepth 0` liefert alles statt nur ".".
`-exec echo {} ;` setzt "a.txt" statt "./a.txt" ein, GNU setzt "./a.txt".

### tail (u-root, cmds/core/tail) - erledigt am 23.11.2026 (acmesh-uzd)

Alle Formen unten gehen jetzt; der Patch liegt als
docs/design/patches/uroot-tail-gnu.patch bei, gemessen gegen GNU in 13 Faellen,
sechs neue Batteriepruefungen.

    tail -3 a.txt        flag provided but not defined: -3
    tail -1 a.txt        flag provided but not defined: -1
    tail -c 5 a.txt      flag provided but not defined: -c
    tail -c +2 a.txt     flag provided but not defined: -c
    tail -q a.txt b.log  flag provided but not defined: -q
    tail -v a.txt        flag provided but not defined: -v

`-n 3` und `-f` gehen. `tail -3` ist die Form, die Agents schreiben.

### grep (eigener Fork, internal/forked/grep) - -P erledigt am 23.11.2026 (acmesh-b87)

-P/--perl-regexp geht jetzt ueber eine reine Go-Portierung von PCRE2 10.44
(internal/pcre), inklusive \K, Lookbehind und possessiven Quantoren.

Erledigt am 23.11.2026 (acmesh-v6r): -m, -x, -H, -a, -z, --include, --exclude.
Offen bleibt nur acmesh-bde: ohne -a meldet GNU bei Binaerdateien "binary file
matches", akmesh gibt die Zeile aus.

Neuer Fund beim Pruefen (erledigt am 23.11.2026, acmesh-4gi): das Standardmuster
war ERE, GNU liest BRE. Jetzt umgeschrieben in internal/forked/grep/bre.go,
12 Aufrufe gegen GNU gleich.

    grep -m1 a a.txt                  grep: unknown option "-m"
    grep -x alpha a.txt               grep: unknown option "-x"
    grep -H alpha a.txt               grep: unknown option "-H"
    grep -a alpha a.txt               grep: unknown option "-a"
    grep -P 'a\w+' a.txt              grep: unknown option "-P"
    grep -z alpha a.txt               grep: unknown option "-z"
    grep --include='*.log' -r x .     unrecognized option '--include=*.log'
    grep --exclude='*.log' -r x .     unrecognized option '--exclude=*.log'

`-c -l -n -v -o -E -i -r -A -B -q -F -w -h` gehen.

### date (u-root, cmds/core/date) - erledigt am 23.11.2026 (acmesh-xli)

-d, -I, --iso-8601, -R und %s/%N/%m gehen jetzt; Patch
docs/design/patches/uroot-date-gnu.patch, 22 Faelle gegen GNU geprueft.

    date -d 2020-01-02 +%s        flag provided but not defined: -d
    date -u -d @0 +%F             flag provided but not defined: -d
    date -I                      flag provided but not defined: -I
    date --iso-8601=seconds       flag provided but not defined: --iso-8601

`date -r DATEI` und alle Formatangaben (`+%F %T`, `+%s`) gehen.

### tar (eigener Code, internal/forked/tar) - erledigt am 23.11.2026 (acmesh-vc3, acmesh-9e7)

-O/--to-stdout da; das Archiv wird nicht mehr in sich selbst gepackt (GNU-Meldung,
Exit 0); Verzeichnisse enden im Archiv auf "/"; die Schreibweise des Arguments
bleibt erhalten ("." wird "./"); absolute Quellpfade funktionieren wie bei GNU
(leading "/" wird entfernt, mit Warnung).

    tar -xOf x.tar a.txt    tar: unknown option "-O"

Zusaetzlich ein echter Fehler: packt man ein Verzeichnis in ein Archiv, das in
diesem Verzeichnis liegt (`tar -cf t.tar .`), bricht akmesh mit
"tar: archive/tar: write too long" ab und laesst ein kaputtes Archiv zurueck;
GNU ueberspringt das Archiv mit Warnung und schreibt den Rest.

### du (eigener Code, internal/disk) - erledigt am 23.11.2026 (acmesh-yd7)

Verzeichnisbloecke, -b (scheinbare Groesse) und -x gehen jetzt; zehn Aufrufe
gegen GNU gleich. Offen bleibt die Zeilenreihenfolge (sortiert statt
Verzeichnisreihenfolge, wie acmesh-kgi bei find).

    du -x .          flag provided but not defined: -x
    du -b a.txt      4096 statt 17 (GNU: Bytes; akmesh: Bloecke)
    du -s .          24 statt 32 (Verzeichnisse selbst werden nicht mitgezaehlt)
    du -a/-h/-d      Pfad ohne "./" vorne

### sed (u-root, cmds/exp/sed bzw. go-sed)

    sed -z 's/a/X/' a.txt    flag provided but not defined: -z

### wc

    wc -L a.txt      wc: unknown option "-L"

### base64

    base64 -w0 a.txt     schreibt einen Zeilenumbruch, GNU nicht
    base64 -d auf unpadded Eingabe ("aGVsbG8")   akmesh: Exit 1 "invalid input",
                                                GNU: Exit 0, "hello"

## B. Schalter vorhanden, Ergebnis weicht ab

### ls -t

    akmesh ls -t    out.txt x.tar x.tgz ...   (innerhalb derselben Sekunde nach Name)
    GNU ls -t       out.txt x.tgz x.tar ...   (nach echter mtime, Nanosekunden)

### df -h

Zahlen stimmen, die Kopfzeile nicht: akmesh schreibt "1K-blocks" und "Use%",
GNU "Size", "Used", "Avail", "Use%". Ein Skript, das die Spalte "Size" sucht,
findet sie nicht.

## C. Kosmetik ohne falsches Ergebnis (kein Ticket, nur Notiz)

- find gibt die Treffer sortiert aus, GNU in Verzeichnisreihenfolge.
- wc richtet Zahlen linksbuendig aus, GNU rechtsbuendig (" 3  3 17" gegen "3 3 17").
- tar schreibt "." wo GNU "./" schreibt.
- diff -u ohne Zeitstempel in den Kopfzeilen; der Patch selbst ist gleich.
- ls -l zeigt die Zeit in der lokalen Zone des Prozesses, im Container also UTC.

## D. Zusaetzlich in akmesh, kein GNU-Gegenstueck

hexdump, free, ps, pgrep, pkill, die Dokument-Konverter, jq, rg, policy, verify.
Kein Handlungsbedarf; hier ist "anders" erwuenscht.

## Was bereits geradegezogen ist

head (alle geprueften Formen inklusive -3, -n -2, -c 0, -q/-v), sort (-n -r -u -k
-t -V -h -c -f -o), cut (-d -f -c --complement -s --output-delimiter), tr (-d -s
-c -cd, Klassen), sed (s, -n, -E, -i, -e, Adressen, y), cp/mv/rm/mkdir/ln/chmod/
touch (gleiches Ergebnisbaum), stat, xargs (-n -I -0 -r), timeout, split, nl,
truncate, iconv, grep (-c -l -n -v -o -E -i -r -A -B -q -F -w -h).

## Tickets zu dieser Liste

    acmesh-x7f  find: Praedikate, -maxdepth 0, -exec ohne ./
    acmesh-uzd  tail: -N, -c, -q, -v
    acmesh-v6r  grep: -m, -x, -H, -a, -P, -z, --include, --exclude
    acmesh-xli  date: -d, -I      erledigt
    acmesh-vc3  tar: -O und Archiv im gepackten Verzeichnis
    acmesh-yd7  du: Verzeichnisbloecke, -b als Bytes, -x
    acmesh-d9e  sed: -z
    acmesh-j66  wc: -L
    acmesh-449  base64: -w0 ohne Umbruch, -d ohne Auffuellung
    acmesh-fgu  ls -t: Nanosekunden statt Sekunden
    acmesh-jag  df: Kopfzeile wie GNU

Reihenfolge, in der ich sie abarbeiten wuerde: tail (haeufigster Aufruf, kleinster
Eingriff), dann date -d, grep, find, du, tar, sed -z, wc -L, base64, ls -t, df.

## Bauaufwand je Ticket (geschaetzt an den Quellen, 23.11.2026)

Gemessen an den Dateien im u-root-Checkout (cmds/core/tail 316 Zeilen, date 259,
find 212, du 175, tar 170, grep 263; go-sed separat):

    klein (unter einer Stunde, ein Patch):
      acmesh-uzd tail        erledigt
      acmesh-j66 wc -L       eine Option, eine Schleife
      acmesh-449 base64      Umbruchbedingung und Auffuellung
      acmesh-fgu ls -t       mtime mit Nanosekunden vergleichen
      acmesh-jag df          Kopfzeile
      acmesh-9e7 tar ./      Praefix im Writer
      acmesh-5ug diff        Zeitstempel in der Kopfzeile
      acmesh-154 wc Spalten  Breite der groessten Zahl

    mittel (ein halber Tag, Parser- oder Strukturarbeit):
      acmesh-xli date -d     eigener Parser fuer @EPOCHE, YYYY-MM-DD, +-N Tage
      acmesh-v6r grep        acht Optionen; -P nur, soweit Go regexp reicht
      acmesh-yd7 du          Blockzaehlung der Verzeichnisse, -b, -x

    gross (eigene Implementierung noetig, nicht nur ein Patch):
      acmesh-x7f find        u-root-find hat eine flache Praedikatkette; -o
                             verlangt einen Ausdrucksbaum mit und/oder/klammern.
                             Entweder find neu schreiben (etwa 400 Zeilen) oder
                             den Parser im Patch ersetzen - der Patch waere dann
                             groesser als das Original.
      acmesh-d9e sed -z      der Satztrenner steckt im go-sed-Fork; ohne Lesen
                             des Fork-Innenlebens nicht abzuschaetzen, vermutlich
                             mittel bis gross.

    nur Entscheidung, kein Bau:
      acmesh-kgi find-Reihenfolge, acmesh-9e7 tar-Praefix (wenn wir es lassen
      wollen), acmesh-5ug diff-Zeitstempel
