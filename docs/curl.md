# Einschaetzung: curl (Stand 17.11.2026)

Frage war, ob es im Go-Umfeld einen curl-Ersatz gibt, weil Agenten nach curl
greifen. Antwort: brauchbar nein, und unser wget ist zu duenn.

## Was es gibt, gemessen

- github.com/maniartech/gocurl: CLI mit curl-Syntax, windows/amd64 gestrippt
  7.582.368 Bytes (7,23 MB). Zieht golang.org/x/net, brotli und x/text mit.
  Es parst curl-Befehle, verhaelt sich aber nicht wie curl:
  * `-s` unterdrueckt den Body. In curl schaltet -s nur den Fortschrittsbalken
    ab, der Body kommt weiter.
  * `-sL` wird abgelehnt: "unknown flag: -sL". Zusammengefasste kurze
    Schalter, also die Form, die jeder Agent schreibt, kennt es nicht.
  Damit ist es fuer Agentenaufrufe unbrauchbar, auch wenn es huebsch ist.
- github.com/0x0ACF/gocu: "curl reimagined", bewusst nur ein Teil der
  Schalter (-d, -H, -X) plus Platzhalter, cobra als Geruest. Gleiche Grenze bei
  zusammengefassten Schaltern.
- curlie: Huelle um echtes curl, braucht also curl daneben. httpie-go (ht):
  eigene Syntax, nicht curl.
- Echte curl unter Windows: C:\Windows\System32\curl.exe, 711.736 Bytes,
  Version 8.13.0 mit Schannel und zlib. Ist auf jedem aktuellen Windows da,
  aber ausserhalb unseres kontrollierten Userlands und in C geschrieben.

## Unser eigener Stand

u-root wget kennt nur -O. Kein -H, kein -d, kein -L, kein -I. Fuer einen
Agenten, der eine API abfragt, ist das zu wenig.

## Empfehlung

Eigenes curl in internal/tools, Standardbibliothek, Befehlsname curl. Die
Zusatzkosten sind klein, weil net/http und TLS durch ssh und wget schon im
Binary stecken; es kommt im Wesentlichen nur unser Code dazu, geschaetzt 50 bis
100 KB.

Schalter, die ein Agent wirklich braucht:

  -X/--request, -H/--header, -d/--data/--data-raw/--data-binary,
  -F/--form, -u/--user, -A/--user-agent, -e/--referer,
  -I/--head, -i/--include, -s/--silent, -S/--show-error, -f/--fail,
  -L/--location, --max-redirs, -o/--output, -O/--remote-name,
  -k/--insecure, -x/--proxy, --max-time, --connect-timeout,
  -b/--cookie, -c/--cookie-jar, -r/--range, -T/--upload-file, -G/--get,
  -v/--verbose, --compressed

Regeln, die wir dabei festlegen (und pruefen) muessen:
- -s schaltet nur Meldungen ab, nie den Body.
- Zusammengefasste kurze Schalter wie -sL, -sSL, -fsSL werden verstanden.
- Exit-Codes wie curl: 6 Namensaufloesung, 7 Verbindung, 22 HTTP-Fehler mit -f,
  28 Zeitueberschreitung, 0 sonst.
- Keine Nicht-HTTP-Protokolle. Kein FTP, kein file:// - das waere ein Loch in
  der Policy, gegen die akmesh gebaut wird.

Was wir nicht bauen: -w Formatsprache, Fortschrittsbalken, HTTP/2-Priorisierung,
Brotli als eigene Abhaengigkeit.
