package odump

import (
	"bytes"
	"os"
	"strings"
	"testing"
)

// runOd runs od on a string and returns its output.
func runOd(t *testing.T, data string, args ...string) string {
	t.Helper()
	var out, errb bytes.Buffer
	if code := Run("od", args, strings.NewReader(data), &out, &errb); code != 0 {
		t.Fatalf("exit %d: %s", code, errb.String())
	}
	return out.String()
}

// TestHexOneByteColumns is the form agents type: od -An -tx1. The expected
// bytes are the plain ASCII of "hello\n".
func TestHexOneByteColumns(t *testing.T) {
	got := runOd(t, "hello\n", "-An", "-tx1")
	want := " 68 65 6c 6c 6f 0a\n"
	if got != want {
		t.Fatalf("got %q, want %q", got, want)
	}
}

// TestDefaultIsOctalWords: od without arguments prints two-byte octal words and
// an octal address, as it has since Version 7.
func TestDefaultIsOctalWords(t *testing.T) {
	got := runOd(t, "hello\n")
	if !strings.HasPrefix(got, "0000000") {
		t.Fatalf("no octal address: %q", got)
	}
	// "he" little endian is 0o62550; the expected line below was checked
	// against GNU od 9.4 on the same bytes.
	if !strings.Contains(got, "062550 066154 005157") {
		t.Fatalf("the word line does not match od: %q", got)
	}
	if !strings.HasSuffix(got, "0000006\n") {
		t.Fatalf("final address missing: %q", got)
	}
}

// TestCharTypes: -tc prints characters, escapes the ones that have no glyph, and
// -ta prints the names.
func TestCharTypes(t *testing.T) {
	got := runOd(t, "a\tb\n", "-An", "-tc")
	if !strings.Contains(got, "a") || !strings.Contains(got, `\t`) || !strings.Contains(got, `\n`) {
		t.Fatalf("-tc output %q", got)
	}
	got = runOd(t, "a\tb\n", "-An", "-ta")
	for _, want := range []string{"a", "ht", "b", "nl"} {
		if !strings.Contains(got, want) {
			t.Fatalf("-ta output %q misses %q", got, want)
		}
	}
}

// TestSkipAndLimit: -j and -N work on the byte stream, not on the decoded words.
func TestSkipAndLimit(t *testing.T) {
	got := runOd(t, "abcdef", "-An", "-tx1", "-j", "2", "-N", "2")
	if got != " 63 64\n" {
		t.Fatalf("got %q, want %q", got, " 63 64\n")
	}
}

// TestSquashRepeats: a block that repeats gives one "*", and -v prints it again.
func TestSquashRepeats(t *testing.T) {
	data := strings.Repeat("\x00", 64)
	got := runOd(t, data, "-An", "-tx1")
	if !strings.Contains(got, "*") {
		t.Fatalf("repeated blocks are not squashed: %q", got)
	}
	got = runOd(t, data, "-An", "-tx1", "-v")
	if strings.Contains(got, "*") {
		t.Fatalf("-v still squashes: %q", got)
	}
}

// TestFromFile reads a file, so that the common "od -An -tx1 datei" works.
func TestFromFile(t *testing.T) {
	path := t.TempDir() + "/b.bin"
	if err := os.WriteFile(path, []byte{0xde, 0xad}, 0o600); err != nil {
		t.Fatal(err)
	}
	var out, errb bytes.Buffer
	if code := Run("od", []string{"-An", "-tx1", path}, strings.NewReader(""), &out, &errb); code != 0 {
		t.Fatalf("exit %d: %s", code, errb.String())
	}
	if strings.TrimSpace(out.String()) != "de ad" {
		t.Fatalf("got %q", out.String())
	}
}
