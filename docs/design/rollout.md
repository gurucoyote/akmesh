# Rollout, Deployment und Management fuer apk-Pakete

Stand 24.11.2026, Ticket acmesh-o6ja. Kein Code, sondern die Entscheidungen und
der Zuschnitt. Jeder Satz, der mit "gemessen" beginnt, ist in
docs/measurements.md mit dem Befehl belegt; alles andere ist Vorschlag.

## Was apk von sich aus traegt (gemessen mit apk-tools 2.14.9)

- Frische Installation nimmt die hoechste Version: "Installing rotest (2.0.0-r0)".
- Festschreiben: `apk add rotest=1.0.0-r0` macht ein Downgrade
  ("Downgrading rotest (2.0.0-r0 -> 1.0.0-r0)") und schreibt die Bedingung in
  /etc/apk/world, also `rotest=1.0.0-r0`.
- Ein so festgehaltener Stand bleibt: `apk upgrade` meldet "OK: 0 MiB in
  1 packages" und laesst ihn stehen.
- Pin loesen geht nur ausdruecklich: `apk add rotest=2.0.0-r0` schreibt die neue
  Bedingung in world. Ein blosses `apk add rotest` laesst den Pin stehen.
- Kanaele per Repo-Etikett: `@stable /pfad/zum/repo` in /etc/apk/repositories,
  dann `apk add rotest@stable`; world traegt `rotest@stable`. Gemessen mit 2.14.9.
- Soll-Zustand steht in /etc/apk/world, Ist-Zustand in /lib/apk/db/installed.

Damit sind drei der acht Punkte im Ticket im Kern schon vorhanden: Staffelung
per Etikett oder Repo (Kanaele), Rueckweg per Versionsbedingung, und der
Ist-Zustand, der sich auslesen laesst. Was fehlt, ist der Ablauf drumherum.

## Vorschlag fuer den Zuschnitt

Drei kleine Bausteine, jeder ein eigenes Ticket, keiner im akmesh-Binary
versteckt, keiner mit Host-Spuren (Windows bleibt lokal):

1. Kanal-Dateien statt Logik. Ein Repo je Kanal (stable, testing), aufgebaut
   durch apkbuild. Auf dem Ziel steht die Kanalwahl in /etc/apk/repositories und
   die Wahl je Paket in world. Kein neues Programm noetig, apk kann es.

2. Bericht je Host, nicht je Cloud. Ein Befehl schreibt eine Zeile in eine
   eigene Datei: Hostname, Zeitpunkt, Paketname, Version, Ergebnis je Paket, und
   beim Update zusaetzlich die Ausgabe von apk. Format: eine JSON-Zeile pro
   Paket und Lauf, damit es ohne Parser lesbar bleibt. Quelle ist
   /lib/apk/db/installed; Grundlage ist `apk info -v` (haben wir).

3. Rueckfall als Befehl, nicht als Automat. Ein Skript nennt die letzte
   bekannte gute Version (aus der Berichtsdatei) und ruft
   `apk add pkg=version`. Ob es das von selbst tun soll, ist eine
   Entscheidung (siehe unten), aber der Befehl muss da sein und geuebt.

Was bewusst nicht gebaut wird: kein eigenes Delta-Format (jedes Paket kommt
ganz, Groessen werden vorher gemessen), keine eigene Schluesselverteilung im
Binary (Signatur bleibt Sache von apkbuild und apk), kein Dienst, der sich
selbst installiert.

## Schluesselverwaltung

Gemessen ist nur, dass apk einem Schluessel im Zielverzeichnis vertraut, solange
sein Name zur Signatur passt. Fuer Rotation heisst das: neuer Schluessel und
Signatur mit beiden Namen eine Zeit lang parallel, alter Schluessel erst aus
/etc/apk/keys entfernen, wenn alle Hosts den neuen Stand haben. Eine Sperrliste
muss der Host fuehren (Datei unter /etc/apk/keys), nicht der Server: der Server
kann einem kompromittierten Host nichts befehlen.

## Offene Entscheidungen (Martin)

1. Wer loest ein Update aus: ein Mensch per Befehl, ein Zeitplan, oder beides
   (Zeitplan fuer den Kanal, Handgriff fuer den Sprung zwischen Kanaelen)?
2. Wenn der Health-Check nach dem Update scheitert: automatisch auf die letzte
   gute Version zurueck, oder stehenbleiben, melden und den Menschen holen?

Alles andere in diesem Dokument laesst sich unabhaengig davon bauen; diese zwei
Punkte bestimmen, ob Baustein 3 ein Befehl oder ein Waechter wird.
