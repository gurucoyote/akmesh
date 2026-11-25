# RTF: Blöcke, Tabellen, Link-Ziele, Listenebenen

Stand 25.11.2026, acmesh-crc9. Der Leser hat jetzt zwei Stufen.

## Zwei Stufen statt eines Durchlaufs

`internal/rtfmd/parse.go` liest RTF in Blöcke (`[]Block`): Absatz, Überschrift,
Listeneintrag, Tabelle, jeweils mit Läufen (`[]Run`) für Schriftauszeichnung,
Größe und Link. `internal/rtfmd/rtfmd.go` schreibt diese Blöcke als Markdown,
`Convert` ist `Parse` plus `Render`.

Der Anlass ist nicht Ordnungsliebe: suntoad braucht dieselben Blöcke als HTML,
und ein zweiter Parser dort wäre ein zweiter Satz Fehler. Die Besitzregel steht
in `/root/suntoad/docs/design/rtf2html.md`: der Parser gehört akmesh, der
HTML-Schreiber suntoad. Der Blockaufbau hier kommt aus suntoads
`internal/rtfhtml` (Commit ec96e0f, suntoad-2g7.10) und ist damit wieder in
akmesh, wo er hingehört.

## Was neu ist

- Tabellen aus `\trowd`, `\trhdr`, `\cellx`, `\cell`, `\row`. Vorher liefen die
  Zellen einer Zeile ineinander ("NameWertAlphaEins") und die ganze Tabelle kam
  als ein Absatz an. Markdown kennt keine Tabelle ohne Kopfzeile, deshalb wird
  die erste Zeile zur Kopfzeile; `\trhdr` entscheidet das nicht, es bleibt für
  den HTML-Schreiber da.
- Link-Ziele aus `\field` mit `\fldinst HYPERLINK` und `\fldrslt`. Vorher wurde
  beides übersprungen, der Linktext verschwand. Jetzt steht `[Verweis](ziel)`
  da, und weil `rg --pre` denselben Leser benutzt, ist Linktext auch wieder
  auffindbar. Felder ohne Adresse (DATE, PAGE) behalten ihren Ergebnistext.
- Listenebenen aus `\li`: 720 Twips sind eine Ebene, Markdown rückt mit zwei
  Leerzeichen je Ebene ein. Ein Eintrag ohne `\li` ist Ebene 1 - das ist die
  Annahme, die der alte Leser für `\pntext`-Aufzählungen schon hatte.
- `\fs` und Ausrichtung werden gelesen und im Block geführt, aber nicht
  geschrieben: Markdown hat dafür keine Form, die nicht lügt. Sie stehen für den
  HTML-Schreiber bereit.

## Vier Fehler in der Kopie, gefunden beim Zurückholen

Alle vier treffen suntoad genauso, weil der Code dort derselbe ist:

1. `\ucN`-Ersatzzeichen wurden nur übersprungen, wenn sie einzeln kamen; kamen
   sie als Textlauf, blieben sie stehen ("€??" statt "€"). Behoben im Parser.
2. Eine Zeile mit `\cell` am Ende bekam eine leere dritte Zelle, weil `\cell`
   die Zelle schließt und `\row` danach noch einmal schloss. Behoben: die Zelle
   ist nach `\cell` geschlossen, offen für die nächste, ein Zähler `cellOpen`
   sagt es dem Parser.
3. Eine fertige Tabelle wurde erst am Dokumentende ausgegeben, also hinter dem
   Text, der in Wirklichkeit nach ihr kam. Behoben: eine Tabelle ohne offene
   Zeile wird geschlossen, bevor der nächste Absatz entsteht.
4. Ein Absatzende innerhalb einer Zelle warf den Zelltext weg. Behoben: der Text
   gehört zur Zelle.

Punkte 2 und 3 stehen am 25.11.2026 noch in suntoads HTML-Ausgabe (gemessen mit
`st any2html` auf derselben Datei): leere dritte Spalte, Tabelle nach dem
Schlussabsatz. suntoad muss den Parser neu kopieren, statt ihn weiterzuentwickeln.

## Was gleich geblieben ist

Die Markdown-Ausgabe der bestehenden Fixtures ist zeilengleich; die Tests in
`internal/rtfmd/rtfmd_test.go` sind unverändert und grün. Das war die
Bedingung: der Umbau darf nichts verschlechtern. Die neue Ausgabe steht in
`internal/rtfmd/blocks_test.go` und in der Batterie (rtf2md-Tabelle, Link,
Listenebenen, `rg --pre` auf demselben Dokument).

## Grenzen

- Code pages: nur cp1252 wird gelesen; ein Dokument mit `\ansicpg1251` wird
  abgewiesen statt als Zeichensalat ausgegeben (seit diesem Schritt, vorher
  wurde es still falsch gelesen). `65001` (UTF-8) gilt als unterstützt.
- Bilder (`\pict`) bleiben übersprungen. Ob `any2md` Bilder schreiben soll, ist
  eine eigene Entscheidung und nicht Teil dieses Schritts.
- Tabellenzellen mit mehreren Absätzen werden zu einer Zeile zusammengezogen,
  weil Markdown in einer Zelle keinen Absatz kennt.
