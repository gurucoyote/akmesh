# Roadmap: akmesh als Container-Image

Stand 22.11.2026. Jede Zahl stammt aus docs/measurements.md, jeder offene
Schritt hat ein Ticket. Die Roadmap ist nach Stufen geordnet: jede Stufe hat
ein pruefbares Ende, und keine Stufe setzt eine spaetere voraus.

## Ist-Stand (gemessen)

- Statischer Bau (CGO_ENABLED=0) 17092732 Bytes, laeuft in einem leeren Image
  (per podman import aus einer leeren tar); der dynamische Bau scheitert dort
  an fehlendem ld-musl.
- Im leeren Image laufen ls, sort, ps (mit gebundenem /proc), wc, date, id.
- Systempaket 1 (acmesh-hdb) ist drin: mount, umount, mknod, blkid, losetup;
  mit --privileged gemessen (tmpfs, Geraetedatei, Schreiben darauf).
- Es fehlt: kein init als PID 1, kein ip/dhclient, kein /dev-Aufbau, kein
  Resolver und kein CA-Bundle, kein login/cron/syslog.
- Groesse 21,2-fach ueber BusyBox, Start 6,84 ms je Aufruf (BusyBox 0,86 ms).

## Zwei Wege, und der billigere zuerst

Weg 1, das gemischte Image: BusyBox bleibt PID 1 und /bin/sh, akmesh liegt
daneben als Werkzeug fuer den Agenten. Kostet heute kein neues Feature, weil
busybox ash, init, ip und mdev mitbringt und akmesh nur ergaenzt, was BusyBox
nicht hat (rg, jq, file, patch, die Konverter, policy). Empfehlung fuer alles,
was in den naechsten Wochen laufen soll.

Weg 2, das reine akmesh-Image: akmesh ersetzt BusyBox ganz. Braucht Stufe B
bis D und ist erst danach ehrlich zu empfehlen.

## Stufe A: Symlink-Baum und Image bauen

Heute entsteht das Image von Hand (Symlinks anlegen, podman import). Das ist
kein Produkt, sondern eine Probe.

- Ein Skript scripts/make-image.sh: statisch bauen, Applet-Symlinks aus der
  Befehlsliste erzeugen (eine Quelle, nicht zwei Listen), OCI-Image bauen
  (Containerfile oder podman import), Tag aus dem Produktnamen.
- Der Symlink-Baum muss aus scripts/cmds.txt plus OURS_ALL entstehen, sonst
  fehlt nach dem naechsten Befehl wieder ein Name.
- Ende der Stufe: `podman run akmesh:dev sh -c 'ls /; printf "b\na\n" | sort'`
  antwortet ohne Bind-Mounts und ohne PATH-Gebastel.
- Ticket: acmesh-35q.

## Stufe B: Netz im Container (acmesh-iey)

- ip (u-root, netlink: address, link, route, neigh, rule, netns) und dhclient
  aufnehmen; beide nur Linux, also mit dem Praefix `linux:`.
- Dazu gehoert, was Netz erst nutzbar macht: /etc/resolv.conf (dhclient
  schreibt sie), ein CA-Bundle im Image, sonst endet curl mit Exit 60.
- Ende der Stufe: im Image mit Netz laedt `curl https://example.com` Exit 0,
  und `ip addr` zeigt die Adresse, die dhclient geholt hat.

## Stufe C: init als PID 1 (acmesh-b97)

Ohne PID 1, das Zombies erntet und Signale weiterreicht, bleibt jeder Container
haengen, sobald ein Kindprozess endet.

- Der u-root-init hat weder SIGCHLD-Handling noch wait4 (Quelltextpruefung
  22.11.); entweder forken oder bewusst BusyBox/tini als PID 1 setzen.
- Ende der Stufe: ein Skript startet drei Kinder, die sofort enden; `ps -e`
  zeigt danach keine Z-Zeile, und ein SIGTERM an PID 1 beendet den Container in
  unter einer Sekunde.

## Stufe D: Ausgabe und Rechte wie BusyBox (acmesh-u38)

- mknod: 0666&Umask statt 0660&Umask.
- blkid: `/dev/sdX: UUID="..." TYPE="..."` statt nackter Pfade.
- losetup: `-f` (freies Loop-Geraet) fehlt.
- Ende der Stufe: die drei Aufrufe sind bytegleich mit BusyBox 1.37.0 im
  Alpine-Container, je ein Vergleichsfall in den Go-Tests.

## Stufe E: Groesse im Griff (acmesh-y2o als Vorarbeit)

- Ein Profil `image` in scripts/profiles/: keep-Liste mit dem, was ein Agent im
  Container braucht; das Profil gibt es schon als Mechanik (ext/fs).
- Zahlen: heute 17,1 MB dynamisch, 17,09 MB statisch; ein keep-Profil schneidet
  die Konverter und die Netzwerkbefehle heraus, wenn sie nicht gebraucht werden.
- Ende der Stufe: ein Image unter 10 MB, das die Batterie in einem eigenen,
  kleineren Umfang besteht; die Zahl steht in docs/measurements.md.

## Stufe F: Policy im Image (vorhanden, ungemessen im Container)

- AKMESH_POLICY wird per Landlock vor jedem Befehl angewandt, auch fuer Kinder.
- Ende der Stufe: im Image schreibt ein Lauf mit Policy auf einen gesperrten
  Pfad: Exit nonzero, keine Ausgabe, kein stiller Fallback auf ungeprueft.
- Braucht Kernel >= 5.13; im Image pruefen, nicht annehmen.

## Stufe G: Betrieb

- acmesh-696: die Batterie gegen akmesh.exe (Windows-Ziel) einmal fahren.
- Ein Lauf der Batterie gegen das IMAGE, nicht gegen das Host-Binary: dieselbe
  Pruefung, andere Umgebung (das ist die Luecke, die die leere Box aufdeckt).
- Referenz-Image bleibt debian:stable-slim@sha256:cd9f2189... fuer die
  GNU-Vergleiche; das Produkt-Image ist davon getrennt.

## Was diese Roadmap nicht verspricht

- Kein passwortbasierter Login, kein cron, kein syslog (acmesh-8v3 entscheidet
  das, nicht diese Roadmap).
- Kein ash: akmesh faehrt gosh. Skripte mit ash- oder bash-Eigenheiten laufen
  anders; das ist ein Dialekt, kein Bug.
- Keinen Ersatz fuer den Distributions-Userland ausserhalb des Containers.
