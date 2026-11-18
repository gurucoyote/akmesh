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

Unter Go gebaut ist derselbe Einzelbefehl 8.478.113 Bytes (basename). TinyGo
spart hier also rund 1,1 MB, gemessen am selben Paketausschnitt. Die Groesse ist
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

## Fazit

TinyGo traegt den Datei-und-Text-Teil eines acmesh ohne Netz: rund 1,1 MB
kleiner pro Einzelbefehl, aber mit den Einschraenkungen der kleineren
Standardbibliothek, ohne Debugger und mit langen Uebersetzungszeiten. Fuer einen
schlanken Datei-Baustein (fs-Profil) ist es ein gangbarer Weg; fuer den vollen
Befehlssatz mit curl und wget braeuchte es einen eigenen Netzmotor.
