# Kernstellen gegen coreutils: was fehlt, was lohnt, was nicht

Stand: 2026-11-18. Gemessen am Linux-Build `./acmesh` (commit 4e40dc2),
aufgenommen mit `./acmesh <befehl> <schalter>` je Zeile. Windows wurde hier
nicht befragt; die Befehlsauswahl ist dort eine andere, die Schalter der
u-root-Befehle aber dieselben.

Leitsatz: Der Agent tippt, was er aus GNU kennt. Ein fehlender Schalter ist
kein Erziehungsproblem, sondern eine Sackgasse: der Agent weicht auf
powershell oder python aus, und die Policy greift nicht mehr.

Drei Routen je Luecke:

- R1: eigener Befehl, Schalter in `internal/tools` nachruesten (billig).
- R2: u-root-Befehl durch eigenen ersetzen (Name wandert in `OURS`; kostet
  Bytes und Tests, aber der Schalter ist dann wirklich da).
- R3: nicht nachruesten, aber den Aufruf mit einer Liste der unterstuetzten
  Schalter und Exit 2 abweisen.

## A. Was schon passt (gemessen)

    chmod u+x            Modusnotation in Symbolform funktioniert
    tee -a, ln -sfr, mkdir -p -m, mktemp -d, comm -12
    cp -R -r -f -i -v -P, mv -n -u, rm -rf, ls -la / -l -a
    grep -r/-rn (rekursiv!), grep -i -n -l -c -v -F
    tail -n +2, head -n 5, wc -l -c -w, cmp -l -s
    sort -n -r -u -b -f -C, seq -w -s -f, strings -n -t
    dd if= of= bs= count=, tar -c -f -x -t -v (unkomprimiert)
    gzip -d -c, base64 -d, shasum -a 256
    date -u -r, id -u/-g/-G/-n, which -a, sleep 0.2 / 1m
    ps -ef / ps aux / ps -eo / ps -p
    df -h -k -m, du -sh, free -h, uname -a
    stat -c '%a %n', cut -d: -f1, cut -c1-3, nl -ba ist NICHT ok (siehe B)
    awk -F -v -f, sed -n -e -f -i, diff -u -L, patch -p -i --dry-run

## B. Nachruestbar, Route und Aufwand

### B1. Eigene Befehle (R1), klein

    du -d/--max-depth              fehlt, Aufwand S
    diff -q -r -N -i -w -b         -r/-q = M, Rest S
    patch -R -N -f                 S
    sed -E -r                      S (Go.Sed kennt ERE)
    cut -b --complement --output-delimiter   S
    nl -ba (kombiniert), -d        S
    tac -s -r                      S
    stat --printf -f -L -t         S
    split -d -n --additional-suffix   S
    truncate -r/--reference        S
    sha*sum -c/--check             M, hoher Nutzen
    timeout -s -k --preserve-status   S
    uname -v -o -p -i              S
    df -i -T -P                    M
    kill -l -s                     S
    ps -u --ppid -C -o feld=       M
    dirname -z                     S
    free -t -w                     S

### B2. u-root-Befehle, die den Namen abgeben muessten (R2)

    sort -k -t -h -V -R            -k/-t ist das Standardmuster fuer Logs und
                                   Tabellen; ohne sie ist sort nur ein
                                   Zeilenalphabet
    wc -m (Chars) --files0-from    -r heisst bei u-root "runes", der
                                   GNU-Aufruf -m scheitert
    chmod -R -v +x                 chmod -R ist alltaeglich, +x scheitert
                                   still mit Exit 1
    tar -z -C --strip-components   tar -czf ist der Normalfall, u-root kann
                                   nur unkomprimiert
    cp -a -p -t -n                 cp -a und cp -t (mit xargs) sind
                                   Skript-Standard
    grep -E -A -B -C -o -w -R      -E und Kontextzeilen fehlen; -r geht schon
    find -maxdepth -exec Startpfad ohne -exec ist find halb; ohne Pfad
                                   scheitert es komplett
    tr -s -c                       tr -s ' ' ist ein Einzeiler, den Agenten
                                   oft brauchen
    xargs -I -r -d -P              xargs -I{} ist die Bruecke zu cp und tar;
                                   fehlt sie, faellt der Agent auf
                                   find -exec zurueck, das auch fehlt
    base64 -w0                     Zeilenumbruch abschalten, fuer Token und
                                   Schluessel
    md5sum -c                      u-root md5sum hat gar keine Schalter,
                                   sha*sum kann -c auch nicht
    hexdump -C                     kanonische Ausgabe
    id -un/-gn (kombiniert)        Go-flag zerlegt kurze Schalter nicht
    ls -1 -t -i --color=never      -1 und -t sind haeufig; sie scheitern
                                   STILL (Exit 0, siehe C)
    date -d -I -s                  -d (Datum rechnen) und -I (ISO) fehlen;
                                   die usage-Zeile nennt -d FILE, das Flag
                                   existiert nicht
    readlink -e -m, realpath -e -m Existenzpruefung
    cpio -o -i -t                  in der GNU-Form gar nicht benutzbar,
                                   nur -H
    touch -t -d -r                 -t scheitert mit Exit 2; -d akzeptiert
                                   nur RFC3339, also kein "2020-01-01"
    head -5 (Kurzform), -q         head -5 ist die Kurzform, die Agenten
                                   tippen
    uniq -f -s -w                  fehlt

