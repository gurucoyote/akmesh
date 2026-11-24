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
