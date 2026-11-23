package uniq

import (
	"bytes"
	"strings"
	"testing"
)

// run calls Run with in as stdin and returns stdout, stderr and the status.
func runU(t *testing.T, in string, args ...string) (string, string, int) {
	t.Helper()
	var out, errb bytes.Buffer
	code := Run(args, strings.NewReader(in), &out, &errb)
	return out.String(), errb.String(), code
}

// TestGNUBehaviour holds what GNU coreutils 9.7 printed in the measurement of
// acmesh-6bq: the -c shape, the skip flags, -z and the status on a bad option.
func TestGNUBehaviour(t *testing.T) {
	const words = "b\nA\na\nc\n"
	cases := []struct {
		name string
		in   string
		args []string
		want string
	}{
		{"plain", words, nil, words},
		{"count width", "a\na\nb\n", []string{"-c"}, "      2 a\n      1 b\n"},
		{"count past nine", strings.Repeat("x\n", 12), []string{"-c"}, "     12 x\n"},
		{"repeated", "a\na\nb\n", []string{"-d"}, "a\n"},
		{"unique", "a\na\nb\n", []string{"-u"}, "b\n"},
		{"both", "a\na\nb\n", []string{"-d", "-u"}, ""},
		{"ignore case", "a\nA\nb\n", []string{"-i"}, "a\nb\n"},
		{"skip fields", "x one\ny one\nz two\n", []string{"-f", "1"}, "x one\nz two\n"},
		{"skip chars", "aab\nabc\n", []string{"-s", "2"}, "aab\nabc\n"},
		{"check chars", "abcdef\nabcxyz\n", []string{"-w", "3"}, "abcdef\n"},
		{"unterminated last", "a\na\nb", nil, "a\nb\n"},
		{"glued flags", "a\na\nb\n", []string{"-ci"}, "      2 a\n      1 b\n"},
	}
	for _, c := range cases {
		got, errOut, code := runU(t, c.in, c.args...)
		if code != 0 || errOut != "" {
			t.Fatalf("%s: code %d, err %q", c.name, code, errOut)
		}
		if got != c.want {
			t.Fatalf("%s: got %q, want %q", c.name, got, c.want)
		}
	}
}

// TestZeroTerminated keeps NUL separated records apart, the pair to find -print0.
func TestZeroTerminated(t *testing.T) {
	got, _, code := runU(t, "a\x00a\x00b\x00", "-z")
	if code != 0 || got != "a\x00b\x00" {
		t.Fatalf("code %d, got %q", code, got)
	}
}

// TestBadOptionIsStatusTwo: GNU refuses an unknown option with status 2, not
// with a silent best guess.
func TestBadOptionIsStatusTwo(t *testing.T) {
	_, errOut, code := runU(t, "", "-Q")
	if code != 2 || !strings.Contains(errOut, "invalid option") {
		t.Fatalf("code %d, err %q", code, errOut)
	}
}
