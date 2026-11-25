# akmesh: Shell und Userland in einem Binary

Status: Skizze, erste Fassung lauffaehig (Windows-Binary mit 48 Befehlen).
Dieses Dokument haelt Bauform und Entscheidungen; die Zahlen stehen in
`docs/measurements.md`.

## Anlass

Ein Harness (lait, KleinKai) soll seinem Agenten nicht die echte System-Shell
geben. Gebraucht wird eine Shell plus Werkzeuge, die (a) in einem Stueck
ausgeliefert werden, (b) auf Linux und Windows laufen und (c) an einer Stelle
kontrollierbar sind: welche Pfade, welche Befehle, welche Prozesse.

Der Name wird Ak Mesh gesprochen: ein Geflecht kleiner Werkzeuge, keine grosse
Shell. Wer eine Anspielung hoert, darf sie behalten; das Projekt erklaert sie
nicht.

## Bauform

Drei Schichten, jede mit einer klaren Aufgabe:

1. Interpreter: mvdan.cc/sh/v3 (BSD-3). POSIX und bash, dazu Parser und
   Formatter. Die Haekel (OpenHandler, StatHandler, ReadDirHandler2,
   ExecHandler, CallHandler) sind der Ort, an dem die Regeln stehen.
2. Werkzeuge: u-root (BSD-3), mit gobusybox in dasselbe Binary gebaut.
   Verteilung ueber `os.Args[1]`, also `akmesh.exe ls pfad`.
3. Eigene Befehle und die Policy-Schicht: dort, wo u-root fuer das Ziel nicht
   baut und dort, wo eine Regel durchgesetzt werden muss, die kein
   Fremdprogramm einhaelt.

Wichtig: Schicht 1 allein erzwingt nichts. Die Handler fangen Umleitungen,
`source`, `test -f` und Programmpfade ab; `[ -r datei ]` (access(2)),
Glob-Expansion und jeder erlaubte externe Prozess laufen daran vorbei - auf
diesem Host dreimal gemessen. Deshalb kommt die Durchsetzung auf Kernel-Ebene
dazu: Landlock unter Linux, Job Objects unter Windows, fuer jedes Kind.

## Entscheidungen

- Go, nicht Rust. brush waere in Rust die reife Wahl, oeffnet die Dateiebene
  aber nicht (`sys::fs` ist `pub(crate)`), und mvdan/sh ist in Go die reife
  Wahl mit vollstaendigen Haken. Rust-Begruendung: keine.
- u-root als Befehlsquelle, nicht Eigenbau. 48 der 57 gewuenschten Befehle
  bauen fuer Windows ohne Aenderung.
- gobusybox statt vieler Einzeldateien: 8,9 MB fuer 48 Befehle plus Shell
  gegen rund 3 MB je Einzel-EXE.
- Unterbefehl-Form (`akmesh.exe ls`), nicht Symlink: Symlinks brauchen unter
  Windows Rechte.
- Die neun Befehle, die fuer Windows nicht bauen, werden per Build-Tag
  ausgeschlossen, nicht gepatcht. Sechs davon bauen wir selbst.
- Ausnahme von "nicht patchen": Fehlerpfade, die einen Erfolg vortaeuschen.
  `ls` druckte bei einem unbekannten Schalter die Usage und listete trotzdem,
  Exit 0; `chmod +x` scheiterte ohne eine Zeile Meldung. Beides liegt in
  `scripts/patch-uroot.sh`, wird bei jedem Build auf den gepinnten Checkout
  angewandt und bricht ab, wenn der erwartete Upstream-Text fehlt. Ein neuer
  Schalter wird weiterhin NICHT so nachgeruestet - dafuer gilt der Fork
  (Abschnitt "Eigene Befehle").

