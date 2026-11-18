# Go nach wasm, ohne TinyGo

Antwort auf die Frage, ob es dafuer TinyGo braucht: nein. Go selbst hat zwei
wasm-Ziele, beide offiziell und ohne Zusatzwerkzeug.

    GOOS=wasip1 GOARCH=wasm   laeuft ohne Browser in einer Laufzeitumgebung
                              (wazero, wasmtime); das ist das Ziel fuer ein
                              Werkzeug wie acmesh
    GOOS=js     GOARCH=wasm   laeuft im Browser oder unter Node, braucht immer
                              die Datei wasm_exec.js aus GOROOT und einen
                              JavaScript-Host

wasip1 gibt es seit Go 1.21. Der Modulstandard WASI legt fest, was das Programm
von aussen bekommt: Kommandozeile, Umgebung, Standardeingabe und -ausgabe, und
Verzeichnisse nur, wenn die Laufzeit sie freigibt (preopen). Es gibt keine
Prozesse, keine Signale, kein cgo, keine Threads.

## Was in acmesh fehlte

Beide Ziele bauten auf Anhieb nicht, und zwar an drei Stellen, die nichts mit
wasm selbst zu tun haben, sondern mit unseren Plattformdateien:

- internal/policy hatte nur landlock_linux.go und landlock_windows.go. Fuer
  wasm fehlte Enforce. Neu: landlock_other.go mit der Tag-Bedingung
  "!linux && !windows", das wie Windows mit ErrUnsupported ablehnt.
- internal/tools/kill_linux.go, ps_linux.go und storage_linux.go tragen ihr
  Betriebssystem im Dateinamen, greifen also nur unter Linux. Fuer wasm fehlten
  die Symbole. Neu: platform_other.go mit Stubs, die "not available on this
  platform" melden statt Zahlen zu erfinden.
- internal/tools/platform_unix.go hatte "!windows" und zog damit auch wasm an,
  wo syscall.Sync fehlt. Die Bedingung ist jetzt
  "!windows && !wasip1 && !js".

Danach baut cmd/ext (unsere eigenen Befehle, Tag nonet) fuer beide Ziele.

## Groesse

    native, linux/amd64, -s -w      6.353.159 Bytes
    wasip1, -s -w                  11.455.926 Bytes
    js/wasm, -s -w                 11.481.724 Bytes

wasm ist also rund 1,8 mal so gross wie das native Binary. Der Grund ist die
Laufzeit: Go legt in wasm seinen eigenen Speicherverwalter und die
Laufzeitpruefungen mit ins Modul, und die wasip1-Schicht kommt dazu.

## Laeuft es wirklich

In wazero 1.12 gestartet, mit denselben Pruefungen wie nativ:

    echo '{"a":[1,2,3]}' | wazero run ext.wasm jq -r '.a|add'      -> 6
    wazero run ext.wasm sh -c 'echo hallo-wasm; echo $((6*7))'     -> hallo-wasm, 42
    printf 'a1\n' | wazero run ext.wasm sed 's/[0-9]/#/'           -> a#
    printf '2\n3\n' | wazero run ext.wasm awk '{s+=$1} END {print s}' -> 5

Das js/wasm-Modul laedt unter Node mit wasm_exec.js und laeuft bis zur
Kommandoverteilung. Ein Verzeichnis sieht das Programm nur, wenn die Laufzeit es
freigibt; wazero nimmt dafuer -mount.

## Startzeit, der eigentliche Preis

    native                         4,3 ms je Aufruf
    wasm in wazero, ohne Cache  5.669,0 ms
    wasm in wazero, mit Cache     286,7 ms

wazero uebersetzt das Modul beim ersten Start nach Maschinencode; ohne
-cachedir passiert das bei jedem Aufruf. Fuer ein Werkzeug, das ein Agent
hunderte Male startet, ist das der ausschlaggebende Nachteil, noch vor der
Groesse.

## Was im wasm-Binary nicht geht

- kill und killall: keine Prozesse, keine Signale, Stub meldet das.
- ps: liest /proc, das es unter WASI nicht gibt.
- free, df, du: brauchen statfs, die Zahlen bleiben leer.
- policy: wasm hat kein Landlock, Enforce gibt ErrUnsupported zurueck, das
  Binary verweigert dann wie unter Windows.
- Dateien: nur unter freigegebenen Pfaden, kein Zugriff auf das ganze
  Dateisystem wie im Landlock-Modell.

Ungetestet: der volle Multicall mit den u-root-Befehlen. Die brauchen mehr
Systemaufrufe (stat, readdir, ioctl) und ob wasip1 sie abdeckt, ist offen.

## Fazit

Go allein kann wasm, fuer acmesh auf zwei Wegen: wasip1 fuer eine
Laufzeitumgebung, js/wasm fuer den Browser. Der Preis ist Groesse (1,8-fach) und
vor allem Startzeit (287 ms gegen 4,3 ms mit warmem Cache), und die
Unix-Befehle brauchen Stubs, weil wasm keine Prozesse, keine Signale und kein
statfs kennt. Fuer einen Harness, der viele kleine Befehle ausfuehrt, ist das
kein Ersatz fuer das native Binary, sondern ein Zusatz fuer Umgebungen, in denen
nativ nicht geht.
