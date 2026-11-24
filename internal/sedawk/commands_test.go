package sedawk

import "testing"

// TestSedListCommand: "l" writes the pattern space with escapes, one line per
// record ending in "$", long lines broken with a backslash. Expected values
// measured against GNU sed 4.9 in the reference image (acmesh-qxaf).
func TestSedListCommand(t *testing.T) {
	cases := []struct{ in, want string }{
		{"abc\n", "abc$\n"},
		{"a\tb\x01c\x7fd\n", "a\\tb\\001c\\177d$\n"},
		{"a\\b\n", "a\\\\b$\n"},
		{"a\nb\n", "a$\nb$\n"},
	}
	for _, c := range cases {
		out, code := runSedText(t, c.in, "-n", "l")
		if code != 0 || out != c.want {
			t.Fatalf("sed -n l %q = %d, %q; want %q", c.in, code, out, c.want)
		}
	}
}

// TestSedBranchIfUnchanged: T branches when nothing changed, the opposite of t.
func TestSedBranchIfUnchanged(t *testing.T) {
	out, code := runSedText(t, "a\nb\nc\n", "s/a/A/;T end;s/.*/no/;:end")
	if want := "no\nb\nc\n"; code != 0 || out != want {
		t.Fatalf("sed T = %d, %q; want %q", code, out, want)
	}
}

// TestSedQuitAndZap: Q leaves without printing the pattern space, z empties it.
func TestSedQuitAndZap(t *testing.T) {
	if out, code := runSedText(t, "a\nb\n", "2Q"); code != 0 || out != "a\n" {
		t.Fatalf("sed 2Q = %d, %q; want %q", code, out, "a\n")
	}
	if out, code := runSedText(t, "a\nb\n", "z;s/^/X/"); code != 0 || out != "X\nX\n" {
		t.Fatalf("sed z = %d, %q; want %q", code, out, "X\nX\n")
	}
}

// TestSedUnknownLabelStillFails: a branch to a label that does not exist is
// still an error after the T handling was added.
func TestSedUnknownLabelStillFails(t *testing.T) {
	_, code := runSedText(t, "a\n", "T missing")
	if code == 0 {
		t.Fatal("sed T missing succeeded, want an error")
	}
}
