package pcre

import (
	"strings"
	"testing"
	"time"
)

// timeoutAfter is the guard for the catastrophe test: PCRE2 has to give up long
// before this fires.
func timeoutAfter() <-chan time.Time { return time.After(30 * time.Second) }

// The patterns here are the ones GNU grep -P accepted in the measurement
// (docs/pcre-recherche.md); the expected values are what GNU printed.
func TestGNUCompatPatterns(t *testing.T) {
	cases := []struct {
		pattern string
		subject string
		want    bool
	}{
		{`A(?=B)`, "AB", true},
		{`A(?=B)`, "AC", false},
		{`(?<=C)D`, "CD", true},
		{`(?<n>ab)\k<n>`, "abab", true},
		{`a\Kb`, "ab", true},          // \K: match starts at the b
		{`a++`, "aaa", true},          // possessive quantifier
		{`\p{Greek}`, "abcαβγ", true}, // Unicode property
		{`\p{Greek}`, "abcdef", false},
		{`[[:alpha:]]+`, "AB1", true},
		{`\bword\b`, "a word here", true},
		{`\bword\b`, "wordy", false},
	}
	for _, c := range cases {
		re, err := Compile(c.pattern)
		if err != nil {
			t.Fatalf("Compile(%q): %v", c.pattern, err)
		}
		got, err := re.Match(c.subject)
		if err != nil {
			t.Fatalf("Match(%q, %q): %v", c.pattern, c.subject, err)
		}
		if got != c.want {
			t.Errorf("Match(%q, %q) = %v, want %v", c.pattern, c.subject, got, c.want)
		}
	}
}

func TestKResetsMatchStart(t *testing.T) {
	re, err := Compile(`a\Kb`)
	if err != nil {
		t.Fatal(err)
	}
	got, err := re.FindAll("xab", -1)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 || got[0] != "b" {
		t.Fatalf("FindAll = %q, want [b]", got)
	}
}

func TestFindAllOrderAndCount(t *testing.T) {
	re, err := Compile(`\d+`)
	if err != nil {
		t.Fatal(err)
	}
	got, err := re.FindAll("a1 b22 c333", -1)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Join(got, ",") != "1,22,333" {
		t.Fatalf("FindAll = %q", got)
	}
	got, err = re.FindAll("a1 b22 c333", 2)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 {
		t.Fatalf("FindAll with n=2 returned %d matches", len(got))
	}
}

func TestEmptyMatchTerminates(t *testing.T) {
	// A pattern that can match the empty string must not loop forever.
	re, err := Compile(`b*`)
	if err != nil {
		t.Fatal(err)
	}
	got, err := re.FindAll("abc", -1)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) < 3 {
		t.Fatalf("FindAll = %q, want at least three empty matches", got)
	}
}

func TestCompileErrorCarriesMessage(t *testing.T) {
	_, err := Compile(`a(b`)
	if err == nil {
		t.Fatal("Compile(a(b) returned no error")
	}
	if !strings.Contains(err.Error(), "at offset") {
		t.Fatalf("error %q has no offset", err)
	}
}

func TestInvalidUTF8FallsBackToBytes(t *testing.T) {
	re, err := Compile(`a.b`)
	if err != nil {
		t.Fatal(err)
	}
	// 0xff is not valid UTF-8; GNU grep -P handles such input byte by byte.
	if _, err := re.Match("a\xffb"); err != nil {
		t.Fatalf("Match on invalid UTF-8: %v", err)
	}
}

func TestBacktrackingIsBounded(t *testing.T) {
	// (a+)+$ against a long run of a's plus one b is the classic blowup; the
	// match limit has to stop it instead of running for minutes.
	re, err := Compile(`(a+)+$`)
	if err != nil {
		t.Fatal(err)
	}
	subject := strings.Repeat("a", 40) + "b"
	done := make(chan struct{})
	var matchErr error
	go func() {
		_, matchErr = re.Match(subject)
		close(done)
	}()
	select {
	case <-done:
		if matchErr == nil {
			t.Fatal("expected a limit error for a catastrophic pattern")
		}
	case <-timeoutAfter():
		t.Fatal("Match did not return; the limit is not applied")
	}
}

// TestFindReportsOffsets: rg needs the byte bounds, not just a yes.
func TestFindReportsOffsets(t *testing.T) {
	re, err := Compile(`b(c+)`)
	if err != nil {
		t.Fatal(err)
	}
	start, end, ok, err := re.Find("aabccc")
	if err != nil || !ok {
		t.Fatalf("Find: %v %v", ok, err)
	}
	if start != 2 || end != 6 {
		t.Fatalf("Find = [%d,%d), want [2,6)", start, end)
	}
	if _, _, ok, err := re.Find("nothing"); ok || err != nil {
		t.Fatalf("Find on a miss = %v %v", ok, err)
	}
}
