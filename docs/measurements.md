# Messungen

Jede Zahl mit dem Aufruf, der sie erzeugt hat. Ohne Aufruf keine Zahl.

Groessen: seit 23.11.2026 immer in MiB und Bytes, mit Bindeart und Stripping,
wie es die `report:`-Zeile des Baus druckt (Regel: AGENTS.md, Bau). Aeltere
Abschnitte nennen nur Bytes; sie bleiben, wie sie gemessen wurden.

## Bestand (17.11.2026, u-root 869f00f, Go 1.27.4, WSL2 Alpine)

Querkompilieren jedes Kandidaten, einer nach dem anderen:

    GOOS=windows GOARCH=amd64 go build -o /dev/null ./cmds/core/<name>

- Bauen fuer Windows: 48 von 57.
- Bauen nicht: df, du, free, hostname, kill, ps, sync, timeout, uname
  (Meldung "build constraints exclude all Go files").
- cd und pwd sind Builtins des Interpreters und brauchen keinen Befehl.

## Ein Binary (gobusybox)

    GOTOOLCHAIN=auto GOOS=windows GOARCH=amd64 makebb -o akmesh.exe ./cmds/core/...

- Bauzeit 18 s, Groesse 8,9 MB (9.314.304 Bytes) fuer 48 Befehle plus gosh.
- Vergleich: 16 Einzeldateien waren zusammen rund 45 MB.
- Sechs Befehle als Zwischenprobe: 2,5 MB in 7 s.

## Laeuft unter Windows (A, ausgefuehrt aus WSL per Interop)

- `akmesh.exe ls C:/Windows` -> Liste mit Windows-Pfaden (C:\Windows\AppReadiness ...).
- `akmesh.exe grep -c Microsoft C:/Windows/System32/drivers/etc/hosts` -> 2.
- `akmesh.exe gosh -c 'echo hallo; echo $((6*7))'` -> hallo, 42.
- `akmesh.exe` ohne Argument -> Liste der enthaltenen Befehle.
- `akmesh.exe tar` -> Optionshilfe mit -c und -create.

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

    ./scripts/build-bb.sh windows -> akmesh.exe 10.503.680 Bytes (10,0 MiB)
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

Pruefung: scripts/test-ext.sh gegen akmesh.exe und gegen bb, je 19 Faelle, beide
0 Fehler. ssh antwortet mit seiner Usage, strace mit der seinen, ps und free
liefern Daten.

Bekannt kaputt: u-root df unter WSL meldet
"mountinfo()=_,no such device, want: _,nil". Ein eigenes df steht auf der Liste.
Das go.work liegt in .build/ und wird vom Build erzeugt; es ist nicht im Repo.

## Umbenannt (17.11.2026)

Das Binary heisst jetzt akmesh beziehungsweise akmesh.exe, gebaut von
scripts/build-bb.sh (Skriptname bleibt). Frueher hiess es bb; in den aelteren
Abschnitten dieses Dokuments und in docs/groesse.md steht noch bb, weil das die
Groessen von damals sind. Inhaltlich ist es dasselbe Binary.

## Ein Binary, ein Name (18.11.2026)

Das eigene ext-Binary ist entfernt, cmd/ext gibt es nicht mehr. Jeder eigene
Befehl ist nur noch ein cmd/<name>-Paket, das build-bb.sh in dasselbe Binary
baut. Die Testbatterie heisst deshalb scripts/test-akmesh.sh. In den aelteren
Abschnitten steht noch scripts/test-ext.sh und die Groesse von ext: das waren
Messungen am damaligen zweiten Binary, sie bleiben als Verlauf stehen.

## ext entfernt, Meldungen auf akmesh (18.11.2026)

cmd/ext und der zweite Verteiler (tools.Main, usage, Names, List) sind weg.
Jeder eigene Befehl ist nur noch ein cmd/<name>-Paket. Skripte: build-ext.sh
geloescht, test-ext.sh heisst test-akmesh.sh.

    ./scripts/build-bb.sh linux   -> akmesh     11.358.332 Bytes (10,83 MiB)
    ./scripts/test-akmesh.sh ./akmesh -> 76 checks, 0 failures
    go test ./internal/...        -> ok policy, ok tools

Meldungstext: "akmesh: unknown command" nennt jetzt den aufgerufenen Namen
(programName aus os.Args[0]).

Gepatcht am 18.11.2026: `./akmesh gibtsnicht` antwortet mit
"command is not present in akmesh". Der Text steckt per //go:embed in makebb,
deshalb ersetzt build-bb.sh ihn in den gobusybox-Quellen und baut makebb neu,
wenn es noch den alten Text enthaelt. Der Bau prueft danach sein eigenes
Binary und bricht ab, wenn der alte Text noch drin ist.

    ./scripts/build-bb.sh linux   -> akmesh     11.341.948 Bytes (10,82 MiB)
    ./scripts/build-bb.sh windows -> akmesh.exe 11.367.936 Bytes (10,84 MiB)
    ./scripts/test-akmesh.sh ./akmesh     -> 79 checks, 0 failures
    ./scripts/test-akmesh.sh ./akmesh.exe -> 65 checks, 0 failures

(Beide Groessen mit go1.27.0 gebaut, siehe den Toolchain-Befund im naechsten
Abschnitt.)

## makebb gepinnt (18.11.2026)

MAKEBB_VERSION=v0.6.0 steht in scripts/build-bb.sh; der Bau liest die Version
mit go version -m aus dem installierten Werkzeug und laedt nur, wenn es fehlt
oder nicht auf v0.6.0 steht. Danach ersetzt er den Dispatcher-Text und baut
makebb genau dann neu, wenn der alte Text noch eingebettet ist.

Frischer Pfad geprueft: makebb geloescht, Modul-Cache auf den alten Text
zurueckgesetzt, ./scripts/build-bb.sh linux laeuft durch und das Binary meldet
"not present in akmesh".

    ./scripts/build-bb.sh linux   -> akmesh     11.358.332 Bytes (10,83 MiB)
    ./scripts/build-bb.sh windows -> akmesh.exe 11.383.296 Bytes (10,84 MiB)
    ./scripts/test-akmesh.sh ./akmesh     -> 79 checks, 0 failures
    ./scripts/test-akmesh.sh ./akmesh.exe -> 65 checks, 0 failures

Wichtiger als makebb: auch der Go-Toolchain ist nicht gepinnt. GOTOOLCHAIN=auto
hat waehrend dieser Arbeit von go1.27.0 auf go1.27.4 gewechselt, und derselbe
Quellstand ergibt damit 11.341.948 gegen 11.358.332 Bytes. Reproduzierbar ist
der Bau erst, wenn auch die Toolchain feststeht.

## file, iconv, env (17.11.2026, gebaut und geprueft)

    ./scripts/build-bb.sh windows -> akmesh.exe 10.546.688 Bytes (10,06 MiB)
    ./scripts/build-bb.sh linux   -> akmesh     10.641.532 Bytes (10,15 MiB)

Die drei Befehle kosten zusammen rund 43 KB gegenueber dem Stand ohne sie
(10.503.680 beziehungsweise 10.596.476 Bytes).

Belegt am gebauten Binary:
- akmesh.exe file akmesh.exe -> "PE32+ executable (console) x86-64, for MS Windows"
- akmesh file akmesh -> "ELF 64-bit LSB executable, x86-64"
- akmesh.exe iconv -f CP1252 -t UTF-8 auf "Gr\xfc\xdfe \x80" -> "Gruesse Euro"
  mit richtigen Umlauten und Euro-Zeichen
- akmesh.exe env A=1 akmesh.exe env zeigt A=1 im Kindprozess
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

    ./scripts/build-bb.sh linux   -> akmesh     10.616.956 Bytes (10,13 MiB)
    ./scripts/build-bb.sh windows -> akmesh.exe 10.592.256 Bytes (10,10 MiB)

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

Pruefungen: scripts/test-ext.sh 34 Faelle je Ziel, akmesh, akmesh.exe und ext
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

    ./scripts/build-bb.sh linux   -> akmesh     10.604.668 Bytes (10,11 MiB)
    ./scripts/build-bb.sh windows -> akmesh.exe 10.616.320 Bytes (10,12 MiB)

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

Pruefungen: scripts/test-ext.sh jetzt 44 Faelle je Ziel, akmesh, akmesh.exe und
ext alle 0 Fehler. Go-Tests: Groessenformat, Einheitenschalter, du auf einem
gebauten Baum (Summe, -a, -c, nicht kleiner als eine Datei), Ablehnung
unbekannter Schalter, Kopfzeilen von free und df samt gleicher Feldzahl je
Zeile.

## curl und wget (18.11.2026, gebaut und geprueft)

    ./scripts/build-bb.sh linux   -> akmesh     10.657.916 Bytes (10,16 MiB)
    ./scripts/build-bb.sh windows -> akmesh.exe 10.669.568 Bytes (10,18 MiB)
    ./scripts/build-ext.sh linux  -> ext         7.442.695 Bytes (7,10 MiB)

Im akmesh kosten curl und wget rund 53 KB, weil net/http und TLS durch ssh
schon drin waren. Das eigene ext-Binary waechst dagegen um 3,9 MB, weil es
vorher kein net/http enthielt. Das ist der Beleg fuer die Regel aus
docs/groesse.md: die Zutat zaehlt, nicht der Befehl.

Belegt am gebauten Binary (Linux und Windows):
- curl -s https://example.com gibt den Body aus. Genau das kann gocurl nicht.
- curl -fsSL https://example.com folgt dem Redirect, zusammengefasste Schalter.
- curl -sS -f auf eine 404-Seite: Meldung "HTTP error 404", Exit 22.
- wget -q -O - https://example.com schreibt nach stdout.
- wget -q -S -O /dev/null zeigt "< 200 OK" und die Kopfzeilen.
- wget --spider meldet "Remote file exists."

Go-Tests gegen einen httptest-Server, damit sie ohne Netz reproduzierbar sind:
-s mit Body, -fsSL, ohne -L kein Folgen, -H wirkt, -d ergibt POST mit Formular,
-u erzeugt den Basic-Auth-Kopf, -F mit Datei, Cookies ueber -b und -c samt
geschriebener Jardatei, -f mit 404 Exit 22, --max-time 0.1 auf eine langsame
Route ergibt Zeitueberschreitung, -o schreibt die Datei, -r 2-4 liefert "234",
file:// wird mit Exit 3 abgelehnt, wget-Front mit -O -, --post-data und -S,
unbekannte Schalter Exit 2, -r bei wget Exit 2.

Exit-Codes wie curl: 6 Namensaufloesung, 7 Verbindung, 22 HTTP-Fehler mit -f,
28 Zeit, 35 TLS, 47 zu viele Redirects. Nur http und https; file:// und ftp://
sind ein Fehler, kein stiller Fallback.

Pruefungen: scripts/test-ext.sh 50 Faelle je Ziel, akmesh, akmesh.exe und ext
alle 0 Fehler.

Grenzen: kein -w (Formatsprache), keine Fortschrittsanzeige, kein HTTP/2
sondern das, was net/http aushandelt, Cookies im einfachen Netscape-Format,
wget ohne rekursives Spiegeln und ohne -N.

## jq im Binary (18.11.2026)

    ./scripts/build-bb.sh linux   -> akmesh     11.481.212 Bytes (10,95 MiB)
    ./scripts/build-bb.sh windows -> akmesh.exe 11.510.272 Bytes (10,98 MiB)
    ./scripts/build-ext.sh linux  -> ext         9.097.479 Bytes (8,68 MiB)

Zuwachs gegenueber dem Stand ohne jq: 823.296 Bytes (804 KB) Linux,
840.704 Bytes (821 KB) Windows. Die Vorabmessung mit einer Probe im Binary
hatte 802.816 / 817.152 Bytes ergeben; der kleine Unterschied ist die
Schalterverarbeitung um die gojq-Aufrufe herum.

Belegt am gebauten Binary:
- jq -r '.items[] | "\(.n)=\(.v)"' auf einer Datei gibt "a=1" und "b=2".
- ./akmesh curl -fsSL -A akmesh https://api.github.com/repos/golang/go | \
  ./akmesh jq -r '"\(.full_name) stars=\(.stargazers_count)"' ->
  "golang/go stars=140183".
- Windows: dieselbe URL durch akmesh.exe jq -c '.license.key' -> "bsd-3-clause".
- HTML statt JSON ergibt "jq: invalid JSON: invalid character '<' ..." und
  Exit 2, also eine Meldung, mit der man etwas anfangen kann.

Pruefungen: scripts/test-ext.sh 58 Faelle je Ziel, akmesh, akmesh.exe und ext
alle 0 Fehler. Go-Tests: Felder, verschachtelte Pfade, .a[], -c, -r, -rc, -s,
mehrere Dokumente im Strom, -n, select, map, length, --arg, --argjson,
--slurpfile, -e mit 0/1/4, -R und -R -s, kaputter Filter (Exit 3), kaputtes
JSON (Exit 2), unbekannter Schalter (Exit 2), Datei als Eingabe.

Nebenbefund: gojq marshalisiert anders als encoding/json (NaN wird null,
<>& werden nicht maskiert). Fuer die huebsche Ausgabe wird json.Indent auf die
gojq-Ausgabe gelegt, damit die Maskierung von gojq erhalten bleibt.

## Policy und eigene Shell (18.11.2026)

    ./scripts/build-bb.sh linux   -> akmesh     11.370.620 Bytes (10,84 MiB)
    ./scripts/build-bb.sh windows -> akmesh.exe 11.399.168 Bytes (10,87 MiB)
    ./scripts/build-ext.sh linux  -> ext         9.810.183 Bytes (9,36 MiB)

Die Policy-Schicht und die eigene Shell kosten zusammen nichts, sie sparen
sogar: gegenueber dem Stand mit jq und mit u-root-gosh sind es 110.592 Bytes
weniger (Linux) und 111.104 Bytes weniger (Windows). Der Grund ist der Tausch
von u-root gosh gegen unsere duenne mvdan/sh-Front: die Zeilenbearbeitung und
die Ergänzungen der u-root-Shell fallen weg, die Landlock-Aufrufe sind ein paar
hundert Zeilen mit x/sys.

Belegt (Linux, WSL-Kernel 6.18, Landlock ABI 7):
- akmesh policy meldet Exit 0, die Policy-Datei und die Pfade.
- Lesen in /work erlaubt, Lesen von /etc/hostname verweigert.
- Schreiben ausserhalb verweigert, die Datei entsteht nicht; innen erlaubt.
- curl mit leerer Netzliste: "policy ... does not allow network access to
  example.com", Exit 1.
- Kaputte Policy (version 9): Exit 1, kein ungeschuetzter Lauf.
- Windows mit gesetzter Policy: Exit 1, "a filesystem policy cannot be enforced
  on windows yet".

Pruefungen: scripts/test-ext.sh Linux 74, Windows 62, alle 0 Fehler. Go-Tests
neu: internal/policy (Parsing, Tippfehler, relative Pfade, Version, Hosts mit
Port und Wildcard, fehlende Policy, kaputte Datei).

## Eigene ssh und scp (18.11.2026)

    ./scripts/build-bb.sh linux   -> akmesh     11.350.140 Bytes (10,82 MiB)
    ./scripts/build-bb.sh windows -> akmesh.exe 11.378.688 Bytes (10,85 MiB)

Gegenueber dem Stand mit u-root-ssh sind das je 20.480 Bytes weniger, obwohl
scp neu dazugekommen ist. Der Grund: x/crypto war durch die alte ssh schon im
Binary, unsere Front ist duenner als die u-root-Shell um dieselbe Bibliothek.

Belegt durch scripts/test-ssh.sh gegen einen echten sshd (15 Pruefungen, 0
Fehler, auf akmesh und akmesh.exe): Ausgabe, Mehrzeiligkeit, Remote-Exitcode 7,
test -f wahr und falsch, stdin-Pipe, unbekannter Host ohne Freigabe Exit 255,
Upload, rekursiver Upload, Download, rekursiver Download, jeweils mit Inhalt.

## scp -p (18.11.2026)

    ./scripts/build-bb.sh linux   -> akmesh     11.354.236 Bytes (10,83 MiB)
    ./scripts/build-bb.sh windows -> akmesh.exe 11.382.272 Bytes (10,86 MiB)

Zuwachs gegenueber scp ohne -p: 4.096 Bytes je Ziel. Umsetzung: T-Records des
scp-Protokolls in beide Richtungen, rund 60 Zeilen samt einer kleinen
Plattformdatei fuer die Zugriffszeit (Linux) beziehungsweise fuer ihr Fehlen
(Windows).

Belegt: Datei mit Zeitstempel 2020-01-02 03:04:05 behaelt ihn beim Hochladen und
beim Herunterladen, Modus 600 bleibt 600, rekursiv behalten die Kinder ihre
Zeiten; ohne -p bekommt die Kopie die aktuelle Zeit.

-C wird abgelehnt: x/crypto/ssh unterstuetzt nur compressionNone, das
scp-Protokoll selbst hat keine Kompression. Fortsetzen gibt es in scp nicht,
auch nicht in OpenSSH; dafuer waere ein sftp-reget noetig.

Pruefungen: scripts/test-ssh.sh jetzt 18 Faelle je Ziel, beide 0 Fehler.

## Profile und TinyGo (18.11.2026)

    ./scripts/build-bb.sh linux full  -> akmesh     11.354.236 Bytes (10,83 MiB)
    ./scripts/build-bb.sh linux fs    -> akmesh-fs   9.777.276 Bytes (9,32 MiB)

fs ist 1.576.960 Bytes kleiner. In seinem Abhaengigkeitsgraphen kommen net/http,
crypto/tls und golang.org/x/crypto/ssh nicht mehr vor (go list -deps: je 0), und
die Meldungen des Netzteils fehlen im Binary. Pruefungen: 66 statt 74, die acht
Netzpruefungen werden uebersprungen.

TinyGo 0.39.0 (Go 1.25.7, LLVM 19.1.2) baut mit dem Tag nonet sechs Einzelbefehle
aus demselben Paket: basename 7.381.640, jq 7.381.840, sh 7.381.792, sed
7.381.680, awk 7.381.632, cut 7.381.680 Bytes. Go baut denselben Einzelbefehl in
8.478.113 Bytes. Hello-World unter TinyGo: 1.083.552 Bytes.

Die Netzbefehle bauen unter TinyGo nicht: http.Transport hat dort kein
Proxy-Feld, und net/http/cookiejar fehlt. golang.org/x/crypto/ssh allein baut
(2.496.232 Bytes), es ist also der Http-Teil.

## Korrektur 18.11.2026: TinyGo spart nichts

Der erste TinyGo-Vergleich stellte 7.381.640 Bytes (TinyGo) gegen 8.478.113
Bytes (Go, ungestrippt) und las daraus 1,1 MB Vorteil. Unser Bau laeuft aber mit
-s -w. Richtig gemessen, gleicher Befehl, Tag nonet:

    TinyGo                    7.381.640 Bytes (7,04 MiB)
    Go gestrippt              5.861.668 Bytes (5,59 MiB)
    Go ungestrippt            8.478.113 Bytes (8,09 MiB)

TinyGo ist also 1.519.972 Bytes groesser als der gestrippte Go-Bau. Ein
TinyGo-Bau des fs-Multicalls waere entsprechend rund 11,3 MB statt 9,32 MiB.

## Korrektur 18.11.2026: das fs-Profil enthielt Netzcode

Die Profil-Ausschluesse haben nie gegriffen. scripts/cmds.txt gruppiert die
Namen auf Zeilen ("ping netcat"), der Filter verglich aber ganze Zeilen, also
traf "exclude ping" nichts, und die Ausschlussliste nannte netcat zudem "nc".
Damit blieb netcat im fs-Binary, und netcat zieht crypto/tls mit.

Behoben in scripts/build-bb.sh (Liste wird zu einer Zeile pro Name, Mitgliedstest
ueber eine wieder zusammengesetzte Zeile) und scripts/profiles/fs.txt (netcat
statt nc). Gemessen mit Go:

    akmesh-fs vor der Korrektur     9.777.276 Bytes (9,32 MiB)
    akmesh-fs nach der Korrektur    6.959.228 Bytes (6,64 MiB)

2.817.048 Bytes, die vorher als Netzcode mitgelaufen sind. Die Aussage, das
fs-Binary enthalte kein net/http und kein TLS, galt nur fuer unseren eigenen
Code; die u-root-Applets waren nie ausgeschlossen.

## Voller fs-Bau unter TinyGo (18.11.2026)

makebb laesst sich mit -compiler umlenken, das ist der Weg. Der Quelltext wird
mit -g erzeugt, dann baut TinyGo ihn. Vier Huerden, alle im erzeugten Baum
geloest, ohne Aenderung am Repository ausser den Skripten:

- Modul und Abhaengigkeiten verlangen Go 1.27, TinyGo traegt 1.25.7: go.mod und
  vendor/modules.txt werden heruntergesetzt;
- u-root markiert viele Dateien mit "!tinygo", die Befehle lassen sich aber mit
  dem Tag tinygo.enable zurueckholen; 5 Dateien brauchten die Ersetzung
  "!tinygo" -> "(!tinygo || tinygo.enable)";
- klauspost/compress hat unter TinyGo keine little-endian-Implementierung,
  weil die einzige Datei an "!purego" haengt: Konstante im erzeugten Baum
  entfernt;
- cpio braucht os.Root, time braucht ProcessState.UserTime, beides fehlt in
  TinyGos os: diese zwei Befehle fallen raus.

Gemessen, gleicher Befehlssatz (fs ohne cpio und time):

    Go, -s -w          6.828.156 Bytes (6,51 MiB)
    TinyGo, -no-debug  4.430.544 Bytes (4,22 MiB)

Also 2.397.612 Bytes weniger, 35 Prozent kleiner. Der volle fs-Bau mit allen
Befehlen ist unter Go 6.959.228 Bytes, unter TinyGo nicht baubar.

Was laeuft, geprueft am Binary: jq (6 aus [1,2,3]), sed, awk (Summe 6), sh mit
Arithmetik, ls, cat. Was nicht laeuft, die Pruefbatterie test-ext.sh:
61 von 66 gruen, 5 rot. Alle fuenf scheitern an TinyGos os/exec, das kein Dir und
keine zusaetzlichen Dateien setzt:

    env-assign           "env: files setting not implemented"
    timeout-expires      "sh: directory setting not implemented"
    sh-pipe              leere Ausgabe
    policy-read-inside   leere Ausgabe
    policy-write-inside  leere Ausgabe

## Tests nach der Durchsicht (18.11.)

    go test ./internal/...      -> ok policy, ok tools
    ./scripts/test-ext.sh ./ext -> 76 checks, 0 failures

76 Prüfungen, davon zwei neu: "write implies read", drinnen erlaubt und
draußen verweigert. Neu in Go: Summenvektoren,
Verteilungstabelle gegen cmd/<name>, Zeitlimit liefert 28 statt 7, 302 ohne -L
liefert den Weiterleitungstext statt einer leeren Ausgabe.

## sort und shuf (18.11.2026)

Beide sind jetzt eigener Code: `internal/sort` (Treiber von uns, Vergleicher
und -k-Grammatik aus qiangli/coreutils, MIT) und `internal/shuf` (Eigenbau nach
GNU-Oberflaeche). Die Namen belegen sie selbst; `sort` steht nicht mehr in
`scripts/cmds.txt`, beide stehen in `OURS_ALL`.

Differenztest gegen echtes GNU sort, 42 Aufrufe, 0 Abweichungen:

    podman run --rm -i -v $PWD:/work:ro,Z docker.io/library/debian:stable-slim \
        sort -V /work/f.txt

Geprueft: Vorgabe, -r, -u, -n, -f, -d, -b, -i, -M, -V, -h, -g, -s, -z,
-k2, -k2,2, -k1.1,1.2, -t' ' -k2,2n, -k2,2 -u, -k1,1 -k2,2n, -c, -C, -cu,
Eingabe von stdin, leere Eingabe. Dabei fielen vier Fehler der Vorlage auf,
die behoben sind: monthCompare verglich bei gleichem Monat den Text im Key
statt den Key gleich zu lassen, unitOrder nahm ohne Zahl den ersten Buchstaben
als SI-Einheit ("Zebra" galt als Zetta), filevercmp war ein Bytevergleich
statt gnulib-Ordnung ('~' vor allem, Nicht-Alphanumerisches hinter Buchstaben),
und -g las "0x10" nicht als 16.

Was gegen GNU NICHT abgedeckt ist: nur C/POSIX-Locale. Ein anderes LC_ALL oder
LC_COLLATE wird mit Exit 2 abgewiesen statt still byteweise zu sortieren.

    go test ./internal/sort ./internal/shuf   -> ok, 16 GNU-Faelle plus
                                                 Eigenschaftstests fuer -R und shuf
    ./scripts/test-akmesh.sh ./akmesh         -> 88 checks, 0 failures
    ./scripts/test-akmesh.sh ./akmesh.exe     -> 74 checks, 0 failures

Groesse (vorher 11.358.332 / 11.383.296):

    akmesh      11.395.196   +36.864
    akmesh.exe  11.425.792   +42.496

-R verhaelt sich wie GNU: Hash je Schluessel, gleiche Schluessel bleiben
zusammen. Gemessen: printf 'b\na\nb\n' | ./akmesh sort -R -> b, b, a.
Wer wirklich durchmischen will, nimmt shuf (eigener Befehl).

## Testabdeckung je Befehl (18.11.2026)

Zaehlung gegen die Befehlstabelle (34 Namen), nicht gegen Dateien:

    Go-Tests beruehren direkt      9   curl wget df du free split sha*sum jq ps
    nur die Live-Batterie         25   awk basename cut diff dirname env file
                                       iconv kill killall nl patch sed tac
                                       timeout truncate uname sort shuf ...
    keine Pruefung irgendwo        4   stat sync hostname policy

Die Batterie (scripts/test-akmesh.sh, 88 Pruefungen auf Linux) ist eine echte
Ebene: sie laeuft gegen das gebaute Binary. Sie ist aber Smoke - sie belegt
"Antwort kommt", nicht "Antwort ist richtig". Bei stat z.B. prueft sie nichts,
obwohl Agenten `stat -c '%a %n'` parsen.

Hollow-Test-Pruefung: die vorhandenen Negativtests tragen ihren Gegenbeleg
(TestCurlNoFollowWithoutL prueft den Rumpf der 302, nicht nur "enthält das Ziel
nicht"). Der eine t.Skip in internal/policy ist begruendet (eine aktive Policy
im Prozess) und nicht als Dauerausnahme gedacht.

Neu geschlossen: die Tabelle konnte einen Namen auf den falschen Befehl zeigen,
ohne dass irgendein Test das merkte - cmd/<name>/main.go ruft Run("name")
richtig, waehrend der Tabelleneintrag auf den Nachbarn zeigt. Dagegen jetzt
TestNoCommandSharesAnotherImplementation und, fuer die Closures der Summen,
TestSumsAreWiredToTheirOwnAlgorithm (vergleicht gegen die veroeffentlichten
Vektoren von "abc").

Rotsproben gefahren, jede mit dem erwarteten Grund, danach zurueckgenommen:

    "stat": runSync            -> FAIL "stat and sync share one implementation"
    "sha256sum": runSum("md5") -> FAIL "printed 90015098..., want the sha256sum digest"
    Monatsordnung gedreht      -> FAIL TestSortMatchesGNU/-M
    -R mit festem Seed         -> FAIL "-R produced the same order five times"

## echo, pwd, true, false (18.11.2026)

Vier Befehle, die in u-root liegen, aber nie in `scripts/cmds.txt` standen -
ein Agent, der `pwd` oder `echo` tippt, landete im Ausweichpfad, den die Policy
nicht sieht. Jetzt eigener Code in `internal/tools/corecmds.go`, alle vier in
`OURS_ALL`.

echo bewusst NICHT von u-root uebernommen: dessen echo interpretiert Escapes
standardmaessig (`-e` ist dort true). Gemessen gegen GNU echo 9.7:

    /bin/echo 'a\nb'        -> a\nb      (GNU: literal)
    /bin/echo -e 'a\nb'     -> zwei Zeilen
    /bin/echo -e 'x\0101y'  -> xAy
    /bin/echo -e 'x\x41y'   -> xAy
    /bin/echo -e 'a\cb'     -> "a", ohne Zeilenumbruch

Unsere Fassung liefert dieselben fuenf Faelle (TestEchoMatchesGNU), u-roots
haette beim ersten und beim letzten abweichend geantwortet. Rotsprobe:
Escapes standardmaessig an -> FAIL "default-keeps-backslash".

pwd -P loest Symlinks auf, -L bleibt logisch (Vorgabe). Unter Windows druckt
das Binary einen UNC-Pfad; die Batterie vergleicht deshalb nur die letzte
Pfadkomponente.

    ./scripts/test-akmesh.sh ./akmesh     -> 95 checks, 0 failures
    ./scripts/test-akmesh.sh ./akmesh.exe -> 81 checks, 0 failures

Groesse: akmesh 11.403.388 (+8.192), akmesh.exe 11.431.936 (+6.144).

## Stille Fehlschlaege von u-root-Befehlen (18.11.2026)

Drei gemessene Faelle, alle behoben:

    ls -Z, ls -t, ls -1    vorher Exit 0 nach gedruckter Usage (und die
                           Verzeichnisliste kam trotzdem); jetzt Exit 2
    chmod +x DATEI         vorher Exit 1, KEINE Meldung, nichts geaendert;
                           jetzt 0755
    chmod 999 DATEI        vorher still; jetzt "chmod: ... unable to decode
                           mode", Exit 1 (GNU chmod exit 1, im Container gemessen)

Ursache bei ls: `f.Parse(...)` mit ContinueOnError, Rueckgabe ignoriert, danach
wird trotzdem gelistet. Ursache bei chmod: die Klassenbuchstaben waren Pflicht,
GNU erlaubt "+x", und main hat den Fehler verworfen statt ihn zu drucken.

Bauform: scripts/patch-uroot.sh, aufgerufen aus build-bb.sh nach dem
Dispatcher-Rename. Jeder Patch ist ein woertlicher Austausch mit zwei Wachen -
steht der neue Text schon da, wird der Schritt uebersprungen; fehlt der alte,
bricht der Build ab. Der Riegel wurde geprueft: ein gefaelschter Checkout
(/tmp/fakeuroot mit umformulierter ls.go) endet mit
"patch-uroot: expected text not found in .../ls.go", Exit 1.

Noch offen und nur festgehalten: cp, rm, mv, mkdir, find und gzip beenden einen
unbekannten Schalter mit Exit 1 statt GNU's 2. Die Meldung ist eindeutig
("flag provided but not defined: ..." plus Usage), der Agent erkennt den Fehler,
deshalb bleibt es bei der Notiz.

    ./scripts/test-akmesh.sh ./akmesh     -> 101 checks, 0 failures
    ./scripts/test-akmesh.sh ./akmesh.exe -> 86 checks, 0 failures

Eine Pruefung ist plattformabhaengig: unter Windows gibt es kein x-Bit, dort
zaehlt nur der Exit-Code von chmod +x.

## Tests fuer stat, sync, hostname, policy (18.11.2026)

Die vier Befehle, die als einzige gar keine Pruefung hatten. Je eine Behauptung
statt "lief ohne Absturz", alle vier mit Rotsprobe:

    stat     %a ohne fuehrende Null ("644"), %s = 6, %F = "regular file",
             Standardform beginnt mit "PFAD 6 0644", fehlender Operand Exit 2,
             unbekannter Schalter Exit 2, fehlende Datei Exit 1
    sync     vorhandene Datei Exit 0, fehlende Datei Exit 1, ohne Operand
             Linux 0 / Windows 1 (dort gibt es kein ganze-System-Sync)
    hostname = os.Hostname(), mit Operand Exit 2 ("setting the name is not
             supported" statt still ignorieren)
    policy   ohne AKMESH_POLICY Exit 1 und "no policy ...", --check auf eine
             lesbare Datei "allowed: " Exit 0, auf eine fehlende "denied: "
             Exit 1, --help Exit 0, unbekanntes Argument Exit 2

Rotsproben, jeweils FAIL mit dem erwarteten Grund und danach zurueckgenommen:

    stat: "%04o" -> "%o"          -> FAIL TestStatFormatDirectives
    sync: Fehlerzweig -> return 0 -> "sync on a missing file: exit 0, want 1"
    hostname: Operand akzeptiert  -> "hostname with an operand: exit 0, want 2"
    policy: Exit 1 -> 0           -> "no policy = 0 ... want exit 1"

Batterie: 113 Checks Linux, 97 Windows, 0 Fehler. Zwei Pruefungen sind
plattformabhaengig: das x-Bit und der Modus von chmod/stat gibt es unter
Windows nicht.

## coreutils-Schalter in internal/tools (18.11.2026, acmesh-nl9)

Gegen GNU coreutils im Container (docker.io/library/debian:stable-slim) mit
denselben Dateien und Aufrufen verglichen. Linux-Binary ./akmesh, Windows
./akmesh.exe aus scripts/build-bb.sh, jeweils 11 MiB.

    stat -c %a %A %s %F %i %U:%G %h %b   identisch
    stat -t                              identisch in allen 16 Feldern, %f und
                                         %D in hex (vorher dezimal: falsch)
    stat --printf '%n\t%s\n'             Tab echt, kein eigener Zeilenumbruch
    stat -L / ohne -L                    Link als Link, mit -L als Zieldatei
    stat -f -c %T /                      ext4 (Host), overlayfs im Container
    stat (Standardblock)                 Labels File, Size, Device, Inode, Links,
                                         Access, Modify, Change wie GNU
    diff -i a A                          Exit 0 (vorher 1 mit leerem Kopf)
    diff -w / -b                         Exit 0
    diff -q                              "Files A and B differ", kein Hunk
    diff -r a b                          "Only in a: x", Exit 1
    diff -r -N a b                       Kopf "diff -r -N a/x b/x" plus Hunk
    patch w.txt < p.diff                 Exit 0, Datei geaendert
    patch -N doppelt                     "Reversed (or previously applied)
                                         patch detected!  Skipping patch.", Exit 1
    patch -R                             Exit 0, Ausgangsstand wiederhergestellt
    patch -f -p1                         Exit 0
    timeout -s KILL 1 sleep 5            137 (GNU: 137)
    timeout 1 sleep 5                    124 (GNU: 124)
    timeout --preserve-status 1 sleep 5  143 (GNU: 143)
    kill -s 0 PID                        Exit 0, freie PID Exit 1
    kill -l                              enthaelt EXIT und KILL

Testbilanz: go test ./internal/tools gruen, 21 neue Pruefungen in
internal/tools/compat_test.go. Batterie 113 Checks Linux, 97 Windows, 0 Fehler.

## Erster Fork: head (18.11.2026, acmesh-e0o)

Vorlage fuer die Fork-Mechanik, gleichzeitig Ticket acmesh-86m (-5 und -q).
Kopie: internal/forked/head/head.go, 256 Zeilen upstream plus Run() und
Argumentparser, Upstream-Commit 869f00f6b224ce02b1db51806159d6cd9c246f76.

Gemessen am gebauten Binary (go build ./cmd/head, dann ./scripts/build-bb.sh):

    head -5 (stdin 1..12)       1 2 3 4 5      Exit 0   (vorher: invalid option)
    head -q a.txt b.txt         a b ohne Kopf  Exit 0   (vorher: -q unbekannt)
    head -v a.txt               mit Kopf       Exit 0
    head a.txt b.txt            zwei Koepfe     Exit 0, wie GNU
    head -n 2 / -c 3            upstream-Verhalten unveraendert
    head /nicht/vorhanden       Exit 1          (vorher Exit 0 trotz Fehlermeldung)
    head -Z                     Exit 2 mit usage
    head -n 2 -c 2              Exit 2 (errCombine)

Driftprobe: UROOT_COMMIT in scripts/build-bb.sh auf Null-Commit gesetzt ->
TestManifestMatchesPin FAIL mit "fork head was taken from 869f00f... but the
build pins 0000...", danach zurueckgenommen und wieder gruen.

Kosten, gemessen mit scripts/build-bb.sh vor und nach dem Fork:

    Linux    11481212 -> 11485308 Bytes  (+4096, 4 KiB)
    Windows  11505664 -> 11511808 Bytes  (+6144, 6 KiB)

Der erste Zwischenstand (+8 KiB) war noch mit u-root's head im Binary: solange
der Name in scripts/cmds.txt steht, gewinnt u-root und der Fork ist unerreichbar
(gemessen: head -5 lief weiter in den u-root-Befehl, Exit 0 bei fehlender
Datei). Erst nach dem Streichen des Namens aus cmds.txt liefert der Fork.

Der Fork ersetzt den u-root-Befehl, er kommt nicht dazu: ein Name, ein Befehl.
Die 8 KiB sind der Preis fuer den eigenen Argumentparser und die
Fehlerbehandlung.

## Forks: find und grep (18.11.2026, acmesh-29n, acmesh-2nz)

Beide nach der Mechanik aus acmesh-e0o geforkt. Verglichen mit GNU (debian
stable-slim) auf denselben Dateien, Ausgabe und Exit-Code.

    find                                      GNU
    find . -name '*.txt'                      identisch, mit "./"-Praefix
    find -name a.txt (ohne Startpfad)         identisch (vorher: usage, Exit 1)
    find . -maxdepth 1 -type d                identisch (".", "./sub")
    find . -mindepth 2 -type f                identisch
    find . -type f -not -name '*.log'         identisch
    find . -name '*.txt' -exec CMD {} ;       laeuft, kein impliziter Print
    find /nicht/vorhanden                     Exit 1

    grep                                      GNU
    grep alpha a.txt                          identisch
    grep -E 'al.a'                            identisch
    grep -i bravo                             identisch
    grep -w alp / -w alpha                    identisch
    grep -o alp / -o -n alpha                 identisch
    grep -A 1 / -B 1 / -C 1 / -C 2 alpha      identisch, inklusive "--" nur bei Luecke
    grep -rn alpha .                          identisch, "./"-Praefix
    grep -l / -c (eine und zwei Dateien)      identisch
    grep zenix                                Exit 1 (vorher Exit 0)
    grep '('                                  Exit 2 mit Meldung (vorher: Panic)

Zwei Fehler, die die Vergleiche gefunden haben: ein Verzeichnis auf maxdepth
wurde uebersprungen statt gemeldet (find . -maxdepth 1 -type d fehlte ./sub),
und bei grep wurden after-context-Zeilen doppelt gedruckt und "--" auch ohne
Luecke gesetzt.

Groesse (vor find/grep: 11485308 Linux, 11511808 Windows):

    Linux    11501692 Bytes  (+16384 fuer beide Forks)
    Windows  11527168 Bytes  (+15360 fuer beide Forks)

Tests: internal/forked/find (6) und internal/forked/grep (6), go test ./...
gruen. Batterie: 128 Checks Linux, 112 Windows, 0 Fehler; neue Pruefungen
find-optional-path, find-dot-prefix, find-maxdepth, find-exec, find-missing,
grep-extended, grep-ignore-case, grep-word, grep-context, grep-only-match,
grep-no-match, grep-recursive.

## Forks: chmod, xargs, cp, tar (18.11.2026, acmesh-agt, acmesh-icw, acmesh-1fs, acmesh-r9r)

Verglichen mit GNU (debian stable-slim), Ausgabe und Exit-Code.

    chmod                                     GNU
    chmod -v 755 f.txt                        identisch ("mode of 'f.txt' changed from 0644 (rw-r--r--) to 0755 (rwxr-xr-x)")
    chmod -c 755 f.txt (unveraendert)         identisch (keine Ausgabe)
    chmod -R 700 sub                          identisch
    chmod +x / u+x                            identisch (Patch +x bleibt im Fork)
    chmod --reference=a.txt b.txt             identisch
    chmod 999 f.txt                           Exit 1, "invalid mode: '999'" + Try-Zeile
    chmod 755                                 Exit 1, "missing operand after '755'" + Try-Zeile

    xargs                                     GNU
    xargs -I{} echo '[{}]'                    identisch (eine Zeile, Leerzeichen bleiben)
    xargs -n 2 echo                           identisch
    xargs -0 echo                             identisch
    xargs -d, echo                            identisch
    xargs -r echo  (leer)                     identisch (kein Lauf)
    Kind schlaegt fehl                        Exit 123 (GNU), Befehl fehlt: 127

    cp                                        GNU
    cp -a src dst                             identisch, Modus 640 und mtime 2020-01-02 03:04:05 erhalten
    cp -R src dst                             identisch
    cp -t DIR a b                             identisch
    cp -n a b                                 identisch (b bleibt)
    cp -v a b                                 identisch ("'a' -> 'b'")
    cp nix x                                  Exit 1, "cannot stat 'nix': No such file or directory"

    tar                                       GNU
    tar -czf x.tgz d                          gzip-Archiv, GNU liest es (tar -tzf und -xzf)
    tar -tzf x.tgz                            identisch zur GNU-Liste
    tar -tf x.tgz (ohne -z)                   identisch (gzip am Magic erkannt)
    tar -xzf gt.tgz (GNU-Archiv)              entpackt vollstaendig
    tar -czf y.tgz --exclude='*.log' d        skip.log fehlt, Rest vollstaendig
    tar -xzf x.tgz -C out --strip-components=1  a.txt und sub/b.txt oben
    tar -cf z.tar -C d a.txt                  Archiv enthaelt nur a.txt
    tar -cjf x.tbz                            Exit 2 mit Meldung (kein bzip2 im Binary)

Befund beim Bauen: tar war noch in scripts/cmds.txt, also gewann u-root und der
Fork war im Binary unerreichbar - dieselbe Falle wie bei head. Erst nach dem
Streichen des Namens war -czf da.

Groesse (vor dieser Runde: 11501692 Linux, 11527168 Windows):

    Linux    11620476 Bytes  (+118784 fuer chmod, xargs, cp, tar)
    Windows  11650560 Bytes  (+123392 fuer chmod, xargs, cp, tar)

Tests: internal/forked/chmod (4), xargs (6), cp (4), tar (5); go test ./...
gruen. Batterie: 152 Checks Linux, 133 Windows, 0 Fehler; neue Pruefungen
chmod-recursive(-mode), chmod-verbose, chmod-try-hint, xargs-replace/-chunk/
-no-run-empty/-child-fails, cp-archive(-mode/-nested), cp-target-dir/-files,
cp-no-clobber, tar-create-gzip, tar-list-gzip, tar-list-without-z, tar-exclude
(-worked), tar-extract-strip, tar-strip-file, tar-C-create/-listing,
tar-j-refused.

## Einzelschalter: hexdump -C, base64 -w, wc -m, tr -s/-c, touch -t/-d (18.11.2026)

Geforkt und gegen GNU (debian stable-slim) verglichen, Ausgabe und Exit-Code.
Hexdump und touch brauchten Werkzeuge, die im slim-Image fehlen bzw. eine
Zeitzone: hexdump kam per bsdextrautils, touch verglich ich mit TZ=Europe/Berlin.

    hexdump -C                               byte-identisch zur GNU-Ausgabe
                                             (Offset, 16 Slots in zwei Achtergruppen, |ASCII|)
    hexdump -C -n 4                          identisch

    base64 (Default 76 Spalten)              identisch
    base64 -w 10 / -w0                       identisch
    base64 -d / -d -i                        identisch; ohne -i Exit 1 auf Muell

    wc / wc -l / -m / -c / -lw               identisch
    wc --files0-from=LISTE                   identisch (je Datei eine Zeile, total)

    tr a-z A-Z                               identisch (Bereiche werden aufgeloest)
    tr -s a / -s ' '                         identisch (ein SET genuegt)
    tr -cd 0-9                               identisch
    tr -c a AB                               identisch (SET2 wird mit dem letzten Zeichen verlaengert)
    tr -ds a / tr a                          Exit 1, GNUs zwei Zeilen plus Try-Hinweis

    touch -t 202001020304                    identisch (lokale Zeit)
    touch -t 2001020304                      identisch (zweistelliges Jahr)
    touch -t 202001020304.05                 identisch
    touch -d '2020-01-02 03:04:05'           identisch
    touch -d 2020-01-02 / -d now             identisch
    touch -r DATEI                           identisch

Zwei Fehler, die die Vergleiche gefunden haben: base64 liess den letzten
Teilblock weg (der Encoder wurde nicht geschlossen, bei 100 Zeichen Eingabe
fehlten die letzten Zeichen), und tr behandelte "a-z" als drei Zeichen statt als
Bereich - damit war jedes tr a-z A-Z falsch. Ausserdem standen tr und wc noch in
scripts/cmds.txt, also gewann u-root und die Forks waren unerreichbar.

Groesse (vor dieser Runde: 11620476 Linux, 11650560 Windows):

    Linux    11645052 Bytes  (+24576 fuer die fuenf Forks)
    Windows  11674624 Bytes  (+24064 fuer die fuenf Forks)

Tests: hexdump (3), base64 (3), wc (3), tr (5), touch (4); go test ./...
gruen. Batterie: 165 Checks Linux, 145 Windows, 0 Fehler; neue Pruefungen
hexdump-canonical/-ascii/-final-offset, base64-wrap0/-default-wrap, wc-chars,
wc-bytes, tr-range, tr-complement, tr-squeeze, touch-stamp(-year), touch-date.

## Stufe 1 vollstaendig: file, iconv, env (18.11.2026, acmesh-94e.13)

Die drei zuletzt offenen Befehle aus Stufe 1 stehen schon im Binary; geprueft
wurde auf beiden Zielen, mit der Batterie und von Hand:

    file script.sh        Linux und Windows: "script.sh: ASCII text"
    file u8.txt           Linux und Windows: "u8.txt: UTF-8 Unicode text"
    file -b bin.dat       "ELF data"
    iconv -f UTF-8 -t ISO-8859-1 u8.txt   Linux und Windows: h<0xE4>llo
    iconv -f ISO-8859-1 -t UTF-8          "Gruesse" mit echten Umlauten
    iconv -f CP1252 -t UTF-8              Euro-Zeichen
    env FOO=bar                           Kind sieht FOO, Ausgabe wie GNU
    env -u HOME                           HOME fehlt im Kind

Rundlauf diff gegen patch, wie die Abnahme es verlangt:

    diff -u a.txt b.txt > p.diff      Exit 1, sobald es einen Unterschied gibt (wie GNU)
    patch a2.txt p.diff               Exit 0
    cmp a2.txt b.txt                  gleich
    gleiches auf Windows mit akmesh.exe: ebenfalls gleich

Batterie: 165 Pruefungen Linux, 145 Windows, 0 Fehler; diff-header, patch-apply
und patch-result decken den Rundlauf dauerhaft ab, file-*, iconv-* und env-*
die drei Befehle.

Groesse: 11796604 Bytes Linux, 11825664 Bytes Windows (mit mvdan/sh 3.14.1).

## Aufteilung internal/tools, Etappe 1: die forked Befehle (18.11.2026, acmesh-mq1)

Gemessen mit scripts/measure-size.sh, das die Zahl mit dem Befehl zusammen
ausgibt (neues Skript, damit keine Zahl mehr aus dem Gedaechtnis kommt).

Vorher: cmd/<name>/main.go rief internal/tools.Run("name"), und die Tabelle in
internal/tools zieht jeden eigenen Befehl mit. Ein Binary mit genau einem
eigenen Befehl wog 10.338.428 Bytes, mit zwei eigenen exakt dasselbe.

Nachher: die zwoelf forked Befehle importieren ihr eigenes Paket
(internal/forked/<name>), nicht mehr die Tabelle.

    probe (base64)            1.753.212 Bytes   vorher 10.338.428
    probe (base64 wc tr)      1.871.996 Bytes   vorher 10.338.428
    Bodenwert echo true pwd   1.814.652 Bytes   (u-root allein, Verteiler + Go-Laufzeit)

Das heisst: der Verteiler kostet nichts, jeder weitere eigene Befehl rund 60 KB
im Probe-Bau. Die 8,6 MB Unterschied waren die nicht gebrauchten Befehle.

Volles Binary unveraendert: 11796604 Linux, 11827200 Windows. Erwartet, denn
dort sind ohnehin alle Befehle drin; die Tabelle in internal/tools bleibt als
Rueckweg fuer die Tests stehen.

Batterie nach dem Umbau: 165/0 Linux und 145/0 Windows.

Was noch fehlt: die uebrigen rund 28 Befehle liegen weiter in einem Paket
internal/tools (fileops, text, netz, prozess). Fuer die muss der Code erst
herausgezogen werden, die zwoelf hier hatten ihr Paket schon.

## Aufteilung Etappe 2: Pruefsummen, und sha*sum -c (18.11.2026, acmesh-mq1, acmesh-t5k)

Die vier Pruefsummen-Befehle liegen jetzt in internal/sum, ihre cmd/main.go
importieren nur dieses Paket. Gleichzeitig ist -c gebaut, der Teil, den das
Ticket acmesh-t5k verlangt hat (vorher: "sha256sum: -c is not supported yet",
Exit 2).

Gegen GNU (debian stable-slim), Ausgabe und Exit-Code:

    sha256sum -c gute Liste        "a.txt: OK", Exit 0
    sha256sum -c Liste mit Fehler  "a.txt: FAILED" + "WARNING: 1 computed checksum did NOT match", Exit 1
    sha256sum -c Liste mit fehlender Datei
                                   "nix.txt: FAILED open or read" +
                                   "sha256sum: nix.txt: No such file or directory" +
                                   "WARNING: 1 listed file could not be read", Exit 1
    sha256sum --status -c          nichts auf stdout, Exit 1
    Trennzeichen "hex *name"       wird verstanden (Binaerform von GNU)

Die Ausgabe ist byte-gleich zu GNU in allen drei Faellen.

Groesse: 11800700 Linux, 11833856 Windows. Probe mit nur sha256sum:
2.019.452 Bytes (vorher 10.338.428, weil die Tabelle des Pakets alles mitzog).
Der Aufschlag gegen den Bodenwert von 1.814.652 kommt von crypto/sha256.

Batterie: 169 Checks Linux, 149 Windows, 0 Fehler; neue Pruefungen
sums-check-ok, sums-check-exit, sums-check-failed, sums-check-failed-exit.

## Aufteilung Etappe 3: die kleinen Textbefehle (18.11.2026, acmesh-mq1)

basename, dirname, nl, tac und cut liegen jetzt in eigenen Paketen
(internal/basename, internal/dirname, internal/nl, internal/tac,
internal/cut). Die gemeinsamen Dateihelfer (OpenInputs, CloseInputs,
ReadAllLines) stehen in internal/shared, damit sie einmal existieren und nicht
in jedes Paket kopiert werden. truncate bleibt vorerst bei fileops, das kommt
mit der Datei-Gruppe.

Probe mit den vier Textbefehlen: 2.031.740 Bytes; vorher trug ein Bau mit einem
einzigen eigenen Befehl 10.338.428 Bytes.

Groesse des vollen Binarys: 11804796 Linux, 11835904 Windows (der Umbau selbst
aendert sie kaum, weil dort alle Befehle drin sind).

go test ./... gruen. Der Test fuer die cut-Liste (1, 1,3, 2-4, 5-) ist mit dem
Code nach internal/cut gewandert, statt in internal/tools zurueckzubleiben.

Batterie: 169 Checks Linux, 149 Windows, 0 Fehler.

## Aufteilung Etappe 4: die Datei-Gruppe (18.11.2026, acmesh-mq1)

stat, truncate, split, sync, uname und hostname liegen in eigenen Paketen unter
internal/. stat hat seine Formathelfer (statfmt.go) und die beiden
Plattformdateien (statfs_linux.go, statfs_other.go) mitgenommen; die
Passwortabfrage fuer %U und %G steht als shared.ReadPasswd mit Build-Tags in
internal/shared, weil ps sie spaeter auch braucht. truncate und split teilen
sich shared.ParseSize (2K, +5, -5).

Nachgemessen gegen GNU (debian stable-slim) nach dem Umbau, damit die Aufteilung
nicht still etwas veraendert:

    stat -c %U:%G /etc/passwd     root:root            gleich
    stat -c %a %n /etc/passwd     644                 (GNU meldet %n ebenfalls als fehlende Datei)
    stat -f -c %T /               ext4                (im Container overlayfs, der Name kommt von dort)
    stat -t /etc/passwd           gleich bis auf das Geburtsdatum, das wir als 0 zeigen
    stat -c %s /etc/passwd        1431                gleich
    sync, sync DATEI              Exit 0, Datei wird geflusht (Windows: Hinweis statt Vortaeuschung)

Probe mit den sechs Befehlen: 1.843.324 Bytes, also kaum mehr als der Bodenwert
von 1.814.652 - der Aufschlag ist syscall und time.

Groesse: 11808892 Linux, 11838976 Windows.

Tests: die split- und ParseSize-Tests sind mit dem Code gewandert
(internal/split, internal/shared); go test ./... gruen. Batterie: 169 Checks
Linux, 149 Windows, 0 Fehler. Ein Fehler unterwegs: mein erster sync kannte nur
den Aufruf ohne Argumente und brach den alten Test, der sync DATEI prueft -
behoben, bevor irgendetwas committet wurde.

## Aufteilung Etappe 5: Prozess und Datentraeger (18.11.2026, acmesh-mq1)

ps, kill und killall liegen in internal/proc, weil sie ein Prozessmodell teilen
(procInfo, die Signal-Tabelle, das Auflisten). timeout liegt in
internal/timeout und holt sich Signaltabelle und Signalnamen aus proc ueber
NormaliseSignal, SignalNumber und SignalProcess - zwei Tabellen waeren
auseinandergelaufen. free, df und du liegen in internal/disk mit ihren
storage_*.go-Dateien.

Die Altdateien platform_unix.go/platform_windows.go/platform_other.go in
internal/tools wurden dabei nicht mehr gebraucht: ihre Helfer (inodeOf, absPath,
wholeSystemSync) sind mit den Befehlen umgezogen.

Fehler unterwegs, beide vor dem Commit gefunden: mein erster sync kannte nur den
Aufruf ohne Dateiargumente, und der Windows-Bau brach mit "absPath redeclared",
weil internal/disk die Funktion doppelt hatte. Beide behoben.

Probe mit free, df, du, ps, kill und timeout: 2.125.948 Bytes.

Groesse: 11812988 Linux, 11843584 Windows.
go test ./... gruen, Batterie 169/0 Linux und 149/0 Windows.

## Aufteilung Etappe 6: die Netzgruppe, und die Policy (18.11.2026, acmesh-mq1)

curl, wget, ssh und scp liegen jetzt in internal/net, samt Cookie-Jar, scptimes
und der Engines. Die nonet-Bauart bleibt erhalten: mit dem Tag sind die
Wrapper in internal/tools/netcmds.go ausgeblendet, und mit ihnen net/http, TLS
und die ssh-Bibliothek.

Dabei kam ein echter Defekt heraus. Die Policy wurde bisher nur in
tools.Run angewandt. Sobald ein Befehl sein eigenes Paket importiert, ist dieser
Weg umgangen, und die Batterie zeigte es sofort:

    FAIL policy-blocks-network
    FAIL policy-network-message

Das ist die Regel des Projekts, nicht ein Schoenheitsfehler: ein Befehl, der
ohne Policy laeuft, ist der Fehler, den es nicht geben darf.

Behoben mit policy.Enter(), das jeder der 55 cmd/main.go als erste Zeile ruft;
es wendet AKMESH_POLICY einmal pro Prozess an und beendet den Prozess bei
kaputter Policy, wie vorher tools.Run. tools.Run ruft jetzt denselben Eintritt,
statt eine zweite Sperre zu halten. Batterie danach wieder 169/0 Linux und
149/0 Windows.

Groesse: 11817084 Linux, 11847168 Windows.

## Aufteilung Etappe 7: der Rest, und das Ende der Sammelstelle (18.11.2026, acmesh-mq1)

Die letzten Befehle liegen jetzt in eigenen Paketen:

    internal/corecmd    echo, pwd, true, false
    internal/env        env
    internal/file       file
    internal/iconv      iconv
    internal/jq         jq
    internal/sedawk     sed, awk
    internal/diffpatch  diff, patch
    internal/policycmd  policy
    internal/sh         sh, gosh

internal/tools enthaelt danach nur noch die Tabelle (registry.go) und die
Wrapper zu den Paketen; jede Implementierung ist draussen. Einzig cmd/sh zieht
die Tabelle bewusst mit, weil die Shell ihre eingebauten Befehle daraus nimmt.

Nachgemessen, welche cmd-Pakete die Tabelle noch ziehen (go list -deps ./cmd/X,
Zaehlung der Zeilen mit akmesh/internal/tools): echo, true, jq, file, iconv,
env, sed, awk, diff, patch und policy stehen bei 0. Vorher stand jeder bei 1.

Proben mit dem neuen Stand:

    probe (echo true)                       2.547.836 Bytes
    probe (jq file iconv env)               4.116.604 Bytes
    probe (diff patch awk sed)              3.866.748 Bytes
    probe (curl ssh)                        7.155.836 Bytes   (Netz, aus Etappe 6)

Zum Vergleich der Anfang: ein Bau mit genau einem eigenen Befehl wog 10.338.428
Bytes, weil die Tabelle jeden Befehl mitzog.

Volles Binary: 11821180 Linux, 11854848 Windows.
go test ./... gruen, Batterie 169/0 Linux und 149/0 Windows.

## Das kleine Binary als Profil, und das Groessenbudget (18.11.2026, acmesh-94e.11, acmesh-94e.15)

scripts/build-bb.sh kennt jetzt die Profilzeile "keep": alles, was nicht genannt
ist, faellt aus dem Bau, unsere Befehle und die von u-root gleichermassen.
Vorher konnte ein Profil nur ausschliessen, und unsere eigenen Befehle kamen
immer alle mit.

Neues Profil scripts/profiles/ext.txt: kein Netz (nonet), keine Compiler,
ps/kill/killall/uname/hostname/free/df/du/sync/timeout, sh, die Textwerkzeuge,
sha256sum, file, jq, find.

    ./scripts/build-bb.sh linux ext     acmesh-ext     6963324 Bytes (6,6 MiB)
    ./scripts/build-bb.sh windows ext   acmesh-ext.exe 6982144 Bytes (6,7 MiB)
    ./scripts/build-bb.sh linux         akmesh        11821180 Bytes
    ./scripts/build-bb.sh windows       akmesh.exe    11854848 Bytes

Der kleine Bau ist damit 4,9 MB kleiner als der volle, bei 11,8 MB vorher gab es
also gar keinen kleinen Bau.

Geprueft auf dem Ziel: acmesh-ext auf Linux (ps -ef, uname -s, free -m, awk,
sed -n, sha256sum, ls) und acmesh-ext.exe auf Windows (uname -s, ps, awk,
sha256sum, diff). Ein nicht enthaltener Befehl sagt "command is not present in
akmesh: curl" und endet mit Exit 1 - nicht mit 0 und nicht mit 127, das ist die
Antwort des Verteilers von gobusybox.

Groessenbudget im Bau: Warnung ueber 12.000.000 Bytes, harter Fehlschlag ueber
16.000.000, und ein Zuwachs ueber 1 MB gegenueber dem letzten Bau desselben
Ziels steht im Baubericht. Die letzte Groesse liegt in .build/last-size-<GOOS>-
<Profil>, also nicht im Repo. Beide vollen Ziele bauen mit der Pruefung durch,
Batterie 169/0 Linux und 149/0 Windows.

## Linux-Binary bauen und messen (18.11.2026, acmesh-94e.2)

Der Bau laeuft auf beiden Zielen aus demselben Skript, die Zahlen sind von heute:

    ./scripts/build-bb.sh linux     akmesh      11825276 Bytes
    ./scripts/build-bb.sh windows   akmesh.exe  11854848 Bytes
    ./scripts/build-bb.sh linux ext acmesh-ext   6963324 Bytes
    ./scripts/build-bb.sh windows ext acmesh-ext.exe 6982144 Bytes

Befehle im Binary, gezaehlt ueber die Ausgabe des Verteilers:

    Linux    88 Befehle
    Windows  87 Befehle

Der Unterschied ist genau strace, das ptrace braucht und deshalb nur fuer Linux
baut (cmds/core/strace, siehe docs/ecosystem.md). Alle anderen Namen sind auf
beiden Zielen dieselben - geprueft mit comm ueber die sortierten Listen; es gibt
keinen Namen, der nur unter Windows existiert.

Batterie: Linux 169 Pruefungen, Windows 149, beide 0 Fehler.

## tracedeps (18.11.2026, acmesh-n0p)

Ein Befehl, der ein Programm unter ptrace startet und die Dateien ausgibt, die
es geoeffnet oder gestartet hat. Zweck: die Kopierliste fuer eine
distroless-Huelle, ohne eine Spur als Text zu lesen.

    ./akmesh tracedeps /tmp/k.sh        (k.sh: #!/bin/sh; cat /etc/hostname)
    /bin/cat
    /etc/hostname
    /tmp/k.sh

Mit -probes kommen die Pfade dazu, die nur geprueft wurden (PATH-Suche):

    ./akmesh tracedeps -probes /tmp/k.sh
    /bin/cat
    /etc/hostname
    /sbin/cat
    /tmp
    /tmp/k.sh
    /usr/bin/cat
    /usr/local/bin/cat
    /usr/local/sbin/cat
    /usr/sbin/cat

Groesze: 11.837.564 Bytes Linux (vorher 11.825.276, also +12.288),
11.857.408 Bytes Windows (dort ist nur der Stummel drin, der sagt, dass es den
Befehl nur unter Linux gibt). Batterie: Linux 172 Pruefungen, Windows 150,
beide 0 Fehler.

Was beim Bau zu lernen war, jeder Punkt hat einen Lauf gekostet:

- Ohne runtime.LockOSThread gibt es keine ptrace-Ereignisse: der Thread, der
  das Kind gestartet hat, muss es auch weiterfuehren.
- Nach dem ersten Stopp muss einmal PtraceSyscall kommen, sonst laeuft das Kind
  gar nicht erst los und Wait4 wartet ewig.
- fork, vfork und clone melden ein Ereignis statt eines Syscall-Exit-Stopps.
  Wer eine Enter/Exit-Parität fuehrt, muss sie dort zuruecksetzen, sonst ist
  jeder spaetere Syscall um eins verschoben und die Pfade fehlen.
- Robuster als Paritaet: die Argumente bei jedem Syscall-Stopp lesen. Der
  Exit-Stopp wiederholt sie in denselben Registern, Doppelte landen im selben
  Map-Eintrag.
- /proc/<pid>/mem liefert EIO auf Seiten eines frisch geforkten Kindes, also
  genau dort, wo die interessanten Pfade liegen. PTRACE_PEEKDATA geht.
- Der eigene execve des Programms ist nie dabei (das Kind stoppt am
  Exec-Ereignis, nicht an einem Syscall-Paar). Deshalb wird das Programm aus
  der Kommandozeile mit aufgenommen.

## httpd (18.11.2026, acmesh-m5j)

Ein Befehl, der ein Verzeichnis ueber HTTP oder HTTPS ausliefert, mit den
Optionen von busybox httpd.

    ./akmesh httpd -f -p 127.0.0.1:8099 -h /tmp/www
    curl -s -o /dev/null -w '%{http_code}' http://127.0.0.1:8099/           -> 200
    curl -si http://127.0.0.1:8099/a.css | grep -i content-type              -> text/css; charset=utf-8
    curl -s -r 0-3 http://127.0.0.1:8099/index.html                          -> <h1>
    curl -s -o /dev/null -w '%{http_code}' http://127.0.0.1:8099/nix          -> 404
    ./akmesh httpd -p 8098                                                    -> "started in the background, pid N", 200 danach
    ./akmesh httpd -f -p 127.0.0.1:8443 -S c.pem -K k.pem                     -> https 200, Klartext auf demselben Port 400
    ./akmesh httpd -M x.mjs                                                   -> text/javascript; charset=utf-8
    ./akmesh httpd -d 'a%20b'                                                 -> a b
    ./akmesh httpd -e '<a>'                                                   -> &lt;a&gt;

Groesze, gemessen mit measure-size.sh probe:

    probe (base64)          2609276 Bytes
    probe (base64 html2md)  2957436 Bytes   also 348160 fuer html2md
    probe (base64 httpd)    6799484 Bytes   also 4190208 fuer httpd mit TLS

Beides zusammen hebt das Binary von 11.927.676 auf 12.365.948 Bytes Linux und
12.395.520 Bytes Windows. Die Warnlinie im Build steht darum jetzt bei 13 MB,
der harte Stopp bleibt bei 16 MB.

Batterie: Linux 178 Pruefungen, Windows 156, beide 0 Fehler.

Zwei Fallen, die beim Bau Zeit gekostet haben:

- Ein Hintergrundserver, der die Ausgabe des Aufrufers offen haelt, laesst den
  Aufrufer haengen (busybox-Verhalten ohne -f). Der Kindprozess bekommt deshalb
  /dev/null auf allen drei Streams und eine eigene Session.
- Die .exe laeuft unter /init, der PID aus der Shell ist also nicht der Server.
  Beim Aufraeumen muss das Muster den Port nennen, sonst bleibt ein Server auf
  einem geloeschten Verzeichnis stehen und die naechste Pruefung fragt den
  falschen. Ebenso darf der Port nicht fest sein: er kommt aus der PID dieses
  Laufs.

## html2md (18.11.2026)

    ./akmesh html2md /root/rekr/content/music/demo.html
    ### Atmende Seelen
    play: [AtmendeSeelen.mp3](AtmendeSeelen.mp3) (03:57 min, 128kbit/sec)

Ein HTML-Fragment nach Markdown, Regeln von Turndown und
JohannesKaufmann/html-to-markdown. Gegenprobe an derselben Seite: 148 Woerter
vorher und nachher, wie beim Python-Umsetzer in rekr; der Unterschied ist die
Struktur: die Ueberschrift kommt als "### Atmende Seelen" heraus, nicht als
"###", Leerzeile, Text.

Groesze: 348160 Bytes (siehe oben). Batterie: Linux 178, Windows 156, 0 Fehler.

## docx2md (18.11.2026, acmesh-gow Etappe 2)

Ein .docx nach Markdown. Der Leser ist selbst geschrieben: archive/zip und
encoding/xml aus der Standardbibliothek, kein fremdes Paket.

    ./akmesh docx2md "/root/earthartsky.de/AntwortMaxi.docx" > maxi.md
    ./akmesh docx2md document.docx            # mehrere Dateien nacheinander
    ./akmesh docx2md < document.docx          # von der Standardeingabe

Was die Teile tragen, an denen es scheitert, wenn man nur den Text liest:

    Ueberschrift    pStyle "Heading1"/"berschrift1"/"Title"  -> # bis ######
    Liste           numPr + word/numbering.xml (numFmt)      -> "- " oder "1. "
    Tabelle         w:tbl, erste Zeile als Kopf              -> GFM-Tabelle
    Link            w:hyperlink r:id + document.xml.rels     -> [Text](Ziel)
    fett/kursiv     rPr b/i auf dem Lauf                     -> ** / *
    Zeilenumbruch   w:br als Kind von w:p                    -> zwei Leerzeichen + Umbruch

Gemessen an echten Dateien (AntwortMaxi.docx 308 Woerter, 3 ms; Shownotes
dokument): vor dem Fix kamen Zeilenumbrueche als eine Zeile heraus, benachbarte
fette Laeufe ergaben "**fett ****auch fett**", und "11. Juni" wurde zu
"11\. Juni" maskiert. Alle drei sind jetzt Tests:
TestLineBreakBetweenRuns, TestAdjacentBoldRunsMerge,
TestNumbersInTheMiddleAreNotEscaped, dazu TestMarkersAlreadyProtectALine.

Rotprobe: in listItem die Bedingung "format != bullet" mit "&& false"
abgeschaltet -> genau TestNumberedList faellt, TestBulletList bleibt gruen;
zurueckgenommen -> gruen.

Groesze, mit measure-size.sh probe:

    probe (base64)           2609276 Bytes
    probe (base64 docx2md)   3043452 Bytes   also 434176 fuer docx2md
    akmesh                   12472444 Bytes, akmesh.exe 12504064 Bytes

Batterie: Linux 181 Pruefungen, Windows 159, beide 0 Fehler.

Eine Falle, die eine Stunde gekostet hat und nicht im Bau lag: die .exe wird
unter WSL ueber /init gestartet, und ein laufender akmesh.exe-Prozess haelt das
Abbild seines Pfades fest. Nach dem Bau lieferte ./akmesh.exe deshalb weiter den
alten Stand (der Befehl fehlte, obwohl die Datei byte-identisch mit dem guten
Bau war: md5 gleich, eine Kopie unter anderem Namen lief richtig). Erst nach
kill der beiden alten httpd-Prozesse war der neue Befehl da. Wer nach einem Bau
"der Befehl fehlt" sieht, prueft zuerst laufende akmesh.exe: pgrep -af akmesh.exe.

## httpd Hintergrundbetrieb (21.11.2026, acmesh-rds)

Der Fehler: ohne -f startete kein Server, sondern die Meldung

    httpd: exec: "httpd": executable file not found in $PATH     (exit 1)

Ursache: der Verteiler setzt os.Args[0] auf den Befehlsnamen, also "httpd";
exec.Command(os.Args[0]) sucht diesen Namen dann im PATH statt das laufende
Binary zu starten.

Reparatur: das Kind wird mit os.Executable() als Programm und dem Befehlsnamen
als erstem Argument gestartet, also akmesh httpd -f ...; ein eigenstaendig
gebautes cmd/httpd braucht den Namen nicht und bekommt ihn nicht
(detachArgv in internal/httpd/httpd.go).

Beweis am gebauten Binary:

    $ ./akmesh httpd -p 127.0.0.1:18081 -h /tmp/www2
    httpd: started in the background, pid 3999217          exit 0
    $ curl -s -o /dev/null -w '%{http_code}' http://127.0.0.1:18081/     -> 200
    $ kill 3999217                                            -> Port zu, 000

Derselbe Weg in der eigenen Shell und als eigenstaendiges Binary:

    $ ./akmesh sh -c '/root/akmesh/akmesh httpd -p 127.0.0.1:18082 -h /tmp/www2'
    httpd: started in the background, pid ... / curl -> 200
    $ /tmp/hd2 -p 127.0.0.1:18083                            -> pid ..., curl 200

Rotprobe: mit wiederhergestelltem exec.Command(os.Args[0]) kommt genau die
Meldung aus dem Ticket (exit 1, kein Server); zurueckgenommen -> pid und 200.

Drei Go-Tests fuer den reinen Teil (detachArgv): Name im Mehrfachbinary, kein
Name beim eigenstaendigen Binary, kein zweites -f. Batterie: zwei neue
Pruefungen httpd-detach-pid und httpd-detach-serves, damit der Weg ohne -f
nicht wieder still kaputtgehen kann.

## rtf2md und pdf2md (18.11.2026, acmesh-gow Etappe 3 und 4)

Beide lesen selbst, kein fremdes Paket: rtf2md ein Tokenizer ueber die Gruppen
und Steuerwoerter, pdf2md archive/zip-los mit compress/zlib und den Operatoren
des Inhaltsstroms.

    ./akmesh rtf2md story.rtf
    ./akmesh pdf2md bericht.pdf

rtf2md nimmt: \par und Backslash am Zeilenende als Absatz, \line als harten
Umbruch, \tab, \b/\i als Auszeichnung, \'hh als Byte der Codepage, \uN als
Zeichen (Rueckfallzeichen werden geschluckt), Ueberschriften aus \sN und dem
Namen im Stylesheet, Aufzaehlungen als "- ". Font-, Farbtabelle, Info und Bilder
werden uebersprungen. An "Blitz and Donner.rtf": 1575 Woerter, Absaetze, fette
Titelzeile und die krummen Anfuehrungszeichen stimmen.

pdf2md nimmt: Objekte per Scan (auch bei kaputter xref), Flate-Entpackung,
Seiten in der Reihenfolge des Seitenbaums, ToUnicode-Map, WinAnsi sonst, zwei
Byte je Zeichen bei Type0, Zeichenbreiten aus /W und /Widths fuer den Abstand
zwischen Woertern, und Ueberschriften aus der Schriftgroesze relativ zum
Fliesstext (Stufen 1.8, 1.4, 1.15 - derselbe Ansatz wie pdf-inspector, auf dem
anydoc aufbaut).

Gegenprobe gegen pdftotext an echten Dateien (Woerter):

    FlyerRunenVoehringen.pdf           pdftotext 452   pdf2md 461
    TarotCamp/Tarot Teaser.pdf         pdftotext  84   pdf2md  84
    WorldWide+Witchcamp+Grid.pdf       pdftotext 322   pdf2md 372

Die dritte Datei setzt jeden Buchstaben einzeln ueber Td; ohne vollstaendige
Breitentabelle bleibt dort gelegentlich ein Leerzeichen im Wort ("G rid"), der
Text ist aber vollstaendig. Was pdf2md nicht kann und nicht vortaeuscht: eine
gescannte Seite hat keinen Text, ein verschluesseltes PDF wird mit Meldung
abgelehnt, und Tabellen werden nicht erkannt - dafuer braucht es die
Rechteck-Zeichenoperationen und die Ausrichtung der Textstuecke, wie
pdf-inspector es macht. Das ist ein eigenes Ticket.

Groesze, mit measure-size.sh probe:

    probe (base64)           2609276 Bytes
    probe (base64 rtf2md)    2707580 Bytes   also 98304 fuer rtf2md
    probe (base64 pdf2md)    2744444 Bytes   also 135168 fuer pdf2md
    akmesh 12537980, akmesh.exe 12569600 Bytes

Tests: rtf2md elf, pdf2md zwoelf, inklusive Rotproben (NumFmt-Bedingung,
Schriftgroeszen-Ueberschrift).
Batterie: Linux 188 Pruefungen 0 Fehler, Windows 166 0 Fehler.

Eine zweite WSL-Falle, gemessen 18.11.2026: der Pfadcache. Nach einem Bau
lieferte /root/akmesh/akmesh.exe weiter den alten Stand (die neuen Befehle
fehlten), obwohl md5 von akmesh.exe und einer Kopie gleich waren und die Kopie
unter /tmp/a2.exe richtig lief. Umbenennen und touch halfen nicht, eine Kopie
unter neuem Namen im selben Verzeichnis schon. Die Batterie laeuft deshalb auf
einer Kopie mit frischem Pfad, und wer von Hand prueft, kopiert die .exe vorher
unter einen neuen Namen.

## Tabellen aus PDF (18./19.11.2026, acmesh-juj)

Zwei Wege, beide ohne gezeichnete Gitterlinien:

1. Textspalten. Stuecke einer Zeile, die durch eine Luecke groeszer als 0,9 der
   Schriftgroesze getrennt sind, sind Zellen; mindestens zwei Zeilen mit
   gleicher Zellenzahl und Startpunkten innerhalb von 3 pt sind eine Tabelle.
   Der erste Zeilenkopf wird Tabellenkopf, wie in jedem Werkzeug, das ein Agent
   kennt.
2. Zellrechtecke. Viele Kalender und Formulare zeichnen jede Zelle als
   gefuelltes Rechteck. Rechtecke werden mit der aktuellen Transformationsmatrix
   (q/Q/cm) in Seitenkoordinaten gebracht, als Zeilen und Spalten geclustert und
   mit dem Text gefuellt, dessen Mitte in der Zelle liegt. Ohne q/Q/cm passen
   Text und Rechteck nicht zusammen; die Matrix ist der Kern davon.

Ein Lauf mit drei oder mehr Leerzeichen im Text wird getrennt: manche Erzeuger
legen die Spalte in den String statt in die Position.

Gegenprobe an echten Dateien (Zeilen mit "|"):

    EAS_AnmeldeformularTarot25_Jeannette(pe).pdf  11 Tabellenzeilen
      | zum: | Tarot Camp 25 |
      | am: | 14. - 21. Juni 2025 |
    EAS_AnmeldeformularTarot25_Claudia.pdf         4
    FlyerRunenVoehringen.pdf, Tarot Teaser.pdf, Witchcamp-Grid   0

Im Witchcamp-Grid gibt es 22 Rechtecke, aber alle in einer Spalte (x=105,
w=194) - Tagesbalken, kein Zellgitter. Dort bleibt es beim Text, und das ist
richtig.

Noch nicht gelesen: Formular-XObjects (/Subtype /Form) mit eigenem
Inhaltsstrom. In den Anmeldeformularen steht ein Teil der Felder dort, deshalb
fehlen Zeilen wie "Kursgebuehr:" in der Tabelle. Das ist ein eigenes Ticket.
Batterie: Linux 189 Pruefungen 0 Fehler, Windows 167 0 Fehler.
Groesze: pdf2md 2769020 Bytes gegen base64 2609276, akmesh 12562556,
akmesh.exe 12595712.

## Formular-XObjects (19.11.2026, acmesh-39w)

Ein Formular, ein Stempel, eine Kopfzeile ist oft ein Form-XObject: eigener
Inhaltsstrom, eigene Ressourcen, platziert per "Do" unter einer eigenen Matrix.
pdf2md liest es jetzt mit: das /Matrix des XObject wird an die aktuelle Matrix
angehaengt, die Ressourcen des Formulars gewinnen ueber die der Seite, und die
Tiefe ist auf vier Ebenen begrenzt (ein Formular im Formular im Formular).
Der Text des Formulars wird an die Stelle gehaengt, an der es steht, also in
dieselbe Zeile beziehungsweise Tabellenzeile.

Beleg: im EAS_AnmeldeformularTarot25_Jeannette(pe).pdf erscheint jetzt
"Anmeldung" am Kopf, vorher fehlte die Zeile ganz. Rotprobe: der Test
TestTextFromFormXObject war rot, bevor run/doXObject eingebaut war
("Text aus dem Form-XObject fehlt").

Was weiterhin Text statt Tabellenzeile bleibt: "Kursgebuehr: EUR 630,-". Der
Wert steht dort in einem anderen Lauf als das Label und die Spaltenkanten
liegen weiter auseinander als die Gruppentoleranz von 3 pt.

Batterie: Linux 190 Pruefungen 0 Fehler, Windows 168 0 Fehler, neuer Fall
pdf2md-form-xobject (Feldtext aus einem Form-XObject). Groesze: akmesh
12562556, akmesh.exe 12597248, pdf2md 2769020 Bytes.

Der Pfadcache traf wieder zu: nach dem Bau listete ./akmesh.exe pdf2md nicht
auf, eine Kopie (/tmp/acmesh-check.exe) gab "Feldtext" aus, und "strings
akmesh.exe | grep -c pdf2md" fand den Namen zehnmal - die Datei war also neu,
nur das Abbild unter dem Pfad nicht. Auch rm und Neubau halfen nicht. Wer unter
WSL von Hand prueft, kopiert die .exe vorher unter einen neuen Namen; die
Batterie macht das von sich aus.

## md2txt-l (19.11.2026, acmesh-66u, umbenannt am selben Tag)

Der Befehl heiszt seit dem 19.11.2026 md2txt-l: das l steht fuer light, er
bleibt der eigene Parser ohne Abhaengigkeit, waehrend die neuen Schreibziele auf
goldmark gehen (Entscheidung Martin, Ticket acmesh-54s.5). Das Paket heiszt
weiter internal/md2txt, weil der Name innen niemanden stoert.

Markdown nach reinem Text, mit pandoc 3.6.4 als Maszstab. Gemessen, nicht
geraten (pandoc -f markdown-smart -t plain):

    # Titel              -> Titel
    **fett**, *kursiv*   -> fett, kursiv
    `code`               -> code
    [Link](url)          -> Link          (URL entfaellt)
    <https://auto.link>  -> https://auto.link
    ![alt](bild.png)     -> [alt]
    - a / 1. eins        -> "- a" / "1.  eins" (zwei Leerzeichen nach dem Punkt)
      nested             -> zwei Leerzeichen je Ebene
    > Zitat              -> zwei Leerzeichen eingerueckt, Zeilen zusammengefuehrt
    ```code```           -> vier Leerzeichen eingerueckt
    Tabelle              -> Spalten gepolstert auf Breite+2, eine Zeile aus
                            Minuszeichen nach der ersten Zeile
    ---                  -> 72 Minuszeichen
    Absatz               -> Umbruch bei 72 Zeichen

Zwei Eigenheiten, die im Vergleich auffielen und die wir bewusst anders machen
beziehungsweise uebernommen haben:

1. pandoc ersetzt ohne -f markdown-smart " --- " im Absatz durch einen
   Gedankenstrich. Wir ersetzen nichts stillschweigend; die Gegenprobe laeuft
   deshalb gegen markdown-smart.
2. Eine Liste, die direkt auf eine Absatzzeile folgt, ist in pandoc keine Liste,
   sondern Teil des Absatzes. Ohne diese Regel wurde aus "Rules:" plus Liste bei
   uns eine Liste, und die Zeilenzahlen wichen ab; der Absatzsammler bricht
   deshalb nicht mehr bei einem Listenzeichen ab.

Gegenprobe, Byte fuer Byte gegen pandoc:

    md-beispiel.md (Ueberschriften, Listen, Zitat, Code, Tabelle, hr)  identisch
    md-b2.md (lange Absaetze, verschachtelte Listen, harter Umbruch,
              Definitionsliste, Sonderzeichen)                       identisch
    /root/AGENTS.md, akmesh/README.md, roderik/AGENTS.md             identisch
    zwei Tabellenproben (2 und 3 Spalten)                            identisch

Batterie: Linux 194 Pruefungen 0 Fehler, Windows 172 0 Fehler, neue Faelle
md2txt-l-heading, md2txt-l-inline, md2txt-l-table, md2txt-l-wrap.

Groesze: probe (base64 md2txt-l) 2650236 gegen base64 2609276, also 40960 Bytes -
der kleinste der Dokumentbefehle, weil er nur Standardbibliothek und einen
eigenen kleinen Parser braucht. akmesh 12591228, akmesh.exe 12624896.

## Was goldmark kostet (19.11.2026, acmesh-54s.1)

Gemessen, nicht geschaetzt: je ein Probe-Programm unter
/root/.hermes/cache/scratch/goldprobe, CGO_ENABLED=0, linux/amd64, Go 1.25.7,
goldmark v1.7.13 mit GFM-Erweiterung. Basis ist ein Programm, das nur base64
und fmt benutzt, 2260820 Bytes.

    Basis (base64)                                2.260.820
    + goldmark, HTML-Renderer, Textspaziergang    3.824.582   also +1.563.762
    + goldmark-pdf                               15.847.359   also +13.586.539
    + go-epub                                     7.020.683   also +4.759.863
    + eigener docx-Schreiber (zip + xml)          2.677.471   also +416.651
    alles zusammen (html + pdf + epub)           16.279.762   also +14.018.942

Einordnung:

- goldmark selbst kostet rund 1,5 MB und bringt den HTML-Renderer mit; ein
  Klartext-Ausgang ueber den AST ist ein Spaziergang von rund 60 Zeilen, kein
  Paket. Damit waere auch md2html ohne Zusatz gedeckt.
- goldmark-pdf ist der Brocken: 13,6 MB, mehr als das ganze akmesh heute
  (12,6 MB). Ursache ist nicht der Code, sondern die mitgelieferten Schriften:
  im Probe-Binary finden sich 1581 Font-/TTF-Muster und 242 Treffer auf
  DejaVu/Liberation/Noto, go-text/typesetting und go-fpdf bringen sie mit.
- go-epub kostet allein 4,8 MB, zusammen mit PDF aber nur 0,4 MB zusaetzlich,
  weil sich golang.org/x/net und anderes die beiden teilen.
- Der docx-Schreiber kostet gemessen 416 KB fuer zip und xml - im akmesh-Binary
  sind beide schon drin (docx2md liest so), also real rund null.
- Ein Binary mit allen vieren waere rund 26 MB gegen 12,6 MB heute und damit
  ueber der harten Stopplinie von 16 MB in scripts/build-bb.sh. Entweder steigt
  die Linie mit Begruendung, oder md2pdf wird ein eigenes Binary
  (profile- oder build-tag-getrennt), oder es wird auf goldmark-pdf verzichtet.
- Ohne PDF: goldmark + HTML + EPUB + eigener docx-Schreiber waere rund 12,6 + 1,5
  + 4,8 (epub, dann ohne pdf) = rund 18,9 MB - auch schon ueber der Linie.
  Nur goldmark und HTML: rund 14,2 MB, das geht noch.

## Lizenz-Gate (19.11.2026)

`./scripts/check-licences.sh` prueft die Module, die wirklich kompiliert werden
(`go list -deps`), ueber ihre Lizenzdatei im Modulcache:

    ./scripts/check-licences.sh
    check-licences: 8 modules checked, all licences usable     (exit 0)

Rotprobe in /tmp/agplprobe (eigenes Modul mit fumiama/go-docx):

    check-licences: github.com/fumiama/go-docx is on the forbidden list
    check-licences: github.com/fumiama/go-docx is AGPL, which we cannot ship
    check-licences: FAILED                                     (exit 1)

Damit ist akmesh heute frei von AGPL: weder go.mod noch go.sum nennen
fumiama/go-docx, zikani03/goldmark-docx oder unioffice, und der docx-Probebau
unter /root/.hermes/cache/scratch/goldprobe benutzt archive/zip und
encoding/xml aus der Standardbibliothek, kein fremdes docx-Paket.

## goldmark-pdf Fork (19.11.2026, acmesh-54s.2)

Der schlankste brauchbare Fork von goldmark-pdf v0.4.2, gemessen mit
CGO_ENABLED=0, -ldflags="-s -w", Go 1.26.1, gleiche Probe (leeres Programm
1.597.602 Bytes):

    goldmark, Parser ohne Wurzelpaket            2.093.218   +0,50 MB
    goldmark + GFM (+ HTML-Renderer)             2.584.738   +0,99 MB
    goldmark + GFM, wirklich rendern             2.719.906   +1,12 MB
    go-epub allein                               4.141.218   +2,54 MB
    Fork ohne chroma, Google-Schriften gestubbt  8.016.034   +6,42 MB
    Fork ohne chroma                             9.097.378   +7,50 MB
    goldmark-pdf unveraendert                   12.218.530  +10,62 MB

Gegen akmesh heute (12.591.228 Bytes roh) ergibt der magere Fork rund 19,0 MB,
also ueber dem harten Stopp von 16.000.000 (scripts/build-bb.sh:276).

Der Fork baut keine Syntaxfarben mehr, holt keine Schriften ueber HTTP und liest
stattdessen eine TrueType-Datei von der Platte (BuiltinFontPath). Geprueft an
einem Dokument mit Ueberschriften, Tabelle, Zitat, Codeblock und Fussnote:
53.008 Bytes PDF, Umlaute korrekt, Schrift eingebettet mit uni=ja.

Aufbau, Aenderungen und Empfehlung: docs/design/md2pdf-goldmark-fork.md.
Forks im Zwischenlager /root/.hermes/cache/scratch/pdfprobe (forkpdf, fork2,
fork3).

## Groeszenlinie neu gesetzt (19.11.2026, acmesh-54s.1)

Entscheidung Martin: md2pdf kommt auf Basis des schlanken goldmark-pdf-Forks,
und weil sich einzelne Features beim Bau ausschlieszen lassen, darf der Warner
hoeher liegen. In scripts/build-bb.sh steht deshalb:

    Warnlinie  25000000   (vorher 13000000)
    harter Stopp 30000000 (vorher 16000000)

Begruendung aus den Messungen: goldmark mit GFM +1,12 MB, goldmark-pdf ohne
chroma +7,50 MB (mit Google-Fonts-Anbindung; ohne beides +6,42 MB, dann muessen
eigene TTF mitgeliefert werden), go-epub +2,54 MB, eigener docx-Schreiber
+0,42 MB. Mit allen Zielen landet akmesh bei rund 23 MB, also 13 MB als Warner
waere bei jedem Bau ein Fehlalarm. Der Stopp bei 30 MB laeszt Platz, eine
Regression zu bemerken, statt sie mit der Warnlinie zu verwechseln.

Warum der Fork noetig ist: goldmark-pdf v0.4.2 zieht chroma in vier Dateien fest
herein (option.go, renderer_funcs.go samt lexers, style.go, writer.go), und der
Import von chroma/v2/lexers registriert alle Lexer; build-Tags oder ein
Linker-Schalter existieren nicht. Nur die Lexer wiegen allein 5230754 Bytes.

## md2pdf mit dem mageren goldmark-pdf-Fork (19.11.2026)

Aufruf: `md2pdf [-o FILE] [FILE...]`, ohne FILE von stdin, ohne -o nach stdout.
Renderer: goldmark-pdf v0.4.2, gepatcht (docs/design/patches/), angewandt auf den
Modulcache durch scripts/apply-goldmark-pdf-fork.sh, das build-bb.sh und
check-all.sh vor dem Kompilieren aufrufen.

Groesze:

    akmesh vorher (ohne goldmark)                 12.591.228
    akmesh mit md2pdf, Fork ohne chroma           15.077.500   also +2.486.272
    akmesh.exe                                    15.152.640

Der Zuwachs ist kleiner als die Rohmessung der Probe (7,50 MB fuer den Fork),
weil HTTP, TLS und die uebrigen Netzpakete im akmesh ohnehin schon stecken.

Ein Befund, der wichtiger ist als die Zahl: ein Bau ohne den Patch scheitert
jetzt. go mod tidy hat chroma aus dem Modulgraph genommen, weil der gepatchte
Quelltext es nicht mehr importiert; ohne Patch meldet der Compiler

    github.com/stephenafamo/goldmark-pdf@v0.4.2/option.go:10:2:
    could not import github.com/alecthomas/chroma/v2 (no metadata)

Der Fork ist damit nicht optional, sondern Bedingung des Baus - und wer den
Modulcache neu holt, merkt es sofort und nicht erst an der Groesze.

Rundlauf, der eigentliche Beleg:

    printf '# Titel\n\nEin Absatz mit Umlauten: äöüß.\n' | md2pdf -o m-out.pdf m.pdf.md
    pdf2md m-out.pdf   ->  "## Titel", "Ein Absatz mit Umlauten: äöüß."

Batterie: Linux 199 Pruefungen, neue Faelle md2pdf-is-pdf, md2pdf-has-eof,
md2pdf-roundtrip, md2pdf-umlaut, md2pdf-unknown-option.
Go-Tests: sechs in internal/pdfgen, Rotprobe gemacht (ein Leck auf stdout bei -o
wird von TestRunWritesFileAndKeepsStdoutClean gefangen).

## Label-Wert-Paare als Tabellenzeilen (19.11.2026, acmesh-gow)

Die Restschuld war: ein Formular schreibt "Ort: Pumpkin Mart" in einen Lauf, ohne
Zelluecke, und pdf2md liest es als Fliesstext. Zwei Regeln schliessen das:

1. Zeilengruppe mit zwei Zellen, deren erste auf ":" endet: dann muss nur die
   Label-Spalte ausgerichtet sein (3 pt), die Wertspalte darf wandern, weil ein
   Wert so breit ist, wie er eben ist.
2. Label und Wert in einer Zelle ("Komplexitaet: 6"): zwei oder mehr solche
   Zeilen untereinander ergeben eine Tabelle mit zwei Spalten, eine einzelne
   Zeile bleibt Prosa.

Dazu ein Fix, ohne den beides nicht greift: viele Erzeuger setzen jede Zelle als
eigenes Textobjekt auf derselben Grundlinie. Frueher begann bei jedem Textobjekt
eine neue Zeile, Label und Wert landeten untereinander. Jetzt entscheidet die
Grundlinie (Toleranz 0,3 x Schriftgroesse), nicht die Operation.

Belege:

    prüf-PDF mit sechs Paaren (scripts/make-test-pdf-labels.py)
      vorher: sechs Zeilen Fliesstext, nachher:
      | Ort: | Pumpkin Mart |
      | Datum: | 31. Oktober |   ...   (6 Zeilen + Trennzeile = 7)

    echte Dateien, vorher -> nachher (altes und neues Binary verglichen)
      EAS_AnmeldeformularTarot25_Alina.pdf      11 -> 11 Tabellenzeilen, 518 -> 518 Woerter
      EAS_AnmeldeformularTarot25_Celia.pdf       4 ->  4, 326 -> 326
      H_Anmeldung_Paar.pdf                       3 ->  3, 272 -> 272
      DecorativeGoreSeasonHandout_DE_v4.pdf      0 ->  3 (Formularzeilen im Text)
      keine Regression: Wortzahlen unveraendert

Go-Tests: TestLabelValuePairs, TestLabelValueInOneCell, TestSingleColonLineStaysProse,
TestColonProseStaysProse. Rotproben: Label-Regel aus -> rot; Zeilenumbruch wie
frueher -> rot; Paar-Erkennung aus -> rot.
Batterie: neue Faelle pdf2md-labels und pdf2md-labels-rows; Linux 201/0.

## md2docx (19.11.2026, acmesh-54s.3)

Aufruf: `md2docx [-o FILE] [FILE...]`, ohne FILE von stdin, ohne -o nach stdout.
Ein .docx ist ein ZIP aus wenigen XML-Teilen; der Schreiber liegt in
internal/docxgen, benutzt archive/zip und goldmark (schon fuer md2pdf im Binary)
und keine fremde Bibliothek.

Was geschrieben wird: [Content_Types].xml, _rels/.rels, word/document.xml,
word/styles.xml, word/numbering.xml, word/_rels/document.xml.rels.
Abgebildet: Ueberschriften als Word-Stile Heading1 bis Heading6, Absaetze, fett,
kursiv, Inline-Code, Aufzaehlung und nummerierte Liste (verschachtelt bis Ebene
9), Zitat als eingerueckter Absatz, Codeblock zeilenweise in Monospace, Tabellen
als echte Word-Tabellen mit Kopfzeile, Trennlinie als Absatz mit unterer Bordüre.
Nicht abgebildet, bewusst: Bilder (der Alternativtext wird geschrieben),
Fussnoten, Kommentare, Aenderungsverfolgung.

Die Fallen stammen aus dem notker-Projekt, das dasselbe Format byteweise
bearbeitet: ein Teil, der nicht in [Content_Types].xml und in keiner Beziehung
steht, ist fuer Word unsichtbar; ein fehlender Stil macht den Absatz still
unsichtbar-formatiert; eine Liste braucht Absatz, num und abstractNum zusammen;
eine Zelle enthaelt Absaetze, keine Laeufe.

Groesze:

    akmesh vorher (ohne md2docx)      15.077.500
    akmesh mit md2docx                15.130.748   also +53.248

Belege, drei unabhaengige Leser:

    akmesh docx2md (eigener Leser)
      "# Titel", "**fetter**", "- eins", "| Anna | Bremen |" kommen zurueck

    notker (fremdes Programm, gleiches Format)
      notker check /tmp/t.docx     -> "check clean: 6 parts identical"
      notker read                  -> p0 (heading 1) Titel, p2/p3 (List Paragraph)
      notker lists                 -> numId 2 bullet 2 use(s), numId 3 decimal 2 use(s)
      notker check /tmp/t2.docx    -> "check clean: 6 parts identical"
      notker read                  -> heading 1, List Paragraph, Quote, Code

    Word (COM, /mnt/c/temp/word-check-md2docx.ps1)
      paragraphs=11 tables=1 rows=2 firstStyle=Ueberschrift 1 sha256=aa7f0122...
      "contains=True" fuer aeoesz - Word findet die Umlaute selbst
      Ein zweiter Lauf (word-check-umlaut.ps1) prueft den Text in Word, weil die
      Konsole auf dem Rueckweg nach WSL die Umlaute zu "????" macht; das ist die
      Konsole, nicht das Dokument.

Go-Tests: neun in internal/docxgen, alle ueber den Rundlauf durch docxmd.
Rotproben: Heading-Stil auf Normal -> TestHeadings rot; numPr entfernt ->
TestBulletAndNumberedList rot.
Batterie: sechs neue Faelle (md2docx-is-zip, -heading, -bold, -list, -table,
-unknown-option); Linux 207 Pruefungen 0 Fehler.

Zwei Erkenntnisse aus dem Testbau, die im Code stehen:
- Die Kopfzeile einer GFM-Tabelle hat den Knoten-Typ "TableHeader", nicht
  "TableRow"; wer nur TableRow sammelt, verliert die erste Zeile jeder Tabelle.
- In Markdown ist ein nacktes <spitz> ein HTML-Tag, kein Text. Wer spitze
  Klammern als Text will, muss sie maskieren - sonst ist die Ausgabe richtig und
  der Test falsch.

## md2epub und epub2md (19.11.2026, acmesh-54s.4 und acmesh-70y)

Beide Befehle ohne Fremdmodul: md2epub rendert mit goldmark nach HTML, schneidet
an den Ueberschriften der Ebene 1 in Kapitel und schreibt die Teile mit
archive/zip; epub2md liest die Teile und gibt die Kapitel durch htmlmd.

Aufbau eines Buches, das der Schreiber erzeugt:

    mimetype                     "application/epub+zip", ERSTER Eintrag, unkomprimiert
    META-INF/container.xml       verweist auf das Paketdokument
    OEBPS/package.opf            dc:title, dc:language, dc:identifier, manifest, spine
    OEBPS/nav.xhtml              Inhaltsverzeichnis, properties="nav"
    OEBPS/chapter1.xhtml ...     ein Kapitel je Ebene-1-Ueberschrift, sonst eins

Was den Unterschied zwischen einem Zip und einem EPUB macht, steht im Code:
mimetype zuerst und mit Methode Store; jedes Kapitel in der spine (die spine ist
die Lesereihenfolge, Dateinamen sind es nicht); nav ist deklariert und kein
Kapitel. Der Zeitstempel im Paketdokument ist fest (2026-01-01), damit zwei Bauten
desselben Buches byte-gleich bleiben.

Groesze:

    akmesh vorher (ohne epub)    15.130.748
    akmesh mit md2epub+epub2md   15.220.860      also +90.112

Belege:

    eigener Rundlauf  Markdown -> EPUB -> Markdown
      "# Erstes", "| A | B |", Liste und Ueberschriften kommen zurueck

    Zip- und XML-Pruefung (python zipfile + ElementTree)
      erster Eintrag mimetype, Methode 0 (Store), Inhalt "application/epub+zip"
      alle XML-Teile wohlgeformt, spine mit 2 Eintraegen, dc:title vorhanden

    pandoc 3.6.4 als FREMDER Leser (wie notker bei docx)
      pandoc -f epub -t markdown b.epub
      -> "# Eins", "Ein Absatz mit **fett**.", "- punkt", Tabelle als echte
         Tabelle mit Kopfzeile, "# Zwei" - das Buch ist fuer ein fremdes
         Programm lesbar und die Struktur ist erhalten

Nicht geprueft: epubcheck. Java fehlt auf diesem Rechner; Aussagen ueber die
volle EPUB-3-Konformitaet sind damit nicht belegt, nur Struktur, Wohlgeformtheit
und zwei unabhaengige Leser.

Go-Tests: sieben in internal/epubgen, neun in internal/epubmd. Rotproben:
mimetype komprimiert -> rot; Kapitel nicht getrennt -> rot; Kapitel nach
Dateinamen sortiert -> rot; nav nicht uebersprungen -> rot (erst nachdem die
Fixture nav wirklich in die spine stellte, denn vorher konnte der Test nicht
scheitern).

Zwei Fehler, die der Rundlauf aufdeckte:
- Der Leser gab anfangs den Kopf des Kapitels mit aus: XML-Deklaration, Titel und
  saemtliche CSS-Regeln landeten im Buch. Jetzt wird nur der Inhalt zwischen
  <body> und </body> gelesen; der Test prueft seither auf "<?xml", "DOCTYPE" und
  "font-family" im Text.
- goldmark schreibt jede Tabellenzelle in eine eigene Quellzeile; der HTML-Leser
  machte daraus eine Zelle je Zeile mit haengendem " | ". Im Tabellenbereich wird
  der Whitespace zwischen den Tags jetzt entfernt - vor der Uebergabe an htmlmd,
  damit sich das Verhalten von html2md fuer Seiten nicht aendert.

Batterie: sieben neue Faelle (md2epub-is-zip, -mimetype, -mimetype-first,
-roundtrip, epub2md-table, -chapter-count, -refuses-zip); Linux 214 Pruefungen
0 Fehler.

## Umbenennung acmesh -> akmesh (19.11.2026, akmesh-qfp)

Gesprochen wird der Name Ak Mesh. Die Anspielung, die jemand hoeren kann, bleibt
stehen; das Projekt erklaert sie nicht.

Umfang der Umbenennung, gemessen am Baum:

    850 Treffer in 175 Textdateien, 168 davon im ersten Durchgang geaendert
    Modulpfad      module akmesh, damit alle Importe akmesh/internal/...
    Binaernamen    akmesh, akmesh.exe, akmesh-fs, akmesh-ext, akmesh-fs-tinygo
    Umgebungsvar.  AKMESH_POLICY (vorher ACMESH_POLICY)
    Skript         scripts/test-acmesh.sh -> scripts/test-akmesh.sh
    Dokument       docs/design/acmesh.md -> docs/design/akmesh.md
    Verzeichnis    /root/acmesh -> /root/akmesh
    Ticket-Prefix  akmesh (bd config set prefix)

Nicht umbenannt: die bestehenden Ticket-IDs. Sie sind Historie; wer eine
Fundstelle in einem Dokument liest, muss denselben Ticket wiederfinden. Neue
Tickets heissen akmesh-xxx, alte bleiben acmesh-xxx - .beads/issues.jsonl ist von
der Textersetzung ausgenommen.

Zwei Stellen brauchten mehr als Suchen und Ersetzen:

- scripts/build-bb.sh benennt in u-root den Verteilertext "busybox" um. Ein
  bereits gepatchter Checkout traegt den alten Programmnamen, und die Pruefung
  brach deshalb beim zweiten Bau ab ("u-root no longer says ..."). Die Funktion
  akzeptiert jetzt Zielname und mehrere moegliche alte Wortlaute.
- internal/forked/manifest.json haelt sha256-Summen der u-root-Quelldateien, die
  scripts/patch-uroot.sh mit Kommentaren versieht. Der Kommentar enthaelt den
  Programmnamen, also aendert sich die Summe von cmds/core/chmod/chmod.go
  zwangslaeufig. Eine einzige Zeile wurde neu berechnet; alle anderen Summen
  blieben gleich.

Belege nach der Umbenennung:

    build-bb.sh linux    akmesh 15.220.860 Bytes (unveraendert gegen vorher)
    build-bb.sh windows  akmesh.exe 15.301.120 Bytes (unveraendert)
    Batterie Linux       214 Pruefungen, 0 Fehler
    Batterie Windows     192 Pruefungen, 0 Fehler
    Groesse unveraendert, weil nur Namen wandern, kein Code dazukommt

Fund beim Umbau, nicht von ihm verursacht: tests von internal/tools waren rot
(siehe Ticket zum Registry-Test). Nachgewiesen mit einem git worktree auf
5951d37: derselbe Fehlschlag vor der Umbenennung. Mein frueherer Nachweis
zaehlte nur "ok"-Zeilen und verschluckte die FAIL-Zeile - Zaehlen ist kein
Nachweis, wenn nur eine Sorte Treffer gezaehlt wird.

## 2026-11-19 Mutationsprobe mit gremlins

Werkzeug: github.com/go-gremlins/gremlins, mit Go 1.27 gebaut.
Falle: ohne `--timeout-coefficient 50` meldet gremlins jeden Mutanten als
TIMED OUT, obwohl `go test ./internal/sum` in 6 ms durchläuft. Der Koeffizient
wird aus dem Erstlauf abgeleitet und ist bei sehr schnellen Suiten zu klein.
Jeder Lauf hier also:

    gremlins unleash --workers 2 --timeout-coefficient 50 ./internal/<paket>

Ergebnisse vor der Arbeit (getötet / überlebt, Wirkungsgrad):

    shuf        61 / 0    100,00 %
    sort       440 / 0    100,00 %   (114 nicht abgedeckt)
    basename    11 / 1     91,67 %
    shared      10 / 1     90,91 %
    wc          22 / 4     84,62 %
    diffpatch   93 / 19    83,04 %
    tac          9 / 2     81,82 %
    grep        65 / 20    76,47 %
    head        37 / 14    72,55 %
    nl          15 / 6     71,43 %
    dirname      5 / 2     71,43 %
    cut         12 / 5     70,59 %
    sum         27 / 13    67,50 %
    tr          34 / 21    61,82 %
    stat         0 / 0      0,00 %   (113 nicht abgedeckt, Tests lagen in internal/tools)
    truncate     0 / 0      0,00 %   (31 nicht abgedeckt)
    uname        0 / 0      0,00 %   (7 nicht abgedeckt)
    forked/xargs  Abbruch: TestParallelRunsEverything rot (Data Race im Testhelfer)

Nach der Arbeit:

    sum         34 / 8     80,95 %
    stat        29 / 31    48,33 %
    truncate    17 / 8     68,00 %
    uname        4 / 1     80,00 %   (vorher 0 / 5)

Beobachtung zum Werkzeug: gremlins erzeugt nicht jede Falschung. Das von Hand
gedrehte Vorzeichen in humanCompare (sort -h) erzeugt es nicht, deshalb meldet
sort 100 Prozent, obwohl dieser Zweig ungeprueft war.

Batterie unmittelbar nach einem parallelen gremlins-Lauf (check-all zusammen mit
zwei gremlins-Workern): Windows-Batterie 193 Pruefungen, 7 Fehler, alle mit
"WSL ... UtilAcceptVsock:281: accept4 failed 110"; dieselbe Batterie allein
gestartet: 193 Pruefungen, 0 Fehler. Heisst: check-all nicht parallel zu
weiteren Go-Laeufen starten, und ein Fehlschlag dort erst allein nachpruefen,
bevor er als Regression gilt.

    Batterie Linux      215 Pruefungen, 0 Fehler (vorher 214, +1 sort-human)
    Batterie Windows    193 Pruefungen, 0 Fehler (vorher 192, +1 sort-human)

## 2026-11-19 TLS- und HTTP/2-Fingerabdruck unseres Clients

Gemessen, nicht behauptet:

    ./akmesh curl -s --max-time 20 https://tls.peet.ws/api/all

Antwort des Reflektors (Go 1.27, net/http):

    ja3_hash    03117a8ed39ef02427ebbc39f121275c
    ja4         t13d1312h2_f57a46bbacb6_f50d94e863eb
    akamai h2   2:0;4:4194304;5:1048576;6:10485760|1073741824|0|a,m,p,s
    SETTINGS    ENABLE_PUSH=0, INITIAL_WINDOW_SIZE=4194304,
                MAX_FRAME_SIZE=1048576, MAX_HEADER_LIST_SIZE=10485760
    ALPN        h2
    User-Agent  Go-http-client/2.0

Chrome auf dem Desktop sendet (Werte aus den Profiltabellen der
Impersonations-Bibliotheken):

    ja4 core    8daaf6152771 (Chrome/Edge)
    akamai h2   1:65536;2:0;4:6291456;6:262144|15663105|0|m,a,s,p

Unterschied in drei Punkten: die Cipher-Liste, die SETTINGS-Werte samt
Reihenfolge und die Reihenfolge der Pseudo-Header (bei uns a,m,p,s, bei Chrome
m,a,s,p). Dazu der User-Agent, der Go selbst nennt.

Vergleichswerkzeug: curl-impersonate (MIT) patcht curl gegen BoringSSL
(Chrome) oder NSS (Firefox) und aendert die TLS-Erweiterungen und die
HTTP/2-SETTINGS; es ist C mit eigener Bibliothek, also fuer ein Go-Binary nur
als Vorbild brauchbar, nicht als Abhaengigkeit.

## 2026-11-19 uTLS gemessen: Groesse, Fingerabdruck, Integration

Probe: zwei kleine Programme in /root/.hermes/cache/scratch, gebaut mit
CGO_ENABLED=0, -trimpath und -ldflags "-s -w".

    plain (net/http, crypto/tls)            5.623.992 Bytes
    + uTLS v1.8.2, ohne x/net/http2         7.028.920 Bytes   (+1.404.928)
    + uTLS und x/net/http2                  7.749.794 Bytes   (+2.125.802)

Was der Reflektor sieht (tls.peet.ws):

    crypto/tls heute   ja4 t13d1312h2_f57a46bbacb6_ab7e3b40a677
                       akamai 2:0;4:4194304;5:16384;6:10485760|1073741824|0|a,m,p,s
    uTLS Chrome        ja4 t13d1516h2_8daaf6152771_d8a2da3f94cd   (veroeffentlichter Chrome-Kern)
    uTLS Firefox       ja4 t13d1715h2_5b57614c22b0_5c2c66f702b0   (veroeffentlichter Firefox-Kern)
    uTLS Go            ja4 t13d1311h2_f57a46bbacb6_e7c285222651   (nicht identisch mit nativem Go)

Integrationsfalle, gemessen: mit http.Transport und nur DialTLSContext plus
ForceAttemptHTTP2 stimmt der ClientHello (JA4 passt), die Anfrage geht aber als
HTTP/1.1 raus - ALPN sagt h2, gesprochen wird es nicht. Erst mit einem
ausdruecklichen http2.Transport aus x/net/http2 kommt h2 zustande. Der
HTTP/2-Fingerabdruck bleibt dabei Go'scher Standard:

    uTLS Chrome + x/net/http2   http_version h2
                                akamai 2:0;4:4194304;5:16384;6:10485760|1073741824|0|a,m,p,s

Chrome sendet 1:65536;2:0;4:6291456;6:262144|15663105|0|m,a,s,p. Die
Pseudo-Header-Reihenfolge unseres Clients ist a,m,p,s (aus den gesendeten
HEADERS-Frames von peet.ws), Chrome sendet m,a,s,p.

Paketname: uTLS heisst als Paket "tls", also braucht der Import einen Alias,
sonst kollidiert er mit crypto/tls.

Reife (GitHub-API, mit akmesh curl gelesen, 19.11.2026):
- 2.649 Sterne, 410 Forks, 70 offene Issues, nicht archiviert, Lizenz BSD-3-Clause.
- Angelegt 2017, letzter Push 18.11.2026.
- Letztes Release v1.8.2 vom 13.01.2026 (Sicherheitsupdate); auf master liegt
  seither ein Fix ("keep a private key for every classical key share"), der
  noch nicht als Release erschienen ist.

## 2026-11-19 amazon.de: welcher Client wird abgewiesen

Gleiche Anfrage, vier Clients, vier Runden, URL
https://www.amazon.de/s?k=kaffeemaschine, Status aus der Antwort gelesen.

    Runde            1     2     3     4
    curl 8 Vorgabe   503   503   503   503
    curl 8 Chrome-UA 503   503   503   503
    akmesh Vorgabe   503   503   503   503
    akmesh Chrome-UA 200   200   200   200
    uTLS Chrome+GoUA 503   503   503   503
    uTLS Chrome+ChUA 200   200   -     -

Die Erwartung war umgekehrt: nicht unser Client wird abgewiesen, sondern das
echte curl. Die 503 kommt von CloudFront mit 2292 Bytes, unsere eigene 503 hat
2904 Bytes, also antworten zwei verschiedene Regeln.

Isolation in zwei weiteren Runden, jede Zeile zweimal gleich:

    akmesh  Go-UA                    503   2904 B
    akmesh  Chrome-UA                200   1,87 MB
    akmesh  Chrome-UA + Accept */*   200   1,85 MB
    akmesh  curl-UA                  200   586 KB
    akmesh  kein UA + Accept */*     200   913 KB
    curl8   Chrome-UA, Accept weg    200   1,86 MB
    curl8   Go-UA, Accept weg        200   923 KB

Damit ist der entscheidende Hebel der User-Agent: die Zeichenkette
"Go-http-client" wird abgewiesen, jede andere nicht. Der Accept-Kopf
entscheidet beim echten curl mit: mit dem Vorgabe-Accept ist es 503, ohne
ist es 200.

Und der Befund, der die Reihenfolge der Stufen bestimmt: uTLS mit exaktem
Chrome-JA4 und Go-UA bekommt 503, mit Chrome-UA 200 - also genau wie der
gewoehnliche Go-Client. Die TLS-Ebene allein aendert bei Amazon nichts. Der
Inhalt haengt dagegen am UA: mit curl-UA 586 KB, mit Chrome-UA 1,87 MB.

Nachtrag 14:07 Uhr: die 503er dieses Abschnitts sind nicht reproduzierbar und
waren eine CloudFront-Fehlerantwort, siehe Korrektur im Abschnitt "Stufe 1".

## 2026-11-19 Stufe 1: eigener User-Agent statt Go

Vorher (tls.peet.ws): user_agent "Go-http-client/2.0", dazu nur
accept-encoding gzip, kein Accept.

Echtes curl 8.23 im Container sendet: ':method, :scheme, :authority, :path',
dann "user-agent: curl/8.23.0" und "accept: */*" - also genau zwei Kopfzeilen.

Nachher:

    ./akmesh curl -s https://tls.peet.ws/api/all
    user_agent  akmesh/0.1 (curl compatible)
    accept      */*
    ja4         t13d1312h2_f57a46bbacb6_f50d94e863eb   (unveraendert, wie erwartet)

Wirkung, am selben Tag gemessen an amazon.de (Produktsuche Kaffeemaschine):

    akmesh Vorgabe vorher   503 in vier von vier Runden
    akmesh Vorgabe nachher  200 mit echter Seite, 908 und 916 KB in zwei Runden

KORREKTUR, 14:07 Uhr: dieser Zusammenhang haelt nicht. Um 14:07 bekommt
derselbe Client mit -A 'Go-http-client/2.0' ebenfalls 200, mit und ohne
Accept-Kopf, zweimal hintereinander. Die 503er von 13:29 waren auch keine
Bot-Sperre: beide Antworten, die von curl und die von akmesh, tragen dieselbe
ETag "6ae98b03-8f4", dieselbe content-length 2292 und "X-Cache: Error from
cloudfront" - es war eine CloudFront-Fehlerantwort, kein Regelwerk gegen
unseren Client. Der Unterschied 2904 gegen 2292 Bytes ist nur unser -i-Praefix
"< " je Kopfzeile.

Was bleibt, ist etwas anderes: der User-Agent waehlt die Seitenvariante.
Gemessen, je zwei Runden, alles 200:

    curl/8.23.0                      595 KB   "Suchergebnis Auf Amazon.de Fuer: ..."
    akmesh/0.1 (curl compatible)     918 KB   "Amazon.de : kaffeemaschine"
    Go-http-client/2.0               903-1020 KB
    golang                           1,5-1,7 MB

Der eigene Agent bleibt trotzdem richtig, weil Go sonst den Werkzeugnamen
mitschickt und curl seine Kennung auch setzt - aber als Zugangsmittel ist er
nicht belegt.

Bekannte Grenze, im Ticket acmesh-ijz festgehalten: net/http schreibt Kopfzeilen
alphabetisch sortiert, curl in der Reihenfolge, in der sie gesetzt wurden. Die
Statuszeile von -i ist jetzt zeichengleich ("HTTP/2 200 " mit Leerzeichen), die
Kopfzeilenreihenfolge nicht.

Groesse: 15.233.148 Bytes nach -w, -i und User-Agent (vorher 15.220.860).
Batterie: 221 Pruefungen Linux, 0 Fehler.

Windows nach -w, -i und User-Agent (19.11.2026): 199 Pruefungen, 0 Fehler,
akmesh.exe 15.308.288 Bytes. Linux 221/0 im selben Stand, akmesh 15.233.148 Bytes.

## 2026-11-19 Stufe 2: --impersonate mit uTLS

Schalter: --impersonate chrome, edge (auch chromium), firefox, safari, go.
Kein "curl": uTLS baut keine OpenSSL- oder NSS-Handshakes nach, und ein
Schalter, der luegt, waere schlimmer als einer, der fehlt.

Gemessen an tls.peet.ws, mit akmesh nach dem Einbau:

    ohne Flag            ja4 t13d1312h2_f57a46bbacb6_f50d94e863eb   HTTP/2
    --impersonate chrome   ja4 t13d1515h1_8daaf6152771_0a20fe35d3a5   HTTP/1.1
    --impersonate firefox  ja4 t13d1715h1_5b57614c22b0_5c2c66f702b0   HTTP/1.1
    --impersonate safari   ja4 t13d2014h1_a09f3c656075_14788d8d241b   HTTP/1.1

Der Cipher- und Erweiterungsteil ist der veroeffentlichte des jeweiligen
Browsers (Chrome 8daaf6152771, Firefox 5b57614c22b0, Safari a09f3c656075).
Der ALPN-Teil steht auf h1, weil diese Verbindung HTTP/1.1 spricht.

Ohne die Anpassung war es inkonsistent: die erste Fassung meldete
t13d1516h2_8daaf6152771_... also h2 im ALPN, waehrend die Anfrage als
HTTP/1.1 lief - genau die Ungereimtheit, nach der ein Fingerabdruckdienst
sucht. Deshalb wird der Preset jetzt nachgebaut: ALPN auf http/1.1 gekuerzt
und die verschluesselte ALPS-Erweiterung entfernt.

Bewusste Grenzen, im Code und im Ticket:
- Nur HTTP/1.1 unter dem Schalter. Go's HTTP/2-Stack laesst sich nicht an eine
  fremde TLS-Verbindung haengen; der HTTP/2-Abdruck bleibt ausserhalb.
- Mit --proxy zusammen verweigert der Schalter mit Exit 2, weil der
  ClientHello sonst durch einen CONNECT-Tunnel muesste.

Groesse: 16.507.004 Bytes gegen 15.233.148 vorher, also +1.273.856 (1,27 MB).
Windows: akmesh.exe 16.611.840 Bytes (vorher 15.308.288).

Batterie: 222 Pruefungen Linux, 0 Fehler; 200 Pruefungen Windows, 0 Fehler.
Der erste Windows-Lauf meldete einen Fehler (httpd-detach-serves) und der
zweite vier 404er auf /index.html: ein Restprozess aus dem abgebrochenen Lauf
hielt den Port. Nach dem Aufraeumen 200/0. Als acmesh-bvy aufgenommen.

Tests: Profiltabelle mit exakter Zuordnung (eine falsche Zuordnung wurde in
der Rotprobe erkannt), unbekanntes Profil Exit ungleich 0, echter
TLS-Handshake gegen httptest mit -k, und die Proxy-Verweigerung. Die erste
Fassung der Profilpruefung war zu schwach: sie pruefte nur, dass ein Name
akzeptiert wird, nicht welches Profil dahinter steht.

## 2026-11-19 echte Produktsuche bei amazon.de: wer bekommt Treffer

Gezählt wurde der Marker, den Amazon an jeden Treffer haengt
(data-component-type="s-search-result"), nicht die Seitenlaenge.
URL: https://www.amazon.de/s?k=kaffeemaschine, 14:13 bis 14:16 Uhr.

    Client                                     Status  Treffer  Preise  Bytes
    echtes curl 8, Vorgabe                     503     0        0       2292
    echtes curl 8, Chrome-Kopfzeilen           200     22       35      1,86 MB
    akmesh, Vorgabe                            200     16       16      913 KB
    akmesh, Chrome-UA                          200     22       35      1,80 MB
    uTLS Chrome + Chrome-UA                    200     22       35      1,86 MB

Die 503 ist keine Stoerung im Betrieb: der Rumpf ist Amazons eigene Seite
"Tut uns Leid!" (ETag "6ae98b03-8f4", x-cache "Error from cloudfront"). Sie
wird aber nicht zufaellig verteilt - curl bekommt sie fast immer, unser Client
fast nie.

Einzelkopfzeilen reichen nicht, um den Unterschied zu erklaeren. Sieben
curl-Laeufe, je eine Kopfzeile variiert:

    Vorgabe                              503   0 Treffer
    nur Accept: text/html                503   0 Treffer
    nur Accept-Language                  200   0 Treffer (leere Seite)
    Chrome-UA allein                     503   0 Treffer
    Chrome-UA + Accept text/html         503   0 Treffer
    Chrome-UA + Accept */*               503   0 Treffer
    Vorgabe-UA + Accept text/html        503   0 Treffer

Erst das Buendel aus Browser-UA, text/html, Accept-Language, Sec-Fetch-* und
Upgrade-Insecure-Requests brachte curl die volle Liste. Ein einzelner
Ausloeser laesst sich damit nicht benennen; das ist die Grenze dieser Messung.

Fuer die Praxis zaehlt: akmesh bekommt mit seiner Vorgabe eine echte
Trefferliste (16 Treffer) und mit einer Browser-Kennung die volle Liste
(22 Treffer, identisch zu curl mit Browser-Kopfzeilen und zu uTLS).

## 2026-11-19 Zertifikate mit der Standardbibliothek: Machbarkeit belegt

Probe in /root/.hermes/cache/scratch/certprobe, nur stdlib (crypto/x509,
crypto/ecdsa, crypto/ed25519, crypto/tls, encoding/pem), CGO aus, -s -w:

    Groesze der Probe allein   4.583.608 Bytes  (crypto/tls ist komplett drin)

Vier Schritte, alle ausgefuehrt:

1. mkcert-Ersatz: selbstsignierte CA plus Blatt, ECDSA P-256, dazu ein
   ed25519-Zertifikat. Ausgabe ca-cert.pem, ca-key.pem, leaf-cert.pem,
   leaf-key.pem, ed-cert.pem.
2. Auslesen: Subject CN=localhost, Issuer CN=akmesh test CA,O=akmesh,
   Seriennummer 2, ECDSA-SHA256, SANs [localhost akmesh.test], IP 127.0.0.1.
3. Pruefen: x509.Verify gegen die eigene CA mit DNSName localhost - "verifiziert".
4. Echtes Serverzertifikat: tls.Dial zu www.amazon.de:443, Kette drei Glieder,
   Blatt CN=www.amazon.de, Issuer CN=GeoTrust TLS RSA CA G1, OU=www.digicert.com,
   O=DigiCert Inc, C=US, gueltig bis 2027-04-07, acht DNS-Namen; Zwischen-
   zertifikat ausgestellt von DigiCert Global Root G2.

Kosten im akmesh: crypto/tls ist durch curl und wget schon verlinkt, und
crypto/x509 haengt zwingend daran (Zertifikatspruefung). Der Zuwachs durch
einen dieser Befehle ist damit nur der eigene Code, nicht die Krypto.

Grenzen: PEM ja, PKCS#12 und JKS nein; kein OCSP, keine CRL, keine
Certificate Transparency; kein HSM.

Tickets: acmesh-vcl (certinfo), acmesh-pav (mkcert), acmesh-0kh (verify).

## html2md: Einrueckung und gemeinsame Datei (19.11.2026)

Beim Umsetzen von 867 HTML-Seiten (traumwind.de, Kopie unter
/root/.hermes/cache/scratch/corpus) hatte die Ausgabe von html2md auf 137
Seiten mindestens eine Zeile, die mit Tabulator oder vier Leerzeichen beginnt -
in Markdown ein Codeblock (CommonMark 4.4), der Inhalt landet woertlich in
<pre><code>.

Nach dem Entfernen der Einrueckung: noch 6 Seiten mit eingerueckten Zeilen, und
zwar gewollt - drei davon in einem Codeblock (pre-Inhalt behaelt seine
Einrueckung), drei sind verschachtelte Listen, deren Einrueckung die
Listenstruktur ist. Fehlerfaelle: 0.

    python3 - <<'EOF'
    import glob, subprocess, re
    for p in glob.glob("corpus/**/*.html", recursive=True):
        out = subprocess.run(["./akmesh","html2md",p], capture_output=True).stdout
        if re.search(rb"(?m)^(\t|    )\S", out): print(p)
    EOF

Die Datei ist seither identisch mit internal/htmlmd/htmlmd.go in rekr; beide
Projekte halten sie gleich (diff leer).

Tickets: acmesh-4uw.

## 2026-11-19 usage und --help: Bestandsaufnahme

Werkzeug: scripts/help-audit.sh BIN. Es prueft jeden Befehl aus OURS_ALL
(build-bb.sh) und scripts/cmds.txt mit --help und -h, ohne stdin, mit 5 s
Zeitlimit, und notiert Exit-Code, Groesze und erste Zeile.

    Befehle geprueft        98
    mit usage-Text          57
    ohne usage-Text         41
    Exit 0 bei --help       55

Gut: curl, wget, jq, ps, policy, sh, ssh, scp, xargs, chmod, file, shuf,
sort, kill, killall, httpd, tracedeps, gosh und die md2*-Konverter.

Schlechte Faelle, weil sie die Anfrage falsch deuten statt sie abzulehnen:

    hostname --help   "hostname: setting the name is not supported"
    sync --help       "sync: open --help: no such file or directory"
    touch -h          "touch: missing file operand"
    base64 --help     "base64: unknown option \"--\""
    wc --help         "wc: unknown option \"--\""

Nebenbefund: du -h ist human-readable (richtig so), lief aber ueber den ganzen
Baum und wurde vom Zeitlimit abgebrochen; sort -h und df -h, free -h ebenso
richtig belegt. -h ist bei diesen Befehlen also kein Hilfeschalter und darf es
auch nicht werden.

Als acmesh-py8 aufgenommen.

## 2026-11-19 Gegenprobe: haben die Originale -h und --help?

In einem Debian-Container (13.7, 91 Pakete: coreutils, findutils, diffutils,
procps, gawk, patch, tar, bsdextrautils, hostname) fuer jeden der 34
auffaelligen Befehle beide Schalter aufgerufen, ohne stdin.

Ergebnis --help: ALLE 34 antworten mit einem usage-Text und Exit 0
(hostname mit Exit 255, free und hexdump mit einer Leerzeile davor).

Ergebnis -h: Hilfetext nur bei
    awk       -h -> Usage (gawk)
    hostname  -h -> Usage (Exit 255)
Anders belegt, also kein Hilfeschalter:
    df, du, free, sort   -h = human readable
    grep                 -h = no-filename
    tar                  -h = dereference (Exit 2 ohne Aktion)
    sed, tar             -h teils mit usage im Fehlertext, aber Exit ungleich 0
    alle coreutils sonst -h = "invalid option -- 'h'" (cp, head, cut, base64,
                         dirname, nl, pwd, sha*sum, split, stat, sync, tac,
                         timeout, truncate, touch, basename, env)

Folge fuer acmesh: die Regel aus der ersten Fassung war falsch. Richtig ist
- --help ueberall mit usage-Text und Exit 0 - die Originale koennen das alle,
- -h nirgends neu belegen, ausser bei awk und hostname, wo es das Original
  selbst so macht.

Damit ist die Liste der Ausnahmen kleiner als gedacht, und -h bei df, du, free
und sort bleibt human readable, wie es das Original will.

## 2026-11-19 Hilfe-Texte: vorher und nachher (akmesh-vr1, akmesh-py8, akmesh-4qw)

Gemessen mit `./scripts/help-audit.sh`, 98 Befehle, je `--help` und `-h`, stdin zu,
Zeitlimit 5 s. Gezählt wird: usage-Text in der ersten Zeile und Exit-Code.

Vorher (Binary vom 19.11.2026, 14:23):

    50 Befehle: usage-Text, Exit 0
     7 Befehle: usage-Text, Exit ungleich 0
    41 Befehle: gar kein usage-Text

Nachher (Binary vom 19.11.2026, 16:06):

    89 Befehle: usage-Text, Exit 0
     6 Befehle: usage-Text, Exit ungleich 0
     3 Befehle: gar kein usage-Text

Die sechs mit anderem Exit sind ls (2), mv (1), rm (1), mkdir (1), gzip (1) - alle
aus u-root - und false, das wie GNU seinen Hilfetext mit 1 beendet. Ohne usage
bleiben uptime, printenv und man, ebenfalls u-root.

Umsetzung: eine Tabelle in `internal/usage` mit 41 kurzen Texten aus den echten
Parsern und ein gemeinsamer Einstieg (`usage.Run`, `usage.RunIO`, je eine Variante
für Pakete mit mehreren Befehlen), den jeder `cmd/<name>/main.go` benutzt. Vorher
hing die Hilfe an der Befehlstabelle in `internal/tools`; die erreicht das Binary
aber nicht, weil jeder Einstieg sein eigenes Paket aufruft (akmesh-mq1).

`scripts/help-audit.sh --check` ist das Gate: es schlägt fehl, sobald ein Befehl
ohne Hilfe dazukommt. Die acht u-root-Befehle stehen als benannte Liste darin, damit
die Lücke sichtbar bleibt statt in einer großen Zahl zu verschwinden. `check-all.sh`
ruft das Gate nach der Linux-Batterie. Rotprobe: ein Ersatz-Binary, das für wc nur
"wc: unknown option" ausgibt, lässt das Gate mit Exit 1 und der Zeile
"wc(keine usage) wc(exit=1)" abbrechen.

## 2026-11-19 Gegenprobe der Originale, enger gefasst

Im Debian-Container (rust:1-slim-bookworm, coreutils 9.1, mawk, net-tools hostname),
ohne stdin:

    base64 --help    -> Usage, Exit 0
    wc --help        -> Usage, Exit 0
    sync --help      -> Usage, Exit 0
    touch -h         -> "touch: missing file operand", Exit 1
    hostname --help  -> Usage, Exit 255
    hostname -h      -> Usage, Exit 255
    awk -h (mawk)    -> "awk: not an option: -h", Exit 2
    true --help      -> Usage, Exit 0
    false --help     -> Usage, Exit 1
    /usr/bin/echo --help       -> Usage, Exit 0
    /usr/bin/echo --help foo   -> "--help foo", Exit 0
    sha256sum --bogus -> "sha256sum: unrecognized option '--bogus'",
                         "Try 'sha256sum --help' for more information.", Exit 1
    wc --bogus        -> dieselben zwei Zeilen, Exit 1

Damit ist die Notiz "alle Originale enden mit 0" im Abschnitt darüber falsch:
hostname endet mit 255, false mit 1. akmesh bleibt bei 0, außer bei false, das wie
GNU 1 liefert (acmesh-py8 sagt für alles andere Exit 0 zu; die Abweichung zu
hostname ist damit bewusst und steht hier).

`-h` bleibt bei allem außer hostname, was der eigene Parser damit tut: mawk kennt
`-h` nicht als Hilfe, die frühere Messung galt für gawk. touch -h ist bei GNU und
bei uns dieselbe Meldung.

`echo` folgt GNU: `--help` als einziges Argument druckt die Hilfe, mit einem
weiteren Wort wird "--help" ausgegeben (acmesh-0bq, entschieden nach der Regel in
AGENTS.md, dem Original zu folgen).

## 2026-11-19 Größe und Prüfungen der Hilfe-Arbeit

    akmesh    vorher 16.507.004 Bytes, nachher 16.535.676 Bytes  (+28.672)
    akmesh.exe vorher 16.611.840 Bytes, nachher 16.636.928 Bytes (+25.088)
    makebb    linux 2026-11-19 16:06, windows 2026-11-19 16:09
    Batterie  ./scripts/test-akmesh.sh ./akmesh     -> 244 Prüfungen, 0 Fehler
    Batterie  ./scripts/test-akmesh.sh ./akmesh.exe -> 222 Prüfungen, 0 Fehler
              (vorher 222 auf Linux gemessen; die 22 neuen Prüfungen ergeben auf
              Windows 222, also 200 vorher - gerechnet, nicht gemessen, weil das
              alte Binary überschrieben ist)
    Gate      ./scripts/help-audit.sh --check ./akmesh      -> Exit 0
    Gate      ./scripts/help-audit.sh --check ./akmesh.exe  -> Exit 0
              (Windows erlaubt zusätzlich tracedeps: der Befehl verweigert dort
              die Arbeit selbst mit "tracing is built for linux/amd64 only")

Rotproben: `internal/tools/help_test.go` meldet 42 Fehlschläge, sobald der
gemeinsame Einstieg `--help` nicht mehr beantwortet; die Meldung bei unbekannter
Langoption fällt ohne die Reparatur auf `unknown option "--"` zurück (beide
Proben gefahren, Ausgabe gesehen).

## 2026-11-19 find: Batterie-Prüfungen in Go-Tests (acmesh-qz2)

Vorher: sechs Testfunktionen in internal/forked/find, Abdeckung 70,5 Prozent, fünf
Batterie-Prüfungen (find-optional-path, find-dot-prefix, find-maxdepth, find-exec,
find-missing). Nachher: 15 Testfunktionen, Abdeckung 84,8 Prozent, vier der fünf
Prüfungen entfernt.

    GOTOOLCHAIN=auto go test ./internal/forked/find/ -cover
    ok  akmesh/internal/forked/find  0.105s  coverage: 84.8% of statements

Neu geprüft, jede mit Blick auf GNU: -name mit Glob, -path, -regex, -mode, -print0,
-type d, Symlink gemeldet aber nicht verfolgt, Fehler beim Durchlaufen (Startpfad
unter einer Datei) mit Exit 1, und zehn Parser-Fehler mit Exit 2 und Usage-Zeile.

Fund beim Schreiben von TestPathMatchesTheWholePath: `find . -path './sub/*'` fand
nichts, GNU fand ./sub/x.txt. Ursache: die Matcher bekamen den Pfad ohne den "./"-
Präfix, den erst die Ausgabe hinzufügt. Repariert in find_find.go (die Matcher sehen
den Pfad wie er gedruckt wird); Rotprobe: ohne die Zeile fällt der Test.

Mutationsprobe von Hand, weil gremlins unter der aktuellen Last (load 25 bis 28)
nach 25 Minuten noch keinen Mutanten gemeldet hatte und abgebrochen wurde:

    depth >= f.maxDepth -> depth >           TestMaxDepthReportsTheBoundaryDirectory rot
    depth < f.minDepth  -> depth <=          TestMindepth rot
    "d": os.ModeDir     -> "d": 0            erst ÜBERLEBT, nach dem Anziehen rot
    "./"+name -> name                        TestStartPathIsOptional rot

Der überlebende Mutant war ein Fehler im Test, nicht im Code: TestTypeDirectory
prüfte nur die Anzahl der Treffer, und -type d mit Datei-Bits findet ebenfalls drei
Einträge. Jetzt sind die drei Namen festgeschrieben.

Batterie danach:

    ./scripts/test-akmesh.sh ./akmesh      -> 240 Prüfungen, 0 Fehler
    ./scripts/test-akmesh.sh ./akmesh.exe  -> 218 Prüfungen, 0 Fehler
    (vorher 244 und 222; je vier find-Prüfungen entfernt)

Auf Windows lief die Batterie zweimal: der erste Lauf meldete httpd-detach-serves
rot, der zweite grün. Das ist der bekannte Fall aus acmesh-ert (httpd-Port), nicht
die find-Migration.

## 2026-11-19 grep: Batterie-Prüfungen in Go-Tests (acmesh-4aj)

Vorher: sechs Testfunktionen, Abdeckung 60,6 Prozent, sieben Batterie-Prüfungen.
Nachher: 24 Testfunktionen, Abdeckung 83,2 Prozent, alle sieben entfernt.

Drei Abweichungen von GNU grep 3.11, alle beim Schreiben der Tests gefunden und
hier repariert:

    grep -l PAT FILE          druckte gar nichts (GNU: den Dateinamen; bei stdin
                              "(standard input)") - der Name wurde nie ausgegeben
    grep -r PAT .             brach an einem Symlink auf ein Verzeichnis mit
                              "is a directory" ab, Exit 2; GNU folgt bei -r nicht
                              und schweigt, bei -R folgt es
    grep alpha /fehlt         "grep: stat /fehlt: no such file or directory"
                              statt "grep: /fehlt: No such file or directory"

Unbekannte Option meldet jetzt `grep: unrecognized option '--bogus'` plus den
Hinweis auf --help (Exit 2); GNU druckt zusätzlich die Usage-Zeile, das ist die
bewusste, projektweite Form von usage.BadOption.

Jede Erwartung im Test wurde vorher mit GNU grep 3.11 im Container erzeugt
(docker.io/library/rust:1-slim-bookworm):
`grep -A1 -e alpha -e beta` -> alpha, 1, --, beta; `grep -B1 beta` -> 3, beta;
`grep -n -C1` -> "1:alpha", "2-1", "--", "4-3", "5:beta"; nahtlose Fenster ohne
"--"; `grep -l` bei stdin -> "(standard input)".

Mutationsprobe von Hand (gremlins unter Last erneut nicht zum Ende gekommen):

    p.quiet   true -> false            TestQuietPrintsNothingAndKeepsTheStatus rot
    p.wordMatch true -> false          TestExtendedRegexpAndWordMatch rot
    pr.groups > 0 -> > 1               TestContextGroups... rot
    first > pr.last+1 -> first > pr.last  erst ÜBERLEBT, nach TestAdjacentWindows rot
    errText(err) -> err.Error()        TestMissingFileIsTwoWithGNUsWording rot
    if !p.followAll -> if true/false   TestDashRAndDashRDifferOnSymlinks rot (beide)
    -l-Ausgabe entfernt                TestFilesWithMatches rot

Der überlebende Mutant deckte eine fehlende Prüfung auf: zwei Kontextfenster, die
sich berühren, sind eine Gruppe und bekommen kein "--". Test ergänzt, Mutant tot.

Batterie:

    ./scripts/test-akmesh.sh ./akmesh  -> 233 Prüfungen, 0 Fehler

Der erste Lauf meldete vier httpd/curl-Prüfungen rot: ein alter `acmesh httpd`
aus /root/acmesh hielt 127.0.0.1:18080 und beantwortete die Prüfungen mit
/tmp/www2 als Wurzel (404 auf die Testdatei). Nach dem Beenden des Restprozesses
grün. Das ist der Fall aus acmesh-ert, in Zahlen: PID 3999173, Port 18080.

## 2026-11-19 head und wc: Batterie-Prüfungen in Go-Tests (acmesh-795)

Vorher: head 66,7 Prozent Abdeckung (3 Prüfungen in der Batterie), wc 75,7 Prozent
(2 Prüfungen). Nachher: head 93,3 Prozent, wc 93,9 Prozent, alle fünf Prüfungen
entfernt.

    go test ./internal/forked/head/ -cover  ->  93.3%
    go test ./internal/forked/wc/ -cover    ->  93.9%

Vier Abweichungen von GNU head 9.1/9.4, alle beim Testschreiben gefunden und
repariert:

    head -n -2    druckte die ersten ZWEI Zeilen; GNU liest das als "alles ausser
                  den letzten zwei" (auf fünf Zeilen also drei)
    head -c -3    ignorierte das Vorzeichen und gab die ganze Datei aus
    head -n 0     fiel auf die Vorgabe von zehn Zeilen durch und druckte alles,
    head -c 0     dasselbe; GNU druckt nichts
    head -n 1 -c 1 brach mit "can't combine line and byte counts" ab (Exit 2);
                  GNU laesst die letzte der beiden Optionen gewinnen
    -v mit -q     -v gewann immer; GNU laesst die letzte Option gewinnen

wc war beim Zaehlen richtig (-m zaehlt Zeichen, -c Bytes, Summenzeile "total",
--files0-from). Nur die Spaltenbreite weicht ab: GNU richtet Zahlen rechtsbuendig
aus (teils mit fuehrendem Leerzeichen), akmesh trennt mit einem Leerzeichen ohne
Auffuellen.

    GNU    " 5  5 10 five.txt" / " 2  2  4 two.txt" / " 7  7 14 total"
    akmesh "5 5 10 five.txt"    / "2 2 4 two.txt"    / "7 7 14 total"

Die Zahlen sind gleich, `awk '{print $1}'` liest beide Formen richtig. Die
Ausrichtung folgt bei GNU keiner einfachen Regel (je Spalte eigene Breite, dazu
eine Mindestbreite bei mehreren Dateien oder Spalten); nicht nachgebaut, hier
festgehalten. Ein Versuch mit rechtsbuendigen Spalten wurde verworfen, weil er
GNU nur halb traf und den Umbau nicht wert war.

Mutationsprobe von Hand, sieben Mutanten, alle getötet:

    head: Vorzeichen in parseCount verworfen     TestNegativeCount... rot
    head: "count == 0" durchfallen lassen        TestZeroPrintsNothing rot
    head: -q ohne Wirkung                        TestHeadersFollowTheLastOption rot
    head: -c -N-Zweig entfernt                   TestNegativeBytesKeepTheHead rot
    wc:   -m zaehlt Bytes statt Zeichen          TestUmlautsAreCharactersNotBytes rot
    wc:   Summenzeile ab drei Dateien            TestTotalLineForSeveralFiles rot
    wc:   -l setzt words                         TestLinesWordsBytes rot

Batterie: 228 Prüfungen, 0 Fehler Linux; 206 Prüfungen, 0 Fehler Windows.
Ein Lauf dazwischen meldete eine Prüfung rot und war im naechsten Lauf gruen -
derselbe Lastfall wie bei acmesh-ert.

## 2026-11-19 cut und base64: Batterie-Prüfungen in Go-Tests (acmesh-pfm)

Vorher: cut 15,9 Prozent Abdeckung, base64 57,1 Prozent, je zwei Prüfungen in der
Batterie. Nachher: cut 89,2 Prozent, base64 86,3 Prozent, alle vier Prüfungen
entfernt.

    go test ./internal/cut/ -cover             ->  89.2%
    go test ./internal/forked/base64/ -cover   ->  86.3%

cut, fünf Abweichungen von GNU cut 9.4, alle repariert:

    cut -d: -f3,1      gab "c:a" aus; GNU sortiert die Auswahl aufsteigend ("a:c")
    cut -f2,2          gab das Feld zweimal aus; GNU einmal
    cut -f1-2,2-3      gab "a:b:b:c" aus; GNU "a:b:c"
    cut -f-2           wurde abgelehnt; GNU liest "Feld 1 bis 2"
    Zeile ohne Trenner blieb leer statt der ganzen Zeile (GNU druckt sie ganz,
                       ausser mit -s)
    cut -b1-2 auf "äbc" gab vier Bytes statt zwei: der Byte-Modus baute
                       Ein-Zeichen-Strings und kodierte sie neu als UTF-8

Fehlbedienung jetzt in GNU-Form mit Exit 1 statt 2:
"cut: the delimiter must be a single character", "cut: only one list may be
specified", "cut: you must specify a list of bytes, characters, or fields",
"cut: unrecognized option '--bogus'" - jeweils mit "Try 'cut --help' ...".

base64, drei Abweichungen:

    -d auf "aGVsbG8"   verlor den letzten Block ("hel" statt "hello" und
                       "decoding: unexpected EOF"). Der Fehler vom 18.11.: ein
                       letzter Block ohne Auffuellung wurde verworfen. Jetzt wird
                       er dekodiert und GNU-gleich "base64: invalid input" mit
                       Exit 1 gemeldet.
    -d auf "!!!!"      meldete "decoding: illegal base64 data at input byte 0"
                       statt "base64: invalid input"
    --bogus, zwei Dateien, fehlende Datei: GNU-Wortlaut mit --help-Hinweis,
                       Exit 1; vorher eigene Sprueche
    encodeUpstream()   war toter Code aus u-root (nirgends aufgerufen) und wurde
                       entfernt

Mutationsprobe von Hand, alle getötet: cut -s umgedreht, Bereiche nicht
zusammengefasst, Zeile ohne Trenner leer, base64 kurzer Block ohne Meldung, zweites
Argument erlaubt, Umbruchbreite (dieser Mutant laeuft in eine Endlosschleife und
wird vom Test-Timeout gefangen). Zwei Mutanten ueberlebten und sind gleichwertig:
"from := 1" auf 0 (Feld 0 wird ohnehin nie ausgegeben) und ein Umbau
[]byte -> string -> []byte (byte-identisch).

Batterie: 224 Prüfungen, 0 Fehler Linux; 202 Prüfungen, 0 Fehler Windows. Der
erste Windows-Lauf meldete vier Fehler und war im naechsten Lauf gruen - der
bekannte Lastfall aus acmesh-ert.

## 2026-11-19 tr: Batterie-Prüfungen in Go-Tests (acmesh-49y)

Vorher: fünf Testfunktionen, 67,9 Prozent Abdeckung, drei Batterie-Prüfungen.
Nachher: 13 Testfunktionen, 80,5 Prozent, alle drei entfernt.

    go test ./internal/forked/tr/ -cover  ->  80.5%

Zwei Abweichungen von GNU tr 9.4 (LC_ALL=C.UTF-8), beide repariert:

    tr -cd '[:digit:]'   löschte ALLES. Im Komplement-Pfad wurde SET1 als
                         Zeichenkette "[:digit:]" gelesen statt als Klasse, also
                         fiel jedes Zeichen heraus. Jetzt entscheidet die
                         Klassenprädikate, ob ein Zeichen bleibt.
    tr '[:lower:]' '[:upper:]'  machte aus "äöü" ein "ÄÖÜ". GNU lässt Nicht-ASCII
                         unberührt, weil die Klasse in der C-Locale a-z ist.
                         Klassen und die Gross-/Kleinschreibung sind jetzt
                         ASCII-basiert (0-9, A-Z, a-z, ASCII-Satzzeichen).

Dazu: "tr" ohne Argumente und "tr -d a b" drucken jetzt den Hinweis auf --help,
und die Klassenprädikate sind reine ASCII-Bereiche statt unicode.Is*.

Offen und dokumentiert, nicht gebaut: tr arbeitet hier auf Zeichen, GNU auf Bytes.
Bei "tr 'äöü' aou" gibt akmesh "aou", GNU in C.UTF-8 "uouuuu" (es verschiebt
einzelne Bytes). Fuer -c/-C zaehlt akmesh ein Zeichen als eine Einheit, GNU jedes
Byte, deshalb sind die Ausgaben bei mehrbytigen Zeichen verschieden lang. Ein
byte-genauer Umbau waere ein Neuschreiben des u-root-Forks; als Ticket
acmesh-508 festgehalten. Ebenso fehlt die Wiederholungsform [c*3].

Mutationsprobe von Hand, drei Mutanten, alle getötet: Klassenprädikat im
Komplement ignoriert, Klasse wieder auf unicode.IsLower, -s ohne Wirkung.

Batterie: 221 Prüfungen, 0 Fehler Linux; 199 Prüfungen, 0 Fehler Windows.

## 2026-11-19 check-all mit -race (acmesh-vyc)

scripts/check-all.sh faehrt jetzt vor dem einfachen Lauf auch
`go test -race -count=1 ./internal/...`. Gemessen:

    go test -race -count=1 ./internal/...   13,96 s   48 Pakete ok
    go test        -count=1 ./internal/...    5,19 s

Ein voller check-all-Lauf mit dem neuen Schritt, beiden Zielen, beiden Batterien,
Hilfe-Gate und beiden Builds:

    ./scripts/check-all.sh   183,34 s   exit 0
    battery linux            221 Pruefungen, 0 Fehler
    battery windows          199 Pruefungen, 0 Fehler
    help-audit               every command answers --help except the documented u-root gap

Ohne den neuen Schritt waere der Lauf 169,4 s lang (Differenz, nicht separat
gemessen). Der Schritt kostet also rund 14 Sekunden von gut drei Minuten.

Der erste -race-Lauf fand sofort etwas: TestUnknownLongOptionNamesTheWholeToken in
internal/tools war seit dem base64-Umbau rot, weil er die Anfuerungszeichen der
alten Meldung festschrieb (`"` statt GNU-`'`). Der Test prueft jetzt den Token
selbst und beide Verbote der alten Kurzform. Lehre: nach einem Umbau, der eine
Meldung aendert, `go test ./...` laufen lassen, nicht nur das Paket.

check-all bleibt einzeln: ein paralleler Go-Lauf laesst WSL die .exe mit
"accept4 failed 110" nicht starten, und die Windows-Batterie meldet dann Fehler,
die es nicht gibt (Kommentar im Skript).

## 2026-11-19 Batterie nach der Migration (acmesh-j4f)

Stand am Ende der Migration, gemessen mit dem gebauten Binary auf beiden Zielen:

    ./scripts/test-akmesh.sh ./akmesh      214 Pruefungen, 0 Fehler
    ./scripts/test-akmesh.sh ./akmesh.exe  192 Pruefungen, 0 Fehler

Vor der Migration: 244 und 222. Entfernt sind 30 Pruefungen, jede mit dem Go-Test,
der sie ersetzt (docs/batterie-migration.md, Spalte "ersetzt durch"):

    7  find     5 -> Go
    7  grep     7 -> Go
    5  head/wc  3 und 2 -> Go
    4  cut/base64 2 und 2 -> Go
    3  tr       3 -> Go
    1  truncate TestShrinkAndGrow
    2  split    TestLinesSplitIntoParts, TestTooManyPartsIsRefused
    4  stat     TestCountedFieldsAndOperandErrors (stat-mode bleibt: Rechte auf dem Ziel)

Was bleibt und warum (Zahlen je Prefix in docs/batterie-migration.md):
Windows-Verhalten und Rechte (stat-mode, chmod, cp -a, CRLF), Prozessverhalten
(kill, killall, httpd --detach), Policy im Unterprozess (18), Netz (curl 10,
ssh/scp), Groeszenbudget und Profile (fs, ext), dazu die Stichprobe von zehn
Befehlen der Hilfe-Schleife, weil sie das gebaute Binary prueft und nicht den
Go-Code.

Was noch als Go-Test ginge, aber keinen hat (zweite Runde, Ticket acmesh-qiz):
cp 5, diff 1, patch 2, tar 10. Diese Befehle haben heute keinen einzigen Go-Test;
die Batterie ist dort die einzige Deckung.

Abschlusslauf der ganzen Reihe mit dem fertigen Stand:

    ./scripts/check-all.sh   182,78 s   exit 0
    battery linux            214 Pruefungen, 0 Fehler
    battery windows          192 Pruefungen, 0 Fehler
    go test -race            ohne Befund

## 2026-11-19 tr: Entscheidung Zeichen statt Bytes und [c*n] (acmesh-508)

Entschieden mit Martin: tr bleibt zeichenweise. Gemessen im Container, GNU tr 9.4
mit LC_ALL=C.UTF-8:

    printf 'äöü\n' | tr 'äöü' aou     GNU: uouuuu   akmesh: aou
    printf 'abc'   | tr -C a x        GNU: axx      akmesh: axx

Der erste Fall ist die bewusste Abweichung: GNU verschiebt einzelne Bytes und
zerlegt damit den Umlaut, akmesh behandelt ihn als ein Zeichen. Die Begruendung
steht in docs/design/akmesh.md unter "Entscheidungen".

Gebaut wurde die fehlende Form [c*n] in SET2, gegen GNU gemessen:

    tr a '[b*3]'      aaa -> bbb
    tr abc '[x*2]y'   abc -> xxy
    tr a '[b*0]c'     aaa -> ccc     (Zaehler 0 heisst null Kopien)
    tr a '[b*]c'      aaa -> ccc     (leerer Zaehler ist 0)
    tr a '[b*2]c'     aaa -> bbb
    tr ab '[x*2]'     abab -> xxxx
    tr a '[*3]'       aaa -> [[[     (kein Zeichen vor dem Stern: woertlich)
    tr abc '[a-c*2]'  abc -> [ab     (Bereich im Konstrukt: woertlich)
    tr a '[b*x]'      Fehler "invalid repeat count 'x' in [c*n] construct", Exit 1

Abdeckung tr: 82,4 Prozent, zwei neue Testfunktionen.

## 2026-11-19 cp, tar, diff und patch in Go (acmesh-qiz)

Diese vier hatten keinen einzigen Go-Test; die Batterie war ihre einzige Deckung.
Die Tests laufen jetzt durch den Dispatcher (RunArgs) mit echtem Arbeitsverzeichnis
und stdin, also genau so, wie das Binary sie aufruft:
internal/tools/files_test.go, Helfer runFileCommand.

    go test ./internal/tools/ -run 'TestCp|TestTar|TestDiff'   -> alle gruen

Abgedeckt, je mit der alten Pruefung:
    cp -a, verschachtelt      TestCpArchiveCopiesTheTree     cp-archive, cp-archive-nested
    cp -t                     TestCpTargetDirectory          cp-target-dir, cp-target-files
    cp -n                     TestCpNoClobber                cp-no-clobber
    tar -czf/-tzf/-tf         TestTarCreateAndList           tar-create-gzip, tar-list-gzip, tar-list-without-z
    tar --exclude             TestTarExclude                 tar-exclude, tar-exclude-worked
    tar -xzf -C --strip       TestTarExtractStripComponents  tar-extract-strip, tar-strip-file
    tar -cf -C                TestTarCreateFromDirectory     tar-C-create, tar-C-listing
    tar -cjf                  TestTarRefusesBzip2            tar-j-refused
    diff und patch            TestDiffAndPatchRoundTrip      diff-header, patch-apply, patch-result

In der Batterie bleibt cp-archive-mode: die Rechte nach "cp -a" gelten nur auf dem
echten Ziel.

Mutationsprobe von Hand, zwei Mutanten, beide getötet: tar --exclude ignoriert,
cp -n wirkungslos.

Batterie: 196 Pruefungen Linux, 174 Windows, beide 0 Fehler (vorher 214 und 192).
Ein Linux-Lauf meldete httpd-file und drei curl-Pruefungen rot, der naechste war
gruen - derselbe Aussetzer wie in acmesh-ert, jetzt auch auf Linux gesehen.

## 2026-11-19 acmesh-ert: fremder Server auf dem Port war die Ursache

Der Aussetzer in der Batterie (httpd-file, httpd-404, die curl-Pruefungen und
zuletzt httpd-detach-serves) hatte eine nachweisbare Ursache: der Pruefport war
belegt, und die Warteschleife brach bei JEDER Antwort ab, auch bei der eines
fremden Servers. Der fremde Server antwortet dann mit seinem Inhalt, und alle
folgenden Pruefungen fallen.

Nachweis, mit einem fremden Server (python3 -m http.server) auf 18777 und dem
Pruefport fest auf 18777 gesetzt:

    ohne Waechter   httpd-file, curl-writeout-size, curl-include-status rot  (196 Pruefungen, 3 Fehler)
    mit Waechter    196 Pruefungen, 0 Fehler

Die Batterie sucht den Port jetzt: solange etwas auf 127.0.0.1:PORT antwortet,
geht sie in Siebenerschritten weiter (maximal 20 Versuche), fuer den Vordergrund-
und den abgesetzten Fall. Die Warteschleifen warten ausserdem auf UNSERE Seite
(grep auf "hd") statt auf irgendeine Antwort, und nennen Port und PID, wenn nach
20 Sekunden nichts kommt.

Danach drei Windows-Laeufe hintereinander 174/0 und ein Linux-Lauf 196/0.

## 2026-11-19 winmode: POSIX-Rechte unter Windows (acmesh-m6x)

Bisher war chmod unter Windows ein stiller Erfolg ohne Wirkung: jeder Modus gab
Exit 0, und stat/ls zeigten weiter 666 bzw. rw-rw-rw-.

Uebernommen wurde das Paket winmode der Gabel qiangli/sh (Copyright Daniel
Marti, BSD-3 wie mvdan/sh). Es ist eigenstaendig, 627 Zeilen Produktionscode
(273 gemeinsam, 319 Windows, 35 andere Ziele) plus 407 Zeilen Tests, und braucht
nur golang.org/x/sys/windows, das ohnehin im Projekt liegt. Kein 25,2-MB-Fork.

Das Paket legt einen Modus als ACE-Folge in der DACL ab (deny NULL-SID als
Marker, dann owner, group, Everyone) und liest ihn zurueck. Verkabelt:
chmod schreibt ihn nach os.Chmod, stat wickelt jede FileInfo durch
winmode.Apply, damit %a und %A den aufgezeichneten Modus zeigen.

Gemessen mit akmesh.exe auf einem echten NTFS-Pfad (Windows-Temp), 19.11.2026:

    frische Datei ohne chmod    stat -c %a -> 666        (kein Marker, wie Cygwin)
    chmod 755                   stat -c %a -> 755, %A -> -rwxr-xr-x
    chmod 600 / 640 / 700 / 444 600 / 640 / 700 / 444, %A entsprechend
    chmod +x auf 666            -> -rwxrwxrwx (666 | 111), GNU-Regel
    chmod 644                   -> 644

Auf dem WSL-Dateisystem (\\wsl.localhost) scheitert das Setzen: chmod gibt
"chmod: Unzulaessige Funktion." mit Exit 1 statt vorher still 0. Das ist bewusst:
lieber ein ehrlicher Fehler als ein Erfolg ohne Wirkung (Entscheidung im Design).
Wer auf einem Share chmod braucht, bekommt jetzt eine Meldung statt Lug und Trug.

Offen: ls -l zeigt weiter rw-rw-rw-, weil ls ein u-root-Befehl ohne Fork ist
(scripts/patch-uroot.sh). stat ist der Leser, den Skripte benutzen.

## 2026-11-19 ps: Kommandozeile unter Windows (acmesh-94e.22)

Die Kommandozeile steht nicht in der Toolhelp32-Tabelle, aus der ps seine Zeilen
holt. Gelesen wird sie jetzt ueber ntdll: ProcessCommandLineInformation (Klasse 60)
kopiert die Zeile in unseren eigenen Puffer. Das braucht nur
PROCESS_QUERY_LIMITED_INFORMATION, also dasselbe Recht, das ps schon fuer die
Zeiten oeffnet - kein Fremdspeicher, kein PEB, kein Absturzrisiko bei falschen
Zeigern. 120 Zeilen statt der im Ticket geschaetzten 150 bis 250.

Gemessen mit akmesh.exe, 19.11.2026:

    ./akmesh.exe ps -ef
      BUSYS\Spernau 37400 21128 ?  00:06 ?  00:00 cmd.exe /c "ping -n 40 127.0.0.1 >nul"
      BUSYS\Spernau 36216 37400 ?  00:06 ?  00:00 ping  -n 40 127.0.0.1
    tasklist /FI "IMAGENAME eq PING.EXE"  -> PING.EXE 36216, also dieselbe PID

    ./akmesh.exe ps -eo pid,cmd    -> Kopf und Zeilen mit Argumenten, z.B.
      TPOSD.EXE /UEFI\\.\pipe\{C6A9690C-...}OnScreenDisplay

Laufzeit fuer 375 Prozesse: 0,62 s und 0,74 s in zwei Laeufen. Jeder Prozess
kostet ein OpenProcess und zwei Aufrufe von NtQueryInformationProcess.

Wo der Aufruf scheitert (geschuetzte Prozesse, aeltere Systeme ohne Klasse 60,
leere Zeile), bleibt die Spalte beim Executable-Namen: ps zeigt dann wie bisher.

Dazu: killall verglich die Kommandozeile mit firstWord, das die Anfuehrungszeichen
von Windows ("C:\...\cmd.exe" /c ...) mitnahm. Jetzt werden sie abgeschnitten,
Test in TestMatchesName.

Abschlusslauf nach acmesh-m6x und acmesh-94e.22:

    ./scripts/check-all.sh   188,66 s   exit 0
    battery linux            196 Pruefungen, 0 Fehler
    battery windows          174 Pruefungen, 0 Fehler
    help-audit               every command answers --help except the documented u-root gap
    go test -race            ohne Befund

## 2026-11-19 drei Zertifikatsbefehle: certinfo, mkcert, verify (acmesh-vcl, acmesh-pav, acmesh-0kh)

Alle drei nur mit der Standardbibliothek (crypto/x509, crypto/tls, encoding/pem),
also nach docs/groesse.md Regel 4 fast gratis. Zusammen mit ihrer Verdrahtung
kostet das Linux-Binary 16.662.652 B statt 16.543.868 B (+119 KB), Windows
16.790.528 B statt 16.671.360 B (+119 KB).

### certinfo DATEI | HOST[:PORT] (acmesh-vcl)

Datei: alle PEM-Bloecke in Reihenfolge, DER (.cer aus Windows) als Rueckfall.
Host: die Kette, die der Server schickt, Blatt zuerst, TLS ohne Pruefung, damit
auch abgelaufene und selbstsignierte Hosts lesbar sind. --json/-j fuer
Maschinenlesbarkeit.

Gegenprobe mit openssl am selben Server, 19.11.2026:

    openssl s_client -connect www.amazon.de:443  |  akmesh certinfo www.amazon.de
    subject=CN=www.amazon.de                      Subject: CN=www.amazon.de
    issuer=C=US, O=Amazon, CN=Amazon RSA 2048 M04 Issuer: CN=Amazon RSA 2048 M04,O=Amazon,C=US
    notBefore=Oct 30 00:00:00 2026 GMT            Validity: 2026-10-30T00:00:00Z ...
    serial=01D42E38...                            Serial: 1D42E38... (ohne Vorzeichen-Null)

Coverage 87,7 %, sechs Testfunktionen, darunter ein httptest-TLS-Server.

### mkcert -host ... [-ca BASE] (acmesh-pav)

Sieben Testfunktionen, Coverage 79,1 %. Geprueft: Dateinamen, Rechte, SANs (DNS
und IP getrennt), Gueltigkeit, alle drei Schluesselarten ueber x509.Verify gegen
die im selben Lauf erzeugte CA, Wiederverwendung der CA, alle Fehlerfaelle mit
Text und Exit 1.

Live gemessen (Linux, 19.11.2026):

    ./akmesh mkcert -host localhost,127.0.0.1 -ca testca -out leaf
      wrote leaf-cert.pem / wrote leaf-key.pem / signed by testca-cert.pem
    ls -l: leaf-key.pem -rw------- , testca-key.pem -rw-------, Zertifikate 644

Unter Windows schrieb os.Chmod nur das Read-only-Bit, die Schluessel waren 666.
Jetzt setzt writePEM den Modus ueber winmode.SetQuiet (dieselbe Regel wie chmod,
acmesh-m6x). Gemessen auf echtem NTFS, Windows-Temp, Probe danach geloescht:

    akmesh.exe stat -c %a  ->  600 ca-key.pem, 644 ca-cert.pem,
                               600 leaf-key.pem, 644 leaf-cert.pem
    icacls leaf-key.pem    ->  Eigentuemer (R,W,D,WDAC,WO), Gruppe und Jeder
                               nur (Rc,S,RA), also Lesen der Rechte, nicht der Daten

Rotproben, beide getoetet: SANs weggelassen (TestNamesFilesAndKeyMode sah "[]"),
Schluessel 0644 (Mode-Test erwartet 600).

### verify [-ca CA] [-host NAME] CERT [KETTE...] (acmesh-0kh)

Exit 0 nur bei guter Kette, 1 bei fachlichem Fehlschlag, 2 bei Fehlbedienung.
Sechs Testfunktionen. Gemessen Linux und Windows:

    verify -ca testca-cert.pem -host localhost leaf-cert.pem  -> OK, 0
    verify -ca testca-cert.pem -host falsch.test ...          -> "x509: certificate is valid for localhost, not falsch.test", 1
    verify -host localhost leaf-cert.pem (Systemwurzeln)      -> "x509: certificate signed by unknown authority", 1

Rotprobe: die Pruefung selbst entfernt, dann fallen Expired, WrongHost,
UnknownAuthority und ChainFile.

### Batterie: die drei zusammen

Neuer Block am Ende der Batterie: mkcert erzeugt CA und Blatt, verify prueft
gegen die CA, akmesh httpd liefert dasselbe Blatt ueber TLS, certinfo liest die
Kette vom laufenden Server. Vier Pruefungen: mkcert-exit, verify-ca-ok,
verify-wrong-host, certinfo-httpd-tls. Gegenprobe von aussen: curl mit
--cacert testca-cert.pem gegen denselben TLS-Server gibt HTTP 200.

Batterie danach: 200 Pruefungen Linux, 178 Windows, beide 0 Fehler.

## 2026-11-19 ls neu geschrieben statt geforkt (acmesh-wx6)

Gemessen vorher, GNU ls gegen das u-root-ls in einem Testverzeichnis. Acht Flags
wurden abgelehnt (Exit 2, "flag provided but not defined"): -1, -t, -r, -i, -n,
-A, -u, -c. Und die Ausgabe war an vier Stellen falsch: kein total-Kopf, das Jahr
fehlte bei alten Dateien (2020 wurde zu "Jan 1 19:55", also die Uhrzeit des Laufs),
Symlinks begannen mit grossem L statt l, -h schrieb "2.1 MB" statt "2.0M". Dazu
-R ohne Kopf je Verzeichnis und -Q mit -F ohne Indikator.

Entscheidung Martin: forken oder re-implementieren, Ziel ist, dem Agenten zu geben,
was er erwartet. Gewaehlt: internal/ls mit der Standardbibliothek (docs/groesse.md
Regel 4). Ein Fork haette pkg/ls (291 Zeilen) und unixflag (83) mitgebracht und
die vier Fehler geerbt.

Gegenprobe danach, dieselben Aufrufe, GNU gegen akmesh, 19.11.2026:
    ohne Flag, -1, -a, -A, -r, -S, -d, -F, -Q, -i, -R, -t   Zeichen fuer Zeichen gleich
    -l    die Felder sind gleich (awk-Vergleich ohne Unterschied), nur die
          Spaltenbreiten sind enger: GNU polstert auf eigene Mindestbreiten.
          Wie bei wc als Kosmetik vermerkt, die Werte sind identisch.
    -lh   total 16K und 2.0M wie GNU.

Windows gemessen (akmesh.exe, NTFS im Windows-Temp, Probe geloescht):
    chmod 750 neu.txt ; ls -l  ->  -rwx-x--- mit den Rechten aus der DACL,
                                   also der Modus, den acmesh-m6x aufzeichnet
    ls -t                      ->  bei gleicher Sekunde entscheidet der Name,
                                   wie GNU es auf groben Zeitstempeln tut

Tests: 14 Testfunktionen in internal/ls, darunter die Datumsregel (alt mit Jahr,
neu mit Uhrzeit), total-Kopf, -a gegen -A, -tr, -QF, -R mit "./sub:"-Kopf,
unbekannte Option Exit 2. Die Sortierung vergleicht ganze Sekunden, weil zwei
Dateien in derselben Sekunde sonst in Nanosekunden-Reihenfolge stehen und GNU das
nicht tut.

ls ist aus scripts/cmds.txt gestrichen und aus scripts/patch-uroot.sh entfernt;
die Hilfe-Luecke in scripts/help-audit.sh schrumpft auf mv rm mkdir gzip uptime
printenv man. Batterie: ls-sort-time prueft jetzt die echte Sortierung (vorher
pinnte sie das Ablehnen), ls-long-has-year ist neu.

Batterie danach: 201 Pruefungen Linux, 179 Windows, beide 0 Fehler.

Abschlusslauf nach dem ls-Umbau (acmesh-wx6), 19.11.2026:

    ./scripts/check-all.sh   189,63 s   exit 0
    battery linux            201 Pruefungen, 0 Fehler
    battery windows          179 Pruefungen, 0 Fehler
    help-audit               every command answers --help except the documented u-root gap
    go test -race            ohne Befund

## 2026-11-19 Absetzen in den Hintergrund: welche Befehle, und stimmt es (acmesh-wx6 nachfolgend)

Bestandsaufnahme, nicht geraten, sondern im Quelltext und am Binary geprueft:

  httpd     setzt sich per Vorgabe ab; -f bleibt im Vordergrund.
            internal/httpd/detach_unix.go legt eine eigene Session an (Setsid),
            detach_windows.go ein DETACHED_PROCESS ohne eigenes Konsolenfenster.
  sshd      gibt es bei uns NICHT. u-root hat cmds/core/sshd (nur Linux, nutzt
            vsock, sftp, PTY), es steht in keinem Build-Listen-Eintrag, und
            weder scripts/cmds.txt noch internal/tools kennen den Namen. Was wir
            haben, ist der Client: ssh und scp.
  alles     kein weiterer Befehl setzt sich ab: kein Setsid in unseren Paketen
  andere    und keines in den u-root-Befehlen, die wir bauen. sh, timeout, xargs
            und find -exec starten Kinder, aber im Vordergrund.

Gemessen mit akmesh httpd ohne -f, Linux und Windows, 19.11.2026:

  Ausgabe      "httpd: started in the background, pid N", Exit 0
  Dauer        Linux 0 ms bis zur Rueckkehr an die Shell
  Linux        PPID 7 (an init abgegeben), SID = eigene PID, also eigene Session
  Linux        Seite danach abrufbar, SIGTERM beendet den Prozess
  Windows      tasklist zeigt akmesh.exe mit der genannten PID, Seite abrufbar
  Windows      Stop nur ueber unseren eigenen kill: "akmesh.exe kill 39448" gibt
               0, danach 000 und tasklist leer. Die Shell von WSL kann eine
               Windows-PID nicht erreichen - das ist der Unterschied, der in der
               Batterie gefehlt hat.

Gefunden und behoben: sieben verwaiste httpd aus frueheren Batterie-Laeufen
(PPID 7, Ports 19255 bis 19451 belegt). Ursache war der TLS-Block der Batterie,
der den Server ueber die Shell-Funktion run startete: $! war die Subshell, nicht
der Server, und kill traf die falsche PID. Jetzt startet der Block das Binary
direkt, beendet zusaetzlich per pkill auf den Port und prueft danach.

Neue Pruefungen in der Batterie: httpd-detach-stopped und tls-httpd-stopped
(warten bis zu 10 s auf 000, weil der .exe durch die Interop-Schicht einen Moment
braucht), und der Stop des abgesetzten Servers laeuft ueber "$BIN" kill, weil
die Shell-PID dort eine Windows-PID ist.

Batterie danach: 203 Pruefungen Linux, 181 Windows, beide 0 Fehler, keine
Restprozesse.

## 2026-11-19 Lokale CA mit Namensbeschraenkung (acmesh-816, Schritt 1)

mkcert kann jetzt eine CA mit Namensbeschraenkung erzeugen:

    akmesh mkcert -host test1.local,test2.local -ca ./local -ca-domains .local -out ./leaf

Gemessen am gebauten Binary:

    openssl x509 -in leaf-cert.pem -ext subjectAltName
        DNS:test1.local, DNS:test2.local
    openssl x509 -in local-cert.pem -ext nameConstraints
        Excluded: IP:0.0.0.0/0, IP:::0/0   (mit Beschraenkung sind alle IPs ausgeschlossen)
    akmesh verify -ca local-cert.pem -host test1.local leaf-cert.pem
        OK: test1.local chains to a trusted root, Exit 0
    akmesh verify -ca local-cert.pem -host test1.example.com leaf-cert.pem
        x509: certificate is valid for test1.local, test2.local, not test1.example.com, Exit 1

Der eigentliche Beweis steht im Go-Test TestCADomainsConstrainTheCA: ein Blatt,
das die CA selbst fuer example.com bzw. fuer 127.0.0.1 ausgestellt hat, verkettet
nicht bis zur Wurzel - die Beschraenkung greift auch dann, wenn jemand falsch
signiert. Dazu TestCADomainsRefusals: ein Suffix ohne Punkt, -ca-domains ohne -ca
und eine vorhandene CA ohne Beschraenkung werden abgelehnt statt still passend
gemacht.

Rotprobe: mit entfernter PermittedDNSDomains-Zuweisung faellt der Test mit
"CA permits [], want [.local]"; nach dem Zurueckspielen gruen.

## 2026-11-19 Wurzel installieren und test1.local per HTTPS (acmesh-816, Schritt 2)

    akmesh mkcert -host test1.local -ca ./local -ca-domains .local -out ./leaf
    akmesh mkcert -install -ca ./local
    akmesh httpd -f -p 127.0.0.1:18961 -h . -S leaf-cert.pem -K leaf-key.pem

Gemessen am gebauten Binary, 19.11.2026, WSL Alpine:

    akmesh mkcert -install -ca ./local
        installed ./local-cert.pem as /usr/local/share/ca-certificates/akmesh-local.crt
        and refreshed the trust store
    curl --resolve test1.local:18955:127.0.0.1 https://test1.local:18955/index.html
        local-tls-ok, Exit 0 - ohne --cacert, nur ueber das Systemvertrauen
    akmesh curl --resolve test1.local:18961:127.0.0.1 https://test1.local:18961/index.html
        eigenes-curl-ok, Exit 0
    akmesh curl https://test1.local:18961/index.html          (ohne --resolve)
        Exit 6, Name nicht aufloesbar - der Name bleibt im Request, nur das
        Ziel wird umgebogen
    akmesh mkcert -uninstall -ca ./local  ;  curl ...
        Exit 60 (Zertifikat nicht vertrauenswuerdig) - die Wurzel war der Grund

curl kann jetzt --resolve HOST:PORT:ADRESSE, in der Form von GNU. Die Adresse
wird nur beim Verbinden eingesetzt, Host-Header und SNI behalten den Namen.
Nicht unterstuetzt und mit Exit 2 abgelehnt: die Formen mit +, * und eine
eigene Portangabe nach Komma.

Ein Fehler beim Bauen, den der Test gefunden hat: -install mit einer noch nicht
vorhandenen CA hat eine neue CA angelegt und ins echte Systemverzeichnis
geschrieben; im Test stand ein /usr/local/share/ca-certificates/akmesh-nope.crt.
Jetzt stubben beide Install-Tests das Verzeichnis, und das angelegte Zertifikat
traegt die Namensbeschraenkung (-ca-domains wurde im Install-Zweig nicht
weitergereicht).

## 2026-11-19 httpd signiert pro SNI-Namen (acmesh-816, Schritt 3)

    akmesh mkcert -host test1.local -ca ./local -ca-domains .local -out ./leaf
    akmesh mkcert -install -ca ./local
    akmesh httpd -f -p 127.0.0.1:18971 -h . --tls-ca local-cert.pem -K local-key.pem

Gemessen am gebauten Binary, 19.11.2026:

    curl --resolve test1.local:18971:127.0.0.1 https://test1.local:18971/index.html
        sni-ok, Exit 0
    curl --resolve test2.local:18971:127.0.0.1 https://test2.local:18971/index.html
        sni-ok, Exit 0 - ein anderer Name am selben Prozess, eigenes Blatt
    curl --resolve example.com:18971:127.0.0.1 https://example.com:18971/
        Exit 35 (Handshake abgelehnt)
    Serverlog:
        TLS handshake error from 127.0.0.1:47924:
        example.com is not covered by the CA's name constraints (.local)

Das Blatt: ECDSA P-256, der Name im SAN, 390 Tage (Test prueft 389 bis 391),
Signatur von der lokalen Wurzel. Ein Name wird gehasht und beim zweiten Handshake
wiederverwendet, ein fremder Name wird nie signiert. Ein Blattzertifikat als
--tls-ca wird abgelehnt ("is not a CA certificate").

Dazu in dieser Runde: curl kann --cacert (ersetzt den Systemstore, wie bei GNU)
und --resolve. Ein Handshake mit einem Namen ausserhalb der Beschraenkung endet
mit Exit 35, nicht mit einer leeren Antwort.

Batteriepruefungen dazu: ca-sni-test1, ca-sni-test2, ca-sni-foreign (Exit 35) und
ca-sni-notice (die Logzeile ist da). Der Block installiert nichts ins
Systemverzeichnis: curl bekommt die Wurzel mit --cacert, damit die Batterie
nichts ausserhalb ihres Verzeichnisses anfasst.

Abschlusslauf nach der lokalen CA (acmesh-816), 19.11.2026:

    ./scripts/check-all.sh   278,15 s   exit 0
    battery linux            207 Pruefungen, 0 Fehler
    battery windows          185 Pruefungen, 0 Fehler
    help-audit               every command answers --help except the documented u-root gap
    go test -race            ohne Befund

Windows brauchte bei ca-sni-notice eine Warteschleife: die Logzeile schreibt der
Server in eigener Goroutine, und einmal nachgesehen war sie noch nicht da. Der
Lauf dauerte laenger als die 190 s vorher, weil der CA-Block auf beiden Zielen
auf Handshakes wartet.

## 2026-11-19 Parallele Anfragen: curl -Z, --parallel-max, --repeat (acmesh-730)

curl kann jetzt mehrere Anfragen gleichzeitig fahren: -Z und --parallel wie GNU,
--parallel-max N (Vorgabe 50 wie curl), --repeat N als unsere Ergaenzung fuer N
Anfragen auf dieselbe URL, --no-keepalive fuer Connection: close. Am Ende steht
eine Zeile auf stderr: Anzahl, Fehler, Dauer, Anfragen pro Sekunde, Statusverteilung.
Ohne -Z aendert sich nichts.

Gemessen, httpd mit einer Seite auf 127.0.0.1, ulimit -n 10240, je 2000 Anfragen:

    --parallel-max 50      2000 requests, 0 failed, 0.33 s, 6007 req/s, 200: 2000
    --parallel-max 200     2000 requests, 0 failed, 0.35 s, 5671 req/s, 200: 2000
    --parallel-max 300     2000 requests, 0 failed, 0.48 s, 4194 req/s, 200: 2000
    --parallel-max 500     2000 requests, 0 failed, 0.54 s / 0.99 s / 1.07 s in drei Laeufen
    --parallel-max 1000    1000 requests, 0 failed, 0.74 s, 1359 req/s
    --no-keepalive, 200    2000 requests, 221 failed, 0.50 s, 4032 req/s

Die Grenze ist nicht scharf: ein Lauf mit 500 gleichzeitigen Anfragen direkt nach
zwei anderen hatte 1905 von 2000 ohne Antwort, bei gleicher Einstellung danach
null Fehler, und der Server antwortete weiter. Das ist die Annahme-Queue des
Kernels und der Druck auf die kurzlebigen Ports, nicht der Handler. Ohne
keep-alive kostet jede Anfrage eine Verbindung, deshalb dort die Fehler.

Server: keine Aenderung noetig. net/http fahrt jede Verbindung in einer eigenen
Goroutine, und der Handler nimmt kein globales Lock. Die Zahlen ueber Loopback
messen Go und den Kernel, nicht das Netz.

certinfo kann jetzt --resolve HOST:PORT:ADRESSE und liest damit zum Beispiel
test2.local, ohne Eintrag in /etc/hosts. Batterie: ca-sni-two-leaves nutzt das
jetzt statt eines Umwegs.

wget: mit GNU als Vorbild ergaenzt um --ca-certificate FILE (der GNU-Name fuer
curls --cacert), --resolve, --referer, --method, --user und --password. Kein -Z:
wget kennt keine parallelen Anfragen, und ein erfundenes Flag waere schlimmer als
eines, das fehlt.

Batterie: curl-parallel-count, curl-parallel-status, curl-parallel-unknownflags
(Exit 2 bei --repeat 0) und ca-sni-two-leaves. 211 Pruefungen Linux, 189 Windows,
beide 0 Fehler.

Abschlusslauf nach den parallelen Anfragen (acmesh-730), 19.11.2026:

    ./scripts/check-all.sh   216,94 s   exit 0
    battery linux            211 Pruefungen, 0 Fehler
    battery windows          189 Pruefungen, 0 Fehler
    help-audit               every command answers --help except the documented u-root gap
    go test -race            ohne Befund

Ein erster check-all-Lauf war rot, und zwar in den drei neuen --resolve-Tests
(Exit 7, keine Ausgabe). Einzeln und mit -race liefen sie gruen: unter WSL
bricht waehrend des ganzen Suitelaufs gelegentlich ein Dial ab (dieselbe
Erscheinung wie accept4 failed 110 beim .exe). Die Tests versuchen es jetzt bis
zu dreimal, bevor sie einen Fehlschlag melden; das ist im Testkommentar
begruendet, damit niemand es fuer einen stillen Retry haelt.

## Schriften und Bilder aus der joshua-Uebernahme (19.11.2026)

Schriften: `internal/webfonts` uebernommen (Lora fuer Text, Go Mono fuer Code,
Lizenzpruefung aus der Namenstabelle), `internal/pdfgen/fonts.go` registriert sie
als `FontTypeCustom`. Der goldmark-pdf-Patch holt kein Google-Fonts mehr:

    md2pdf Binary ...... 13,51 MB vorher, 12,98 MB nachher
    go list -deps ...... kein go-swiss/fonts, kein ttlcache mehr im Build
                         (in go.mod bleiben sie als indirect, weil die go.mod
                         des gepatchten Moduls sie noch nennt)
    PDF-BaseFont ....... utf8lora, utf8loraB, utf8loraI, utf8loraBI, utf8go
                         vorher utf8roboto* und Helvetica
    ohne Netz .......... Rendern geht ohne Netz, die Bytes kommen aus dem Modul

Fehler auf dem Weg, die jetzt Tests abdecken:

    pdf.WithConfig nach WithPDF ...... wirft das PDF weg, gofpdf schreibt dann
                                       mit leerer Glyphentabelle: "index out of
                                       range [80] with length 0"
    Styles.CodeBlock vergessen ....... fragt eine Google-Schrift, die es nicht gibt
    UnicodeTranslator auf UTF-8-Font .. doppelte Kodierung: "Titel über Öl"
                                       wurde zu "Titel 좥r ֬"

Bilder im docx (md2docx): Betten jetzt ein statt nur [alt] zu schreiben.

    Testseite mit bild.png (120x60) .. word/media/bild1.png, 191 Bytes
    LibreOffice -> PDF ................ pdfimages: 120x60 rgb, 1 Bild
    LibreOffice -> HTML ............... genau ein <img>
    breites Bild (2000x1000) .......... cx=contentWidthEMU, cy genau die Haelfte
    fehlende Datei, SVG ............... bleiben Ersatztext, kein Medien-Teil

Zwei Fallen, mutationsgeprueft: ohne `xmlns:wp` am Dokument sagt LibreOffice
"source file could not be loaded", und ein `w:drawing` in einem offenen `w:p`
ist ein verschachtelter Absatz, den beide Programme nicht zeigen.

## md2epub: Schriften, Bilder, Lizenzseite (20.11.2026)

md2epub gab es schon (goldmark -> XHTML, Kapitel an den H1, OPF/nav selbst
geschrieben). Gefehlt haben Schriften, Bilder und die Lizenz - jetzt drin:

    Testbuch mit bild.png (120x60) ... 575817 Bytes, davon 570 KB Schriften
      OEBPS/fonts/lora-regular.ttf .... 212196
      OEBPS/fonts/lora-bold.ttf .......  90024
      OEBPS/fonts/lora-italic.ttf .....  94204
      OEBPS/fonts/go-mono-regular.ttf . 173248
      OEBPS/images/bild1.png ..........    191
      OEBPS/style.css .................    866
      OEBPS/colophon.xhtml ............   2876
    Alle xhtml/opf wohlgeformt (xml.dom.minidom), mimetype erster Eintrag, stored
    epub2md liest das Buch zurueck: Kapitel, Bildpfad images/bild1.png und die
    Lizenzseite kommen als Markdown heraus

Der Bildweg ist derselbe wie im docx: internal/imgload entscheidet nach den
Bytes (nur PNG, JPEG, GIF; nie HTTP), fehlt das Bild, bleibt [Beschreibung]
stehen. Mutationen: Bild-Tag unveraendert durchreichen -> Test rot; Manifest-
Eintrag fuer das Bild weglassen -> Test rot.

Die Lizenzseite ist ein eigenes Dokument (colophon.xhtml) und keine Kapitel-
datei: sie ist nicht das, was der Autor geschrieben hat, steht aber im Spine,
damit ein Leser sie findet.

## pdf2md: Tabellenseite verliert nicht mehr den Rest der Seite (20.11.2026)

Fehler: sobald eine Seite Zellrechtecke hatte, gab pdfmd nur die Tabelle aus
(pdfmd.go, alter Kommentar "the rest of the page order would only repeat it").
Gemessen an unserem eigenen PDF mit acht Abschnitten, 30-Zeilen-Tabelle und
Codeblock:

    /tmp/pdf2md  (alt) ...  63 Zeilen,  504 Woerter
    /tmp/pdf2md  (neu) ... 152 Zeilen, 1186 Woerter
    pdftotext -layout  ...          1177 Woerter

Neu gilt: der Textweg entscheidet zuerst (ausgerichtete Spalten tragen jede
Zeile), die Rechtecke nur, wenn der Text allein keine Spalten ergibt. Und dann
gehoeren ihnen nur die Zeilen innerhalb einer Zelle; alles andere bleibt in
Lesereihenfolge erhalten. Die Koerpergroesse kommt von der ganzen Seite, nicht
vom Bruchstueck - sonst verliert eine Ueberschrift ihre Ebene, wenn sie allein
vor der Tabelle steht.

Mutation: den alten Kurzschluss wieder einsetzen faerbt
TestPageWithTableKeepsTheTextAroundIt rot ("page lacks ...").

## docx2md liest Bilder und Inline-Code (20.11.2026)

Vorher: ein eingebettetes Bild verschwand spurlos, Inline-Code kam ohne Ticks
heraus. Jetzt:

    docx2md -o out.md doc.docx .... out.md mit "![Ein rotes Rechteck](bild1.png)"
                                    und bild1.png daneben, 191 Bytes (wie im Paket)
    ohne -o (stdout) .............. "[Ein rotes Rechteck]" als Text, weil es
                                    keinen Ordner gibt, in den das Bild kaeme
    zwei Bilder auf einer Datei ... bild1.png, bild1-2.png (eigene Namen)
    Inline-Code ................... `inline` aus der Zeichenvorlage CodeChar
                                    oder aus einer Monospace-Schrift (Consolas,
                                    Courier, Menlo, ...)
    Kette docx -> md -> epub ...... Bild kommt mit, epub2md liest
                                    images/bild1.png zurueck

Mutationen: Bildzweig abgeschaltet -> TestPictureComesOutOfThePackage rot;
Code-Erkennung abgeschaltet -> TestInlineCodeBecomesBackticks und
TestMonospaceFaceIsCode rot.

## pdf2md liest den Strukturbaum getaggter PDFs (20.11.2026)

Ein getaggtes PDF sagt, was sein Text ist. pdfmd folgt jetzt diesem Baum, statt
Ebenen aus Schriftgroessen zu raten. Am eigenen Testdokument
(internal/pdfmd/testdata/tagged.pdf, Ausgabe von joshua html2pdf):

    vorher (nur Seiten) ....... "# Kapitel eins", Tabellenzeile als Text,
                                Liste als zwei Absaetze
    jetzt (Strukturbaum) ...... "# Kapitel eins", "## Unterkapitel",
                                "- Erster Punkt", "- Zweiter Punkt",
                                "| Name | Wert |", "| Alpha | 1 |"

Drei Fehler fielen dabei auf, alle mit Test:

    value(dict, "/S") ......... gab fuer einen Namen als Wert "" zurueck, weil
                                die Schleife beim fuehrenden "/" schon endete.
                                Folge: jede Rolle leer, der Baum war keiner.
    Schluesselsuche ............ "/S" traf auch "/StructTreeRoot" ("tructElem").
                                Jetzt muss der Name an einem Trenner enden.
    arrayItems ................ "/K [27 0 R 28 0 R]" wurde an Leerzeichen
                                zerschnitten, und die Zahl aus der Referenz
                                landete als MCID im Baum.

Dazu zwei Fehler in der Textextraktion, die erst durch den Baum sichtbar wurden:

    TJ-Array .................. der Stift wurde nicht weiterbewegt und die
                                Zahlen im Array (Kerning, Wortabstand) fielen
                                weg: "DerersteAbsatz".
    BT und Tm ................. setzten die gemerkte Textposition zurueck, damit
                                war jeder Wortabstand unsichtbar. Jetzt bleibt
                                das Ende des vorigen Stuecks bekannt; Schwelle
                                0,2 der Schriftgroesse (Wortabstand bei 12 pt: 3 pt,
                                Kerning: Bruchteile eines Punktes).

Mutationen: Baumweg abgeschaltet -> Liste und Tabelle kommen nicht mehr als
solche zurueck; MCID-Zahlen im /K ignoriert -> dito.

## HTML als zweite Sprache: epub2html und html2epub (20.11.2026)

Die Kapitel eines EPUB sind selbst XHTML, also geht es ohne Markdown-Zwischen-
schritt. Neu: `epub2html` (Buch -> eine HTML-Seite) und `html2epub` (HTML ->
Buch, ueber `epubgen.ConvertHTML`).

    epub -> html ............ doc.html + images/bild1.png, 191 Bytes wie im Paket
    html -> epub ............ chapter1.xhtml, images/bild1.png, Fonts, colophon
    epub2md zurueckgelesen .. "# Bilderprobe", "![Ein rotes Rechteck](images/bild1.png)"
    ohne -o ................. statt <img> steht "[Ersatzttext]", kein toter Verweis
    Tabelle/verschachtelte Liste/Inline-Code kommen als HTML durch (Test)

Drei Dinge, die der Umbau gebraucht hat:

    bodyOfPage .............. eine ganze Seite bringt <!DOCTYPE>, <head> und ihren
                              Titel mit; im Buch steht davon nur der Body
    colophon.xhtml .......... wird beim Zuruecklesen uebersprungen, sonst steht die
                              Lizenzseite im naechsten Buch zweimal
    Bilder .................. brauchen einen Ordner; ohne -o bleibt der Ersatztext

Mutationen: bodyOfPage aus -> Titel der Seite leckt ins Kapitel (rot);
Colophon-Ueberspringen aus -> Lizenzseite reist mit (rot).

## pdf2html: getaggtes PDF als HTML (20.11.2026)

Der Strukturbaum traegt schon die Rollen, also schreibt `pdf2html` HTML statt
Markdown. Am eigenen Fixture (testdata/tagged.pdf):

    <h1>Kapitel eins</h1>          <h2>Unterkapitel</h2>
    <ul><li>Erster Punkt</li><li>Zweiter Punkt</li></ul>
    <thead><tr><th>Name</th><th>Wert</th></tr></thead>
    <tbody><tr><td>Alpha</td><td>1</td></tr>

Ohne Strukturbaum geht es ueber den Seitenweg: der geratene Markdown-Text wird
mit goldmark zu HTML, also eine Regelmenge und ein Ergebnis.

Mutationen: TH nicht mehr als Kopfzeile gezaehlt -> TestHTMLTableHeadIsAHead rot;
Liste als Absatz geschrieben -> TestHTMLKeepsTheStructure rot ("body lacks <ul>").

Was nicht mitkommt: Bilddaten (pdfmd liest keine Bildstroeme, die Figure steht mit
ihrem Alt-Text) und Links (Ziel steht in der Anmerkung, nicht im Baum).

## html2docx: HTML direkt nach docx (20.11.2026)

Der Weg ist HTML -> Markdown -> Dokument, und das ist Absicht: der Markdown-Leser
ist der, dem dieses Werkzeug schon vertraut, ein zweiter Dokumentbauer wuerde
davon abweichen. Damit die Tabelle dabei eine Tabelle bleibt, schreibt
`htmlmd.ConvertGFM` jetzt eine GFM-Tabelle mit Trennzeile; `Convert` bleibt
unveraendert, weil fuer das Lesen die schlichte Form schoener ist.

Gemessen an /tmp/t2.html (Ueberschrift, Code, fett, Link, Tabelle mit Kopf,
verschachtelte Liste, Bild):

    docx-Paket ............... 1x <w:tbl>, 6x <w:numPr>, <w:b/>, word/media/bild1.png
    notker check ............. "check clean: 7 parts identical"
    notker read .............. 7 Absaetze, 1 Tabelle (Kopfzeile als Tabellenzeile),
                               Listeneintraege als "List Paragraph", [image]
    docx2md zurueck .......... "# Probe", Tabelle mit Trennzeile, "- eins / - tief",
                               "[Link](https://example.org)"

Zwei Fehler unterwegs: ohne Trennzeile war die Tabelle fuer goldmark kein GFM,
also wurde sie zu Pipe-Text; und der Seitentitel aus <head> wanderte als erster
Absatz ins Dokument.

Mutationen: ConvertGFM -> Convert (Tabelle weg, rot); pageBody abgeschaltet
(Titel zweimal im Dokument, rot).

Was durch Markdown nicht erhalten bleibt und deshalb auch hier nicht: Spaltenbreiten,
verbundene Zellen, Farben, Schriften, Textboxen.

## any2md: ein Leser, fuenf Aliase (21.11.2026)

Gebaut mit `GOTOOLCHAIN=auto ./scripts/build-bb.sh linux`, geprueft am Binary
gegen fuenf echte Dokumente (docx/epub mit unseren eigenen Schreibern aus einem
Markdown gebaut, PDF = internal/pdfmd/testdata/tagged.pdf, dazu eine HTML-Seite
und eine RTF-Datei):

    any2md angebot.docx ....... "# Kapitel", "Ein Absatz mit **fett**."
    any2md buch.epub .......... dito
    any2md seite.html ......... "# Seite"
    any2md brief.rtf .......... "**Fett und Text.**"
    any2md bericht.pdf ........ "# Kapitel eins"
    any2md ohne-endung ........ (Kopie der PDF) "# Kapitel eins" -> Erkennung an den Bytes
    any2md notiz.txt .......... "unknown document format (any2md reads pdf, docx, epub, rtf, html)", exit 1

Die Aliase sind gegen any2md byte-gleich (`cmp`, alle vier gruen):

    pdf2md bericht.pdf      == any2md -f pdf bericht.pdf
    docx2md -o d1.md        == any2md -o d2.md
    html2md seite.html      == any2md seite.html
    epub2md buch.epub       == any2md buch.epub
    rtf2md brief.rtf        == any2md brief.rtf

Format gepinnt, Bytes sagen etwas anderes:

    pdf2md seite.html -> "pdf2md: seite.html: not a pdf, the bytes look like html", exit 1
    any2md -o x.md seite.html brief.rtf -> exit 2 ("-o takes one document")

Gatter: `./scripts/test-akmesh.sh ./akmesh` = 211 Checks, 0 Fehler;
`./scripts/help-audit.sh --check ./akmesh` = exit 0 (any2md und die fuenf
antworten auf --help mit ihrem Text). Groesse linux: 16310396 Bytes, wie vor dem
Umbau (16310396) - die Aliase kosten kein Byte, weil sie derselbe Leser sind.

Nebenbefund aus dem Gatter: epub2md las die eigene Lizenzseite als drittes
Kapitel (Batterie epub2md-chapter-count erwartete 2, bekam 3). Ursache war die
Spine-Schleife, die nur `properties="nav"` uebersprang; jetzt auch
`path.Base(href) == "colophon.xhtml"` (acmesh-g5x).

## Gruppierte Befehlsliste (21.11.2026)

Der Build generiert mit `makebb -g` und baut selbst (`GOTOOLCHAIN=auto
./scripts/build-bb.sh linux`, danach `windows`).

    ./akmesh            -> "Supported commands are:", dann "Documents (...)",
                           dann "Files and text:", "System:", "Network:",
                           "Shell:", "Other:"; 107 Zeilen " - name"
    ./akmesh.exe        -> dasselbe, 106 Namen (tracedeps fehlt auf Windows)

    Groesse linux ...... 16359651 Bytes (vorher 16310396, +49 KB fuer Tabelle und
                         Text). Ohne `-a -gcflags=all=-l` waeren es 18182268
                         Bytes gewesen: makebb baut mit abgeschaltetem Inlining,
                         und ein go build ohne diese Flags liegt 1,87 MB darueber.
    Batterie ........... 211 Checks, 0 Fehler
    help-audit --check . exit 0

Waechter, die rot werden muessen: Patch greift nicht (build-bb.sh bricht ab),
`printSupported()` fehlt in der generierten main.go (Abbruch), Binary ohne
"Files and text:" (Abbruch), Befehl ohne Gruppe (TestEveryCommandIsPlaced).

## Produktname und Zertifikats-Issuer (22.11.2026)

Gebaut mit `AKMESH_NAME=bssdsh GOTOOLCHAIN=auto ./scripts/build-bb.sh linux`,
danach wieder mit dem Standardnamen.

    ./bssdsh ............... "Failed to run command: command is not present in bssdsh: bssdsh"
    ./bssdsh shuf --version  "shuf (bssdsh) 1.0"
    ./bssdsh jq -V .......... "jq-bssdsh-1.0 (gojq)"
    ./akmesh (danach) ....... "command is not present in akmesh: akmesh"

Der Hin- und Rueckweg ist der Punkt: bssdsh bauen, dann akmesh bauen, dann sagt
jedes Binary seinen eigenen Namen. Groesse Linux 16363747 Bytes (vorher
16359651, +4 KB fuer internal/appname), Windows 16422400.

Issuer, mit `--ca ca --org "Test GmbH"`:

    certinfo ca-cert.pem .... Subject: CN=bssdsh test CA ca,O=Test GmbH
    certinfo leaf-cert.pem .. Issuer:  CN=bssdsh test CA ca,O=Test GmbH
    zweiter Aufruf, --org "Andere AG", gleiche CA:
        "mkcert: ca-cert.pem keeps its issuer \"Test GmbH\"; -org applies to a new CA"
    ohne --org, neue CA ...... Subject: CN=bssdsh test CA ca2,O=bssdsh mkcert

Gatter: Batterie 211 Checks 0 Fehler Linux, 189 Checks 0 Fehler Windows;
help-audit --check exit 0 auf beiden. `./akmesh mkcert --help` nennt -org, den
Standard aus dem Build-Namen und den Dateinamen im Trust-Store.

Zwei Fehler unterwegs: das erste sed fuer "NAME dispatched" traf auch
`var dispatched atomic.Pointer[string]` und zerlegte register.go (Build brach mit
"syntax error" ab); jetzt ist das Muster an Anfuehrungszeichen und ` %q`
verankert. Und makebb bettet register.go beim Installieren ein, also muss nach
einem Namenswechsel neu installiert werden - der Build prueft dafuer jetzt
`not present in $NAME` im makebb-Binary statt gegen eine Liste alter Namen.

## gripgrep auf Windows (22.11.2026, Vorarbeit für ein rg-Kommando)

Gegenstand: `gg.exe` (gripgrep, github.com/yackey-labs/gripgrep v0.17.0,
windows/amd64) gegen `/usr/bin/rg` (ripgrep 15.1.0, linux/amd64), gestartet
über WSL-Interop. Beide Binaries lesen dieselben Bytes, also ist der Vergleich
der Ausgabe ein Byte-Vergleich und kein Plattformvergleich.

Gebaut mit:

    GOOS=windows GOARCH=amd64 go build -trimpath -ldflags "-s -w" \
      -o binwin/gg.exe github.com/yackey-labs/gripgrep/cmd/gg   # 3313664 Bytes

Testkorpus (git-Repo, damit die Ignore-Regeln greifen): `crlf.txt` mit CRLF,
`lf.txt` mit LF, `latin1.txt` mit Bytes 0xfc/0x0f, `bin.dat` mit NUL-Bytes,
`.gitignore` sperrt `secret.txt` und das geschachtelte Verzeichnis
`sub/deep/`.

    gg.exe --files <dir>          gleiche Dateiliste wie rg, inklusive
                                  .gitignore und geschachteltem Verzeichnis
    rg -n token crlf.txt | od     beide: "value \r \n", das CR bleibt stehen
    gg.exe -n token crlf.txt|od   identisch, also CRLF wie bei rg behandelt
    latin1.txt                    0xfc geht unveraendert durch, keine
                                  UTF-8-Ersetzung, beide gleich
    bin.dat                        beide: -c meldet 1, Exitcode 0
    -n -c -l -F -w                alle Ausgaben nach sort bytegleich mit rg
    Exitcodes                     Treffer 0, kein Treffer 1, beide gleich
    relativer Pfad, "." und "sub"  funktionieren, Ausgabe mit "/" wie bei rg

Ganze Baum-Suche, warm, je drei bis fuenf Laeufe (`time`, Sekunden):

    Literal "func Run"        rg 0,021-0,025   gg 0,010-0,016
    Alternation "regexp|strings" rg 0,024-0,028 gg 0,012-0,013
    --files                   rg 0,021-0,023   gg 0,010-0,016

Faktor rund zwei zugunsten von gg auf diesem Korpus, nicht langsamer.

Nebenbefund, vorhandener Befehl: `akmesh.exe grep -r <dir>` setzt Pfade unter
Windows falsch zusammen. Gemessen im selben Korpus:

    .\akmesh.exe grep -r func sub   -> "sub/deep\\b.txt" (gemischte Trenner)
    .\akmesh.exe grep -r func .     -> "grep: ./git\\HEAD: ... nicht gefunden"
                                       fuer jede Datei in .git
    .\akmesh.exe grep -r func <abs> -> "<abs>/root\\.hermes\\..." , der ganze
                                       Pfad steht doppelt

Das CR am Zeilenende entfernt `akmesh.exe grep` anders als rg: auf beiden
Plattformen ohne `\r`, also plattformunabhaengig richtig und anders als rg,
das das CR stehen laesst.

## grep -r Anzeigepfade: Punktverzeichnisse (22.11.2026, acmesh-6nl)

Vorher (grep.go:505, doppeltes `strings.TrimPrefix`): `filepath.Walk` liefert
bereits bereinigte Namen, also ass der Trim den Punkt von `.git` und der Lauf
oeffnete `./git/HEAD`. Nachher `filepath.Rel` plus `filepath.ToSlash`, der
Anzeigetrenner bleibt fest "/".

Korpus (`$T` = frisches Verzeichnis mit `.git/HEAD` und `sub/b.txt`, `hit`):

    Linux,  akmesh grep -r hit .
      vorher:  grep: ./git/HEAD: No such file or directory   (exit 0)
      nachher: ./.git/HEAD:hit  und  ./sub/b.txt:hit          (exit 0)
    Windows, akmesh.exe grep -r hit C:\...\probe\.
      vorher:  grep: ./git\HEAD: ... nicht gefunden
      nachher: C:\...\probe\./.git/HEAD:hit, ./.git/HEAD:hit  (exit 0)
    Windows, akmesh.exe grep -r hit C:\...\probe   (absolute Wurzel)
      vorher:  <abs>/C:\...\probe\... , der Pfad doppelt
      nachher: <abs>/.git/HEAD:hit, ohne Doppelung           (exit 0)
    Windows, akmesh.exe grep -r hit C:\...\probe\sub
      nachher: <abs>\sub/b.txt:hit, Anzeigetrenner "/"        (exit 0)

Tests: TestRecursiveDotRootKeepsTheDotOfHiddenDirs und
TestRecursiveRootsAgreeWithTheirOwnSpelling (rot vor dem Fix: stderr
"grep: ./hidden/deep/h.txt: No such file or directory"). Gatter: go test ./...
gruen, gofmt sauber, Batterie 211 Checks Linux und 189 Checks Windows,
0 Fehler, help-audit --check exit 0.

## sed gegen GNU sed (22.11.2026, acmesh-4ne)

Referenz ist echtes GNU sed 4.9, nicht /bin/sed auf diesem Host (das ist
BusyBox, "This is not GNU sed version 4.0"):

    podman run --rm -i docker.io/library/debian:stable-slim sed --version

Vergleichslauf, je Fall `printf ... | ./akmesh sed <args>` gegen
`printf ... | podman run --rm -i docker.io/library/debian:stable-slim sed <args>`:

    Fall                        Argumente                                    vorher        GNU/akmesh jetzt
    a|ab auf "ab"               -E  s/a|ab/X/                                Xb            X
    Gruppe, Rückverweis         -E  s/(ab)/<\1>/                             x<1>y         x<ab>y
    ganzer Treffer              -E  s/(ab)/<&>/                              x<&>y         x<ab>y
    BRE plus                    s/a\+/X/ auf "aaa"                           aaa           X
    BRE Fragezeichen /g         s/a\?/X/g auf "ba"                           ba            XbX
    BRE Gruppe, Rückverweis     s/\(ab\)/<\1>/                               xaby (kein
                                                                              Gruppieren)   x<ab>y
    BRE Intervall               s/\(.*[0-9]\)\([0-9]\{3\}\)/\1,\2/          unverändert   1234,567
    Punkt über Zeilenumbruch    N;s/a.*b/X/ auf "a\nb"                       a\nb          X
    BRE, nackter Strich         s/a|ab/X/                                    ab            ab
    BRE Alternation             s/x\|y/Z/g                                   xyx           ZZZ
    POSIX-Klasse                s/[[:upper:]]/./g auf "aBcD"                 a.c.          a.c.

13 von 13 Fällen bytegleich mit GNU sed 4.9, davon 8 vorher abweichend.

Der Weg: `regexp.CompilePOSIX` aus dem Ticketvorschlag ist gemessen
untauglich - Go lässt im POSIX-Modus jeden Treffer fallen, der einen
Zeilenumbruch überspannt:

    CompilePOSIX("[^<]*").FindAllString("a\nb")  ->  ["a" "b"]
    Compile("[^<]*").FindAllString("a\nb")       ->  ["a\nb"]
    CompilePOSIX("a.*b").FindAllString("a\nb")   ->  []

In einem sed-Programm mit `N` und `b loop` wurde daraus eine Endlosschleife
(Go.Sed-Test TestRemoveTags, 20 s Timeout). Geforkt wird deshalb als Patch
(docs/design/patches/go-sed-gnu-semantics.patch, angewandt von
scripts/apply-go-sed-fork.sh): POSIX-linkeste, längste Wahl über die
Alternation des Musters selbst, BRE-Übersetzung, sed-Ersetzung mit \1..\9 und
&, "(?s)" auf jedem Muster, damit "." den Zeilenumbruch trifft, und
-E/-r über NewExtended/NewQuietExtended.

Grenze, die offen bleibt und im Fehlertext steht: Rückverweise im Suchmuster
(`s/\(a\)\1/X/`) brauchen einen Backtracking-Motor, den keine Go-Engine hat.

Gatter: go test ./... grün, gofmt sauber, Batterie 211 Checks Linux und 189
Checks Windows, 0 Fehler, help-audit --check exit 0. Größe linux 16371939
Bytes (vorher 16363747, also +8192), windows 16433664 (vorher 16422400).

## rg: ripgrep-Syntax auf der gripgrep-Fassade (22.11.2026, acmesh-55u)

Gegen das installierte ripgrep 15.1.0 gemessen, nicht gegen eine Erwartung.

- 26 Vergleichsfälle, Ausgabe als sortierte Menge (der Walker läuft parallel,
  die Reihenfolge ist kein Vertrag) plus Exitcode: rekursiv, -n, -c, -l, -F,
  -w, -x, -i, -S, -v, -A/-B/-C, -m, -o, -g (auch mit !), -t, -d, --hidden,
  --no-ignore, --files, --files -g, zwei -e, zwei Pfade, kein Treffer. Alle 26
  grün: `cd internal/rgcmd && go test -run TestMatchesRipgrep -v -count=1 .`
  (ohne rg im PATH überspringt der Test mit Meldung, er wird nicht still grün).
- Exitcodes: 0 bei Treffer, 1 ohne, 2 bei kaputtem Muster und bei fehlendem
  Pfad (`go test -run TestExitCodes .`).
- Probe am echten Baum:
  `diff <(./akmesh rg -n -t go 'func Run' internal | sort) <(rg -n -t go 'func Run' internal | sort)`
  -> identisch, 65 Zeilen. `./akmesh rg -c 'func ' internal` nennt 268 Dateien,
  `./akmesh rg --files .` 414 Dateien.
- rg im Sammel-Binary: `./akmesh rg --help` exit 0, `./akmesh gibtsnicht` listet
  " - rg" unter "Files and text:", `./scripts/help-audit.sh --check ./akmesh`
  exit 0.
- Windows (akmesh.exe aus dieser WSL-Shell, ohne wine):
  `./akmesh.exe rg -n 'for 16' C:/Windows/win.ini` -> `1:; for 16-bit app
  support` exit 0, CRLF bleibt im Treffer; `./akmesh.exe rg --files
  C:/Windows/System32/drivers/etc` listet 5 Dateien mit "/" als Trenner.
- Größe: linux 17080547 Bytes (vorher 16371939, +708608),
  windows 17157120 Bytes (vorher 16433664, +723456). Warnlinie 25 MB,
  harte Grenze 30 MB.
- Batterie: `./scripts/test-akmesh.sh ./akmesh` -> 211 Checks, 0 Fehler.

Was das Kommando bewusst nicht nachbaut (steht so in `rg --help`): PCRE2,
--type-add/--type-clear, --encoding, die globale gitignore-Datei
(core.excludesFile), --json und -q.

## rg --pre: Dokumente durchsuchen über any2md (22.11.2026, acmesh-idv)

Gemessen am gebauten Sammel-Binary, nicht am Entwicklungsstand.

- Ein docx (mit `akmesh md2docx` aus Markdown erzeugt) mit dem Satz
  "Der Vertrag endet am 31.12.": `./akmesh rg -n --pre any2md 'endet am 31.12' .`
  -> `./vertrag.docx:3:Der Vertrag endet am 31.12.`, Exit 0. Die Zeile 3 ist
  die dritte Zeile der Umwandlung ("# Vertrag", Leerzeile, Text), nicht eine
  Zeile im docx; so steht es auch in `rg --help`.
- Dieselbe Datei ohne `--pre`: `./akmesh rg 'endet am 31.12' .` -> kein
  Treffer, Exit 1. Der Treffer oben kommt also vom Preprocessor.
- Kaputtes PDF (Bytes mit `%PDF-1.4`, kein xref) im Verzeichnis:
  `rg: ./kaputt.pdf: no objects found`, Exit 2, Datei übersprungen. Kein
  stiller Ausfall.
- `--pre-glob '*.txt'` in einem Lauf: die .txt geht durch den Wandler
  (`any2md: unknown document format` -> Exit 2), das docx bleibt roh und wird
  nicht gefunden. Beide Richtungen in einem Lauf sichtbar.
- Externer Befehl (`--pre <programm>`, Dateipfad als letztes Argument):
  `cat` über eine .dat liefert den Treffer; der Test überspringt sich mit
  Meldung, wenn das Programm fehlt.
- Dauer für 20 docx (je ~2,9 KB, `akmesh md2docx`, bash time, 3 Läufe):
  mit `--pre any2md` 0,023-0,029 s, ohne `--pre` 0,009-0,010 s. Die Umwandlung
  kostet also rund 0,7 ms je Datei (jede Datei wird in einem Lauf genau
  einmal gewandelt; einen Plattencache gibt es nicht).
- Windows: `./akmesh.exe rg --pre any2md --pre-glob '*.ini' 'for 16'
  C:/Windows/win.ini` -> `rg: C:/Windows/win.ini: any2md: unknown document
  format ...`, Exit 2. Der Fehlerpfad des Preprocessors läuft auf Windows; ein
  Erfolgsfall mit Dokument liegt dort nicht vor, ohne eine Datei auf C: anzulegen.
- Größe: linux 17084643 Bytes (vorher 17080547, +4096),
  windows 17163264 (vorher 17157120, +6144).
- Gatter: `go test ./...` grün, gofmt sauber, Hilfe-Audit exit 0, Batterie
  211 Checks 0 Fehler, Lizenzprüfung 19 Module.

## ps: Spalten wie procps-ng 4.0.4 (22.11.2026, acmesh-vfj)

Gemessen am gebauten Binary gegen procps im Container, mit --pid=host, damit
beide denselben Prozess sehen.

- procps-ng 4.0.4 (debian stable-slim, per apt nachinstalliert) sagt:
  `ps -ef` -> `UID          PID    PPID  C STIME TTY          TIME CMD`,
  `ps aux` -> `USER         PID %CPU %MEM    VSZ   RSS TTY      STAT START   TIME COMMAND`.
  akmesh gibt jetzt dieselben Kopfzeilen in derselben Reihenfolge aus:
  `UID PID PPID C STIME TTY TIME CMD` und
  `USER PID %CPU %MEM VSZ RSS TTY STAT START TIME COMMAND`.
- Feldliste: `ps -o pid,pcpu,pmem,vsz,rss,time,start,stime,c -p PID` liefert
  `PID %CPU %MEM VSZ RSS TIME START STIME C` (START mit Sekunden wie procps
  -o start, STIME ohne).
- Werte an einem Besetzt-Prozess (`while :; do :; done`, 3 s alt):
  VSZ 1656 / 1656, RSS 948 / 948, %CPU 85,1 / 86,5 (die zwei Läufe lagen
  Sekunden auseinander, %CPU ist der Lebensdurchschnitt), C 85 / 86
  (abgeschnitten, nicht gerundet), TIME 00:01:33 / 00:01:53 aus demselben Grund.
- RSS: erst aus dem rss-Feld von /proc/pid/stat gelesen, das lag 144 KB unter
  VmRSS (804 gegen 948). procps nimmt die resident-Zahl aus /proc/pid/statm;
  nach der Umstellung sind beide gleich.
- TIME: bei -ef und im Standardaufruf immer HH:MM:SS (procps: 00:00:00), in aux
  kompakt (0:00), wie procps es zeigt.
- START/STIME: heute HH:MM:SS bzw. HH:MM, früherer Tag im selben Jahr "Nov19",
  älter "2025". Im Container stand 16:22, auf dem Host 17:22 - das ist die
  Zeitzone (UTC im Container), nicht die Uhrzeit.
- Windows (`./akmesh.exe ps aux`): Kopfzeile identisch; Prozesse, die sich
  öffnen lassen, füllen VSZ/RSS/%CPU/START (z. B. AUDIODG.EXE mit Nov07).
  Geschützte Prozesse (System, svchost) bleiben bei "-", weil OpenProcess
  abgelehnt wird. RSS kann dort größer als VSZ sein, weil Windows
  WorkingSetSize und PagefileUsage liefert und nicht virtuellen Speicher.
- Größe: linux 17096931 Bytes (vorher 17084643, +12288),
  windows 17173504 (vorher 17163264, +10240).
- Gatter: go test ./... grün, gofmt sauber, Batterie 211 Checks 0 Fehler,
  Hilfe-Audit exit 0.

## Referenz-Image fuer GNU-Vergleiche gepinnt (22.11.2026, acmesh-ytk)

Das wandernde Tag debian:stable-slim lieferte im November 2026 nacheinander
coreutils 9.1, 9.4 und 9.7. Ab jetzt steht das Image mit Digest in
docs/reference-image.txt, samt Debian-Version (13.7) und den Werkzeugversionen
(coreutils 9.7, sed 4.9, grep 3.11, tar 1.35, procps-ng 4.0.4 per apt). Neue
Messabschnitte nennen die Version des verglichenen Werkzeugs; aeltere Abschnitte
bleiben als Historie stehen.

## akmesh als minimales System im Container (22.11.2026, acmesh-w6d)

Proben mit dem gebauten Binary, nicht mit einer Erwartung.

- Der Standardbau ist dynamisch gegen musl: `ldd /akmesh` im Alpine-Container
  nennt `/lib/ld-musl-x86_64.so.1`. In debian stable-slim antwortet die Shell
  deshalb `sh: 1: /akmesh: not found` - der Loader fehlt, nicht die Datei.
- Ursache sind die cgo-Pakete `net`, `os/user`, `runtime/cgo` (gezaehlt mit
  `go list -deps`). Mit CGO_ENABLED=0 baut das Skript unveraendert; die Groesse
  faellt von 17096931 auf 17035388 Bytes.
- Dieses statische Binary laeuft in einem leeren Image (per `podman import`
  aus einer leeren tar gebaut): `/akmesh sh -c 'echo static-ok'` antwortet.
- Mit Applet-Symlinks (`/bin/ls -> /akmesh` usw.) und PATH=/bin laeuft ein
  richtiges Skript: `ls /`, `printf "b\na\n" | sort` (a, b), `ps -e`, `wc -w`
  (3), `date -u`. Dazu braucht es nur einen Mount von /proc (fuer ps) - den
  legt der Container-Laufzeit an, nicht akmesh.
- Was fehlt, sobald das Image leer ist:
  `id` bricht mit `open /etc/passwd: no such file or directory` ab (kein
  Rueckfall auf die nackte UID), `cat /etc/passwd` ebenso; curl/ssh brauchen
  /etc/resolv.conf und das CA-Bundle; /dev/null legt podman an, akmesh kann es
  nicht (kein mknod, kein mdev).
- Nicht vorhanden und fuer ein bootendes System noetig: init/PID1 (kein
  Zombie-Reaping), mount, mdev, ip/ifconfig, login/passwd, chroot, flock, fuser.
- Windows: alles aus scripts/cmds.txt baut mit; nur die Liste
  scripts/cmds-linux-only.txt (df du free hostname kill ps sync timeout uname)
  ist ausgenommen, wobei kill und ps inzwischen eigene Windows-Fassungen haben.
  Nicht uebertragbar bleiben mount, mdev, init als PID1, chroot, ip/ifconfig
  (Windows-API), login/passwd (Windows-Konten).

## id: antwortet ohne /etc/passwd, wie GNU (22.11.2026, acmesh-eqq)

Geforkt (internal/forked/id, Upstream-Commit und Hashes im Manifest), weil
u-root-id bei fehlender Datei abbricht. Vergleich im Container mit leeren
Dateien bzw. im leeren Image.

- GNU id mit leerer /etc/passwd und /etc/group (debian stable-slim, Dateien per
  -v auf leere Kopien gebunden): `uid=0 gid=0 groups=0`, Exit 0.
  `id -un`: druckt `0`, dazu `id: cannot find name for user ID 0` auf stderr,
  Exit 1.
- akmesh im leeren Image (statischer Bau, /bin/id -> /akmesh, nur /proc
  gemountet): `uid=0 gid=0 groups=0`, Exit 0; `id -un` druckt `0` plus
  dieselbe Meldung, Exit 1. Vorher: `open /etc/passwd: no such file or
  directory`, keine Ausgabe.
- Primaergruppe: u-root nahm nur syscall.Getgroups(); im leeren Image war die
  Liste leer und die Ausgabe endete bei `groups=`. Jetzt wird die GID des
  Prozesses ergaenzt, wenn sie fehlt - daher `groups=0`.
- Kombinierte Kurzflags: `id -un` scheiterte am flag-Paket
  ("flag provided but not defined: -un"); jetzt werden -g/-G/-n/-u/-r
  zerlegt, `id -un` druckt `root`, `id -Gn` die Gruppen, `id -uZ` bleibt ein
  Fehler mit Exit 2.
- Ein bekannter Nutzer behaelt seine Namen: `./akmesh id` auf dem Host
  `uid=0(root) gid=0(root) groups=0(root),...` wie vorher.
- Windows: `./akmesh.exe id` -> `uid=-1 gid=-1 groups=-1`, unveraendert
  gegenueber u-root (es gibt dort keine uid; eine Windows-Fassung ueber
  Token-SIDs waere eigene Arbeit).
- Groesse: linux 17105123 Bytes (vorher 17096931, +8192),
  windows 17178112 (vorher 17173504, +4608).
- Gatter: go test ./... gruen (einmalig flackerte
  TestDashDashStillEndsTheOptions mit fremdem stdin, einzeln und im zweiten
  Lauf gruen), Batterie 211 Checks 0 Fehler, Hilfe-Audit exit 0.

## BusyBox-Ersatz: gemessener Bedarf statt Schaetzung (22.11.2026, acmesh-w6d)

Fortsetzung des Abschnitts oben. Messsystem: Alpine 3.16, BusyBox 1.37.0
(`busybox 2>&1 | head -1`), akmesh vom 22.11. 19:51 (Linux-Bau).

Anzahl und Abdeckung:

- `busybox --list | wc -l` -> 304 Applets.
- Als Symlink auf /bin/busybox ausgeliefert (Python-Lauf ueber /bin, /sbin,
  /usr/bin, /usr/sbin: `os.readlink` endet auf `busybox`) -> 283 Applets.
  Die 21 Differenz sind Applets, die diese Installation nicht verlinkt
  (z.B. linux32, linux64, nanddump/lzop-Familie).
- `./akmesh gibtsnicht` -> 108 Befehle (Zeilen der Form " - Name").
- Schnittmenge: 71 von 283 ausgelieferten Applets, also 25 Prozent. Es fehlen
  212 Namen.
- Umgekehrt: 38 akmesh-Befehle hat BusyBox nicht - any2md, die *2md-Aliase,
  *2html, md2*, html2*, rg, jq, curl, file, patch, mkcert, policy, verify,
  strace, tracedeps, certinfo, iconv, shasum, netcat, ssh, scp.

Was dieses System wirklich aufruft (nicht geschaetzt): 60 Skripte mit
`#!/bin/sh` oder `#!/sbin/openrc-run` unter /etc/init.d, /etc/periodic,
/etc/profile.d, /usr/local/bin, /etc/apk, /etc/network, /lib/rc,
/usr/share/udhcpc sowie /etc/profile und /etc/crontabs/root (86 Dateien
gesehen, 54 davon openrc-run). Tokenzaehlung, jeweils erstes Wort eines
Segments nach ; & | ( ) und $(, Zuweisungen abgeschnitten.

Haeufigste Aufrufe, die akmesh NICHT hat (Anzahl Aufrufstellen):

    mount 30, ip 26, brctl 26, modprobe 11, sysctl 8, printf 7, umount 6,
    mknod 5, test 2, ifdown 2, rmdir 2, dmesg 2, ifconfig 2, route 2,
    hwclock 2, swapon 1, swapoff 1, su 1, chattr 1, fuser 1, chgrp 1,
    halt 1, reboot 1, fsck 1, zcat 1, blkid 1, depmod 1, pgrep 1

Haeufigste Aufrufe, die akmesh hat: echo 27, rm 21, grep 19, mkdir 18,
sed 13, sleep 13, ln 9, cat 8, awk 7, find 5, chmod 5, sync 4, cp 4,
xargs 4, mv 4, env 4, cut 3, mktemp 3, uname 3, timeout 3, wget 2,
readlink 2, hostname 2, true 2, dazu je einmal touch, dirname, dd, md5sum,
which, df, tail, ping, kill, head.

Grenze der Methode: die Zaehlung ist eine Obergrenze. Sie zaehlt auch Woerter
aus case-Zweigen, Funktionsnamen und Aufrufe, die erst in einer if-Verzweigung
gelten. Sie sagt nichts ueber Haeufigkeit zur Laufzeit; dazu gehoerte ein
Mitschnitt auf einem laufenden System, den es hier nicht gibt.

Groesse und Startzeit (300 Laeufe je Binary, while-Schleife mit echo):

- Groesse: akmesh 17105123 Bytes gegen BusyBox 808712 - Faktor 21,2,
  Mehrverbrauch 15,6 MB im Image.
- Startzeit: akmesh 2053 ms fuer 300 Laeufe (6,84 ms je Aufruf), BusyBox
  258 ms (0,86 ms). Also rund 8x und knapp 6 ms Aufschlag je Kommando.
  Fuer eine Skriptkette mit 100 Kommandos sind das 0,6 Sekunden.

Stichproben gleicher Namen (BusyBox 1.37.0 gegen akmesh):

- `ls -l f.txt`: akmesh `-rw-r--r-- 1 root root 4 Nov 22 21:56 f.txt`,
  BusyBox polstert Rechte und Groesse breiter (`-rw-r--r--    1 root     root
  ...`). Gleiche Felder, andere Spaltenbreite - Skripte, die Spalten per cut
  schneiden, brechen.
- `tar czf t.tgz f.txt` (alte Form ohne Bindestrich): BusyBox macht es,
  akmesh antwortet `tar: must supply at least one of: -c, -x, -t` und Exit 2.
  Mit `-czf` oder `-c -z -f` baut akmesh das Archiv, das BusyBox wieder
  auspackt. Altform ohne Bindestrich ist also die einzige Luecke.
- `gzip`: beide erzeugen g1.gz bzw. g2.gz, Exit 0.
- `sh`: akmesh faehrt gosh (mvdan/sh) statt ash; `echo $((1+1))`, Zuweisung
  und Ausgabe stimmen. Das ist ein anderer Dialekt, kein Nachbau: 54 der
  gescannten Skripte sind openrc-run, und /bin/sh zeigt hier auf BusyBox-ash.

Antwort: nein als vollstaendiger Ersatz, teilweise als Werkzeugkasten.

- Als Ersatz fuer busybox auf diesem System: nein. Es fehlt die ganze
  Systemklasse - kein init/PID1 mit Zombie-Reaping, kein mount, kein ip,
  kein mknod/mdev, kein ash als /bin/sh, kein login/passwd. Ohne diese fuenf
  bootet nichts, was heute bootet.
- Als Shell fuer einen Agenten: ja, mit Bedingungen. Der Agentenkern ist
  breiter als BusyBox (rg, jq, curl, file, patch, die Konverter), die
  Textwerkzeuge sind gemessen GNU-nah (sed 13 von 13, ps nach procps,
  grep -r Pfade), und die Policy greift prozessweit.
- Der Abstand ist billiger zu schliessen, als er aussieht: von den 27
  fehlenden, tatsaechlich aufgerufenen Namen bringt u-root 12 schon mit
  (mount, umount, ip, brctl, mknod, dmesg, modprobe, hwclock, blkid, reboot,
  switch_root, init) - das ist eine Zeile in scripts/cmds.txt, kein neuer
  Code. Geprueft per `ls .build/uroot/cmds/core` und `cmds/exp`; ob die
  u-root-Fassung die benutzten Unterbefehle kann, sagt erst ein Lauf im
  leeren Image.

Luecken nach Klasse (Namen, die diese Installation ausliefert und akmesh nicht
hat; erzeugt aus den Symlinks gegen die Befehlsliste, ohne Gewichtung):

- Kern (Text, Dateien, Suche), 21: ash beep bunzip2 bzcat bzip2 chgrp chown
  cksum egrep fgrep groups gunzip install od printf rmdir test tty who whoami
  zcat
- Netzwerk, 19: arp brctl ether-wake ifconfig ifenslave ip ipaddr iplink
  iproute iprule iptunnel nc netstat nslookup ping6 route traceroute udhcpc
  vconfig
- Hardware und System, 61: blkid blockdev dmesg dnsdomainname eject fbsplash
  fdisk findfs fsck fstrim getopt hwclock init insmod ionice iostat ipcrm ipcs
  killall5 klogd less loadfont loadkmap logger logread losetup lsmod lsof lsusb
  mdev mknod mkswap modinfo modprobe mount mountpoint nameif nbd-client
  partprobe pivot_root poweroff pwdx raidautorun readahead rev rfkill setconsole
  slattach su swapoff swapon switch_root sysctl syslogd top tunctl vlock watch
  watchdog zcip
- Boot und Init, 1 (der Rest der Bootkette steht oben unter Hardware): reboot
- Login und Rechte, 12: addgroup adduser chpasswd chroot crond crontab cryptpw
  delgroup deluser login mkpasswd passwd
- Ohne eigene Klasse, 98: acpid add-shell adjtimex arch arping bbconfig bc
  blkdiscard cal chvt dc deallocvt depmod dos2unix dumpkmap expand expr factor
  fallocate fatattr fbset fdflush flock fold fsync getty hd hostid inotifyd
  ipcalc ipneigh kbd_mode last link linux32 linux64 lzop lzopcat makemime mesg
  microcom mkfifo mpstat nanddump nandwrite nice nmeter nohup nologin nproc
  nsenter ntpd openvt paste pgrep pidof pipe_progress pkill pmap pscan rdate
  rdev reformime remove-shell renice resize rmmod run-parts sendmail setfont
  setkeycodes setlogcons setpriv setserial setsid sha3sum showkey shred stty sum
  traceroute6 tree ttysize udhcpc6 umount unexpand unix2dos unlink unlzop
  unshare usleep uudecode uuencode vi volname whois

## Systempaket 1: mount, umount, mknod, blkid, losetup (22.11.2026, acmesh-hdb)

Fuenf u-root-core-Befehle aufgenommen, weil w6d sie als gemessene Luecke zeigt
(mount 30 Aufrufstellen in den Init-Skripten, umount 6, mknod 5, blkid 1).

Bau und Plattform:

- Neue Zeile in scripts/cmds.txt: `linux: mount umount mknod blkid losetup`.
  Ein Praefix `linux:` markiert Namen, die nur fuer Linux bauen; der Bau filtert
  sie fuer GOOS=windows heraus, statt am fehlenden Linux-Code zu scheitern. Die
  Kennung lesen ausserdem internal/cmdlist (Test) und scripts/help-audit.sh.
- `./akmesh gibtsnicht` listet die fuenf unter System, `./akmesh.exe gibtsnicht`
  listet keinen davon (gezaehlt mit grep -c).
- Groesse Linux 17146083 Bytes (vorher 17105123, +40960), Windows 17178624
  (vorher 17178112, +512). Statischer Bau (CGO_ENABLED=0) 17092732.
- `bash scripts/help-audit.sh --check ./akmesh` exit 0. blkid antwortet auf
  --help ohne Usage-Text (u-root), steht deshalb in KNOWN_NO_HELP.

Lauf im leeren Image (Rezept references/minimal-system-probe.md, statischer Bau):

- Ohne --privileged: `mount -t tmpfs tmpfs /mnt/t` -> "operation not permitted",
  Exit 1; `mknod` ebenso. Das ist der fehlende CAP_SYS_ADMIN des Containers,
  nicht der Befehl. Der Befehl nimmt also Rechte ernst.
- Mit --privileged: mount Exit 0, Datei in den tmpfs geschrieben und gelesen,
  umount Exit 0; `mknod /tmp/nk c 1 3` Exit 0, danach `echo x > /tmp/nk` Exit 0
  (die Geraetedatei tut, was sie soll).

Stichproben gegen BusyBox 1.37.0 im Alpine-Container, gleiche Aufrufe:

- `mknod /tmp/nk c 1 3`: BusyBox legt `crw-r--r--` an (0666 und Umask 0022),
  akmesh `crw-r-----` (0660 und Umask). Gleiche Geraetenummern, andere Rechte.
- `blkid`: BusyBox druckt je Geraet `"/dev/sdd: UUID=... TYPE=..."`. akmesh
  druckt nur nackte Pfade (`/dev/loop0`, `/dev/sda`) ohne UUID und TYPE - und
  nennt sie auch dann, wenn /dev im Container gar keine Blockgeraete hat, die
  Liste kommt also nicht aus /dev.
- `losetup -f`: BusyBox druckt `/dev/loop0`. u-root-losetup kennt -f nicht:
  "flag provided but not defined: -f", Exit 2.
- Alle drei Abweichungen stehen als eigenes Ticket, weil sie Ausgabeformate
  sind, die Skripte lesen, und nicht nur fehlende Namen.

## sh: Login-Modus, Startdateien und eine Sitzung, die bleibt (22.11.2026, acmesh-4g7)

Befund vorher (am Quelltext, nicht vermutet): `internal/sh` las ohne `-c` die
ganze Standardeingabe bis EOF und fuehrte sie einmal aus; `-i` wurde geparst
und verworfen; Startdateien wurden nicht gelesen. u-root-gosh hilft nicht, sein
interaktiver Teil ist `errNotImplemented`.

Umgesetzt:

- `-l` und `--login` lesen /etc/profile, dann ~/.profile; heisst die Shell
  `bash`, ~/.bash_profile statt ~/.profile (beide zu lesen wuerde die Einrichtung
  doppelt ausfuehren).
- Ein interaktiver Aufruf ohne Login liest `$ENV`, wie POSIX es vorsieht.
- Ein fehlendes Profil ist kein Fehler (ein leeres Image hat keines); `exit` im
  Profil beendet die Sitzung mit dem Exitcode.
- Prompt aus PS1, Vorgabe `# ` fuer root und `$ ` sonst; eine von aussen
  geerbte PS1 wird entfernt; bei unfertiger Eingabe (for/if/heredoc) `> `.
- Zustand bleibt ueber Zeilen: `x=7` in einer Zeile, `echo $x` in der naechsten
  gibt `7`.
- `umask` fehlte in mvdan/sh und wird von jedem /etc/profile aufgerufen; jetzt
  als eingebauter Befehl (oktal, Abfrage und `-p`); Symbolik meldet ehrlich
  "not supported", Windows meldet "not supported on Windows".

Zwei eigene Fehler, die erst die Tests zeigten:

- Ein ganzer `*syntax.File` an `Runner.Run` bedeutet "exit": der Interpreter ist
  danach verbraucht, die naechste Zeile tut nichts. `Runner.Reset()` dazwischen
  "repariert" das nur scheinbar - Reset loescht Variablen, Optionen und das
  Verzeichnis. Richtig ist, die Statements einzeln zu fahren (`runStatements`).
- Der Interpreter puffert stdin fuer eigene Befehle und stiehlt der Sitzung eine
  Zeile; das war zu einem Drittel der Testlaeufe zu sehen. Die Leseschleife
  liest jetzt die Roh-Eingabe, und die Shell bekommt einen leeren Leser.

Belege am gebauten Binary (Linux, 17166563 Bytes, +20480 gegen 17146083;
Windows 17196544, +17920):

- Pipe: `printf 'echo eins\necho zwei\nx=7\necho x=$x\n' | ./akmesh sh -i`
  druckt `# eins`, `# zwei`, dann `# x=7`, Exit 0.
- Mehrzeilig: `for i in a b; do` zeigt `> `, danach `teil a` und `teil b`.
- Echtes Terminal (pty): Prompt `# `, getippte Zeile wird ausgefuehrt, naechster
  Prompt erscheint.
- Login mit dem echten /etc/profile (Alpine): `umask` gibt `0022` und keine
  Fehlermeldung mehr; PATH kommt aus dem Profil.
- Container (alpine, Binary eingebunden): `sh -l -c 'echo "PATH=$PATH"; umask'`
  -> PATH aus /etc/profile, `0022`; ein eigenes ~/.profile im Container setzt
  `MARK=im-container` und das ist in der Sitzung sichtbar.
- Tests: 13 Faelle in internal/sh (Reihenfolge der Profile, .bash_profile,
  fehlendes Profil, exit im Profil, Zeilenschleife, Fehler ueberlebt, PS1,
  Zustand ueber Zeilen, umask in drei Faellen, unbekannte Option).

- `sh --help` und `sh -h` drucken die Hilfe und enden mit 0 (vorher 2: der
  Hilfe-Audit meldete beide Namen zu Recht).
- Gatter: go test gruen, Hilfe-Audit exit 0, Batterie 211 Checks 0 Fehler.

Grenzen, offen als Ticket acmesh-51b: in der Sitzung liest das builtin `read`
nichts, weil die Shell einen leeren Leser bekommt; und `$0` ist in jedem Aufruf
`gosh`, weil der Dispatcher argv[0] setzt - der Login ueber den Namen `-sh` ist
damit nicht erreichbar, `-l`/`--login` ist der Weg.

## login: eine Sitzung als anderer Nutzer (22.11.2026, acmesh-7q7)

Der zweite Teil des Logins: acmesh-4g7 hat der Shell beigebracht, Profile zu
lesen; `login` bringt die Sitzung als den richtigen Nutzer ins richtige
Home-Verzeichnis.

- `login [-f] USER` liest /etc/passwd (Name, UID, GID, Home, Shell) und wechselt
  der Reihe nach: Gruppe, Zusatzgruppen (auf die eine Gruppe gesetzt), dann UID.
  Ein Fehler dabei bricht ab; es gibt keinen Rueckfall auf den aktuellen Nutzer.
- Home-Verzeichnis wird betreten; fehlt es, gibt es eine Warnung und die Sitzung
  startet in /.
- Umgebung: HOME, USER, LOGNAME, SHELL und PATH werden gesetzt, der Rest der
  Aufrufer-Umgebung bleibt, der PATH des Aufrufers ebenfalls (ein Login an der
  Konsole erbt, was getty hatte). Eine geerbte HOME des Aufrufers wird ersetzt,
  nicht verdoppelt.
- Die Shell wird mit `-l` gestartet, wenn sie sh oder bash heisst; damit liest sie
  /etc/profile und dann ~/.profile.
- Ohne `-f` verweigert login die Anmeldung mit einem Satz: in diesem Bau gibt es
  keine Passwortpruefung (kein PAM, kein /etc/shadow-Leser). Ein Login, der
  Anmeldung vortaeuscht, waere schlimmer als einer, der es sagt.
- Windows: der Befehl antwortet "not supported on Windows" (kein /etc/passwd,
  kein setuid), Hilfe mit Exit 0.

Beleg im Container (alpine, eigenes /etc/passwd eingebunden, /sh -> /akmesh):

    /akmesh login -f tester < /dev/null   -> Exit 0
    ls -l /home/tester                    -> proof, Besitzer tester uid 1000
    cat /home/tester/proof                -> "profil von tester"

Das zeigt in einem Lauf: UID/GID-Wechsel, Home als Arbeitsverzeichnis,
Login-Shell gestartet, ~/.profile gelesen - und dass die Datei dem neuen Nutzer
gehoert, nicht root.

Weitere Belege: 7 Tests in internal/login (ID-Wechsel, PATH bleibt, HOME wird
ersetzt, ohne -f verweigert, unbekannter Nutzer, Hilfe); go test gruen;
`scripts/help-audit.sh --check ./akmesh` Exit 0; Batterie 211 Checks 0 Fehler;
`./akmesh.exe login -h` antwortet ebenfalls.

Groesse: Linux 17182947 Bytes (+16384 gegen 17166563), Windows 17198080
(+2048 gegen 17196032).

## login: Passwortpruefung ueber /etc/shadow (23.11.2026, acmesh-7tx)

`login` ohne `-f` prueft jetzt ein Passwort gegen /etc/shadow. Ohne Passwort geht
es nicht weiter, und der Zustand des Kontos wird benannt.

Umgesetzt (internal/login):

- sha512-crypt ($6$) und sha256-crypt ($5$) in reinem Go, internal/login/shacrypt.go.
  Rounds werden gelesen, auf 1000 bis 999999999 begrenzt und im Ergebnis gefuehrt,
  wie glibc es tut.
- Pruefwerte stammen von der Maschine selbst, nicht aus dem Gedaechtnis: sechs
  Hashes von `openssl passwd -6/-5` (OpenSSL 3.5.5) und `perl -e 'print crypt(...)'`
  gegen glibc, darunter rounds=10000, rounds=1400 und die Rundung von rounds=10
  auf 1000. Alle sechs stimmen; je Hash wird auch das falsche Passwort abgelehnt.
- Vergleich in konstanter Zeit (subtle.ConstantTimeCompare).
- Nicht unterstuetzte Formate ($y$ yescrypt, $2$ bcrypt, $1$ md5-crypt, alte
  DES-Hashes) werden als "unsupported" gemeldet, nicht als falsches Passwort.
- /etc/shadow: gesperrt (! oder *), abgelaufen (Feld 8), ohne Passwort und eine
  fehlende Datei haben je eine eigene Antwort.
- Ohne -f: Frage "Password: ", auf einem Terminal ohne Echo (x/term), sonst eine
  Zeile von der Eingabe. Falsches Passwort und unbekannter Name ergeben denselben
  Satz und warten 2 Sekunden (failDelay, im Test auf 0).
- Das Passwort wird auch bei unbekanntem Namen gelesen, damit die Zeit nicht
  verraet, welche Namen existieren.

Befund an einer Bibliothek: github.com/GehirnInc/crypt (MIT, reines Go) rechnet
$6$/$5$ ohne rounds korrekt, schreibt bei "rounds=" aber ein doppeltes `$` und
einen anderen Hash:

    $6$rounds=10000$roundsalt$$ZJS34DsPMrYafh0uQ9RwBlcJF2Q9pr/...   (falsch)

Deshalb eigene Implementierung, keine neue Abhaengigkeit: go.mod ist wieder ohne
Fremdmodul. Der erste Versuch war ausserdem selbst falsch (P und S wurden auf ihre
Laenge gekuerzt statt so oft geschrieben, wie ihre Laenge ist; der letzte
Base64-Block nahm Byte 0 statt einer Null) - die Pruefwerte haben beides gefunden.

Belege am gebauten Binary (Linux 17195235 Bytes):

- Container, Alpine, eigenes /etc/passwd und /etc/shadow mit $6$-Hash:
  `echo falsch | akmesh login tester` -> Exit 1, "incorrect name or password",
  Laufzeit rund 2 Sekunden (gemessen: 2,594 s fuer beide Laeufe zusammen).
  `echo geheim | akmesh login tester` -> Exit 0, Shell startet als tester,
  ~/.profile schreibt /home/tester/proof mit "profil von tester".
- Echtes Terminal (pty): das Passwort wird nicht gespiegelt (Echo aus), danach
  startet die Sitzung.
- Tests: 12 Faelle in internal/login (Pruefwerte, unbekannte Formate, richtiges
  und falsches Passwort, Sperre, Ablauf, kein Echo-Pfad im Test, Hilfe).
- Gatter: go test gruen, Hilfe-Audit Exit 0, Batterie 211 Checks 0 Fehler,
  Windows meldet "login: not supported on Windows".

## sh: zwei Reste aus dem Login (23.11.2026, acmesh-7tx)

Beim Login auf einem Terminal fielen zwei Fehler auf, beide behoben:

- `sh -l` war auf einem Terminal nicht interaktiv: die Startdateien liefen mit
  der Sitzungs-Eingabe als stdin, und ein Profil, das stdin liest, leerte sie.
  Jetzt bekommt die Shell bei interaktivem Aufruf von Anfang an einen leeren
  Leser; die Zeilenschleife liest die Roh-Eingabe.
- Ein Profil, das PS1 auf "\[..\]\u@\h:\w\$" setzt, wurde woertlich angezeigt.
  expandPrompt setzt jetzt \u, \h, \w, \W, \$, \n und \\ um und entfernt die
  \[..\]-Marker. Vorher: "\\033]0;golang \\w\\a\\h:\\w\\ngolang \\$". Nachher:
  "033]0;golang /tmpaBSNB67:/tmp\ngolang #".

## Systempaket 2: ip und dhclient (23.11.2026, acmesh-iey)

ip ist mit 26 Aufrufstellen der zweithaeufigste fehlende Befehl gewesen
(ip link set, ip addr add, ip route add in /etc/init.d und /lib/rc), dhclient
fehlte fuer Adressen und /etc/resolv.conf im leeren Image. Beide kommen jetzt aus
u-root mit, als Linux-only markiert (`linux: ip dhclient` in scripts/cmds.txt).

Bauaenderung: `ip` liegt in u-root unter cmds/core, `dhclient` unter cmds/exp.
`scripts/build-bb.sh` sucht ein cmds.txt-Namenspaket jetzt in core und dann in
exp und bricht mit einer Meldung ab, wenn der Name in keinem von beiden steht -
vorher wurde blind `./cmds/core/$c` gebaut.

Groesse (gemessen):

Gemessen mit der `report:`-Zeile von scripts/build-bb.sh:

- Linux dynamisch: 17.5 MiB (18338019 Bytes), dynamically linked, stripped.
  Vorher 17195235 Bytes, also +1142784 Bytes (1,09 MiB); das ist der Preis von
  netlink.
- Linux statisch (CGO_ENABLED=0): 17.4 MiB (18276476 Bytes), statically linked,
  stripped. Nur dieser Bau laeuft im leeren Image.
- Windows: 16.4 MiB (17200128 Bytes), Windows PE, stripped; unveraendert, `ip`
  und `dhclient` fehlen dort (`./akmesh.exe gibtsnicht` nennt keinen der beiden
  Namen).

Belege im leeren Image (localhost/local/empty, --privileged, /proc eingebunden,
sonst keine Werkzeuge):

    ip link show                       -> lo und eth0 mit MTU, Zustand, MAC
    ip addr show lo                    -> 127.0.0.1/8, ::1/128

und in einem Lauf, also mit bestehendem Netns (die Befehle der Skripte):

    ip addr add 10.42.0.2/24 dev eth0   -> Exit 0, Adresse erscheint in addr show
    ip route add 10.99.0.0/16 dev eth0  -> Exit 0, Route erscheint in route show
    ip link set eth0 down / up          -> beide Exit 0

dhclient:

- startet, zaehlt die Schnittstellen auf, versucht DHCPv4 und DHCPv6
  (`dhclient -timeout 3 lo` im Container).
- schreibt /etc/resolv.conf: die Pfadkonstante ist ResolvConfPath =
  "/etc/resolv.conf", geschrieben in pkg/dhclient.WriteDNSSettings (0o644).
- Echter Lease, gemessen in einem eigenen podman-Netz (10.89.5.0/24,
  --disable-dns, dnsmasq als DHCP-Server in einem Nebenkontainer):

      /akmesh dhclient eth0   (im leeren Image, --cap-add NET_ADMIN NET_RAW)
      -> "Configured eth0 with IPv4 DHCP Lease IP 10.89.5.127/24"
      -> ip addr show eth0: inet 10.89.5.127/24
      -> /etc/resolv.conf:  "nameserver 10.89.5.2"

  Damit ist auch der resolv.conf-Teil von Stufe 3 im leeren Image belegt.
- Befund 1: ohne Lease endet dhclient mit Exit 0. Ein Skript, das auf den
  Exitcode schaut, haelt das fuer Erfolg.
- Befund 2: nach einem Lease laeuft dhclient weiter (Erneuerung, wie dhclient -d)
  und endet nicht von selbst; `-timeout 10` hat ihn nicht beendet, erst ein
  aeusseres `timeout 12` tat es. Ein Skript, das dhclient direkt aufruft,
  blockiert also. Beide Befunde in acmesh-er3.

Weitere Abweichung gegen iproute2: u-root druckt bei fehlender Angabe `<nil>`,
zum Beispiel `brd <nil>` in addr show und `src <nil>` in route show.

Gatter: go build und go vet gruen, `internal/cmdlist` gruen (ip und dhclient in
der Gruppe Network), Hilfe-Audit Exit 0 (`ip -h` und `dhclient -h` antworten),
Batterie 211 Checks 0 Fehler.

## u-root-Abweichungen behoben: ip und dhclient (23.11.2026, acmesh-er3)

Drei Abweichungen, alle als Patches in scripts/patch-uroot.sh (gleiche zwei
Waechter wie bei chmod: neuer Text schon da -> Schritt faellt aus, alter Text
fehlt -> Bau bricht ab).

1. ip druckte fehlende Angaben als `<nil>`: `brd <nil>` in addr show,
   `src <nil>` in route show und `via <nil>` bei einer Default-Route ohne
   Gateway. Jetzt entfaellt das Feld wie bei iproute2.
2. dhclient endete ohne Lease mit Exit 0. Jetzt Exit 2, wie GNU dhclient
   ("no working leases"). Gemessen im leeren Image ohne DHCP-Server:

       /akmesh dhclient -4 -timeout 3 -retry 1 eth0
       -> "Could not configure eth0 for IPv4: ... no matching response packet"
       -> rc=2

3. dhclient kannte -4 und -6 nicht (iproute2 schon), und ein Aufruf ohne sie
   wartete zusaetzlich auf DHCPv6, was wie ein Haenger aussah. Beide Flags sind
   jetzt da; mit -4 kehrt der Aufruf zurueck, sobald der IPv4-Lease steht.

Beleg mit echtem Lease (eigenes podman-Netz, dnsmasq als Server):

    /akmesh dhclient -4 eth0   -> rc=0, "Configured eth0 with IPv4 DHCP Lease
                                  IP 10.89.5.102/24"
    /akmesh ip addr show eth0  -> inet 10.89.5.102/24, kein <nil> in der Ausgabe
    /akmesh cat /etc/resolv.conf -> nameserver 10.89.5.2

Der gesamte Containeraluf (dhclient, ip, cat) brauchte 3,5 s.

Nebenbei richtiggestellt: dhclient liegt in u-root unter cmds/core, nicht in
cmds/exp. Die Suche in beiden Verzeichnissen bleibt trotzdem nuetzlich.

Gatter: Linux 17.4 MiB (18276476 Bytes) statisch und gestrippt, Windows 16.4 MiB
(17200128 Bytes) gestrippt, Batterie 211 Checks 0 Fehler, Hilfe-Audit Exit 0.

## sh: read in der Sitzung und $0 (23.11.2026, acmesh-51b)

Beide Reste aus acmesh-4g7 sind behoben.

1. `read` las in der Sitzung nichts. Die Ursache lag nicht im Interpreter, sondern
   im Wrapper: mvdan/sh gibt jedem stdin, der keine *os.File ist, eine Pipe mit
   einem io.Copy-Goroutinen-Helfer (interp/stdin_os.go, newStdinFile). Der Copier
   liest voraus und nimmt der Zeilenschleife die Zeilen. Im Bau war das
   unsichtbar, weil die Schleife selbst gepuffert las.
   Jetzt: bei einem Datei-Deskriptor bekommt die Shell denselben Leser, und die
   Schleife liest ungepuffert Byte fuer Byte (lineReader in internal/sh/sh.go).
   Bei einem echten Terminal oder einer Pipe ist stdin ein *os.File, also der
   Normalfall; ein Nicht-Datei-Leser (Tests) bekommt weiterhin einen leeren
   Leser, sonst liest der Copier die Sitzung leer.

2. `$0` war immer "gosh": mvdan/sh setzt das fest (interp/vars.go, Fall "0").
   Deshalb ein Fork als Patch, nach dem Muster des Go.Sed-Forks: die Aenderung
   liegt in docs/design/patches/sh-dollar-zero.patch, scripts/apply-sh-fork.sh
   spielt sie idempotent in den Modul-Speicher, keine Kopie von Fremdcode.
   Die Shell liest $0 jetzt aus der Umgebungsvariablen "0", die sh.RunAs setzt:
   der Aufrufname, im Login-Fall mit fuehrendem Minus, wie ash es macht.

Belege am gebauten Binary (Linux statisch, 17.4 MiB, gestrippt):

    akmesh sh -c 'echo "0=$0"'          -> 0=sh
    akmesh sh -l -c 'echo "0=$0"'       -> 0=-sh
    printf 'hallo welt\n' | akmesh sh -c 'read a b; echo "A=$a B=$b"'
                                        -> A=hallo B=welt
    pty-Sitzung: "read x", dann "hallo du", dann "echo X=$x"
                                        -> X=hallo du, kein Zeilenverlust,
                                           danach weiter Prompt "# "
    Container mit login -f tester       -> 0=-sh pwd=/home/tester

Tests: TestDollarZeroIsTheInvokedName (sh und -sh) und TestReadReadsInTheSession
(Pipe als *os.File) neu; 16 Tests in internal/sh gruen.

## ps: -u, --ppid, -C und eigene Spaltenkoepfe (23.11.2026, acmesh-rmx)

Neu in internal/proc: Auswahl nach Nutzer (-u/--user, Name oder numerische uid),
nach Elternprozess (--ppid PID[,PID]), nach Kommandonamen (-C NAME, exakt wie
procps, auch als Liste) und eigene Spaltenkoepfe in -o/-eo (feld=KOPF, feld=
laesst den Kopf weg). Mehrere Auswaehler werden mit AND verknuepft; -e/-A
gewinnt gegen einen Filter, wie procps es macht ("ps -e -u root" listet alles).

Dabei aufgefallen und mit korrigiert: die CMD-Spalte der Standardansicht zeigte
die ganze Befehlszeile. procps zeigt dort den Kommandonamen ("sh"), die volle
Zeile nur unter -f, -o cmd und in aux. akmesh folgt jetzt procps.

Vergleich im gepinnten Referenz-Image (debian:stable-slim, procps-ng 4.0.4),
beide mit --pid=host auf demselben /proc:

    Aufruf           procps  akmesh
    ps -u root          39      39      Zeilen ohne Kopf
    ps --ppid 1          5       5
    ps -C sh             4       4
    ps -C nosuchproc     0       0
    Koepfe: "PID TTY TIME CMD" und "PID MEINKOPF" bei beiden gleich benannt

Zwei bewusst gemessene Unterschiede: die Spaltenbreiten sind bei uns enger
(procps polstert PID und TIME breiter), und fuer eine tty, deren Geraet im
Container fehlt, druckt procps "?", wir "pts/0".

Tests: TestSelectByUserPpidAndComm (Name, uid, --ppid, -C, AND-Verknuepfung,
-e gewinnt) und TestCustomHeaderFromO (Kopf ersetzen, Kopf weglassen).

## mknod, blkid und losetup wie BusyBox (23.11.2026, acmesh-u38)

Drei Abweichungen aus acmesh-hdb, alle behoben. Gemessen gegen BusyBox 1.37.0 in
Alpine, auf derselben Maschine, im selben Lauf:

    mknod /tmp/x c 1 3     busybox: crw-r--r--   akmesh vorher: crw-r-----
                           akmesh jetzt: crw-r--r--
    losetup -f             busybox: /dev/loop0   akmesh vorher: Exit 2
                           akmesh jetzt: /dev/loop0
    blkid /dev/loop0       busybox: /dev/loop0: UUID="11112222-3333-4444-5555-666677778888" TYPE="ext4"
    (8 MiB ext4, mkfs.ext4)akmesh jetzt: dieselbe Zeile

Was dafuer gemacht wurde:

- mknod: Patch in scripts/patch-uroot.sh, 0660 -> 0666 (BusyBox nimmt 0666 ohne
  Umask weg); ein Geraeteknoten ist damit auch fuer Nicht-root lesbar.
- losetup: Patch ebenso, -f druckt das erste freie Loop-Geraet. Den Weg dorthin
  gibt es in u-root schon (pkg/mount/loop.FindDevice, das der Ein-Argument-Aufruf
  nutzt); es fehlte nur der Schalter.
- blkid: eigener Befehl in internal/blkid statt des u-root-Stubs. Der Stub druckte
  den Pfad und nur dann UUID und TYPE, wenn pkg/mount/block sie zufaellig kannte -
  auf einem gewoehnlichen Geraet also nie. Der eigene Leser prueft die Superblocks
  von ext2/ext3/ext4 (Magic 0xEF53, Unterscheidung ueber EXTENTS/64BIT/HAS_JOURNAL),
  XFS ("XFSB"), vfat (FAT12/16/32-Signatur, Seriennummer als 1234-ABCD) und swap
  ("SWAPSPACE2"), druckt Pfad: UUID="..." TYPE="..." und laesst unbekannte
  Dateisysteme als nackten Pfad stehen. Ein nicht lesbares Geraet gibt Exit 2,
  wie BusyBox. Ohne Argument listet er /sys/class/block.
  Windows meldet ehrlich "not supported on Windows".

Tests: sieben Faelle in internal/blkid (ext2, ext3, ext4, xfs, vfat, swap,
unbekannt, Zeilenformat, Exit 2 bei fehlendem Geraet) mit kuenstlichen
Superblocks; kein Loop-Geraet und kein Root noetig.

Groesse: Linux 17.4 MiB (18284668 Bytes) statisch, gestrippt; Windows 16.4 MiB
(17211392 Bytes), gestrippt. Batterie 211 Checks 0 Fehler, Hilfe-Audit Exit 0.

## cron: gibt es das in unseren Upstreams? (23.11.2026, Vorarbeit zu acmesh-8v3)

Geprueft im gepinnten u-root-Checkout (869f00f): `find . -iname '*cron*'` liefert
nichts, kein Treffer auf "crontab" im ganzen Baum - u-root hat keinen cron.
qiangli/coreutils, das wir fuer einzelne coreutils-Schalter nutzen, ist eine
coreutils-Portierung ohne cron. In go.mod und im Modulcache steht nichts mit cron.

Supercronic (aptible/supercronic, der Container-Cron, den Martin meinte):
Go, MIT, `go 1.27.3`. Brauchbare Pakete: `cronexpr` (Ausdruecke parsen, auch mit
Sekundenaufloesung) und `crontab` (crontab-Dateien lesen). Das Paket `cron` ist
kein allgemeiner Scheduler, sondern ein Laeufer an `crontab.Context` gebunden.

Gemessen in einem eigenen Proberaum, jeweils statisch mit `-s -w` (Bytes):

    leeres Programm mit fmt.Println           1523872   (Grundlast)
    + robfig/cron v3 (MIT, keine Transitives) 1708192   (+184320)
    + supercronic/cronexpr                    1912992   (+389120, zieht logrus)
    + supercronic/cronexpr und crontab        2228384   (+704512)

Dazu in der Modulgraphen von supercronic: prometheus, sentry, fsnotify - sie
landen im Bau der Pakete cronexpr und crontab nicht im Binary (nur logrus),
sie sind aber im Modulgraphen.

## Gruppen im Bau ausschliessen: exclude-group (23.11.2026, acmesh-17s)

Die Befehlsliste hatte schon Gruppen (Documents, Files, System, Network, Shell,
Other) und Profile kannten `exclude` und `keep` je Name. Neu: eine ganze Gruppe
laesst sich in einer Zeile streichen.

    # scripts/profiles/services-off.txt
    exclude-group services

Die Namen dafuer stehen nicht im Shell-Skript, sondern kommen aus der Tabelle
selbst: `internal/cmdlist/cmdlisttool <id>` druckt sie (Namen aus
`internal/cmdlist.cmdlist.go`), `scripts/build-bb.sh` zieht sie von `NAMES` und
`OURS_ALL` ab. Eine unbekannte ID bricht den Bau. Die Dienste (heute `login`,
spaeter getty, crond, syslogd/logger/logread) haben dafuer eine eigene Gruppe
`Services` neben `System`.

Gemessen:

    go run ./internal/cmdlist/cmdlisttool services      -> login
    go run ./internal/cmdlist/cmdlisttool dienste       -> unbekannte Gruppe, Exit 2
    ./scripts/build-bb.sh linux services-off
      -> "profile services-off: excluding groups services"
      -> report: akmesh-services-off is 17.4 MiB (18284668 bytes), statically linked, stripped
    ./akmesh-services-off login    -> command is not present in akmesh: login
    ./akmesh-services-off          -> Ueberschriften ohne "Services"
    ./akmesh                       -> " - login" steht in der Liste
    ./akmesh                       -> Batterie 211 Checks, 0 Fehler
    akmesh.exe                     -> 109 Befehle (unveraendert), login meldet
                                      "not supported on Windows"

Die Groesse bleibt gleich, weil `login` im Verhaeltnis zum Binary klein ist; die
Wirkung ist die fehlende Strecke und die fehlende Zeile in der Liste, nicht die
Groesse. Die vollen Groessen: Linux 17.4 MiB (18284668 Bytes) statisch, Windows
16.4 MiB (17211392 Bytes), beide gestrippt.

## Image-Bau: scripts/make-image.sh (23.11.2026, acmesh-35q)

Ein Skript baut das Image, damit das Probe-Image nicht mehr von Hand entsteht.
Statisch bauen, Symlink-Baum, tar, `podman import` - kein Dockerfile, weil die
Befehlsliste dann ein zweites Mal dastehen wuerde.

Eine Quelle fuer die Namen: das Binary druckt seine Befehle, wenn man es mit
einem unbekannten Namen aufruft. Das Skript liest genau diese Liste (Waechter:
bricht ab, wenn weniger als 50 Namen kommen) und legt fuer jeden einen Symlink
auf /bin/akmesh. Ein neuer Befehl ist damit beim naechsten Lauf im Baum.

Im Image: /bin/akmesh, 116 Symlinks, /etc/passwd, /etc/group, /etc/hosts,
/etc/resolv.conf, /etc/profile, /etc/ssl/certs/ca-certificates.crt vom Host
(oder eine Warnung), /tmp mit 1777, leere /proc /sys /dev /root /home. /dev und
/proc fuellt die Laufzeit - mdev und udev gibt es in diesem Binary nicht. /run
legt der Runtime an.

Abnahme aus dem Ticket, woertlich:

    $ podman run --rm akmesh:latest sh -c 'ls /; printf "b\na\n" | sort'
    bin dev etc home proc root run sys tmp
    a
    b

Keine Bind-Mounts, kein PATH-Gebastel: PATH=/bin steht als ENV im Image,
CMD ist /bin/sh.

Weitere Pruefungen aus dem Skript (alle Exit 0): uname -s -> Linux,
head -1 /etc/passwd -> root:x:0:0:root:/root:/bin/sh, date -> 2026-11-23,
awk -> 2, grep -c -> 1, `ps -e | wc -l` -> 4 (also sieht ps den eigenen Prozess).

Zahlen:

    Image                18.6 MB (localhost/akmesh:latest)
    Binary im Image      17.4 MiB (18284668 Bytes), statisch, gestrippt
    Symlinks in /bin     116
    5 Container-Starts   400 ms je Start (Runtime enthalten, Wandeuhr, bash EPOCHREALTIME)
    50 x /bin/true      16.45 ms je Ausfuehrung in EINEM Container
                        (Seiten des 17.4-MiB-Binaries kommen dazu)

Methode der beiden Zeiten: `EPOCHREALTIME` aus bash um die podman-Aufrufe; ohne
bash ueberspringt das Skript die Messung, weil `date +%s%N` hier nur Sekunden
gibt (busybox) und die frueheren "0 ms" genau daran lagen.

## Batterie gegen akmesh.exe (23.11.2026, acmesh-696)

Die Batterie lief erstmals gegen das Windows-Binary, aus der WSL-Shell, ohne
wine:

    ./scripts/test-akmesh.sh ./akmesh.exe    -> 189 checks, 0 failures
    ./scripts/test-akmesh.sh ./akmesh        -> 211 checks, 0 failures

Der Unterschied sind 22 Checks, die fuer .exe uebersprungen werden. Um das nicht
zu schaetzen, wurde eine Kopie des Skripts instrumentiert (jeder Check schreibt
seinen Namen in eine Datei), beide Ziele liefen und die Namenslisten wurden
verglichen. Klassen der uebersprungenen Checks:

- Rechte-Bits (6): chmod-plusx-bit, chmod-recursive-mode, chmod-verbose,
  cp-archive-mode, stat-mode, touch-stamp-year. Windows hat ACLs statt
  POSIX-Modi; `stat -c %a` und `ls -ld` zeigen dort nichts Vergleichbares.
- Landlock-Policy (13): policy-active, -read-inside, -write-inside,
  -write-only-inside-allowed, -blocks-read, -blocks-write, -blocks-network,
  -no-outside-file, -write-only-outside-denied, -check-denied, -shows-source,
  -network-message, -broken-refused. Statt ihrer laeuft auf Windows ein Check,
  der prueft, dass die Policy ehrlich abgelehnt wird (policy-refused).
- tracedeps (3 Linux, 1 Windows): ptrace gibt es nur unter Linux/amd64; auf
  Windows prueft tracedeps-says-no-windows die ehrliche Meldung.
- sh-pipe (1): der Check pipet sort, uniq und wc ueber PATH; ein
  Windows-Prozess wuerde dort die Windows-Werkzeuge nehmen. Absolut aufrufen
  geht nicht, weil ein .exe die POSIX-Pfade der Batterie nicht aufloest.

Nebenbefund, behoben: scripts/test-akmesh.sh hatte keinen Shebang. Aus einer
Shell gestartet ging es trotzdem (die Shell faellt bei ENOEXEC auf sh zurueck),
aber `timeout ./scripts/test-akmesh.sh ...` endete mit "Exec format error".
Jetzt steht `#!/bin/sh` in Zeile 1, und der Aufruf ueber timeout laeuft
(211 checks, 0 failures).

Zaehlweise: 211 ist die Zahl der Check-Aufrufe, policy-check-allowed steht
zweimal im Skript (zwei Zustaende, ein Name), deshalb 210 verschiedene Namen.

Kein Produktdefekt gefunden: alle Abweichungen sind Plattformgrenzen, die das
Skript schon benennt. Fuer Windows sind es 109 statt 116 Befehle (dhclient, ip,
losetup, mknod, mount, strace, umount fehlen), und login/blkid melden dort
ehrlich "not supported".

## init als PID 1 (23.11.2026, acmesh-b97)

Eigener Befehl internal/akinit (kein u-root-Fork: dessen init startet eine Liste
einmal, erntet nur beim eigenen Ende und ignoriert SIGTERM wie jeder PID 1).
Inittab im BusyBox-Format `id:runlevels:action:command` mit den Aktionen
sysinit, wait, once, respawn, shutdown; unbekannte Aktion oder leeres Kommando
brechen mit Zeilennummer ab. Reaping per SIGCHLD + Wait4(-1, WNOHANG), Signale
werden an die Prozessgruppe der Kinder weitergegeben, nach 3 s Grace SIGKILL,
respawn mit wachsender Pause (100 ms bis 5 s), damit ein sterbender Dienst keine
Fork-Bombe wird. Ohne /etc/inittab laeuft der uebergebene Befehl einmal und sein
Status ist der Exit-Code.

Beweis im Image (static build, 117 Namen, /etc/inittab per Bind-Mount):

    podman run -d -v inittab:/etc/inittab:ro akmesh:latest /bin/init
    /proc/1/comm                        -> init
    ps -e                               -> 1 init, sh, ps, sh, sleep; keine Z-Zeile
    Zombies (Z-Zeilen)                  -> 0          (drei once-Dienste endeten vorher)
    respawn-Zaehler /tmp/runs           -> 5 Laeufe in 3 s bei sleep 0.3
    podman logs                         -> init-start, "exited with 0" je Dienst, init-stop
    podman stop -t 10 (unser init)      -> 167 ms, Container-Exit 0
    podman stop -t 10 (busybox sleep    -> 10133 ms, Container-Exit 137
      als PID 1, ohne init)

Die letzte Zeile ist der Punkt des Tickets: ohne Init laeuft jeder Stop in die
10-Sekunden-Frist und endet mit SIGKILL; mit init ist es ein sauberes Ende in
unter einer Sekunde (Vorgabe war "unter einer Sekunde").

Tests (7, alle gruen): Inittab-Parser samt Abweisungen, Exit-Code-Durchreichung,
sysinit vor den Diensten, respawn startet neu, drei endende Kinder hinterlassen
keine Z-Zeile (direkt aus /proc gezaehlt), Backoff wartet bei Dauersterben.

Groessen: Linux dynamisch 17.5 MiB (18374883 Bytes), statisch 17.5 MiB
(18313340 Bytes), Windows 16.4 MiB (17213440 Bytes), alle gestrippt. Windows
meldet ehrlich "not supported on Windows (no PID 1; services belong to the SCM)".

Batterie 211 Checks 0 Fehler, Hilfe-Audit Exit 0, init erscheint in der Gruppe
Services.

## crond: Zeitjobs im Container (23.11.2026, acmesh-qgf)

Eigener Befehl internal/crond. Ausdruecke kommen von robfig/cron v3 (MIT, keine
Transitives), alles andere ist unser Code: crontab-Dateien, Umgebung, Neustart per
SIGHUP, Jobs starten und ihre Ausgabe.

Format: Vixie/BusyBox - fuenf Zeitfelder, dann das Kommando, dazu @hourly,
@daily, @weekly, @monthly, @yearly, @reboot und @every DAUER. Kein Sekundenfeld:
"0 0 * * * 5 echo x" laesst sich nicht von einer Zeile mit Sekundenfeld
unterscheiden, und Raten verschiebt Jobs stillschweigend. Subminuetig geht ueber
"@every 10s", das der Ausdrucksparser versteht.

Verhalten: Umgebung der crontab wird an den Job vererbt (auch das, was supercronic
auszeichnet), Ausgabe geht unveraendert nach stdout/stderr, je Job eine Zeile
"start" mit Zeit und Kommando und "end" mit Exit-Code. Ein Job, der noch laeuft,
wird nicht doppelt gestartet. Ein falscher Ausdruck wird mit Datei und Zeile
gemeldet und nicht still uebergangen. Jobs laufen durch unsere eigene Shell
(<binary> sh -c ...), damit dieselbe crontab auch unter Windows gilt, wo es kein
/bin/sh gibt. SIGTERM stoppt die Jobs (Prozessgruppe, 2 s Gnade), SIGHUP liest
die Dateien neu - auch eine neu abgelegte Datei in /etc/cron.d.

Gemessen im Image (akmesh:latest, 118 Namen):

    crond: 3 job(s) from /etc/crontab and /etc/cron.d
    /tmp/ticks nach 13 s        reboot, tick, tick   (@reboot einmal, @every 5s zweimal)
    Log                         "start echo tick >> /tmp/ticks" / "end ... (exit 0)"
    Datei job2 nach dem Start   + SIGHUP -> "crond: reloading", "3 job(s) after reload",
                                Zaehler nachgeladen 2 -> 4
    podman stop -t 10           154 ms, Container-Exit 0

Beim ersten Anlauf war der Reload blind: die Dateiliste wurde beim Start
festgehalten, eine neue Datei in /etc/cron.d wurde nie gesehen. Behoben (Dateien
und Verzeichnis getrennt, Verzeichnis bei jedem Reload neu gelesen) und mit
TestReloadSeesANewFileInTheDirectory festgenagelt; ein doppelter Pfad (-c zeigt
in dasselbe Verzeichnis) wird entdoppelt, sonst liefe jeder Job zweimal.

Tests (9): Parser mit Kommentaren, Variablen, Descriptors und Zeilennummern;
Abweisungen; Trennung von Zeit und Kommando; Exit-Code und Start/End-Marker;
vererbte Umgebung; falscher Ausdruck mit Zeile; @every wird terminiert; cron.d
ueberspringt Punktdateien und Sicherungen; Reload sieht neue Dateien.

Groessen: Linux statisch 17.5 MiB (18382972 Bytes), das sind +69632 Bytes gegen
den Bau ohne crond (18313340); Windows 16.5 MiB (17310720 Bytes), +97280 Bytes.
Beide gestrippt. Lizenzpruefung: 20 Module, alle verwendbar.

## syslogd, logger, logread (23.11.2026, acmesh-g7n)

Drei Befehle, eine Umsetzung (internal/syslog, je Befehl ein duennes Paket und
cmd-Einstieg). Format RFC 3164, wie BusyBox und jeder /dev/log-Schreiber es
erzeugt. syslogd legt die Zeile so ab, wie sie ankam - was logread zeigt, ist
das, was das Programm geschickt hat.

- syslogd: unixgram-Socket (Standard /dev/log, 0666), Datei -O (Standard
  /var/log/messages), Rotation bei -s Bytes nach NAME.0 (eine Sicherung, wie
  BusyBox' -s), uebriggebliebene Socket-Datei wird entfernt, aber nur wenn sie
  wirklich ein Socket ist; SIGTERM raeumt Socket und Datei auf.
- logger: -t TAG, -p FACILITY.LEVEL (auch Level allein oder Zahl), -s (zusaetzlich
  stderr), -S SOCKET; ohne Argument gehen die Zeilen von stdin raus, eine je
  Zeile.
- logread: -O FILE, -n ANZAHL, -f folgt; mit -f kommt erst der Bestand, dann die
  neuen Zeilen - das ist, was BusyBox' logread -f tut.
- Windows: kein /dev/log, also keine Socket-Fassade. logger schreibt mit -O FILE
  direkt in eine lokale Datei, logread liest sie, syslogd meldet ehrlich
  "not supported on Windows (no /dev/log; use logger -O FILE)". Nichts wird beim
  Host registriert.

Beweis im Image:

    /bin/syslogd -O /tmp/log/messages -s 2048 & ... logger -t probe -p local0.info "hallo welt"
    logread -O /tmp/log/messages
    <134>Nov 23 12:29:52 probe[20]: hallo welt
    <27>Nov 23 12:29:52 probe[25]: fehler text      (daemon.err = 3*8+3)

    Rotation, 120 Zeilen bei -s 2048:  messages 912 Bytes, messages.0 2033 Bytes
    (je Rotation eine Sicherung), logread -n 1 zeigt die letzte Zeile.

    logread -f:  erst "alt  vor dem folgen", dann "live  waehrend des folgens"
    - also Bestand plus Nachlauf, wie bei tail -f.

    Unter init (Inittab: syslogd und ein Ticker als respawn): ps zeigt init,
    syslogd und den Ticker, logread -n 3 gibt drei ticker-Zeilen im 2-Sekunden-Takt,
    podman stop -t 10: 210 ms, Container-Exit 0.

    Windows: akmesh.exe logger -O /tmp/winlog.txt -t wintest -p local0.warning
    "hallo von windows" -> <132>Nov 23 13:32:26 wintest[10028]: hallo von windows,
    logread liest sie, syslogd verweigert, logger ohne -O verweigert mit Hinweis.

Bewusste Abweichung zu BusyBox: logread liest die Datei, nicht den
Shared-Memory-Ring. BusyBox' Ring lebt nur, solange syslogd laeuft; eine Datei
ueberlebt einen Neustart und ist mit jedem Werkzeug lesbar.

Tests (7): Prioritaeten (Namen, Zahlen, Fehlerfaelle), Format (Tag, PID,
Leerzeichen vor Tageszahl, kein Umbruch in der Zeile), normalizeLine, Rotation
(Datei bleibt unter der Grenze, Inhalt in .0), logger ueber einen echten Socket
(Prioritaet, Tag, PID im Datagramm), logger liest stdin zeilenweise, logread -n.

Groessen: Linux statisch 17.6 MiB (18423932 Bytes, +40960 gegen den Bau mit crond
18382972), dynamisch 17.6 MiB (18485475), Windows 16.5 MiB (17335808, +25088).
Alle gestrippt. Befehle: Linux 121, Windows 114. Batterie 211/0, Hilfe-Audit 0,
Lizenzpruefung 20/20.

## Aufpasser-Logging (23.11.2026, acmesh-17s)

internal/supervise haengt sich zwischen Dienst und seine Ausgabe: es reicht jede
Zeile unveraendert an das eigene stdout bzw. stderr weiter (podman logs und die
Konsole sehen alles) und schickt sie zusaetzlich nach syslog. stdout geht als
info (PRI 14), stderr als err (PRI 11) raus, damit logread Daten nicht als Fehler
zeigt. Tag ist das Programm des Dienstes: kind.sh, ticker, selbst.

Drei Modi, geschaltet mit AKMESH_SUPERVISE: dup (Standard, durchreichen UND
weiterleiten), forward (nur weiterleiten), pass (nur durchreichen, auf den echten
Deskriptoren, nichts wird gelesen). Ein unbekannter Wert ist ein Fehler und kein
stilles Standardverhalten. init und crond nutzen denselben Code, jedes Kind hat
eine eigene Zeile, kein Kind muss etwas dafuer tun - auch fremde Binaries sind
abgedeckt.

Regeln aus dem Ticket, alle gehalten: eine Roehre, die nicht mehr abnimmt, wirft
Zeilen weg und zaehlt sie, das Kind laeuft weiter (gemessene Grenze unten);
Teilzeilen am Ende kommen raus; Zeilen ueber 4096 Bytes werden gekuerzt und mit
"(gekuerzt)" markiert; der Exit-Code des Kindes bleibt, was er war; ein Kind, das
selbst per logger schreibt, ergibt genau eine syslog-Zeile.

Beweis im Image (Inittab: syslogd, ein Kind mit 2x stdout und 3x stderr, ein
logger-Kind, ein Kind mit Exit 7):

    podman logs        eins / zwei-auf-stderr / drei / vier-auf-stderr /
                       fuenf-auf-stderr   (alle fuenf durchgereicht)
    logread            <14> kind.sh: eins
                       <14> kind.sh: drei
                       <11> kind.sh: zwei-auf-stderr
                       <11> kind.sh: vier-auf-stderr
                       <11> kind.sh: fuenf-auf-stderr
                       <13> selbst: durch logger        (genau einmal)
    init-Log           init: /bin/sh -c 'sleep 1; exit 7' exited with 7
    podman stop -t 10  219 ms / 462 ms in zwei Laeufen, Container-Exit 0

Grenze bei totem Socket, gemessen im Test (Socket aufgemacht und nie gelesen,
500 Zeilen): 278 Zeilen nahm der Socket-Puffer noch an, 217 wurden verworfen und
gezaehlt, das Kind lief in 0 ms durch. Der Aufpasser blockiert also nie, er
verliert im Extremfall Zeilen - und sagt es.

Zwei Fehler beim Beweis gefunden und behoben:
1. Der Sink wurde beim Start von init angelegt; /dev/log gab es da noch nicht,
   also wurde gar nicht weitergeleitet. Jetzt wird die Verbindung beim ersten
   Satz aufgebaut.
2. Danach fehlte die allererste Zeile des ersten Dienstes, weil syslogd noch
   startete. Jetzt versucht der Sink es fuenfmal im Abstand von 100 ms, bevor er
   eine Zeile aufgibt.

Der Preis steht im Ticket und hier: ein Kind sieht eine Roehre statt eines
Terminals, manche Programme schalten dann Farbe ab oder puffern anders. Dafuer
gibt es den Modus pass.

Tests (8): Dup reicht durch und leitet weiter (2x info, 3x err), forward bleibt
still, Teilzeile kommt raus, lange Zeile wird gekuerzt, Exit-Code, voller Sink
blockiert nicht, toter Socket blockiert nicht, AKMESH_SUPERVISE wird gelesen und
ein Tippfehler abgelehnt.

Groessen: Linux statisch 17.6 MiB (18440316 Bytes, +16384 gegen g7n 18423932),
Windows 16.5 MiB (17348608, +12800), gestrippt. Batterie 211/0, Hilfe-Audit 0,
Lizenzpruefung 20/20, 121 Befehle Linux / 114 Windows.

## getty: Login auf der Konsole (23.11.2026, acmesh-bu1)

internal/getty oeffnet das Terminal, druckt /etc/issue, fragt nach dem Namen und
uebergibt die Sitzung an internal/login im selben Prozess (kein exec auf
/bin/login, damit dieselbe Policy gilt und das Image keine zweite Datei braucht).
Eine Sitzung pro Lauf, wie agetty und BusyBox: nach dem Logout endet getty, und
init startet es per respawn neu. Eine getty, die in sich selbst schleift, wuerde
ein kaputtes Terminal ewig behalten.

Issue-Escapes: \n Hostname, \l Terminalname, \t Uhrzeit, \d Datum, \\ Backslash;
ein unbekannter Escape bleibt sichtbar stehen, damit ein Tippfehler auffaellt.
Ohne -f wird /etc/issue gelesen, ein fehlendes Issue ist kein Fehler. -t TTY oeffnet
ein Geraet (Fehler, wenn es nicht aufgeht - kein stiller Rueckfall auf stdin);
ohne -t wird stdin genommen, wenn es ein Terminal ist.

Beweis im Image (Testkonto root mit openssl passwd -6 gehasht, /etc/passwd und
/etc/shadow per Bind-Mount):

    printf 'root\ngeheim\nid -un; pwd\nexit\n' | podman run -i -t --rm ... /bin/getty
    17415ab10872 (akmesh) tty        <- Issue aus dem Image, \n und \l ersetzt
    17415ab10872 login: root
    Password:                        <- kein Echo
    # root
    /root

    falsches Passwort:
    Password:
    login: incorrect name or password
    logout (exit 1)

    unter init mit 'g::respawn:/bin/getty' und stdin /dev/null (Terminal sofort zu):
    init-Log zeigt fuenfmal "/bin/getty exited with 0" in 3 Sekunden, also
    startet init getty nach jedem Ende neu (mit Backoff).

Windows: "getty: not supported on Windows (no console login to own)" - eine
Konsolenanmeldung gehoert dem Sitzungsmanager des Hosts, und den fasst dieses
Projekt nicht an.

Tests (4): Issue-Escapes inkl. unbekanntem Escape, Frage nach dem Namen mit
verworfener Leerzeile und Uebergabe an login, Ende bei geschlossenem Terminal mit
Exit 0, nicht oeffenbares TTY ist ein Fehler.

Im Image neu: /etc/issue mit "\n (akmesh) \l", damit ein Login-Prompt zeigt, wo
man ist.

Groessen: Linux statisch 17.6 MiB (18448508 Bytes, +8192 gegen 17s 18440316),
Windows 16.5 MiB (17351168, +2560), gestrippt. Befehle: 122 Linux / 115 Windows.
Batterie 211/0, Hilfe-Audit 0, Lizenzpruefung 20/20.

## Image mit trivy gescannt (23.11.2026)

Werkzeug: trivy 0.68.2 lokal auf Alpine 3.16. Vorgehen, weil das Image nur ein
statisches Go-Binary ohne Distribution ist: `podman save` in eine tar, dann
`trivy image --input`. Ein Scan der Verzeichnisse waere dasselbe Ergebnis, die
tar zeigt aber genau das, was podman gespeichert hat. Das Skript dafuer ist
scripts/scan-image.sh (exportiert, scannt, unterdrueckt anhand .trivyignore).

Erster Lauf, Stand d813888 (go1.27.1, x/crypto v0.58.0):

    Total: 20 (UNKNOWN: 1, LOW: 1, MEDIUM: 11, HIGH: 7, CRITICAL: 0)
    stdlib go1.27.1   CVE-2026-111943 crypto/x509 MITM (Fix 1.27.3)
                      CVE-2026-97032, CVE-2026-97054 net/http DoS (Fix 1.27.4)
                      CVE-2026-56857 os: Windows junction traversal (Fix 1.27.2)
    golang.org/x/net  CVE-2026-78661 (Fix 0.62.0)
    golang.org/x/crypto GO-2026-5932 openpgp (unmaintained; Fix-Pseudoversion)
    u-root            CVE-2020-7665, CVE-2020-7669

Behoben:

- `go mod edit -go=1.27.4` und neu gebaut: `go version -m akmesh` sagt
  go1.27.4. Das raeumt alle stdlib-CVEs ab (7 HIGH und 11 MEDIUM waren
  ueberwiegend net/http, x509, os).
- golang.org/x/net auf v0.62.0, golang.org/x/crypto auf
  v0.58.1-0.20261009181718 (die im Advisory genannte Fixversion). openpgp wird
  nicht importiert (wir nutzen cryptobyte, chacha20, chacha20poly1305), die
  Aenderung ist trotzdem gratis.

Zweiter Lauf, gleiches Image neu gebaut:

    Total: 3 (UNKNOWN: 1, HIGH: 2)  -> nach .trivyignore: Total 0, unterdrueckt 2

Die zwei u-root-CVEs sind ein Fehlalarm und stehen mit Begruendung in
.trivyignore: u-root ist an Commit 869f00f6 (17.11.2026) gepinnt, die CVEs sind
in 0.9.0 (2021) behoben. Go schreibt fuer einen lokalen Ersatz aber "v0.0.0", und
diese Version kann trivy nicht einordnen - deshalb meldet es die CVEs fuer jeden
so gebauten Binary. Geprueft mit `--show-suppressed`: es sind genau diese zwei.

Secrets: sauber (--scanners secret, 0 Funde). Groesse unveraendert 17.6 MiB
statisch (18464892 Bytes); go1.27.4 kostet nichts, x/crypto ebenso wenig.
Windows neu gebaut mit demselben Toolchain: 16.6 MiB (17370112 Bytes).

## Bedarfsmessung: welche Werkzeuge fehlen wirklich (23.11.2026, acmesh-y2o)

Gemessen, nicht geschaetzt. Quellen und Umfang, damit die Zahlen einordbar sind:

- Hermes-Sitzungen mit cwd /root/akmesh: 6 Sitzungen, 3210 Nachrichten mit
  Werkzeugaufrufen, davon 2532 terminal-Aufrufe (state.db, nur gelesen).
- Shell-Historie des Hosts: 17589 Zeilen.
- Skripte und Messungen dieses Repos.
- Vergleichsliste: BusyBox 1.37.0, 405 Applets (`--entrypoint /bin/busybox --list`).

Zaehlweise: jeder Aufruf wird an &&, ||, ;, |, $() und ` zerlegt, Heredoc-Koerper
werden vorher entfernt (sonst zaehlt Python- und Go-Code als Befehl), Env-Zuweisungen
und sudo/doas/time/nohup/exec/command abgestreift, nur Namen gezaehlt, die entweder
im PATH liegen oder zu akmesh gehoeren. Shell-Builtins (cd, echo, printf, export,
exit, ...) fallen raus. Ergebnis: 104 verschiedene Befehle, 10224 Vorkommen.

Haeufig gebraucht und vorhanden (Vorkommen, Agent + Historie zusammen):

    head 1727  grep 1422  tail 1180  sed 488  ls 384  cat 360  wc 141
    tr 140    ps 114    cp 112    sleep 111  sh 110  timeout 100
    awk 95    rm 83

Damit ist der Kern bestaetigt: die Pipeline-Werkzeuge sind die, die wirklich laufen.

Fehlend, aber gebraucht - und in BusyBox-Klasse (Kandidaten fuer akmesh):

    Befehl      Agent  Historie  Aufwand  Weg
    od             29        0    M        eigener Bau (hexdump existiert, anderes Format)
    pgrep          16        0    S-M      eigener Bau (internal/proc ist da)
    unzip          10       56    M        eigener Bau (archive/zip)
    openssl        10        1    L        bewusst nicht; nur "passwd -6" ist der Bedarf
    chown           8        5    M        eigener Bau (Login liest passwd/group)
    netstat         5        0    S        u-root cmds/core, nur aufnehmen
    pkill           5        1    S        mit pgrep zusammen
    su              3       11    M        ueber internal/login
    perl            3        0    L        bewusst nicht (Sprache, kein Werkzeug)
    bash           52        3    L        bewusst nicht: GPL, wir haben sh (POSIX)
    passwd          2        0    M        ueber vorhandene sha512-crypt-Krypto
    udhcpc          2        0    -        bewusst nicht, wir haben dhclient
    dpkg/apk        2/3   0/26    -        bewusst nicht: Paketmanager der Distribution
    readelf         2        0    S        u-root cmds/exp, nur aufnehmen
    mkfs.ext4       2        0    L        bewusst nicht (Dateisystem anlegen ist nicht Agentenalltag)
    setsid          1        0    S        Syscall-Aufruf
    sum             1        0    S        kleine Pruefsumme

Fehlend, aber Host-Werkzeug, nicht Shell-Userland - bewusst nicht nachbauen:

    git 923+573, bd 553+32, go 417+438, python3 378, podman 308, gofmt 256,
    busybox 32, trivy 2.

Ein Harness, das akmesh statt einer echten Shell gibt, muss fuer diese Werkzeuge
entweder einen zweiten Weg anbieten oder die Aufgabe ablehnen - sie gehoeren nicht
in die Binary.

Gegen BusyBox gehalten: 91 Namen gemeinsam, 314 BusyBox-Applets fehlen, 31 Befehle
hat akmesh zusaetzlich (die Dokument-Konverter, jq, rg, curl, wget, policy, verify).
"91 gemeinsam" heisst nicht "91 gleich": die Semantik weicht ab (u-root find kennt
kein -maxdepth, head kein -3), das steht in den jeweiligen Abschnitten.

Vorhanden, aber nie gebraucht: 55 Befehle kamen in keiner der beiden Quellen vor.
Nach Pruefung sind davon 14 keine echten Kandidaten (echo, pwd, true, false, kill,
mv, ping, scp, tee, mktemp, wget, jq, yes, df sind Builtins oder anderswo benutzt),
und die 12 Dokument-Konverter plus die fuenf Dienste (init, syslogd, logger, logread,
policy) bleiben, weil ihr Zweck ausserhalb einer Shell-Sitzung liegt. Streichkandidaten
mit Zahl statt Gefuehl: gosh, man, more, shuf, verify (Ticket acmesh-kpp).

## Gegenprobe im Image: wo der Agent ausweicht

Neun Aufgaben im gebauten Image, jede mit dem Befehl, den ein Agent tippen wuerde:

    Aufgabe                              Ergebnis
    Binaerdaten ansehen (od)             od fehlt -> hexdump -C zeigt die Bytes, anderes Layout
    Prozess nach Namen (pgrep)           fehlt -> ps -e -o pid,comm | grep, funktioniert
    Prozess beenden (pkill, kill)        kill <pid> im sh: "kill: unsupported builtin", Exit 2,
                                         Prozess lebt weiter. /bin/kill <pid> geht (rc=0)
    Hintergrund-PID ($!)                 echo $! gibt "g1" statt einer PID (mvdan/sh-Job-ID)
    Rechte aendern (chown)               fehlt, kein Ersatz; chmod geht
    Zip entpacken (unzip)                fehlt, kein Ersatz
    Passwort-Hash (openssl passwd -6)    fehlt, kein Ersatz
    Benutzerwechsel (su)                 fehlt -> login -f root funktioniert
    Rekursive Suche (grep -r)            funktioniert
    Groesse (du -sh)                     "du: unknown option -sh"; du -s -h liefert 248K

Damit sind drei echte Defekte belegt, die keine Bedarfsfrage sind, sondern Fehler:
kill im sh (acmesh-oxw), $! gibt eine Job-ID (acmesh-tms), du ohne kombinierte kurze
Flags (acmesh-7hg). Die uebrigen Luecken sind Tickets der Klasse S bis M.

Grenzen der Messung: sechs Sitzungen sind ein Ausschnitt, die Host-Historie ist nicht
die Zielumgebung, und die Aufnahme eines u-root-Befehls ist noch kein Beweis, dass er
sich wie BusyBox verhaelt. Die Zahlen sagen, wo nachzusehen ist - nicht, dass ein
Befehl deswegen fehlen muss.

## Drei Defekte aus der Bedarfsmessung behoben (23.11.2026)

Alle drei kamen aus der Gegenprobe im Image (acmesh-y2o), nicht aus einer
Vermutung. Jeder hat einen Test, der vorher fehlschlug.

### kill im sh (acmesh-oxw)

mvdan/sh fuehrt kill in seiner Builtin-Liste und antwortet dann "unsupported
builtin"; weil es als Builtin gilt, sucht die Shell auch /bin/kill nicht. Ein
Skript konnte also gar keinen Prozess beenden. Jetzt beantwortet
internal/sh/kill_unix.go den Aufruf im Prozess: -s, -n, -SIGNAME, -9, -0, --,
mehrere PIDs, kill -l mit und ohne Nummer, ESRCH-Meldung wie ash. Unter Windows
sagt es "kill: not supported on Windows (no POSIX signals)".

Beim Testen fiel ein eigener Fehler auf: "kill -0 PID" wurde als negatives
PID-Argument gelesen, also als Prozessgruppe, und raeumte die Testgruppe mit
SIGTERM ab. In kill bedeutet ein fuehrender Strich immer ein Signal; -0 ist
Signal 0. Behoben, vier Tests.

### $! liefert die PID (acmesh-tms)

mvdan/sh vergibt virtuelle Jobs, $! antwortete "g1". Der Fork-Patch
docs/design/patches/sh-bg-pid.patch traegt die Hintergrund-Anweisung ueber den
Context zum Exec-Handler, der die echte PID nach dem Start in eine geteilte Zelle
schreibt; $! wartet kurz darauf, damit "sleep 30 & echo $!" auch ohne sleep
dazwischen stimmt. Zwei Fehler dabei gefunden: erst ging die PID verloren, weil
bgProc als Wert in die Slice kopiert wird (jetzt Zeiger auf die Zelle), dann war
$! zu frueh dran (jetzt Warten auf pidReady).

### du mit kombinierten kurzen Flags (acmesh-7hg)

internal/disk bekommt splitShortFlags: -sh wird wie bei getopt zu -s -h, -d2 zu
-d 2, lange Optionen und "-" bleiben unangetastet. Auch fuer df und free
eingehaengt, weil dort dieselbe Klasse zuschlaegt. Zwei Tests.

### Belege

Am Binary auf dem Host:

    ./akmesh sh -c 'sleep 30 & echo "pid=[$!]"'      pid=[1395474]
    ./akmesh sh -c 'sleep 30 & p=$!; sleep 0.2; kill $p'   kill ok, Prozess weg
    ./akmesh sh -c 'kill -0 $$ && echo lebt'          lebt
    ./akmesh du -sh /etc                              3.1M
    ./akmesh du -s -h /etc                            3.1M   (vorher: unknown option "-sh")
    ./akmesh du -s /etc                               3217   (1K-Bloecke wie BusyBox)

Im gebauten Image (akmesh:latest):

    $! = 7                    (PID im Container-Namensraum)
    kill $! -> kill ok        und danach "ist weg"
    kill -0 $$ -> lebt
    kill -9 $! -> sigkill ok
    kill -l                   alle 31 Signale, je 8 pro Zeile
    du -sh /etc -> 248K, du -s -h -> 248K, du -s -> 248

Windows (akmesh.exe): "kill: not supported on Windows (no POSIX signals)", der
Bau ist gruen, die Batterie laeuft.

### Zwei Umgebungsfehler, die dabei auffielen

- Der Projekt-GOCACHE hatte einen kaputten Eintrag: "could not import
  sync/atomic (open : no such file or directory)". `go clean -cache` reparieren,
  dann baut dieselbe Quelle normal.
- Der Go.Sed-Modulcache stand in einem Mischzustand (alte Datei regexp_akmesh.go
  neben dem aktuellen Patch, Folge: "longestMatcher redeclared"). Modulverzeichnis
  loeschen, `go mod download`, Patch einmal anwenden - danach ist die Suite gruen
  (build-rc 0, alle Pakete ok).

Groessen: dynamisch 18538723 Bytes, statisch 18477180 (+12288 gegen 18464892),
Windows 17375232 (+4096 gegen 17370112); alle gestrippt. 122 Befehle.

## pgrep, pkill und killall (23.11.2026, acmesh-2qr)

Bedarfsmessung: pgrep 16 Aufrufe, pkill 5. killall gab es schon (internal/proc),
aber ohne -w und -l. Neu in internal/proc/pgrep.go, alle drei teilen die
vorhandene /proc-Auswertung aus ps.go (acmesh-mq1: ein Prozessmodell).

Was gebaut ist

- pgrep: -f (ganze Kommandozeile), -x (Muster muss den Namen ganz abdecken),
  -u (Name oder Zahl), -P (Eltern-PID), -n/-o (jüngster/ältester Treffer),
  -l, -a, -c, -d SEP. Muster ist ein regulaerer Ausdruck, Exit 1 heisst "nichts
  gefunden".
- pkill: dieselben Suchschalter, Signal als -SIGNAL, -s SIGNAL oder -9,
  Vorgabe TERM. Der aufrufende Prozess wird nie signalisiert.
- killall: -w wartet, bis der Prozess wirklich weg ist (Grenze 30 s, damit kein
  Agent haengt), -l listet die Signalnamen wie kill -l.

Drei Entscheidungen, die von procps abweichen und warum

- "pgrep -P 123" und "pgrep -u root" ohne Muster gelten und listen alles, was
  der Filter trifft; ein nacktes "pgrep" bleibt ein Fehler (Exit 2). Grund: das
  sind die Formen, die Agents schreiben, und ein Fehler dort kostet mehr als
  eine breitere Antwort.
- -x sieht durch die Endung .exe, weil Windows den Prozess akmesh.exe nennt und
  der Agent akmesh tippt (wie killall es schon machte).
- -u loest eine Zahl ueber die Benutzertabelle auf: pgrep -u 0 und pgrep -u root
  sind dasselbe. Belegt: Host 25 Prozesse fuer beide, im Image 2 fuer beide.

Probe im Image (akmesh:latest, 124 Namen in /bin)

    pgrep -P $$              -> 7 8 (ohne Muster, vorher Fehlermeldung)
    pgrep -u 0 / -u root     -> je 2
    pgrep                    -> "no pattern given", Exit 2
    sleep 30 & pgrep -f "sleep 30"   -> drei Treffer (auch die Shells, deren
                              Kommandozeile das Muster enthaelt - das ist -f)
    pgrep -l -x sleep        -> 32 sleep
    pkill -x sleep           -> Exit 0, danach findet pgrep nichts mehr
    pkill -9 -f "sleep 31"   -> Exit 0
    killall -w sleep         -> Exit 0, danach 0 Prozesse
    killall slee             -> "no process found", Exit 1 (Teiltreffer trifft nicht)

Gefahr, die dokumentiert ist: -f trifft jede Kommandozeile, die das Muster
enthaelt, also auch die Shell, die pkill aufruft. Beim Bau ist genau das
passiert: ein Testlauf mit "pkill -9 -f ..." hat die eigene Prozessgruppe
abgeraeumt. pkill ueberspringt nur sich selbst (wie procps). Die Hilfe von
pkill sagt das jetzt und raet, vorher mit denselben Schaltern pgrep zu benutzen.

Zwei Fallen in der Batterie (beide behoben)

- Unter Windows startet die Batterie eine Kopie namens acmesh-under-test.exe;
  eine Pruefung auf "pgrep -x akmesh" kann dort nicht treffen. Der Name kommt
  jetzt aus $BIN.
- Windows-Ausgaben enden auf CRLF; zwei Pruefungen brauchten tr -d '\r'.

Groessen: dynamisch 18559203 (+20480), statisch 18497660 (+20480), Windows
17398272 (+23040), alle gestrippt. 124 Befehle Linux und Windows.
Batterie 219 Checks Linux, 197 Windows, beide ohne Fehler; Hilfe-Audit 0;
Lizenzen 20/20.

## Fuenf Luecken aus der Bedarfsmessung geschlossen (23.11.2026)

Alle in einem Zug gebaut, jeder mit eigenen Tests. Die Pruefungen liefen am Ende
gegen beide Ziele und im Image.

### acmesh-vgr: netstat, readelf, setsid, sum

netstat (cmds/core) und readelf (cmds/exp) kommen aus u-root, Aufnahme kostet nur
die Zeile in scripts/cmds.txt; netstat ist linux: nur, weil es die Kernel-Tabellen
liest.
setsid ist eigen (internal/setsid): SysProcAttr{Setsid:true}, Exitcode wird
durchgereicht; unter Windows die ehrliche Absage, weil dort ein Prozess nicht mit
seiner Konsole stirbt.
sum ist eigen (internal/bsdsum): BSD-Rotationssumme als Vorgabe, System-V mit -s.
Die Zahlen sind gegen BusyBox 1.37.0 festgeschrieben: "hello
" ergibt 36979 im
BSD-Modus und 542 mit -s, "abc" 16556 und 294.
readelf zeigt eine Besonderheit von u-root: es druckt JSON und kennt -h/-a nicht
(Usage of readelf: bei -h). Das ist eine Upstream-Eigenschaft, keine Luecke hier.

### acmesh-6p8: chown

-R, -h (Symlink selbst), -v, -c, Benutzer und Gruppe als Name oder Zahl, Formen
user:group, user.group, :group. Namen kommen aus /etc/passwd und /etc/group.
Windows: "not supported (no POSIX ownership; access is controlled by ACLs)".
Belegt im Image: chown 12345:12345 setzt beide Zahlen, chown -R :777 setzt nur die
Gruppe und laesst den Besitzer stehen.

### acmesh-2yc: od

-A (d/o/x/n), -t (a, c, dN, oN, uN, xN, f4/f8), -N, -j, -v, Sprungzeile als *.
Zwei eigene Fehler hat der Test gefunden: Hex und Oktal waren nicht mit Nullen
aufgefuellt ("a" statt "0a") und die Adresse hatte Leerzeichen statt Nullen.
Nach dem Fix ist die Ausgabe zeichengenau wie GNU:

    printf "hello\n" | od         0000000 062550 066154 005157 / 0000006
    printf "AB" | od -An -tx1      41 42

### acmesh-f1e: unzip

-l, -d, -o, -q, -p, einzelne Member. Zip-Slip wird entschaerft ("../" wird
entfernt), Rechte und Zeiten bleiben erhalten, verschluesselte Eintraege werden
gemeldet statt halb ausgepackt. Belegt im Image: Liste zeigt Groesse, Datum und
Name, Auspacken liefert "hi" aus hi.txt.

### acmesh-a4d: su, passwd, mkpasswd

su prueft das Kennwort ueber den vorhandenen login-Code, wechselt die IDs und
kann -c; der Exitcode kommt durch (su -c "exit 5" -> 5, su -c "id -u" root -> 0).
passwd fragt zweimal, hasht mit SHA-512 und ersetzt nur das Hash-Feld der
Shadow-Zeile; die neue Datei entsteht daneben und wird umbenannt, damit kein
halbes Shadow-File entstehen kann. mkpasswd erzeugt $6$ und $5$.

Der Hash ist gegen openssl geprueft, zeichengenau:

    mkpasswd -m sha-512 -s salz geheim  =  openssl passwd -6 -salt salz geheim
    mkpasswd -m sha-256 -s salz geheim  =  openssl passwd -5 -salt salz geheim

Zwei Fehler unterwegs, beide behoben und mit Test festgehalten:

- passwd las die beiden Zeilen mit zwei bufio-Readern; bei einer Pipe frass der
  erste die zweite Zeile und passwd endete mit "passwd: EOF". Jetzt ein Reader
  fuer beides, und auf einem Terminal term.ReadPassword.
- Das Image hatte gar kein /etc/shadow. login konnte dort nie pruefen und passwd
  nie schreiben. scripts/make-image.sh legt die Datei jetzt an, Modus 0400, root
  gesperrt ("!"), ein echter Hash kommt ueber AKMESH_ROOT_HASH (z.B. aus
  "akmesh mkpasswd -m sha-512"). Belegt im Image: Datei da, passwd root setzt
  "$6$...", falsches Kennwort ergibt "login: incorrect name or password".

Bekannte Grenze: speist man login ueber eine Pipe, kann der Kennwort-Leser die
naechste Zeile schon gepuffert haben; am Terminal (getty) tritt das nicht auf.

Groessen, alle gestrippt: dynamisch 18956515, statisch 18894972, Windows 17589248.
134 Namen im Image (vorher 124). Batterie 232 Checks Linux, 210 Windows, beide
ohne Fehler; 13 Pruefungen sind neu. Hilfe-Audit 0, Lizenzen 20/20.

## Abgleich gegen GNU: 179 Aufrufe, 67 Abweichungen (23.11.2026)

Methode: dieselben Aufrufe im gepinnten Referenz-Image
(debian:stable-slim @ sha256:cd9f2189b62dc5ce003b22ad8cb109bbc3cef8ddcc8bc13771472a6edeb16980,
coreutils 9.7, sed 4.9, grep 3.11, tar 1.35) und gegen ./akmesh, gleiches
Verzeichnis, gleiche Testdateien; verglichen Exitcode, stdout und ob stderr leer
ist. Sonde: scripts/deviation-probe.py, Testdaten erzeugt sie selbst.
Ergebnis: 132 Aufrufe im ersten und zweiten Durchgang, 47 im dritten, insgesamt
179; davon 67 abweichend. Zeitstempel-Unterschiede (Container UTC, Host CET) sind
abgezogen.

Ohne Befund: head (alle geprueften Formen), sort (11 Formen), cut (7), tr (5),
sed (10), cp/mv/rm/mkdir/ln/chmod/touch (Ergebnisbaum), stat, xargs, timeout,
split, nl, truncate, iconv, grep (-c -l -n -v -o -E -i -r -A -B -q -F -w -h).

Mit Befund, aufgeschluesselt nach Werkzeug und Ticket: docs/abweichungen-gnu.md.

## tail in den GNU-Formen (23.11.2026, acmesh-uzd)

Der Befund aus docs/abweichungen-gnu.md: tail -3, -1, -c, -q und -v endeten mit
"flag provided but not defined", weil u-root tail die Go-flag-Bibliothek benutzt
und Go -3 nicht als Zahl liest.

Gefixt als Patch gegen u-root: docs/design/patches/uroot-tail-gnu.patch, 376
Zeilen, angewendet von scripts/patch-uroot.sh (neue Funktion patch_diff, die
einen Diff idempotent anwendet). Die Lese-Logik von u-root bleibt unberuehrt; neu
sind ein eigener Argumentparser (cmds/core/tail/tail_args.go) und drei Zweige in
tail(): Bytes, von-vorne und die bisherige Zeilenlogik.

Was jetzt geht, gemessen gegen GNU coreutils 9.7 im Referenz-Image, 13 Faelle,
12 vorher abweichend, jetzt alle gleich:

    tail -3 a.txt              alpha beta gamma
    tail -1 a.txt              gamma
    tail -n 3 / -n2            wie GNU
    tail -n +2 a.txt           ab Zeile 2
    tail -c 5 a.txt            letzte 5 Bytes
    tail -c +12 a.txt          ab Byte 12
    tail -q -2 a b             ohne Kopfzeilen
    tail -v -1 a.txt           mit Kopfzeile
    tail -n 0 a.txt            nichts
    tail a.txt b.log           Kopfzeilen wie GNU
    tail -n -2 a.txt           wie GNU: die letzten zwei Zeilen

Eine Eigenheit, die bewusst mitgemacht wird: GNU liest eine negative Zahl wie
eine positive, "tail --lines=-2" heisst also die letzten zwei Zeilen, nicht
"alles ausser den letzten zwei". Akmesh macht das genauso, weil ein Agent, der
die Zahl negativ schreibt, die letzten Zeilen meint.

u-root bringt eigene Tests mit (tail_test.go); sie laufen mit dem Patch weiter
("ok github.com/u-root/u-root/cmds/core/tail"). Dafuer bleibt die alte
run()-Signatur als duenne Huelle erhalten.

Batterie: 238 Checks Linux (vorher 232, sechs neue tail-Pruefungen), 0 Fehler.
Groessen: dynamisch 18964707, Windows 17598976, beide gestrippt.

## date in den GNU-Formen (23.11.2026, acmesh-xli)

Befund: date -d, -I und --iso-8601 endeten mit "flag provided but not defined: -d",
und der Formatierungsmap von u-root fehlte %s ganz ("date +%s" druckte "%s"), %m
hatte keine fuehrende Null.

Gefixt als Patch: docs/design/patches/uroot-date-gnu.patch (338 Zeilen), ueber
scripts/patch-uroot.sh angewendet wie beim tail. Neu: cmds/core/date/date_parse.go
mit einem Parser fuer die Formen, die in Skripten vorkommen.

Was jetzt geht, gemessen gegen GNU coreutils 9.7 (22 Faelle, alle gleich):

    date -d @0 +%s                    0
    date -d 2020-01-02 +%s            1577923200
    date -d 2020-01-02T03:04:05Z +%s  1577934245
    date -d "2020-01-02 03:04" +%s    1577934240
    date -d "2 days ago" / "+3 hours" / "last week" / "next monday"
    date -I, date -Iseconds, --iso-8601=seconds
    date -R                           Mon, 02 Jan 2006 15:04:05 +0000
    date +%s, +%N, +%j, +%m/%d/%Y     wie GNU

Was bewusst nicht geht: die ganze GNU-Sprache ("third friday", "2 fortnights").
Jede nicht verstandene Zeichenkette endet mit "date: invalid date ..." und Exit
1 - eine falsche Zeit waere schlimmer als eine ehrliche Absage.

Zwei Eigenheiten, die dabei mit repariert wurden: %s fehlte (ohne %s ist -d fuer
Zeitstempel nutzlos) und %m gab "1" statt "01", was "%Y-%m-%d" zu "1970-1-01"
machte.

Ein Fehler, den die Batterie gefunden hat: --help endete bei tail mit Exit 1,
weil der eigene Argumentparser das Flag nicht kannte und das Hilfe-Gate jede
Zeile ohne usage-Antwort ablehnt. Beides ist jetzt drin (eigener usage-Text,
Exit 0), und das Gate ist wieder gruen.

Batterie: 245 Checks Linux, 223 Windows (Stand vor dem letzten Windows-Lauf),
0 Fehler; sieben neue date-Pruefungen. Groessen: dynamisch 18972899, Windows
17610240, beide gestrippt.

## grep -P ueber eine PCRE2-Portierung (23.11.2026, acmesh-b87)

Entscheidung Martins: die treuere Engine, nicht die kleinere.

Neu: internal/pcre mit Compile, Match, FindAll, hinter der genau eine
Abhaengigkeit steht (github.com/go-regexp/pcre2 v0.1.0, BSD-3).
internal/forked/grep kennt jetzt -P/--perl-regexp; die Scan-Schleife spricht ein
matcher-Interface, das beide Engines erfuellen, damit der Unterschied eine Zeile
bleibt. -i und -w wirken auch unter -P (sie setzen (?i) und \b(?:...)\b vor das
Muster).

Sicherungen in internal/pcre, weil eine Shell, die haengt, schlimmer ist als eine,
die ablehnt: matchLimit 10.000.000 Schritte, depthLimit 10.000, heapLimit
20.000.000. Ueberschreitung endet mit "match limit exceeded" und Exit 2, nicht mit
einer Endlosschleife. Nachgemessen: (a+)+$ gegen 40 a und ein b endet nach 0,35 s
mit dem Limitfehler.

Gemessen gegen GNU grep -P (coreutils/grep 3.11 im Referenz-Image), neun Faelle:
sieben gleich, zwei mit Begruendung:

    \p{Greek}     GNU ohne Locale rc 1, mit LANG=C.UTF-8 rc 0; akmesh verhaelt
                   sich immer wie UTF-8, weil es keinen Locale-Begriff hat.
                   \p{L} stimmt in beiden Faellen.
    (?=x) ohne -P  GNU liest BRE (Literal), akmesh ERE und meldet einen Fehler.
                   Siehe eigenes Ticket acmesh-4gi, das ist der groessere Fund.

Der zweite Fund wiegt schwerer als der Anlass: beim Pruefen fiel auf, dass unser
grep das Standardmuster als ERE liest, GNU aber als BRE. "grep 'a+b'" trifft bei
uns "aab", bei GNU nichts; "grep 'a\(a\)b'" trifft bei GNU, bei uns nichts.
Ticket acmesh-4gi mit beiden Messungen.

Groesse: dynamisch 18972899 -> 19456227 Bytes (+483.328, 2,5 Prozent).
Windows: 18095104 Bytes. Beide gestrippt. Der Zuwachs ist groesser als die
208 KiB aus der Einzelmessung, weil dort der Linker ungenutzte Teile wegwarf;
gemessen ist der echte Binary-Zuwachs.

Batterie: 250 Checks Linux, 209 Windows, sieben neue grep--P-Pruefungen.

## grep: Standardmuster ist BRE wie bei GNU (23.11.2026, acmesh-4gi)

Der Fund kam beim Pruefen von -P: unser grep uebersetzte jedes Muster als ERE,
GNU liest ohne -E eine BRE. Gemessen vorher und nachher, gleiche Fixture:

    Muster          vorher (akmesh)      GNU / jetzt
    a+b             Treffer "aab"        nichts, Exit 1
    a\+b            nichts               "aab"
    a\(a\)b        nichts               "aab"
    a\|b            nichts               beide Zeilen
    a\{2\}         nichts               "aab"
    (               Fehler, Exit 2       Literal, Exit 1
    [               Fehler, Exit 2       Fehler, Exit 2

Umgesetzt in internal/forked/grep/bre.go: eine Umschrift auf dem Mustertext,
rund 80 Zeilen. Jedes Muster wird einzeln umgeschrieben, dann mit | verbunden -
in GNU ist jedes -e sein eigenes BRE. -E und -P bleiben unangetastet, -F ohnehin.

Regeln der Umschrift: \( \) \| \+ \? \{ \} werden zu ihren ERE-Zeichen,
nackte ( ) | + ? { } werden maskiert, \< und \> werden zu \b (GNU-Wortgrenzen),
ein fuehrender * wird maskiert, Zeichenklassen werden unveraendert kopiert.

Gegen GNU im Referenz-Image: 12 Aufrufe, alle gleich (a+b, a\+b, a\(a\)b,
a\|b, a\{2\}, (, [, \<axb\>, dazu -E, -F, -P und -c auf denselben Fixture).

Batterie: sieben neue BRE-Pruefungen; Go-Tests decken die Umschrift-Tabelle und
die gemessenen Faelle ab. Groesse: 19460323 Bytes dynamisch (vorher 19456227,
also 4096 mehr fuer die Umschrift).

## grep: die sieben uebrigen Schalter (23.11.2026, acmesh-v6r)

Nach -P und der BRE-Umschrift fehlten noch -m, -x, -H, -a, -z sowie --include und
--exclude. Alle sieben sind jetzt drin, alle in internal/forked/grep.

    -m N, -mN       nach N Treffern je Datei aufhoeren (nicht je Aufruf)
    -x              das Muster muss die ganze Zeile treffen (^(?:...)$)
    -H              Dateiname immer mitschreiben
    -a, --text      Binaerdateien als Text lesen
    -z, --null-data Datensaetze sind NUL-getrennt, Ausgabe ebenso
    --include=GLOB  nur Dateien mit passendem Namen durchsuchen
    --exclude=GLOB  solche Dateien auslassen

Beim Bauen fiel auf, dass die Scan-Schleife fuer -z und -m umgebaut werden musste:
aus dem Schleifenkoerper wurde eine Funktion handle(record, num), die sagt, ob die
Datei zu Ende ist. -z liest die Eingabe ganz und trennt an NUL, weil ein
bufio.Scanner immer an Zeilenumbruechen trennt. --include/--exclude greifen im
Walk, also nur fuer Dateien aus einem Verzeichnis; eine auf der Kommandozeile
genannte Datei wird immer durchsucht, wie bei GNU.

Gegen GNU im Referenz-Image, zehn Aufrufe: neun gleich, einer nicht vergleichbar
(die Sonde kann Binaerausgabe nicht als UTF-8 lesen). -a von Hand nachgeprueft:
die Ausgabe ist byte-gleich mit GNU.

Ein neuer, kleiner Unterschied bleibt und ist als acmesh-bde notiert: ohne -a
meldet GNU bei Binaerdateien "binary file matches", akmesh gibt die Zeile aus,
weil es keine Binaererkennung hat. Fuer Agenten ist das brauchbarer, aber es ist
eben nicht GNU.

Batterie: sieben neue Pruefungen (264 Linux, 242 Windows). Groesse unveraendert
19460323 Bytes dynamisch, Windows 18102272.

## du: Verzeichnisbloecke, -b und -x (23.11.2026, acmesh-yd7)

Befund: du zaehlte die Bloecke der Verzeichnisse selbst nicht mit (24 statt 32 auf
der Fixture), -b aenderte nur die Einheit und lieferte weiter Bloecke (4096 statt
17), -x fehlte, und Pfade wurden ohne "./" gedruckt.

Ursache im Code: walkUsage summierte nur die Kinder, nie den Ordner selbst;
parseUnitFlags setzte -b auf Einheit 1, was die Messung nicht aendert; -x gab es
nicht.

Gefixt in internal/disk:
  * walkUsage zaehlt jetzt die eigenen Bloecke des Verzeichnisses mit (das sind
    auf den meisten Dateisystemen 4096 Bytes je Ordner).
  * usageOfB trennt die zwei Groessen: scheinbare Groesse mit -b (Verzeichnis
    zaehlt 0), belegte Bloecke sonst.
  * -x/--one-file-system: devOf liefert das Geraet, der Walk springt ueber
    alles, was auf einem anderen Geraet liegt. Auf Windows gibt es das nicht,
    dort ist -x wirkungslos (ehrlich so dokumentiert).
  * duPath verbindet ohne filepath.Join, damit "du ." wie GNU "./sub" druckt.

Gegen GNU du im Referenz-Image, zehn Aufrufe, alle gleich (sortiert verglichen,
weil GNU in Verzeichnisreihenfolge laeuft und wir sortieren):

    du, du -b, du -s, du -c, du -h, du -d1, du -x, du -b -s, du -sh, du -a

Fixture: 5000-Byte-Datei in sub/deep, zwei kleine Dateien, ein Unterordner.
GNU und akmesh: 12 ./sub/deep, 20 ./sub, 32 .; mit -b: 5000, 5002, 5016.

Offen und bewusst: die Reihenfolge der Zeilen ist bei uns sortiert, bei GNU in
Verzeichnisreihenfolge (dieselbe Klasse wie acmesh-kgi bei find).

Batterie: sechs neue Pruefungen (Windows-tauglich, weil sie auf scheinbaren
Groessen rechnen, die auf beiden Zielen gleich sind). Groesse 19468515 Bytes
dynamisch.

## tar: -O, Verzeichnis-Schraegstrich, kein Archiv in sich selbst (23.11.2026, acmesh-vc3, acmesh-9e7)

Vier Sachen waren falsch, alle gemessen gegen GNU tar 1.34 im Referenz-Image:

1. -O fehlte ("unknown option").  Jetzt: -O/--to-stdout schreibt den Inhalt nach
   stdout, mit Namen nur die genannten Dateien. Gegenprobe: gleiche Ausgabe wie
   GNU, auch bei ganzen Verzeichnissen im Archiv.
2. Ein Archiv im gepackten Verzeichnis wurde mitgepackt, waehrend es wuchs: der
   Archivinhalt enthielt sich selbst abgeschnitten, ohne Meldung.
   GNU laesst die Datei aus und sagt "tar: ./g.tar: archive cannot contain itself;
   not dumped", Exit 0. Jetzt genauso, inklusive Namen mit "./".
3. Verzeichnisse wurden ohne Schraegstrich abgelegt ("d" statt "d/"), was
   Werkzeuge verwirrt, die auf "d/" pruefen. Jetzt "d/".
4. "tar -cf x.tar ." schrieb "a.txt", GNU schreibt "./a.txt" (acmesh-9e7). Jetzt
   bleibt die Schreibweise des Arguments erhalten: "." wird "./", "./x" bleibt
   "./x", "dir" bleibt "dir".

Dabei fiel ein fuenfter Fehler auf, der nicht im Ticket stand: ein absoluter
Quellpfad brach mit "Rel: can't make /tmp/d relative to" und Exit 2 ab. GNU
schreibt "tar: Removing leading `/' from member names" und legt "tmp/d/a.txt" an.
Jetzt genauso, Warnung einmal, Exit 0.

Gegen GNU geprueft, GNU liest unsere Archive und wir lesen seine, Listen
byte-gleich:
    d/  d/a.txt  d/s/  d/s/b.txt         identisch in beide Richtungen
    ./  ./a.txt  ./g.tar  ./gnu.txt  ./sub/   identisch
    tar -xO   gleich

Beim Schreiben werden Namen jetzt immer mit "/" als Trenner abgelegt, auch aus
dem Windows-Build, weil "d\a.txt" kein GNU-tar lesen kann.

Batterie: sieben neue tar-Pruefungen (276 Linux, 253 Windows; die Pruefung mit
absolutem Pfad laeuft nur gegen den Linux-Build, weil ein WSL-Pfad fuer die
Windows-Binary kein absoluter Pfad ist). Groesse 19468515 Bytes dynamisch,
Windows 18107392.

Auf Windows fiel dabei noch auf: ein Laufwerksbuchstabe landete im Archivnamen
("C:/Users/..."），jetzt wird er entfernt wie der fuehrende Schraegstrich; und ein
Dateimodus, den Go tar nicht kodieren kann, brach das ganze Archiv ab, jetzt wird
die Datei mit Meldung uebersprungen.

## find: Ausdrucksbaum gegen GNU find 4.9 (23.11.2026, acmesh-x7f, acmesh-kgi)

Der u-root-Fork kannte nur eine flache Liste von Matchern: kein -o, keine
Klammern, keine Negation, kein Ausdrucksbaum. Damit fehlten genau die Formen,
die ein Agent schreibt. Neu gebaut (internal/forked/find): Praezedenz
! > -a > -o, Klammern, Aktionen, Default -print nur bei wahrem Ausdruck.

Neu dazu: -iname, -ipath, -size (c/w/b/k/M/G), -empty, -newer, -mtime, -mmin,
-user, -group, -perm (auch symbolisch wie -u+x), -printf, -delete, -quit,
-exec ... +, -print0, -xdev, -follow.

Gemessen mit scripts/find-probe.sh (beide Seiten durch "sh -c", sortiert
verglichen, weil die Reihenfolge von readdir kommt) auf /tmp/ffx
(a.txt 3 B, empty.txt 0 B, b.log 3 B, sub/c.txt, sub/deep):

    Aufrufe                          31
    gleich                           31
    abweichend                        0

Darunter: "( -name *.txt -o -name *.log ) -size -5c", "! -type d",
"-maxdepth 0" (nur der Startpfad), "-mindepth 1 -maxdepth 1",
"-type f -printf '%p %s\n'", "-exec echo HIT {} ;" (ein Aufruf pro Treffer),
"-perm -u+x", "-user root", "-mmin -1", "-regex", "-print0".

Reihenfolge: GNU gibt die readdir-Reihenfolge aus, nicht sortiert. Der Fork
sortiert nicht mehr, damit "-maxdepth 1" dieselbe Folge liefert (acmesh-kgi).

Fehlerfaelle: unbekanntes Praedikat, fehlender Operand und offene Klammer
enden mit Exit 2 samt Usage-Zeile, wie GNU.

Batterie: sechs neue Pruefungen (find-or, find-not-dirs, find-delete,
find-delete-gone, find-broken-expr, find-unknown-predicate).

## rg: PCRE2 ueber -P und --engine (23.11.2026, acmesh-4v0)

ripgrep nimmt -P/--pcre2 und --engine default|pcre2|auto nur mit dem
Cargo-Feature pcre2. akmesh haengt die Engine an internal/pcre (derselbe
Wrapper wie grep -P, mit Schritt-, Tiefen-, Heap- und Zeitlimit).
Neu: internal/rgcmd/pcrematcher.go (match.Matcher auf PCRE2, NonMatchingLineTerm
false, damit der Sucher zeilenweise arbeitet), --engine in parse.go.

Gemessen mit scripts/rg-probe.sh auf /tmp/ffx:

    rg -P 'a(?=a)' .              -> ./a.txt:aa   Exit 0
    rg -P 'a\Ka' .                -> ./a.txt:aa   Exit 0
    rg -P 'a++' .                 -> ./a.txt:aa   Exit 0
    rg --engine=pcre2 '(?<=b)b' . -> ./b.log:bb   Exit 0
    rg --engine=auto 'a(?=a)' .   -> ./a.txt:aa   Exit 0
    rg -Pw 'aa' .                 -> ./a.txt:aa   Exit 0
    rg 'a(?=a)' .                 -> "invalid or unsupported Perl syntax: `(?=`"  Exit 2
    rg --engine=bogus x .         -> "invalid value \"bogus\" for --engine"       Exit 2

Die Engine laeuft unter einem Mutex: der PCRE-Wrapper haelt einen MatchContext
je Muster, und ripgrep teilt einen Matcher zwischen Walk-Workern. Das ist
korrekt und langsam; gemessen wurde es nicht, weil die Aufrufe oben klein sind.

Batterie: sieben neue Pruefungen (rg-pcre-lookahead, -lookbehind, -auto,
rg-re2-rejects, rg-engine-default-rejects, rg-bad-engine, rg-pcre-word).

## wc: -L und die Spaltenbreite gegen GNU wc 9.7 (23.11.2026, acmesh-j66, acmesh-154)

-L fehlte ganz, und die Zahlen standen ohne Auffuellung nebeneinander. Beides
im eigenen Fork (internal/forked/wc) gebaut.

Die Breite folgt einer Regel, die erst die Messung zeigte: GNU nimmt fuer ALLE
Zahlenspalten eine Breite, und zwar aus dem groessten Wert aller Zaehler
(Zeilen, Woerter, Zeichen, Bytes, laengste Zeile) ueber alle Dateien und die
Summenzeile - nicht nur aus den Spalten, die dieser Aufruf druckt:

    wc -l a.txt z5k.txt     a.txt: 3 Zeilen, 9 Bytes; z5k.txt: 0 Zeilen, 5000 Bytes
    GNU                     "   3 a.txt" und "   0 z5k.txt"  (Breite 4!)

-L zaehlt wie GNU in der UTF-8-Locale: ein Zeichen ist eine Spalte, ein
Tabulator rueckt auf das naechste Vielfache von 8. Gemessen:

    wc -L a.txt       3        (drei Zeilen, laengste 3)
    wc -L tab.txt     9        ("x<TAB>y" = 9 Spalten)
    wc -L u.txt       3        ("äöü"; die C-Locale sagt hier 0)
    printf 'abcde' | wc -L    5   (ohne Zeilenende)
    wc -L a.txt tab.txt u.txt  mit Summenzeile 9

14 Aufrufe gegen GNU verglichen, alle gleich (Breite, Inhalt, Summenzeile,
einfache Datei ohne Auffuellung). Batterie: fuenf Pruefungen (wc-max-line,
wc-longest-from-stdin, wc-columns, wc-total-row, wc-single-no-pad).

## base64: -w0 und -d ohne Auffuellung gegen GNU base64 9.7 (23.11.2026, acmesh-449)

Zwei Stolpersteine, beide gemessen und behoben:

    base64 -w0 b.txt          GNU: kein Umbruch am Ende
    printf aGVsbG8 | base64 -d  GNU: "hello", Exit 0 (ohne "=")

Dazu zwei Regeln, die die Messung nebenbei klaerte:

    Leerzeichen ist Muell, kein Format: "aGVs bG8" -> GNU schreibt "hel" und
    endet mit "invalid input", Exit 1. Nur Zeilenumbruch und CR sind erlaubt.
    Mit -i verschwindet der Muell: "aGVs bG8" -> "hello", Exit 0.
    Eine Gruppe aus drei Zeichen ohne "=" dekodiert auch mit -i (Exit 0).

11 Aufrufe byte-gleich gegen GNU (od -c verglichen). Batterie: fuenf
Pruefungen (base64-w0-no-newline, -decode-unpadded, -decode-unpadded-status,
-decode-space-is-garbage, -decode-ignore-space).

## ls -t: Nanosekunden statt Sekunden gegen GNU ls 9.7 (23.11.2026, acmesh-fgu)

Der Vergleich kuerzte die mtime auf ganze Sekunden, also waren zwei Dateien aus
derselben Sekunde ein Gleichstand und der Name entschied - bei "x.tar, x.tgz,
y.txt" in dieser Sekunde also die falsche Reihenfolge.

Gemessen (GNU ls im Container auf eigenem Dateisystem, dort bleiben die
Nanosekunden erhalten):

    x.tar  .001   x.tgz  .299   y.txt  .605
    GNU ls -t   -> y.txt  x.tgz  x.tar
    vorher      -> x.tar  x.tgz  y.txt   (Name)
    jetzt       -> y.txt  x.tgz  x.tar

Ein echter Gleichstand faellt weiter auf den Namen zurueck. Ein Messversuch
ueber den /tmp-Mount des Containers taugte nicht: dort gehen die Nanosekunden
beim Binden verloren, beide Seiten zeigten dieselbe (falsche) Reihenfolge. Der
Beleg kommt deshalb aus dem Container-Dateisystem selbst und aus
TestSortByTimeComparesNanoseconds.

Batterie: zwei Pruefungen (ls-t-nanoseconds, ls-tr-nanoseconds).

## df: Kopfzeile und Spalten gegen GNU df 9.7 (23.11.2026, acmesh-jag)

Kopfzeile war "1K-blocks ... Avail ..." auch bei -h. GNU nennt die Spalte bei -h
"Size", sonst nach der Einheit, und "Avail" nur bei -h:

    df -h        Filesystem      Size  Used Avail Use% Mounted on
    df           Filesystem      1K-blocks      Used Available Use% Mounted on
    df -P        Filesystem     1024-blocks      Used Available Capacity Mounted on

Dazu die Breiten, die vorher ein starres 24/9-Raster waren: der Name steht
linksbuendig in Breite 14 oder der Breite des laengsten Namens, danach ein
Leerzeichen, und jede Zahlenspalte ist rechtsbuendig auf die Breite ihrer
eigenen Kopfzeile oder ihres laengsten Werts. Damit liest awk dieselbe Spalte
wie bei GNU.

Nicht vergleichbar war die ganze Tabelle: der Container und WSL haben andere
Mounts. Verglichen wurden Kopfzeilen und das Spaltenraster an einzelnen Mounts
(df -h /, df -h /proc, df -h /usr/src), wo beide Seiten dieselbe Zeile zeigen.

Batterie: fuenf Pruefungen (df-h-size-header, df-blocks-header, df-posix-header,
df-posix-capacity, df-fields-in-columns).

## diff -u: Zeitstempel in der Kopfzeile gegen GNU diffutils (23.11.2026, acmesh-5ug)

GNU schreibt hinter jeden Namen in der Kopfzeile einen Tabulator und die mtime:

    --- d1.txt<TAB>2026-11-23 19:44:46.581456399 +0000
    +++ d2.txt<TAB>2026-11-23 19:44:46.785086680 +0000

Nachgemessen bei uns (gleiche Dateien, lokale Zone):

    --- d1.txt<TAB>2026-11-23 20:44:46.581456399 +0100
    +++ d2.txt<TAB>2026-11-23 20:44:46.785086680 +0100

Form, Tabulator und die neun Nachkommastellen sind gleich; nur die Zone
unterscheidet sich, weil der Container in UTC laeuft. Eine Seite, die nur durch
-N entsteht, bekommt wie bei GNU die Epoche (1970-01-01 00:00:00.000000000
+0000). Der Patch selbst bleibt unveraendert; patch -p0 findet die Datei jetzt
auch ohne -i.

Batterie: zwei Pruefungen (diff-u-header-stamp, diff-u-plus-line).

Gesamtstand nach dieser Runde: 308 Pruefungen Linux, 283 Windows, 0 Fehler.

## sed -z: NUL-getrennte Saetze gegen GNU sed 4.9 (23.11.2026, acmesh-d9e)

GNU -z liest Saetze bis zum NUL und schreibt sie mit NUL statt Zeilenumbruch -
die Form, in der ein Agent "find -print0" weiterverarbeitet. Umgesetzt im
Wrapper (internal/sedawk), nicht im sed-Engine-Patch: die Engine liest Zeilen,
also wird der Strom beim Hineingehen uebersetzt (NUL -> Zeilenumbruch, ein
echter Zeilenumbruch im Satz -> 0x01 0x30, das Byte 0x01 selbst -> 0x01 0x31)
und beim Herausgehen zurueck. Die Abbildung ist umkehrbar fuer jede Eingabe.

Grenze, ehrlich: ein Skript, das einen Zeilenumbruch INNERHALB eines NUL-Satzes
mit einem Muster trifft (etwa "s/a\nb/X/"), sieht im Muster die Codierung statt
des Zeilenumbruchs. Gemessen wurde das nicht als Abweichung, weil GNU hier nur
mit einem echten Zeilenumbruch im Muster vergleichbar waere; der Fall steht in
acmesh-d9e als offener Punkt.

Gemessen mit scripts/sed-z-probe.sh (beide Seiten durch sh -c, od -c verglichen),
Eingabe "a.txt\0b c.txt\0d.txt\0":

    s/^/X/ je Satz            gleich
    s/txt/TXT/g je Satz       gleich
    s/a/A/;s/d/D/             gleich
    -zn p                     gleich
    s/ /_/                    gleich
    2d                        gleich
    $s/txt/END/               gleich
    Zeilenumbruch im Satz     gleich
    Byte 0x01 im Satz         gleich

Neun von zehn Aufrufen gleich. Der zehnte ist alt und hat mit -z nichts zu tun:
"sed 's/txt/TXT/'" ohne -z auf Daten mit NUL im Text ersetzt bei uns alle drei
Treffer, GNU nur den ersten, weil GNU die Datei als eine Zeile liest. Als
Befund notiert, nicht als Regression dieser Arbeit.

Batterie: vier Pruefungen (sed-z-prefix, sed-z-count, sed-z-delete,
sed-z-glued). Dazu zwei Go-Tests (TestZeroTerminatedRecords,
TestZeroTerminatedKeepsLineBreakInRecord).

Nachtrag zur Grenze: "$" am Satzende trifft richtig - "sed -z 's/txt$/END/'"
ersetzt in allen drei Saetzen (a.END|b c.END|d.END), wie GNU. Die Umsetzung
haengt also nicht an einem Zeilenumbruch am Satzende.

## grep und Binaerdateien gegen GNU grep 3.11 (23.11.2026, acmesh-bde)

Vorher las unser grep jede Datei als Text. GNU erkennt eine Datei an einem
Nullbyte in den ersten 8 KiB, und dann:

    grep ABC bin1                 stderr "grep: bin1: binary file matches", Exit 0
    grep -c ABC bin1              1            (zaehlt normal)
    grep -l ABC bin1 txt          bin1 und txt
    grep -L ABC bin1              nichts, Exit 0
    grep -I ABC bin1              nichts, Exit 1
    grep -a ABC bin1              die Rohzeile
    grep ABC bin1 txt             Meldung fuer bin1, Zeile fuer txt
    grep -q ABC bin1              Exit 0, nichts
    grep --binary-files=text      wie -a
    grep --binary-files=without-match  wie -I

So gebaut: isBinary liest 8 KiB, springt zurueck, und der Drucker bekommt ein
suppress-Flag, das die Treffer zaehlt, aber keine Zeile schreibt. Dabei fiel auf,
dass scan() die Trefferzeile an emit() vorbei direkt schrieb; das war der Grund,
warum die erste Fassung die Meldung UND die Rohzeile ausgab.

-L fehlte bei uns ganz und ist mitgebaut: es nennt die Dateien ohne Treffer und
endet mit 0, auch wenn nichts zu nennen ist (so gemessen).

Gemessen mit scripts/grep-binary-probe.sh, 13 Aufrufe, alle gleich. stdout und
stderr werden getrennt verglichen; im gemeinsamen Kanal haengt die Reihenfolge
von der Pufferung ab, was nichts ueber das Verhalten sagt.

Batterie: sieben Pruefungen (grep-binary-message, -silent-stdout, -rc, -a, -I,
-count, -L-rc). Dazu zwei Go-Tests (TestBinaryFileMessage,
TestFilesWithoutMatch).

## Image-Neubau nach der GNU-Runde (23.11.2026, acmesh-k6e)

Neu gebaut mit dem Stand nach wc, base64, ls -t, df, diff -u, sed -z und grep
(Binaererkennung). Belege:

    report: 19456124 bytes, statically linked, stripped (das Image nimmt den
            statischen Bau, die dynamische 19513571-Version bleibt fuer den Host)
    make-image: 134 names linked into /bin
    make-image: image size 19.8 MB, 5 container starts a 350 ms,
                50 execs von /bin/true a 14.59 ms
    trivy: bin/akmesh, gobinary, 0 Vulnerabilities, Secrets sauber

Stichproben im Container (akmesh:latest), jede mit Ausgabe:

    wc -L /a.txt                3 /a.txt      (a.txt = a, bb, ccc)
    sed -z s/^/X/ /l.nul        Ausgabe 14 Bytes, grep -c X = 1
                                (Xa.txt\0Xb.txt\0, zwei Saetze, ein "Zeilenende")
    grep ABC /b.bin             "grep: /b.bin: binary file matches", rc 0
    ls -t /t                     y.txt vor x.tar (y.txt juenger)
    df -h | head -1              Filesystem      Size Used Avail Use% Mounted on

Beim Pruefen zwei neue Luecken gefunden, nicht Teil dieses Tickets:

    od -c /tmp/txt              od: unrecognized option "-c"  (GNU druckt Zeichen)
    tr "\0" "@"                 tr: unknown escape sequence "\0"  (GNU ersetzt NUL)

## Vorbereitung der Messreihe Textwerkzeuge (23.11.2026, acmesh-efg)

Rahmen statt Einzelskript: scripts/gnu-probe.sh TOOL AUFRUF-DATEI vergleicht
stdout, stderr und Exitcode getrennt gegen das gepinnte Debian-Image. Die
Aufrufe stehen in scripts/probes/<tool>.probe, das Fixture baut
scripts/gnufix.sh in /tmp/gnufix (elf Dateien: Woerter, Felder mit Doppelpunkt
und Tabulator, Gross-/Kleinschreibung, Zahlen, Umlaute, Leerzeilen, eine Zeile
ueber 8 KiB, leer, eine Binaerdatei, ein Text ohne Zeilenende).

Beleg, dass Rahmen und Fixture zusammen laufen (erster Lauf, sort):

    sort colon.txt              gleich
    sort -r colon.txt           gleich
    sort -k2 -t: colon.txt      gleich
    sort -u words.txt           gleich
    4 gleich, 0 abweichend

Der Fehler beim ersten Lauf war meiner: das Skript wechselt ins Fixture und las
die Probe-Datei danach relativ - jetzt wird der Pfad vorher absolut gemacht. Ein
Lauf ohne gelesene Aufrufe endet mit 2 statt mit "0 gleich".

Zuschnitt und Regeln stehen in docs/design/messreihe-textwerkzeuge.md, die
Einzel-Tickets in acmesh-efg (Dach) und sechs Geschwister.

## sort und uniq gegen GNU (23.11.2026, acmesh-6bq)

Rahmen: scripts/gnu-probe.sh, Fixture scripts/gnufix.sh.

sort: 25 Aufrufe, alle gleich GNU coreutils 9.7. Abgedeckt: Grundsortierung,
-r, -k2, -k2,2 mit -t:, -t: -k3, -u, -n, -nr, -n -k1, -f, -d, -s, -b auf
Leerzeilen, leere Datei, letzte Zeile ohne Umbruch, Umlaute, -c (Exit bei
unsortierter und sortierter Datei), -V, -h, -k2n, -u -k1,1, Eingabe von stdin,
-z, --version-sort. sort war schon unser eigenes Paket; die Messung hat nichts
zu reparieren gefunden.

uniq: 8 von 16 Aufrufen wichen ab, das war u-roots Fassung. Vier Fehler, alle
gebaut (jetzt internal/forked/uniq, mit u-roots Copyright):

    -c        GNU druckt die Zahl in sieben Spalten und dann ein Leerzeichen
              ("      1 b"), u-root druckte "1<TAB>b".
    -f N      fehlte ganz: die ersten N Blaetter und Felder ignorieren.
    -s N      fehlte ganz: die ersten N Zeichen ignorieren, nach -f.
    -w N      fehlte ganz: hoechstens N Zeichen vergleichen.
    -z        fehlte ganz: NUL-getrennte Saetze, das Gegenstueck zu find -print0.
    --lang    die langen Schreibweisen (--count, --repeated, --unique,
              --ignore-case, --skip-fields, --skip-chars, --check-chars,
              --zero-terminated) und zusammengesetzte Flags wie -ci.

Dazu ein Fund, der beim Messen selbst passierte: GNU uniq nimmt INPUT [OUTPUT],
kein Verzeichnis von Eingaben. "uniq -c words.txt colon.txt" schreibt das
Ergebnis nach colon.txt und gibt nichts aus - mein erster Entwurf las mehrere
Dateien als einen Strom, was das Fixture zerstoerte. Jetzt wie GNU, mit
Exit 1 bei einem dritten Operanden.

Belege: sort 25/25 gleich, uniq 17/17 gleich, drei Go-Tests
(TestGNUBehaviour mit 12 Faellen, TestZeroTerminated, TestBadOptionIsStatusTwo)
und acht Batteriepruefungen. Batterie 329 Linux / 306 Windows, help-audit gruen.

Zwei Probe-Aufrufe mussten umgeschrieben werden, weil sie das Fixture
beschrieben (uniq mit zwei Operanden). Der Ersatz laeuft ueber eine
Ausgabedatei und "cat", damit beide Seiten dasselbe messen.

## ugrep als Vorbild für Archive: Semantik gemessen (23.11.2026, acmesh-5nn)

Anlass: Plan in docs/design/rg-archive.md. ugrep 7.8.5 ist auf diesem Host aus
dem Alpine-Repo installiert (`apk add ugrep`), Auskunft von `ugrep --version`:
`+avx2; -P:pcre2jit; -z:zlib,bzip2,lzma,lz4,zstd,brotli,bzip3,7z,tar/pax/cpio/zip`.
Fixtures mit 7z gebaut (zip), tar czf für .tar.gz, alle in einem Quellordner
`src/` mit a.txt, b.zip{inner.txt}, nested.zip{deep.zip{deep.txt}},
c.tar.gz{doc.txt}, doc.txt.

- `ugrep -z -n 'hallo' src` (Default --zmax=1): Treffer in a.txt, b.zip{inner.txt},
  c.tar.gz{doc.txt}; für das Archiv im Archiv kommt
  `Binary file src/nested.zip{deep.zip} matches` - also NICHT still
  übersprungen, sondern als binär gesucht und gemeldet.
- `ugrep -z --zmax=2 -n 'hallo' src`: zusätzlich
  `src/nested.zip{deep.zip:deep.txt}:1:hallo tief`. Die Ebenen trennt ein
  DOPPELPUNKT, keine zweite Klammer.
- Drei Ebenen, Archiv als Argument: `ugrep -z --zmax=3 -n 'hallo' lvl1.zip`
  -> `{lvl2.zip:lvl3.zip:three.txt}:1:hallo drei`. Das äußerste Archiv steht
  nicht in den Klammern.
- zip{tar{}}: `ugrep -z --zmax=2 -n 'hallo' mix.zip` ->
  `{inner.tar:intar.txt}:1:hallo im tar`.
- `-l` und `-c` nennen dieselben Etiketten (`src/b.zip{inner.txt}`), auch die
  binäre Meldung wird als Datei gezählt.
- Ohne Treffer: Exit 1. Ohne `-z`: keine Treffer, Exit 1, kein Hinweis.
- Kaputtes zip (`PK\x03\x04murks`): `ugrep: cannot decompress kaputt.zip: an
  error was detected in the zip compressed data`, Exit 1.

## cut, comm, split, tee gegen GNU (23.11.2026, acmesh-y16)

cut: 20 Aufrufe, alle gleich. -d/-f in allen Formen (1, 1,3, 2-, 1-2, f5),
--complement, -s, -c, -b, --output-delimiter, fehlende Felder, leere Datei,
letzte Zeile ohne Umbruch, stdin, --delimiter=/--fields=.

tee: 6 Aufrufe, alle gleich (Durchreichen, -a, zwei Dateien, -i,
--append, /dev/null).

comm: war u-roots Fassung, 7 von 9 Aufrufen wichen ab. Neu als Fork
internal/forked/comm mit u-roots Copyright:

    -12/-13/-23/-123  u-root nutzte das flag-Paket, also nur -1 -2 -3 einzeln.
    --total           fehlte: GNU haengt "4\t6\t0\ttotal" an.
    --check-order     fehlte.
    --output-delimiter=STR  fehlte.
    Leerzeilen        u-root warf jeden Satz weg, dessen Ausgabe zwei Tabs war -
                      genau die leere gemeinsame Zeile. Dazu kuerzte es die
                      Tabs am Ende, GNU behaelt sie ("\t\t" fuer eine leere
                      gemeinsame Zeile).
    Unterdrueckte Spalte  u-root setzte die Tabs trotzdem, GNU nicht:
                      "comm -12" gibt die gemeinsamen Zeilen ohne Tabs aus.
    -                 meint stdin.

Jetzt 9 von 9 gleich.

split: 10 Aufrufe, 9 gleich; --verbose fehlte ("creating file 'xaa'"), ist
gebaut. 10 von 10 gleich.

Ein Befund ausserhalb des Tickets, der beim Messen auffiel: unsere cat-Meldung
lautet "cat: can't open 'xac': No such file or directory", GNU "cat: xac: No
such file or directory". Als Ticket notiert.

Batterie: zwoelf neue Pruefungen (cut 6, comm 4, split 2, tee 2).

## xargs gegen GNU findutils 4.9.0 (23.11.2026, acmesh-ji5)

19 Aufrufe, zuerst 9 abweichend. xargs war schon ein Fork von u-root; die
Fehler steckten in der Auswertung, nicht im Ablauf:

    Optionen nach dem Befehl  "xargs sh -c 'exit 5'" wurde als xargs-Option -c
                              gelesen: 127 statt der Kinderausfuehrung. Nun
                              endet die Auswertung am ersten Wort, das keine
                              Option ist (GNU liest Befehl und alles danach).
    Anfuehrungszeichen         "'a b'" kam mit den Quotes an, GNU gibt "a b"
                              als ein Argument. Eigener Zerleger: Blanks
                              trennen, ' und " halten zusammen, \ nimmt das
                              naechste Zeichen.
    -a FILE                   fehlte.
    -i[kurz]                  fehlte (veraltet, Synonym zu -I{}).
    -L N / -l                 fehlte: eine Ausfuehrung je N Zeilen.
    -E EOF / -e EOF           fehlte.
    --max-args=1 usw.         die langen Schreibweisen fehlten ganz.
    Exit 123                   folgte erst, wenn der Befehl wirklich lief.

Dazu zeigte die Messung eine Reihenfolge-Falle bei -t: GNU druckt "echo a",
dann a, dann "echo b", dann b; unsere Fassung druckte beide Spurzeilen zuerst,
weil sie vor dem Belegen des freien Platzes schrieb. Der Platz wird jetzt zuerst
genommen.

Stand: 19 von 19 gleich. Das ist der erste Befehl dieser Reihe, dessen Ablauf
stehen blieb und nur die Auswertung neu war.

Belege: sieben Batteriepruefungen (n1, -I, Quotes, -a, Optionen nach dem Befehl,
Exit 123, -E) und sechs Go-Tests. Auf Windows bleibt die Gruppe aus, weil dort
kein eigenes echo-Programm als Kind laeuft; die uebrigen Windows-Messungen
bleiben unberuehrt. Batterie 350 Linux / 320 Windows, beide 0 Fehler.

## awk gegen mawk 1.3.4 (23.11.2026, acmesh-722)

Rahmen: scripts/gnu-probe.sh, scripts/probes/awk.probe, 87 Aufrufe. Referenz
ist das awk des gepinnten Debian-Images: mawk 1.3.4 20250131 (mit
`awk -W version` belegt), nicht gawk - das Image hat kein gawk.

Ergebnis: 83 von 87 gleich. Unser awk ist goawk 1.31.0 (in go.mod gepinnt),
also kein Eigenbau; die Abweichungen stammen alle aus dieser Bibliothek:

    RSTART vor match       mawk: leer, goawk: 0. `print substr(...), RSTART`
                           gibt bei mawk "bcd ", bei uns "bcd 0".
    1/0                    mawk: +inf, Exit 0; goawk: "division by zero",
                           Exit 2. gawk verhaelt sich wie goawk.
    unbekannte Funktion    mawk "function xor never defined", goawk
                           "parse error at 1:13: undefined function \"xor\"".
    ** (Potenz)            mawk: Syntaxfehler; goawk rechnet 2**3 = 8. Eine
                           Obermenge, nach der Regel unschaedlich.

Alle drei echten Abweichungen stehen als Ticket acmesh-fl3 mit der Frage
mawk folgen oder dokumentieren. Bewusst nicht gebaut: gawk-Erweiterungen wie
BEGINFILE, --posix, -i inplace.

Abgedeckt waren unter anderem: Muster und Aktionen, -F mit Zeichen und mit
Tabulator, NF/NR/FNR/FILENAME, BEGIN/END, -v, -f progfile, printf in allen
Formatbreiten, gsub/sub/match mit RSTART/RLENGTH, split, index, substr in allen
Lagen (auch negativ), toupper/tolower, Arrays mit delete und length,
Feldzuweisung und NF=, OFS/ORS/FS, getline aus Datei und aus Pipe, next,
do-while, Funktionen mit Parametern, ENVIRON, die Mathefunktionen,
Zahlenformate (1e3, .5, 07/08) und der Vergleich "10" gegen 9.

Batterie: acht neue Pruefungen (print, -F, NR, END, gsub, printf, Array, -v).

Zwei Fehler in meiner eigenen Batteriearbeit, beide beim Gegenpruefen gefunden:
die awk-Pruefungen standen vor der Zeile, die c1.txt anlegt (leere Ausgabe statt
Fehlerbild), und unter Windows schreibt goawk CRLF, weshalb jede awk-Pruefung
jetzt vor dem Vergleich `tr -d '\r'` nutzt - so wie die uebrigen Pruefungen auch.
Batterie 358 Linux / 328 Windows, beide 0 Fehler.

## head und tail, Grenzfaelle gegen GNU (24.11.2026, acmesh-fqj)

Erst ein eigener Fehler: die erste Aufrufdatei mischte head und tail. Der Rahmen
scripts/gnu-probe.sh legt nur EINEN Namen in den PATH, also liefen die
tail-Aufrufe gegen das tail des Hosts (BusyBox 1.37.0, an seiner Meldung
erkannt). Getrennt in scripts/probes/head.probe (20 Aufrufe) und tail.probe
(17 Aufrufe).

head: 18 von 20 gleich, zwei Fehler, beide gebaut:
    -z           fehlte ganz; GNU liest damit NUL-getrennte Saetze
                 (head -z -n1 z.nul gibt "a.txt\0").
    Zeilenende   unser head druckte jeden Satz mit einem angehaengten Umbruch.
                 Bei einer Datei, die ohne Umbruch endet, kam so ein Zeichen
                 mehr heraus als bei GNU (gemessen an z.nul mit od -c: GNU endet
                 nach dem letzten \0, wir hatten ein \n dazu). Jetzt gehen die
                 Saetze roh hinaus, so wie sie gelesen wurden.
Jetzt 20 von 20 gleich.

tail: 16 von 17 gleich, die einzige Abweichung ist -z. Dazu faellt bei der
Meldung auf, dass u-root sie ueber log.Fatalf ausgibt: "2026/11/24 00:10:56
tail: invalid option -- 'z'". Ein Datum vor einer Fehlermeldung kennt kein
GNU-Werkzeug. Beides als Ticket: tail -z mit der Frage Fork oder Patch (P2) und
der Zeitstempel als eigene Sache (P3), weil er in zwoelf u-root-Befehlen der
Liste steckt (cat, more, tail, tee, mv, rm, ln, mktemp, realpath, cpio, gzip,
readelf).

Was sonst gleich war: -n in allen Schreibweisen (-2, -n2, -n 2, --lines=2,
-n0, -n -1), +N bei tail, -c in Bytes und mit negativem Wert, -q und -v,
Koepfe bei zwei Dateien, leere Datei, letzte Zeile ohne Umbruch, stdin,
mehr Zeilen als die Datei hat, --help.

Batterie: elf neue Pruefungen (head 8, tail 3).

Batterie 370 Linux / 340 Windows, beide 0 Fehler, help-audit gruen.
Groessen: 19534051 Bytes Linux dynamisch, 18176000 Bytes Windows, beide
gestrippt (head hat nichts gekostet, der Umbau ist gleich gross).

## tr und od gegen GNU (24.11.2026, acmesh-3qy / bly / 4b7)

Referenz GNU coreutils 9.7 im gepinnten Image. tr 22 Aufrufe, od 20 Aufrufe.

tr: zuerst 3 Abweichungen, alle gebaut.
    \0            "unknown escape sequence '\0'": unser tr kannte nur die
                  benannten Escapes. Jetzt liest es oktal wie GNU (\0 NUL,
                  \12 Zeilenumbruch, \101 das A), bis zu drei Stellen, Wert
                  modulo 256. Damit gehen "tr '\0' '@'" und "tr -d '\0'".
    [:class:]     eine Klasse zaehlte nur, wenn sie den ganzen Satz ausmachte.
                  "tr '[:upper:][:digit:]' 'a'" blieb deshalb wirkungslos; GNU
                  bildet das A und die 1 auf "a" ab. Jetzt werden Klassen
                  ueberall im Satz aufgeloest, in aufsteigender Reihenfolge.
Stand 22 von 22 gleich.

od: zuerst 6 von 20, dann 18, jetzt 20 von 20. Drei Ursachen.
    Kurzformen    -c, -b, -x, -o, -d, -s und -a fehlten ganz ("unrecognized
                  option '-c'", Exit 1). Jetzt sind sie eine Abkuerzung fuer
                  -t c, o1, x2, o2, u2, d2 und a, und sie lassen sich buendeln:
                  -An -tc, -cd, -w4.
    Spaltenbreiten "od -t u1" druckte vier Spalten statt drei. Gemessen sind
                  die Breiten von GNU: u 3/5/10/20, d 4/6/11/20, o 3/6/11/22,
                  x 2/4/8/16, Buchstaben und Zeichen je 3. Die Adresse ist bei
                  o und d siebenstellig, bei x sechsstellig ("000008").
    Mehrere Typen "-t x1o1" gab je Typ eine eigene Adresse aus. GNU setzt die
                  Adresse einmal je Block und rueckt die folgenden Zeilen um
                  ihre Breite ein; ausserdem richtet es die Spalten ueber alle
                  Typen an der breitesten aus. Beides ist gebaut.
    -w            Bytes je Zeile fehlte, jetzt vorhanden.

Nebenbei zwei Messfehler meinerseits: "-A n" schien die erste Spalte nicht
aufzufuellen, bis ich die GNU-Ausgabe byteweise mit od -tx1 gelesen habe (drei
Leerzeichen, dann das Zeichen) - die Annahme war falsch und ist verworfen.
Ebenso war "tr '\101' 'Z'" auf "a" als Eingabe eine falsche Erwartung: \101 ist
das A.

Batterie: zwoelf neue Pruefungen (tr 5, od 7). Damit sind die beiden P2-Tickets
aus dem Imagebau erledigt: od -c (acmesh-4b7) und tr \0 (acmesh-bly).

Batterie 382 Linux / 352 Windows, beide 0 Fehler, help-audit gruen.
Groessen: 19538147 Bytes Linux dynamisch, 18181120 Bytes Windows, beide
gestrippt (+8192 Linux und +5120 Windows gegen den Stand vor diesem Block).

## tail -z und die Meldung beim Oeffnen (24.11.2026, acmesh-bmr)

Das Image wurde vorher neu gebaut: 19.8 MB, akmesh:latest, trivy 0 (bin/akmesh,
keine Funde, keine Secrets). Der Stand davor stammte von vor grep -P, uniq,
comm, xargs, awk, head, tr und od.

tail: die Probe hatte 17 Aufrufe, und der einzige Abweichler war -z. Gebaut ist
    -z            ein Satz endet am NUL, wie bei GNU (tail -z -n2 auf
                  "a\0b\0c\0d\0" gibt "c\0d\0"; Trennzeichen geht durch alle
                  Modi: Rueckwaertslesen, Vorwaertslesen, +N).
    Meldungen     u-root schrieb sie ueber log.Fatalf, also mit Datum und
                  Uhrzeit davor. Jetzt "tail: invalid option -- 'Y'" und
                  "tail: cannot open 'nofile.txt' for reading: No such file or
                  directory", Exit 1. Der Text nach dem Doppelpunkt ist der des
                  Systems: unter Windows steht dort der deutsche Satz, deshalb
                  vergleicht die Batterie ihn nur unter Linux.
Der Leser meldet unabhaengig davon die deutsche Systemmeldung - das ist Go und
kein Fehler des Programms.

Der Patch liegt in docs/design/patches/uroot-tail-gnu.patch (jetzt 535 Zeilen)
und wurde gegen den Pin 869f00f in einer frischen Kopie als anwendbar geprueft.
Vier neue Batteriepruefungen (tail -z zwei Faelle, unbekannter Schalter,
fehlende Datei).

## Zeitstempel und cat-Meldung (24.11.2026, acmesh-jlk4 / acmesh-2rl)

Elf Befehle meldeten Fehler ueber log.Fatal, also mit "2026/11/24 00:10:56 " vor
der Zeile. Mit dem neuen Patch docs/design/patches/uroot-messages-gnu.patch
(cat, more, tee, mv, rm, ln, mktemp, realpath, cpio, gzip, readelf) schreibt
jeder Befehl jetzt selbst:

    fmt.Fprintf(os.Stderr, "NAME: %v\n", err); os.Exit(1)

Vorher: "2026/11/24 00:10:56 rm: remove nofile: no such file or directory"
Nachher: "rm: remove nofile: no such file or directory", Exit 1. Kein Datum mehr
in keiner der elf Meldungen; der Patch ist gegen den Pin 869f00f in einer
frischen Kopie als anwendbar geprueft und in scripts/patch-uroot.sh eingetragen.

cat dazu genauer (acmesh-2rl): GNU nennt Datei und Systemsatz, ohne Go-Wortlaut,
und faengt nach einer fehlenden Datei nicht an. Gemessen im gepinnten Image:

    GNU     cat nofile            -> "cat: nofile: No such file or directory", 1
    GNU     cat nofile hi.txt     -> die Meldung, dann "hi", Exit 1
    vorher  cat nofile            -> "cat: can't open 'nofile': No such file..."
                                    (u-root) bzw. "cat: open nofile: no such ..."
                                    nach dem Zeitstempel-Patch
Jetzt gibt unser cat "cat: nofile: No such file or directory" aus, kopiert die
uebrigen Dateien weiter und endet mit 1. Der Systemsatz wird wie bei glibc mit
grossem Anfangsbuchstaben geschrieben.

Drei neue Batteriepruefungen (cat fehlt, cat kopiert weiter, Status 1); die
Meldungstexte werden nur unter Linux verglichen, weil Windows den deutschen
Systemsatz liefert.

Batterie 389 Linux / 357 Windows, beide 0 Fehler, help-audit gruen.
Groessen: 19546339 Bytes Linux dynamisch, 18183168 Bytes Windows, beide
gestrippt (+8192 gegen den tail-Stand).
