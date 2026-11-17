# Einschaetzung: ps, kill, killall (Stand 17.11.2026)

Gefragt war der Aufwand. Gemessen, nicht geschaetzt: drei Proben gebaut und
unter Windows laufen lassen.

## Was schon gemessen ist

1. Prozessliste unter Windows. Mit golang.org/x/sys/windows und
   CreateToolhelp32Snapshot plus Process32First/Next laeuft die Liste:
   "0 0 44 [System Process]", "4 0 347 System", "796 4 2 smss.exe". Der Aufwand
   dafuer sind etwa 25 Zeilen.
2. Beenden. OpenProcess mit PROCESS_TERMINATE plus TerminateProcess beendet
   einen selbst gestarteten Kindprozess (exit 1). An einem geschuetzten Prozess
   (PID 4) kommt "Zugriff verweigert", also der erwartete Fehlerpfad.
3. Groesse. golang.org/x/sys/windows ist im Binary schon drin (ssh und andere
   u-root-Befehle nutzen es). Eine Probe, die die Prozess-APIs benutzt, macht
   acmesh.exe um 1.536 Bytes groesser: 10.546.688 auf 10.548.224.
4. Ein Fund am Rand: die erste Probe mit eigenen syscall-LazyDLL-Strukturen
   wurde von Windows beim Start abgelehnt ("Invalid argument", auch ueber
   cmd.exe: "Das angegebene Programm kann nicht ausgefuehrt werden"), die
   x/sys-Variante lief sofort. Ursache unklar, Verdacht (Martin): ein
   Virenscanner, der ein Prozess-Werkzeug beim Start blockt. Lehre: x/sys
   benutzen, nicht eigene LazyDLL-Gebilde, und beim Ausliefern pruefen, ob der
   Scanner mitspielt.

## Umfang der Arbeit

ps, Windows:
- Kern: PID, PPID, Threads, Exe-Name (fertig gedacht, 25 Zeilen).
- Zusatz, je Stueck klein: voller Pfad ueber QueryFullProcessImageName, CPU-Zeit
  ueber GetProcessTimes, Speicher ueber psapi!GetProcessMemoryInfo per LazyDLL,
  Benutzer ueber OpenProcessToken und LookupAccountSid.
- Teuer und riskant: die Kommandozeile. Die steht nicht in der Toolhelp-Struktur,
  sie liegt im PEB des Zielprozesses. Das heisst NtQueryInformationProcess,
  Fremdspeicher lesen, 32-Bit-Prozesse auf 64-Bit-Windows gesondert behandeln.
  Aufwand 150 bis 250 Zeilen mit echtem Absturzrisiko. Vorschlag: erst ohne
  Kommandozeile ausliefern.

kill, Windows: 30 bis 40 Zeilen, Fehlerbilder klar (Zugriff verweigert, PID gibt
es nicht).

killall: gibt es in u-root gar nicht, auch nicht unter Linux. Also beide Seiten:
- Windows: Prozessliste nach Namen filtern, alle Treffer beenden, eigenen
  Prozess auslassen.
- Linux: /proc durchgehen, comm lesen.
Namensvergleich ohne .exe, Gross- und Kleinschreibung egal, -i zum Nachfragen,
-q still. 60 bis 80 Zeilen.

ps und kill unter Linux: bleiben bei u-root, die laufen schon. Unsere Umsetzung
ist nur fuer Windows, per Build-Tag getrennt.

## Risiken

- PID-Wiederverwendung zwischen Auflisten und Beenden. Bei killall ueber den
  Namen filtern, nicht ueber eine PID-Liste.
- Rechte: halb Windows laesst sich nicht beenden. Das muss eine Meldung und
  Exit 1 werden, kein Absturz.
- Prozesse ohne Namen, System-Prozesse mit PID 0 und 4, geschuetzte Prozesse:
  in der Ausgabe kennzeichnen statt auszublenden.
- Windows-Defender und Firmenrechner koennen Prozesswerkzeuge blocken. Das
  betrifft auch den Agenten, nicht nur uns.

## Einschaetzung

ps ohne Kommandozeile, kill und killall zusammen: rund 400 Zeilen, zwei
Plattformpfade, keine neuen Abhaengigkeiten, etwa 2 KB im Binary. Das ist ein
Zug von der Groesse, die file und iconv zusammen waren, mit etwas mehr Risiko
wegen der Rechte und der zwei Plattformen.

Die Kommandozeile in ps ist ein eigener Zug und der einzige wirklich teure Teil.

## Nachtrag: was u-root unter Linux schon hat (geprueft 17.11.2026)

u-root hat ps (cmds/core/ps/ps_linux.go, 532 Zeilen) und kill (897 Zeilen
ueber mehrere Dateien). Beide sind im Linux-acmesh und laufen.

Gemessen am gebauten Binary:

| Aufruf | Ergebnis |
|---|---|
| ps | Spalten PID TTY TIME CMD, laeuft |
| ps -e | alle Prozesse, laeuft |
| ps aux | laeuft, zeigt PGRP, SID, STAT und COMMAND |
| ps -x | laeuft, lange Kommandozeile (z.B. "plan9 --control-socket 6 ...") |
| ps -ef | FEHLER: "flag provided but not defined: -f" |
| ps -bsd | FEHLER: "flag provided but not defined: -b" |
| kill -9 PID | laeuft (Signal vor der PID) |
| killall | gibt es nicht |

Damit ist der Linux-Teil halb fertig: ps und kill sind da, aber nicht in der
Form, die Agenten tippen. "ps -ef" ist der haeufigste Aufruf ueberhaupt und
scheitert; PPID und UID gibt es gar nicht, Feldauswahl wie "ps -eo pid,ppid,cmd"
auch nicht. u-root ps kann dafuer die lange Kommandozeile unter Linux, die wir
unter Windows bewusst weglassen.

Entscheidung fuer die Umsetzung: eine ps-Umsetzung fuer beide Plattformen, mit
einem kleinen Feldsatz (pid, ppid, user, stat, time, etime, cmd, comm) und einem
Parser fuer die Aufrufe, die wirklich getippt werden: -e/-A, -f, -ef, aux, -eo
FELDER, -p PID. Dann ist die Ausgabe auf Windows und Linux gleich, und der
Unterschied zwischen den Plattformen beschraenkt sich auf das, was fehlt
(Kommandozeile unter Windows).

Mehrarbeit gegenueber dem reinen Windows-ps: rund 150 Zeilen fuer /proc und den
Feldparser. Dafuer faellt die Falle weg, dass derselbe Befehl auf zwei Zielen
verschieden aussieht.
