# Batterie-Migration: welche Prüfung wird ein Go-Test

Stand 19.11.2026, Regel aus akmesh-byw. Gilt für die Kindtickets der Migration
(find, grep, head und wc, cut und base64, tr) und für acmesh-j4f, das die Batterie
danach verkleinert.

## Die Regel

Eine Batterie-Prüfung wird ein Go-Test, wenn sich ihre Zusage aus Argumenten, stdin
und stdout entscheiden lässt, ohne den Betriebssystemzustand: also Ausgabe, Exit-Code
und Meldung für eine gegebene Eingabe. Sobald sie eine echte Datei mit Rechten, einen
zweiten Prozess, ein anderes Binary, das Netz, das andere Ziel oder Kernelzustand
braucht, bleibt sie in der Batterie. Hat sie beide Anteile, gilt "beides": der reine
Teil wird Go-Test, der Rest bleibt, und die Batterie prüft dann die Grenze statt des
Verhaltens.

Zwei Folgerungen, die beim Zählen zählen:

- Ein Go-Test ersetzt die Prüfung nur, wenn er dieselbe Zusage deckt. Ein Test, der
  eine Ausgabe prüft, aber den Exit-Code nicht, ersetzt eine Prüfung, die beides
  prüft, nicht - dann wird die Prüfung geteilt, nicht gestrichen.
- Plattform-Verhalten (Windows gegen Linux, CRLF, Rechte) kann ein Go-Test nur mit
  Build-Tags abdecken; solche Prüfungen bleiben in der Batterie, weil sie den
  gebauten Befehl auf dem echten Ziel meinen.

## Zahlen vorher

Statisch im Skript stehen 222 Prüfungen (171 `check`, 51 `checkcode`), dazu eine
Schleife mit zwei Vorlagenzeilen, die zur Laufzeit 22 Prüfungen für zehn Befehle
erzeugt (--help, 20 Prüfungen und zwei für echo). Die Batterie meldet deshalb

    244 Prüfungen auf Linux, 222 auf Windows

Die Differenz sind die Blöcke, die nur auf einem Ziel laufen.

Einordnung der 220 namentlich geprüften Prüfungen:

    Go-Test jetzt            24 Prüfungen in  7 Befehlen
    Go-Test später möglich   87 Prüfungen in 26 Befehlen
    beides                   25 Prüfungen in  7 Befehlen
    Batterie                 84 Prüfungen in 22 Befehlen

"Jetzt" heißt: eigenes Kindticket. "Später" heißt: der Weg ist frei, es gibt nur noch
kein Ticket, und das Verhalten ist heute durch die Batterie gedeckt. Die zwei
Vorlagenzeilen der Hilfe-Schleife sind "beides": der Go-Test
`TestEveryCommandWithTextAnswersHelp` deckt alle Befehle mit Text ab, die Batterie
behält die Stichprobe von zehn Befehlen, weil sie das gebaute Binary auf dem Ziel
prüft.

## Einordnung je Befehl

Die Einordnung gilt für jede in der letzten Spalte genannte Prüfung. Gezählt ist nach
dem Präfix des Prüfungsnamens, nicht nach dem Befehl in der Zeile: eine Prüfung wie
`cp-archive-nested` gehört zu cp, auch wenn sie ihr Ergebnis mit cat liest.