- tr arbeitet auf Zeichen, GNU auf Bytes (Entscheidung 19.11.2026, acmesh-508).
  `tr 'äöü' aou` gibt hier "aou"; GNU mit LC_ALL=C.UTF-8 gibt "uouuuu", weil es
  einzelne Bytes verschiebt. Bei -c und -C zaehlt akmesh ein Zeichen als eine
  Einheit. Grund: niemand verlaesst sich auf "uouuuu", das ist eine Grenze von
  GNU und kein Verhalten, auf das ein Agent baut, waehrend zerlegte Umlaute in
  deutschen Texten ein Schaden sind. Die byte-genaue Form bleibt moeglich, waere
  aber ein Neuschreiben des Forks. Die messbaren Unterschiede stehen in
  `docs/measurements.md`. Die Form `[c*n]` in SET2 folgt GNU.

## Eigene Befehle (geplant)

| Befehl | Windows-Umsetzung |
|---|---|
| ps | Prozessliste ueber Toolhelp32Snapshot oder x/sys/windows |
| kill | TerminateProcess; -9 und -TERM als Synonym, Signale gibt es nicht |
| killall | Prozessliste nach Namen filtern, dann kill |
| uname | GOOS/GOARCH/Version aus Go und der Windows-Version |
| hostname | os.Hostname() |
| free | Speicherstatus ueber die Windows-API |
| df | Laufwerke und belegter Platz |
| du | Verzeichnisgroesse, rekursiv |
| sync | ohne Wirkung (Go schreibt durch), aber vorhanden |
| timeout | os/exec mit Zeitlimit, danach beenden |

Jeder eigene Befehl bekommt zwei Dateien mit Build-Tag (`windows`, `!windows`)
und geht durch dieselbe Policy-Huelle wie die eingebauten.

- ps liest die Kommandozeile unter Windows ueber ntdll
  (ProcessCommandLineInformation, Klasse 60) statt ueber den PEB (19.11.2026,
  acmesh-94e.22): dafuer braucht es ein Recht, das ps ohnehin oeffnet, und keinen
  Zugriff auf Fremdspeicher. Scheitert der Aufruf, bleibt die Spalte beim
  Executable-Namen.

- Drei Zertifikatsbefehle, je eine Aufgabe (19.11.2026, acmesh-vcl/pav/0kh):
  certinfo liest, mkcert stellt aus, verify prueft. Getrennt, weil ein Befehl, der
  alles kann, im Fehlerfall nicht mehr sagt, welche der drei Fragen verneint
  wurde. Alle drei nur mit der Standardbibliothek.
- mkcert schreibt PEM im Klartext und sagt das in der Hilfe: kein PKCS#12, keine
  Passphrase (die Standardbibliothek kann verschluesseltes PKCS#8 nicht
  schreiben), kein HSM. Der Schutz des Schluessels ist 0600, unter Windows ueber
  dieselbe DACL-Aufzeichnung wie chmod (winmode.SetQuiet).
- verify prueft Kette und Name, nicht Widerruf: OCSP, CRL und Certificate
  Transparency werden nicht gefragt, ein widerrufenes Zertifikat geht durch. Das
  steht in der Hilfe und im Ticket, statt in einem Halbsatz.
- Ein Dateisystem, das keine DACL traegt (WSL-Share, FAT), nimmt keinen Modus auf.
  winmode.SetQuiet laesst diesen einen Fehler stillschweigend durch, weil os.Chmod
  dort das Einzige getan hat, was die Plattform kann; stat meldet dann den
  0666 der Plattform, also nichts Falsches.

- ls ist seit 19.11.2026 unser eigener Befehl (internal/ls, acmesh-wx6), keine
  gepatchte u-root-Kopie. Grund: das u-root-ls lehnte -1 -t -r -i -n -A -u -c ab
  und zeigte bei Dateien aelter als sechs Monate statt des Jahres die Uhrzeit des
  Laufs. Ein Fork haette pkg/ls und unixflag mitgebracht (374 Zeilen); neu
  schreiben war kuerzer und erlaubt die GNU-Form an einer Stelle.
  Nicht nachgebaut: GNU polstert die Spalten in -l auf eigene Mindestbreiten.
  Die Felder sind identisch, ein awk-Skript sieht keinen Unterschied.

## Dokumente lesen: any2md, und die fuenf Namen als Aliase (21.11.2026)

