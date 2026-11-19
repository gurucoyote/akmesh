package head

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// runHeadTest calls Run directly, the way the dispatcher does, and returns
// stdout plus the exit code.
func runHeadTest(t *testing.T, args []string, input string) (string, string, int) {
	t.Helper()
	var out, errOut strings.Builder
	code := Run(args, strings.NewReader(input), &out, &errOut)
	return out.String(), errOut.String(), code
}

// TestShortFormAndQuiet is the reason for the fork: agents write head -5 and
// head -q, and upstream's flag package refused the first and ignored the second.
func TestShortFormAndQuiet(t *testing.T) {
	out, _, code := runHeadTest(t, []string{"-5"}, "1\n2\n3\n4\n5\n6\n7\n")
	if code != 0 || out != "1\n2\n3\n4\n5\n" {
		t.Fatalf("head -5 = %d, %q; want five lines", code, out)
	}
	dir := t.TempDir()
	one := filepath.Join(dir, "a.txt")
	two := filepath.Join(dir, "b.txt")
	if err := os.WriteFile(one, []byte("a\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(two, []byte("b\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	out, _, code = runHeadTest(t, []string{one, two}, "")
	if code != 0 || !strings.Contains(out, "==> "+one+" <==") {
		t.Fatalf("two files = %d, %q; want a header per file", code, out)
	}
	out, _, code = runHeadTest(t, []string{"-q", one, two}, "")
	if code != 0 || strings.Contains(out, "==>") || out != "a\nb\n" {
		t.Fatalf("head -q = %d, %q; want the bodies without headers", code, out)
	}
}

// TestNormalFormStillWorks: the upstream behaviour has to survive the fork.
func TestNormalFormStillWorks(t *testing.T) {
	out, _, code := runHeadTest(t, []string{"-n", "2"}, "a\nb\nc\n")
	if code != 0 || out != "a\nb\n" {
		t.Fatalf("head -n 2 = %d, %q", code, out)
	}
	out, _, code = runHeadTest(t, []string{"-c", "3"}, "abcdef")
	if code != 0 || out != "abc" {
		t.Fatalf("head -c 3 = %d, %q", code, out)
	}
}

// TestErrorsAreNotSuccess: a missing file and an unknown option must not exit 0,
// which is what upstream did with the read error.
func TestErrorsAreNotSuccess(t *testing.T) {
	if _, _, code := runHeadTest(t, []string{"/nicht/vorhanden"}, ""); code != 1 {
		t.Fatalf("missing file exit %d, want 1", code)
	}
	if _, _, code := runHeadTest(t, []string{"-Z"}, "a\n"); code != 2 {
		t.Fatalf("unknown option exit %d, want 2", code)
	}
	// -n and -c are not an error in GNU: the last one wins. This used to be a
	// "can't combine" error, which no agent expects from head.
	out, _, code := runHeadTest(t, []string{"-n", "2", "-c", "2"}, "abcdef\n")
	if code != 0 || out != "ab" {
		t.Fatalf("-n 2 -c 2 = %d, %q, want the two bytes", code, out)
	}
	out, _, _ = runHeadTest(t, []string{"-c", "2", "-n", "1"}, "abcdef\ng\n")
	if out != "abcdef\n" {
		t.Fatalf("-c 2 -n 1 = %q, want the line back", out)
	}
}

// writeHeads lays down the fixtures the file tests share.
func writeHeads(t *testing.T, files map[string]string) string {
	t.Helper()
	dir := t.TempDir()
	for name, body := range files {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return dir
}

// TestNegativeCountKeepsEverythingButTheEnd: GNU reads "-n -2" as "all but the
// last two lines". It used to behave like "two", which cuts a log the wrong way.
func TestNegativeCountKeepsEverythingButTheEnd(t *testing.T) {
	in := "1\n2\n3\n4\n5\n"
	out, _, code := runHeadTest(t, []string{"-n", "-2"}, in)
	if code != 0 || out != "1\n2\n3\n" {
		t.Fatalf("head -n -2 = %d, %q, want the first three", code, out)
	}
	out, _, _ = runHeadTest(t, []string{"-n", "-1"}, "a\nb\n")
	if out != "a\n" {
		t.Fatalf("head -n -1 = %q, want a", out)
	}
	// Asking for more than there is gives an empty answer, not an error.
	out, _, code = runHeadTest(t, []string{"-n", "-9"}, "a\nb\n")
	if code != 0 || out != "" {
		t.Fatalf("head -n -9 = %d, %q, want nothing", code, out)
	}
}

// TestNegativeBytesKeepTheHead: -c -N drops the last N bytes, GNU's byte form of
// the same idea.
func TestNegativeBytesKeepTheHead(t *testing.T) {
	out, _, code := runHeadTest(t, []string{"-c", "-3"}, "1\n2\n3\n4\n5\n")
	if code != 0 || out != "1\n2\n3\n4" {
		t.Fatalf("head -c -3 = %d, %q, want four lines without the newline", code, out)
	}
}

// TestZeroPrintsNothing: "-n 0" and "-c 0" mean nothing at all. A zero used to
// fall through to the default ten lines.
func TestZeroPrintsNothing(t *testing.T) {
	for _, args := range [][]string{{"-n", "0"}, {"-c", "0"}, {"-n0"}, {"-c0"}} {
		out, _, code := runHeadTest(t, args, "1\n2\n3\n")
		if code != 0 || out != "" {
			t.Errorf("%v = %d, %q, want nothing", args, code, out)
		}
	}
}

// TestHeadersFollowTheLastOption: -v forces a header, -q suppresses it, and the
// later of the two wins, as GNU does.
func TestHeadersFollowTheLastOption(t *testing.T) {
	dir := writeHeads(t, map[string]string{"a.txt": "a1\na2\n", "b.txt": "b1\n"})
	a, b := filepath.Join(dir, "a.txt"), filepath.Join(dir, "b.txt")

	out, _, code := runHeadTest(t, []string{a, b}, "")
	if code != 0 {
		t.Fatalf("exit %d", code)
	}
	want := "==> " + a + " <==\na1\na2\n\n==> " + b + " <==\nb1\n"
	if out != want {
		t.Fatalf("two files = %q, want %q", out, want)
	}

	// -v with one file: a header anyway.
	out, _, _ = runHeadTest(t, []string{"-v", a}, "")
	if !strings.HasPrefix(out, "==> "+a+" <==\n") {
		t.Fatalf("-v = %q, want a header", out)
	}
	// -v -q: silent. -q -v: headed again.
	out, _, _ = runHeadTest(t, []string{"-v", "-q", a, b}, "")
	if strings.Contains(out, "==>") {
		t.Fatalf("-v -q = %q, want no headers", out)
	}
	out, _, _ = runHeadTest(t, []string{"-q", "-v", a, b}, "")
	if !strings.Contains(out, "==>") {
		t.Fatalf("-q -v = %q, want headers", out)
	}
}

// TestShortFileIsNotAnError: asking for more lines than a file has must not fail
// a script.
func TestShortFileIsNotAnError(t *testing.T) {
	dir := writeHeads(t, map[string]string{"two.txt": "a\nb\n"})
	out, errOut, code := runHeadTest(t, []string{"-n", "100", filepath.Join(dir, "two.txt")}, "")
	if code != 0 || out != "a\nb\n" || errOut != "" {
		t.Fatalf("-n 100 = %d, %q, stderr %q", code, out, errOut)
	}
}

// TestParseCountSpellings: the multiplier suffixes and the sign, directly on the
// parser, because they are the part a shell script leans on.
func TestParseCountSpellings(t *testing.T) {
	cases := []struct {
		in   string
		want int
		ok   bool
	}{
		{"3", 3, true},
		{"+3", 3, true},
		{"-2", -2, true},
		{"1k", 1024, true},
		{"2K", 2048, true},
		{"1m", 1024 * 1024, true},
		{"2M", 2 * 1024 * 1024, true},
		{"", 0, false},
		{"x", 0, false},
	}
	for _, c := range cases {
		got, err := parseCount(c.in)
		if c.ok && (err != nil || got != c.want) {
			t.Errorf("parseCount(%q) = %d, %v, want %d", c.in, got, err, c.want)
		}
		if !c.ok && err == nil {
			t.Errorf("parseCount(%q) accepted a bad count", c.in)
		}
	}
}

// TestLongFormsAndCombinedShortForm: the spellings an agent writes without
// thinking: --lines=2, --bytes=2, --silent, -q3.
func TestLongFormsAndCombinedShortForm(t *testing.T) {
	in := "1\n2\n3\n4\n"
	cases := []struct {
		args []string
		want string
	}{
		{[]string{"--lines=2"}, "1\n2\n"},
		{[]string{"--bytes=3"}, "1\n2"},
		{[]string{"--lines", "2"}, "1\n2\n"},
		{[]string{"--bytes", "2"}, "1\n"},
		{[]string{"-n2"}, "1\n2\n"},
		{[]string{"-c3"}, "1\n2"},
		{[]string{"-q3"}, "1\n2\n3\n"},
	}
	for _, c := range cases {
		out, errOut, code := runHeadTest(t, c.args, in)
		if code != 0 || out != c.want {
			t.Errorf("%v = %d, %q (stderr %q), want %q", c.args, code, out, errOut, c.want)
		}
	}
	// --silent is the long spelling of -q.
	dir := writeHeads(t, map[string]string{"a.txt": "a\n", "b.txt": "b\n"})
	out, _, _ := runHeadTest(t, []string{"--silent", filepath.Join(dir, "a.txt"), filepath.Join(dir, "b.txt")}, "")
	if strings.Contains(out, "==>") {
		t.Fatalf("--silent = %q, want no headers", out)
	}
}

// TestUsageErrorsOnTheirOwn: a missing value and a broken count each get exit 2
// and a message, and "--" still ends the options for a file called "-n".
func TestUsageErrorsOnTheirOwn(t *testing.T) {
	for _, args := range [][]string{{"-n"}, {"-c"}, {"-n", "x"}, {"--lines=x"}} {
		_, errOut, code := runHeadTest(t, args, "a\n")
		if code != 2 || errOut == "" {
			t.Errorf("%v = %d, stderr %q, want 2 and a message", args, code, errOut)
		}
	}
	dir := writeHeads(t, map[string]string{"-n": "dash\n"})
	out, _, code := runHeadTest(t, []string{"--", filepath.Join(dir, "-n")}, "")
	if code != 0 || out != "dash\n" {
		t.Fatalf("-- file = %d, %q, want the file content", code, out)
	}
}
