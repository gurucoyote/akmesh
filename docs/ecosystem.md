# Ecosystem: was es an Go-Werkzeugen gibt und was davon traegt

Stand 17.11.2026. Alles hier ist gebaut und ausgefuehrt worden, nicht aus
Beschreibungen abgeschrieben; wo nur die fremde Beschreibung vorliegt, steht B.

## Der grosse Fund: bashy (qiangli)

- qiangli/sh: Fork von mvdan.cc/sh/v3 mit "in-process bash interpreter".
- qiangli/coreutils: MIT, Klon HEAD 6dd7a3e, Go 1.27. 153 Applets in einem
  Multicall-Binary, ausdruecklich Linux, macOS und Windows gleich, "pure Go,
  kein cgo, kein Shell-out".
- qiangli/yoke: der Rest (tar, jq, tree, which, git) als eigene Applets.
- qiangli/bashy: das Ganze als Shell. README behauptet "86/86 on GNU Bash's own
  test suite" (B, nicht nachgemessen).

Gemessen (A):

    GOOS=windows GOARCH=amd64 go build -o coreutils.exe ./cmd/coreutils
    -> 40,9 MB, Bauzeit 1 min 41 s

Laeuft unter Windows:
- `coreutils.exe` ohne Argument listet 153 Applets.
- `awk '{s+=$2} END {print "summe:",s}'` -> summe: 5.
- `sed 's/welt/erde/'` -> hallo erde.
- `uname -a` -> Windows_NT BSNB67 10.0.22631 Build 22631 x86_64.
- `du -s /tmp` -> 15354089 /tmp.
- `hostname` -> BSNB67.
- `kill --help` -> Signalhilfe vorhanden.

Mit Einschraenkungen unter Windows:
- `ps` -> "live process inspection is supported only on Linux". Kein Windows.
- `df` -> "free file-slot counts are unavailable for C:\".
- `sync` -> "not supported on windows", mit Hinweis auf Dateioperanden.
- `timeout 1 sleep 5` -> laeuft, scheitert nur, weil sleep nicht im PATH lag.
- `df -h C:` -> "C:: No such file or directory", der Doppelpunkt wird falsch
  zerlegt.

Vergleich mit u-root (unser heutiger Stand): 8,9 MB fuer 48 Befehle ohne awk,
sed, diff, patch; 40,9 MB mit 153 Applets inklusive awk, sed, diff, patch, ed,
m4, make, bc, split, stat, csplit, join, nl, od, paste.

## Die einzeln gefragten Werkzeuge

### sed und awk

- awk: github.com/benhoyt/goawk, POSIX-kompatibel, aktiv, nur Standardbibliothek.
  Ausserdem als Applet in bashy/coreutils enthalten (gemessen, laeuft).
- sed: github.com/rwtodd/Go.Sed (MIT, 140 Sterne) ist Go-flavoured sed - es nutzt
  Gos regexp, also `(a*)` und `$1` statt `\(a*\)` und `\1`. Fuer Skripte, die
  von aussen kommen, ist das eine Falle. bashy/coreutils hat sed mit einer
  GNU-Kompatibilitaetsschicht (BRE, \1, &) - das ist die bessere Wahl.

### vi

Kein reifer Vi in Go. Kandidaten:
- github.com/bkthomps/Ven - "vi enhanced", in Go, naeher an vim.
- github.com/fortio/gvi - selbst beschrieben als Demo, "not a fully usable
  editor".
- github.com/kisielk/vigo - frueh, seit Jahren nicht gepflegt.
- codeberg.org/JohnAnthony/vikenstein - modal als Bibliothek, eigener main noetig.
- Bushaltestelle: u-root hat nur `ed` (cmds/exp/ed) und `more`.
- bashy fuehrt `vi` ausdruecklich als "optional external provider", also nicht
  in Go.

### strace

- u-root hat cmds/core/strace (pkg/strace), baut aber NICHT fuer Windows
  ("build constraints exclude all Go files"). Das ist kein Zufall: strace ist
  ptrace, und Windows hat kein ptrace. Naechstes Aequivalent waere ETW oder
  Procmon, also ein anderes Werkzeug, keine Portierung.
- Eigenbau in Go waere ptrace ueber golang.org/x/sys/unix, also Linux-only.

### lsof

- Kein eigenstaendiges Go-lsof-CLI gefunden. Bibliotheken: DataDog lsof
  (/proc, Linux), diamondburned/lsof (Linux, FreeBSD, Solaris, Plan 9),
  keybase lsof (nur macOS-Dateideskriptoren). Alle lesen /proc oder libproc.
- Windows: offene Handles je Prozess braucht NtQuerySystemInformation oder
  Restart Manager. Das waere Eigenbau.

