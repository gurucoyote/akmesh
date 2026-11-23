// Package syslog holds the three logging commands of a booted container:
// syslogd (a /dev/log socket and a log file with rotation), logger (a message on
// that socket) and logread (read the file, optionally following it).
//
// The wire format is RFC 3164, what BusyBox and every /dev/log writer produce:
//
//	<PRI>Jan _2 15:04:05 TAG[PID]: message
//
// syslogd stores the line as it arrived, so what logread shows is what the
// program sent - the same behaviour BusyBox has, and the reason a bug report can
// be compared with the program's own output.
//
// Deliberate difference to BusyBox: logread reads the log FILE, not a shared
// memory ring. BusyBox's ring lives in its own ipc and exists only while syslogd
// runs; a file survives a restart and can be read by anything.
package syslog

import (
	"bufio"
	"fmt"
	"io"
	"strconv"
	"strings"
	"time"
)

// newLineScanner reads lines of up to 1 MiB: a stack trace in one line is not
// unusual, and a 64 KiB default would cut it in two.
func newLineScanner(r io.Reader) *bufio.Scanner {
	sc := bufio.NewScanner(r)
	sc.Buffer(make([]byte, 0, 64*1024), 1024*1024)
	return sc
}

// facilities is the RFC 3164 name table, index = number.
var facilities = []string{
	"kern", "user", "mail", "daemon", "auth", "syslog", "lpr", "news",
	"uucp", "cron", "authpriv", "ftp",
	"", "", "", "", "local0", "local1", "local2", "local3", "local4", "local5",
	"local6", "local7",
}

// levels is the severity table, index = number.
var levels = []string{"emerg", "alert", "crit", "err", "warning", "notice", "info", "debug"}

// levelAliases are the spellings tools accept (syslog.h names and the common
// abbreviations).
var levelAliases = map[string]int{
	"panic": 0, "emerg": 0,
	"alert": 1,
	"crit":  2, "critical": 2,
	"err": 3, "error": 3,
	"warn": 4, "warning": 4,
	"notice": 5,
	"info":   6, "informational": 6,
	"debug": 7,
}

// ParsePriority reads "facility.level", "level" or a plain number like
// "local0.info" / "err" / "30". It returns the RFC 3164 priority value.
func ParsePriority(text string) (int, error) {
	text = strings.TrimSpace(text)
	if text == "" {
		return 13, nil // user.notice, the default of logger
	}
	if n, err := strconv.Atoi(text); err == nil {
		if n < 0 || n > 191 {
			return 0, fmt.Errorf("priority %d out of range 0..191", n)
		}
		return n, nil
	}
	fac := 1
	if dot := strings.Index(text, "."); dot >= 0 {
		name := text[:dot]
		text = text[dot+1:]
		f, err := facilityNumber(name)
		if err != nil {
			return 0, err
		}
		fac = f
	}
	l, ok := levelAliases[strings.ToLower(text)]
	if !ok {
		return 0, fmt.Errorf("unknown level %q (known: emerg, alert, crit, err, warning, notice, info, debug)", text)
	}
	return fac*8 + l, nil
}

func facilityNumber(name string) (int, error) {
	name = strings.ToLower(name)
	for i, f := range facilities {
		if f != "" && f == name {
			return i, nil
		}
	}
	return 0, fmt.Errorf("unknown facility %q (known: kern, user, mail, daemon, auth, syslog, lpr, news, uucp, cron, authpriv, ftp, local0-7)", name)
}

// Format builds one RFC 3164 line without the trailing newline.
func Format(priority int, tag string, pid int, msg string, at time.Time) string {
	tag = sanitizeTag(tag)
	if pid > 0 {
		tag = fmt.Sprintf("%s[%d]", tag, pid)
	}
	// RFC 3164: the day is space padded, not zero padded.
	ts := fmt.Sprintf("%s %2d %02d:%02d:%02d", at.Format("Jan"), at.Day(), at.Hour(), at.Minute(), at.Second())
	msg = strings.ReplaceAll(msg, "\n", " ")
	return fmt.Sprintf("<%d>%s %s: %s", priority, ts, tag, msg)
}

// sanitizeTag removes what would break the one-line format.
func sanitizeTag(tag string) string {
	if tag == "" {
		return "user"
	}
	var b strings.Builder
	for _, c := range tag {
		switch {
		case c == ' ' || c == '[' || c == ']' || c == ':' || c == '\n' || c == '\r':
			b.WriteByte('_')
		default:
			b.WriteRune(c)
		}
	}
	return b.String()
}

// normalizeLine makes a received datagram safe to store: no newlines inside, one
// trailing newline at the end.
func normalizeLine(msg string) (string, bool) {
	msg = strings.TrimRight(msg, "\r\n")
	msg = strings.ReplaceAll(msg, "\r", "")
	if strings.TrimSpace(msg) == "" {
		return "", false
	}
	return msg + "\n", true
}
