# akmesh aufteilen: mehrere Binaries statt eines

Status: Analyse und Vorschlag, 24.11.2026. Kein Code; die Zahlen sind gemessen,
die Groessen nach der Aufteilung sind geschaetzt und als solche markiert.
Anlass: Martin am 24.11.2026 - der Sammel-Binary ist bei 21,5 MiB, und mit
`apk` koennen Teile nachinstalliert werden, statt immer alles mitzuschleppen.

## Was heute gemessen wurde

Alle Zahlen: linux amd64, go 1.27.4, Inlining an, gestrippt, aus der
`report:`-Zeile des Baus. Die Wegwerf-Profile (`scripts/profiles/zz-*.txt`)
wurden nach der Messung geloescht; die Messung selbst steht in
docs/measurements.md, Abschnitt "Profilgroessen fuer die Aufteilung".

    voll (heute)                                 21,5 MiB   22 511 843 Bytes
    fs  (alles ausser Netz)                      16,9 MiB   17 723 619 Bytes
    nur Text und Dateien (keep-Liste, 37 Namen)  15,0 MiB   15 728 867 Bytes
    nur Shell (sh, gosh)                         14,4 MiB   15 130 851 Bytes
    nur Netz (curl wget ssh scp netcat ping ip
              dhclient netstat httpd)            12,5 MiB   13 127 907 Bytes
    nur Dokumente (14 Namen)                     10,4 MiB   10 432 739 Bytes
    nur apk (apk, apkbuild, apkreport, apkguard)  8,8 MiB    9 208 035 Bytes
    nur Dienste (init getty login crond syslogd
                 logger logread plus Benutzer)    4,2 MiB    4 407 523 Bytes
    ein eigener Befehl (pwd)                      3,2 MiB    3 363 043 Bytes
    ein u-root-Befehl (more)                      2,3 MiB    2 437 347 Bytes

Zwei Zahlen sind der Kern:

- Ein Binary mit genau einem eigenen Befehl kostet 3,2 MiB. Das ist der Sockel:
  Laufzeit, Verteiler, `internal/policy`, `internal/usage`.
- Ein Binary mit irgendeinem Befehl aus `internal/tools` kostet 15,0 MiB, auch
  wenn nur `grep`, `sed` und `cat` drin sind. Der Unterschied sind 11,8 MiB
  fremder Code, den dieser Befehl nie braucht.

## Warum die Profile heute nicht trennen

`scripts/profiles/*.txt` waehlt aus, welche `cmd/<name>`-Pakete makebb
uebernimmt - und das funktioniert: ein Profil mit `keep apk` antwortet auf `rg`
mit "command is not present in akmesh". Was es nicht kann, ist Code innerhalb
eines Pakets abwaehlen. `internal/tools` ist ein einziges Paket mit einer
Tabelle; seine Dateien importieren gojq, mvdan/sh, goldmark, apkcmd, anymd,
gripgrep. Wer also `grep` mitnimmt, nimmt das ganze Paket mit, und damit die
Importe aller anderen Befehle. Deshalb:
`nur Text` 15,0 MiB, `nur Shell` 14,4 MiB, `nur apk` 8,8 MiB - drei fast
gleiche Groessen fuer drei ganz verschiedene Aufgaben.

Die Aufteilung scheitert heute also nicht an makebb und nicht an den Profilen,
sondern an einer Paketgrenze. Genau eine Grenze muss fallen.

## Abhaengigkeiten je Block

