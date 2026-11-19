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

## Ein Binary, ein Name (18.11.2026)

Das eigene ext-Binary ist entfernt, cmd/ext gibt es nicht mehr. Jeder eigene
Befehl ist nur noch ein cmd/<name>-Paket, das build-bb.sh in dasselbe Binary
baut. Die Testbatterie heisst deshalb scripts/test-acmesh.sh. In den aelteren
Abschnitten steht noch scripts/test-ext.sh und die Groesse von ext: das waren
Messungen am damaligen zweiten Binary, sie bleiben als Verlauf stehen.

## ext entfernt, Meldungen auf acmesh (18.11.2026)

cmd/ext und der zweite Verteiler (tools.Main, usage, Names, List) sind weg.
Jeder eigene Befehl ist nur noch ein cmd/<name>-Paket. Skripte: build-ext.sh
geloescht, test-ext.sh heisst test-acmesh.sh.

    ./scripts/build-bb.sh linux   -> acmesh     11.358.332 Bytes (10,83 MiB)
    ./scripts/test-acmesh.sh ./acmesh -> 76 checks, 0 failures
    go test ./internal/...        -> ok policy, ok tools

Meldungstext: "acmesh: unknown command" nennt jetzt den aufgerufenen Namen
(programName aus os.Args[0]).

Gepatcht am 18.11.2026: `./acmesh gibtsnicht` antwortet mit
"command is not present in acmesh". Der Text steckt per //go:embed in makebb,
deshalb ersetzt build-bb.sh ihn in den gobusybox-Quellen und baut makebb neu,
wenn es noch den alten Text enthaelt. Der Bau prueft danach sein eigenes
Binary und bricht ab, wenn der alte Text noch drin ist.

    ./scripts/build-bb.sh linux   -> acmesh     11.341.948 Bytes (10,82 MiB)
    ./scripts/build-bb.sh windows -> acmesh.exe 11.367.936 Bytes (10,84 MiB)
    ./scripts/test-acmesh.sh ./acmesh     -> 79 checks, 0 failures
    ./scripts/test-acmesh.sh ./acmesh.exe -> 65 checks, 0 failures

(Beide Groessen mit go1.27.0 gebaut, siehe den Toolchain-Befund im naechsten
Abschnitt.)

## makebb gepinnt (18.11.2026)

MAKEBB_VERSION=v0.6.0 steht in scripts/build-bb.sh; der Bau liest die Version
mit go version -m aus dem installierten Werkzeug und laedt nur, wenn es fehlt
oder nicht auf v0.6.0 steht. Danach ersetzt er den Dispatcher-Text und baut
makebb genau dann neu, wenn der alte Text noch eingebettet ist.

Frischer Pfad geprueft: makebb geloescht, Modul-Cache auf den alten Text
zurueckgesetzt, ./scripts/build-bb.sh linux laeuft durch und das Binary meldet
"not present in acmesh".

    ./scripts/build-bb.sh linux   -> acmesh     11.358.332 Bytes (10,83 MiB)
    ./scripts/build-bb.sh windows -> acmesh.exe 11.383.296 Bytes (10,84 MiB)
    ./scripts/test-acmesh.sh ./acmesh     -> 79 checks, 0 failures
    ./scripts/test-acmesh.sh ./acmesh.exe -> 65 checks, 0 failures

Wichtiger als makebb: auch der Go-Toolchain ist nicht gepinnt. GOTOOLCHAIN=auto
hat waehrend dieser Arbeit von go1.27.0 auf go1.27.4 gewechselt, und derselbe
Quellstand ergibt damit 11.341.948 gegen 11.358.332 Bytes. Reproduzierbar ist
der Bau erst, wenn auch die Toolchain feststeht.

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

## curl und wget (18.11.2026, gebaut und geprueft)

    ./scripts/build-bb.sh linux   -> acmesh     10.657.916 Bytes (10,16 MiB)
    ./scripts/build-bb.sh windows -> acmesh.exe 10.669.568 Bytes (10,18 MiB)
    ./scripts/build-ext.sh linux  -> ext         7.442.695 Bytes (7,10 MiB)

Im acmesh kosten curl und wget rund 53 KB, weil net/http und TLS durch ssh
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

