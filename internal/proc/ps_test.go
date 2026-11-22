package proc

import (
	"io"
	"os"
	"strings"
	"testing"
	"time"
)

// TestParsePSInvocations pins the command lines agents actually type. If one of
// these fails, an agent gets an error message instead of a process list, which
// is worse than a slightly different column set.
func TestParsePSInvocations(t *testing.T) {
	cases := []struct {
		args []string
		all  bool
		full bool
		bsd  bool
	}{
		{nil, false, false, false},
		{[]string{"-e"}, true, false, false},
		{[]string{"-A"}, true, false, false},
		{[]string{"-f"}, false, true, false},
		{[]string{"-ef"}, true, true, false},
		{[]string{"-fe"}, true, true, false},
		{[]string{"aux"}, true, false, true},
		{[]string{"-aux"}, true, false, true},
		{[]string{"-x"}, false, false, true},
	}
	for _, c := range cases {
		req, err := parsePS(c.args)
		if err != nil {
			t.Fatalf("parsePS(%v): %v", c.args, err)
		}
		if req.all != c.all || req.full != c.full || req.bsd != c.bsd {
			t.Fatalf("parsePS(%v) = all:%v full:%v bsd:%v, want all:%v full:%v bsd:%v",
				c.args, req.all, req.full, req.bsd, c.all, c.full, c.bsd)
		}
	}
}

func TestParsePSFieldsAndPids(t *testing.T) {
	req, err := parsePS([]string{"-eo", "pid,ppid,cmd"})
	if err != nil {
		t.Fatalf("parsePS: %v", err)
	}
	if len(req.fields) != 3 || req.fields[0].name != "pid" || req.fields[2].name != "cmd" {
		t.Fatalf("fields = %+v", req.fields)
	}
	req, err = parsePS([]string{"-p", "1,42"})
	if err != nil {
		t.Fatalf("parsePS: %v", err)
	}
	if len(req.pidOnly) != 2 || req.pidOnly[1] != 42 {
		t.Fatalf("pids = %v", req.pidOnly)
	}
	// A bare pid list works too, as in ps 1234.
	req, err = parsePS([]string{"1234"})
	if err != nil || len(req.pidOnly) != 1 {
		t.Fatalf("bare pid: %v, %v", req.pidOnly, err)
	}
}

func TestParsePSRejectsUnknown(t *testing.T) {
	for _, args := range [][]string{{"-Z"}, {"-efx"}, {"--weird"}} {
		if _, err := parsePS(args); err == nil {
			t.Fatalf("parsePS(%v) should fail", args)
		}
	}
	if _, err := parsePS([]string{"-eo", "pid,bogus"}); err == nil {
		t.Fatalf("unknown field should fail")
	}
}

// TestColumnsFor checks the layouts, including the one -f must produce because
// agents read PPID from it.
func TestColumnsFor(t *testing.T) {
	headerFor := func(args ...string) string {
		req, err := parsePS(args)
		if err != nil {
			t.Fatalf("parsePS(%v): %v", args, err)
		}
		var parts []string
		for _, f := range req.columnsFor() {
			parts = append(parts, f.header)
		}
		return strings.Join(parts, " ")
	}
	if got := headerFor("-ef"); got != "UID PID PPID C STIME TTY TIME CMD" {
		t.Fatalf("-ef header = %q, want the procps columns", got)
	}
	if got := headerFor("-f"); got != "UID PID PPID C STIME TTY TIME CMD" {
		t.Fatalf("-f header = %q, want the procps columns", got)
	}
	if got := headerFor("aux"); got != "USER PID %CPU %MEM VSZ RSS TTY STAT START TIME COMMAND" {
		t.Fatalf("aux header = %q, want the procps columns", got)
	}
	if got := headerFor(); got != "PID TTY TIME CMD" {
		t.Fatalf("default header = %q", got)
	}
	if got := headerFor("-eo", "pid,comm"); got != "PID COMM" {
		t.Fatalf("-eo header = %q", got)
	}
	if got := headerFor("-eo", "pid,vsz,rss,pcpu,pmem,start,stime,c"); got != "PID VSZ RSS %CPU %MEM START STIME C" {
		t.Fatalf("-eo header = %q", got)
	}
}

func TestTimeFormatting(t *testing.T) {
	cases := []struct {
		in   time.Duration
		want string
	}{
		{0, "00:00:00"},
		{65 * time.Second, "00:01:05"},
		{3661 * time.Second, "01:01:01"},
		{-1, "-"},
	}
	for _, c := range cases {
		if got := formatCPUTime(c.in); got != c.want {
			t.Fatalf("formatCPUTime(%v) = %q, want %q", c.in, got, c.want)
		}
	}
	short := []struct {
		in   time.Duration
		want string
	}{
		{0, "0:00"},
		{65 * time.Second, "1:05"},
		{3661 * time.Second, "1:01:01"},
	}
	for _, c := range short {
		if got := formatCPUTimeShort(c.in); got != c.want {
			t.Fatalf("formatCPUTimeShort(%v) = %q, want %q", c.in, got, c.want)
		}
	}
	elapsed := []struct {
		in   time.Duration
		want string
	}{
		{30 * time.Second, "00:30"},
		{2 * time.Hour, "02:00:00"},
		{26 * time.Hour, "1-02:00:00"},
		{-1, "-"},
	}
	for _, c := range elapsed {
		if got := formatElapsed(c.in); got != c.want {
			t.Fatalf("formatElapsed(%v) = %q, want %q", c.in, got, c.want)
		}
	}
}

