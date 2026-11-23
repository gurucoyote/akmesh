package diffpatch

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestLabelsNameTheirOwnFile pins the head of a unified diff: the first label
// names the first file, the second the second one. Using the first name twice
// (acmesh-bwv) still patches, but it reads wrong and no longer matches GNU.
func TestLabelsNameTheirOwnFile(t *testing.T) {
	dir := t.TempDir()
	a := filepath.Join(dir, "eins.txt")
	b := filepath.Join(dir, "zwei.txt")
	if err := os.WriteFile(a, []byte("eins\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(b, []byte("zwei\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	out := filepath.Join(dir, "out")
	outFile, err := os.Create(out)
	if err != nil {
		t.Fatal(err)
	}
	code := Run("diff", []string{"-u", a, b}, nil, outFile, outFile)
	outFile.Close()
	if code != 1 {
		t.Fatalf("exit: want 1 for differing files, got %d", code)
	}
	data, err := os.ReadFile(out)
	if err != nil {
		t.Fatal(err)
	}
	lines := strings.Split(string(data), "\n")
	if len(lines) < 2 {
		t.Fatalf("no head in %q", string(data))
	}
	if !strings.HasPrefix(lines[0], "--- "+a+"\t") {
		t.Errorf("first label: want %q, got %q", "--- "+a, lines[0])
	}
	if !strings.HasPrefix(lines[1], "+++ "+b+"\t") {
		t.Errorf("second label: want %q, got %q", "+++ "+b, lines[1])
	}
}