### B3. Gemeinsamer Vor-Filter statt R2 (billigste Variante fuer Kombischalter)

Einige Ausfaelle sind reine Syntax, nicht Funktion: `id -un`, `head -5`,
`cp -r` (geht), `gzip -dc`, `ls -la` (geht, u-root nutzt dort getopt).
Ein kleiner Umschreiber vor der Verteilung koennte kombinierte Kurzschalter
aufloesen und `-5` in `-n 5` wandeln - aber nur fuer Befehle, die Go-flag
benutzen. Fuer die getopt-Befehle (ls, grep, rm, cp, dd, tar) waere er
sogar schaedlich. Empfehlung: nicht global, sondern als Liste
"Befehl -> Umschreibungen" mit Test je Eintrag. Sonst entsteht genau die
Rategrammatik, die das Projekt vermeiden will.

## C. Stille Fehlschläge - schlimmer als fehlende Schalter

Gemessen, und der wichtigste Einzelbefund:

    ./acmesh sort -h            druckt Usage, Exit 0, sortiert NICHTS
    ./acmesh ls -Z              druckt Usage, Exit 0, listet NICHTS
    ./acmesh ls -t              druckt Usage, Exit 0
    ./acmesh chmod +x datei     keine Meldung, Exit 1, nichts geaendert
    ./acmesh mkdir -p a/b       ok
    ./acmesh true|echo|pwd      "command is not present in acmesh", Exit 1

`sort -h` ist besonders boshaft: Go-flag deutet `-h` als Hilfe. Der Agent
bekommt Exit 0, keine Fehlermeldung im engeren Sinn, und eine unveraenderte
Datei. Ebenso `ls -Z`: Exit 0, leer.

Regel 0 verlangt bei Nichtverstehen "Liste der unterstuetzten Schalter plus
Exit 2". u-root-Befehle verletzen das an mehreren Stellen. Zwei Wege:
Umschliessen (unsere Huelle prueft vorher, ob der Schalter bekannt ist) oder
je Befehl ersetzen. Ersteres ist der billigere Naeherungsschritt und deckt
auch Befehle ab, die wir nicht neu schreiben wollen.

## D. Bewusst nicht nachruesten (R3, klare Meldung reicht)

- ls: --color, --time-style, -X, --quoting-style - Ausgabeform, kein
  Agentenmuster; ein Mensch am Terminal will Farbe, der Agent nicht.
- tar: --strip-components, --exclude, --transform - Auspacken mit Umbau ist
  Skriptlogik, die in Go schnell falsch wird; klare Meldung.
- find: -printf, -newerXY, -perm /mode, -regex - selten und teuer.
- cpio: ganzes GNU-Modusmodell; wenn gebraucht, eigener Befehl, sonst
  Meldung "use tar".
- xargs -P (parallel) und -a: Nebenlaeufigkeit ohne Nutzen im Harness.
- more/man: interaktiv, im Harness sinnlos; man sollte wenigstens die
  Usage des acmesh-Befehls ausgeben statt "No manual entry".
- time: GNU -v/-f braucht rusage-Details; u-root liefert nur -p. Meldung
  statt Nachbau.
- netcat: ncat-Schaltervielfalt (-broker, -chat, -ssl) - nicht nachbauen,
  dokumentieren.
- dd: conv=-Kombinationen und status=progress-Details - nur die
  gemessenen Formen zusagen.
- date: natuerliche Sprache ("next monday") - bewusst nicht.

## E. Reihenfolge nach Nutzen je Byte

1. Stille Fehlschlaege beenden (C): Exit 2 und Schalterliste, mindestens
   fuer sort, ls, chmod, cp, mv, tar, tr, xargs.
