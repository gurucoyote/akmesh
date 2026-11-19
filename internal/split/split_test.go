package split

import (
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
)

// TestSuffixName checks that split names stay unique and ordered.
func TestSuffixName(t *testing.T) {
	seen := map[string]bool{}
	// 26^2 names is the capacity of a two letter suffix; beyond that the
	// caller must refuse, which runSplit does.
	for i := 0; i < 26*26; i++ {
		name := suffixName(i, 2)
		if len(name) != 2 {
			t.Fatalf("suffix %q has length %d", name, len(name))
		}
		if seen[name] {
			t.Fatalf("suffix %q repeated at index %d", name, i)
		}
		seen[name] = true
	}
}

// tempFile is one of the three real streams Run needs, with its path.
func tempFile(t *testing.T, dir, name string) (*os.File, string) {
	t.Helper()
	path := filepath.Join(dir, name)
	f, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { f.Close() })
	return f, path
}

// TestLinesSplitIntoParts: "split -l 2 big part" is what the battery checked
// with ls and wc. Five lines at two per part are three parts named partaa,
// partab, partac with 2, 2 and 1 lines - count and sizes both matter, because a
// script that reads partaa expects the first two lines there.
func TestLinesSplitIntoParts(t *testing.T) {
	dir := t.TempDir()
	in, _ := tempFile(t, dir, "big.txt")
	if _, err := in.WriteString("1\n2\n3\n4\n5\n"); err != nil {
		t.Fatal(err)
	}
	in.Seek(0, 0)
	out, _ := tempFile(t, dir, "out")
	errOut, _ := tempFile(t, dir, "err")
	prefix := filepath.Join(dir, "part")
	if code := Run([]string{"-l", "2", in.Name(), prefix}, in, out, errOut); code != 0 {
		t.Fatalf("split -l 2 = %d", code)
	}

	parts, err := filepath.Glob(prefix + "[a-z][a-z]")
	if err != nil {
		t.Fatal(err)
	}
	if len(parts) != 3 {
		t.Fatalf("got %d parts %v, want 3", len(parts), parts)
	}
	sort.Strings(parts)
	wantNames := []string{prefix + "aa", prefix + "ab", prefix + "ac"}
	wantLines := []int{2, 2, 1}
	for i, p := range wantNames {
		data, err := os.ReadFile(p)
		if err != nil {
			t.Fatalf("%s: %v", p, err)
		}
		if got := strings.Count(string(data), "\n"); got != wantLines[i] {
			t.Fatalf("%s has %d lines, want %d (%q)", p, got, wantLines[i], data)
		}
	}
}

// TestTooManyPartsIsRefused: the two-letter suffix holds 26*26 parts, the next
// one has to be refused instead of overwriting partaa.
func TestTooManyPartsIsRefused(t *testing.T) {
	dir := t.TempDir()
	in, _ := tempFile(t, dir, "big.txt")
	if _, err := in.WriteString(strings.Repeat("x\n", 26*26+1)); err != nil {
		t.Fatal(err)
	}
	in.Seek(0, 0)
	out, _ := tempFile(t, dir, "out")
	errOut, _ := tempFile(t, dir, "err")
	if code := Run([]string{"-l", "1", in.Name(), filepath.Join(dir, "part")}, in, out, errOut); code == 0 {
		t.Fatalf("split with more parts than suffixes = 0, want a refusal")
	}
}
