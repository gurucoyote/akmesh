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

// GNU coreutils in the container (LC_ALL=C.UTF-8) is the source of every
// expected value below. The classes are ASCII there: "[:upper:]" leaves ä alone.

// want runs tr and returns stdout only, failing the test when it does not exit 0.
func want(t *testing.T, stdin string, args ...string) string {
	t.Helper()
	out, errOut, code := runTr(t, stdin, args...)
	if code != 0 {
		t.Fatalf("%v = %d, stderr %q", args, code, errOut)
	}
	return out
}

// TestClassesAreASCII: the class is a-z/A-Z, so a German umlaut stays as it is.
// Using unicode.IsUpper gave Ä and drifted from GNU.
func TestClassesAreASCII(t *testing.T) {
	if out := want(t, "Hello ab12c äöü\n", "[:lower:]", "[:upper:]"); out != "HELLO AB12C äöü\n" {
		t.Fatalf("[:lower:] [:upper:] = %q", out)
	}
	if out := want(t, "Hello ab12c äöü\n", "-d", "[:lower:]"); out != "H 12 äöü\n" {
		t.Fatalf("-d [:lower:] = %q", out)
	}
	if out := want(t, "abc123", "-cd", "[:digit:]"); out != "123" {
		t.Fatalf("-cd [:digit:] = %q", out)
	}
}

// TestDeleteAndComplement: -d drops the set, -C turns it around.
func TestDeleteAndComplement(t *testing.T) {
	if out := want(t, "Hello ab12c\n", "-d", "0-9"); out != "Hello abc\n" {
		t.Fatalf("-d 0-9 = %q", out)
	}
	// -C replaces everything that is not in the set, the newline included.
	if out := want(t, "abc", "-C", "a", "x"); out != "axx" {
		t.Fatalf("-C a x = %q", out)
	}
}

// TestSqueezeWithTwoSets: -s ab collapses runs of a and b to one character.
func TestSqueezeWithTwoSets(t *testing.T) {
	if out := want(t, "aaabbb", "-s", "ab"); out != "ab" {
		t.Fatalf("-s ab = %q", out)
	}
	// GNU keeps the squeezed blank at the end: the newline is a space too.
	if out := want(t, "a   b  c\n", "-s", "[:space:]", " "); out != "a b c " {
		t.Fatalf("-s with a class = %q", out)
	}
}

// TestUnequalSetsRepeatTheLast: when set2 ends, its last character repeats,
// which is how "tr a-c x" replaces a, b and c.
func TestUnequalSetsRepeatTheLast(t *testing.T) {
	if out := want(t, "abc", "a-c", "x"); out != "xxx" {
		t.Fatalf("tr a-c x = %q", out)
	}
	if out := want(t, "abcd", "a-d", "xy"); out != "xyyy" {
		t.Fatalf("tr a-d xy = %q", out)
	}
}

// TestEscapesInSets: the backslash spellings a shell script writes.
func TestEscapesInSets(t *testing.T) {
	if out := want(t, "a\nb\n", `\n`, " "); out != "a b " {
		t.Fatalf(`tr '\n' ' ' = %q`, out)
	}
	if out := want(t, "a\tb", `\t`, "_"); out != "a_b" {
		t.Fatalf(`tr '\t' '_' = %q`, out)
	}
}

// TestNonASCIITranslatesAsOneCharacter: tr works on characters here, so one
// umlaut is one unit. GNU in C.UTF-8 works on bytes and produces garbage for the
// same input; that difference is documented in docs/measurements.md.
func TestNonASCIITranslatesAsOneCharacter(t *testing.T) {
	if out := want(t, "äöü\n", "äöü", "aou"); out != "aou\n" {
		t.Fatalf("tr äöü aou = %q", out)
	}
}

// TestOperandErrorsFollowGNU: exit 1 with the same sentence and the pointer to
// --help, which the battery never saw.
func TestOperandErrorsFollowGNU(t *testing.T) {
	for _, args := range [][]string{{}, {"a"}, {"-d", "a", "b"}} {
		out, errOut, code := runTr(t, "abc\n", args...)
		if code != 1 {
			t.Errorf("%v: exit %d, want 1", args, code)
			continue
		}
		if out != "" {
			t.Errorf("%v: printed %q, want nothing", args, out)
		}
		if !strings.Contains(errOut, "Try 'tr --help'") {
			t.Errorf("%v: stderr %q has no --help hint", args, errOut)
		}
	}
}

// TestSqueezeAloneAndClassErrors: "tr -s SET" only squeezes, and the class
// mistakes GNU refuses are refused here too.
func TestSqueezeAloneAndClassErrors(t *testing.T) {
	if out := want(t, "a--b", "-s", "-"); out != "a-b" {
		t.Fatalf("tr -s - = %q", out)
	}
	// A class in SET2 that is not upper/lower is an error.
	if _, errOut, code := runTr(t, "abc", "a", "[:digit:]"); code != 1 || errOut == "" {
		t.Fatalf("class in SET2 = %d, %q", code, errOut)
	}
	// [:lower:] against [:lower:] is the misalignment GNU names.
	if _, errOut, code := runTr(t, "abc", "[:lower:]", "[:lower:]"); code != 1 || !strings.Contains(errOut, "misaligned") {
		t.Fatalf("misaligned = %d, %q", code, errOut)
	}
	// An escape that does not exist.
	if _, errOut, code := runTr(t, "abc", `\q`, "x"); code != 1 || !strings.Contains(errOut, "unknown escape") {
		t.Fatalf("bad escape = %d, %q", code, errOut)
	}
	// Three operands.
	if _, errOut, code := runTr(t, "abc", "a", "b", "c"); code != 1 || !strings.Contains(errOut, "extra operand") {
		t.Fatalf("three operands = %d, %q", code, errOut)
	}
}

// TestComplementWithAClassTranslates: -c on a class translates the rest, not the
// literal text of the class name.
func TestComplementWithAClassTranslates(t *testing.T) {
	if out := want(t, "ab12", "-c", "[:digit:]", "x"); out != "xx12" {
		t.Fatalf("tr -c [:digit:] x = %q", out)
	}
}