Gemessen mit
`GOWORK=.build/go.work go list -deps -f '{{with .Module}}{{.Path}}{{end}}' <pkg>`
(nur Fremdmodule, Stdlib ausgeblendet):

    Dokumente (any2md, *2md, md2*, html2*, epub2html, pdf2html)
        yuin/goldmark, stephenafamo/goldmark-pdf, phpdave11/gofpdf,
        go-opentype/fonts (eingebettete Schriften)
    rg
        gripgrep, go-regexp/pcre2, grafana/regexp, aho-corasick, uniseg,
        yuin/goldmark (ueber anymd fuer --pre)
    jq
        itchyny/gojq, itchyny/timefmt-go
    curl, wget
        refraction-networking/utls, x/crypto, andybalholm/brotli,
        klauspost/compress, x/term
    ssh, scp
        x/crypto
    apk (und die Shell, die Paketskripte ausfuehrt)
        mvdan.cc/sh/v3
    sh, gosh
        mvdan.cc/sh/v3, rwtodd/Go.Sed, benhoyt/goawk, bluekeyes/go-gitdiff,
        gojq, pcre2, utls (die Shell kennt diese Befehle als Builtins)
    crond
        robfig/cron/v3
    ps, kill, killall, mkdir, chmod: nur x/sys (billig)
    cat, ls, more, tar, gzip, cpio: nur u-root (billig)

Die Trennlinien sind damit nicht Geschmack, sie fallen mit den Bibliotheken
zusammen:

1. **mvdan/sh** - Shell und apk. Paketskripte laufen in unserer Shell, also
   braucht `apk` die Shell ohnehin; beide gehoeren in ein Binary.
2. **goldmark und die PDF-/EPUB-Schreiber** - allein die Dokumente, plus `rg`
   ueber `--pre any2md`.
3. **utls, x/crypto, brotli, klauspost** - alles, was Netzdaten anfasst:
   curl, wget, ssh, scp, httpd (TLS), mkcert/verify/certinfo.
4. **gojq** - nur jq, 1 MiB Klasse, kann zum Sockel.
5. **robfig/cron** - nur crond, klein, kann zum Sockel.
6. Der Rest ist Stdlib und Kosten von wenigen hundert KiB.

## Vorschlag: vier Binaries

Aufteilung nach Aufgabe, jede Grenze deckt sich mit einer Bibliotheksgrenze.

**akmesh** (Sockel, geschaetzt 5-6 MiB)
```
cat ls cp mv rm mkdir rmdir ln chmod chown stat file find grep head tail wc
cut sort uniq comm tr nl tac basename dirname split truncate seq shuf diff
patch iconv env echo pwd true false md5sum sha1sum sha256sum sha512sum shasum
base64 od hexdump strings readelf dd more tee cmp mktemp readlink realpath
tar gzip unzip cpio ps kill killall pgrep pkill df du free uname hostname
time timeout sync yes which man printenv blkid init getty login passwd adduser
addgroup deluser delgroup crond syslogd logger logread policy jq certinfo
```
Bibliotheken: nur Stdlib, x/sys, x/term, gojq, robfig/cron. Kein TLS, keine
Dokumente, keine Shell. Die Dienste sind mit ~1 MiB Aufpreis billig und
gehoeren zum "laeuft ein System"-Bild; wer sie nicht will, laesst das Binary
weg.

**akmesh-docs** (geschaetzt 10-11 MiB)
```
any2md pdf2md docx2md epub2md rtf2md html2md md2txt-l md2pdf md2docx md2epub
html2docx html2epub epub2html pdf2html rg
```
Bibliotheken: goldmark, goldmark-pdf, gofpdf, fonts, gripgrep, pcre2. `rg`
sitzt hier, weil `rg --pre any2md` den Dokumentleser braucht und weil der
goldmark-Anteil dann nur einmal im System liegt. Wer nur suchen will, bekommt
`rg` ohne `--pre` als eigenes Paket spaeter (siehe offene Fragen).

**akmesh-net** (geschaetzt 12-13 MiB)
```
curl wget ssh scp httpd netcat ping ip dhclient netstat mkcert verify
```
Bibliotheken: utls, x/crypto, brotli, klauspost/compress. Das ist das teuerste
Paket; es ist auch das, das ein Harness am seltensten braucht - und heute
bezahlt es jeder mit.

