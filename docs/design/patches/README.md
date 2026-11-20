# patches: der goldmark-pdf-Fork ohne chroma und ohne Google-Schriften

Grundlage: `github.com/stephenafamo/goldmark-pdf v0.4.2`, Lizenz MIT.
Hier liegen nur die Aenderungen, kein fremder Quellcode - die vollstaendige
Quelle holt `go mod download` (AGENTS.md: keine Kopie fremden Projekts ins Repo).

## Was der Patch tut

Eine Datei: `goldmark-pdf-0.4.2-no-chroma-no-google-fonts.patch`, neun Dateien.

Ohne chroma (Syntaxfarben):

- renderCodeBlock setzt den Codeblock als Kasten in Monospace, ohne Lexer;
- `Styles.CodeBlockTheme` (chroma.Style) entfaellt, ebenso `WithCodeBlockTheme`
  und `ChromaToStyle`, damit auch der chroma-Import samt aller Lexer.

Ohne Google Fonts:

- der Import `github.com/go-swiss/fonts/google` ist weg; `getFontBytes` gibt
  einen Fehler zurueck statt eine Schriftenliste im Netz zu holen. Damit faellt
  der gesamte Metadatensatz (6,4 MB JSON) aus dem Binary: gemessen 13,51 MB
  vorher, 12,98 MB nachher;
- der Cache-Typ `fonts.Cache` und ttlcache sind ebenfalls weg: `NewFpdf` und
  die Header-/Footer-Rueckrufe nehmen keinen Cache mehr, `WithFontsCache`
  entfaellt, `cache.go` ist leer bis auf den Kommentar.

Wie die Schriften jetzt ins PDF kommen: der Aufrufer registriert sie selbst als
`pdf.Font{Type: pdf.FontTypeCustom}` und schiebt die Bytes ueber `Fpdf.AddFont`
hinein. Das macht `internal/pdfgen/fonts.go` mit dem Buch aus
`internal/webfonts` (Lora fuer Text, Go Mono fuer Code) - dieselbe Quelle, aus
der auch docx und epub ihre Schriften nehmen.

## Anwenden

Nicht von Hand: `scripts/apply-goldmark-pdf-fork.sh` legt den Patch idempotent
in den Modul-Cache, und der Build ruft das Skript vor dem Uebersetzen auf. Der
Merker `.build/goldmark-pdf-fork.applied` haelt Version und Patch-Pruefsumme.

## Fallen (19.11.2026)

- `pdf.WithConfig` ersetzt die **ganze** Konfiguration, also auch das gesetzte
  PDF. Es muss **vor** `pdf.WithPDF` stehen, sonst rendert eine
  Standardinstanz ohne unsere Schriften, und gofpdf schreibt mit leerer
  Glyphentabelle: "index out of range \[80\] with length 0" beim ersten Text.
- Jeder Stil braucht unsere Schrift, auch `Styles.CodeBlock`, nicht nur
  `CodeFont`. Ein vergessener Stil fragt die Bibliothek nach einer
  Google-Schrift, die es nicht mehr gibt.
- Kopf- und Fusszeile duerfen nicht mehr durch
  `UnicodeTranslatorFromDescriptor` laufen. Bei eingebetteten UTF-8-Schriften
  kodiert das doppelt: aus "Titel über Öl" wurde "Titel 좥r ֬".
- Der Modul-Cache ist schreibgeschuetzt; das Skript macht ihn mit `chmod -R u+w`
  beschreibbar. Nach `go clean -modcache` laeuft der Patch erneut, was der Build
  von selbst tut.
- In `go.mod` bleiben `go-swiss/fonts` und `ttlcache` als `// indirect`
  stehen, weil die go.mod des gepatchten Moduls sie noch nennt. Im Build sind
  sie nicht mehr: `go list -deps ./cmd/md2pdf` zeigt keinen Treffer.

## Stand 20.11.2026: der Patch ist selbsttragend

Der Patch ist ein vollstaendiges Abbild von goldmark-pdf v0.4.2 (unveraendert aus
dem Modul-Zip) auf den Stand, den dieses Repository braucht. Er laesst sich ohne
Vorbedingung auf ein frisches Modul anwenden:

    unzip v0.4.2.zip /tmp/gp-clean            # oder frischer Modul-Cache
    patch -p1 -d <modulverzeichnis> < patches/goldmark-pdf-0.4.2-....patch

Warum das wichtig ist: eine aeltere Fassung dieses Patches baute auf einem
frueheren no-chroma-Patch auf, der nicht mehr existierte. Der Build gelang nur,
weil der lokale Modul-Cache noch gepatcht war - ein frischer Clone haette nicht
gebaut. Erzeugt wurde der Patch mit `diff -ru` zwischen ungepacktem Modul und
gepatchtem Modul; neun Dateien, danach mit `diff -r` gegen den gepatchten Stand
als identisch geprueft.

Zweite Lehre aus demselben Lauf: `scripts/apply-goldmark-pdf-fork.sh` erkannte
einen bereits angewandten Patch ueber `patch -R --dry-run`. Unter BusyBox-patch
(der Build ruft das Skript ueber `sh`) antwortet das anders als GNU patch, und der
Build scheiterte mit "patch does not apply". Die Erkennung fragt jetzt das Modul
selbst: steht die Zeile "google fonts are not built in" in `fonts.go`, ist der
Fork in Kraft.
