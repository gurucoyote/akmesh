# Policy: was die Shell im Harness anfassen darf

Das ist der Grund, warum akmesh existiert. Ein Harness soll dem Agenten nicht
die echte Shell geben, sondern eine, in der die Regeln an einer Stelle gesetzt
und ohne Raten durchgesetzt werden.

## Wo die Regel sitzt

Die Regel sitzt nicht in den Befehlen, sondern im Kernel. Beim Start liest
akmesh die Datei aus `AKMESH_POLICY`, baut daraus eine Landlock-Regel und
wendet sie auf sich selbst an. Landlock ist eine Linux-Eigenschaft: ab diesem
Moment gilt die Regel fuer diesen Prozess und fuer jedes Kind, das er startet.

Das heisst:

- Ein Befehl, der eine Datei ausserhalb der Liste oeffnet, bekommt
  "permission denied" - vom Kernel, nicht von uns.
- Ein Kindprozess, den die Shell startet, erbt die Regel.
- Es gibt keinen Befehl, der die Regel umgehen kann, weil er sie nicht kennt.
- Wir muessen in keinem Werkzeug raten, welche Argumente wohl Pfade sind.

## Die Datei

```json
{
  "version": 1,
  "read": ["/work/readonly"],
  "write": ["/work/out"],
  "exec": ["/work/bin"],
  "network": ["example.com", "*.testbild.tv:443"],
  "system_access": true
}
```

- `read`: Pfade, die gelesen und aufgelistet werden duerfen. Ein Ordner gilt
  fuer alles darunter.
- `write`: Pfade, die geaendert werden duerfen. Schreiben schliesst Lesen ein.
- `exec`: Pfade, deren Programme gestartet werden duerfen.
- `network`: Hosts, die curl und wget erreichen duerfen, als `host` oder
  `host:port`; ein fuehrendes `*.` erlaubt eine Domain mit allen Unterdomains.
  Eine leere Liste sperrt das Netz ganz.
- `system_access`: erlaubt Lesen und Ausfuehren von /usr, /bin, /sbin, /lib,
  /etc/ld.so.cache und den wenigen Geraeten, die ein Programm zum Starten
  braucht. Standard ist true. Ohne das kann kein Programm starten.
- `version`: muss 1 sein.

Unbekannte Felder werden abgelehnt, relative Pfade werden abgelehnt, und eine
Datei ohne read und write wird abgelehnt. Ein Tippfehler soll nicht still mehr
oeffnen als gedacht.

## Fehlverhalten ist geschlossen

Wenn `AKMESH_POLICY` gesetzt ist und die Datei nicht lesbar, nicht gueltig oder
nicht durchsetzbar ist, beendet sich akmesh mit Exit 1 und einer Meldung. Es
laeuft nicht ungeschuetzt weiter. Das ist die einzige vertretbare Richtung.

Ohne `AKMESH_POLICY` ist akmesh ein normaler Busybox-Ersatz ohne Regeln.

## Nachsehen und pruefen

    akmesh policy                    # was gilt, Exit 0 wenn durchgesetzt
    akmesh policy --check /etc/shadow # versucht wirklich zu lesen

`policy` zeigt die Datei, die Aufzaehlung der Pfade und die Kernel-Version
(Landlock ABI). `--check` macht einen echten Leseversuch in diesem Prozess: Exit
0 heisst erlaubt, Exit 1 heisst verweigert.

## Die Shell

`akmesh sh` (auch als `gosh` erreichbar) ist mvdan/sh als Bibliothek, also eine
POSIX-nahe Shell mit den gewohnten Dingern: Variablen, Arithmetik, Pipes,
Umleitungen, `&&`, `||`, Funktionen, `case`, `for`. Sie ersetzt die u-root-gosh
und ist dabei kleiner, weil die Zeilenbearbeitung nicht mitkommt.

Die Shell prueft selbst nichts. Sie braucht es auch nicht: die Regel gilt schon,
bevor sie den ersten Befehl liest. Ein `cat /etc/shadow` scheitert also nicht,
weil wir `cat` besonders behandeln, sondern weil der Kernel das Oeffnen
verweigert.

## Windows

Landlock gibt es nur unter Linux. Auf Windows laesst sich dieselbe Regel heute
nicht durchsetzen. Deshalb gilt: ist `AKMESH_POLICY` gesetzt, verweigert das
Windows-Binary den Dienst mit Exit 1 und der Meldung, dass eine Dateisystem-Policy
dort nicht durchgesetzt werden kann. Eine Attrappe, die so tut als ob, waere
schlimmer als eine ehrliche Absage. Ticket `acmesh-94e.3` haelt den Weg ueber
Job Objects und eingeschraenkte Token fest.

## Grenzen, die man kennen muss

- Die Regel gilt fuer den Prozessbaum, nicht fuer das ganze System. Ein Prozess
  ausserhalb der Shell sieht die Regel nicht. Der Harness muss den Agenten also
  durch akmesh laufen lassen, nicht daneben.
- Netz wird auf Host-Ebene geprueft, in curl und wget, nicht im Kernel. Landlock
  kann nur Ports, keine Namen. Ein Programm, das selbst Sockets oeffnet, ist
  damit nicht gebunden; das waere eine eigene Stufe (Landlock-Netz oder
  Namespaces).
- `kill` und `ps` sind nicht eingeschraenkt. Wer Signale schicken darf, ist eine
  andere Frage als wer Dateien lesen darf.
- Ein Pfad, der beim Start nicht existiert, wird uebersprungen und gemeldet. Ein
  spaeter angelegter Ordner ist damit nicht erlaubt, bis der Prozess neu startet.

## Nachweis

Getestet mit der Testbatterie (scripts/test-akmesh.sh), Linux:

- policy meldet Exit 0 und nennt die Datei.
- Lesen in /work: erlaubt und liefert den Inhalt.
- Lesen von /etc/hostname: verweigert.
- Schreiben ausserhalb: verweigert, die Datei entsteht nicht.
- Schreiben innerhalb: erlaubt.
- curl auf example.com bei leerer Netzliste: Exit 1 und
  "does not allow network access".
- Kaputte Policy: Exit 1, kein ungeschuetzter Lauf.
- Auf Windows: Policy gesetzt ergibt Exit 1 mit Begruendung.

Dazu Go-Tests: Policy-Parsing samt Ablehnung von Tippfehlern, relative Pfade,
falsche Version; Host-Abgleich mit Ports und Wildcard; fehlende Policy sperrt
nichts; kaputte Datei setzt nichts.