Ein Dokument zu lesen war bisher fuenf Befehle: html2md, rtf2md, docx2md,
epub2md, pdf2md. Jeder hatte seine eigene Argumentschleife, seine eigene
Fehlermeldung und seinen eigenen `-o`-Weg. Jetzt gibt es einen Leser
(internal/anymd), und die fuenf Namen sind Aliase, die ihr Format festnageln.

Warum ein Name und nicht fuenf:

- Die Richtung ist bei Lesern eindeutig. Eine PDF ist keine DOCX, das Format
  steckt in den Bytes (`Detect`: `%PDF-`, `{\rtf`, ZIP mit `word/document.xml`
  oder `META-INF/container.xml`, sonst fuehrendes `<`). Damit braucht any2md kein
  Formatflag und keine Wahl, die ein Agent raten muesste.
- Die alten Namen enthielten eine Symmetriefalle: html2md, pdf2html,
  html2docx, aber kein docx2html. Wer die Richtung raet, raet manchmal falsch,
  und "command not found" ist ein verschwendeter Turn.
- Anthropic raet in "Writing effective tools for agents" vom Eins-zu-eins-
  Verpacken von Endpunkten ab und zur Zusammenfassung auf den Arbeitsablauf.

Warum die fuenf Namen bleiben:

- Ein Name ist in einer Shell auch ein Stichwort. Die Namensliste ist die
  Suchliste; fuenf Namen sind fuenf Anker fuer den Agenten.
- Ein expliziter Name ist belegbar: in Tickets und Messwerten steht "pdf2md
  gegen pdftotext -layout", nicht "any2md".
- Ein Skript, das meint "das muss eine PDF sein", sagt weiter pdf2md. Der Alias
  nagelt das Format fest und lehnt eine erkennbar andere Datei ab
  (`pdf2md seite.html` -> "not a pdf, the bytes look like html", exit 1).

Was der Umbau mitnimmt:

- `-f/--from FORMAT` ist der Notausgang fuer Bytes ohne Marker; `-o FILE.md`
  schreibt die Datei und legt die docx-Bilder daneben.
- Ein Fehler in der Argumentbehandlung ist jetzt einer statt fuenf.
- `-o` mit mehr als einer Eingabe war vorher stiller Datenverlust (die letzte
  Datei gewann); jetzt exit 2 mit dem Hinweis, umzuleiten.
- Eine fehlende Datei beendet die Liste nicht mehr: gemeldet, exit 1, die
  naechste Datei wird gelesen.
- Die Hilfetexte der fuenf liegen in internal/anymd (aliasUsage), nicht mehr
  neben einer zweiten Implementierung. Der Text ist der alte, ergaenzt um den
  Satz, der das Format nennt.

Die Schreiberichtung bleibt getrennt: md2pdf, md2docx, md2epub sind keine
Erkennung, sondern eine Wahl. Ein any2any braeuchte ein Ziel-Flag, und ein Flag
ist genau die Stelle, an der ein Agent raet.

## Die Befehlsliste gehoert uns (21.11.2026)

Wer das Binary mit einem Namen aufruft, den es nicht traegt, bekommt eine Liste
seiner Befehle. Diese Liste war ein flaches Alphabet aus gobusybox, und dort
standen die Konverter zwischen `df` und `echo`.

Was jetzt gilt:

- Der Text und die Gruppierung stehen in `internal/cmdlist` (eine Tabelle
  Name -> Gruppe, die Bloecke Dokumente, Dateien/Text, System, Netzwerk, Shell,
  Sonstiges). Sie ist unser Code, mit Tests, und ohne Fremdquelle.
- Der Build forkt die Ausgabe: `makebb -g` erzeugt den Multicall-Quellbaum, dann
  schreibt `scripts/build-bb.sh` unser `cmdlist.go` (Paketklausel umgeschrieben)
  und `scripts/gen/print.go` in diesen Baum, patcht die Fehlerverzweigung der
  generierten `main.go` (`docs/design/patches/gobusybox-grouped-list.patch`) und
  baut das Binary mit `go build` selbst.
- Warum generierte Datei statt gobusybox-Quelle: die Aenderung liegt dann in
  unserem Repo, nicht als Patch an fremdem Code. Der Generator bleibt unberuehrt.
