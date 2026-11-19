# md2pdf: goldmark-pdf als magerer Fork

Status: beschlossen von Martin am 19.11.2026 - md2pdf wird auf Basis dieses Forks gebaut. Tickets: acmesh-54s.2 (md2pdf), Messungen acmesh-54s.1. Die Aenderungen liegen als anwendbare Patches in `patches/` (siehe patches/README.md), damit sie nicht in einem Scratch-Verzeichnis verloren gehen.
Stand der Messungen: 19.11.2026.

## Anlass

Fuer md2pdf soll Markdown zu PDF werden, ohne fremdes Werkzeug, im Binary. Der
naechstliegende Renderer ist goldmark-pdf v0.4.2 (MIT). Er bringt aber zwei Dinge mit,
die zu acmesh nicht passen:

1. chroma samt allen Lexern, nur fuer Syntaxfarben im Codeblock. Kostet 3,12 MB.
2. Die Schriften kommen zur Laufzeit ueber HTTP von Google. Der Beleg steht im
   Modul github.com/go-swiss/fonts, google.go:74 bis 92: http.NewRequestWithContext
   und http.DefaultClient.Do, danach ReadAll. Im Binary liegt nur die Metadatenliste
   aller Google-Schriften (embed.FS, 2,3 MB), nicht die Schrift selbst. Ein Binary,
   das ohne Netz ein PDF setzen soll, funktioniert damit nicht.

## Was der Fork aendert

Grundlage: goldmark-pdf v0.4.2, vier Stellen geaendert, sonst unveraendert.

- renderCodeBlock: kein Lexer, kein Tokenisieren. Der Codeblock wird als Kaesten
  in Monospace gesetzt, Zeile fuer Zeile, ohne Farben.
- Styles.CodeBlockTheme (chroma.Style) entfaellt, an seine Stelle tritt ein
  einfacher Stil CodeBlock.
- WithCodeBlockTheme und ChromaToStyle entfallen, damit auch der chroma-Import.
- fonts_google.go entfaellt. AddFonts liest die TrueType-Datei von der Platte
  (FontTypeCustom), BuiltinFontPath ist eine Variable, DefaultStyles nutzt sie.

Damit enthaelt das Binary weder chroma noch die Google-Metadaten, und es braucht
kein Netz.

## Gemessene Groeszen

Alle Zahlen mit CGO_ENABLED=0, -ldflags="-s -w", Go 1.26.1, gleiche Probe. Vergleich
ist ein leeres Programm mit 1.597.602 Bytes.

```
leeres Programm                                  1.597.602
goldmark, Parser ohne Wurzelpaket (kein HTML)     2.093.218   +0,50 MB
goldmark + GFM (+ HTML-Renderer)                  2.584.738   +0,99 MB
goldmark + GFM, wirklich rendern                  2.719.906   +1,12 MB
goldmark-pdf v0.4.2 unveraendert                 12.218.530  +10,62 MB
Fork ohne chroma                                  9.097.378   +7,50 MB
Fork ohne chroma und ohne Google-Schriften        8.016.034   +6,42 MB
go-epub allein (Vergleich, md2epub)               4.141.218   +2,54 MB
```

Bezogen auf acmesh heute (12.591.228 Bytes roh, abgelesen an acmesh und an
.build/last-size-linux-full am 19.11.2026) heisst das:

```
heute                                      12.591.228
+ goldmark mit GFM und HTML                13.578.364
+ Fall mit dem mageren Fork                19.009.660   6 MB unter der Warnlinie
+ mit dem unveraenderten goldmark-pdf      23.212.156   1,8 MB unter der Warnlinie
```

Die Warnlinie steht bei 25 MB, der harte Stopp bei 30.000.000 Bytes; Martin hat
sie am 19.11.2026 fuer die Dokumentenschreiber angehoben (Kommentar in
scripts/build-bb.sh, Commit 95494fb, Ticket acmesh-54s.1). Damit ist die Groesze
kein Ausschlusskriterium mehr: beide Wege passen unter die Linie, der magere Fork
mit 19,0 MB und 6 MB Luft bis zur Warnung, das unveraenderte goldmark-pdf mit
23,2 MB und 1,8 MB Luft. Das Argument fuer den Fork ist damit nicht mehr allein
die Groesze, sondern dass er ohne Netz auskommt.

Achtung beim Lesen alter Notizen: die Messungen der anderen Sitzung zu diesem
Ticket sind in base64 notiert (Faktor 4/3), die acmesh-Groesze dort aber in rohen
Bytes. Base64 16.279.762 entspricht 12.209.821 rohen Bytes; beide Messreihen
meinen also dasselbe, nur anders gezaehlt.

## Was der Fork leistet, geprueft

Testdokument mit Ueberschriften, fett, kursiv, durchgestrichen, Inline-Code,
verschachtelter Liste, Task-Liste, Tabelle, Zitat, Codeblock und Fussnote.
Ergebnis: 53.008 Bytes, eine Seite, pdftotext gibt alles mit korrekten Umlauten
zurueck, pdffonts zeigt die eingebettete Schrift mit uni=ja (emb=yes).

Unveraendert gegenueber dem Original, weil nicht angefasst:

