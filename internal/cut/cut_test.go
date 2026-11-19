package cut

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestParseList covers the cut list syntax that scripts actually use.
func TestParseList(t *testing.T) {
	cases := []struct {
		list string
		want []span
	}{
		{"1", []span{{1, 1}}},
		{"1,3", []span{{1, 1}, {3, 3}}},
		{"2-4", []span{{2, 4}}},
		{"5-", []span{{5, 1 << 30}}},
	}
	for _, c := range cases {
		got, err := parseList(c.list)
		if err != nil {
			t.Fatalf("parseList(%q): %v", c.list, err)
		}
		if fmt.Sprint(got) != fmt.Sprint(c.want) {
			t.Fatalf("parseList(%q) = %v, want %v", c.list, got, c.want)
		}
	}
	for _, bad := range []string{"", "x", "4-2", "0"} {
		if _, err := parseList(bad); err == nil {
			t.Fatalf("parseList(%q) should fail", bad)
		}
	}
}

// runCut feeds input through temporary files, because Run takes *os.File: the
// command reads real streams, not strings.
func runCut(t *testing.T, args []string, input string) (string, string, int) {
	t.Helper()
	dir := t.TempDir()
	inPath := filepath.Join(dir, "in")
	if err := os.WriteFile(inPath, []byte(input), 0o644); err != nil {
		t.Fatal(err)
	}
	in, err := os.Open(inPath)
	if err != nil {
		t.Fatal(err)
	}
	defer in.Close()
	out, err := os.Create(filepath.Join(dir, "out"))
	if err != nil {
		t.Fatal(err)
	}
	errFile, err := os.Create(filepath.Join(dir, "err"))
	if err != nil {
		t.Fatal(err)
	}
	code := Run(args, in, out, errFile)
	out.Close()
	errFile.Close()
	o, _ := os.ReadFile(filepath.Join(dir, "out"))
	e, _ := os.ReadFile(filepath.Join(dir, "err"))
	return string(o), string(e), code
}

// TestFieldsAndCharacters: the two everyday forms, checked against GNU cut.
func TestFieldsAndCharacters(t *testing.T) {
	out, _, code := runCut(t, []string{"-d:", "-f2"}, "a:b:c\n")
	if code != 0 || out != "b\n" {
		t.Fatalf("cut -d: -f2 = %d, %q", code, out)
	}
	out, _, _ = runCut(t, []string{"-c2-4"}, "abcdef\n")
	if out != "bcd\n" {
		t.Fatalf("cut -c2-4 = %q", out)
	}
	// Without -d the delimiter is a tab.
	out, _, _ = runCut(t, []string{"-f2"}, "a\tb\n")
	if out != "b\n" {
		t.Fatalf("cut -f2 with tab = %q", out)
	}
}

// TestSelectionComesOutSorted: GNU prints the fields in increasing order, so
// "cut -f3,1" gives the first field first. A script that reorders the list to
// reorder the output would be surprised by anything else.
func TestSelectionComesOutSorted(t *testing.T) {
	out, _, _ := runCut(t, []string{"-d:", "-f3,1"}, "a:b:c\n")
	if out != "a:c\n" {
		t.Fatalf("cut -f3,1 = %q, want a:c", out)
	}
	out, _, _ = runCut(t, []string{"-c2-4,1"}, "abcdef\n")
	if out != "abcd\n" {
		t.Fatalf("cut -c2-4,1 = %q, want abcd", out)
	}
	out, _, _ = runCut(t, []string{"-d:", "-f2,2"}, "a:b:c\n")
	if out != "b\n" {
		t.Fatalf("cut -f2,2 = %q, want the field once", out)
	}
}

// TestOverlappingRangesAreOne: "1-2,2-3" selects fields one to three exactly once.
func TestOverlappingRangesAreOne(t *testing.T) {
	out, _, _ := runCut(t, []string{"-d:", "-f1-2,2-3"}, "a:b:c\n")
	if out != "a:b:c\n" {
		t.Fatalf("cut -f1-2,2-3 = %q", out)
	}
}

