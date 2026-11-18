package tools

import (
	"os"
	"path/filepath"
	"testing"
)

// TestChecksumVectors pins the digests against the values the system tools
// print for the same input. A hash that is only self-consistent would still
// break every recipe that compares our output with sha256sum output, so the
// claim is "equals the published digest", not "returns something".
func TestChecksumVectors(t *testing.T) {
	cases := []struct {
		kind   string
		digest string
		input  string
	}{
		{"md5", "d41d8cd98f00b204e9800998ecf8427e", ""},
		{"md5", "aee97cb3ad288ef0add6c6b5b5fae48a", "hallo\n"},
		{"sha1", "a9993e364706816aba3e25717850c26c9cd0d89d", "abc"},
		{"sha256", "622cb3371c1a08096eaac564fb59acccda1fcdbe13a9dd10b486e6463c8c2525", "hallo\n"},
		{"sha512", "ddaf35a193617abacc417349ae20413112e6fa4e89a97ea20a9eeee64b55d39a2192992a274fc1a836ba3c23a3feebbd454d4423643ce80e2a9ac94fa54ca49f", "abc"},
	}
	for _, c := range cases {
		// The separator is two spaces and the name is "-" for stdin: that exact
		// shape is what "sha256sum -c" parses later.
		want := c.digest + "  -\n"
		got, code := captureIn(t, runSum(c.kind), nil, c.input)
		if code != 0 {
			t.Fatalf("%ssum(%q) exit %d", c.kind, c.input, code)
		}
		if got != want {
			t.Fatalf("%ssum(%q) = %q, want %q", c.kind, c.input, got, want)
		}
	}
}

// TestChecksumFiles checks the operand path: the file is read, the name is
// echoed as the caller wrote it, and one unreadable file does not swallow the
// result for the others.
func TestChecksumFiles(t *testing.T) {
	dir := t.TempDir()
	good := filepath.Join(dir, "a.txt")
	if err := os.WriteFile(good, []byte("hallo\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	missing := filepath.Join(dir, "fehlt.txt")

	got, code := captureIn(t, runSum("sha256"), []string{good, missing}, "")
	if code != 1 {
		t.Fatalf("one missing file must exit 1, got %d (%q)", code, got)
	}
	want := "622cb3371c1a08096eaac564fb59acccda1fcdbe13a9dd10b486e6463c8c2525  " + good + "\n"
	if got != want {
		t.Fatalf("output %q, want %q", got, want)
	}
}

// TestChecksumRefusesUnsupported pins the refusal that keeps a caller from
// believing a check happened when it did not. Since acmesh-t5k, -c does work
// (see internal/sum), so only the unknown option is left here.
func TestChecksumRefusesUnsupported(t *testing.T) {
	if _, code := captureIn(t, runSum("md5"), []string{"-Z"}, ""); code != 2 {
		t.Fatalf("unknown option must be refused with 2, got %d", code)
	}
}
