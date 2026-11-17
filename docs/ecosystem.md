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