2. echo, true, false, pwd als Befehle aufnehmen - sie fehlen ganz, und ein
   Agent, der `pwd` tippt, landet sofort im Ausweichpfad.
3. sha*sum -c wirklich implementieren (Pruefsummen verifizieren ist der
   haeufigste Grund, ueberhaupt eine Summe zu bilden).
4. sort -k -t, grep -E -A -B -C, find -maxdepth -exec, du -d, diff -r -q,
   chmod -R, tar -z, xargs -I.
5. Der Rest aus B1 (S-Aufwand, je eine Stunde).

## F. Fork statt Neubau (entschieden 2026-11-18)

Martin hat entschieden: die u-root-Befehle werden nicht neu geschrieben,
sondern als gepatchte Kopie geforkt. Gemessen im gepinnten Checkout
(commit 869f00f) ist das tragbar: sort 553, wc 359, tr 457, xargs 316 Zeilen
ohne pkg-Importe; grep 559 + unixflag 164; chmod 567 + uroot/util 26;
tar 314 + tarutil 510 + upath; cp 630 + pkg/cp 149; find 399 + find 398 +
ls 1037; ls 643 + ls 1037. Alle BSD-3 mit Copyright-Kopf.

Drei Auflagen stehen im Fork-Ticket acmesh-e0o: Copyright-Kopf bleibt,
Importe werden mitkopiert statt u-root als Modulabhaengigkeit zu erben (das
Modul liegt nur im go.work, ein require zwaenge jeden Fremdbau ins Netz),
und der Upstream-Commit steht im Dateikopf, damit Drift beim naechsten
Pin-Wechsel auffaellt.

### Reihenfolge der Vorlagen (nachgezogen 2026-11-18)

Nicht jeder Befehl wird am besten bei u-root geforkt. Vor der Kopie die
Quelle waehlen, in dieser Reihenfolge:

1. Eigenbau, wenn die Luecke klein ist (du -d, diff -q, sed -E ...).
2. qiangli/coreutils (MIT, Copyright (c) 2026 Qiang Li; Zeiger in
   docs/ecosystem.md), wenn dort schon eine GNU-vollstaendige Fassung liegt.
   Gemessen: cmds/sort 934 Zeilen mit -k -t -h -V -R --random-source -z
   --files0-from, cmds/shuf 343 Zeilen mit -n -e -i -o -r -z und
   --random-source/--random-seed. Beides MIT, also kopierbar.
   Preis: sort haengt dort an pkg/locale, pkg/collate, pkg/ctype (zusammen
   rund 95 KB Quelltext ohne Tests) fuer die glibc-Locale-Schicht. Im
   Harness genuegt C-Locale, also Byte-Order; die Schicht wird beim
   Uebernehmen gekuerzt und die Kuerzung in docs/design/acmesh.md begruendet.
   shuf haengt nur an tool/ (Applet-Geruest), das durch eigene
   Argumentbehandlung ersetzt wird.
3. u-root forken, wo keine der beiden Quellen passt (grep, chmod, tar, cp,
   xargs, find, tr, wc, base64, head, touch, hexdump).
4. Neu schreiben, wo alle Quellen mehr kosten als der Nutzen.

Folge fuer die Tickets: acmesh-91n (sort) ist von "u-root forken" auf
"qiangli/coreutils als Vorlage" umgestellt, acmesh-f0z (shuf) ist neu und
kommt aus derselben Quelle.

### Ticketliste

    acmesh-nl9  low hanging fruits in internal/tools (Route R1)
    acmesh-e0o  Fork-Mechanik, blockiert alle Fork-Tickets
    acmesh-7s9  stille Fehlschlaege: Schalterhuelle mit Exit 2
    acmesh-4ex  pwd, echo, true, false aufnehmen
    acmesh-91n  sort      acmesh-2nz  grep      acmesh-29n  find
    acmesh-agt  chmod     acmesh-r9r  tar       acmesh-1fs  cp
    acmesh-icw  xargs     acmesh-7ih  wc        acmesh-6lw  tr
    acmesh-dv0  base64    acmesh-86m  head      acmesh-6u2  touch
    acmesh-0fd  hexdump   acmesh-f0z  shuf

## G. Offen

- Windows-Build nicht gemessen; dort sind df, du, free, ps, uname, kill,
  timeout unsere eigenen, also gelten die B1-Zeilen, nicht die u-root-Zeilen.
- Die usage-Zeile von u-root date nennt `-d FILE`, das Flag existiert nicht.
  Entweder Doku oder Schalter - beides jetzt falsch.
