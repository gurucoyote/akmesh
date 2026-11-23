//go:build !windows

package syslog

import (
	"bytes"
	"net"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestParsePriority(t *testing.T) {
	cases := map[string]int{
		"local0.info": 134,
		"user.notice": 13,
		"err":         11, // user.err
		"debug":       15, // user.debug
		"30":          30,
		"":            13,
	}
	for in, want := range cases {
		got, err := ParsePriority(in)
		if err != nil {
			t.Errorf("%q: %v", in, err)
			continue
		}
		if got != want {
			t.Errorf("%q = %d, want %d", in, got, want)
		}
	}
	for _, bad := range []string{"nosuchf.level", "local0.nosuchlevel", "200"} {
		if _, err := ParsePriority(bad); err == nil {
			t.Errorf("%q accepted", bad)
		}
	}
}

func TestFormat(t *testing.T) {
	at := time.Date(2026, 11, 23, 13, 30, 5, 0, time.UTC)
	got := Format(134, "mytool", 42, "hallo welt", at)
	want := "<134>Nov 23 13:30:05 mytool[42]: hallo welt"
	if got != want {
		t.Fatalf("got %q, want %q", got, want)
	}
	// A newline inside the message would split the record.
	got = Format(13, "t", 0, "zeile1\nzeile2", at)
	if strings.Count(got, "\n") != 0 {
		t.Fatalf("message kept a newline: %q", got)
	}
	// A tag that would break "tag[pid]:" is repaired, not dropped.
	got = Format(13, "a b]", 1, "x", at)
	if strings.Contains(got, "a b]") || strings.Contains(got, "]]") {
		t.Fatalf("tag not sanitised: %q", got)
	}
	// The day is space padded like RFC 3164 wants.
	single := Format(13, "t", 0, "x", time.Date(2026, 11, 3, 1, 2, 3, 0, time.UTC))
	if !strings.Contains(single, "Nov  3 01:02:03") {
		t.Fatalf("day not padded: %q", single)
	}
}

func TestNormalizeLine(t *testing.T) {
	got, ok := normalizeLine("hallo\n")
	if !ok || got != "hallo\n" {
		t.Fatalf("got %q ok=%v", got, ok)
	}
	if _, ok := normalizeLine("  \r\n"); ok {
		t.Fatal("empty line accepted")
	}
	got, _ = normalizeLine("a\r\nb\r\n")
	if strings.Contains(got, "\r") {
		t.Fatalf("carriage return kept: %q", got)
	}
}

// TestFileWriterRotates is the rotation duty: the file stays bounded and the old
// content is in NAME.0, not gone.
func TestFileWriterRotates(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "messages")
	w, err := OpenFileWriter(path, 100)
	if err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 20; i++ {
		if _, err := w.Write([]byte("0123456789\n")); err != nil {
			t.Fatal(err)
		}
	}
	w.Close()
	if st, err := os.Stat(path); err != nil || st.Size() > 100 {
		t.Fatalf("file size %v err %v, want <= 100", st, err)
	}
	backup, err := os.ReadFile(path + ".0")
	if err != nil {
		t.Fatalf("no backup: %v", err)
	}
	if len(backup) == 0 {
		t.Fatal("backup empty")
	}
}

// TestLoggerSendsToSocket runs the real code path: a datagram arrives with the
// priority, the tag and the pid in place.
func TestLoggerSendsToSocket(t *testing.T) {
	dir := t.TempDir()
	sock := filepath.Join(dir, "log")
	addr, err := net.ResolveUnixAddr("unixgram", sock)
	if err != nil {
		t.Fatal(err)
	}
	conn, err := net.ListenUnixgram("unixgram", addr)
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()

	var stdout, stderr bytes.Buffer
	code := RunLogger("logger", []string{"-S", sock, "-t", "probe", "-p", "local0.info", "hallo welt"},
		strings.NewReader(""), &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit %d: %s", code, stderr.String())
	}
	buf := make([]byte, 4096)
	n, _, err := conn.ReadFromUnix(buf)
	if err != nil {
		t.Fatal(err)
	}
	line := string(buf[:n])
	if !strings.HasPrefix(line, "<134>") {
		t.Errorf("priority missing: %q", line)
	}
	if !strings.Contains(line, "probe[") || !strings.HasSuffix(line, "hallo welt") {
		t.Errorf("line = %q", line)
	}
}

// TestLoggerReadsStdin: one message per input line, which is what a pipe needs.
func TestLoggerReadsStdin(t *testing.T) {
	dir := t.TempDir()
	sock := filepath.Join(dir, "log")
	addr, _ := net.ResolveUnixAddr("unixgram", sock)
	conn, err := net.ListenUnixgram("unixgram", addr)
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()
	var stdout, stderr bytes.Buffer
	if code := RunLogger("logger", []string{"-S", sock, "-t", "p"}, strings.NewReader("eins\nzwei\n"), &stdout, &stderr); code != 0 {
		t.Fatalf("exit %d: %s", code, stderr.String())
	}
	buf := make([]byte, 4096)
	for i, want := range []string{"eins", "zwei"} {
		n, _, err := conn.ReadFromUnix(buf)
		if err != nil {
			t.Fatalf("line %d: %v", i, err)
		}
		if !strings.HasSuffix(string(buf[:n]), want) {
			t.Fatalf("line %d = %q, want suffix %q", i, string(buf[:n]), want)
		}
	}
}

// TestLogreadTail reads the last lines of a real file.
func TestLogreadTail(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "messages")
	if err := os.WriteFile(path, []byte("a\nb\nc\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	var stdout, stderr bytes.Buffer
	if code := RunLogread("logread", []string{"-O", path, "-n", "2"}, &stdout, &stderr); code != 0 {
		t.Fatalf("exit %d: %s", code, stderr.String())
	}
	if stdout.String() != "b\nc\n" {
		t.Fatalf("got %q, want %q", stdout.String(), "b\nc\n")
	}
}