- Task-Listen zeigen "[x] erledigt" als Text, kein Kaestchen.
- Tabelle hat eine graue Kopfzeile, keine Rahmenlinien.
- Fussnote steht am Seitenende, ohne Trennstrich.
- Rohe HTML-Bloecke fallen weg.
- Bilder: im fpdf-Backend eingebettet, aber 1 Pixel gleich 1 Punkt, ohne Skalierung.

## Was der Fork kostet

- Keine Syntaxfarben im Codeblock. Wer sie will, holt chroma gezielt zurueck und
  zahlt die 3,12 MB.
- Die Schrift muss mitgeliefert oder im Dateisystem vorhanden sein. BuiltinFontPath
  steht auf DejaVuSans, vier Pfade sind moeglich (regular, italic, bold, boldItalic);
  der Vorgabewert nutzt fuer alle vier dieselbe Datei, also gibt es ohne weitere
  Angabe keine echte Fettschrift. Fuer md2pdf gehoeren DejaVuSans-Bold und die
  Kursivfassung dazu, sonst sehen Ueberschriften aus wie Fliesstext.
- Pflege: ein Fork ist eigener Code. Upstream ist MIT, der Copyright-Hinweis bleibt.
  Aenderungen am Original muessen von Hand nachgezogen werden, das gehoert in den
  Wartungsabschnitt des Tickets.

## Empfehlung

md2pdf auf den Fork bauen, als eigenes Paket im acmesh-Baum, mit vier getrennten
Schriftpfaden. Drei Gruende, in dieser Reihenfolge:

1. Das Original holt seine Schrift zur Laufzeit ueber HTTP. Ein Busybox-Binary, das
   ohne Netz kein PDF setzen kann, ist kein Busybox-Binary.
2. 4,2 MB weniger, und mit 19,0 MB bleibt Luft bis zur Warnlinie.
3. Syntaxfarben sind fuer unsere Dokumente der teuerste Einzelposten ohne Wert.

Wenn Farben doch gebraucht werden, ist die Reihenfolge: erst ohne Farben ausliefern,
dann chroma als eigenes Ticket nachziehen, mit der gemessenen Zuwachszahl.

## Reproduktion

Die drei Forks liegen unter /root/.hermes/cache/scratch/pdfprobe:

- forkpdf: ohne chroma (9.097.378 Bytes)
- fork2: ohne chroma, Google-Schriften nur ausgestubbt (8.016.034 Bytes, laeuft nicht,
  weil DefaultStyles Roboto verlangt)
- fork3: ohne chroma, Schriften von der Platte, DefaultStyles darauf umgestellt
  (8.016.034 Bytes, laeuft, das ist der empfohlene Stand)

Der Scratch-Ordner ist Zwischenlager, kein Projektstand. Wer md2pdf baut, kopiert
fork3 in den Baum und zieht den Copyright-Hinweis mit.

## Kopf- und Fusszeile (2026-11-19)

Vorbild ist go-pdfkit/html2pdf, das die Randkaesten der CSS Paged Media nutzt:
Titel oben, Seitenzahl unten, und die Gesamtzahl aus einem Zaehler, der erst am
Ende feststeht. Uebernommen ist die Idee, nicht der Code; html2pdf bringt eine
ganze Layout-Engine und eine JS-Laufzeit mit (34,9 MB Binary gegen 13,3 MB hier).

Was jetzt drin ist:

- Fusszeile auf jeder Seite, mittig, "Seite N von M". M kommt aus dem {nb}-Alias
  von gofpdf; der wird beim Schreiben der letzten Seite aufgeloest, deshalb steht
  im Code ein Platzhalter und nicht die Zahl.
- Kopfzeile mit dem ersten H1 des Dokuments, links, ueber einer Haarlinie. Auf
  Seite 1 bleibt sie weg, weil dort die Ueberschrift selbst steht.
- Das H1 wird zugleich zum PDF-Titel im Metadatum. Beim Lesen des Titels werden
  HTML-Entities aufgeloest, sonst stuende "&amp;" im Fenstertitel.
- Beide Zeilen nutzen Helvetica, einen der vierzehn Schriften, die jeder Leser
  hat. Sie kosten also keine Bytes im Ergebnis.

Zwei Eigenheiten des Renderers, die den Einbau bestimmt haben:

- goldmark-pdf setzt nach einem Seitenumbruch die Schreibposition nicht zurueck.
  Der Text lief deshalb oben aus der Seite heraus und die Kopfzeile lag mitten
  darin. Die Kopfzeilen-Funktion setzt am Ende SetY(42), seither beginnt der Text
  jeder Folgeseite unter der Linie.
- Mit EscapeHTML an wurde jedes & als &amp; in den Text geschrieben; mit
  EscapeHTML aus verschwinden spitze Klammern samt Inhalt. Wir fahren jetzt ohne
  Escaping, weil in einem PDF kein HTML-Kontext existiert. Was das kostet: rohe
  HTML-Tags im Markdown verschwinden, statt als Text zu erscheinen.

Gemessen: 13,35 MB Binary (vorher 13,34), Seitenzahl und Titel ueber pdftotext auf
jeder Seite geprueft, vier neue Tests in internal/pdfgen/footer_test.go.