**akmesh-shell** (geschaetzt 6-7 MiB mit Sockel)
```
sh gosh apk apkbuild apkreport apkguard
```
Bibliotheken: mvdan/sh. Die Shell bringt ihre Builtins mit (Go.Sed, goawk,
go-gitdiff, gojq) - das ist der Grund, warum dieses Paket nicht kleiner wird,
aber es sind genau die Bibliotheken, die die Shell selbst benutzt.

Fuenf Binaries, jede mit eigenem Verteiler und `policy.Enter()` am Anfang; die
Policy gilt also in jedem Paket, nicht nur im Sockel. Jedes Binary bekommt
seine Befehlsnamen als Symlinks in /bin (wie scripts/make-image.sh es heute
fuer das eine Binary tut), damit ein Agent weiter `cat`, `rg` oder `sh` tippt
und nicht `akmesh-docs rg`.

Summe auf der Platte, geschaetzt: 5,5 + 10,5 + 12,5 + 6,5 = 35 MiB in vier
Binaries (der Sockel steckt in jedem mit drin). Das ist mehr als heute
(21,5 MiB) - Aufteilen kostet Platz, weil Laufzeit, Verteiler und Policy
viermal bezahlt werden. Der Gewinn liegt nicht in der Summe, sondern darin, was
ein einzelner Rechner installieren muss: heute 21,5 MiB fuer `cat` und `ls`,
nach der Aufteilung 5,5 MiB.

## Auslieferung ueber apk

`apk` ist jetzt da, also ist das Paket die Einheit, nicht das Image:

    akmesh         -> /bin/akmesh, Symlinks fuer den Sockel
    akmesh-docs    -> /bin/akmesh-docs, Symlinks any2md ... rg
    akmesh-net     -> /bin/akmesh-net, Symlinks curl ... verify
    akmesh-shell   -> /bin/akmesh-shell, Symlinks sh, apk ...

Jedes Paket deklariert `depends = akmesh` nur dann, wenn es Symbole aus dem
Sockel braucht. `akmesh-docs` braucht keine: es ist ein eigenstaendiges Binary
mit eigenem Verteiler. Also duerfen die Pakete unabhaengig sein, was die
Installation klein haelt.

Ein Punkt, der geklaert werden muss: welcher Verteiler antwortet, wenn ein
Befehl fehlt. Heute nennt `akmesh` die volle Liste. Nach der Aufteilung kennt
jedes Binary nur seine eigene. Vorschlag: der Sockel-Vertelier listet seine
eigenen Befehle und einen Hinweis "weitere Pakete: akmesh-docs, akmesh-net,
akmesh-shell"; jedes andere Binary ebenso, aber ohne die Liste der anderen
vorzutaeuschen. Ein Befehl, den es nicht gibt, wird damit nie "still
vorhanden" - das ist die Regel, die schon bei acmesh-7s9 galt.

## Reihenfolge der Arbeit

1. **Paketgrenze ziehen, ohne Binary zu aendern.** `internal/tools` in
   `internal/fs/*` (Befehle ohne Fremdbibliothek), `internal/doc/*`,
   `internal/pkg/*`, `internal/shell/*` aufteilen; die Tabelle in
   `internal/tools` bleibt als Zusammensetzung, damit akmesh heute noch alles
   kann. Pruefbar: `go build ./...`, Batterie unveraendert gruen, Groesse des
   vollen Binaries bleibt innerhalb eines Prozents (das ist die
   Neutralitaetsprobe, acmesh-*Skill "neutralitaetsprobe").
2. **Zweiter Verteiler als Machbarkeitsbeweis.** `cmd/akmesh-docs` (eigener
   main, eigene Tabelle, eigener Verteiler) bauen und messen: erwartet
   10-11 MiB statt 15,0. Erst wenn diese Zahl steht, ist die Aufteilung
   belegt und nicht nur behauptet.
3. **Restliche Verteiler** (net, shell), je mit Batterie-Lauf und
   Groessenmessung; `scripts/test-akmesh.sh` bekommt einen Modus, der eine
   Befehlsliste als Eingabe nimmt statt der vollen.
4. **apk-Pakete** fuer die vier Binaries, Symlink-Satz je Paket,
   `make-image.sh` baut das Image aus Paketen statt aus einem Binary.