// TestMatchesName covers the killall comparison: case, .exe suffix, and the
// command line fallback.
func TestMatchesName(t *testing.T) {
	p := procInfo{Comm: "PING.EXE", CmdLine: `C:\Windows\System32\PING.EXE -n 5 127.0.0.1`}
	for _, want := range []string{"ping", "PING", "PING.EXE", "ping.exe"} {
		if !matchesName(p, []string{want}) {
			t.Fatalf("%q should match %q", want, p.Comm)
		}
	}
	for _, want := range []string{"pin", "pings", "explorer"} {
		if matchesName(p, []string{want}) {
			t.Fatalf("%q must not match %q", want, p.Comm)
		}
	}
	// Windows puts the image path in quotes in the command line it reports, and
	// killall has to see through them (acmesh-94e.22).
	quoted := procInfo{Comm: "CMD.EXE", CmdLine: `"C:\Windows\system32\cmd.exe" /c dir`}
	for _, want := range []string{"cmd", "CMD.EXE"} {
		if !matchesName(quoted, []string{want}) {
			t.Fatalf("%q should match a quoted Windows command line", want)
		}
	}
	if got := firstWord(quoted.CmdLine); got != `C:\Windows\system32\cmd.exe` {
		t.Fatalf("firstWord = %q, want the path without quotes", got)
	}

	// A process whose name has no .exe, as on Linux.
	q := procInfo{Comm: "sleep", CmdLine: "sleep 300"}
	if !matchesName(q, []string{"sleep"}) {
		t.Fatalf("sleep should match itself")
	}
}

// TestPrintTableAlignment checks that columns line up and numbers are right
// aligned, because that is what awk-based scripts rely on.
func TestPrintTableAlignment(t *testing.T) {
	procs := []procInfo{
		{PID: 1, Comm: "init", Elapsed: time.Minute, CPUTime: time.Second, TTY: "?"},
		{PID: 12345, Comm: "a-very-long-command-name", Elapsed: time.Hour, CPUTime: time.Minute, TTY: "pts/0"},
	}
	// printTable writes to an *os.File, so the check goes through a temp file.
	f, err := os.CreateTemp(t.TempDir(), "ps")
	if err != nil {
		t.Fatalf("temp file: %v", err)
	}
	defer f.Close()
	req, _ := parsePS([]string{"-eo", "pid,tty,time,cmd"})
	printTable(f, req.columnsFor(), procs)
	if _, err := f.Seek(0, 0); err != nil {
		t.Fatalf("seek: %v", err)
	}
	data, err := io.ReadAll(f)
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	out := string(data)
	lines := strings.Split(strings.TrimRight(out, "\n"), "\n")
	if len(lines) != 3 {
		t.Fatalf("expected header plus two rows, got %d lines:\n%s", len(lines), out)
	}
	if !strings.HasPrefix(lines[1], "    1") {
		t.Fatalf("pid 1 should be right aligned in its column: %q", lines[1])
	}
	if lines[2][:6] != "12345 " {
		t.Fatalf("pid column width should follow the widest value: %q", lines[2])
	}
}

// TestProcpsValueSpellings pins the spellings procps 4.0.4 prints and the way
// the derived numbers are rounded: START today is a clock, an older day is
// MonDD, an older year is the year; C truncates, %CPU and %MEM keep one decimal.
func TestProcpsValueSpellings(t *testing.T) {
	now := time.Date(2026, 11, 22, 17, 0, 0, 0, time.UTC)
	cases := []struct {
		start time.Time
		short bool
		want  string
	}{
		{time.Date(2026, 11, 22, 9, 5, 7, 0, time.UTC), false, "09:05:07"},
		{time.Date(2026, 11, 22, 9, 5, 7, 0, time.UTC), true, "09:05"},
		{time.Date(2026, 11, 19, 9, 5, 7, 0, time.UTC), true, "Nov19"},
		{time.Date(2025, 12, 31, 9, 5, 7, 0, time.UTC), true, "2025"},
		{time.Time{}, true, "-"},
	}
	for _, c := range cases {
		if got := formatStart(c.start, now, c.short); got != c.want {
			t.Fatalf("formatStart(%v, short=%v) = %q, want %q", c.start, c.short, got, c.want)
		}
	}
	if got := formatPercent(92.46); got != "92.5" {
		t.Fatalf("formatPercent(92.46) = %q, want 92.5", got)
	}
	if got := formatPercent(unknown); got != "-" {
		t.Fatalf("formatPercent(unknown) = %q, want -", got)
	}
	if got := formatCPUInt(92.9); got != "92" {
		t.Fatalf("formatCPUInt(92.9) = %q, want 92 (truncated like procps)", got)
	}
	if got := formatKB(2680); got != "2680" {
		t.Fatalf("formatKB = %q", got)
	}
	if got := formatKB(unknown); got != "-" {
		t.Fatalf("formatKB(unknown) = %q", got)
	}
	if got := orQuestion(""); got != "?" {
		t.Fatalf("orQuestion(empty) = %q, want ?", got)
	}
}