Pruefungen: scripts/test-ext.sh 50 Faelle je Ziel, acmesh, acmesh.exe und ext
alle 0 Fehler.

Grenzen: kein -w (Formatsprache), keine Fortschrittsanzeige, kein HTTP/2
sondern das, was net/http aushandelt, Cookies im einfachen Netscape-Format,
wget ohne rekursives Spiegeln und ohne -N.

## jq im Binary (18.11.2026)

    ./scripts/build-bb.sh linux   -> acmesh     11.481.212 Bytes (10,95 MiB)
    ./scripts/build-bb.sh windows -> acmesh.exe 11.510.272 Bytes (10,98 MiB)
    ./scripts/build-ext.sh linux  -> ext         9.097.479 Bytes (8,68 MiB)

Zuwachs gegenueber dem Stand ohne jq: 823.296 Bytes (804 KB) Linux,
840.704 Bytes (821 KB) Windows. Die Vorabmessung mit einer Probe im Binary
hatte 802.816 / 817.152 Bytes ergeben; der kleine Unterschied ist die
Schalterverarbeitung um die gojq-Aufrufe herum.

Belegt am gebauten Binary:
- jq -r '.items[] | "\(.n)=\(.v)"' auf einer Datei gibt "a=1" und "b=2".
- ./acmesh curl -fsSL -A acmesh https://api.github.com/repos/golang/go | \
  ./acmesh jq -r '"\(.full_name) stars=\(.stargazers_count)"' ->
  "golang/go stars=140183".
- Windows: dieselbe URL durch acmesh.exe jq -c '.license.key' -> "bsd-3-clause".
- HTML statt JSON ergibt "jq: invalid JSON: invalid character '<' ..." und
  Exit 2, also eine Meldung, mit der man etwas anfangen kann.

Pruefungen: scripts/test-ext.sh 58 Faelle je Ziel, acmesh, acmesh.exe und ext
alle 0 Fehler. Go-Tests: Felder, verschachtelte Pfade, .a[], -c, -r, -rc, -s,
mehrere Dokumente im Strom, -n, select, map, length, --arg, --argjson,
--slurpfile, -e mit 0/1/4, -R und -R -s, kaputter Filter (Exit 3), kaputtes
JSON (Exit 2), unbekannter Schalter (Exit 2), Datei als Eingabe.

Nebenbefund: gojq marshalisiert anders als encoding/json (NaN wird null,
<>& werden nicht maskiert). Fuer die huebsche Ausgabe wird json.Indent auf die
gojq-Ausgabe gelegt, damit die Maskierung von gojq erhalten bleibt.

## Policy und eigene Shell (18.11.2026)

    ./scripts/build-bb.sh linux   -> acmesh     11.370.620 Bytes (10,84 MiB)
    ./scripts/build-bb.sh windows -> acmesh.exe 11.399.168 Bytes (10,87 MiB)
    ./scripts/build-ext.sh linux  -> ext         9.810.183 Bytes (9,36 MiB)

Die Policy-Schicht und die eigene Shell kosten zusammen nichts, sie sparen
sogar: gegenueber dem Stand mit jq und mit u-root-gosh sind es 110.592 Bytes
weniger (Linux) und 111.104 Bytes weniger (Windows). Der Grund ist der Tausch
von u-root gosh gegen unsere duenne mvdan/sh-Front: die Zeilenbearbeitung und
die Ergänzungen der u-root-Shell fallen weg, die Landlock-Aufrufe sind ein paar
hundert Zeilen mit x/sys.

Belegt (Linux, WSL-Kernel 6.18, Landlock ABI 7):
- acmesh policy meldet Exit 0, die Policy-Datei und die Pfade.
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

    ./scripts/build-bb.sh linux   -> acmesh     11.350.140 Bytes (10,82 MiB)
    ./scripts/build-bb.sh windows -> acmesh.exe 11.378.688 Bytes (10,85 MiB)

Gegenueber dem Stand mit u-root-ssh sind das je 20.480 Bytes weniger, obwohl
scp neu dazugekommen ist. Der Grund: x/crypto war durch die alte ssh schon im
Binary, unsere Front ist duenner als die u-root-Shell um dieselbe Bibliothek.

