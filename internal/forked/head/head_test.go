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
	if _, _, code := runHeadTest(t, []string{"-n", "2", "-c", "2"}, "a\n"); code != 2 {
		t.Fatalf("combined -n and -c exit %d, want 2", code)
	}
}
