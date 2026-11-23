package tr

import (
	"strings"
	"testing"
)

// runT calls Run with in as stdin and returns stdout and the status.
func runT(t *testing.T, in string, args ...string) (string, int) {
	t.Helper()
	var out, errb strings.Builder
	code := Run(args, strings.NewReader(in), &out, &errb)
	return out.String(), code
}

// TestOctalEscapes: GNU reads \0, \12 and \101 as octal, so a NUL byte in the
// input can be replaced or deleted (measured 24.11.2026, acmesh-bly).
func TestOctalEscapes(t *testing.T) {
	if got, code := runT(t, "x\x00y\n", `\0`, "@"); code != 0 || got != "x@y\n" {
		t.Fatalf(`\0: code %d, got %q`, code, got)
	}
	if got, _ := runT(t, "x\x00y\n", "-d", `\0`); got != "xy\n" {
		t.Fatalf("delete \\0: got %q", got)
	}
	if got, _ := runT(t, "A\n", `\101`, "Z"); got != "Z\n" {
		t.Fatalf(`\101: got %q`, got)
	}
}

// TestClassInsideSet: a class counts wherever it appears, not only as the whole
// set, so "[:upper:][:digit:]" maps both A and 1.
func TestClassInsideSet(t *testing.T) {
	got, code := runT(t, "A1b\n", "[:upper:][:digit:]", "a")
	if code != 0 || got != "aab\n" {
		t.Fatalf("code %d, got %q", code, got)
	}
}
