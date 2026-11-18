# TinyGo an acmesh gemessen

Stand 18.11.2026. TinyGo 0.39.0, LLVM 19.1.2, mitgeliefertes Go 1.25.7, liegt in
/root/3rd/tinygo. Kein Podman, kein Alpine: TinyGo ist ein Host-Compiler wie Go
auch.

## Aufsetzen

    curl -sL -o tinygo.tar.gz \
      https://github.com/tinygo-org/tinygo/releases/download/v0.39.0/tinygo0.39.0.linux-amd64.tar.gz
    tar xzf tinygo.tar.gz -C /root/3rd
    /root/3rd/tinygo/bin/tinygo version

Das Tarball ist 153,6 MB, entpackt rund 700 MB, davon fast alles LLVM. Der
Compiler laeuft ohne weitere Einrichtung, ein Hello-World baut und gibt aus:
1.083.552 Bytes.

## Zwei Huerden

Erste Huerde, die Go-Version: acmesh steht auf `go 1.27`, TinyGo bringt 1.25.7
mit und laesst sich mit GOTOOLCHAIN nicht hochziehen. Fuer den Test wurde eine
Kopie des Pakets mit `go 1.25` und aelteren x/crypto-, x/sys- und
x/term-Versionen genommen (v0.40.0, v0.34.0, v0.33.0). Am acmesh-Modul selbst
wurde dafuer nichts geaendert.

Zweite Huerde, die Standardbibliothek: TinyGo hat kein net/http/cookiejar, und
sein net/http ist ein Ausschnitt. Beim ersten Versuch scheiterte deshalb jeder
Befehl, auch basename, weil das Paket internal/tools geschlossen uebersetzt wird
und netengine.go den Cookie-Jar importiert.

## Was baut

Mit unserem Build-Tag `nonet` (dem fs-Profil) geht der ganze Datei- und
Textteil durch:

    basename   7.381.640 Bytes
    jq         7.381.840
    sh         7.381.792
    sed        7.381.680
    awk        7.381.632
    cut        7.381.680

Der Vergleich mit Go muss gestrippt sein, weil unser Bau mit -s -w laeuft.
Gemessen am selben Befehl (basename, Tag nonet):

    TinyGo                           7.381.640 Bytes (7,04 MiB)
    Go, ungestrippt                  8.478.113 Bytes (8,09 MiB)
    Go, gestrippt (-s -w)            5.861.668 Bytes (5,59 MiB)

Also ist TinyGo hier nicht kleiner, sondern 1.519.972 Bytes groesser als der
gestrippte Go-Bau. Eine fruehere Fassung dieses Dokuments hat TinyGo mit dem
ungestrippten Go-Bau verglichen und daraus einen Vorteil von 1,1 MB gemacht; das
war ein Messfehler. Die Groesse ist
bei allen Befehlen fast gleich, weil die Registry alle Befehle in einem Binary
haelt; das ist bei Go genauso.

Alle sechs laufen: jq rechnet `.a|add` auf 6, sh gibt auf `$((2+3))` die 5 aus,
cut schneidet, sed ersetzt, awk summiert 3 und 5 zu 8, basename kuerzt den Pfad.

## Was nicht baut

Die Netzbefehle. TinyGos net/http kennt keinen Proxy:

    internal/tools/netengine.go:297: unknown field Proxy in struct literal of type http.Transport
    internal/tools/netengine.go:304: undefined: http.ProxyURL

Dazu fehlt net/http/cookiejar. Fuer den Jar gibt es inzwischen das Tag
`nocookiejar` (cookiejar_std.go / cookiejar_none.go), damit wenigstens der
Fehler eine ehrliche Meldung wird statt eines Build-Abbruchs; der Proxy-Teil
bleibt aber offen. Ein TinyGo-Build der Netzwerkzeuge braeuchte also einen
eigenen, schmaleren Netzmotor ohne Proxy und ohne Jar.

Die ssh selbst ist nicht das Problem: ein kleines Programm, das nur
golang.org/x/crypto/ssh importiert, baut unter TinyGo in 2.496.232 Bytes. Es
ist also der Http-Teil von netengine, der scheitert, nicht die
Verbindungsbibliothek.

## Tempo

Der Compiler ist deutlich langsamer als Go: basename braucht wenige Sekunden,
jq und sh je mehrere Minuten, weil LLVM optimiert. Ein vollstaendiger
Multicall-Build mit allen Befehlen ist in dieser Messung nicht fertig geworden;
fuer den Alltag heisst das: TinyGo lohnt nur fuer kleine, klar geschnittene
Baue, nicht als Ersatz fuer den normalen Bau.

## Recherche 18.11.2026: Krypto und Groesse, korrigiert

Alle Zahlen hier sind eigene Laeufe (Stufe A); die Web-Quellen stehen jeweils
daneben. TinyGo 0.39.0 und zusaetzlich 0.44.0 (die aktuelle Fassung vom
30.10.2026, LLVM 22) liegen in /root/3rd.

