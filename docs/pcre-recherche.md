# PCRE in akmesh: Recherche und Messung (23.11.2026)

Frage von Martin: "lass uns das mit den pcre mal global recherchieren... ich habe
so das gefuehl sed, awk und evtl rg koennen das auch brauchen?"

Kurzantwort: Nein, sed und awk brauchen es nicht - GNU hat es dort nicht. Nur grep
und rg haben einen PCRE-Schalter, und den sollten wir nachbauen.

## Was GNU wirklich hat (gemessen im Referenz-Image)

    grep -P 'A(?=B)'      auf "AB"   -> AB, Exit 0     PCRE ist da
    sed -P 's/A/X/'       auf "AB"   -> sed: invalid option -- 'P'
    awk  '/A(?=B)/'       auf "AB"   -> keine Ausgabe   (Lookahead gibt es nicht)

GNU sed kennt nur BRE und ERE (-E), GNU awk nur ERE plus eigene Erweiterungen.
Kein pcre in beiden. Wer dort PCRE nachruestet, baut Verhalten, das auf einem
echten Linux nicht existiert - und AGENTS.md sagt, wir folgen dem Werkzeug, das
der Agent kennt, nicht dem, das wir schoener finden.

## Was ripgrep hat

ripgrep (rg) hat -P/--pcre2, aber nur, wenn es mit dem Cargo-Feature "pcre2"
gebaut wurde; ohne das Feature endet -P mit einer Fehlermeldung. Neuere Versionen
kennen zusaetzlich --engine default|pcre2|auto, wobei auto das RE2-aehnliche
Standardwerk verwendet, wenn das Muster dort kompiliert.

Unser rg (internal/rgcmd) kennt -P nicht: "rg: unrecognized flag -P".

## Kandidaten in Go (statischer Bau, CGO=0)

Gemessen, gestrippt, mit einem Programm, das nur kompiliert und einmal matcht:

    ohne Regex-Bibliothek                        1.790.136 Bytes
    + github.com/dlclark/regexp2 v1.12.0         1.929.400 Bytes  (+136 KiB)
    + github.com/go-regexp/pcre2 v0.1.0          2.003.128 Bytes  (+208 KiB)

    Lizenzen: regexp2 MIT; go-regexp/pcre2 BSD-3-Clause plus LICENSE-PCRE2.
    Beide sind mit der Lizenzpruefung des Projekts vertraeglich.

Cgo-Varianten (libpcre2 direkt) scheiden aus: CGO_ENABLED=0 ist Vorgabe, und ein
statischer Bau mit libpcre2 zieht eine C-Toolchain in jeden Build.

## Treue gegen GNU grep -P (9 Muster, gemessen)

    Muster              GNU grep -P   go-regexp/pcre2   regexp2
    A(?=B)              A             ok                ok
    (?<=C)D             D             ok                ok
    (?<n>ab)\k<n>       abab          ok                ok
    a\Kb                b             ok                FEHLT
    a++                 a             ok                FEHLT
    \p{Greek}           (leer)        ok                ok
    [[:alpha:]]+        AB            ok                ok
    \d+                 12            ok                ok
    \bword\b            word          ok                ok

regexp2 ist .NET-Engine, nicht PCRE: \K und possessive Quantoren fehlen. Genau
die zwei Dinge, die Agents in -P-Mustern gern benutzen.

## Geschwindigkeit (10 MB Text, 200.000 Treffer)

    go-regexp/pcre2, \bword\b               24 ms  (reines Go, ohne JIT)
    GNU grep -P 'word(?!\d)' im Container  337 ms (inklusive Containerstart)

Fuer die Dateigroessen, mit denen ein Agent arbeitet, ist das kein Thema.

## Empfehlung

1. github.com/go-regexp/pcre2 hinter eine kleine Huelle in internal/pcre legen
   (Compile, Match, FindAll, Timeout), BSD-3, 208 KiB, volle PCRE2-Treue.
2. grep -P darauf stuetzen; die uebrigen sieben fehlenden grep-Schalter kommen
   ohne PCRE aus (Ticket acmesh-v6r).
3. rg -P und --engine default|pcre2|auto mit derselben Huelle.
4. sed und awk bleiben, wie sie sind. Kein PCRE, weil GNU dort keines hat.

Das Risiko, das im Dokument stehen muss: go-regexp/pcre2 ist v0.1.0 und im
November 2026 erschienen. Die Bibliothek behauptet Byte-Gleichheit mit libpcre2
10.44 und testet gegen die Referenz; gemessen haben wir neun Muster. Wenn das
Projekt Zuverlaessigkeit vor Treue stellt, ist regexp2 die reifere Wahl - dann
fehlen \K und possessive Quantoren, und das muss in der Hilfe stehen.

## Zahlen fuer die Entscheidung

    Groesse:   +208 KiB von 18,1 MiB (gut ein Prozent) fuer go-regexp/pcre2
               +136 KiB fuer regexp2
    Tempo:     24 ms auf 10 MB, kein Unterschied, den ein Agent merkt
    Risiko:    v0.1.0 gegen v1.12 (regexp2), dafuer Treue gegenueber GNU