// TestOpenEndedRanges: "-2" starts at one, "2-" ends at the last field.
func TestOpenEndedRanges(t *testing.T) {
	out, _, code := runCut(t, []string{"-d:", "-f-2"}, "a:b:c\n")
	if code != 0 || out != "a:b\n" {
		t.Fatalf("cut -f-2 = %d, %q, want a:b", code, out)
	}
	out, _, _ = runCut(t, []string{"-d:", "-f2-"}, "a:b:c\n")
	if out != "b:c\n" {
		t.Fatalf("cut -f2- = %q", out)
	}
}

// TestComplementKeepsTheRest: --complement is the field form of grep -v.
func TestComplementKeepsTheRest(t *testing.T) {
	out, _, code := runCut(t, []string{"-d:", "-f2", "--complement"}, "a:b:c\n")
	if code != 0 || out != "a:c\n" {
		t.Fatalf("cut -f2 --complement = %d, %q", code, out)
	}
	out, _, _ = runCut(t, []string{"-c2", "--complement"}, "abc\n")
	if out != "ac\n" {
		t.Fatalf("cut -c2 --complement = %q", out)
	}
}

// TestOnlyDelimitedDropsTheRest: -s is what a script uses to skip lines that do
// not look like a record at all.
func TestOnlyDelimitedDropsTheRest(t *testing.T) {
	out, _, _ := runCut(t, []string{"-d:", "-s", "-f2"}, "abc\na:b\n")
	if out != "b\n" {
		t.Fatalf("cut -s = %q, want only the delimited line", out)
	}
	// Without -s the line without a delimiter is printed whole.
	out, _, _ = runCut(t, []string{"-d:", "-f2"}, "abc\na:b\n")
	if out != "abc\nb\n" {
		t.Fatalf("cut without -s = %q", out)
	}
}

// TestBytesAndOutputDelimiter: -b counts bytes, -o replaces the join character.
func TestBytesAndOutputDelimiter(t *testing.T) {
	out, _, _ := runCut(t, []string{"-b1-2"}, "äbc\n")
	if out != "ä\n" {
		t.Fatalf("cut -b1-2 = %q, want the two bytes of ä", out)
	}
	out, _, _ = runCut(t, []string{"-d:", "-f1,3", "-o", "-"}, "a:b:c\n")
	if out != "a-c\n" {
		t.Fatalf("cut -o = %q, want a-c", out)
	}
}

// TestRefusalsFollowGNU: every wrong command line exits 1 with GNU's wording,
// not 2 with an own sentence.
func TestRefusalsFollowGNU(t *testing.T) {
	cases := []struct {
		args []string
		want string
	}{
		{[]string{"-d:xy", "-f1"}, "cut: the delimiter must be a single character\n"},
		{[]string{}, "cut: you must specify a list of bytes, characters, or fields\n"},
		{[]string{"-b1", "-c1"}, "cut: only one list may be specified\n"},
		{[]string{"--bogus"}, "cut: unrecognized option '--bogus'\n"},
	}
	for _, c := range cases {
		_, errOut, code := runCut(t, c.args, "a:b\n")
		if code != 1 {
			t.Errorf("%v: exit %d, want 1", c.args, code)
			continue
		}
		if !strings.HasPrefix(errOut, c.want) {
			t.Errorf("%v: stderr %q, want it to start with %q", c.args, errOut, c.want)
		}
		if !strings.Contains(errOut, "Try 'cut --help'") {
			t.Errorf("%v: stderr %q does not point at --help", c.args, errOut)
		}
	}
}

// TestFileArgumentAndBadList: a file on the command line, and a list that is not
// a number.
func TestFileArgumentAndBadList(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "f.txt")
	if err := os.WriteFile(path, []byte("a:b\nc:d\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	out, _, code := runCut(t, []string{"-d:", "-f2", path}, "")
	if code != 0 || out != "b\nd\n" {
		t.Fatalf("cut file = %d, %q", code, out)
	}
	_, errOut, code := runCut(t, []string{"-f", "x"}, "a\n")
	if code == 0 || errOut == "" {
		t.Fatalf("cut -f x = %d, %q, want a refusal", code, errOut)
	}
}
