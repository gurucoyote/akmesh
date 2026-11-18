package diffpatch

import (
	"os"
	"path/filepath"
	"testing"
)

// TestPatchNamesTheOldFile checks that a plain `diff a b | patch` writes a, not
// b: the "---" line names the file that gets the change, and the "+++" line the
// result. The parser behind gitdiff files both names as the new one, so the
// old name is read from the header itself.
func TestPatchNamesTheOldFile(t *testing.T) {
	dir := t.TempDir()
	a := filepath.Join(dir, "a.txt")
	b := filepath.Join(dir, "b.txt")
	if err := os.WriteFile(a, []byte("eins\nzwei\ndrei\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(b, []byte("eins\nZWEI\ndrei\nvier\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	patchPath := filepath.Join(dir, "p.diff")
	f, err := os.Create(patchPath)
	if err != nil {
		t.Fatal(err)
	}
	if code := Run("diff", []string{"-u", a, b}, nil, f, f); code != 1 {
		t.Fatalf("diff exit: want 1, got %d", code)
	}
	f.Close()

	in, err := os.Open(patchPath)
	if err != nil {
		t.Fatal(err)
	}
	defer in.Close()
	out, err := os.CreateTemp(dir, "out*")
	if err != nil {
		t.Fatal(err)
	}
	defer out.Close()
	errOut, err := os.CreateTemp(dir, "err*")
	if err != nil {
		t.Fatal(err)
	}
	defer errOut.Close()
	code := Run("patch", nil, in, out, errOut)
	errText, _ := os.ReadFile(errOut.Name())
	if code != 0 {
		t.Fatalf("patch exit: want 0, got %d (%s)", code, string(errText))
	}
	got, err := os.ReadFile(a)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != "eins\nZWEI\ndrei\nvier\n" {
		t.Errorf("patched file: got %q", string(got))
	}
}
