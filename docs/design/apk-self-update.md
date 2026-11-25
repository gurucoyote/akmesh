# apk: atomares Schreiben und Selbst-Update

Stand 25.11.2026. Zwei Tickets, ein Thema: ein Paket, das das laufende Binary
austauscht, darf das System nicht ohne Werkzeug zurücklassen.

## Was vorher passierte

`internal/apkcore/db.go` hat jede reguläre Datei so geschrieben:

1. `os.RemoveAll(target)` - die alte Datei war damit weg.
2. `os.OpenFile(target, O_CREATE|O_WRONLY|O_TRUNC)` - neu anlegen und füllen.

Folgen, gemessen am 25.11.2026 in `/root/.hermes/cache/scratch/selfup`:

- Ein Schreibversuch auf ein **laufendes** Binary scheitert mit `Text file busy`.
  Unser Weg funktionierte nur, weil er vorher löscht.
- Zwischen Löschen und fertigem Schreiben fehlt die Datei oder ist halb da. Ein
  Absturz, eine volle Platte oder ein abgebrochener Download hinterlässt eine
  kaputte Datei ohne Rückweg - auch für das Binary, mit dem man das reparieren
  würde.
- `rename(2)` über ein laufendes Binary gelingt dagegen; der laufende Prozess
  behält seinen alten Inode. Genau das nutzt der neue Weg.

## Was jetzt gilt (acmesh-vrt1)

`writeMember` schreibt jede reguläre Datei als `.apk-<zufall>` in das
Zielverzeichnis, dann `Sync`, `Chmod`, und zuletzt `os.Rename` auf den
Zielnamen. Sichtbar ist immer entweder der alte oder der vollständige neue
Stand. Ein abgebrochener Vorgang lässt die alte Datei unverändert und räumt die
Zwischendatei weg. Ein Verzeichnis im Weg wird vorher entfernt - das ist der
einzige Fall, den rename nicht kann.

Symlinks, Hardlinks und Verzeichnisse laufen weiter über den alten Weg; sie
tragen keinen Inhalt, der halb geschrieben werden könnte.

Kosten: ein zusätzliches Sync pro Datei. Für Paketinstallationen ist das
vernachlässigbar, der Vorteil ist der ganze Punkt der Änderung.

## Selbst-Update (acmesh-ieh3)

`internal/apkcmd/selfupdate.go` schützt die eine Datei, deren Verlust das
System handlungsunfähig macht: das laufende Binary selbst.

1. Vor dem Entpacken: wenn die Installation in ein Verzeichnis geht, das
   `/bin/<name>` des laufenden Binaries enthält, wird dieses Ziel als
   `DeferTarget` gemeldet.
2. `apkcore.Extract` schreibt diesen einen Eintrag als `<ziel>.apk-new`
   daneben statt auf das Ziel (`ExtractOptions.DeferTarget`/`Deferred`).
3. Nach allen Dateien und Skripten des Pakets wird die neue Fassung einmal
   gestartet: `<ziel>.apk-new true`. Das prüft Verteiler, Policy und
   Rückgabewert in einem Zug.
4. Nur bei Rückgabewert 0 wird sie per rename auf ihren Platz gesetzt. Sonst
   bleibt die alte Fassung stehen, der Befehl endet mit 1, und die untestete
   Datei bleibt als `.apk-new` zur Untersuchung liegen. Die Paketdatenbank
   wird in diesem Fall nicht geschrieben, weil `add` vorher abbricht.

Damit ist der Rückweg eingebaut und nicht nachgereicht: solange die Prüfung
nicht bestanden ist, wurde nichts ersetzt. `apkguard` bleibt für den Fall
danach - wenn der Start mit der neuen Fassung scheitert.

## Wie Alpine es regelt

Aus dem Gedächtnis, nicht frisch in der Quelle gelesen (gitlab.alpinelinux.org
blockt Bots, der GitHub-Spiegelpfad für 2.14 war nicht erreichbar) - bei Bedarf
nachzuziehen:

- apk ersetzt sich selbst als normales Paket; derselbe Inode-Mechanismus macht
  das möglich.
- Es gibt keinen Live-Patch: laufende Dienste und Shells behalten den alten Code
  bis zum Neustart. Die Upgrade-Dokumentation sagt das ausdrücklich.
- Konfigurationsdateien landen als `.apk-new`, die alte bleibt stehen.
- Der Rettungsanker ist statisches busybox und `apk --root`, nicht apk selbst.

Was wir übernehmen: den Inode-Mechanismus und die Ehrlichkeit über laufende
Dienste. Was wir zusätzlich machen: prüfen, bevor ersetzt wird - Alpine tut das
nicht, dort kann ein kaputtes apk-tools-Paket den Paketmanager selbst
unbrauchbar machen.

## Grenzen

- Windows: ein laufendes Programm lässt sich dort nicht ersetzen. Das Paket
  scheitert mit einer klaren Meldung; das ist ehrlicher als ein halber Zustand.
- Die Prüfung startet das neue Binary einmal. Sie beweist nicht, dass es
  richtig funktioniert - nur, dass es läuft und einen Befehl dispatcht.
- Andere Dateien eines Pakets werden weiterhin sofort ersetzt; die Prüfung gilt
  dem Binary, nicht dem ganzen System.