- Drei Waechter: der Patch muss greifen, die generierte main.go muss den Aufruf
  `printSupported()` enthalten, und das fertige Binary muss den Block-Titel
  "Files and text:" tragen. Sonst bricht der Build ab, statt still wieder die
  flache Liste zu liefern.
- `go build` laeuft mit den Flags, die makebb benutzt haette (`-a`,
  `-gcflags=all=-l`, `-ldflags "-s -w -buildid="`, `-trimpath`, `GOWORK=off`).
  Ohne `-a -gcflags` waechst das Binary um 1,87 MB (gemessen), und die
  Groessengrenze haette den Build gestoppt.
- Ein neuer Befehl braucht eine Zeile in der Tabelle: `TestEveryCommandIsPlaced`
  liest `scripts/cmds.txt` und `OURS_ALL` und macht aus einem vergessenen Namen
  einen roten Test.

## Der Name gehoert dem Build, nicht den Quellen (22.11.2026)

Das Repository heisst akmesh; ein Binary daraus muss nicht so heissen. Ein Build
kann sich bssdsh nennen, und alles, was ein Mensch liest, sagt dann bssdsh.

Wie der Name reist:

- `internal/appname.Name` ist eine var (keine const, nur eine var ist per
  Linker setzbar). `scripts/build-bb.sh` liest `AKMESH_NAME`, schreibt den Namen
  in `-ldflags "-X akmesh/internal/appname.Name=..."`, in den Dateinamen des
  Binaries und in die zwei Dispatcher-Meldungen aus gobusybox.
- Die zwei Meldungen sind Zeichenketten in einer fremden Datei. Der Build
  ersetzt in ihnen das Wort, das ein frueherer Build hinterlassen hat, statt
  gegen eine Liste alter Namen zu pruefen: `AKMESH_NAME=bssdsh` und danach ein
  normaler Build muessen in jeder Reihenfolge gehen, und eine schwarze Liste
  kennt nicht jeden Namen, den ein Build je hatte.
- Der zweite Satz ("NAME dispatched %q") wird an Anfuehrungszeichen und ` %q`
  verankert. Die Datei deklariert naemlich auch
  `var dispatched atomic.Pointer[string]`; ein Muster ohne Anker hat daraus eine
  kaputte Deklaration gemacht (gesehen am 22.11.2026, Build brach mit
  "syntax error" in register.go ab).
- Felder, die dem Namen folgen: die Befehlsliste (kein Name), `shuf --version`,
  `jq -V`, der User-Agent von curl/wget (`bssdsh/0.1 (curl compatible)`), die
  Organisation eines Testzertifikats (`bssdsh mkcert`) und das Dateipraefix im
  Trust-Store (`bssdsh-BASE.crt`).
- Der Modulpfad bleibt `akmesh/internal`. Name und Importpfad sind zwei Dinge,
  und ein Binary namens bssdsh darf weiter akmesh-Pakete importieren.

Der Zertifikats-Issuer ist zusaetzlich zur Laufzeit waehlbar: `mkcert -org NAME`,
sonst `AKMESH_CERT_ORG`, sonst `<Buildname> mkcert`. Eine CA, die schon existiert,
behaelt ihren Issuer - ihr Subject steckt in jedem ausgestellten Zertifikat und
in jedem Store, der ihr vertraut -, und mkcert sagt das, statt still ein anderes
Subject zu liefern. Der Name im Trust-Store folgt dem Build-Namen, nicht `-org`:
der Eintrag muss ohne Raten wieder zu entfernen sein, und die Fehlermeldung nennt
den Dateinamen.

## sed spricht den GNU-Dialekt, ueber einen Fork von Go.Sed (22.11.2026, acmesh-4ne)