5. **Profile aufraeumen:** `fs`, `ext`, `services-off` verlieren ihren Zweck,
   sobald die Pakete die Auswahl treffen; sie bleiben nur, solange das
   Einzel-Binary der Standard ist.

## Stand 25.11.2026, zweiter Schritt: Leser und Schreiber getrennt (acmesh-1zzk)

    akmesh         21.5 MiB   Sockel, voll
    akmesh-docs     5.8 MiB   Leser plus md2txt-l und rg
    akmesh-write    9.2 MiB   md2pdf, md2docx, md2epub, html2docx, html2epub

Der Verteiler ist ein Paket (`internal/multicall`), die Tabellen sind je Binary
eine (`internal/doccmd`, `internal/writecmd`). Ein Harness, das Dokumente liest,
installiert 5,8 MiB statt 21,5; wer Berichte schreibt, nimmt akmesh-write dazu.

## Stand 25.11.2026: der zweite Verteiler steht (acmesh-h3ux)

`cmd/akmesh-docs` ist gebaut, mit eigener Tabelle in `internal/doccmd`, eigenem
Verteilertext und `policy.Enter()` am Anfang. `scripts/build-bb.sh` baut es
neben dem vollen Binary fuer beide Ziele, eine Profil-Messung ueberspringt es.

    akmesh-docs      11.1 MiB (11612387 Bytes), gestrippt
    akmesh-docs.exe  11.3 MiB (11817984 Bytes)
    akmesh            21.5 MiB, unveraendert

Die Erwartung von 10-11 MiB war knapp zu niedrig, aber die Aussage stimmt: ein
Text-Binary kostet ein Drittel des vollen. Der Grund ist gemessen und nicht
geraten: die Dokument-Schreiber (md2pdf, md2docx, md2epub, html2docx,
html2epub) machen 5,5 MiB aus, die Leser dagegen fast nichts, weil sie dieselben
Bibliotheken benutzen. Wer nur liest (any2md plus rg), liegt bei 6,1 MiB.

Batterie: `scripts/test-akmesh.sh` kennt jetzt `-c/--commands LIST` und laeuft
gegen einen beliebigen Verteiler; `-g documents -c "<Liste>"` ergibt 57
Pruefungen auf beiden Zielen, 0 Fehler.

Noch offen aus dieser Stufe: die Pakete selbst (Schritt 4) und der Symlink-Satz
im Image - heute liegt akmesh-docs als eigenes Binary neben `akmesh`, das Image
wird weiter aus dem vollen Binary gebaut.

## Stand 25.11.2026, dritter Schritt: die Dienste als eigener Block (acmesh-rk8i)

`internal/toolsvc` haelt init, getty, login, crond, syslogd, logger und logread -
genau die Namen, die die Befehlsliste als "Services" druckt. Die Umsetzungen
bleiben in ihren Paketen, die Tabelle und der Einstieg sind neu. Groesse
unveraendert (A/B gemessen), aber die Grenze ist jetzt explizit, und die Gruppe
services hat 13 Batterie-Pruefungen statt keiner.

## Stand 25.11.2026, vierter Schritt: fuenf Binaries und zwei Images

    akmesh         21.5 MiB   Sockel, alles in einem
    akmesh-docs     5.8 MiB   Leser plus md2txt-l und rg
    akmesh-write    9.2 MiB   die fuenf Dokument-Schreiber
    akmesh-net     10.0 MiB   curl, wget, ssh, scp, httpd, mkcert, verify, certinfo
    akmesh-shell    8.6 MiB   sh, gosh und die vier apk-Befehle

Alle vier Teil-Binaries laufen ueber `internal/multicall` (Namensaufloesung,
Liste mit Hinweis auf die anderen Pakete, `policy.Enter`, `os.Args[0]` auf den
Befehlsnamen). Gebaut werden sie in `scripts/build-bb.sh`; ein Profil-Bau
ueberspringt sie.

