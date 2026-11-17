# ssh und scp: was da ist und was nicht

Stand 18.11.2026, geprueft am gebauten Binary.

## ssh: vorhanden und funktionsfaehig

Das Binary bringt ssh mit, aus u-root, gebaut auf golang.org/x/crypto/ssh und
github.com/kevinburke/ssh_config. Auf beiden Zielen enthalten, ohne
Build-Tag-Einschraenkung.

Nachweis mit einem echten Server: ein sshd auf 127.0.0.1:2222 mit eigenem
Hostschluessel und eigenem Client-Schluessel.

    acmesh ssh -i clientkey root@127.0.0.1:2222 'echo hallo-von-acmesh'
    -> hallo-von-acmesh, Exit 0

    acmesh ssh -i clientkey root@127.0.0.1:2222 'uname -s; echo zwei'
    -> Linux
       zwei

Remote-Befehle laufen also, die Ausgabe kommt zurueck, mehrere Zeilen
inbegriffen.

## Vier Eigenheiten, die man kennen muss

1. Der Port kommt nur aus dem Ziel, nicht aus der Konfiguration.
   `user@host:2222` oder `ssh://user@host:2222` funktioniert. Aus ~/.ssh/config
   wird nur IdentityFile gelesen; ein `Port 2222` dort wird ignoriert, und dann
   verbindet es auf 22.

2. Es gibt keinen Schalter -p. Die Schalter sind nur -F, -i und -d.

3. known_hosts ist Pflicht, StrictHostKeyChecking wird ignoriert. Ein unbekannter
   Host scheitert hart mit "knownhosts: key is unknown". Fuer einen Port
   ausserhalb 22 muss der Eintrag in der Form `[host]:port` stehen; steht dort
   nur der nackte Hostname, kommt "key mismatch". Beides zugleich zu pflegen ist
   unnoetig, der Eintrag mit Port genuegt.

4. Der Exit-Status des entfernten Befehls kommt nicht an. Ein
   `ssh host 'exit 7'` endet mit Exit 0 und schreibt stattdessen eine Zeile mit
   Zeitstempel auf stderr: "failed to run command: Process exited with status 7".
   Fuer einen Agenten ist das die unangenehmste Eigenheit: er kann an einem
   `ssh host 'test -f datei'` nicht ablesen, ob die Datei da war.

## scp: nicht vorhanden

    $ acmesh scp -h
    Failed to run command: command is not present in busybox: scp

u-root bringt kein scp mit. Es gibt auch kein sftp. Der Befehl fehlt, er ist
nicht kaputt.

Wege, das zu schliessen:

- Eigenes scp auf x/crypto/ssh: das SCP-Protokoll ist klein (C/D/E/T, Byte- und
  Zeitmeldungen), braucht aber ein scp auf der Gegenseite. Rund 200 Zeilen.
- Eigenes sftp: moderner und ohne scp auf der Gegenseite, aber deutlich mehr
  Protokoll (Pakete, Handles, Attribute). Rund 600 Zeilen.
- Notbehelf ohne neues Protokoll: tar ueber ssh, also
  `acmesh ssh host 'cat > datei'` oder `tar -c | acmesh ssh host 'tar -x'`.
  Funktioniert heute schon, ist aber kein scp.

## Empfehlung

Ein eigener ssh-Befehl in internal/tools auf x/crypto/ssh wuerde die Punkte 1,
2 und 4 in einem Zug loesen: -p, Port aus der Konfiguration, Exit-Status
durchgereicht, Meldungen ohne Zeitstempel. Die Bibliothek ist schon im Binary,
es waeren rund 200 Zeilen und kein nennenswerter Zuwachs. scp danach als zweite
Front auf derselben Verbindung.
