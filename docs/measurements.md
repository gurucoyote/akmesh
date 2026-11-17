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

    GOTOOLCHAIN=auto GOOS=windows GOARCH=amd64 makebb -o bb.exe ./cmds/core/...

- Bauzeit 18 s, Groesse 8,9 MB (9.314.304 Bytes) fuer 48 Befehle plus gosh.
- Vergleich: 16 Einzeldateien waren zusammen rund 45 MB.
- Sechs Befehle als Zwischenprobe: 2,5 MB in 7 s.

## Laeuft unter Windows (A, ausgefuehrt aus WSL per Interop)

- `bb.exe ls C:/Windows` -> Liste mit Windows-Pfaden (C:\Windows\AppReadiness ...).
- `bb.exe grep -c Microsoft C:/Windows/System32/drivers/etc/hosts` -> 2.
- `bb.exe gosh -c 'echo hallo; echo $((6*7))'` -> hallo, 42.
- `bb.exe` ohne Argument -> Liste der enthaltenen Befehle.
- `bb.exe tar` -> Optionshilfe mit -c und -create.

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