Zwei Image-Varianten (Martin, 25.11.2026):

    ./scripts/make-image.sh           -> akmesh:latest        21.5 MiB Binary
    ./scripts/make-image.sh --split   -> akmesh-split:latest  57.8 MB, 5 Binaries

Das Ein-Binary-Image bleibt der Standard: in einem Container ist ein /bin aus
einer Datei einfacher. Das geteilte Image zeigt, was ein einzelner Job wirklich
kostet - /bin/any2md zeigt auf akmesh-docs mit 5.8 MiB, nicht auf 21.5.

Abhaengigkeiten im geteilten Image, gemessen und nicht vermutet: akmesh-shell
braucht den Sockel fuer die Befehle, die ein apk-Skript aufruft (mkdir im Test);
akmesh-docs, akmesh-write und akmesh-net brauchen ihn nicht.

## Offene Fragen

1. `rg` ohne `--pre` als eigenes, kleines Paket (geschaetzt 6-7 MiB)?
   Entscheidung erst nach Schritt 2 mit gemessener Zahl.
2. Bleibt `akmesh` (Sockel) der Name des Sammel-Binaries, oder heisst der
   Sockel `akmesh-base`? Ein Agent kennt heute `akmesh ls`; ein Bruch kostet
   Vertrautheit.
3. Windows: dort gibt es kein apk. Sind die Pakete unter Windows einfach vier
   Verzeichnisse, oder bleibt Windows beim Sammel-Binary?
4. Der Sockel enthaelt `login`, `getty`, `crond`. Wenn ein Image das
   Einzel-Binary installiert, ist das der PID-1-Kandidat; in einem Paket, das
   nur `cat` und `ls` will, waeren sie Ballast. Alternative: eigenes
   `akmesh-svc` (heute 4,2 MiB) - Entscheidung nach Schritt 2.

## Nicht in diesem Vorschlag

- Ein Befehl pro Binary (coreutils-Stil). Der Sockel kostet 3,2 MiB pro
  Binary; bei 100 Befehlen waere das 320 MiB. Die BusyBox-Form (mehrere Namen
  pro Binary) bleibt, nur mit mehreren Buendeln.
- Alles in ein Plugin-System mit Loader zu giessen. Go-Binaries teilen keinen
  Code zur Laufzeit; die Aufteilung in Pakete ist der billigere Weg.
- TinyGo. Bleibt eine eigene, spaetere Frage.

## Stand 25.11.2026: die Paketgrenze ist gezogen

Schritt 1 ist gebaut (acmesh-58me). `internal/tools` ist nicht mehr das eine
Paket, das alles importiert; die schweren Bloecke liegen daneben, jede mit
eigener Tabelle und eigenem `Run`:

    internal/cmdrun   Policy, Hilfe-Gatter, Programmname - der gemeinsame
                      Einstieg, damit jedes Paket dieselben drei Zeilen hat
    internal/tools    der Sockel: Datei- und Textbefehle, ps/df/du, Dienste,
                      sed/awk (Go.Sed, goawk), diff/patch (go-gitdiff), tar
    internal/toolpkg  apk, apkbuild, apkguard, apkreport
    internal/toolnet  curl, wget, ssh, scp (nur ohne nonet) und die Zertifikate
    internal/toolsh   sh, gosh
    internal/tooljq   jq
    internal/cmds     die Sammeltabelle fuer Tests und Hilfe-Audit; ein
                      cmd/<name>/main.go darf sie nie importieren, sonst zieht
                      dieses eine Binary wieder alle Bloecke herein

Der Eigene-Werkzeug-Haken fuer Paketskripte (`apkscript.OwnTool`) sitzt jetzt in
`internal/apkcmd`, weil er nur dort gebraucht wird: ein Paketskript laeuft in
der apk-Familie, nicht im Sockel. Vorher hing er an `internal/tools`; das war
auch der eine rote Batteriefall nach dem Umbau (apk-script-own-mkdir).

