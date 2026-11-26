# apk als Paketweg auf Windows: winget als eine Quelle, eigene .apk-Repos daneben

Stand 26.11.2026. Gebaut in acmesh-ellk: add, del, search, info, list --all,
update und upgrade --all sprechen mit winget. Messungen in docs/measurements.md.

## Ziel

In der portablen Windows-Sitzung bleibt `apk` das eine Wort für "installier mir
etwas". Zwei Sorten Quelle stehen gleichberechtigt daneben:

- echte apk-Repositories (APKINDEX.tar.gz, signiert) - dort liegen unsere eigenen
  Windows-Werkzeuge als `.apk`,
- winget, als eine Quelle unter den anderen.

Ein Befehl, mehrere Quellen, wie bei Alpine auch: das ist die Form, die ein Agent
schon kennt.

## Die Repo-Zeilen

`etc/apk/repositories` im Bündel, eine Zeile je Quelle:

    https://packages.example.org/akmesh/windows/amd64    eigenes Repo
    winget                                               delegiert an winget.exe
    winget:msstore                                       optional, Store-Quelle

Regeln:

- `-X URL` überstimmt die Datei wie heute.
- `@tag URL` bleibt reine apk-Syntax. Ein `@tag` an `winget` ist ein Fehler mit
  Meldung, keine stille Ignoranz - ein Kanal, den winget nicht hat, darf nicht
  so aussehen, als gäbe es ihn.
- `winget` ohne weitere Angabe heißt immer die Community-Quelle `winget`, nicht
  `msstore` (der Store will häufig ein Konto).

## Auflösung, wenn ein Name in beiden Welten liegt

    apk search jq      beide Quellen, Spalte Quelle zeigt, woher der Treffer kommt
    apk add jq         erst apk-Repos (Name, Version, Kanal), dann winget
    apk del jq         erst die eigene Datenbank (root/lib/apk/db), dann winget uninstall
    apk info jq        erst apk, dann winget show
    apk list           apk-Datenbank plus winget export (JSON)
    apk upgrade        apk-Pakete wie heute; winget nur mit eigenem Schalter

Gewinnt das apk-Repo, sagt der Befehl es. Findet keine Quelle etwas, bleibt es
beim heutigen "no such package"; findet winget mehrere Kandidaten, werden sie
gelistet und der Befehl endet mit Fehler - nicht raten.

## Wo was landet

- apk-Pakete in die Wurzel: `-p/--root`, sonst `AKMESH_ROOT`, im Bündel
  `<bundle>\root`. Das Profil setzt beides und hängt `root\usr\bin` vorne an den
  PATH. So bleibt der portable Ordner die Installation.
- winget-Pakete in Windows' eigenen Ordnern (`WinGet\Packages`, Namen als Aliase
  in `WindowsApps`). Das können wir nicht umleiten und wollen es nicht: das ist
  die eine bewußte Ausnahme, und der Nutzer löst sie mit dem Befehl aus.

## Was wir unter Windows als `.apk` bauen

Ein Paket aus `apkbuild`: tar.gz mit `.PKGINFO`, Nutzlast ein Windows-`.exe`
unter `usr/bin`, signiert mit unserem Schlüssel, Prüfung wie heute. Es ist
bewußt kein Alpine-Paket - dieselbe Datei würde auf Alpine installieren und
nichts starten. Deshalb trägt es im `.PKGINFO` eine eigene Arch-Angabe
(`windows-amd64`), und `apk` prüft sie wie heute die Arch.

## Was nicht passiert

- Kein Parsen der lokalisierten winget-Ausgabe. Wahrheit ist der Exit-Code;
  gelesen werden nur `search` (Spalten) und `export` (JSON).
- Keine Lizenz- oder Kontofragen: `--accept-source-agreements
  --accept-package-agreements --disable-interactivity` stehen immer an.
- Kein stilles Ausweichen, wenn winget fehlt: dann sagt `apk`, daß die Quelle
  nicht da ist, und arbeitet mit den apk-Repos weiter.
- winget prüft Pakete nicht gegen unseren Schlüssel. Das ist seine Grenze; im
  Hilfetext steht sie, statt daß wir sie verschweigen.

## Offene Punkte für den Bau

1. Reihenfolge beim `upgrade`: nur apk-Pakete, oder mit `--all` auch
   `winget upgrade --all`? Ich neige zu: nur mit `--all`, weil ein
   unbeaufsichtigtes Upgrade sonst fremde Programme anfaßt.
2. `apk list` braucht bei installiertem winget einen Aufruf, der ein paar
   Sekunden dauert. Ich würde es nur auf Verlangen (`apk list --all`) machen und
   sonst nur die apk-Datenbank zeigen.
