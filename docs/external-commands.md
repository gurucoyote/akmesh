# Fremde Programme aufrufen, besonders unter Windows

Gemessen am 18.11.2026 mit akmesh.exe (voller Bau, 11.382.272 Bytes) auf Windows
10.0.22631, gestartet aus WSL heraus ueber den UNC-Pfad
\\wsl.localhost\golang\root\akmesh\akmesh.exe, damit nichts auf C: kopiert
werden muss.

## Antwort

Ja. akmesh.exe startet fremde Windows-Programme, mit Argumenten, und der
Exitcode kommt zurueck. Belegt:

    akmesh.exe sh -c 'hostname'            -> HOST (der Windows-Rechner)
    akmesh.exe sh -c 'whoami'              -> user
    akmesh.exe env whoami                  -> user (ueber exec.LookPath)
    akmesh.exe sh -c 'cmd /c ver'          -> Microsoft Windows [Version 10.0.22631.6199]
    akmesh.exe sh -c 'cmd /c whoami /user' -> Benutzername und SID
    akmesh.exe sh -c 'cmd /c exit 3; echo $?' -> 3

Die Suche laeuft ueber PATH und PATHEXT, "hostname" findet also hostname.exe
ohne Endung. Der Weg ist derselbe wie unter Linux: mvdan/sh ruft fuer fremde
Befehle os/exec auf, und Go loest dort auf Windows PATHEXT auf.

## Zwei Einschraenkungen

.cmd und .bat lassen sich nicht direkt starten. Getestet mit einer Datei
hello.cmd:

    Windows selbst:            & \\wsl.localhost\...\hello.cmd  -> hallo-von-cmd, Exit 7
    akmesh.exe sh -c '<pfad>'  -> not found
    akmesh.exe sh -c 'cmd /c "..."' -> laeuft, aber Quoting will sorgfaeltig sein

Der Grund ist Go: CreateProcess kann Batchdateien nicht ausfuehren, os/exec
reicht sie nicht durch. Der Ausweg ist immer cmd /c.

Mit gesetzter Policy startet die Shell auf Windows gar nicht:

    akmesh: policy ...: a filesystem policy cannot be enforced on windows yet
    (landlock is a linux feature)

Das ist Absicht: lieber verweigern als sich ungeschuetzt ausgeben. Ohne
AKMESH_POLICY laeuft alles normal.

## git in diesem Fall

git liess sich nicht testen, weil auf diesem Windows kein git installiert ist
("where git" findet nichts, unter C:\Program Files kein git.exe). Die
Auflösung ueber PATH ist aber belegt, ein installiertes git waere also
erreichbar: akmesh.exe sh -c 'git --version'.

## Was das fuer den Harness heisst

Ein akmesh auf Windows ist eine echte Schnittstelle zu Windows-Werkzeugen: git,
powershell, eigene .exe. Was fehlt, ist die Confinement-Schicht; die gibt es
dort nur ueber die Policy nicht. Wer Begrenzung braucht, muss unter Windows
entweder auf die Job-Object-Erweiterung warten (ApplyToChild ist der Haken
dafuer) oder akmesh ohne Policy als vertrauensvolles Werkzeug betreiben.