### Krypto baut, TLS ist Klartext

34 Standardpakete einzeln mit 0.39 gebaut, jedes nur als leerer Import. Alle
Kryptopakete bauen: aes, cipher, des, ecdh, ecdsa, ed25519, hmac, md5, rand, rsa,
sha1, sha256, sha512, tls, x509, dazu pem, hex, crc32, json, regexp, reflect,
os/exec, os/user, syscall, gzip, zlib. Nur net/http/cookiejar fehlt.

Ein Programm, das die Krypto auch benutzt, trennt die beiden Versionen deutlich.
Unter 0.39 scheitert schon das Linken, sobald crypto/rand im Spiel ist:

    /root/3rd/tinygo/src/internal/syscall/unix/getrandom.go:26:
        linker could not find symbol runtime.vgetrandom
    /usr/lib/go/src/crypto/internal/sysrand/rand.go:48:
        linker could not find symbol crypto/internal/sysrand.fatal

Unter 0.44 laeuft dasselbe Programm und liefert: sha256 32 Bytes, X25519 32
Bytes, ed25519 Signatur 64 Bytes, RSA 2048 Bit, tls.VersionTLS13 als 772.

Die Krypto selbst arbeitet also zur Laufzeit. Beleg: ein Programm mit
golang.org/x/crypto/ssh verbindet sich unter TinyGo 0.44 mit einem echten sshd
und fuehrt einen Befehl aus ("hallo-von-tinygo"), derselbe Quelltext mit Go
gebaut liefert "hallo-von-go". ssh verschluesselt mit den Kryptopaketen selbst,
ohne crypto/tls.

crypto/tls dagegen ist auf Linux ein Platzhalter, und zwar ein gefaehrlicher. Mit
einem eigenen Lauschserver auf 127.0.0.1:443 gemessen: das TinyGo-Binary schickt
"HEAD / HTTP/1.0\r\nHost: 127.0.0.1\r\n\r\n" im Klartext, das erste Byte ist
0x48 ("H") und kein TLS-Record (0x16 0x03). Gegen echte Ziele dasselbe Bild:
example.com, self-signed.badssl.com und expired.badssl.com antworten alle drei
gleich mit "HTTP/1.1 400 Bad Request", also Klartext auf einem TLS-Port; das
abgelaufene Zertifikat wird nicht beanstandet, waehrend Go es mit "certificate
has expired" ablehnt.