Belegt durch scripts/test-ssh.sh gegen einen echten sshd (15 Pruefungen, 0
Fehler, auf acmesh und acmesh.exe): Ausgabe, Mehrzeiligkeit, Remote-Exitcode 7,
test -f wahr und falsch, stdin-Pipe, unbekannter Host ohne Freigabe Exit 255,
Upload, rekursiver Upload, Download, rekursiver Download, jeweils mit Inhalt.

## scp -p (18.11.2026)

    ./scripts/build-bb.sh linux   -> acmesh     11.354.236 Bytes (10,83 MiB)
    ./scripts/build-bb.sh windows -> acmesh.exe 11.382.272 Bytes (10,86 MiB)

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

    ./scripts/build-bb.sh linux full  -> acmesh     11.354.236 Bytes (10,83 MiB)
    ./scripts/build-bb.sh linux fs    -> acmesh-fs   9.777.276 Bytes (9,32 MiB)

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

    acmesh-fs vor der Korrektur     9.777.276 Bytes (9,32 MiB)
    acmesh-fs nach der Korrektur    6.959.228 Bytes (6,64 MiB)

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
    ./scripts/test-acmesh.sh ./acmesh         -> 88 checks, 0 failures
    ./scripts/test-acmesh.sh ./acmesh.exe     -> 74 checks, 0 failures

Groesse (vorher 11.358.332 / 11.383.296):

    acmesh      11.395.196   +36.864
    acmesh.exe  11.425.792   +42.496

-R verhaelt sich wie GNU: Hash je Schluessel, gleiche Schluessel bleiben
zusammen. Gemessen: printf 'b\na\nb\n' | ./acmesh sort -R -> b, b, a.
Wer wirklich durchmischen will, nimmt shuf (eigener Befehl).

## Testabdeckung je Befehl (18.11.2026)

Zaehlung gegen die Befehlstabelle (34 Namen), nicht gegen Dateien:

    Go-Tests beruehren direkt      9   curl wget df du free split sha*sum jq ps
    nur die Live-Batterie         25   awk basename cut diff dirname env file
                                       iconv kill killall nl patch sed tac
                                       timeout truncate uname sort shuf ...
    keine Pruefung irgendwo        4   stat sync hostname policy

Die Batterie (scripts/test-acmesh.sh, 88 Pruefungen auf Linux) ist eine echte
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

    ./scripts/test-acmesh.sh ./acmesh     -> 95 checks, 0 failures
    ./scripts/test-acmesh.sh ./acmesh.exe -> 81 checks, 0 failures

Groesse: acmesh 11.403.388 (+8.192), acmesh.exe 11.431.936 (+6.144).

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

    ./scripts/test-acmesh.sh ./acmesh     -> 101 checks, 0 failures
    ./scripts/test-acmesh.sh ./acmesh.exe -> 86 checks, 0 failures

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
    policy   ohne ACMESH_POLICY Exit 1 und "no policy ...", --check auf eine
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
denselben Dateien und Aufrufen verglichen. Linux-Binary ./acmesh, Windows
./acmesh.exe aus scripts/build-bb.sh, jeweils 11 MiB.

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
    gleiches auf Windows mit acmesh.exe: ebenfalls gleich

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
es wendet ACMESH_POLICY einmal pro Prozess an und beendet den Prozess bei
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
Zaehlung der Zeilen mit acmesh/internal/tools): echo, true, jq, file, iconv,
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
    ./scripts/build-bb.sh linux         acmesh        11821180 Bytes
    ./scripts/build-bb.sh windows       acmesh.exe    11854848 Bytes

Der kleine Bau ist damit 4,9 MB kleiner als der volle, bei 11,8 MB vorher gab es
also gar keinen kleinen Bau.

Geprueft auf dem Ziel: acmesh-ext auf Linux (ps -ef, uname -s, free -m, awk,
sed -n, sha256sum, ls) und acmesh-ext.exe auf Windows (uname -s, ps, awk,
sha256sum, diff). Ein nicht enthaltener Befehl sagt "command is not present in
acmesh: curl" und endet mit Exit 1 - nicht mit 0 und nicht mit 127, das ist die
Antwort des Verteilers von gobusybox.

