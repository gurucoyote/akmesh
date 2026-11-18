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
