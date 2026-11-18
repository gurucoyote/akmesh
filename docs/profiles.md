# Profile: was beim Bauen drin ist

Der Bau nimmt als zweites Argument ein Profil aus scripts/profiles. Ein Profil
ist eine kleine Textdatei mit zwei Zeilenarten:

    tags nonet
    exclude curl wget ssh scp

- `tags` setzt Go-Build-Tags fuer den eigenen Code.
- `exclude` nimmt Namen aus der u-root-Liste und aus unseren Befehlen heraus.

Die mitgelieferten Profile:

- `full` (Standard): alles.
- `fs`: ohne Netz. Kein curl, wget, ssh, scp, und aus der u-root-Liste keine
  Netz-Werkzeuge. Zusaetzlich sorgt das Tag `nonet` dafuer, dass die Dateien mit
  dem Netzmotor gar nicht uebersetzt werden.

## Warum das mehr ist als Ausblenden

Bei einem reinen Ausblenden waeren die Befehle zur Laufzeit weg, der Code aber
im Binary. Deshalb haengen die Netzbefehle an eigenen Dateien mit
`//go:build !nonet`: netengine.go, curl.go, wget.go, sshfront.go, scp.go und
scptimes.go. Mit dem Tag verschwinden sie samt ihrer Importe. Nachweisbar:

    GOFLAGS=-tags=nonet go list -deps ./cmd/ext | grep -cE '^net/http$'   -> 0
    GOFLAGS=-tags=nonet go list -deps ./cmd/ext | grep -cE '^crypto/tls$' -> 0

und im Binary fehlen die Meldungen des Netzteils ("known_hosts", "HTTP error
%d for", "does not allow network access").

## Groessen

    ./scripts/build-bb.sh linux full  -> acmesh     11.354.236 Bytes (10,83 MiB)
    ./scripts/build-bb.sh linux fs    -> acmesh-fs  9.777.276 Bytes (9,32 MiB)

Das sind 1.576.960 Bytes weniger, also 1,50 MiB. Ein Profil schreibt in eine
eigene Datei (acmesh-fs, acmesh-fs.exe), damit die Varianten nebeneinander
liegen koennen.

## Pruefungen

scripts/test-ext.sh erkennt selbst, ob ein Build Netz hat (curl -h antwortet).
Der fs-Bau laeuft damit durch dieselbe Batterie: 66 Pruefungen statt 74, die
acht Netz-Pruefungen werden uebersprungen, nicht als Fehler gezaehlt.

## Ein eigenes Profil

    # scripts/profiles/nur-lesen.txt
    tags nonet
    exclude curl wget ssh scp
    exclude nc ping ip

    ./scripts/build-bb.sh linux nur-lesen

Die Grenze ist der Befehlssatz, nicht ein Schalter: was im Profil steht, wird
nicht eingebaut, und was nicht gebaut ist, kann zur Laufzeit auch nicht
aufgerufen werden.
