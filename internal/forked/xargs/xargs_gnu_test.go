package xargs

import (
	"strings"
	"testing"
)

// runX calls Run with text on stdin and returns stdout, stderr and the status.
func runX(t *testing.T, text string, args ...string) (string, string, int) {
	t.Helper()
	var out, errb strings.Builder
	code := Run(args, strings.NewReader(text), &out, &errb)
	return out.String(), errb.String(), code
}

// TestQuoteParsing: GNU's default mode keeps a quoted blank inside one item, so
// a file name with a space arrives as one argument.
func TestQuoteParsing(t *testing.T) {
	cases := []struct{ in, want string }{
		{"'a b'\nc\n", "a b\nc\n"},
		{`"a b" c`, "a b\nc\n"},
		{"a b\nc\n", "a\nb\nc\n"},
	}
	for _, c := range cases {
		got, errOut, code := runX(t, c.in, "-n1", "echo")
		if code != 0 || errOut != "" {
			t.Fatalf("%q: code %d, err %q", c.in, code, errOut)
		}
		if got != c.want {
			t.Fatalf("%q: got %q, want %q", c.in, got, c.want)
		}
	}
}

// TestOptionsEndAtCommand: GNU treats everything after the command word as the
// command line, so -c must reach sh instead of failing as an xargs option.
func TestOptionsEndAtCommand(t *testing.T) {
	got, errOut, code := runX(t, "a\nb\n", "-n1", "sh", "-c", "echo $0")
	if code != 0 || errOut != "" {
		t.Fatalf("code %d, err %q", code, errOut)
	}
	if got != "a\nb\n" {
		t.Fatalf("got %q", got)
	}
}

// TestExitStatus: a failing child gives 123, the max 255 gives 124, as GNU.
func TestExitStatusLikeGNU(t *testing.T) {
	if _, _, code := runX(t, "a\n", "-n1", "sh", "-c", "exit 5"); code != 123 {
		t.Fatalf("exit 5: status %d, want 123", code)
	}
	if _, _, code := runX(t, "a\n", "-n1", "sh", "-c", "exit 255"); code != 124 {
		t.Fatalf("exit 255: status %d, want 124", code)
	}
}

// TestEndString: -E stops before the marker line.
func TestEndString(t *testing.T) {
	got, _, code := runX(t, "a\nSTOP\nb\n", "-E", "STOP", "-n1", "echo")
	if code != 0 || got != "a\n" {
		t.Fatalf("code %d, got %q", code, got)
	}
}

// TestMaxLines: -L1 gives one command per line, all words of the line together.
func TestMaxLines(t *testing.T) {
	got, _, code := runX(t, "a b\nc\n", "-L1", "echo", "zeile")
	if code != 0 || got != "zeile a b\nzeile c\n" {
		t.Fatalf("code %d, got %q", code, got)
	}
}