### ssh

- Ja, und es ist da: u-root cmds/exp/ssh baut fuer Windows (gemessen, "OK").
  Dazu cmds/core/sshd (Linux-only) und pkg/ssh9p, pkg/sshstream.
- Ausserdem als Bibliothek: golang.org/x/crypto/ssh, gliderlabs/ssh (Server).

## Was daraus folgt

1. ps unter Windows gibt es in keinem der beiden Projekte. Das bleibt unsere
   Arbeit - und ist damit auch ein Alleinstellungsmerkmal.
2. Fuer sed und awk brauchen wir keinen Eigenbau: bashy/coreutils liefert beide
   mit GNU-Naehe, gemessen lauffaehig unter Windows.
3. df, du, hostname, uname, timeout, sync bekommen wir dort ebenfalls, mit den
   oben genannten Einschraenkungen bei df und sync.
4. vi bleibt offen. Kein Kandidat ist reif; Ven ist der naechstliegende.
5. strace bleibt Linux-only, lsof waere Eigenbau.
6. Die Frage, die jetzt ansteht: zwei Quellen (u-root plus bashy) oder eine.
   bashy ist groesser, dafuer vollstaendiger und an denselben Interpreter
   gedacht (mvdan/sh, ExecHandler) - genau die Naht, die acmesh braucht.

## strace, im Lauf geprueft (17.11.2026)

Fundort: u-root, cmds/core/strace mit pkg/strace. Build-Tag
`(linux && arm64) || (linux && amd64) || (linux && riscv64)` - also nicht
Windows, und das ist keine Nachlaessigkeit: ptrace gibt es dort nicht.

    GOOS=linux GOARCH=amd64 go build ./cmds/core/strace
    -> 3.674.648 Bytes, laeuft

Ausgabe von `strace_u echo hallo`, gekuerzt:

    [pid 1653184] E arch_prctl(0x1002, 0x7778f9ae9b28)
    [pid 1653184] X arch_prctl(0x1002, 0x7778f9ae9b28) = 0x0 (103.5us)
    [pid 1653184] E mmap(0x62b5ab4ab000, 0x1000, 0x0, 0x32, ...)

Also Ein- und Austritt je Syscall mit Dauer, PID davor. Aber: Argumente
bleiben Zahlen, keine Namen, keine Dateipfade, keine Deskriptoren - das ist
kein strace-Format, sondern ein Rohdump. Optionen: nur `-o` (Ausgabedatei).

Grenzen gegen echtes strace: kein `-e`-Filter, kein `-p` an einen laufenden
Prozess, keine symbolische Ausgabe.

Andere Go-Herkunft: gvisors pkg/sentry/strace (nur innerhalb einer gvisor-
Sandbox), robertmin1/strace (Demo mit seccomp, ausdruecklich unfertig).

Unter Windows gibt es kein Gegenstueck. Naechstes Konzept ist ETW
(Event Tracing for Windows) oder Procmon, beides ein anderes Werkzeug mit
anderem Aufbau, keine Portierung.

## Taugt das fuer den Dateibedarf einer distroless-Images? (17.11.2026)

Geprueft, ob man mit u-roots Tracer ermitteln kann, welche Dateien ein Programm
braucht - der uebliche Weg fuer ein distroless-Abbild.

Ergebnis: brauchbar, und zwar besser als ueber die Textausgabe. Drei Gruende:

1. Pfade werden aufgeloest. Beispiel aus dem Lauf:

       E open(0x7ffc1614fe18 /etc/hostname, O_RDONLY|0x8000, ----------)
       X open(0x7ffc1614fe18 /etc/hostname, O_RDONLY|0x8000, ----------) = 0x3

   Die Syscall-Tabelle in pkg/strace kennt den Argumenttyp Path und liest die
   Zeichenkette aus dem Prozess (syscall_linux_amd64.go:284, openat mit Path).
2. Kinder werden verfolgt (PTRACE_O_TRACEFORK, TRACECLONE, TRACEVFORK in
   tracer.go:239). Ein Test mit einem Shell-Skript, das cat aufruft, zeigt zwei
   PIDs.
3. Die Bibliothek liefert Ereignisse als Struktur (EventCallback), nicht als
   Text. Ein Sammler kann also direkt Pfade mitnehmen, statt Zeilen zu parsen.

Messung am Beispiel /tmp/k.sh mit cat /etc/hostname: 141 Zeilen Spur, daraus
per Regex 9 eindeutige Pfade - /bin/cat, /usr/bin/cat, /usr/sbin/cat,
/usr/local/bin/cat, /sbin/cat, /tmp/k.sh, /etc/hostname, /tmp. Also genau das,
was man kopieren muesste.

