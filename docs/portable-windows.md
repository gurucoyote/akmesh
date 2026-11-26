# akmesh portabel für Windows

Das Ziel: ein Ordner, den man entpackt, und der eine Shell so weit ersetzt, daß
man losarbeiten kann - ohne Installation, ohne Registry, ohne Dienst. Löschen des
Ordners ist die Deinstallation, und nichts überlebt ihn.

## Was das Bündel enthält

Gebaut mit `./scripts/make-windows-bundle.sh` aus `akmesh.exe`:

    akmesh-portable/start.cmd                     der einzige Einstieg
    akmesh-portable/bin/akmesh.exe                20 672 512 Bytes, Windows PE, statisch, gestrippt
    akmesh-portable/etc/names.txt                 136 Namen, aus dem Binary geschrieben
    akmesh-portable/etc/profile.sh                PATH, Prompt, cd ins Home
    akmesh-portable/terminal/akmesh-profile.json  Vorlage für Windows Terminal
    akmesh-portable/README.txt                    dasselbe in kurz, für den Endnutzer
    akmesh-portable.zip                           alles zusammen, 8 364 162 Bytes

Das Binary liegt in `bin\`, nicht im Wurzelverzeichnis: oben wäre es das erste,
was ein Mensch doppelklickt, und ein Programm, das unter dem Namen `akmesh`
startet, weiß nicht, welcher Befehl gemeint ist. Im Wurzelverzeichnis liegt
neben `start.cmd` nur noch das README, `etc\`, `bin\` und `terminal\`.

`bin\` ist NICHT im Zip. Ein Hardlink lässt sich nicht archivieren, und eine
Kopie je Name wären 136 mal 20 MB. `start.cmd` legt die Links beim ersten Start
an, sie kosten keinen zusätzlichen Platz:

    mklink /H bin\ls.exe bin\akmesh.exe

Das ist auch der Grund, warum ein Name überhaupt als Befehl funktioniert: das
Binary entscheidet an seinem eigenen Dateinamen, welcher Befehl gemeint ist.

## Was dafür am Code nötig war

1. **Der Verteiler schneidet `.exe` ab.** Ohne das hieß ein Hardlink `sh.exe`
   für ihn `sh.exe`, fand keinen Befehl und nahm das erste Argument als Namen
   ("Failed to run command: command is not present in akmesh: -c"). Jetzt:
   `strings.TrimSuffix(strings.TrimPrefix(filepath.Base(os.Args[0]), "-"), ".exe")`
   an beiden Namensstellen, gepatcht in `scripts/build-bb.sh`; eine ältere
   Fassung mit nur dem Login-Strich wird beim Bauen ersetzt.
2. **Ein Profil ohne /etc.** `internal/sh` liest als Login-Shell
   `$AKMESH_PROFILE`, sonst `/etc/profile`, sonst `etc/profile.sh` neben dem
   Binary (oder eine Ebene darüber, für `bin\sh.exe`). `start.cmd` setzt
   `AKMESH_PROFILE` auf sein eigenes `etc\profile.sh`.
3. **Der PATH-Trenner ist der der Plattform.** `";"` auf Windows, `":"` sonst.
   Das Profil nahm zuerst einen Doppelpunkt, und damit wurde aus
   `C:\Users\Spernau/.local/bin:C:\TEMP\amtest2\bin` EIN Eintrag, den es nicht
   gibt - `ls: executable file not found in $PATH`, obwohl die Datei daneben lag.

## Gemessen (26.11.2026, Windows 11, aus der WSL-Shell gestartet)

    cmd.exe /c "cd /d C:\temp\amtest2 && start.cmd -c \"echo hallo-aus-dem-buendel; ...\""
    hallo-aus-dem-buendel
    HOME=C:\Users\Spernau
    PWD=C:\Users\Spernau
    ls | head -4           vier Dateien aus dem Home
    PATHEXT=.COM;.EXE;.BAT;.CMD;...
    rc=0

    bin\ nach dem ersten Start: 136 Links, alle auf akmesh.exe
    ./sh.exe -c 'echo 0=$0'        ->  0=sh          (der Strich und die Endung fallen weg)
    ./clear.exe -x | od -c         ->  033 [ H 033 [ 2 J 033 [ H

## Pakete: apk mit winget als Quelle

Im Bündel liegt root\etc\apk\repositories mit einer Zeile: `winget`. Damit ist
der Windows-Paketmanager eine Quelle unter apk, und apk-Repositories können
daneben stehen, sobald es welche gibt:

    apk search yq          fragt winget, jede Zeile beginnt mit "apk: winget:"
    apk add jqlang.jq      installiert über winget, sagt vorher wohin
    apk add MikeFarah.yq   ohne Punkt im Namen wird erst gesucht, bei
                           Mehrdeutigkeit werden Kandidaten gelistet
    apk list --all         apk-Datenbank und was winget installiert hat
    apk del jqlang.jq      entfernt über winget, wenn apk es nicht kennt
    apk upgrade --all      aktualisiert apk-Pakete und winget-Pakete

Zwei Dinge sind dabei bewusst so: winget-Pakete landen in Windows' eigenen
Ordnern, nicht im Bündel, und jede Ausgabe sagt das. Und die Namen in bin\
stehen vor WindowsApps im PATH, ein von winget installiertes jq wird also von
akmesh' eigenem jq (gojq) verdeckt - sichtbar an `jq --version`.

## Was fehlt, bewußt

- **git** kommt nicht mit. Separat installieren; akmesh tritt ihm nicht in den
  Weg, beide leben im selben PATH.
- **`/c/...`-Pfade** werden nicht übersetzt. Windows-Pfade (`C:/Users/...`,
  auch `C:\Users\...`) versteht jeder akmesh-Befehl; eine MSYS-Übersetzung würde
  nur Skripte belohnen, die fremde Konventionen erwarten.
- **Job-Control** (`%1`, `fg`, `bg`) gibt es nicht; das ist eine eigene Baustelle.
- **Kein PATH-Eintrag, kein Startmenü, kein Kontextmenü.** Wer akmesh in jeder
  CMD-Sitzung will, trägt `...\akmesh-portable\bin` selbst in die
  Benutzerumgebung ein - das ist dann seine Entscheidung und nicht unsere Spur.

## Windows Terminal

Die Vorlage in `terminal/akmesh-profile.json` in die `profiles.list` von
Windows Terminals `settings.json` übernehmen und den Pfad anpassen. Ohne sie
startet `start.cmd` im normalen Konsolenfenster, was genauso funktioniert.

## Wie man es prüft

    ./scripts/make-windows-bundle.sh          Zip neu bauen
    unzip -l akmesh-portable.zip              Inhalt ansehen
    cp -r akmesh-portable /mnt/c/temp/t/      auf die Windows-Seite legen
    cmd.exe /c "cd /d C:\temp\t && start.cmd -c \"ls /c/Windows | head -3\""
