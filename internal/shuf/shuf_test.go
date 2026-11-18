package shufcmd

import (
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
)

// runShuf executes the command the way the binary does.
func runShuf(t *testing.T, args []string, stdin string) (int, string) {
	t.Helper()
	dir := t.TempDir()
	inPath := filepath.Join(dir, "in")
	if err := os.WriteFile(inPath, []byte(stdin), 0o644); err != nil {
		t.Fatal(err)
	}
	inF, err := os.Open(inPath)
	if err != nil {
		t.Fatal(err)
	}
	defer inF.Close()
	outF, err := os.Create(filepath.Join(dir, "out"))
	if err != nil {
		t.Fatal(err)
	}
	defer outF.Close()
	errF, err := os.Create(filepath.Join(dir, "err"))
	if err != nil {
		t.Fatal(err)
	}
	defer errF.Close()
	code := Run(args, inF, outF, errF)
	out, _ := os.ReadFile(outF.Name())
	return code, string(out)
}

func lines(out string) []string {
	if out == "" {
		return nil
	}
	return strings.Split(strings.TrimSuffix(out, "\n"), "\n")
}

// TestPermutationIsComplete is the property that matters: shuf must lose
// nothing. Comparing against a fixed order is impossible by design, so the test
// checks the multiset.
func TestPermutationIsComplete(t *testing.T) {
	const in = "one\ntwo\nthree\nfour\nfive\n"
	code, out := runShuf(t, nil, in)
	if code != 0 {
		t.Fatalf("exit %d", code)
	}
	got := lines(out)
	want := []string{"one", "two", "three", "four", "five"}
	sort.Strings(got)
	sort.Strings(want)
	if strings.Join(got, ",") != strings.Join(want, ",") {
		t.Fatalf("got %q, want a permutation of %q", got, want)
	}
}

func TestHeadCountAndRepeat(t *testing.T) {
	code, out := runShuf(t, []string{"-n", "2"}, "a\nb\nc\nd\n")
	if code != 0 {
		t.Fatalf("exit %d", code)
	}
	if n := len(lines(out)); n != 2 {
		t.Fatalf("-n 2 printed %d lines: %q", n, out)
	}
	code, out = runShuf(t, []string{"-r", "-n", "6", "-e", "a", "b"}, "")
	if code != 0 {
		t.Fatalf("-r exit %d", code)
	}
	got := lines(out)
	if len(got) != 6 {
		t.Fatalf("-r -n 6 printed %d lines: %q", len(got), out)
	}
	for _, l := range got {
		if l != "a" && l != "b" {
			t.Fatalf("-r invented a line %q", l)
		}
	}
}

func TestEchoAndInputRange(t *testing.T) {
	code, out := runShuf(t, []string{"-e", "x", "y", "z"}, "")
	if code != 0 {
		t.Fatalf("-e exit %d", code)
	}
	if n := len(lines(out)); n != 3 {
		t.Fatalf("-e printed %d lines: %q", n, out)
	}
	code, out = runShuf(t, []string{"-i", "3-5"}, "")
	if code != 0 {
		t.Fatalf("-i exit %d", code)
	}
	got := lines(out)
	sort.Strings(got)
	if strings.Join(got, ",") != "3,4,5" {
		t.Fatalf("-i 3-5 printed %q", got)
	}
}

func TestZeroTerminated(t *testing.T) {
	code, out := runShuf(t, []string{"-z"}, "a\x00b\x00")
	if code != 0 {
		t.Fatalf("exit %d", code)
	}
	if out != "a\x00b\x00" && out != "b\x00a\x00" {
		t.Fatalf("-z did not keep NUL separators: %q", out)
	}
}

// TestUsageErrorsExit1 pins GNU's exit code for shuf: unlike sort, GNU shuf
// reports usage errors with 1 (measured against GNU shuf in the container).
func TestUsageErrorsExit1(t *testing.T) {
	for _, args := range [][]string{
		{"--bogus"}, {"-i", "5-3"}, {"-e", "-i", "1-2"}, {"-n", "-1"},
	} {
		if code, _ := runShuf(t, args, "a\nb\n"); code != 1 {
			t.Errorf("%v: exit %d, want 1", args, code)
		}
	}
}