sed, awk und grep sind die drei Werkzeuge, bei denen ein Agent seine
gewohnte Eingabe hineinschreibt. sed lief bisher auf `rwtodd/Go.Sed`, dessen
regulaere Ausdruecke Go-Syntax sind: keine BRE-Escapes, `\1` und `&` in der
Ersetzung kaputt, linkester erster statt linkester laengster Treffer. Der
Engine wird deshalb als **Patch** geforkt (kein Kopie-Baum, wie bei
goldmark-pdf): `docs/design/patches/go-sed-gnu-semantics.patch`, angewandt von
`scripts/apply-go-sed-fork.sh`, aufgerufen vor dem Bau in
`scripts/build-bb.sh` und in `scripts/check-all.sh`. Das Skript ist idempotent
und prueft den angewandten Stand am Modulinhalt, nicht per `patch -R`.

Der Patch uebersetzt BRE (`\( \) \+ \? \| \{`), macht `\1..\9` und `&` in
der Ersetzung moeglich (`$1` bleibt gueltig), laesst bei einer Alternation den
laengsten Treffer gewinnen, setzt `(?s)` auf jedes Muster, damit `.` wie in sed
den Zeilenumbruch trifft, und oeffnet `-E`/`-r` ueber
`NewExtended`/`NewQuietExtended`. `regexp.CompilePOSIX` war der naheliegende
Weg und ist gemessen falsch: Go laesst dort jeden Treffer fallen, der einen
Zeilenumbruch ueberspannt (Zahlen und Fälle in `docs/measurements.md`,
"sed gegen GNU sed"; Referenz ist GNU sed 4.9 aus einem Debian-Container, nicht
das BusyBox-`/bin/sed` dieses Hosts).

Offene Grenze: Rueckverweise im Suchmuster (`s/\(a\)\1/X/`) brauchen einen
Backtracking-Motor, den keine Go-Engine hat; sed meldet das als Fehler, statt
etwas Falsches zu treffen.

## Was sich absetzt, und was nicht

Nur httpd laeuft im Hintergrund (Vorgabe), mit -f im Vordergrund: ein
Entwicklungswerkzeug, das Dateien ausliefert, ein Testziel abgibt und TLS
pruefen laesst. Ein abgesetzter Server muss beendbar bleiben, sonst haelt er
seinen Port - die Batterie prueft das jetzt (httpd-detach-stopped,
tls-httpd-stopped).

sshd bauen wir bewusst nicht (Entscheidung 19.11.2026, acmesh-7qk): ein
SSH-Server in einem Harness-Binary ist ein Fernzugang, den kein Agent braucht,
und er braechte Authentifizierung, Schluesselverwaltung und eine eigene
Absicherung mit. Wir liefern die Client-Seite (ssh, scp).

## Policy

Ein Wrapper, durch den jeder Befehl laeuft: er loest relative Pfade gegen das
Arbeitsverzeichnis auf, prueft sie gegen die erlaubten Wurzeln und verweigert
sonst mit einer Meldung. Ein Befehl, der eine Datei selbst oeffnet, ohne diesen
Wrapper, ist ein Defekt.

Offen: ob akmesh als Shell fuer ein Harness die Policy mitbringt oder nur die
Haken liefert, an denen das Harness seine Policy einhaengt. Erste Fassung:
beides, die Policy ist optional und abschaltbar.


## Geforkte u-root-Befehle

Fehlt einem u-root-Befehl ein Schalter, den Agenten tippen, wird er geforkt -
nicht neu geschrieben und nicht per Patch im Checkout veraendert (der Patch
bleibt den Fehlerpfaden vorbehalten, siehe scripts/patch-uroot.sh).

Ablage: internal/forked/<name>/, ein eigenes Paket, das internal/tools ueber
eine duenne Huelle aufruft (internal/tools/<name>.go plus Eintrag in der
Tabelle plus cmd/<name>). Der Dateikopf der Kopie bleibt der Copyright-Kopf der
u-root-Autoren; darunter stehen zwei Zeilen mit Upstream-Commit und Dateipfad
und eine Liste der lokalen Aenderungen.

Anlegen: scripts/fork-uroot.sh <name> <upstream-datei>... Das Skript prueft den
gepinnten Commit, kopiert, benennt package main um, setzt den Herkunftshinweis
und traegt Datei samt SHA-256 in internal/forked/manifest.json ein. Die
Umbauten (main nach Run, flag-Paket raus, pkg-Importe mitkopieren) bleiben
Handarbeit.