Gemessen mit denselben Wegwerf-Profilen wie in der Analyse (linux, gestrippt):

    nur Text (37 Namen)   vorher 15 728 867 -> 8 028 387 Bytes  (-7,5 MiB, -48 %)
    nur Shell (sh,gosh)   vorher 15 130 851 -> 4 817 123 Bytes  (-10,2 MiB, -68 %)
    nur apk (4 Namen)     vorher  9 208 035 -> 9 208 035 Bytes  (unveraendert)
    voll                  vorher 22 511 843 -> 22 507 747 Bytes (gleich)

apk bleibt gleich, und das ist richtig so: apk fuehrt Paketskripte in unserer
Shell aus, mvdan/sh bleibt also in diesem Binary - nur eben nicht mehr in allen
anderen.

Was noch im Sockel steckt und den naechsten Schnitt lohnt: `sed`/`awk` ziehen
Go.Sed und goawk herein, `diff`/`patch` go-gitdiff, `tar` den tar-Schreiber.
Das Text-Binary ist deshalb 7,7 statt der geschaetzten 5,5 MiB. Ein eigener
Block dafuer ist ein Folgeticket; die Messung oben ist der Vergleichswert.

Tests: `scripts/test-akmesh.sh` laeuft jetzt auch pro Block oder pro Befehl
(`-g files`, `-g rg`), die Namen kommen aus der Liste, die das Binary fuer einen
unbekannten Namen selbst ausgibt. Stand am 25.11.2026: documents 38, files 246,
system 148, network 21, shell 5, services 0 Pruefungen (fuer die Daemons hat die
Batterie noch keine Faelle - das ist eine Luecke, keine Auslassung).

## Stand 25.11.2026, zweiter Schnitt: sed/awk und diff/patch (acmesh-rnyu)

`sed` und `awk` liegen jetzt in `internal/toolsed`, `diff` und `patch` in
`internal/tooldiff` - beide als gewoehnliche Bloecke mit eigener Tabelle, die
Sammeltabelle `internal/cmds` nennt sie mit. Die Kompatibilitaetstests gegen GNU
bleiben in `internal/tools` und erreichen die Umsetzungen ueber Test-Shims; sie
pruefen Verhalten, nicht Verdrahtung.

Gemessen (linux, gestrippt):

    Text mit sed/awk/diff/patch  8 028 387 -> 7 860 451 Bytes
    Text ohne sie und ohne tar   6 881 507 Bytes   (neu)
    nur cat                      2 457 827 Bytes
    nur cat und tar              3 752 163 Bytes
    voll                        22 507 747 Bytes   (unveraendert)

tar ist ebenfalls ausgezogen (internal/tooltar), bringt im Text-Binary aber nur
164 KiB: allein neben cat kostet es 1,29 MiB, in einem Binary mit vielen
Dateiwerkzeugen ist das meiste davon schon bezahlt. Der kleine Wert ist das
Ergebnis, nicht die Enttaeuschung - ein Profil ohne tar wird trotzdem kleiner,
und die Grenze ist jetzt dort, wo die Bibliothek liegt.

Der Gewinn an dieser Stelle ist klein (168 KiB fuer das Text-Binary), und das
ist die wichtigere Erkenntnis: Go.Sed und goawk wiegen zusammen etwa 815 KiB
(Text ohne sie gegen Text mit ihnen). Die restlichen 3,8 MiB ueber dem Sockel
kommen nicht aus den Textwerkzeugen:

    nur grep       4 210 915 Bytes  (+1,75 MiB durch pcre2 fuer grep -P)
    nur crond      3 911 907 Bytes  (+1,45 MiB durch robfig/cron und den
                                    Dienst-Unterbau, der heute im Sockel liegt)

Damit ist die naechste Reihenfolge belegt statt geraten: erst die Dienste
(crond, syslogd, logger, logread, init) in einen eigenen Block, dann pcre2
hinter `grep -P` - oder beides in ein eigenes Binary, das nur wer braucht. Beide
sind eigene Tickets; dieser Schnitt ist mit dem oben gemessenen Stand fertig.
