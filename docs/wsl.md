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

## Der C:-Mount: wie er jetzt geht

Beim ersten Start kam bei jedem Aufruf

    wsl: Failed to mount C:\, see dmesg for more details.

und `/mnt/c` blieb leer. Die Recherche (wsl.dev/technical-documentation/drvfs,
src/linux/init/drvfs.cpp) und drei Messungen haben den Weg freigelegt:

- WSL startet die Distribution nicht mit einem eigenen Mount, sondern ruft
  `/bin/mount --internal-only -t drvfs C: /mnt/c -o <Optionen>`. Im Kernel-Log
  stand dazu `/bin/mount failed with status 0x200`, also unser Exitcode 2.
- `drvfs` ist kein Kernel-Dateisystem. WSL bedient es über den Einstiegspunkt
  `mount.drvfs` in seinem eigenen `/init`, ausgewählt über `argv[0]`
  (`MountDrvfsEntry`). Ohne diesen Umweg gibt es nur "unknown filesystem drvfs".
- `/init` ruft bei diesem Umweg erneut `/bin/mount` auf, diesmal mit
  `-i -t 9p ... -o trans=fd,rfd=3,wfd=3`. Der Deskriptor 3 ist der Plan9-Server,
  den WSL bereitstellt.

Deshalb gibt es jetzt unseren eigenen `mount` (internal/mount, Linux-only,
acmesh-2p6w) statt u-roots:

- `-t drvfs` wird an `/init` mit `argv[0] = mount.drvfs` übergeben.
- util-linux-Flags, die hier nichts ändern (`-i`, `-n`, `-v`, `-f`, `-s`, `-w`,
  `-l`, `-a`, `--internal-only`), werden angenommen. Ohne das wurde `-i` als
  Quellpfad gelesen und der Mount landete auf dem falschen Pfad.
- Alles andere geht als `mount(2)` raus, Optionen wie bei mount(8) aufgeteilt.

Damit hängt WSL C: beim Start von selbst ein, `ls /mnt/c` zeigt die Platte, und
`[interop] appendWindowsPath = true` funktioniert wieder: ein Windows-Programm
startet aus der Distribution heraus.

Nicht mehr dabei, was u-roots mount hatte: `-o loop`, das Schreiben von mtab und
das Abarbeiten von `/etc/fstab` ohne Argumente. Für den Fall, um den es hier
geht, braucht es keines davon; was fehlt, fehlt bewusst.

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
