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
Exit 0/1/2 und -c je Datei wie GNU). Bei find fiel pkg/ls weg, bei grep das
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

BSD-3-Clause fuer mvdan/sh, u-root und gobusybox, auch fuer die nach
internal/forked kopierten Befehle (der Copyright-Kopf bleibt dort stehen). Ein
ausgeliefertes Binary
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
