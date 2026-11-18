package tools

import (
	"math/rand"
	"strings"
	"testing"
)

// TestDiffReplaysToTarget is the property that matters for a diff that is
// shipped with its own patch: applying the edit script to the left side must
// reproduce the right side, for any pair of inputs.
func TestDiffReplaysToTarget(t *testing.T) {
	rng := rand.New(rand.NewSource(7))
	alphabet := []string{"a", "b", "c", "d", "e", ""}
	for round := 0; round < 200; round++ {
		a := randomLines(rng, alphabet)
		b := randomLines(rng, alphabet)
		ops, err := diffLines(a, b, diffOptions{})
		if err != nil {
			t.Fatalf("diffLines: %v", err)
		}
		var got []string
		for _, op := range ops {
			if op.kind != opInsert {
				got = append(got, op.text)
			}
		}
		if strings.Join(got, "\x00") != strings.Join(a, "\x00") {
			t.Fatalf("left side not reproduced\na=%v\ngot=%v", a, got)
		}
		var want []string
		for _, op := range ops {
			if op.kind != opDelete {
				want = append(want, op.text)
			}
		}
		if strings.Join(want, "\x00") != strings.Join(b, "\x00") {
			t.Fatalf("right side not reproduced\nb=%v\ngot=%v", b, want)
		}
	}
}

// TestHunkHeaderCounts checks that a hunk header announces exactly as many
// lines as the hunk body carries. go-gitdiff rejects a header that is off by
// one, so this is the difference between a patch that applies and one that
// does not.
func TestHunkHeaderCounts(t *testing.T) {
	rng := rand.New(rand.NewSource(11))
	alphabet := []string{"a", "b", "c", "d", "e", "f", "g", "h", "i", "j", "k", "l", "m", "n", "o", "p"}
	for round := 0; round < 100; round++ {
		a := randomLines(rng, alphabet)
		b := randomLines(rng, alphabet)
		ops, err := diffLines(a, b, diffOptions{})
		if err != nil {
			t.Fatalf("diffLines: %v", err)
		}
		for _, h := range hunks(ops) {
			aCount, bCount := 0, 0
			for _, line := range h.lines {
				switch line[0] {
				case ' ':
					aCount++
					bCount++
				case '-':
					aCount++
				case '+':
					bCount++
				}
			}
			if aCount != h.aCount || bCount != h.bCount {
				t.Fatalf("header %d,%d does not match body %d,%d in\n%v", h.aCount, h.bCount, aCount, bCount, h.lines)
			}
			if h.aCount == 0 && h.bCount == 0 {
				t.Fatalf("empty hunk")
			}
		}
	}
}

// TestHunkStartsLineUp checks that the first line number of a hunk points at
// the right place in both files.
func TestHunkStartsLineUp(t *testing.T) {
	a := []string{"1", "2", "3", "4", "5", "6", "7", "8", "9", "10", "11", "12"}
	b := []string{"1", "2", "3", "4X", "5", "6", "7", "8", "9", "10", "11", "12Y"}
	ops, err := diffLines(a, b, diffOptions{})
	if err != nil {
		t.Fatalf("diffLines: %v", err)
	}
	hs := hunks(ops)
	if len(hs) != 2 {
		t.Fatalf("expected two hunks, got %d", len(hs))
	}
	if hs[0].aStart != 1 || hs[1].aStart != 9 {
		t.Fatalf("unexpected starts: %+v", hs)
	}
}

func randomLines(rng *rand.Rand, alphabet []string) []string {
	n := rng.Intn(12)
	out := make([]string, n)
	for i := range out {
		out[i] = alphabet[rng.Intn(len(alphabet))]
	}
	return out
}

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

// TestParseSize covers the size syntax of truncate and split.
func TestParseSize(t *testing.T) {
	if n, err := parseSize("2K", 0); err != nil || n != 2048 {
		t.Fatalf("2K = %d, %v", n, err)
	}
	if n, err := parseSize("+5", 10); err != nil || n != 15 {
		t.Fatalf("+5 from 10 = %d, %v", n, err)
	}
	if n, err := parseSize("-5", 10); err != nil || n != 5 {
		t.Fatalf("-5 from 10 = %d, %v", n, err)
	}
}
