# ssh und scp

Stand 18.11.2026. Beide sind eigene Werkzeuge in internal/tools, gebaut auf
golang.org/x/crypto/ssh, und auf beiden Zielen enthalten. Die alte ssh aus
u-root ist raus.

## Warum ersetzt

Die u-root-ssh konnte keinen Port aus der Konfiguration, hatte kein -p, und der
Exit-Status des entfernten Befehls kam nicht an: `ssh host 'exit 7'` endete mit
Exit 0 und schrieb stattdessen eine Zeile mit Zeitstempel. Fuer einen Agenten ist
das die schlimmste Sorte Fehler, weil ein `ssh host 'test -f datei'` immer wie
Erfolg aussieht.

## Non-interactive

Das ist der Punkt, auf den es bei Agenten ankommt:

- Ist stdin kein Terminal, wird kein PTY angefordert und kein Passwort gefragt.
  Fehlt ein brauchbarer Schluessel, ist das ein Fehler mit Exit 255, keine
  Rueckfrage.
- BatchMode ist dann der Standard; `-o BatchMode=yes` setzt es auch bei einem
  Terminal.
- Der Exit-Status des entfernten Befehls ist der Exit-Status von ssh.
- stdin wird durchgereicht, also `cat datei | acmesh ssh host 'cat > ziel'`.
- Auf stderr landet nur, was wirklich eine Meldung ist, ohne Zeitstempel.

Belegt:

    acmesh ssh -p 2222 -i key root@127.0.0.1 'exit 7'   -> Exit 7
    acmesh ssh -p 2222 -i key root@127.0.0.1 'test -f /etc/hostname' -> Exit 0
    echo x | acmesh ssh -p 2222 -i key root@127.0.0.1 'cat'         -> x

## Schalter

    -p PORT         Port (auch host:port im Ziel)
    -l USER         Benutzer
    -i KEY          Schluessel, mehrfach moeglich
    -o Key=Value    Port, User, IdentityFile, StrictHostKeyChecking=no|accept-new,
                    UserKnownHostsFile, BatchMode, ConnectTimeout
    -F FILE         Konfigurationsdatei (Standard ~/.ssh/config)
    -t / -T         Terminal erzwingen / keins
    -v              ausfuehrlich

Aus der Konfiguration gelesen werden HostName, Port, User, IdentityFile,
StrictHostKeyChecking, UserKnownHostsFile und ConnectTimeout. Unbekannte
Schluessel werden ignoriert, nicht als Fehler behandelt.

known_hosts wird streng geprueft. Fuer einen ersten Kontakt entweder
`-o StrictHostKeyChecking=no` oder `-o StrictHostKeyChecking=accept-new`.

## scp

Eigene Umsetzung des scp-Protokolls ueber dieselbe Verbindung, also dieselben
Schalter und dieselbe Konfiguration. Auf der Gegenseite genuegt das scp, das bei
OpenSSH dabei ist.

    acmesh scp -P 2222 -i key datei.txt root@host:/tmp/
    acmesh scp -r -P 2222 -i key ordner root@host:/tmp/
    acmesh scp -P 2222 -i key root@host:/tmp/datei.txt ./ziel/

- `-P` ist der Port wie bei OpenSSH scp. `-p` erhaelt Modus, Aenderungs- und
  Zugriffszeit, in beide Richtungen und auch rekursiv.
- `-r` uebertraegt Baeume in beide Richtungen.
- `-i`, `-o`, `-F`, `-v` wie bei ssh.
- Ein Pfad mit Doppelpunkt vor dem ersten Schraegstrich ist entfernt:
  `host:datei`, `user@host:datei`.
- Von entfernt nach entfernt geht nicht; die Meldung sagt, dass die Kopie ueber
  diese Maschine laufen soll.

Nicht enthalten, mit Grund:

- `-C` (Kompression): Die Kompression von OpenSSH sitzt im SSH-Transport, nicht
  im scp-Protokoll. golang.org/x/crypto/ssh kann sie nicht aushandeln
  (supportedCompressions enthaelt nur "none"). Deshalb lehnt der Befehl mit
  einer Meldung ab, statt still unkomprimiert zu uebertragen.
- Fortsetzen: Das ist kein scp-Schalter, auch OpenSSH scp kennt keins (-c ist
  dort die Chiffre). Wer fortsetzen will, nimmt rsync oder das reget in sftp.
  Ein eigener sftp-Client waere ein eigenes Stueck Arbeit, nicht ein Schalter.

Belegt fuer -p: eine Datei mit Zeitstempel 2020-01-02 wird hochgeladen und
kommt mit genau diesem Zeitstempel an, in beide Richtungen; ein Modus 600 bleibt
600; rekursiv bleiben die Zeiten der Kinder erhalten. Ohne -p bekommt die Kopie
die aktuelle Zeit, wie bei OpenSSH.

## Nachweis

scripts/test-ssh.sh startet einen echten sshd auf einem freien Port mit eigenem
Hostschluessel und eigenem Client-Schluessel und prueft 15 Faelle: Ausgabe,
mehrzeilige Ausgabe, Remote-Exitcode, test -f wahr und falsch, stdin-Pipe,
unbekannter Host ohne Freigabe (Exit 255), Upload, Inhalt, rekursiver Upload,
Download, Inhalt, rekursiver Download.

Ergebnis: 15 Pruefungen, 0 Fehler, mit acmesh (Linux) und mit acmesh.exe
(Windows). Der Windows-Lauf funktioniert, weil das Skript in das Arbeitsverzeichnis
wechselt und den Schluessel relativ angibt; ein Windows-Binary kann einen
Linux-Pfad wie /tmp/... nicht oeffnen, einen relativen Namen im gemeinsamen
Verzeichnis schon.

## Groesse

acmesh 11.350.140 Bytes (10,82 MiB), acmesh.exe 11.378.688 Bytes (10,85 MiB).
Gegenueber der u-root-ssh sind das 20.480 Bytes weniger, obwohl scp neu
dazugekommen ist: x/crypto war durch die alte ssh schon im Binary.