| Befehl | Prüfungen | Einordnung | Prüfungsnamen |
|---|---|---|---|
| base64 | 2 | Go (jetzt) | base64-wrap0, base64-default-wrap |
| cut | 2 | Go (jetzt) | cut-fields, cut-chars |
| find | 5 | Go (jetzt) | find-optional-path, find-dot-prefix, find-maxdepth, find-exec, find-missing |
| grep | 7 | Go (jetzt) | grep-extended, grep-ignore-case, grep-word, grep-context, grep-only-match, grep-no-match, grep-recursive |
| head | 3 | Go (jetzt) | head-short, head-quiet, head-missing |
| tr | 3 | Go (jetzt) | tr-range, tr-complement, tr-squeeze |
| wc | 2 | Go (jetzt) | wc-chars, wc-bytes |
| cp | 5 | beides | cp-target-files, cp-archive, cp-target-dir, cp-archive-nested, cp-no-clobber |
| diff | 1 | beides | diff-header |
| patch | 2 | beides | patch-result, patch-apply |
| split | 2 | beides | split-count, split-lines |
| stat | 4 | beides | stat-size, stat-name, stat-missing, stat-no-operand |
| tar | 10 | beides | tar-strip-file, tar-create-gzip, tar-list-gzip, tar-list-without-z, tar-exclude, tar-exclude-worked, tar-extract-strip, tar-C-create, tar-C-listing, tar-j-refused |
| truncate | 1 | beides | truncate |
| awk | 2 | Go (später) | awk-sum, awk-fs |
| basename | 1 | Go (später) | basename |
| dirname | 1 | Go (später) | dirname |
| docx2md | 3 | Go (später) | docx2md-heading, docx2md-text, docx2md-not-a-docx |
| echo | 6 | Go (später) | echo-plain, echo-backslash, echo-escape-lines, echo-no-newline, help-echo-usage, echo-help-literal |
| epub2md | 3 | Go (später) | epub2md-table, epub2md-chapter-count, epub2md-refuses-zip |
| false | 1 | Go (später) | false-exit |
| file | 3 | Go (später) | file-text, file-dir, file-empty |
| hexdump | 3 | Go (später) | hexdump-canonical, hexdump-ascii, hexdump-final-offset |
| html2md | 4 | Go (später) | html2md-heading, html2md-link, html2md-escape, html2md-stdin-empty |
| iconv | 3 | Go (später) | iconv-latin1, iconv-cp1252, iconv-to-latin1 |
| jq | 8 | Go (später) | jq-field, jq-compact, jq-raw, jq-slurp, jq-arg, jq-bad-filter, jq-bad-json, jq-exit-false |
| md2docx | 6 | Go (später) | md2docx-is-zip, md2docx-heading, md2docx-bold, md2docx-list, md2docx-table, md2docx-unknown-option |
| md2epub | 4 | Go (später) | md2epub-is-zip, md2epub-mimetype, md2epub-mimetype-first, md2epub-roundtrip |
| md2pdf | 5 | Go (später) | md2pdf-is-pdf, md2pdf-has-eof, md2pdf-roundtrip, md2pdf-umlaut, md2pdf-unknown-option |
| md2txt-l | 4 | Go (später) | md2txt-l-heading, md2txt-l-inline, md2txt-l-table, md2txt-l-wrap |
| nl | 1 | Go (später) | nl-numbered |
| pdf2md | 6 | Go (später) | pdf2md-text, pdf2md-table, pdf2md-labels, pdf2md-labels-rows, pdf2md-form-xobject, pdf2md-not-pdf |
| pwd | 1 | Go (später) | pwd-here |
| rtf2md | 3 | Go (später) | rtf2md-paragraph, rtf2md-bold, rtf2md-not-rtf |
| sed | 2 | Go (später) | sed-sub, sed-quiet |
| sha256sum | 5 | Go (später) | sha256, sums-check-ok, sums-check-exit, sums-check-failed, sums-check-failed-exit |
| shuf | 2 | Go (später) | shuf-count, shuf-range |
| sort | 8 | Go (später) | sort-default, sort-numeric, sort-key, sort-version, sort-human, sort-unique, sort-disorder, sort-unknown-flag |
| tac | 1 | Go (später) | tac |
| true | 1 | Go (später) | true-exit |
| chmod | 6 | Batterie | chmod-plusx, chmod-bad-mode, chmod-bad-mode-message, chmod-try-hint, chmod-recursive, chmod-verbose |
| curl | 10 | Batterie | curl-help, curl-unknown-flag, curl-file-scheme, curl-writeout-code, curl-writeout-size, curl-include-status, curl-include-no-prefix, curl-include-has-type, curl-default-agent, curl-impersonate-unknown |
| df | 3 | Batterie | df-header, df-human, df-one-row |
| du | 4 | Batterie | du-summary-lines, du-human, du-total, du-unknown |
| env | 2 | Batterie | env-assign, env-unset |
| free | 3 | Batterie | free-mem-row, free-human, free-unknown |
| gibtsnicht | 3 | Batterie | unknown-command-exit, unknown-command-message, unknown-command-no-busybox |
| hostname | 2 | Batterie | hostname-present, hostname-set-refused |
| httpd | 4 | Batterie | httpd-file, httpd-404, httpd-detach-pid, httpd-detach-serves |
| kill | 2 | Batterie | kill-bad-signal, kill-bad-pid |
| killall | 1 | Batterie | killall-no-match |
| ls | 2 | Batterie | ls-unknown-flag, ls-sort-time |
| policy | 18 | Batterie | policy-no-outside-file, policy-unrestricted, policy-check-allowed, policy-unknown, policy-refused, policy-active, policy-shows-source, policy-check-allowed, policy-check-denied, policy-write-only-inside-allowed, policy-write-only-outside-denied, policy-broken-refused, policy-blocks-network, policy-network-message, policy-read-inside, policy-blocks-read, policy-blocks-write, policy-write-inside |
| ps | 4 | Batterie | ps-fields, ps-ef-ppid, ps-has-rows, ps-unknown-field |
| sh | 3 | Batterie | sh-echo, sh-var, sh-status |
| sync | 2 | Batterie | sync-file, sync-missing |
| timeout | 1 | Batterie | timeout-expires |
| touch | 2 | Batterie | touch-stamp, touch-date |
| tracedeps | 4 | Batterie | tracedeps-script, tracedeps-program, tracedeps-usage, tracedeps-says-no-windows |
| uname | 1 | Batterie | uname-machine |
| wget | 3 | Batterie | wget-help, wget-recursive-refused, wget-unknown-flag |
| xargs | 4 | Batterie | xargs-replace, xargs-chunk, xargs-no-run-empty, xargs-child-fails |

