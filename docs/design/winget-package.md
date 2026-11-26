# akmesh als winget-Paket

Was nötig wäre, damit `winget install` eine akmesh-Distribution auf einen
Windows-Rechner bringt - und was daran nicht passt. Notiert am 26.11.2026,
gemessen gegen winget v1.29.380.

## Was winget verlangt

Ein Manifest, drei YAML-Dateien in einem Versionsordner:

    Testbild.Akmesh.yaml                       Version und Abhängigkeiten
    Testbild.Akmesh.installer.yaml             woher und wie installiert wird
    Testbild.Akmesh.locale.en-US.yaml          Name, Beschreibung, Lizenz

Die Felder, die zählen (alle in Schema 1.28.0 vorhanden, nachgelesen):

    PackageIdentifier     Verleger.Name, z. B. Testbild.Akmesh
    PackageVersion        unsere Version, je Release neu
    InstallerType         zip (Archiv), msi, exe, inno, nullsoft, portable, msix
    InstallerUrl          https, erreichbar, ohne Anmeldung
    InstallerSha256       sha256 der ausgelieferten Datei, muss passen
    NestedInstallerType   bei Archiv: der Typ im Inneren, hier portable
    NestedInstallerFiles  RelativeFilePath + PortableCommandAlias
    License               BSD-3-Clause
    ShortDescription      ein Satz

Ein Zip-Bündel mit einem Alias sieht dann so aus:

    PackageIdentifier: Testbild.Akmesh
    PackageVersion: 0.1.0
    InstallerType: zip
    NestedInstallerType: portable
    NestedInstallerFiles:
      - RelativeFilePath: bin\akmesh.exe
        PortableCommandAlias: akmesh
    InstallerUrl: https://.../akmesh-portable-0.1.0.zip
    InstallerSha256: <gemessen>

Für unser Bündel ist `zip` + `portable` der einzige Typ, der zur Regel "keine
Spuren im Host" passt: winget legt die Dateien unter
`%LOCALAPPDATA%\Microsoft\WinGet\Packages\<id>_<quelle>_<hash>\` ab, macht einen
Alias, und Deinstallieren heißt: Ordner weg. Kein MSI, kein Dienst, kein
Registry-Eintrag. Was winget selbst anfasst (PATH-Eintrag, gemessen bei jq),
bleibt seine Sache.

## Was daran für akmesh nicht reicht

Ein Alias, ein Name: `akmesh`. Die 136 Befehlsnamen entstehen bei uns erst
durch Hardlinks in `bin\`, und die legt heute `start.cmd` an. winget ruft kein
start.cmd auf. Zwei Stücke fehlen deshalb:

1. **Selbstfindung.** Das Binary kennt heute nur seine Umgebung; `AKMESH_ROOT`,
   `AKMESH_PROFILE` und die Linkliste setzt start.cmd. Ein winget-Paket hat
   kein start.cmd. Das Binary sollte aus dem eigenen Pfad ableiten, wo sein
   Bündel liegt (`bin\akmesh.exe` → eine Ebene hoch), und fehlende Variablen
   selbst setzen - dann ist es egal, wer es gestartet hat.
2. **Link-Erzeugung als Befehl.** Was `scripts/wsl-links.sh` für `/bin` tut,
   braucht es als `akmesh links`: die Namen aus der eigenen Tabelle lesen,
   fehlende Hardlinks in `bin\` anlegen, nichts überschreiben, das Ergebnis
   melden. Danach sind alle Befehle da, ohne start.cmd.

Mit beidem wäre die Reihenfolge: `winget install Testbild.Akmesh`, dann einmal
`akmesh links`. Wer start.cmd verwendet, merkt keinen Unterschied.

## Wie es in die Welt kommt

Drei Wege, vom offiziellen bis zum eigenen Regal:

1. **Offizielles Repo** `microsoft/winget-pkgs`: Manifest unter
   `manifests/t/Testbild/Akmesh/<version>/` ablegen, `winget validate --manifest`
   (auf dieser Maschine vorhanden, v1.29.380) laufen lassen, Pull Request. Die
   Prüfung dort sieht Syntax, Erreichbarkeit der URL und den Hash. Jedes Release
   ist ein neuer PR - mit `wingetcreate update` halbautomatisch.
2. **Eigene REST-Quelle**: `winget source add --name akmesh --type Microsoft.Rest
   --arg https://.../api/`, dazu ein kleiner Server, der die REST-Spezifikation
   von winget bedient. Mehr Arbeit, aber kein Fremd-Review und eigene Regeln.
3. **Manifest lokal**: Datei mitliefern, `winget install --manifest <datei>`.
   Braucht vorher `winget settings --enable LocalManifestFiles` in einer
   Administrator-Sitzung - eine Einstellung im Host, also genau die Spur, die
   wir sonst vermeiden. Für "mal ausprobieren" gut, für Auslieferung nicht.

## Was außerdem zu klären ist

- **Signatur:** winget signiert nichts. Eine unsignierte exe löst SmartScreen
  aus; ein Zertifikat (oder Geduld der Nutzer) ist eine eigene Entscheidung.
- **Version:** `pkgver` in apk und `PackageVersion` bei winget sollten dieselbe
  Zahl sein, sonst gibt es zwei Wahrheiten.
- **apk im winget-Paket:** funktioniert nur mit Selbstfindung (Punkt 1), sonst
  weiß apk nicht, wo seine Wurzel liegt.
- **Zwei Bezugswege:** Bündel-Zip von Hand und winget-Paket sollten dieselbe
  Zip sein, sonst baut man zwei Produkte.

## Reihenfolge, wenn wir es bauen

1. Selbstfindung im Binary (klein, testbar, nützt auch dem Zip-Bündel).
2. `akmesh links` als Befehl.
3. Zip bauen, sha256 messen, Manifest schreiben, `winget validate` grün.
4. `winget install --manifest` auf der Maschine probieren (mit der
   Administrator-Einstellung, danach wieder abschalten).
5. Wenn es hält: PR nach microsoft/winget-pkgs.