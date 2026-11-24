# rg: Archive durchsuchen wie ugrep

Status: Stufe 1 gebaut (24.11.2026, Ticket acmesh-5nn): zip/jar, tar, gz, bz2,
tar.gz, tar.bz2, Etikett, --zmax, Grenzen, Zusammenspiel mit --pre. Die
Semantik ist am 23.11.2026 gegen ein installiertes ugrep 7.8.5 (alpine, mit
zlib/bzip2/lzma/lz4/zstd/brotli/bzip3/7z/tar/cpio/zip) gemessen, nicht nur aus
der Manpage gelesen; Messungen in docs/measurements.md.

## Anlass

`akmesh rg` durchsucht Text und über `--pre any2md` auch Dokumente (docx, pdf,
epub, rtf, html). Es steigt aber nicht in Archive und komprimierte Dateien
hinab. ugrep kann das: `-z`/`--decompress`, `--zmax=NUM` bis 99 Ebenen,
Trefferpfade im Archiv in geschweiften Klammern. Genau diese zwei Ideen fehlen
uns, und der Vergleich mit rga zeigt, dass der Nutzen im Etikett liegt, nicht
im Auspacken.

Belegt an der ugrep-Manpage (Zweig master, 2026-11-23):
- "--zmax=NUM: searches the contents of compressed files and archives stored
  within archives by up to NUM expansion stages. The default --zmax=1 only
  permits searching uncompressed files stored in cpio, pax, tar, zip and 7z
  archives; compressed files and archives are detected as binary files and are
  effectively ignored. ... NUM may range from 1 to 99"
- "pathnames of files in archives are output in braces"
- Formate des Originals: cpio, pax, tar, zip, 7z sowie gz, bz2, xz, lzma, zstd
  (README zusätzlich lz4, brotli)

## Ziel

`akmesh rg -z --zmax=N PATTERN` findet Treffer in Archiven und komprimierten
Dateien und nennt als Pfad `archiv.zip{inner.txt}`, bei zwei Ebenen
`a.zip{b.tar{g.pdf}}`. Zeilen- und Spaltennummern beziehen sich auf den
gewandelten Text, wie bei `--pre` schon dokumentiert.

Nicht im Umfang: Schreiben in Archive, Verschlüsselung knacken, `--files`
über Archive, `--filter` als freie Kommandoschnittstelle (später prüfen).

## Bauform

Eine Naht, drei Teile:

1. `internal/rgcmd/archive.go` (neu): erkennt Archiv- und Kompressionsformat,
   liefert je Datei eine Folge von `(label string, reader io.Reader, err error)`.
   Reine Standardbibliothek: `archive/zip`, `archive/tar`, `compress/gzip`,
   `compress/bzip2`. Komposition .tar.gz und .tar.bz2 durch zwei Stufen.
2. `internal/rgcmd/rgcmd.go`: in `searchPaths` im `visit`-Zweig, wo heute
   `pre.applies`/`os.Open` entscheiden, kommt davor die Fallunterscheidung
   "Archiv? dann Mitglieder einzeln suchen". `w.searcher.Search(label, reader,
   tracked)` - der Pfad ist bei gripgrep ohnehin nur ein Etikett, es braucht
   keinen Umbau des Searchers.
3. `internal/rgcmd/parse.go` + `internal/usage/usage.go`: `-z`/`--decompress`
   (bool) und `--zmax N` (Default 1, Grenzen 1..99), dazu zwei Zeilen Hilfe im
   bestehenden rg-Text. Ein unbekannter Wert endet mit Exit 2 und Klartext.

Die Verschachtelung ist ein Zähler, kein Baum: ein Mitglied, das selbst Archiv
oder komprimiert ist, wird nur geöffnet, solange `stufe < zmax`.

### Reihenfolge der Wandlungen

Archiv zuerst, dann `--pre`: ein `vertrag.pdf` in `akte.zip` geht durch
zip -> any2md -> Suche, Etikett bleibt `akte.zip{vertrag.pdf}`. Die
Entscheidung, ob `--pre` greift, fällt über den Namen DES MITGLIEDS, nicht über
den Archivnamen (`pre.applies(name)` bekommt den Mitgliedsnamen).

## Semantik, die wir übernehmen

- Default `--zmax=1`: nur unkomprimierte Dateien in Archiven. Komprimierte
  Dateien darin gelten als binär (Details und die gemessene Meldung im Punkt
  darunter).
- `--zmax=2` zusätzlich komprimierte Dateien und Archive im Archiv.
- Etikett in geschweiften Klammern, Pfad im Archiv mit `/` auch unter Windows
  (`filepath.ToSlash`, wie beim tar-Schreiber schon Regel). GEMESSEN: die Ebenen
  werden mit DOPPELPUNKT getrennt, nicht mit weiteren Klammern -
  `nested.zip{deep.zip:deep.txt}`, drei Ebenen `lvl1.zip{...}` -> 
  `{lvl2.zip:lvl3.zip:three.txt}`. Ist das Archiv selbst das Argument auf der
  Kommandozeile, steht es NICHT in den Klammern; die Klammer beginnt beim ersten
  Mitglied.
- GEMESSEN, entgegen der Manpage-Formulierung: bei `--zmax=1` wird ein
  komprimiertes Mitglied nicht still uebersprungen, sondern als binaer gesucht
  und mit `Binary file src/nested.zip{deep.zip} matches` gemeldet (auch -l und
  -c zaehlen es). Ein Treffer auf gepackten Bytes ist ein Zufallstreffer.
- GEMESSEN: Fehler im Archiv melden `ugrep: cannot decompress ...` und enden mit
  Exit 1. Wir nehmen Exit 2, weil das der Vertrag von ripgrep ist (0 Treffer,
  1 keiner, 2 Fehler); die Abweichung gehoert in die Doku, nicht in die
  Ausgabe.
