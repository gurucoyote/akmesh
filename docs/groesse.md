# Wie wir die Groessenexplosion vermeiden

Stand 17.11.2026. Jede Zahl gemessen, Ziel windows/amd64, mit -ldflags "-s -w"
wo es dasteht.

## Die Ursache der Explosion

Nicht die Befehle. Das Geruest.

    coreutils, zwei Applets (uname, hostname)     24,2 MB
    coreutils, acht Luecken                       27,9 MB
    coreutils, alle 153                           40,9 MB
    Go-Programm ohne alles, gestrippt              1,0 MB

Zwischen 1 MB und 24 MB liegt nur gemeinsames Geruest: multicall, tool,
locale, collate, x/text, die Shell und ihr winmode. Wer das einmal linkt,
zahlt es immer.

## Die Regeln, die daraus folgen

1. Kein Fremdgeruest linken. Nicht coreutils als Ganzes, nicht yoke ohne
   Messung. Ein Paket wird erst genommen, wenn seine Groesse auf dem
   Zielsystem gemessen ist.
2. Grosse Zutat als Bibliothek, nicht als Applet-Sammlung. awk im
   coreutils-Geruest kostet 27,4 MB, goawk als Bibliothek rund 1,8 MB Anteil.
3. Strippen ist Pflicht: -ldflags "-s -w". Gemessen an einem eigenen Binary
   mit sed und awk: 4,1 MB zu 2,8 MB.
4. Eigene Befehle nur mit der Standardbibliothek. cut, nl, tac, basename,
   stat, truncate, split kosten dann praktisch nichts ueber der Grundlast.
5. Zwei kleine Binaries statt einer grossen: bb 8,9 MB plus acmesh-ext,
   statt einer 40,9-MB-Lieferung.

## Was die Zutaten einzeln kosten (gemessen)

| Zutat | Groesse |
|---|---|
| Go-Grundlast, gestrippt | 1,0 MB |
| sed (Go.Sed) als eigenes CLI | 3,0 MB (ungestrippt) |
| sed + awk (Go.Sed + goawk) als Bibliotheken | 4,1 MB, gestrippt 2,8 MB |
| diff + patch (go-difflib + go-gitdiff) | 2,0 MB gestrippt |
| bb mit 48 u-root-Befehlen samt Shell | 8,9 MB |

Hochrechnung fuer acmesh-ext mit Stufe 1: Grundlast 1,0, sed und awk rund 1,8,
diff und patch rund 1,0, eigene Befehle mit Standardbibliothek nahe null.
Also 4 bis 5 MB. Zusammen mit bb etwa 13 bis 14 MB gegen 40,9 MB.

## Befund am Rande

diff und patch wurden gegeneinander geprueft: mit einem sauberen
git-Patch wendet go-gitdiff richtig an (a b C d e). Mit einem von
go-difflib erzeugten Diff kam "conflict: src has fewer lines than required by
fragment", weil der Hunk-Kopf eine Zeile zu viel ankuendigte. go-gitdiff ist
strikt, ohne Kontext-Fuzz. Wer diff und patch als Paar ausliefert, muss die
Hunk-Koepfe genau erzeugen - also beim diff-Teil nachbessern oder gits
Diff-Format uebernehmen.
