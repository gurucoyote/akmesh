# Roadmap: akmesh als BusyBox-Ersatz und Systemkern

Stand 22.11.2026, nach Martins Entscheidung vom selben Tag: kein gemischter
Betrieb. akmesh soll BusyBox ersetzen, nicht ergaenzen, und der Kern eines
minimalen WSL- oder Docker-Images sein. Login hat Vorrang, weil erst damit
/etc/profile, ~/.profile und ~/.bash_profile greifen.

Jede Zahl stammt aus docs/measurements.md, jeder Schritt hat ein Ticket.

## Was heute gemessen ist

- Statischer Bau (CGO_ENABLED=0) 17092732 Bytes laeuft in einem leeren Image;
  der dynamische Bau scheitert dort an fehlendem ld-musl.
- Im leeren Image laufen ls, sort, ps (mit gebundenem /proc), wc, date, id;
  mount, umount, mknod, blkid und losetup sind seit acmesh-hdb drin.
- Abdeckung 71 von 283 ausgelieferten BusyBox-Applets; 27 fehlende Namen werden
  von den Init-Skripten dieses Systems wirklich aufgerufen, oben mount, ip,
  brctl.
- Groesse 21,2-fach ueber BusyBox (17105123 gegen 808712), Start 6,84 ms je
  Aufruf gegen 0,86 ms.

## Der Befund, der die Reihenfolge geaendert hat (behoben in acmesh-4g7)

Unser `sh` war kein Login-faehiger Shell-Ersatz:

- `internal/sh/sh.go` liest ohne `-c` die ganze Standardeingabe bis EOF und
  fuehrt sie einmal aus: keine Eingabeaufforderung, keine Zeilenschleife, kein
  `-l`, kein Rueckgabewert je Kommando.
- `-i` wird geparst und dann verworfen (`_ = interactive`).
- Startdateien werden nicht gelesen: kein /etc/profile, kein ~/.profile, kein
  ENV.
- u-root-gosh hilft nicht weiter: `errNotImplemented` fuer den interaktiven
  Teil, ebenfalls keine Startdateien.
- Die Bibliothek kann es: `interp.Interactive(true)` in mvdan.cc/sh/v3 v3.14.1,
  dazu eine Leseschleife und x/term (schon Abhaengigkeit).

Ohne diese Shell gibt es kein Login, und ohne Login greift keine Profildatei -
auch nicht mit /etc/passwd und einem login-Befehl.

## Stufe 0: Image-Bau als Skript (acmesh-35q)

Ohne reproduzierbaren Bau bleibt jede Messung Handarbeit.

- scripts/make-image.sh: statisch bauen, Applet-Symlinks aus scripts/cmds.txt
  plus OURS_ALL erzeugen (eine Quelle), OCI-Image bauen, Tag aus dem
  Produktnamen.
- Ende: `podman run <tag> sh -c 'ls /; printf "b\na\n" | sort'` antwortet ohne
  Bind-Mounts und ohne PATH-Gebastel.

## Stufe 1: Login-faehige Shell, damit Profile greifen (acmesh-4g7, P1) - erledigt

Gebaut am 22.11.2026: `-l`/`--login` lesen /etc/profile und dann ~/.profile
(.bash_profile nur bei bash), Prompt aus PS1, Zeilenschleife mit Fortsetzung,
Zustand bleibt ueber Zeilen, umask eingebaut. Belege in docs/measurements.md,
Abschnitt "sh: Login-Modus". Login ueber den Aufrufnamen `-sh` geht nicht, weil
der Dispatcher argv[0] auf "gosh" setzt: `-l` ist der Weg (acmesh-51b).

Der urspruengliche Befund, der diese Stufe nach vorn holte:

- Interaktive Schleife: Prompt aus PS1 (Vorgabe `# ` fuer root, `$ ` sonst),
  je Zeile parsen und ausfuehren, Exitcode merken.
- Login-Modus: `sh -l`, `sh --login` und der Name `-sh` (argv[0] mit
  Bindestrich) lesen der Reihe nach /etc/profile und dann ~/.profile.
- .bash_profile: nur lesen, wenn /bin/sh als bash aufgerufen wird - sonst
  bleibt es bei ash-Konvention (.profile). Das ist eine Entscheidung, keine
  Auslassung; sie gehoert in die Hilfe.
- ENV nur fuer nicht-interaktive Aufrufe (POSIX).
- Ende: `podman run -it <tag> sh -l` zeigt einen Prompt, ein `echo` in
  /etc/profile und eines in ~/.profile sind vor dem ersten Kommando sichtbar,
  `echo $PATH` zeigt den PATH aus dem Profil.

