# akmesh als WSL-Distribution

`./scripts/make-wsl-image.sh` baut `akmesh-wsl.tar`: ein Rootfs, das Windows mit
`wsl --import` als eigene Distribution aufnimmt. Damit lässt sich akmesh als
tägliche Shell testen, ohne dem Harness eine einzelne Datei unterzuschieben.

## Was im Image ist

- `/bin/akmesh`, statisch gelinkt und gestrippt, dazu ein Symlink je Befehl. Die
  Namensliste liest das Skript aus dem Binary selbst (`__akmesh_list_commands__`),
  es gibt also keine zweite Liste, die driften könnte. Zur Zeit 143 Namen.
- `/usr/bin`, `/sbin`, `/usr/sbin`, `/usr/local/bin`, `/usr/local/sbin` als
  Symlinks auf `/bin`. Nötig, weil jedes apk-Installationsskript zuerst `mkdir`
  ruft; ohne diese Pfade scheitert es an einem PATH, der im Rootfs nirgends
  hinführt.
- Ein echter Nutzer, nicht nur root:

      akmesh:x:1000:1000:akmesh:/home/akmesh:/bin/sh

  Das Passwort ist `akmesh`, abgelegt als SHA-512-crypt-Hash aus unserem eigenen
  `mkpasswd`; der Klartext liegt nirgends im Image. Root bleibt gesperrt (`!`),
  wie im Container-Image: `wsl -u root` kommt ohnehin hinein, und ein gesperrtes
  Konto lügt nicht.
- `/etc/wsl.conf` mit `[user] default=akmesh`. Das ist der ganze Auto-Login: WSL
  startet die Shell des eingetragenen Nutzers, es braucht kein getty und kein
  login. Dazu `[automount]` wie in den anderen Distros (`root = /mnt/`,
  `mountFsTab = false`) und `[interop] enabled = true`, damit `wsl.exe` und
  Windows-Programme von innen weiter funktionieren.
- `/etc/nsswitch.conf` mit `hosts: files dns`, `/etc/profile` samt `PS1` und
  PATH, ein `.profile` im Home, und der CA-Bundle des Hosts, damit curl und wget
  HTTPS sprechen.

## Netzwerk: genauso wie Ubuntu und golang

Nachgemessen am 25.11.2026 an den beiden Distros auf diesem Rechner:

- `Ubuntu-24.04` hat in `/etc/wsl.conf` nur `[boot] systemd=true`,
- `golang` nur `[automount]`,
- beide bekommen `/etc/resolv.conf` von WSL geschrieben, mit
  `nameserver 10.255.255.254` (der NAT-Gateway von WSL2) und den
  Windows-Suchdomänen, und beide haben `hosts: files dns`.

Deshalb konfiguriert das Image kein Netzwerk selbst. Es setzt
`generateResolvConf = true` und `generateHosts = true`, also genau das, was die
anderen Distros auch tun, und legt nur einen Platzhalter hin, der sagt, dass WSL
die Dateien beim Start überschreibt. Ein eigener Resolver im Image wäre der
Fehler: er würde beim ersten Start still ersetzt.

## Importieren und wieder entfernen

Der Import geht von Windows aus, mit Windows-Pfaden:

    wsl --import akmesh C:\WSL\akmesh \\wsl.localhost\<distro>\root\akmesh\akmesh-wsl.tar
    wsl -d akmesh

`<distro>` ist die Distribution, in der das tar gebaut wurde (hier: `golang`);
`wslpath -w akmesh-wsl.tar` im Bauverzeichnis nennt den Pfad. Ist der UNC-Pfad
zu langsam, tut es auch eine Kopie an einen Ort deiner Wahl - dann steht der
Pfad direkt im Befehl.

Entfernen, ohne Rückstände:

    wsl --unregister akmesh

Damit ist die Distribution samt ihrer VHD weg; im Registry bleibt nichts, was
nicht auch ein normaler `wsl --import` angelegt hätte.

## Was geprüft ist und was nicht

Geprüft im Rootfs selbst, per `chroot`, vor dem Verpacken:

- `sh` antwortet, `PATH` kommt aus dem Profil,
- `/etc/passwd` hat drei Zeilen, der Nutzer `akmesh` steht darin, sein Home
  existiert,
- `awk`, `grep`, `ls /etc` über akmesh, `uname`,
- und der Punkt, an dem ein falsch geschriebener Hash auffällt:
  `su akmesh -c 'id -un'` mit eingegebenem Passwort endet als `akmesh`.

Nicht geprüft, weil es Windows braucht: der erste Start über `wsl --import`.
Der Import lässt sich nicht aus einer WSL heraus ohne Zielverzeichnis auf der
Windows-Seite fahren, und ein Verzeichnis auf `C:` legen wir nicht ungefragt an.

## Ein Befund am Rande

Unser `sh` versteht `sh -l -c '...'`, aber nicht die zusammengezogene
Schreibweise `sh -lc '...'`, die ash und bash annehmen. Aufgefallen beim
Schreiben der Prüfung oben; dort steht jetzt die getrennte Form, damit die
Prüfung das Image misst und nicht die Shell.
