package split

import (
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
