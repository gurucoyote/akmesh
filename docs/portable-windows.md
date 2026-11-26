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
   `C:\Users\user/.local/bin:C:\TEMP\amtest2\bin` EIN Eintrag, den es nicht
   gibt - `ls: executable file not found in $PATH`, obwohl die Datei daneben lag.

## Gemessen (26.11.2026, Windows 11, aus der WSL-Shell gestartet)

    cmd.exe /c "cd /d C:\temp\amtest2 && start.cmd -c \"echo hallo-aus-dem-buendel; ...\""
    hallo-aus-dem-buendel
    HOME=C:\Users\user
    PWD=C:\Users\user
    ls | head -4           vier Dateien aus dem Home
    PATHEXT=.COM;.EXE;.BAT;.CMD;...
    rc=0

    bin\ nach dem ersten Start: 136 Links, alle auf akmesh.exe
    ./sh.exe -c 'echo 0=$0'        ->  0=sh          (der Strich und die Endung fallen weg)
    ./clear.exe -x | od -c         ->  033 [ H 033 [ 2 J 033 [ H

## Welcher Trenner gilt

In der Shell ist `/` der richtige Trenner, auch auf Windows:

    ls C:/Windows          geht
    ls C:\Windows          geht nicht unquoted: die Shell frisst den Backslash
                           und sucht "C:Windows"
    cd C:/Windows          geht, pwd meldet danach C:\Windows

Drei Gründe: Windows nimmt beide Formen in seinen Datei-APIs, die Shell benutzt
den Backslash selbst als Escape-Zeichen, und die Tab-Vervollständigung trennt
nur an `/`. Wer einen Backslash-Pfad tippt, muss ihn also in Anführungszeichen
setzen.

Zwischen Verzeichnissen in PATH steht `;` (Windows), nicht `:` - das setzt
profile.sh über PATHEXT. Was die Shell ausgibt, kommt in Windows-Form mit
Backslashes (pwd, ls), weil Windows sie so liefert.

## Pfade in WSL-Distributionen

Die Netzwerkpfade, die der Windows-Explorer anbietet, gehen ohne Umbau:
`\\wsl.localhost\<Distro>\...` und `\\wsl$\<Distro>\...`, mit Backslashes
oder als `//wsl.localhost/...`. Gemessen am 26.11.2026 im Bündel:

    ls '\\wsl.localhost\akmesh\etc'      apk, group, hostname
    cat '\\wsl.localhost\akmesh\etc\hostname'   HOST
    cd '\\wsl.localhost\akmesh\home'    rc=0, pwd zeigt den Pfad
    echo probe > '\\wsl.localhost\akmesh\tmp\probe.txt'   rc=0, lesbar

Es braucht also keine Übersetzung: Windows reicht den Pfad an den WSL-Dienst,
unser Code sieht einen normalen Pfad. Zwei Lücken bleiben:

- Tab vervollständigt nur an "/" (complete.go), ein Backslash-Pfad wird als ein
  Wort behandelt. Für UNC-Pfade also keine Vervollständigung - offen.
- Die git-bash-Schreibweise `/c/Windows` gibt es nicht; wer sie tippt, bekommt
  "cannot access". `C:/Windows` geht.

## Wer bin ich: der Windows-Benutzer, nicht root

Auf Windows gibt es kein USER in der Umgebung, nur USERNAME. Die Shell hat
daraus frueher nichts gemacht und nannte die Sitzung "root", obwohl sie unter
einem gewoehnlichen Windows-Konto lief (gemeldet 26.11.2026). Jetzt setzt die
Shell beim Start USER und LOGNAME aus USERNAME, und \u im Prompt nimmt denselben
Namen:

    HOST:C:\Users\user
    user$

USER und LOGNAME bleiben, was sie waren, wenn die Umgebung sie schon kennt -
auf Linux ist das der Login-Prozess.

### Administrator ist nicht root

Windows hat keinen root-Benutzer und keine uid. "Als Administrator ausfuehren"
gibt demselben Konto ein erhoehtes Token; Dateien und Registry duerfen danach
mehr, der Benutzer bleibt derselbe. Deshalb fragt die Shell das Token ab, nicht
die Umgebung: laeuft der Prozess erhoeht, steht im Prompt user# statt
user$, und `id` bleibt ehrlich bei uid=-1 (acmesh-1779). Wer die Rechte
wissen will, fragt also den Prompt, nicht einen Benutzernamen.

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
Ordnern, nicht im Bündel, und jede Ausgabe sagt das.

Die Reihenfolge im PATH ist Absicht: was installiert wurde, antwortet vor dem,
was akmesh mitbringt.

    root\usr\bin, root\bin        apk-Pakete
    WinGet\Links                   (bleibt leer, winget legt dort nichts ab)
    WinGet\Packages\*              jedes installierte winget-Paket
    WindowsApps                    Store-Aliase, winget.exe
    bin\                           akmesh' eigene Befehle
    der geerbte PATH               Windows selbst

Gemessen: nach `apk add jqlang.jq` meldet `jq --version` jq-1.8.2 (winget) und
nicht mehr jq-akmesh-1.0 (gojq) - das installierte Werkzeug gewinnt. `find`
bleibt akmesh' find und fällt nicht auf C:\Windows\System32\find.exe zurück,
weil bin\ vor dem geerbten PATH steht. Shell-Builtins wie cd und echo sind
davon nicht betroffen, die liegen in der Shell selbst.

Nebenwirkung, die man kennen sollte: winget hängt das Paketverzeichnis an die
Benutzer-PATH in der Registry (bei jq gemessen). Das ist wingets Entscheidung,
nicht unsere, und bleibt nach einer Deinstallation als leerer Eintrag stehen.

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
