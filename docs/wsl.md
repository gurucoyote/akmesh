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
plus `search example.local fritz.box` - dieselben Werte wie in Ubuntu und golang.
DNS und HTTPS funktionieren damit ohne jede Netzwerkkonfiguration im Image.

## Der Login-Strich: warum "wsl -d akmesh" erst scheiterte

Der erste interaktive Start kam als

    Failed to run command: command is not present in akmesh: -sh

WSL startet die Shell aus /etc/passwd als Login-Shell und übergibt dabei
`argv[0] = "-sh"`. Unser Verteiler (gobusybox) schlug den Namen wörtlich nach und
fand ihn nicht. Behoben in `scripts/build-bb.sh`, an beiden Stellen, die den
Namen bilden: der erste Versuch aus argv[0] und der Rückfall aus argv[1] bei
"akmesh command". Der Strich wird nur für das Nachschlagen abgeschnitten,
`os.Args[0]` bleibt "-sh", damit unser sh weiterhin eine Login-Shell erkennt und
/etc/profile liest. Der Build prüft, ob die Änderung in makebb angekommen ist,
und bricht sonst ab.

## Pakete: Alpine edge, mit echten Paketen

Das Image bringt `/etc/apk/repositories` mit edge main und community und
Alpines eigene öffentliche Schlüssel in `/etc/apk/keys`. Damit installiert unser
eigenes apk echte Alpine-Pakete, mit Signaturprüfung:

    apk update        OK: 28559 distinct packages available
    su -c "apk add tree"    tree 2.3.2 mit musl 1.2.6, laeuft
    su -c "apk add go"      go1.27.4, 752.8 MiB in 17 Paketen, "go version" geht
    su -c "apk add rust cargo"  rustc 1.100.0 und cargo 1.100.0, 870.9 MiB in 24

Warum edge: es ist der rollende Zweig, und dort liegen go und rust in den
Fassungen, die man heute erwartet.

Für `su` musste noch etwas passieren: ohne setuid-Bit kann ein normaler Nutzer
weder `/etc/shadow` lesen noch root werden, also scheiterte jedes `apk add` an
der Passwortabfrage ("su: Authentication failure", gemessen). Alpine liefert
busybox su genau deshalb setuid aus. Hier ist `/bin/su` jetzt ein eigenes
kleines Binary (3,1 MB, aus `cmd/su` gebaut) mit Modus 4755. Root hat dafür ein
Passwort: dasselbe Initialpasswort wie der Nutzer, aenderbar mit `passwd`.

Dabei fiel auf, dass unser chmod alles über 0777 ablehnte, also auch `4755` und
`u+s`. Das ist behoben: setuid, setgid und sticky gehen jetzt als Oktalzahl und
in der symbolischen Form. Das Entfernen (`u-s`, `u-t`) meldet noch ausdrücklich,
dass es nicht unterstützt wird, statt es still zu tun.

## Der Prompt, und warum die Sitzung im Home beginnt

Zwei Zeilen, die Form, die Martin auch in seiner golang-Distro hat:

    HOST:~ 
    akmesh$ 

Die erste Zeile nennt Rechner und Verzeichnis (mit ~ fuer das Home), die zweite
Nutzer und Zeichen ($ fuer den Nutzer, # fuer root). Dazu setzt die Shell den
Fenstertitel auf "rechner verzeichnis"; ein Terminal schluckt das, ein
Screenreader liest es nie vor. Das /etc/profile im Image setzt dafuer
PS1='\e]0;\h \w\a\h:\w\n\u\$ '; ohne Profil (z.B. im Container) baut die
Shell denselben Text selbst.

Dafuer musste ein echter Fehler weg: WSL startet die Shell als Login-Shell mit
argv[0]="-sh", der Verteiler schnitt den Strich fuer die Namenssuche ab, und
/bin/sh bekam nur "sh". Die Shell hielt sich damit fuer keine Login-Shell, las
kein /etc/profile - und deshalb fehlten PS1 und der Sprung ins Home. cmd/sh
reicht den Strich jetzt ueber AKMESH_SH_LOGIN weiter. Zwei Folgen davon waren
ebenfalls Fehler: der Prompt wurde einmal beim Start eingefroren (nach "cd" stand
weiter das alte Verzeichnis), und er wurde pro Befehl zweimal gedruckt.

Gemessen in der Distro (25.11.2026):

    /tmp/x/-sh -c 'echo PS1=[$PS1]'     PS1=[\u@\h:\w\$ ]   (Profil greift)
    printf 'pwd\nmkdir projects\nexit\n' | /tmp/x/-sh -i
        HOST:~ / akmesh$ /home/akmesh / HOST:~ / akmesh$ / projects

Das Home gehoert dabei dem Nutzer (1000:1000); vorher war es root und "mkdir
projects" scheiterte mit permission denied.


## Wenn ein neues Binary in die laufende Distro kommt

Ein Name ist in dieser Distro nicht nur im Binary, sondern auch ein Symlink in
/bin: "/bin/clear -> akmesh". Ein frisch importiertes Image legt alle Links aus
der Namensliste des Binaries an. Wird dagegen nur /bin/akmesh ersetzt, fehlt der
Link für einen neuen Namen, und die Shell antwortet "not found", obwohl der
Befehl im Binary steckt (gemessen 25.11.2026 mit clear).

Dafür gibt es scripts/wsl-links.sh: es liest die Namen aus dem Binary und legt
fehlende Links an, echte Dateien bleiben unangetastet (ein Paket bringt eigene
Programme mit). In der Distro als root:

    echo <base64 von scripts/wsl-links.sh> | base64 -d > /tmp/links.sh
    sh /tmp/links.sh
    akmesh-wsl-links: 1 new links in /bin (from /bin/akmesh)

## Tab vervollständigt

Tab ergänzt im Zeileneditor das Wort unter dem Cursor (25.11.2026):

- Erstes Wort einer Zeile: Befehlsnamen aus dem PATH der Shell.
- Jedes weitere Wort: Pfade, gesehen aus dem Verzeichnis der Shell, nicht aus
  dem, in dem der Prozess gestartet ist. Verzeichnisse bekommen einen
  Schrägstrich, ein eindeutiger Name ein Leerzeichen danach.
- Mehrere Kandidaten mit gleichem Anfang: der gemeinsame Teil wird ergänzt, sonst
  werden die Kandidaten untereinander aufgelistet und die Zeile bleibt stehen.
- Dateien mit Punkt am Anfang erst, wenn man den Punkt selbst tippt.

    cd pr<Tab>        privat/  projects/
    cd proj<Tab>      cd projects/
    pro<Tab>          Befehle aus PATH, z.B. progress

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
