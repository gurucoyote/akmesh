# Plan: curl und wget in acmesh

Stand 17.11.2026. Beschlossen nach der Messung in docs/curl.md: kein Go-curl
verhaelt sich wie curl, und unser wget (u-root) kennt nur -O.

## Grundsatz

Ein Motor, zwei Fronten. Beide Befehle teilen sich eine Umsetzung in
internal/tools (netengine.go), damit sie sich nicht auseinanderentwickeln.

Der Grund fuer den gemeinsamen Motor ist die Regel, die Martin formuliert hat:
ein Werkzeug, das Agenten falsch bedienen, ist keins. Wenn wget und curl
getrennte Umsetzungen haetten, wuerden sie sich in Redirects, Cookies, Timeouts
und Fehlertexten unterschiedlich verhalten, und ein Agent wuerde genau daran
scheitern.

## curl

Befehlsname: curl. Standardbibliothek (net/http, crypto/tls, net/http/cookiejar),
keine Fremdbibliothek.

Schalter, die gebaut werden:

| Schalter | Bedeutung |
|---|---|
| -X, --request METHOD | Methode setzen |
| -H, --header "K: V" | Kopfzeile, mehrfach erlaubt |
| -d, --data DATA | Body als Formular, mehrfach wird mit & verbunden |
| --data-raw DATA | Body woertlich, ohne @-Datei |
| --data-binary DATA | Body binaer, @-Datei erlaubt |
| -F, --form | multipart/form-data, @-Datei als Upload |
| -u, --user USER:PASS | Basic Auth |
| -A, --user-agent | User-Agent |
| -e, --referer | Referer |
| -I, --head | nur Kopfzeilen anfragen (HEAD) |
| -i, --include | Antwortkopfzeilen mit ausgeben |
| -s, --silent | nur Meldungen aus, Body bleibt |
| -S, --show-error | Fehlermeldungen trotz -s zeigen |
| -f, --fail | HTTP >= 400 als Fehler, kein Body |
| -L, --location | Redirects folgen |
| --max-redirs N | Obergrenze, Vorgabe 50 |
| -o, --output FILE | in Datei schreiben |
| -O, --remote-name | Dateiname aus der URL |
| -k, --insecure | TLS-Pruefung aus |
| -x, --proxy URL | Proxy |
| --max-time SEK | Gesamtzeit |
| --connect-timeout SEK | Verbindungsaufbau |
| -b, --cookie | Cookie schicken oder Cookiejar laden |
| -c, --cookie-jar | Cookiejar in Datei schreiben |
| -r, --range A-B | Bereichsanfrage |
| -T, --upload-file | Datei per PUT hochladen |
| -G, --get | Daten in die URL statt in den Body |
| -v, --verbose | Anfrage und Antwort im Klartext |
| --compressed | gzip anfordern und entpacken |

Regeln, die geprueft werden:

1. -s schaltet nur Meldungen ab, nie den Body. Das ist der Punkt, an dem
   gocurl scheitert.
2. Zusammengefasste kurze Schalter werden verstanden: -fsSL, -sL, -sS, -iL.
   Das ist die Form, die Agenten schreiben.
3. Exit-Codes wie curl: 0 Erfolg, 6 Name nicht aufloesbar, 7 Verbindung
   fehlgeschlagen, 22 HTTP-Fehler bei -f, 28 Zeitueberschreitung, 35 TLS-Fehler,
   47 zu viele Redirects.
4. Nur http und https. Kein file://, kein ftp://: das waere ein Weg an der
   Policy vorbei, gegen die acmesh gebaut wird.
5. -o schreibt binaer, ohne Umkodierung. Auf Windows keine CRLF-Umschreibung.

## wget

Entscheidung: wget bleibt, bekommt aber unsere Umsetzung. u-root wget wird aus
scripts/cmds.txt entfernt, cmd/wget kommt dazu.

Warum nicht streichen: Agenten und vorhandene Skripte rufen wget auf, wenn sie
eine Datei holen wollen. Ein fehlender Befehl erzeugt Ratlosigkeit und
Ausweichmanoever (dann eben powershell oder python), und die laufen an der
Policy vorbei. Ein wget, das die wichtigsten GNU-Schalter richtig kann, ist
billiger als ein dritter Weg.

Warum nicht u-root wget behalten: es kennt nur -O. Ein Agent, der --header
oder --post-data schreibt, bekommt einen Flag-Fehler, den er nicht einordnen
kann, und probiert herum. Genau der Fall, den Martin meint.

Schalter, abgebildet auf denselben Motor:

| wget-Schalter | Umsetzung |
|---|---|
| -O, --output-document FILE | wie curl -o; -O - schreibt nach stdout |
| -q, --quiet | keine Meldungen, Body bleibt |
| --no-verbose | wie -q, ohne Bodyverlust |
| -c, --continue | Bereichsanfrage ab Dateilaenge |
| --header "K: V" | wie curl -H |
| --post-data DATA | POST mit Body |
| --no-check-certificate | wie curl -k |
| -U, --user-agent | User-Agent |
| -T, --timeout SEK | wie curl --max-time |
| --spider | nur pruefen, HEAD, kein Body |
| -S, --server-response | Antwortkopfzeilen zeigen |

Nicht gebaut: rekursives Spiegeln, -N Zeitstempel-Abgleich, FTP.

## Pruefungen

Go-Tests gegen einen httptest-Server, nicht gegen das Netz: Redirect-Kette,
-Kopfzeilen, POST-Formular, multipart, Basic Auth, Cookies ueber zwei Anfragen,
Bereichsanfrage, -f mit 404, Zeitueberschreitung, -s mit Body, -sL.

Dazu Faelle in scripts/test-ext.sh gegen eine lokale Datei-URL entfallen, weil
file:// verboten ist; stattdessen startet der Testlauf einen kleinen Server aus
dem Go-Test und die Shell-Batterie prueft nur die Schalterparsing-Fehler und
-h.

Abnahme: curl -fsSL <url> liefert den Inhalt und Exit 0, curl -sS -f <404>
liefert Exit 22 und keine Ausgabe, wget -O - <url> schreibt nach stdout,
curl -o datei schreibt binaer unveraendert.

## Erwartete Groesse

net/http und TLS sind durch ssh und wget schon im Binary. Erwartet werden 50 bis
100 KB fuer den Motor und beide Fronten zusammen, zu pruefen nach dem Bau.