Groessenbudget im Bau: Warnung ueber 12.000.000 Bytes, harter Fehlschlag ueber
16.000.000, und ein Zuwachs ueber 1 MB gegenueber dem letzten Bau desselben
Ziels steht im Baubericht. Die letzte Groesse liegt in .build/last-size-<GOOS>-
<Profil>, also nicht im Repo. Beide vollen Ziele bauen mit der Pruefung durch,
Batterie 169/0 Linux und 149/0 Windows.

## Linux-Binary bauen und messen (18.11.2026, acmesh-94e.2)

Der Bau laeuft auf beiden Zielen aus demselben Skript, die Zahlen sind von heute:

    ./scripts/build-bb.sh linux     acmesh      11825276 Bytes
    ./scripts/build-bb.sh windows   acmesh.exe  11854848 Bytes
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

    ./acmesh tracedeps /tmp/k.sh        (k.sh: #!/bin/sh; cat /etc/hostname)
    /bin/cat
    /etc/hostname
    /tmp/k.sh

Mit -probes kommen die Pfade dazu, die nur geprueft wurden (PATH-Suche):

    ./acmesh tracedeps -probes /tmp/k.sh
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

    ./acmesh httpd -f -p 127.0.0.1:8099 -h /tmp/www
    curl -s -o /dev/null -w '%{http_code}' http://127.0.0.1:8099/           -> 200
    curl -si http://127.0.0.1:8099/a.css | grep -i content-type              -> text/css; charset=utf-8
    curl -s -r 0-3 http://127.0.0.1:8099/index.html                          -> <h1>
    curl -s -o /dev/null -w '%{http_code}' http://127.0.0.1:8099/nix          -> 404
    ./acmesh httpd -p 8098                                                    -> "started in the background, pid N", 200 danach
    ./acmesh httpd -f -p 127.0.0.1:8443 -S c.pem -K k.pem                     -> https 200, Klartext auf demselben Port 400
    ./acmesh httpd -M x.mjs                                                   -> text/javascript; charset=utf-8
    ./acmesh httpd -d 'a%20b'                                                 -> a b
    ./acmesh httpd -e '<a>'                                                   -> &lt;a&gt;

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

    ./acmesh html2md /root/rekr/content/music/demo.html
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

    ./acmesh docx2md "/root/earthartsky.de/AntwortMaxi.docx" > maxi.md
    ./acmesh docx2md document.docx            # mehrere Dateien nacheinander
    ./acmesh docx2md < document.docx          # von der Standardeingabe

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
    acmesh                   12472444 Bytes, acmesh.exe 12504064 Bytes

Batterie: Linux 181 Pruefungen, Windows 159, beide 0 Fehler.

Eine Falle, die eine Stunde gekostet hat und nicht im Bau lag: die .exe wird
unter WSL ueber /init gestartet, und ein laufender acmesh.exe-Prozess haelt das
Abbild seines Pfades fest. Nach dem Bau lieferte ./acmesh.exe deshalb weiter den
alten Stand (der Befehl fehlte, obwohl die Datei byte-identisch mit dem guten
Bau war: md5 gleich, eine Kopie unter anderem Namen lief richtig). Erst nach
kill der beiden alten httpd-Prozesse war der neue Befehl da. Wer nach einem Bau
"der Befehl fehlt" sieht, prueft zuerst laufende acmesh.exe: pgrep -af acmesh.exe.

## httpd Hintergrundbetrieb (21.11.2026, acmesh-rds)

Der Fehler: ohne -f startete kein Server, sondern die Meldung

    httpd: exec: "httpd": executable file not found in $PATH     (exit 1)

Ursache: der Verteiler setzt os.Args[0] auf den Befehlsnamen, also "httpd";
exec.Command(os.Args[0]) sucht diesen Namen dann im PATH statt das laufende
Binary zu starten.

Reparatur: das Kind wird mit os.Executable() als Programm und dem Befehlsnamen
als erstem Argument gestartet, also acmesh httpd -f ...; ein eigenstaendig
gebautes cmd/httpd braucht den Namen nicht und bekommt ihn nicht
(detachArgv in internal/httpd/httpd.go).

Beweis am gebauten Binary:

    $ ./acmesh httpd -p 127.0.0.1:18081 -h /tmp/www2
    httpd: started in the background, pid 3999217          exit 0
    $ curl -s -o /dev/null -w '%{http_code}' http://127.0.0.1:18081/     -> 200
    $ kill 3999217                                            -> Port zu, 000

Derselbe Weg in der eigenen Shell und als eigenstaendiges Binary:

    $ ./acmesh sh -c '/root/acmesh/acmesh httpd -p 127.0.0.1:18082 -h /tmp/www2'
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

    ./acmesh rtf2md story.rtf
    ./acmesh pdf2md bericht.pdf

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
    acmesh 12537980, acmesh.exe 12569600 Bytes

Tests: rtf2md elf, pdf2md zwoelf, inklusive Rotproben (NumFmt-Bedingung,
Schriftgroeszen-Ueberschrift).
Batterie: Linux 188 Pruefungen 0 Fehler, Windows 166 0 Fehler.

Eine zweite WSL-Falle, gemessen 18.11.2026: der Pfadcache. Nach einem Bau
lieferte /root/acmesh/acmesh.exe weiter den alten Stand (die neuen Befehle
fehlten), obwohl md5 von acmesh.exe und einer Kopie gleich waren und die Kopie
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
Groesze: pdf2md 2769020 Bytes gegen base64 2609276, acmesh 12562556,
acmesh.exe 12595712.

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
pdf2md-form-xobject (Feldtext aus einem Form-XObject). Groesze: acmesh
12562556, acmesh.exe 12597248, pdf2md 2769020 Bytes.

Der Pfadcache traf wieder zu: nach dem Bau listete ./acmesh.exe pdf2md nicht
auf, eine Kopie (/tmp/acmesh-check.exe) gab "Feldtext" aus, und "strings
acmesh.exe | grep -c pdf2md" fand den Namen zehnmal - die Datei war also neu,
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
    /root/AGENTS.md, acmesh/README.md, roderik/AGENTS.md             identisch
    zwei Tabellenproben (2 und 3 Spalten)                            identisch

Batterie: Linux 194 Pruefungen 0 Fehler, Windows 172 0 Fehler, neue Faelle
md2txt-l-heading, md2txt-l-inline, md2txt-l-table, md2txt-l-wrap.

Groesze: probe (base64 md2txt-l) 2650236 gegen base64 2609276, also 40960 Bytes -
der kleinste der Dokumentbefehle, weil er nur Standardbibliothek und einen
eigenen kleinen Parser braucht. acmesh 12591228, acmesh.exe 12624896.

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
- goldmark-pdf ist der Brocken: 13,6 MB, mehr als das ganze acmesh heute
  (12,6 MB). Ursache ist nicht der Code, sondern die mitgelieferten Schriften:
  im Probe-Binary finden sich 1581 Font-/TTF-Muster und 242 Treffer auf
  DejaVu/Liberation/Noto, go-text/typesetting und go-fpdf bringen sie mit.
- go-epub kostet allein 4,8 MB, zusammen mit PDF aber nur 0,4 MB zusaetzlich,
  weil sich golang.org/x/net und anderes die beiden teilen.
- Der docx-Schreiber kostet gemessen 416 KB fuer zip und xml - im acmesh-Binary
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

Damit ist acmesh heute frei von AGPL: weder go.mod noch go.sum nennen
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

Gegen acmesh heute (12.591.228 Bytes roh) ergibt der magere Fork rund 19,0 MB,
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
+0,42 MB. Mit allen Zielen landet acmesh bei rund 23 MB, also 13 MB als Warner
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

    acmesh vorher (ohne goldmark)                 12.591.228
    acmesh mit md2pdf, Fork ohne chroma           15.077.500   also +2.486.272
    acmesh.exe                                    15.152.640

Der Zuwachs ist kleiner als die Rohmessung der Probe (7,50 MB fuer den Fork),
weil HTTP, TLS und die uebrigen Netzpakete im acmesh ohnehin schon stecken.

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

    acmesh vorher (ohne md2docx)      15.077.500
    acmesh mit md2docx                15.130.748   also +53.248

Belege, drei unabhaengige Leser:

    acmesh docx2md (eigener Leser)
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
