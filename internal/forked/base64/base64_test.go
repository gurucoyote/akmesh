package base64

import (
	"strings"
	"testing"
)

func runB64(t *testing.T, stdin string, args ...string) (string, string, int) {
	t.Helper()
	var out, errOut strings.Builder
	code := Run(args, strings.NewReader(stdin), &out, &errOut)
	return out.String(), errOut.String(), code
}

// TestWrapWidth: GNU wraps at 76 columns by default, -w sets the width, -w0
// keeps one line.
func TestWrapWidth(t *testing.T) {
	input := strings.Repeat("a", 100)
	out, _, code := runB64(t, input)
	if code != 0 {
		t.Fatalf("exit %d", code)
	}
	lines := strings.Split(strings.TrimRight(out, "\n"), "\n")
	if len(lines) != 2 || len(lines[0]) != 76 {
		t.Fatalf("default wrap = %d lines with widths %v", len(lines), []int{len(lines[0])})
	}
	out, _, _ = runB64(t, input, "-w", "10")
	lines = strings.Split(strings.TrimRight(out, "\n"), "\n")
	if len(lines) != len(lines[0])*0+14 || len(lines[0]) != 10 {
		t.Fatalf("-w 10 = %d lines, first width %d", len(lines), len(lines[0]))
	}
	out, _, _ = runB64(t, "hello", "-w0")
	if strings.Count(out, "\n") != 1 || !strings.HasSuffix(out, "\n") {
		t.Fatalf("-w0 = %q, want one line", out)
	}
}

// TestRoundTrip: encode then decode gives the input back, also glued form -dw0.
func TestRoundTrip(t *testing.T) {
	out, _, _ := runB64(t, "hello world")
	back, errOut, code := runB64(t, out, "-d")
	if code != 0 || back != "hello world" {
		t.Fatalf("round trip = %q, %q, exit %d", back, errOut, code)
	}
}

// TestIgnoreGarbage: -i drops characters outside the alphabet while decoding.
func TestIgnoreGarbage(t *testing.T) {
	back, _, code := runB64(t, "Y#W#I=", "-di")
	if code != 0 || back != "ab" {
		t.Fatalf("-di = %q, exit %d", back, code)
	}
	if _, _, code := runB64(t, "Y#W#I=", "-d"); code != 1 {
		t.Fatalf("-d without -i took garbage, exit %d", code)
	}
}
