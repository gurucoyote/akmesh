# Messungen

Jede Zahl mit dem Aufruf, der sie erzeugt hat. Ohne Aufruf keine Zahl.

## Bestand (17.11.2026, u-root 869f00f, Go 1.27.4, WSL2 Alpine)

Querkompilieren jedes Kandidaten, einer nach dem anderen:

    GOOS=windows GOARCH=amd64 go build -o /dev/null ./cmds/core/<name>

- Bauen fuer Windows: 48 von 57.
- Bauen nicht: df, du, free, hostname, kill, ps, sync, timeout, uname
  (Meldung "build constraints exclude all Go files").
- cd und pwd sind Builtins des Interpreters und brauchen keinen Befehl.

## Ein Binary (gobusybox)

    GOTOOLCHAIN=auto GOOS=windows GOARCH=amd64 makebb -o acmesh.exe ./cmds/core/...

- Bauzeit 18 s, Groesse 8,9 MB (9.314.304 Bytes) fuer 48 Befehle plus gosh.
- Vergleich: 16 Einzeldateien waren zusammen rund 45 MB.
- Sechs Befehle als Zwischenprobe: 2,5 MB in 7 s.

## Laeuft unter Windows (A, ausgefuehrt aus WSL per Interop)

- `acmesh.exe ls C:/Windows` -> Liste mit Windows-Pfaden (C:\Windows\AppReadiness ...).
- `acmesh.exe grep -c Microsoft C:/Windows/System32/drivers/etc/hosts` -> 2.
- `acmesh.exe gosh -c 'echo hallo; echo $((6*7))'` -> hallo, 42.
- `acmesh.exe` ohne Argument -> Liste der enthaltenen Befehle.
- `acmesh.exe tar` -> Optionshilfe mit -c und -create.

## Kompatibilitaet des Interpreters (mvdan/sh 3.11.0)

34 eigene Skripte, je eines unter dem Interpreter, unter bash und unter
busybox ash; verglichen wurde stdout plus Exit-Code.

- 31 von 34 wie bash, 20 von 34 wie ash (die Abweichungen zu ash sind ashs
  eigene Grenzen).
- Luecken: printf ohne Praezision (%.2f, %.3s) und ohne %q; printf -v fehlt;
  Dateideskriptoren ab 3 paniken; coproc panikt; $RANDOM leer; trap auf
  Signale scheitert, nur EXIT geht; set -x laesst die Zeile des set +x weg.

## Windows-Eigenheiten (A)

- Prozess-Substitution `<( )`: "TODO: support process substitution on Windows".
- Umgebungsvariablen ohne Gross-/Kleinschreibung: nach FOO=bar liefert $foo bar.
- `cd /` wechselt nicht auf eine Wurzel.
- Externe Programme mit UNC-Arbeitsverzeichnis: cmd.exe warnt, laeuft aber.
- Unix-Werkzeuge fehlen ohne das Binary: `ls` -> 127.

## Syntaxunterschiede der u-root-Befehle (A)

- `find` kennt kein `-maxdepth` ("flag provided but not defined").
- `head` kennt kein `-3`, nur `-n 3`.

## acmesh-ext (18.11.2026, gebaut und geprueft)

    ./scripts/build-ext.sh linux     -> ext      3.592.455 Bytes (3,43 MiB)
    ./scripts/build-ext.sh windows   -> ext.exe  3.685.376 Bytes (3,51 MiB)

Beides gestrippt mit -trimpath -ldflags "-s -w". Zusammen mit bb (8,9 MB) also
12,3 bis 12,4 MB fuer den ganzen Werkzeugsatz.

Enthalten: awk sed diff patch cut nl tac basename dirname stat split truncate
sync timeout uname hostname md5sum sha1sum sha256sum sha512sum.

Geprueft mit scripts/test-ext.sh, je Ziel 19 Pruefungen, beide 0 Fehler:
- basename mit Suffix, dirname, cut -f und -c, tac, nl
- sha256sum gegen das Systemprogramm, gleiche Summe
- sed s///, sed -n, awk Summe, awk -F
- diff erzeugt, patch wendet an, Ergebnis gleich der Zieldatei
- truncate -s, split -l mit Anzahl und Zeilen je Teil
- uname -m, timeout endet mit 124

Go-Tests in cmd/ext: Eigenschaft diff->replay ergibt beide Seiten, Hunk-Kopf
passt zur Zeilenzahl (go-gitdiff ist strikt), Hunk-Startzeilen, cut-Listen,
Teilnamen, Groessenangaben. go test ./cmd/ext gruen.

Fehlt noch aus Stufe 1: file, iconv, env. Dazu ps, kill, killall, free, df,
du aus dem zweiten Plan.