## Die sieben Kandidaten

| Befehl | Prüfungen | Ticket |
|---|---|---|
| find | 5 | acmesh-qz2 |
| grep | 7 | acmesh-4aj |
| head | 3 | acmesh-795 |
| wc | 2 | acmesh-795 |
| cut | 2 | acmesh-pfm |
| base64 | 2 | acmesh-pfm |
| tr | 3 | acmesh-49y |

Das Ticket nennt für tr sieben Prüfungen; gemessen sind es drei, die tr selbst prüfen
(tr-range, tr-complement, tr-squeeze). Die anderen waren die `tr -d '\r'`-Rohre in
Prüfungen anderer Befehle, keine eigenen.

## Was in der Batterie bleibt (acmesh-j4f)

- Windows-Verhalten und das andere Ziel: ps, df, du, free, chmod, cp -a, CRLF.
- Dateisystem, Rechte, Besitzer: chmod, cp, touch, stat, sync.
- Prozesse: kill, killall, httpd (Port, --detach), timeout, xargs, sh.
- Netz und TLS: curl, wget, ssh, scp gegen einen echten sshd.
- Policy im Unterprozess: Landlock lässt sich nur an einem laufenden Prozess messen.
- tracedeps: ptrace, nur Linux.
- u-root-Befehle ohne eigenes Paket (ls, cat, mv, rm, mkdir, gzip) und der
  Dispatcher selbst (unbekannter Befehl, Liste der Befehle).
- Größenbudget und Profile (fs ohne Netz, ext).

## Zahlen nachher

Wird beim Schließen der Kindtickets ergänzt; jedes Entfernen nennt den Go-Test, der
die Prüfung ersetzt.

    vorher   244 Prüfungen Linux, 222 Windows

Stand 19.11.2026:

    nach find      240 Prüfungen Linux, 218 Windows

| Ticket | Befehl | entfernte Prüfungen | ersetzt durch |
|---|---|---|---|
| acmesh-qz2 | find | find-optional-path, find-dot-prefix, find-maxdepth, find-missing | TestStartPathIsOptional, TestMaxDepthReportsTheBoundaryDirectory, TestErrorsAreNotSuccess |

find-exec bleibt: die Prüfung lässt den gebauten Befehl selbst als -exec-Programm
laufen, das ist der Fall "anderes Ziel", den ein Go-Test nicht abdeckt.
