package odump

import (
	"strings"
	"testing"
)

// runO calls Run as od with in as stdin and returns stdout and the status.
func runO(t *testing.T, in string, args ...string) (string, int) {
	t.Helper()
	var out, errb strings.Builder
	code := Run("od", args, strings.NewReader(in), &out, &errb)
	return out.String(), code
}

// TestShorthands keeps the historical spellings GNU documents: -c, -b, -x, -o,
// -d, -s and -a are one -t type each, and they cluster (measured 24.11.2026,
// acmesh-4b7).
func TestShorthands(t *testing.T) {
	cases := []struct {
		args []string
		want string
	}{
		{[]string{"-c"}, "0000000   a  \\n\n0000002\n"},
		{[]string{"-b"}, "0000000 141 012\n0000002\n"},
		{[]string{"-x"}, "0000000 0a61\n0000002\n"},
		{[]string{"-t", "u1"}, "0000000  97  10\n0000002\n"},
		{[]string{"-A", "x", "-tc"}, "000000   a  \\n\n000002\n"},
		{[]string{"-An", "-tc"}, "   a  \\n\n"},
		{[]string{"-N", "1", "-c"}, "0000000   a\n0000001\n"},
	}
	for _, c := range cases {
		got, code := runO(t, "a\n", c.args...)
		if code != 0 {
			t.Fatalf("%v: code %d", c.args, code)
		}
		if got != c.want {
			t.Fatalf("%v: got %q, want %q", c.args, got, c.want)
		}
	}
}

// TestTwoTypesInOneT: "-t x1o1" prints one line per type, as GNU.
func TestTwoTypesInOneT(t *testing.T) {
	got, _ := runO(t, "a\n", "-t", "x1o1")
	want := "0000000  61  0a\n        141 012\n0000002\n"
	if got != want {
		t.Fatalf("got %q, want %q", got, want)
	}
}