## Ein Binary statt zwei (17.11.2026, gebaut und geprueft)

    ./scripts/build-bb.sh windows -> acmesh.exe 10.503.680 Bytes (10,0 MiB)
    ./scripts/build-bb.sh linux   -> bb     10.596.476 Bytes (10,1 MiB)

Enthalten: 48 u-root-Befehle, gosh, ssh auf beiden Zielen, strace und
df/du/free/kill/ps unter Linux, dazu unsere eigenen Werkzeuge aus
internal/tools als eigene Befehlsnamen.

Zum Vergleich zwei Binaries: 8,9 MiB (bb) plus 3,5 MiB (ext) = 12,4 MiB. Die
Ersparnis kommt daher, dass die Go-Laufzeit nur einmal drin ist.

Der Weg dahin: makebb nimmt nur Pakete aus dem Hauptmodul. Ein go.work mit
beiden Modulen hebt die Grenze auf. Danach kollidierte md5sum mit u-root; der
Build filtert jetzt automatisch: wo u-root den Namen fuer das Ziel baut, gilt
u-root, sonst unser Paket.

Pruefung: scripts/test-ext.sh gegen acmesh.exe und gegen bb, je 19 Faelle, beide
0 Fehler. ssh antwortet mit seiner Usage, strace mit der seinen, ps und free
liefern Daten.

Bekannt kaputt: u-root df unter WSL meldet
"mountinfo()=_,no such device, want: _,nil". Ein eigenes df steht auf der Liste.
Das go.work liegt in .build/ und wird vom Build erzeugt; es ist nicht im Repo.

## Umbenannt (17.11.2026)

Das Binary heisst jetzt acmesh beziehungsweise acmesh.exe, gebaut von
scripts/build-bb.sh (Skriptname bleibt). Frueher hiess es bb; in den aelteren
Abschnitten dieses Dokuments und in docs/groesse.md steht noch bb, weil das die
Groessen von damals sind. Inhaltlich ist es dasselbe Binary.

## file, iconv, env (17.11.2026, gebaut und geprueft)

    ./scripts/build-bb.sh windows -> acmesh.exe 10.546.688 Bytes (10,06 MiB)
    ./scripts/build-bb.sh linux   -> acmesh     10.641.532 Bytes (10,15 MiB)

Die drei Befehle kosten zusammen rund 43 KB gegenueber dem Stand ohne sie
(10.503.680 beziehungsweise 10.596.476 Bytes).

Belegt am gebauten Binary:
- acmesh.exe file acmesh.exe -> "PE32+ executable (console) x86-64, for MS Windows"
- acmesh file acmesh -> "ELF 64-bit LSB executable, x86-64"
- acmesh.exe iconv -f CP1252 -t UTF-8 auf "Gr\xfc\xdfe \x80" -> "Gruesse Euro"
  mit richtigen Umlauten und Euro-Zeichen
- acmesh.exe env A=1 acmesh.exe env zeigt A=1 im Kindprozess
- iconv -l nennt acht Zeichensaetze: UTF-8, ASCII, ISO-8859-1, ISO-8859-15,
  WINDOWS-1252, UTF-16LE, UTF-16BE, UTF-16

Pruefungen: scripts/test-ext.sh jetzt 27 Faelle je Ziel, Windows und Linux
beide 0 Fehler. Go-Tests: Rundlauf ueber alle 256 Byte je Einbyte-Satz,
Latin-1- und CP1252-Sonderfaelle, UTF-16 in beiden Byte-Reihenfolgen mit und
ohne BOM, nicht abbildbare Zeichen, Signaturen fuer ELF, PE, PDF, PNG, GIF,
gzip, Zip, SQLite, Text und Binaerdaten, env mit letzter Zuweisung gewinnt.

Bewusste Grenze: kein Zeichensatz-Erkennung, kein //TRANSLIT, und file kennt
eine kurze Signaturliste statt einer Magic-Datenbank.

## ps, kill, killall (18.11.2026, gebaut und geprueft)

    ./scripts/build-bb.sh linux   -> acmesh     10.616.956 Bytes (10,13 MiB)
    ./scripts/build-bb.sh windows -> acmesh.exe 10.592.256 Bytes (10,10 MiB)

Zuwachs gegenueber dem Stand ohne die drei: rund 55 beziehungsweise 46 KB.

ps liest unter Linux /proc (stat, status, cmdline, /proc/stat fuer btime,
/etc/passwd fuer Namen), unter Windows Toolhelp32 plus OpenProcess,
GetProcessTimes und OpenProcessToken fuer CPU-Zeit und Benutzer.

Geprueft am gebauten Binary:

