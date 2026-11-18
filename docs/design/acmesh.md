# acmesh: Shell und Userland in einem Binary

Status: Skizze, erste Fassung lauffaehig (Windows-Binary mit 48 Befehlen).
Dieses Dokument haelt Bauform und Entscheidungen; die Zahlen stehen in
`docs/measurements.md`.

## Anlass

Ein Harness (lait, KleinKai) soll seinem Agenten nicht die echte System-Shell
geben. Gebraucht wird eine Shell plus Werkzeuge, die (a) in einem Stueck
ausgeliefert werden, (b) auf Linux und Windows laufen und (c) an einer Stelle
kontrollierbar sind: welche Pfade, welche Befehle, welche Prozesse.

Der Name acmesh ist der Punkt: ein Geflecht kleiner Werkzeuge, keine grosse
Shell.

## Bauform

Drei Schichten, jede mit einer klaren Aufgabe:

1. Interpreter: mvdan.cc/sh/v3 (BSD-3). POSIX und bash, dazu Parser und
   Formatter. Die Haekel (OpenHandler, StatHandler, ReadDirHandler2,
   ExecHandler, CallHandler) sind der Ort, an dem die Regeln stehen.
2. Werkzeuge: u-root (BSD-3), mit gobusybox in dasselbe Binary gebaut.
   Verteilung ueber `os.Args[1]`, also `acmesh.exe ls pfad`.
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
- Unterbefehl-Form (`acmesh.exe ls`), nicht Symlink: Symlinks brauchen unter
  Windows Rechte.
- Die neun Befehle, die fuer Windows nicht bauen, werden per Build-Tag
  ausgeschlossen, nicht gepatcht. Sechs davon bauen wir selbst.

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

Offen: ob acmesh als Shell fuer ein Harness die Policy mitbringt oder nur die
Haken liefert, an denen das Harness seine Policy einhaengt. Erste Fassung:
beides, die Policy ist optional und abschaltbar.

## Lizenz

BSD-3-Clause fuer mvdan/sh, u-root und gobusybox. Ein ausgeliefertes Binary
muss die Hinweise mitfuehren; dazu gehoert ein NOTICE-Dokument im Repo, bevor
das erste Binary das Haus verlaesst.
