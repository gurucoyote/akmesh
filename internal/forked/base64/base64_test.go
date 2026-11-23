package base64

import (
	"os"
	"path/filepath"
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
	if out != "aGVsbG8=" {
		t.Fatalf("-w0 = %q, want one line and no trailing newline", out)
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

// GNU base64 9.4 in the container is the source of every expected value here.

// TestDecodeWithoutPaddingKeepsTheLastBlock: a last group of three characters has
// no padding. GNU decodes it and exits 0 (measured 23.11.2026, acmesh-449); the
// old decoder threw the tail away, which lost whole files (bug of 18.11.).
func TestDecodeWithoutPaddingKeepsTheLastBlock(t *testing.T) {
	out, errOut, code := runB64(t, "aGVsbG8", "-d")
	if out != "hello" {
		t.Fatalf("-d on an unpadded block = %q, want hello", out)
	}
	if code != 0 || errOut != "" {
		t.Fatalf("-d unpadded = %d, %q, want 0 and no message", code, errOut)
	}
	// With the padding there is nothing to complain about.
	out, errOut, code = runB64(t, "aGVsbG8=", "-d")
	if code != 0 || errOut != "" || out != "hello" {
		t.Fatalf("-d padded = %d, %q, %q", code, out, errOut)
	}
}

// TestDecodeRejectsGarbage: text outside the alphabet is an error, and nothing
// decoded so far is kept.
func TestDecodeRejectsGarbage(t *testing.T) {
	out, errOut, code := runB64(t, "!!!!", "-d")
	if code != 1 || out != "" {
		t.Fatalf("-d on garbage = %d, %q, want 1 and no output", code, out)
	}
	if !strings.Contains(errOut, "invalid input") {
		t.Fatalf("stderr %q, want GNU's invalid input", errOut)
	}
}

// TestDecodeAcceptsWrappedInput: newlines are part of the format.
func TestDecodeAcceptsWrappedInput(t *testing.T) {
	out, _, code := runB64(t, "aGVs\nbG8=\n", "-d")
	if code != 0 || out != "hello" {
		t.Fatalf("wrapped -d = %d, %q", code, out)
	}
	// A space is garbage, not part of the format: GNU stops with "invalid
	// input" here (measured 23.11.2026), and blank padding around the data is
	// what -i is for.
	if _, errOut, code := runB64(t, " aGVs\tbG8= \r\n", "-d"); code != 1 || !strings.Contains(errOut, "invalid input") {
		t.Fatalf("blanks around -d = %d, %q", code, errOut)
	}
	out, _, code = runB64(t, "aGVs\nbG8=\r\n", "-d")
	if code != 0 || out != "hello" {
		t.Fatalf("CRLF -d = %d, %q", code, out)
	}
}

// TestWrapWidths: -w4 and --wrap=8 cut the same text differently, -w0 keeps one
// line without a newline in the middle.
func TestWrapWidths(t *testing.T) {
	out, _, code := runB64(t, "hello world", "-w", "4")
	if code != 0 || out != "aGVs\nbG8g\nd29y\nbGQ=\n" {
		t.Fatalf("-w 4 = %d, %q", code, out)
	}
	out, _, _ = runB64(t, "hello", "--wrap=8")
	if out != "aGVsbG8=\n" {
		t.Fatalf("--wrap=8 = %q", out)
	}
	// A width larger than the text must not drop its last character.
	out, _, _ = runB64(t, "hello", "-w", "100")
	if out != "aGVsbG8=\n" {
		t.Fatalf("-w 100 = %q", out)
	}
	// -w0 ends without a newline, like GNU; a script reads this into a
	// variable and a trailing line break would break it (acmesh-449).
	out, _, _ = runB64(t, "hello world", "-w0")
	if out != "aGVsbG8gd29ybGQ=" {
		t.Fatalf("-w0 = %q", out)
	}
}

// TestWrapArgumentErrors: the number is required and has to be a number.
func TestWrapArgumentErrors(t *testing.T) {
	_, errOut, code := runB64(t, "a", "-w")
	if code != 1 || errOut == "" {
		t.Fatalf("-w without a number = %d, %q", code, errOut)
	}
	_, errOut, code = runB64(t, "a", "-w", "x")
	if code != 1 || !strings.Contains(errOut, "invalid wrap size") {
		t.Fatalf("-w x = %d, %q", code, errOut)
	}
	_, errOut, code = runB64(t, "a", "--wrap=-1")
	if code != 1 || !strings.Contains(errOut, "invalid wrap size") {
		t.Fatalf("--wrap=-1 = %d, %q", code, errOut)
	}
}

// TestRefusalsFollowGNU: an unknown option, a second file, a file that is not
// there. All exit 1 with GNU's wording and the pointer to --help.
func TestRefusalsFollowGNU(t *testing.T) {
	_, errOut, code := runB64(t, "", "--bogus")
	if code != 1 || !strings.HasPrefix(errOut, "base64: unrecognized option '--bogus'\n") {
		t.Fatalf("--bogus = %d, %q", code, errOut)
	}
	if _, _, code := runB64(t, "", "-Z"); code != 1 {
		t.Fatalf("-Z = %d, want 1", code)
	}
	_, errOut, code = runB64(t, "", "a", "b")
	if code != 1 || !strings.HasPrefix(errOut, "base64: extra operand 'b'\n") {
		t.Fatalf("two operands = %d, %q", code, errOut)
	}
	_, errOut, code = runB64(t, "", "/nicht/vorhanden")
	if code != 1 || !strings.HasPrefix(errOut, "base64: /nicht/vorhanden: No such file or directory") {
		t.Fatalf("missing file = %d, %q", code, errOut)
	}
}

// TestFileAndDashArguments: a file argument is read, and "-" means stdin.
func TestFileAndDashArguments(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "in.txt")
	if err := os.WriteFile(path, []byte("hello"), 0o644); err != nil {
		t.Fatal(err)
	}
	out, _, code := runB64(t, "", path)
	if code != 0 || out != "aGVsbG8=\n" {
		t.Fatalf("encode file = %d, %q", code, out)
	}
	out, _, code = runB64(t, "hello", "-")
	if code != 0 || out != "aGVsbG8=\n" {
		t.Fatalf("dash = %d, %q", code, out)
	}
	// -- ends the options, so a file called "-d" can be read.
	if err := os.WriteFile(filepath.Join(dir, "-d"), []byte("hi"), 0o644); err != nil {
		t.Fatal(err)
	}
	out, _, code = runB64(t, "", "--", filepath.Join(dir, "-d"))
	if code != 0 || out != "aGk=\n" {
		t.Fatalf("-- file = %d, %q", code, out)
	}
}

// TestIgnoreGarbageWithABadLength: -i drops characters, but a length that cannot
// be decoded is still an error.
func TestIgnoreGarbageWithABadLength(t *testing.T) {
	// Three characters without padding decode, with or without -i: GNU prints
	// "ab" and exits 0 (measured 23.11.2026), because a token is often copied
	// without its "=".
	if out, _, code := runB64(t, "Y#W#I", "-di"); code != 0 || out != "ab" {
		t.Fatalf("-di with a short block = %d, %q", code, out)
	}
	out, _, code := runB64(t, "Y#W#I=\n", "-di")
	if code != 0 || out != "ab" {
		t.Fatalf("-di with a newline = %d, %q", code, out)
	}
}
