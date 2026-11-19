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
5. Ein Binary statt einer 40,9-MB-Lieferung: akmesh traegt die u-root-Befehle
   und unsere eigenen Befehle zusammen, statt zwei getrennte Binaries.

## Was die Zutaten einzeln kosten (gemessen)

| Zutat | Groesse |
|---|---|
| Go-Grundlast, gestrippt | 1,0 MB |
| sed (Go.Sed) als eigenes CLI | 3,0 MB (ungestrippt) |
| sed + awk (Go.Sed + goawk) als Bibliotheken | 4,1 MB, gestrippt 2,8 MB |
| diff + patch (go-difflib + go-gitdiff) | 2,0 MB gestrippt |
| akmesh mit 48 u-root-Befehlen samt Shell | 8,9 MB |

Hochrechnung fuer unsere eigenen Befehle mit Stufe 1: Grundlast 1,0, sed und
awk rund 1,8, diff und patch rund 1,0, eigene Befehle mit Standardbibliothek
nahe null. Also 4 bis 5 MB als eigenes Binary; im gemeinsamen Binary faellt die
Go-Grundlast nur einmal an.

## Befund am Rande

diff und patch wurden gegeneinander geprueft: mit einem sauberen
git-Patch wendet go-gitdiff richtig an (a b C d e). Mit einem von
go-difflib erzeugten Diff kam "conflict: src has fewer lines than required by
fragment", weil der Hunk-Kopf eine Zeile zu viel ankuendigte. go-gitdiff ist
strikt, ohne Kontext-Fuzz. Wer diff und patch als Paar ausliefert, muss die
Hunk-Koepfe genau erzeugen - also beim diff-Teil nachbessern oder gits
Diff-Format uebernehmen.

## Nachtrag: warum es zuerst zwei Binaries waren - und warum das nicht bleibt

Der Grund war eine Werkzeuggrenze, kein Entwurf. makebb nimmt nur Pakete aus dem
Hauptmodul:

    makebb ... /tmp/acspike/cmd/whoami2
    -> directory ... outside main module or its selected dependencies

Mit einem go.work, das beide Module auffuehrt, faellt die Grenze:

    go 1.27.0

    use (
        /root/akmesh/.build/uroot
        /root/akmesh
    )

Damit gebaut, ein Binary mit allen 48 u-root-Befehlen plus unseren Befehlen:

    9.961.472 Bytes = 9,5 MiB

Gegen zwei Binaries: 8,9 MiB plus 3,5 MiB = 12,4 MiB. Das eine
Binary spart also rund 3 MB, weil die Go-Laufzeit nur einmal drin ist.

Geprueft am gebauten Binary (18.11.2026): `akmesh ls /root`, `akmesh basename
/a/b/c.txt`, `akmesh awk ...`, `akmesh sed ...` und
`akmesh gosh -c 'echo shell-im-selben-binary'` laufen alle.

Die Benennung ist erledigt: jeder eigene Befehl hat ein eigenes kleines
main-Paket, das die gemeinsame Umsetzung aufruft, also heisst er `akmesh sed`
und nicht mehr `ext sed`. Das eigene ext-Binary ist entfernt.
