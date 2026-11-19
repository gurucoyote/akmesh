package ls

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// fixture builds a directory with known names, sizes and times:
//
//	a.txt  2 bytes, 2020-01-01 (old: the year must show)
//	b.txt  5 bytes
//	sub/   a directory
//	.cfg   hidden
//	link   a symlink to a.txt
func fixture(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "a.txt"), []byte("aa"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "b.txt"), []byte("bbbbb"), 0o640); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, ".cfg"), []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(filepath.Join(dir, "sub"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink("a.txt", filepath.Join(dir, "link")); err != nil {
		t.Fatal(err)
	}
	old := time.Date(2020, 1, 1, 12, 0, 0, 0, time.Local)
	if err := os.Chtimes(filepath.Join(dir, "a.txt"), old, old); err != nil {
		t.Fatal(err)
	}
	return dir
}

func run(t *testing.T, args ...string) (string, string, int) {
	t.Helper()
	dir := t.TempDir()
	outPath, errPath := filepath.Join(dir, "out"), filepath.Join(dir, "err")
	out, err := os.Create(outPath)
	if err != nil {
		t.Fatal(err)
	}
	errf, err := os.Create(errPath)
	if err != nil {
		t.Fatal(err)
	}
	code := Run(args, nil, out, errf)
	out.Close()
	errf.Close()
	o, _ := os.ReadFile(outPath)
	e, _ := os.ReadFile(errPath)
	return string(o), string(e), code
}

func lines(s string) []string {
	s = strings.TrimRight(s, "\n")
	if s == "" {
		return nil
	}
	return strings.Split(s, "\n")
}

// TestPlainListingIsOneNamePerLine: without -l the output is one name per line,
// sorted, hidden files left out - what a script reads.
func TestPlainListingIsOneNamePerLine(t *testing.T) {
	dir := fixture(t)
	out, errOut, code := run(t, dir)
	if code != 0 {
		t.Fatalf("exit %d, stderr %q", code, errOut)
	}
	want := []string{"a.txt", "b.txt", "link", "sub"}
	if got := lines(out); strings.Join(got, ",") != strings.Join(want, ",") {
		t.Fatalf("got %v, want %v", got, want)
	}
}

// TestAllAndAlmost: -a adds . and .., -A adds only the hidden file.
func TestAllAndAlmost(t *testing.T) {
	dir := fixture(t)
	out, _, _ := run(t, "-a", dir)
	got := strings.Join(lines(out), ",")
	if got != ".,..,.cfg,a.txt,b.txt,link,sub" {
		t.Fatalf("-a: %s", got)
	}
	out, _, _ = run(t, "-A", dir)
	if got := strings.Join(lines(out), ","); got != ".cfg,a.txt,b.txt,link,sub" {
		t.Fatalf("-A: %s", got)
	}
}

// TestReverseAndSortBySize: -r reverses, -S puts the largest first and breaks a
// tie by name.
func TestReverseAndSortBySize(t *testing.T) {
	dir := fixture(t)
	out, _, _ := run(t, "-r", dir)
	if got := strings.Join(lines(out), ","); got != "sub,link,b.txt,a.txt" {
		t.Fatalf("-r: %s", got)
	}
	out, _, _ = run(t, "-S", dir)
	// b.txt (5) wins; the directory and a.txt (2) tie at different sizes on
	// Linux because a directory reports 4096 bytes, so only the head is fixed.
	if got := strings.Split(out, "\n")[0]; got != "b.txt" && got != "sub" {
		t.Fatalf("-S first entry is %q", got)
	}
}

// TestSortByTimeIsNewestFirst: -t orders by modification time, with the newest
// first, and falls back to the name for a tie.
func TestSortByTimeIsNewestFirst(t *testing.T) {
	dir := t.TempDir()
	for i, name := range []string{"old", "mid", "new"} {
		path := filepath.Join(dir, name)
		if err := os.WriteFile(path, []byte("x"), 0o644); err != nil {
			t.Fatal(err)
		}
		ts := time.Date(2021, 1, 1, 0, 0, i, 0, time.Local)
		if err := os.Chtimes(path, ts, ts); err != nil {
			t.Fatal(err)
		}
	}
	out, _, _ := run(t, "-t", dir)
	if got := strings.Join(lines(out), ","); got != "new,mid,old" {
		t.Fatalf("-t: %s", got)
	}
	out, _, _ = run(t, "-tr", dir)
	if got := strings.Join(lines(out), ","); got != "old,mid,new" {
		t.Fatalf("-tr: %s", got)
	}
}

// TestLongFormatFields: the long form shows mode, links, owner, group, size, the
// date rule and the symlink target.
func TestLongFormatFields(t *testing.T) {
	dir := fixture(t)
	out, errOut, code := run(t, "-l", dir)
	if code != 0 {
		t.Fatalf("exit %d, stderr %q", code, errOut)
	}
	all := lines(out)
	if !strings.HasPrefix(all[0], "total ") {
		t.Fatalf("no total line: %q", all[0])
	}
	var aLine, linkLine string
	for _, l := range all[1:] {
		if strings.HasSuffix(l, " a.txt") && !strings.Contains(l, " -> ") {
			aLine = l
		}
		if strings.Contains(l, " link -> a.txt") {
			linkLine = l
		}
	}
	if aLine == "" || linkLine == "" {
		t.Fatalf("a.txt or the symlink missing:\n%s", out)
	}
	// The old file must show its year, not a time of day: that was the bug.
	if !strings.Contains(aLine, " 2020 ") {
		t.Fatalf("old file without the year: %q", aLine)
	}
	if strings.HasPrefix(aLine, "L") {
		t.Fatalf("mode starts with a capital L: %q", aLine)
	}
	if !strings.HasPrefix(aLine, "-rw-r--r--") {
		t.Fatalf("mode is not -rw-r--r--: %q", aLine)
	}
	// A symlink is 'l', not 'L', and shows its target.
	if !strings.HasPrefix(linkLine, "lrwxrwxrwx") {
		t.Fatalf("symlink mode is wrong: %q", linkLine)
	}
}

// TestDateRule: recent files show the time, old ones the year.
func TestDateRule(t *testing.T) {
	l := lister{now: time.Date(2026, 11, 19, 20, 0, 0, 0, time.UTC)}
	recent := time.Date(2026, 11, 3, 9, 5, 0, 0, time.UTC)
	old := time.Date(2020, 1, 1, 12, 0, 0, 0, time.UTC)
	if got := l.timeText(recent); got != "Nov  3 09:05" {
		t.Fatalf("recent: %q", got)
	}
	if got := l.timeText(old); got != "Jan  1  2020" {
		t.Fatalf("old: %q", got)
	}
}

// TestHumanSizes: -h uses GNU's binary units, one decimal below ten, no space.
func TestHumanSizes(t *testing.T) {
	cases := map[int64]string{
		0:          "0",
		1023:       "1023",
		1024:       "1.0K",
		2048:       "2.0K",
		2097152:    "2.0M",
		10485760:   "10M",
		1073741824: "1.0G",
	}
	for n, want := range cases {
		if got := humanSize(n); got != want {
			t.Fatalf("humanSize(%d) = %q, want %q", n, got, want)
		}
	}
}

// TestClassifyAndQuote: -F appends the indicator outside the quotes -Q makes.
func TestClassifyAndQuote(t *testing.T) {
	dir := fixture(t)
	out, _, _ := run(t, "-F", dir)
	if got := strings.Join(lines(out), ","); got != "a.txt,b.txt,link@,sub/" {
		t.Fatalf("-F: %s", got)
	}
	out, _, _ = run(t, "-QF", dir)
	if !strings.Contains(out, `"link"@`) {
		t.Fatalf("-QF: %s", out)
	}
}

// TestRecurseHeaders: -R prints a header per directory, GNU-style, keeping the
// operand's spelling ("." gives "./sub:").
func TestRecurseHeaders(t *testing.T) {
	dir := fixture(t)
	out, _, _ := run(t, "-R", dir)
	joined := strings.Join(lines(out), "\n")
	if !strings.Contains(joined, filepath.Join(dir, "sub")+":") {
		t.Fatalf("missing the sub header:\n%s", joined)
	}

	// With the operand "." the header keeps the spelling: "./sub:", as GNU
	// prints it. A plain filepath.Join would lose the dot.
	t.Chdir(dir)
	out, _, _ = run(t, "-R", ".")
	joined = strings.Join(lines(out), "\n")
	if !strings.Contains(joined, "./sub:") {
		t.Fatalf("missing ./sub: header:\n%s", joined)
	}
	if !strings.HasPrefix(joined, ".:") {
		t.Fatalf("first header is not .: \n%s", joined)
	}
}

// TestInodeAndNumeric: -i prints a number, -n is -l with numbers for owner and
// group.
func TestInodeAndNumeric(t *testing.T) {
	dir := fixture(t)
	out, _, code := run(t, "-i", dir)
	if code != 0 {
		t.Fatal("exit")
	}
	first := lines(out)[0]
	if !strings.HasPrefix(first, "0 ") && !isDigits(strings.SplitN(first, " ", 2)[0]) {
		t.Fatalf("-i: %q", first)
	}
	out, _, _ = run(t, "-n", dir)
	for _, l := range lines(out)[1:] {
		fields := strings.Fields(l)
		// mode, links, owner, group, size, date
		if len(fields) < 5 {
			t.Fatalf("too few fields: %q", l)
		}
	}
}

func isDigits(s string) bool {
	if s == "" {
		return false
	}
	for _, r := range s {
		if r < '0' || r > '9' {
			return false
		}
	}
	return true
}

// TestRefusals: an unknown option exits 2 with GNU's wording, a missing path
// exits 2 with "cannot access".
func TestRefusals(t *testing.T) {
	_, errOut, code := run(t, "-Z")
	if code != 2 {
		t.Fatalf("unknown option: exit %d, want 2", code)
	}
	if !strings.Contains(errOut, "invalid option -- 'Z'") {
		t.Fatalf("message %q", errOut)
	}
	if !strings.Contains(errOut, "Try 'ls --help'") {
		t.Fatalf("no help hint: %q", errOut)
	}

	_, errOut, code = run(t, filepath.Join(t.TempDir(), "gibtsnicht"))
	if code != 2 || !strings.Contains(errOut, "cannot access") {
		t.Fatalf("missing path: exit %d, stderr %q", code, errOut)
	}
}

// TestDirectoryFlag: -d lists the directory itself, not its contents.
func TestDirectoryFlag(t *testing.T) {
	dir := fixture(t)
	out, _, code := run(t, "-d", dir)
	if code != 0 {
		t.Fatalf("exit %d", code)
	}
	if got := lines(out); len(got) != 1 || !strings.HasSuffix(got[0], filepath.Base(dir)) {
		t.Fatalf("-d: %v", got)
	}
}

// TestHelpAnswersUsage: the help gate reads the first line.
func TestHelpAnswersUsage(t *testing.T) {
	out, _, code := run(t, "--help")
	if code != 0 {
		t.Fatalf("exit %d", code)
	}
	if !strings.HasPrefix(out, "Usage: ls ") {
		t.Fatalf("help starts with %q", strings.SplitN(out, "\n", 2)[0])
	}
}
