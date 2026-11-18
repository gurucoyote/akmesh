package tr

import (
	"strings"
	"testing"
)

func runTr(t *testing.T, stdin string, args ...string) (string, string, int) {
	t.Helper()
	var out, errOut strings.Builder
	code := Run(args, strings.NewReader(stdin), &out, &errOut)
	return out.String(), errOut.String(), code
}

// TestRanges: a-z is a range, not the three characters a, - and z.
func TestRanges(t *testing.T) {
	out, _, code := runTr(t, "abc xyz\n", "a-z", "A-Z")
	if code != 0 || out != "ABC XYZ\n" {
		t.Fatalf("a-z A-Z = %q, exit %d", out, code)
	}
}

// TestSqueeze: -s collapses repeats, and a single SET is enough for that.
func TestSqueeze(t *testing.T) {
	out, _, _ := runTr(t, "aaabbb\n", "-s", "a")
	if out != "abbb\n" {
		t.Fatalf("-s a = %q", out)
	}
	out, _, _ = runTr(t, "a   b\n", "-s", " ")
	if out != "a b\n" {
		t.Fatalf("-s space = %q", out)
	}
}

// TestComplement: -cd keeps only the characters of SET1, which is how an agent
// filters a stream down to its digits.
func TestComplement(t *testing.T) {
	out, _, _ := runTr(t, "ab12cd34\n", "-cd", "0-9")
	if out != "1234" {
		t.Fatalf("-cd 0-9 = %q", out)
	}
}

// TestComplementTranslation: SET2 is extended with its last character, as GNU
// does it.
func TestComplementTranslation(t *testing.T) {
	out, _, _ := runTr(t, "aaabbb\nccc\n", "-c", "a", "AB")
	if out != "aaaBBBBBBBB" {
		t.Fatalf("-c a AB = %q", out)
	}
}

// TestErrorMessage: the two strings GNU demands are named like GNU names them.
func TestErrorMessage(t *testing.T) {
	if _, _, code := runTr(t, "", "a"); code != 1 {
		t.Fatalf("tr a exit %d, want 1", code)
	}
	if _, errOut, _ := runTr(t, "", "-ds", "a"); !strings.Contains(errOut, "Two strings must be given") {
		t.Fatalf("-ds message %q", errOut)
	}
}