| Aufruf | Linux | Windows |
|---|---|---|
| ps | PID TTY TIME CMD | PID TTY TIME CMD |
| ps -e | alle | alle |
| ps -ef | USER PID PPID STAT ELAPSED TTY TIME CMD | dito |
| ps aux | USER PID STAT TTY TIME CMD | dito |
| ps -eo pid,ppid,cmd | freie Feldwahl | freie Feldwahl |
| ps -p 1 | ein Prozess | ein Prozess |
| kill PID | getestet an sleep 300 | getestet an PING.EXE |
| killall NAME | sleep beendet, ohne Treffer Exit 1 | zwei PING.EXE beendet |
| kill -NOSUCHSIGNAL | Exit 2 | Exit 2 |
| kill an geschuetzten Prozess | EPERM | "Zugriff verweigert" |

Unter Windows belegt: ps -eo pid,ppid,user,cmd zeigt "BUSYS\Spernau" als
Benutzer, explorer.exe mit CPU-Zeit 16:57 und Laufzeit 7-14:12:38.

Pruefungen: scripts/test-ext.sh 34 Faelle je Ziel, acmesh, acmesh.exe und ext
alle 0 Fehler. Go-Tests: Aufrufe -e/-f/-ef/aux/-x/-p, Feldlisten, Ablehnung
unbekannter Schalter, Spaltenkoepfe, Zeitformate, Namensvergleich fuer killall
(Gross-/Kleinschreibung, .exe), Spaltenausrichtung.

Bewusste Grenzen:
- Unter Windows kein Kommandozeilen-Text in ps (PEB, siehe
  docs/ps-kill-einschaetzung.md), nur der Programmname.
- ps aux zeigt nicht %CPU, %MEM, VSZ, RSS, START. Statt Zahlen zu erfinden
  fehlen die Spalten; das steht auch in der Hilfe.
- Unter Windows gibt es nur KILL/TERM, alles andere ist ein Fehler mit Meldung.
- SIGNAL an einen anderen Benutzer braucht Rechte; ohne Rechte Exit 1 mit
  Meldung, kein Absturz.

## free, df, du (18.11.2026, gebaut und geprueft)

    ./scripts/build-bb.sh linux   -> acmesh     10.604.668 Bytes (10,11 MiB)
    ./scripts/build-bb.sh windows -> acmesh.exe 10.616.320 Bytes (10,12 MiB)

Zuwachs gegenueber dem Stand ohne die drei: praktisch nichts, weil die
Windows-Aufrufe in der Standardbibliothek liegen und x/sys schon drin war.

-h ist von Anfang an dabei, mit 1024er Stufen und einer Nachkommastelle unter
zehn: 512, 1.0K, 1.5K, 10K, 1.0M, 3.0G, 1.8T.

Belegt am gebauten Binary:

| Aufruf | Linux | Windows |
|---|---|---|
| free | Mem und Swap in KiB | Mem und Swap, Swap als - |
| free -h | 31G total, 3.6G used, 1.4G free | 64G total, 34G used, 29G free |
| df -h | /dev/sdd 1007G 519G 437G 51% / | C: 1.8T 1.1T 757G 59% |
| df -h / | eine Zeile, / | - |
| df -h /mnt/c | C: mit 1.8T, aus Sicht von Linux | - |
| du -h -s docs | 72K | 51K |
| du -c -h docs/design | 4.0K Pfad plus 4.0K total | dito |

Der Unterschied bei du zwischen den Zielen ist gewollt: Linux zaehlt belegte
Bloecke (st_blocks), Windows die Dateigroesse, weil dort die belegte Groesse
einen Handle pro Datei braeuchte.

Bewusste Grenzen:
- free zeigt unter Windows shared und buff/cache als -, weil
  GlobalMemoryStatusEx das nicht hergibt; Swap ebenso, weil TotalPageFile minus
  TotalPhys etwas anderes ist als Swap-Nutzung. Lieber ein Strich als eine
  falsche Zahl.
- df listet Laufwerke, die nicht bereit sind, nicht auf, statt abzubrechen.
- df mit einem Pfad, der zu keinem Einhängepunkt gehoert, gibt eine Meldung und
  Exit 1, damit niemand eine leere Tabelle fuer eine leere Platte haelt.
- du folgt symbolischen Verweisen nicht und zaehlt harte Verweise einmal.

Pruefungen: scripts/test-ext.sh jetzt 44 Faelle je Ziel, acmesh, acmesh.exe und
ext alle 0 Fehler. Go-Tests: Groessenformat, Einheitenschalter, du auf einem
gebauten Baum (Summe, -a, -c, nicht kleiner als eine Datei), Ablehnung
unbekannter Schalter, Kopfzeilen von free und df samt gleicher Feldzahl je
Zeile.
