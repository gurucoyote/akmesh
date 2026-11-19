# md2pdf: goldmark-pdf als magerer Fork

Status: Empfehlung, noch nicht beschlossen. Tickets: acmesh-54s.2 (md2pdf), Messungen acmesh-54s.1.
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
+ goldmark mit GFM und HTML                13.578.364   unter der 16-MB-Grenze
+ Fall mit dem mageren Fork                19.009.660   ueber der Grenze
+ mit dem unveraenderten goldmark-pdf      23.212.156   deutlich ueber der Grenze
```

Die Warnlinie steht bei 13 MB, der harte Stopp bei 16.000.000 Bytes
(scripts/build-bb.sh:262 bis 283, Ticket acmesh-94e.15). Mit dem mageren Fork
reisst md2pdf die harte Linie also um rund 3 MB. Das ist die Groeszenfrage, die
acmesh-54s.1 entscheiden muss, nicht dieser Fork: entweder eigenes Binary, oder
die Linie mit Begruendung anheben, oder kein PDF.

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
Schriftpfaden. Syntaxfarben weglassen; sie sind der teuerste Einzelposten und fuer
Dokumente ohne Codeanteil ohne Wert.

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