Was ein solcher Sammler noch leisten muss:
- relative Pfade gegen /proc/<pid>/cwd aufloesen, sie kommen so heraus, wie das
  Programm sie schreibt;
- den ersten execve mitnehmen, der fehlt (im Code als bewusste Luecke notiert);
- Symlinks aufloesen und Ziele mitkopieren;
- /proc, /dev und Sockets ausfiltern;
- am Ende gegenpruefen: Abbild bauen und starten, nicht nur die Liste glauben.

Offen und zu pruefen: ob der Tracer in einem Container laeuft. Docker und Podman
blocken ptrace per seccomp standardmaessig; ob die eigene Kindprozess-Verfolgung
davon betroffen ist, muss gemessen werden (Ticket).

## Tracer im Container (gemessen 18.11.2026, acmesh-94e.12)

Frage war, ob u-roots Tracer in einem Container laeuft, weil Docker und Podman
ptrace per seccomp standardmaessig blocken.

Gemessen mit dem eigenen Binary als Sonde in debian:stable-slim, dreimal:

    podman run --rm -v acmesh:/acmesh:ro ... /acmesh strace /acmesh echo hallo
    podman run --rm --cap-add SYS_PTRACE ... /acmesh strace /acmesh echo hallo
    podman run --rm --security-opt seccomp=unconfined ... /acmesh strace ...

Ergebnis: alle drei Laeufe verfolgen die Syscalls des Kindes. Der Standardfall
ohne jede Zusatzoption reicht, weil ein Prozess seine eigenen Kinder verfolgen
darf; blockiert wird das Anhaengen an fremde Prozesse (PTRACE_ATTACH), und das
braucht u-root nicht.

Die Zeile "ptrace(PTRACE_SYSCALL): on pid N: no such process" am Ende eines
Laufs ist harmlos: das Kind ist schon tot, wenn der Tracer nachfasst. Sie
taucht mit und ohne SYS_PTRACE auf.

Der Sammler, der aus der Spur eine Kopierliste macht, ist eigenes Ticket
(strace-Sammler, P1).

## lsof und strace: Entscheidung (18.11.2026, acmesh-94e.8)

strace: bleibt drin, aber nur fuer Linux. u-root baut es aus cmds/core/strace,
3,67 MB gemessen, und es ist das einzige Werkzeug im Binary, das ptrace nutzt.
Eingeschraenkt: nur -o als Option, Argumente als Zahlen, kein -e und kein -p;
fuer den Sammler ist trotzdem die Bibliothek pkg/strace die Grundlage, nicht
die Textausgabe. Windows hat kein ptrace, das naechste Konzept dort waere ETW;
bis dahin fehlt strace im Windows-Binary, und das ist ehrlich so gewollt.

lsof: wird nicht gebaut. Es gibt kein eigenstaendiges Go-lsof-CLI, nur
Bibliotheken, die /proc oder libproc lesen. Ein Agent kommt an dieselbe
Information ohne neuen Befehl:

    ls -l /proc/<pid>/fd
    cat /proc/<pid>/maps

Beides laeuft mit dem, was das Binary schon kann (ls und cat). Ein eigener
lsof-Nachbau waere nur dann noetig, wenn sich zeigt, dass Agenten danach
greifen und dann auf python oder powershell ausweichen.

## vi: Ven geprueft (18.11.2026, acmesh-94e.7)

Ven (github.com/bkthomps/Ven) ist ein vi-Nachbau in Go, MIT, Autor Bailey
Thompson, Fassung 0.5.0, angelegt 2019, 13 Sterne.

Bau: beide Ziele bauen ohne Aenderung.

    GOOS=linux   go install github.com/bkthomps/Ven@latest   Ven      4566803 Bytes
    GOOS=windows go install github.com/bkthomps/Ven@latest   Ven.exe  3534848 Bytes

Lauf auf Linux in einer tmux-Sitzung: Datei oeffnen, i, Text tippen, Escape,
:wq - die Datei enthaelt den Text, der Editor endet mit Exit 0. Die
vim-Grundbewegungen sind da (h j k l, w b e, gg G, x, dd, D, / Suche, :w :wq
:q :q!).

Zwei Haken, die fuer dieses Projekt zaehlen:

1. Ven oeffnet /dev/tty selbst. Ohne Terminal endet es mit "open /dev/tty: no
   such device or address".
2. Ven ruft infocmp auf. Auf einem nackten Alpine ohne ncurses endet es mit
   "exec: infocmp: executable file not found in $PATH".

Ein Editor, der ein zweites Programm braucht, passt nicht zu einem Binary, das
genau deshalb existiert, damit die Huelle nichts weiter mitbringen muss. Dazu
kommt der Pflegezustand: eine Fassung, wenige Sterne, seit Jahren keine
Bewegung.

