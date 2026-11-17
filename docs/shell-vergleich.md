# Shell-Kern: mvdan/sh gegen qiangli/sh

Stand 17.11.2026. Beide Kandidaten sind derselbe Code: qiangli/sh ist eine Gabel
von mvdan.cc/sh/v3 und behaelt den Modulpfad `mvdan.cc/sh/v3`. Damit ist die
Wahl keine Richtungsentscheidung, sondern eine Zeile `replace` im go.mod.

## Kompatibilitaet, gemessen

34 eigene Skripte (Wortsplitting, Globs, Parameter, Arrays, Arithmetik,
C-For, case, Heredoc, Here-String, Prozess-Substitution, Pipes mit pipefail,
set -e, Funktionen, printf, read-Schleife, IFS, trap, Subshell-cd, indirekte
Expansion, ANSI-Quoting, Brace-Expansion, readonly, until, Exit 127, case
mit Muster, source, mapfile, getopts, select mit nullglob). Verglichen wurde
stdout plus stderr gegen bash 5.x, ohne die EXIT-Zeilen unseres Laeufers.

    python3 /root/.hermes/cache/scratch/shellsrc/conform/cmp34.py

- mvdan/sh 3.11.0 (unser heutiges bb): 30 von 34.
- mvdan/sh 3.14.1 upstream: 30 von 34.
- qiangli/sh (Klon HEAD a4141771): 31 von 34.

Die drei gemeinsamen Abweichungen sind keine Verhaltensunterschiede:
- 17_set_e: unser Laeufer druckt "FEHLER: exit status 1" dazu, bash nicht.
- 27_readonly_unset: bash stellt "datei: line 1:" voran, die Interpreter nicht.
- 29_exit127: Wortlaut der Meldung ("command not found" gegen
  "executable file not found in $PATH"), der Exit-Code 127 stimmt.

Der eine echte Unterschied: printf mit Praezision (%.2f, %.3s) und %q. 3.11 und
3.14.1 weichen hier ab, die Gabel trifft bash.

Nicht nachgemessen: die Behauptung der bashy-README, "86/86 on GNU Bash's own
test suite" (B).

## Code und Policy, gemessen

Beide halten dieselben fuenf Haken, an denen eine Dateiregel haengen kann:

    OpenHandler, StatHandler, ReadDirHandler2, ExecHandler, CallHandler

- `ExecHandlers` als Middleware-Schicht gibt es auch upstream, nicht erst in der
  Gabel (interp/api.go:533 in 3.14.1). Wer mehrere Regeln schichten will, kann
  das mit beiden.
- Keine der beiden bringt eine Sandbox mit. Suche nach landlock, sandbox,
  DenyPath in der Gabel: nur Treffer in Doku und Tests. Die Durchsetzung bleibt
  unsere Arbeit, in beiden Faellen gleich.
- Die Gabel aendert sehr viel: 3272 Dateien, +447.243 Zeilen gegen v3.14.1 -
  Arithmetik, Brace-Expansion, bashpp, typedjson, dazu der Ordner `winmode`.
- winmode ist der Punkt, der fuer uns zaehlt: POSIX-Rechtebits in der
  Windows-DACL, nach dem Cygwin-Schema. Verdrahtet im Interpreter
  (interp/handler.go:1121 fuer chmod, interp/os_windows.go:314 beim Lesen des
  Modus). Heisst: unter Windows werden `chmod`, `[ -w datei ]` und `[ -x datei ]`
  erst dadurch sinnvoll. Upstream hat das nicht, dort gibt es unter Windows
  keine Rechtebits.
- Preis der Gabel: unser Testlaeufer ist 25,2 MB statt 5,1 MB (mehr
  Abhaengigkeiten: x/mod, typedjson, bashpp).

## Was folgt

1. Kompatibilitaet: die Gabel ist einen echten Fall besser (printf) und im
   Wortlaut bash-naeher. Kein Grund, sie zu meiden.
2. Policy-Bau: identisch. Beide geben uns die Haken, keiner gibt uns die
   Durchsetzung. Unsere Huelle passt auf beide.
3. Windows: nur die Gabel macht Rechtebits und damit `-r/-w/-x` und chmod
   ueberhaupt deutbar. Fuer acmesh, das unter Windows Regeln pruefen soll, ist
   das ein gewichtiges Argument.
4. Wechselkosten: null. Modulpfad ist gleich, ein `replace` entscheidet.
   Vorschlag: mit upstream 3.14.1 anfangen und die Gabel erst dann einziehen,
   wenn Windows-Rechtebits oder bash-Treue gebraucht werden - dann eine Zeile.