Drift-Regel: internal/forked/drift_test.go vergleicht den Commit im Manifest mit
UROOT_COMMIT aus scripts/build-bb.sh und, wenn der Checkout da ist, die
SHA-256 der Upstream-Dateien. Wandert der Pin, wird der Test rot, bis jemand die
Kopie ansieht und das Manifest nachzieht. Stilles Auseinanderlaufen ist der
Preis des Forks; der Test macht ihn sichtbar.

Bisher geforkt: head (acmesh-86m: -5, -q, -v, Exit 1 bei Lesefehler), chmod
(acmesh-agt: -R, -v/-c mit GNUs Wortlaut, +x wie GNU), cp (acmesh-1fs: -a, -p,
-t DIR, -n, -v im GNU-Format), xargs (acmesh-icw: -I, -r, -d, -P, Exit
123/124/127), touch (acmesh-6u2: -t im GNU-Format, -d mit den geschriebenen Datumsformen, -r),
tr (acmesh-6lw: -s, -c, Bereiche wie a-z), wc (acmesh-7ih: -m,
--files0-from), base64 (acmesh-dv0: -w, -i), hexdump (acmesh-0fd: -C),
tar (acmesh-r9r: -czf/-tzf/-xzf, gzip-Erkennung beim Lesen, -C,
--strip-components, --exclude), find
(acmesh-29n: Startpfad optional, -maxdepth, -mindepth, -exec, -not, -path,
-print0, "./" wie GNU) und grep (acmesh-2nz: -E, -w, -o, -A/-B/-C, -r/-R,
Exit 0/1/2 und -c je Datei wie GNU), id (acmesh-eqq: fehlende
/etc/passwd oder /etc/group sind kein Abbruch, die Zahlen werden gedruckt;
-kombinierte Kurzflags wie -un; die Primaergruppe steht in groups=).
Bei find fiel pkg/ls weg, bei grep das
flag-Paket und unixflag: ein Fork, der weniger Fremdcode mitzieht, ist der
kleinere. Ein Dateiname aus pkg/ bekommt das Elternverzeichnis als Praefix
(pkg/find/find.go -> find_find.go), sonst kollidiert er mit dem Befehl selbst.

## Hilfe-Texte (--help)

Seit 19.11.2026 gibt es einen gemeinsamen Hilfe-Einstieg: internal/usage hält
eine Tabelle mit einem kurzen Text je Befehl (Synopsis plus die Schalter, die der
eigene Parser wirklich liest) und beantwortet `--help`, bevor der Befehl seinen
Parser startet. Jeder `cmd/<name>/main.go` ruft `usage.Run`, `usage.RunIO` oder die
Namens-Variante davon und reicht seine eigene `Run`-Funktion durch.

Der Grund für die Stelle: die Hilfe lag zuerst an der Befehlstabelle in
internal/tools, und die erreicht das Binary nicht - jeder Einstieg ruft sein
eigenes Paket direkt auf (akmesh-mq1). Beim Dispatcher statt im Befehl ist die
Regel außerdem nicht umgehbar: fünf Befehle hatten `--help` vorher ausgeführt
statt beantwortet (hostname setzte den Namen, sync öffnete eine Datei).

Regeln: `--help` druckt eine Usage-Zeile und endet mit 0; Ausnahme ist `false`,
das wie GNU 1 liefert. `-h` bleibt, was der eigene Parser damit tut, nur hostname
beantwortet es als Hilfe wie das Original. Befehle mit eigenem, längerem Hilfe-Text
(curl, wget, jq, ps, sh, ssh, scp, xargs, chmod, file, sort, shuf, die
Markdown-Wandler) stehen nicht in der Tabelle und behalten ihren Text.

Prüf-Gate: `scripts/help-audit.sh --check <binary>` in check-all.sh. Die acht
u-root-Befehle ohne Hilfe (ls, mv, rm, mkdir, gzip, uptime, printenv, man) stehen
dort als benannte Liste, damit ein neuer Befehl ohne Hilfe auffällt statt in einer
großen Zahl zu verschwinden.

## Lizenz

