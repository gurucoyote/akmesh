# Einschaetzung: iconv und jq

Stand 17.11.2026. Gebaut und gemessen, Ziel windows/amd64, gestrippt.

## iconv

Drei Bauformen, alle gebaut:

| Variante | Groesse | Zuwachs gegen Grundlast (1,0 MB) |
|---|---|---|
| nur Grundlast | 1,0 MB | - |
| eigene Tabellen (UTF-8, Latin-1, Latin-9, CP1252, UTF-16 LE/BE) | rund 1,05 MB | etwa 30 KB |
| x/text mit charmap und utf16 | 1,76 MB | 0,75 MB |
| x/text mit htmlindex, also alle Zeichensaetze | 2,46 MB | 1,4 MB |

Was die Stufen koennen:
- Eigene Tabellen: die Faelle, die ein Agent wirklich trifft - Windows-Dateien in
  CP1252, Latin-1 aus alten Quellen, UTF-16 mit BOM aus PowerShell. Fehler bei
  nicht abbildbaren Zeichen, Umlaute kommen richtig heraus (getestet:
  ISO-8859-1 nach UTF-8 ergibt "Gruesse" mit Umlauten).
- charmap: zusaetzlich ISO-8859-2 bis -16, KOI8, Windows-1250 bis -1258. Die
  Tabellen sind der Grossteil der 0,75 MB.
- htmlindex: zusaetzlich Shift_JIS, EUC-JP, GBK, GB18030, Big5, EUC-KR.

Was keine Variante kann: //TRANSLIT, //IGNORE als Schalter, und keine
Zeichensatz-Erkennung. Wer eine Datei ohne Angabe der Quelle hat, braucht eine
Erkennung (chardet-aehnlich), das ist ein eigenes Werkzeug.

Empfehlung: eigene Tabellen. 30 KB gegen 1,4 MB, und die drei Faelle, die
zählen, sind abgedeckt. Wenn wir spaeter japanische oder chinesische Daten
sehen, ist der Wechsel auf x/text ein Import und ein Umbau von 20 Zeilen.

## jq

Gemessen: github.com/itchyny/gojq, das uebliche Go-jq. Eigenes Binary:
5.760.512 Bytes, also 5,49 MB gestrippt. Funktioniert ('.a|length' -> 3,
'.b.c' -> "x").

Als Bibliothek in acmesh-ext eingebettet sind es rund 4,5 MB Zuwachs, das
Binary ginge von 3,5 auf etwa 8 MB.

Selbst schreiben ist keine Option: jq ist eine Sprache mit Lexer, Parser,
Compiler, Pfadausdruecken, Reduktionen und eigener Fehlersemantik. Das ist
Wochenarbeit und wird schlechter als gojq.

Die Alternative, die zur Groessenregel passt: jq nicht in ext, sondern als
eigenes Binary nur dort, wo es gebraucht wird. Wer JSON verarbeitet, nimmt es
dazu; wer nur Text schneidet, zahlt die 4,5 MB nicht.

Was gojq nicht ist: 100 Prozent jq. Die Unterschiede stehen im README des
Projekts (Sortierreihenfolge, Zahlenformate, einige Fehlerfaelle). Fuer
Agentenarbeit reicht es, fuer jemanden, der ein jq-Skript portiert, kann es
ueberraschen.
