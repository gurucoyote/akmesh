package wc

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func runWc(t *testing.T, stdin string, args ...string) (string, int) {
	t.Helper()
	var out, errOut strings.Builder
	code := Run(args, strings.NewReader(stdin), &out, &errOut)
	return out.String(), code
}

// TestCharsSwitch: -m counts characters, GNU's spelling. Upstream only knew -r.
func TestCharsSwitch(t *testing.T) {
	out, code := runWc(t, "abc\n")
	if code != 0 {
		t.Fatalf("exit %d", code)
	}
	if strings.TrimSpace(out) != "1 1 4" {
		t.Fatalf("default = %q, want line, word and byte counts", out)
	}
	out, _ = runWc(t, "héllo\n", "-m")
	if strings.TrimSpace(out) != "6" {
		t.Fatalf("-m = %q, want 6 characters", out)
	}
	out, _ = runWc(t, "héllo\n", "-c")
	if strings.TrimSpace(out) != "7" {
		t.Fatalf("-c = %q, want 7 bytes", out)
	}
}

// TestCombinedFlags: -lw is read as -l -w.
func TestCombinedFlags(t *testing.T) {
	out, _ := runWc(t, "a b\nc\n", "-lw")
	if strings.TrimSpace(out) != "2 3" {
		t.Fatalf("-lw = %q", out)
	}
}

