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
  login. Dazu `[interop] enabled = true`. `[automount]` steht bewusst auf
  `enabled = false`, siehe "Der C:-Mount" unten.
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

## Der C:-Mount: warum automount aus ist

Beim ersten Start kam bei jedem Aufruf:

    wsl: Failed to mount C:\, see dmesg for more details.

Untersucht, mit Belegen:

- WSL ist 3.0.5.0, Kernel 6.18.58.1. Die Distribution startet dabei sauber, nur
  `/mnt/c` bleibt leer.
- In der funktionierenden `golang`-Distro steht in `/proc/mounts`
  `9p ... aname=drvfs;path=C:\;...;trans=fd`. Es ist also ein 9p-Mount mit einem
  Deskriptor, den WSL bereitstellt.
- WSL ruft dafür **nicht** das `mount` der Distribution auf: eine Sonde an
  `/bin/mount`, die jeden Aufruf mitschrieb, wurde kein einziges Mal
  ausgeführt - auch nicht, als die Sonde `drvfs` in ihrer Hilfe nannte.
- Ohne unser `mount` (Datei entfernt) bleibt der Fehler derselbe. Er hängt also
  nicht an unserer Implementierung, sondern an WSL.
- Unser `mount` kennt `drvfs` ohnehin nicht: `mount -t drvfs C: /mnt/c` endet mit
  "unknown filesystem drvfs" und dann mit ENODEV.

Mit `enabled = false` ist der Start still, und es entsteht kein leerer Ordner,
der wie ein funktionierendes Laufwerk aussieht. Windows-Interop fällt damit
weg, weil es ohne `/mnt/c` keinen Pfad zu einem Windows-Programm gibt. Ein
Ticket hält den offenen Punkt fest.

## Der Start in WSL, gemessen

    wsl --import akmesh C:\temp\akmesh \\wsl.localhost\golang\root\akmesh\akmesh-wsl.tar

    wsl -d akmesh -- id                            uid=1000(akmesh) gid=1000(akmesh)
    wsl -d akmesh -- sh -c 'echo $USER $HOME'      akmesh /home/akmesh
    wsl -d akmesh -- cat /etc/passwd               root, akmesh, nobody
    wsl -d akmesh -- uname -s -r                   Linux 6.18.58.1-microsoft-standard-WSL2
    wsl -d akmesh -- akmesh curl -s -o /dev/null \
        -w '%{http_code}' https://example.com      https=200

`/etc/resolv.conf` schrieb WSL beim Start selbst: `nameserver 10.255.255.254`
plus `search busys.local fritz.box` - dieselben Werte wie in Ubuntu und golang.
DNS und HTTPS funktionieren damit ohne jede Netzwerkkonfiguration im Image.

## Was geprüft ist und was nicht

Geprüft im Rootfs selbst, per `chroot`, vor dem Verpacken:

- `sh` antwortet, `PATH` kommt aus dem Profil,
- `/etc/passwd` hat drei Zeilen, der Nutzer `akmesh` steht darin, sein Home
  existiert,
- `awk`, `grep`, `ls /etc` über akmesh, `uname`,
- und der Punkt, an dem ein falsch geschriebener Hash auffällt:
  `su akmesh -c 'id -un'` mit eingegebenem Passwort endet als `akmesh`.

Der Import und der Start sind inzwischen auch gelaufen (siehe oben), mit dem
Zielordner `C:\temp\akmesh`. Offen bleibt allein der C:-Mount.

## Ein Befund am Rande

Unser `sh` versteht `sh -l -c '...'`, aber nicht die zusammengezogene
Schreibweise `sh -lc '...'`, die ash und bash annehmen. Aufgefallen beim
Schreiben der Prüfung oben; dort steht jetzt die getrennte Form, damit die
Prüfung das Image misst und nicht die Shell.