Auch die Schnittstelle ist kleiner: ein Programm, das conn.ConnectionState() aufruft,
baut unter TinyGo nicht ("type *net.TLSConn has no field or method
ConnectionState"), derselbe Quelltext mit Go gebaut verhandelt TLS 1.3
(Version 0x304, Cipher 0x1301, example.com liefert 200 OK).

Das ist kein Einzelfall: TinyGo-Issue #5696, angelegt am 17.09.2026, offen,
zuletzt aktualisiert am 22.10.2026, beschreibt genau das ("hosted TLS handshake
returns success without negotiating TLS"). PR #5700, der Go's crypto/tls auf
hosted Linux nehmen soll, ist nicht gemerged. In 0.39 verhaelt es sich anders,
dort gibt es gar kein Netz: "dial error: Netdev not set", weil TinyGos net-Paket
einen netdev verlangt (docs: github.com/tinygo-org/net, "The net package is
modified to use netdev").

Was man tun muesste, damit TLS traegt:

- auf PR #5700 warten, dann Go's crypto/tls statt des Stubs;
- oder einen netdev mit OS-TLS von aussen liefern (Drittanbieter wie
  shibukawa/tinygodriver), also TLS an die Systembibliothek durchreichen;
- fuer curl/wget zusaetzlich den fehlenden Proxy im http.Transport ersetzen und
  einen eigenen Cookie-Jar mitbringen.

Fuer uns heisst das: Krypto kann TinyGo, TLS nicht. Ein acmesh mit curl und wget
darf nicht auf TinyGo gebaut werden, solange #5696 offen ist, sonst gehen
Anmeldedaten im Klartext raus.

### Groesse: die frueheren Zahlen waren falsch

Der Fehler war zweimal derselbe: falsch verglichen. Richtig ist der Vergleich
mit denselben Bedingungen, gleicher Befehl (basename, Tag nonet):

    0.39 default (mit Debug)          7.381.640 Bytes (7,03 MiB)
    0.39 -no-debug                    3.021.736 (2,88 MiB)
    0.39 -no-debug -panic=trap        2.963.648 (2,82 MiB)
    0.39 + -gc=leaking                2.878.368 (2,74 MiB)
    0.39 -opt=2 (Gegenprobe)          3.509.632 (3,34 MiB)
    Go, -s -w (unser Bau)             5.861.668 (5,59 MiB)
    0.44 default                      8.155.632 (7,78 MiB)
    0.44 -no-debug                    3.323.760 (3,17 MiB)
    0.44 -no-debug -panic=trap        3.276.248 (3,12 MiB)

Damit ist die Antwort auf die Groessenfrage: TinyGo ist mit -no-debug rund 2,8 MB
kleiner als der gestrippte Go-Bau, also etwa halb so gross. Ohne -no-debug ist es
groesser, weil TinyGo Debug-Informationen mitliefert und Go sie mit -s -w
wegwirft. -size=short zeigt, wo die 7,38 MiB herkommen: code 1.535.940, data
313.748, bss 162.880, zusammen 1,85 MB Nutzinhalt; der Rest sind Tabellen fuer
Debug und Ablaufverfolgung.

Die Dokumentation nennt dieselben Regler (tinygo.org/docs/guides/optimizing-binaries):
-opt=z ist schon der Standard, -no-debug nimmt die Debug-Informationen,
-panic=trap spart die Panic-Texte, -gc=leaking den Sammler fuer kurzlebige
Programme, -scheduler=none die Goroutinen. Eigene Messung bestaetigt die
Richtung: -no-debug bringt den Grossteil, die anderen Regler je 50 bis 90 KB.

Nicht bestaetigt hat sich die Ankuendigung zu 0.44 ("smaller"): fuer unseren
Befehlssatz ist 0.44 gemessen groesser als 0.39, mit -no-debug 3,32 statt 2,88 MB.

### UPX, die dritte Groesse

    Go -s -w          5.861.668 -> 2.327.036 Bytes, Start 5,3 ms -> 80,3 ms
    TinyGo -no-debug  3.021.736 ->   904.228 Bytes, Start 2,0 ms -> 24,3 ms

UPX 5.3.0 halbiert bis drittelt, kostet aber bei jedem Aufruf das Entpacken. Bei
einem Werkzeug, das ein Agent hunderte Male startet, ist das der falsche Tausch.

### Was offen bleibt

- Ob ein vollstaendiger Multicall mit u-root unter TinyGo baut, ist ungetestet;
  gemessen wurden einzelne Befehle aus internal/tools.
- Wie brauchbar Panic-Meldungen ohne Debug-Informationen sind, ist ungemessen.
- Ob ein netdev mit OS-TLS curl unter TinyGo wirklich traegt, ist ungemessen und
  waere ein eigenes Stueck Arbeit.

## TinyGo ist am 18.11.2026 geloescht

Beide Installationen unter /root/3rd (0.39 und 0.44, zusammen 2,2 GB) sind weg,
die Caches dazu. Die Skripte hier bleiben, brauchen aber wieder ein TinyGo:

    curl -LO https://github.com/tinygo-org/tinygo/releases/download/v0.44.0/tinygo0.44.0.linux-amd64.tar.gz
    tar xzf tinygo0.44.0.linux-amd64.tar.gz
    TINYGO=$PWD/tinygo/bin/tinygo sh scripts/build-tinygo.sh linux fs

Die Zusammenfassung fuer den Alltag liegt in /root/docs/2026-11-18_tinygo.md.

## Brauchen wir TinyGo? (18.11.2026)

Kurz: nein, fuer nichts, was auf unserer Liste steht.

- Linux und Windows: Go baut es nativ, schneller, ohne Patch an Abhaengigkeiten.
  TinyGo spart im fs-Profil 35 Prozent Groesse, kostet aber fuenf von 66
  Pruefungen (os/exec ohne Dir und Dateien) und zwei Befehle (cpio, time).
- wasm: Go kann es selbst, wasip1 und js/wasm, ohne Zusatzwerkzeug.
- Mikrocontroller: TinyGo ist dort die einzige Go-Option, aber unser Befehlssatz
  baut dafuer nicht. Ein Versuch mit cmd/basename fuer esp32-coreboard-v2
  scheitert an mvdan.cc/sh: "undefined: syscall.Stat_t", "undefined: os.SameFile".
  Das liegt an unserer Architektur, jeder Befehl zieht die ganze Registry mit.

Die Untergrenze fuer ESP32 gemessen: ein Hallo-Welt-Programm mit TinyGo 0.44 ist
207.488 Bytes (203 KiB). Das ist fuer 4 MiB Flash nicht viel; gross wird es erst
durch die Pakete, die man mitnimmt.

## Fazit

Krypto-Rechenarbeit traegt TinyGo, TLS nicht: auf Linux ist crypto/tls ein
Platzhalter, der Klartext schickt und Zertifikate nicht prueft (Issue #5696,
offen). Groesse: mit -no-debug etwa halb so gross wie der gestrippte Go-Bau,
ohne -no-debug groesser. Fuer einen Datei-Baustein also brauchbar, fuer alles mit
Netz nicht, und die Groesse ist erst mit -no-debug ein Argument.