// TestFiles0From: the names come NUL-separated from a file, as find -print0
// writes them.
func TestFiles0From(t *testing.T) {
	dir := t.TempDir()
	one := filepath.Join(dir, "one.txt")
	two := filepath.Join(dir, "two.txt")
	list := filepath.Join(dir, "list")
	if err := os.WriteFile(one, []byte("a\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(two, []byte("b\nc\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(list, []byte(one+"\x00"+two+"\x00"), 0o644); err != nil {
		t.Fatal(err)
	}
	out, code := runWc(t, "", "--files0-from="+list)
	if code != 0 {
		t.Fatalf("exit %d", code)
	}
	if !strings.Contains(out, "total") || !strings.Contains(out, "3 3 6 total") {
		t.Fatalf("--files0-from = %q", out)
	}
}

// writeWcFiles lays down the fixtures the file tests share.
func writeWcFiles(t *testing.T, files map[string]string) string {
	t.Helper()
	dir := t.TempDir()
	for name, body := range files {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return dir
}

// TestUmlautsAreCharactersNotBytes: -m counts characters (four for three
// umlauts plus the newline), -c counts bytes (seven). Checked against GNU wc
// under LC_ALL=C.UTF-8; in the C locale GNU counts bytes for -m too.
func TestUmlautsAreCharactersNotBytes(t *testing.T) {
	out, code := runWc(t, "äöü\n", "-m")
	if code != 0 || out != "4\n" {
		t.Fatalf("wc -m = %d, %q, want 4", code, out)
	}
	out, _ = runWc(t, "äöü\n", "-c")
	if out != "7\n" {
		t.Fatalf("wc -c = %q, want 7", out)
	}
}

// TestLinesWordsBytes: the three numbers of a plain "wc", in GNU's order.
func TestLinesWordsBytes(t *testing.T) {
	out, code := runWc(t, "a b\nc\n", "-l", "-w", "-c")
	if code != 0 || out != "2 3 6\n" {
		t.Fatalf("wc -l -w -c = %d, %q, want 2 3 6", code, out)
	}
}

// TestTotalLineForSeveralFiles: more than one file gets a "total" line, one
// file does not.
func TestTotalLineForSeveralFiles(t *testing.T) {
	dir := writeWcFiles(t, map[string]string{"a.txt": "1\n2\n3\n4\n5\n", "b.txt": "x\ny\n"})
	a, b := filepath.Join(dir, "a.txt"), filepath.Join(dir, "b.txt")

	out, code := runWc(t, "", "-l", a, b)
	if code != 0 {
		t.Fatalf("exit %d", code)
	}
	// Values only: GNU pads the columns, the numbers are what a script reads.
	lines := strings.Split(strings.TrimSuffix(out, "\n"), "\n")
	want := [][2]string{{"5", a}, {"2", b}, {"7", "total"}}
	if len(lines) != len(want) {
		t.Fatalf("got %q, want three lines", out)
	}
	for i, w := range want {
		fields := strings.Fields(lines[i])
		if len(fields) != 2 || fields[0] != w[0] || fields[1] != w[1] {
			t.Fatalf("line %d = %q, want %s and %s", i, lines[i], w[0], w[1])
		}
	}
	// One file: no total line.
	out, _ = runWc(t, "", "-l", a)
	if strings.Contains(out, "total") || strings.Count(out, "\n") != 1 {
		t.Fatalf("one file = %q, want a single line without total", out)
	}
}

// TestDefaultCountsLinesWordsBytes: no option means -l -w -c.
func TestDefaultCountsLinesWordsBytes(t *testing.T) {
	out, _ := runWc(t, "a b\nc\n")
	if out != "2 3 6\n" {
		t.Fatalf("wc = %q, want 2 3 6", out)
	}
}

// TestMissingFileIsNamedAndOne: the file that cannot be read is named, the
// other files are still counted, and the exit says something went wrong.
func TestMissingFileIsNamedAndOne(t *testing.T) {
	dir := writeWcFiles(t, map[string]string{"a.txt": "1\n"})
	var out, errOut strings.Builder
	code := Run([]string{"-l", "/nicht/vorhanden", filepath.Join(dir, "a.txt")}, strings.NewReader(""), &out, &errOut)
	if code != 1 {
		t.Fatalf("exit %d, want 1", code)
	}
	if !strings.Contains(errOut.String(), "/nicht/vorhanden") {
		t.Fatalf("stderr %q does not name the file", errOut.String())
	}
	if !strings.Contains(out.String(), "1") {
		t.Fatalf("stdout %q lost the readable file", out.String())
	}
}

// TestLongOptionsAndDashDash: the long spellings, an unknown option, and "--"
// in front of a file whose name starts with a dash.
func TestLongOptionsAndDashDash(t *testing.T) {
	if out, _ := runWc(t, "a b\n", "--lines"); out != "1\n" {
		t.Fatalf("--lines = %q", out)
	}
	if out, _ := runWc(t, "ä\n", "--chars"); out != "2\n" {
		t.Fatalf("--chars = %q", out)
	}
	if out, _ := runWc(t, "a b\n", "--words"); out != "2\n" {
		t.Fatalf("--words = %q", out)
	}
	if out, _ := runWc(t, "abc\n", "--bytes"); out != "4\n" {
		t.Fatalf("--bytes = %q", out)
	}
	var out, errOut strings.Builder
	if code := Run([]string{"--bogus"}, strings.NewReader(""), &out, &errOut); code != 1 {
		t.Fatalf("unknown long option exit %d, want 1", code)
	}
	if !strings.Contains(errOut.String(), "--bogus") {
		t.Fatalf("stderr %q does not name the option", errOut.String())
	}
	out.Reset()
	errOut.Reset()
	if code := Run([]string{"-Z"}, strings.NewReader("a\n"), &out, &errOut); code != 1 {
		t.Fatalf("unknown short option exit %d, want 1", code)
	}

	dir := writeWcFiles(t, map[string]string{"-dash.txt": "a\nb\n"})
	out.Reset()
	errOut.Reset()
	if code := Run([]string{"--", filepath.Join(dir, "-dash.txt")}, strings.NewReader(""), &out, &errOut); code != 0 {
		t.Fatalf("-- file exit %d, stderr %q", code, errOut.String())
	}
	if !strings.HasPrefix(out.String(), "2 ") {
		t.Fatalf("-- file = %q, want the count of that file", out.String())
	}
}

// TestFiles0FromWithoutAValueAndAMissingList: the two error paths of
// --files0-from, which an agent hits when the find | wc -0 pipe is wrong.
func TestFiles0FromWithoutAValueAndAMissingList(t *testing.T) {
	var out, errOut strings.Builder
	if code := Run([]string{"--files0-from"}, strings.NewReader(""), &out, &errOut); code != 1 {
		t.Fatalf("missing value exit %d, want 1", code)
	}
	out.Reset()
	errOut.Reset()
	if code := Run([]string{"--files0-from=/nicht/vorhanden"}, strings.NewReader(""), &out, &errOut); code != 1 {
		t.Fatalf("missing list exit %d, want 1", code)
	}
	if errOut.String() == "" {
		t.Fatal("no message for a list that cannot be read")
	}
}