akmesh selbst steht unter BSD-3-Clause (Datei `LICENSE`, "Guru Coyote",
2026) - dieselbe Lizenz, unter der u-root steht, von dem der Grossteil der
Befehle stammt. Damit kennt die Weitergabe nur eine Sorte Text neben den
Fremdhinweisen in `NOTICE`.

BSD-3-Clause fuer mvdan/sh, u-root und gobusybox, auch fuer die nach
internal/forked kopierten Befehle (der Copyright-Kopf bleibt dort stehen).
Ebenso BSD-3-Clause: internal/winmode, uebernommen aus der Gabel qiangli/sh
(Copyright Daniel Marti), unveraendert bis auf das Paketverzeichnis - die
Copyright-Kopfzeilen der fuenf Dateien bleiben stehen.

sed laeuft auf rwtodd/Go.Sed (github.com/rwtodd/Go.Sed), MIT-Lizenz
(Copyright Richard Todd), geforkt per Patch (siehe den sed-Abschnitt oben). Der
MIT-Hinweis gehoert in dasselbe NOTICE-Dokument. Am 24.11.2026 kamen die Befehle
l, T, Q und z dazu (acmesh-qxaf); der Patch traegt sie, die Lizenzlage bleibt MIT.

Neu dazugekommen (23.11.2026): github.com/go-regexp/pcre2, eine reine
Go-Portierung von PCRE2 10.44, BSD-3-Clause, mit einer zweiten Datei
LICENSE-PCRE2 fuer das Original (PCRE2 selbst steht unter einer
BSD-aehnlichen Lizenz der University of Cambridge). Beide Hinweise gehoeren in
dasselbe NOTICE-Dokument, weil die Portierung den Originalcode abbildet.

Ein ausgeliefertes Binary
muss die Hinweise mitfuehren; dazu gehoert ein NOTICE-Dokument im Repo, bevor
das erste Binary das Haus verlaesst.

### Was nicht hineingeht (Gate seit 19.11.2026)

"Wir liefern ein Binary" heiszt: die Lizenz des Binaries ist die Summe seiner
Abhaengigkeiten. Deshalb ist die Lizenzpruefung Teil des Bau-Gates
(`./scripts/check-licences.sh`, aufgerufen von `check-all.sh`) und keine Notiz in
einem Ticket:

- Ein Modul mit der GNU Affero General Public License faellt durch, weil AGPL
  ein Binary, das ueber ein Netz dient, zur Offenlegung zwingt.
- Ein Modul ohne jede Lizenzdatei faellt ebenfalls durch: Schweigen ist keine
  Erlaubnis.
- Eine kurze Verbotsliste faengt die bekannten Faelle auch dann, wenn im Cache
  kein Lizenztext liegt: `fumiama/go-docx` (AGPL-3.0) und
  `zikani03/goldmark-docx` (gar keine Lizenz) - beide waren fuer md2docx im
  Gespraech und sind damit dauerhaft ausgeschlossen.
- Geprueft werden nur die Module, die wirklich in das Binary kompiliert werden
  (`go list -deps`), nicht Testhelfer, die nie hineingeraten.

Rotprobe: ein Probe-Modul mit `fumiama/go-docx` in /tmp/agplprobe faellt mit
zwei Meldungen durch ("is on the forbidden list", "is AGPL, which we cannot
ship"), exit 1. Im akmesh selbst laufen acht Module durch, exit 0.

- Rechte unter Windows kommen aus internal/winmode (19.11.2026, acmesh-m6x).
  os.Chmod kann dort nur das Read-only-Bit, ein chmod 755 war ein stiller Erfolg
  ohne Wirkung. Das uebernommene Paket der Gabel qiangli/sh (Copyright Daniel
  Marti, BSD-3) schreibt den Modus als ACE-Folge in die DACL und liest ihn
  zurueck; chmod schreibt, stat liest ueber winmode.Apply. Auf Dateisystemen
  ohne DACL (WSL-Share) scheitert das Setzen mit Exit 1, statt Erfolg zu luegen.
  ls -l bleibt bei der Windows-eigenen Anzeige, weil ls ein u-root-Befehl ohne
  Fork ist.