- Binärbehandlung wie bei gefundenen Dateien: `BinaryQuit`, bei auf der
  Kommandozeile genannter Datei `BinaryConvert`.

## Grenzen und Sicherheit (Pflicht, nicht Zugabe)

- Mitgliedsgrenze 64 MiB, Gesamtgrenze je Archiv 512 MiB, höchstens 100 000
  Mitglieder. Überschritten: Meldung auf stderr, Datei weiter, Exit 2 erst,
  wenn sonst nichts gefunden wurde (wie ein Lesefehler heute).
- Keine symbolischen oder harten Links als Mitglied: überspringen mit Meldung.
- Verschlüsselte zip-Einträge: überspringen mit Meldung ("encrypted member"),
  nie stumm als Nichttreffer behandeln.
- Kein Pfad wird angelegt; Etiketten werden nur gedruckt. Ein Mitgliedsname mit
  `..` bleibt im Etikett sichtbar, wird aber nicht normalisiert.
- Rekursion ist durch `zmax` (max 99) begrenzt, jede Stufe zählt gegen das
  Budget der Gesamtgrenze.

## Stufen

Stufe 1 (dieses Ticket): zip/jar, tar, gz, bz2, tar.gz, tar.bz2, Etikett,
zmax, Grenzen, Zusammenspiel mit `--pre`, Tests und Messungen.

Stufe 2 (eigenes Ticket, erst nach Messung der Größe): xz und zstd über
Fremdmodule (`ulikunitz/xz`, `klauspost/compress/zstd`) - Größenbudget steht
bei Warnung 25 MB, harte Grenze 30 MB. 7z und cpio sind ohne Fremdmodul nicht
sinnvoll; cpio wäre mit `archive/tar`-artigem Eigenparser machbar, 7z nicht.

## Tests

- Go-Tests bauen ihre Fixtures selbst (`archive/zip`, `archive/tar`-Writer):
  Treffer im zip, Treffer im tar.gz, Etikettform, `-l`, `-c`, `-n`,
  `--zmax=1` ignoriert ein zip im zip, `--zmax=2` findet es,
  verschlüsseltes Mitglied wird gemeldet, kaputtes Archiv endet mit Exit 2,
  Grenzen greifen (Mitglied zu groß, zu viele Mitglieder).
- Zusammenspiel: `vertrag.pdf` in `akte.zip` mit `--pre any2md` gefunden, ohne
  `--pre` nicht - dieselbe Beweisführung wie bei acmesh-idv.
- Golden gegen das Original: ugrep 7.8.5 ist seit 23.11.2026 auf diesem Host
  installiert (`/usr/bin/ugrep`, aus dem Alpine-Repo). Der Vergleichstest
  vergleicht Etiketten und Exitcodes gegen diese Version und überspringt sich
  mit Meldung, wenn `ugrep` fehlt (andere Rechner) - nie still grün.
  Vorlage sind die gemessenen Fälle aus docs/measurements.md.
- Batterie (`scripts/test-akmesh.sh`): ein Fall mit `tar`-Fixture (akmesh hat
  tar an Bord), zip nur im Go-Test, weil wir keinen zip-Schreiber haben.

## Messungen (nach docs/measurements.md, nicht in den Chat)

- Dauer für ein Archiv mit 20 docx-Mitgliedern, mit und ohne `--pre`, drei
  Läufe.
- Größe des Binaries vor/nach (linux und .exe) gegen die 25/30-MB-Grenze.
- Gegenprobe: `-l` nennt Etiketten, `-c` zählt je Etikett, Exitcodes 0/1/2.

## Offene Entscheidungen

1. xz/zstd: Fremdmodule ja/nein - erst nach Größenmessung. ENTSCHIEDEN: nein für
   Stufe 1, eigenes Ticket nach der Größenmessung (Fremdmodul-Kosten).
2. `--files` über Archive: ugrep listet Mitglieder? Für uns offen, nicht in
   Stufe 1.
3. `--zmax` Default 1 wie ugrep oder 2, weil ein Archiv im Archiv heute der
   häufigste Fall ist? ENTSCHIEDEN: ugrep folgen, Default 1, in der Hilfe
   genannt.
4. Exitcode bei kaputtem Archiv: ugrep endet gemessen mit 1, wir wollen 2
   (rg-Vertrag). ENTSCHIEDEN und gebaut: 2, in der Hilfe genannt.
5. Ein eigener Schalter `--archive-types` analog zu `--filter`? Nicht jetzt.
6. Etikett eines auf der Kommandozeile genannten Archivs: ugrep druckt nur
   `{mitglied}`, im Walk dagegen `pfad{mitglied}`. ENTSCHIEDEN: wir drucken
   immer das volle Etikett (auch bei -l und -c), weil ein Treffer sonst nicht
   auflösbar ist. Das ist die einzige bewusste Abweichung in der Ausgabe; der
   Golden-Test vergleicht deshalb einen Walk.
7. Weiche Meldungen (Grenze, Link, verschlüsseltes Mitglied) setzen keinen
   Fehler: sie gehen auf stderr, der Exitcode bleibt 0 oder 1. Ein kaputtes
   Archiv ist dagegen ein Lesefehler und endet mit 2, auch wenn vorher Treffer
   kamen - so hält es der heutige Code für jeden Lesefehler.

## Reihenfolge für den Fall, dass gebaut wird

1. Leser mit Fixtures (zip, tar, tar.gz) + Etikett, ohne CLI-Schalter.
2. CLI: `-z`, `--zmax`, Hilfe, Parserfehler.
3. Grenzen und Meldungen.
4. Zusammenspiel `--pre`.
5. Messungen, Batterie, Doku.