## Stufe 2: login und getty (acmesh-7q7) - login erledigt, getty offen

Gebaut am 22.11.2026: `login [-f] USER` liest /etc/passwd, wechselt UID/GID,
setzt HOME/USER/LOGNAME/SHELL/PATH und startet die Shell mit -l. Im Container
belegt (Datei aus ~/.profile gehoert danach dem neuen Nutzer). Ohne -f verweigert
es die Anmeldung, weil es keine Passwortpruefung gibt. Belege in
docs/measurements.md, Abschnitt "login". getty ist nicht gebaut: u-root bringt
eines in cmds/exp, es startet eine Shell ohne Nutzerwechsel.

- `login`: /etc/passwd lesen (Name, UID, GID, Home, Shell), setgid/setuid,
  Home-Verzeichnis betreten, HOME, USER, LOGNAME, SHELL, PATH setzen und die
  Shell als Login-Shell starten. Passwortpruefung gegen /etc/shadow ist eine
  eigene Entscheidung: in einem Container ohne PAM ist sie optional, in WSL
  uebernimmt Windows die Anmeldung.
- `getty`: u-root bringt eines (cmds/exp), es oeffnet ein TTY und startet eine
  Shell ohne Passwort - als Konsolenzugang brauchbar, als login-Ersatz nicht.
- Ende: im Image mit /etc/passwd und einem Home-Verzeichnis startet `login -f
  nutzer` eine Shell als dieser Nutzer, `id` zeigt die passende UID, und
  ~/.profile dieses Nutzers wurde gelesen.

## Stufe 3: Netz (acmesh-iey)

- ip (u-root, netlink) und dhclient aufnehmen, beide mit dem Praefix `linux:`
  in scripts/cmds.txt.
- Dazu /etc/resolv.conf (schreibt dhclient) und ein CA-Bundle im Image, sonst
  endet curl mit Exit 60.
- Ende: `curl https://example.com` im Image Exit 0, `ip addr` zeigt die Adresse
  von dhclient.

## Stufe 4: init als PID 1 (acmesh-b97)

Nur fuer Docker noetig: in WSL ist PID 1 der Windows-Init, dort startet
wsl.exe die Shell direkt.

- Der u-root-init hat weder SIGCHLD-Handling noch wait4 (Quelltextpruefung
  22.11.). Forken oder bewusst nicht PID 1 werden.
- Ende: drei Kinder, die sofort enden, hinterlassen keine Z-Zeile in `ps -e`;
  SIGTERM an PID 1 beendet den Container in unter einer Sekunde.

## Stufe 5: Ausgabe und Rechte wie BusyBox (acmesh-u38)

- mknod 0666&Umask statt 0660&Umask, blkid mit `/dev/sdX: UUID=... TYPE=...`,
  losetup mit `-f`.
- Ende: bytegleich mit BusyBox 1.37.0 im Alpine-Container, je ein
  Vergleichsfall in den Go-Tests.

## Stufe 6: Groesse (Vorarbeit acmesh-y2o)

- Profil `image` in scripts/profiles/ mit keep-Liste; die Mechanik gibt es.
- Ende: Image unter 10 MB, das die Batterie in einem kleineren Umfang besteht;
  Zahl in docs/measurements.md.

## Stufe 7: Policy im Container

- AKMESH_POLICY per Landlock, prozessweit, auch fuer Kinder.
- Ende: ein Lauf mit Policy schreibt auf einen gesperrten Pfad: Exit nonzero,
  keine Ausgabe, kein stiller Fallback. Kernel >= 5.13 im Image pruefen.

## Stufe 8: Betrieb

- acmesh-696: Batterie gegen akmesh.exe einmal fahren.
- Batterie gegen das IMAGE, nicht gegen das Host-Binary.
- docs/design/akmesh.md nachziehen, sobald init und login stehen.

## Was der reine Weg kostet, ehrlich

- Bis Stufe 2 ist das Image kein System, sondern eine Probe: kein Login, keine
  Profildateien, keine Netzadresse. Das ist der Preis fuer den Verzicht auf den
  gemischten Betrieb.
- Kein ash: gosh mit `interp.Interactive` ist ein Nachbau, kein ash. Skripte
  mit ash- oder bash-Eigenheiten laufen anders; das bleibt eine dauerhafte
  Grenze und gehoert in die Hilfe.
- Kein mdev und kein udev: /dev muss die Laufzeit liefern (Docker: der Daemon,
  WSL: Windows).
- cron, syslog und passwortbasierter Login entscheidet acmesh-8v3, nicht diese
  Roadmap.
