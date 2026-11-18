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

Als Bibliothek in acmesh eingebettet sind es rund 4,5 MB Zuwachs, das
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

## Korrektur 18.11.2026: jq im Binary kostet 0,8 MB, nicht 4,5

Die erste Schaetzung oben (rund 4,5 MB als Bibliothek) war falsch: sie kam
zustande, indem vom eigenstaendigen gojq-Binary (5,49 MB) eine geratene
Grundlast abgezogen wurde. Jetzt gemessen, mit einer Probe direkt im acmesh,
die gojq.Parse, Compile und Run benutzt (also Parser und Auswerter):

| Stand | linux | windows |
|---|---|---|
| acmesh ohne jq | 10.657.916 (10,16 MiB) | 10.669.568 (10,18 MiB) |
| acmesh mit jq | 11.460.732 (10,93 MiB) | 11.486.720 (10,95 MiB) |
| Zuwachs | 802.816 (784 KB) | 817.152 (798 KB) |

Nur der Parser ohne Auswerter waere 596 KB gewesen; der Unterschied von rund
200 KB ist der Auswerter, und den braucht man.

Probe im Binary ausgefuehrt: '{"a":[1,2,3]}' | acmesh.exe zzjq '.a|length' -> 3
und '.a|add' -> 6 auf Linux.

Folge fuer die Entscheidung: jq muss nicht als eigenes Binary danebenliegen.
0,8 MB sind weniger als ein Siebtel des eigenstaendigen gojq, und dafuer ist der
Befehl dann einfach da, wo alle anderen sind.

Entschieden am 18.11.2026: jq kommt ins acmesh, nicht als eigenes Binary.
Gebaut in internal/tools/jq.go, gemessen 804 KB Linux und 821 KB Windows
(siehe docs/measurements.md).