Entscheidung: vi kommt nicht in den Umfang. Was ein Agent zum Bearbeiten
braucht, ist da: ed (u-root), sed, awk, diff und patch. Ein vi bleibt ein
eigenes Thema, falls es je gebraucht wird; dann mit einer Fassung, die keine
externen Programme und kein /dev/tty voraussetzt.

### Nachtrag zu Ven: Lizenz und Herkunft der Abhaengigkeit (18.11.2026)

Lizenzseite, nachgesehen im Modul-Cache:

    github.com/bkthomps/Ven v0.5.0      MIT      Copyright Bailey Thompson
    github.com/gdamore/tcell v1.4.0     Apache-2.0
    github.com/mattn/go-runewidth 0.0.7 MIT      Copyright Yasuhiro Matsumoto

Alle drei sind freizuegig und vertragen sich mit dem BSD-3-Code im Rest des
Binaries. Die Lizenz ist also nicht der Grund, warum Ven draussen bleibt. Sie
waere eine Zeile mehr in NOTICE: MIT verlangt den Copyright-Hinweis, Apache-2.0
zusaetzlich LICENSE und NOTICE.

infocmp kommt nicht von Ven selbst, sondern aus tcell: tcell waehlt im
Standardfall den dynamischen Terminfo-Treiber (terms_dynamic.go,
terminfo/dynamic/dynamic.go) und ruft dafuer infocmp auf. Ein Editor, der eine
Terminfo-Datenbank oder das Programm infocmp vorfinden muss, ist in einer Huelle
ohne Userland nicht lauffaehig - und genau das ist der Zweck dieses Binaries.

Der Ausweg waere, tcell statisch zu bauen oder Ven zu gabeln. Dann pflegen wir
einen vi-Fork, dessen Upstream seit 2019 bei Fassung 0.5.0 steht, fuer einen
Nutzen, den ed und sed heute schon abdecken.

### Ven gegen vannevar (18.11.2026)

Beide sind in Go, beide auf tcell-Basis, beide koennten einen vi abgeben. Der
Unterschied liegt genau an den zwei Punkten, an denen Ven scheitert.

    Ven 0.5.0                        vannevar (/root/vannevar)
    fremd, MIT                       eigenes Projekt, keine Lizenzdatei
    4.566.803 Linux / 3.534.848 Win   14.416.706 Linux, Windows-Bau rot
    tcell v1 (Apache-2.0)            tcell v2 (Apache-2.0)
    ruft infocmp, braucht /dev/tty   eigene Terminfo, Fallback ohne /dev/tty
    vi-Nachbau                       w3m-Nachbau mit Editor-Funktionen

Ven nutzt tcell v1 mit dem dynamischen Terminfo-Treiber und loest deshalb
infocmp extern. vannevar baut den Bildschirm mit
tcell.NewTerminfoScreenFromTtyTerminfo und bringt die Terminfo selbst mit;
in run.go steht ausserdem eine Behandlung fuer fehlendes /dev/tty
(run.go:1737), also ein Rueckfall statt eines Abbruchs.

Der Windows-Bau von vannevar ist zurzeit rot:

    internal/app/vannevar/run/run.go:1722:20: undefined: syscall.Kill
    internal/app/vannevar/run/run.go:1722:39: undefined: syscall.SIGTSTP

Das ist die Ctrl-Z-Behandlung, die ein eigenes File mit Build-Tag braucht.

Fuer acmesh heisst das: vannevar ist der einzige Kandidat, der ohne fremdes
Programm laeuft, braucht fuer Windows aber erst eine Korrektur.

Korrektur zu einer frueheren Formulierung: ein vi ohne Betrachter gibt es nicht.
Wer editiert, muss den Text sehen; jedes Vollbild-vi ist auch ein Leser. Der
Unterschied bei vannevar ist nicht "Betrachter gegen Editor", sondern der
w3m-Teil: URLs holen (x/net), Verlauf, :open-Dialoge, Layout-Modus. Das ist eine
Navigationsmaschine, und die braucht ein Agent, der Dateien liest und schreibt,
nicht.

Der eigentliche Pruefstein fuer beide ist ein anderer: Ven und vannevar sind
Vollbildprogramme. Sie brauchen ein Terminal, ein tty, Tastendruecke und ein
Auge davor. Ein Agent in einer Huelle hat keins davon - er arbeitet mit sed,
awk, diff, patch und cat. Ein vi in diesem Binary waere also fuer den Menschen
am Terminal gedacht, nicht fuer den Agenten, und das ist ein anderes Ziel als
das Projektziel. Ven ist klein und baut auf beiden Zielen, taugt aber nur dort,
wo ein Userland mit ncurses steht.
