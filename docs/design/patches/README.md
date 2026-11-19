# patches: der magere goldmark-pdf-Fork

Grundlage: `github.com/stephenafamo/goldmark-pdf v0.4.2`, Lizenz MIT.
Hier liegen nur die Aenderungen, kein fremder Quellcode - die vollstaendige
Quelle holt `go mod download` (AGENTS.md: keine Kopie fremden Projekts ins Repo).

## Was die Patches tun

`goldmark-pdf-0.4.2-no-chroma.patch` (vier Dateien: option.go, renderer_funcs.go,
style.go, writer.go):

- renderCodeBlock setzt den Codeblock als Kaesten in Monospace, ohne Lexer und
  ohne Syntaxfarben;
- `Styles.CodeBlockTheme` (chroma.Style) entfaellt, dafuer ein einfacher Stil;
- `WithCodeBlockTheme` und `ChromaToStyle` entfallen, damit auch der
  chroma-Import samt aller Lexer.

`goldmark-pdf-0.4.2-no-chroma-no-google-fonts.patch` (zusaetzlich fonts.go) baut
darauf auf:

- die Schriften kommen nicht mehr zur Laufzeit per HTTP von Google, sondern als
  TrueType-Datei von der Platte (`FontTypeCustom`, `BuiltinFontPath` als
  Variable);
- Voraussetzung: die TTF-Dateien muessen mitgeliefert werden, und ihre Lizenz
  muss geprueft werden. Unser Lizenz-Gate sieht nur Go-Module, keine
  Schriftdateien, das ist hier Handarbeit.

## Anwenden

    cd /pfad/zu/goldmark-pdf@v0.4.2   # oder einer Kopie davon, u+w nicht vergessen
    patch -p1 < goldmark-pdf-0.4.2-no-chroma.patch

Dann im eigenen Modul ersetzen:

    replace github.com/stephenafamo/goldmark-pdf => ./pdfmod

## Beleg (19.11.2026)

Der Patch wurde auf eine frische Kopie von v0.4.2 angewendet, ein Probe-Programm
mit `replace` gebaut und ein PDF gerendert:

    patch -p1 --dry-run        ok (vier Dateien)
    go build -ldflags="-s -w"  9212066 Bytes
    /tmp/patchbuild/out.pdf    27534 Bytes
    akmesh pdf2md out.pdf      "## Größentest / Umlaute: äöüß, ..." (Umlaute korrekt)

Gemessene Groeszen (leeres Programm 1.597.602 Bytes): upstream 12.218.530,
ohne chroma 9.097.378, ohne chroma und ohne Google-Schriften 8.016.034. Zahlen
und Herleitung: ../../measurements.md, Abschnitt Groeszenlinie, und
md2pdf-goldmark-fork.md.
