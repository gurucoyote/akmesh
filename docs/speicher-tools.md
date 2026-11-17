# Einschaetzung: free, df, du (Stand 18.11.2026)

Gefragt war, was leichter ist: die Netzwerkbefehle (curl, wget) oder die
Speicherbefehle (free, df, du).

## Gemessen

Eine Probe mit drei Windows-Aufrufen, gestrippt 1.595.904 Bytes (also die
Grundlast eines Go-Programms, kein Zuwachs):

- GlobalMemoryStatusEx: "RAM total 65033 MB, frei 30684 MB, Last 52%"
- GetLogicalDrives plus GetDiskFreeSpaceEx je Laufwerk:
  "C: total 1906737 MB, frei 774844 MB", D:, W:, X:, Z: ebenfalls,
  E: meldet sauber "Das Gerät ist nicht bereit" statt abzustuerzen.

Mehr braucht free und df unter Windows nicht. Unter Linux liest free
/proc/meminfo und df die Eintraege aus /proc/self/mountinfo plus statfs.

du ist ein Verzeichnisdurchlauf mit Summierung, auf beiden Zielen derselbe
Code; die einzige Feinheit ist, ob die belegten Bloecke oder die Dateigroesse
gezaehlt wird.

## Vergleich

Speicherbefehle:
- free: rund 40 Zeilen Windows, 25 Zeilen Linux.
- df: rund 120 Zeilen, Linux ueber mountinfo und statfs, Windows ueber die
  Laufwerksliste.
- du: rund 120 Zeilen Durchlauf, Hardlinks und symbolische Verweise beachten.
- Summe etwa 300 Zeilen, keine neue Abhaengigkeit, kein neues Risiko. Die
  Windows-Seite ist oben schon belegt.
- Nebeneffekt: unser df ersetzt das kaputte u-root df unter WSL.

Netzwerkbefehle:
- curl mit 29 Schaltern: 700 bis 900 Zeilen. Kein Plattformrisiko, aber viel
  Verhalten, das stimmen muss: Redirects, Cookies, multipart, Range,
  Fehlercodes, -s ohne Bodyverlust.
- wget-Front auf demselben Motor: rund 120 Zeilen.
- Summe etwa 900 bis 1000 Zeilen, dafuer gut testbar gegen httptest.

## Antwort

Die Speicherbefehle sind deutlich leichter: etwa ein Drittel des Codes, zwei
Windows-Aufrufe statt eines Protokolls, und ein Teil davon (df unter Linux)
ersetzt etwas, das heute kaputt ist. Die Netzwerkbefehle sind der groessere
Zug, weil curl sich in vielen kleinen Verhaltensweisen richtig verhalten muss.

Reihenfolge, die sich daraus ergibt: free, df, du zuerst (schneller Nutzen),
dann curl mit wget.
