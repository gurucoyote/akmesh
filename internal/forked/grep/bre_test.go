package grep

import "testing"

// TestToERE checks the rewrite itself. Each expectation is what GNU's BRE
// meaning demands, not what looks tidy: a bare + is a literal in BRE, a
// backslashed one is the quantifier.
func TestToERE(t *testing.T) {
	cases := []struct{ bre, ere string }{
		{`a+b`, `a\+b`},                   // literal plus
		{`a\+b`, `a+b`},                   // GNU's quantified plus
		{`a?b`, `a\?b`},                   // literal question mark
		{`a\?b`, `a?b`},                   // optional
		{`a\|b`, `a|b`},                   // alternation needs the backslash in BRE
		{`a|b`, `a\|b`},                   // bare pipe is a literal
		{`a\(a\)b`, `a(a)b`},              // group
		{`(a)`, `\(a\)`},                  // bare parens are literals
		{`a\{1,3\}`, `a{1,3}`},            // interval
		{`a{1,3}`, `a\{1,3\}`},            // bare braces are literals
		{`*a`, `\*a`},                     // a leading * is a literal in BRE
		{`^a$`, `^a$`},                    // anchors stay
		{`a.b`, `a.b`},                    // the dot is special in both
		{`a\.b`, `a\.b`},                  // escaped dot stays escaped
		{`[[:alpha:]]+`, `[[:alpha:]]\+`}, // a class is copied, the + is literal
		{`\<word\>`, `\bword\b`},          // GNU word boundaries
		{`[\]]`, `[\]]`},                  // a class holding a bracket
	}
	for _, c := range cases {
		if got := toERE(c.bre); got != c.ere {
			t.Errorf("toERE(%q) = %q, want %q", c.bre, got, c.ere)
		}
	}
}

// TestBasicRegexpMatchesGNU holds the measured cases from acmesh-4gi: with this
// rewrite grep answers what GNU answers on the same input.
func TestBasicRegexpMatchesGNU(t *testing.T) {
	const input = "aab\naxb\nb\nc\n"
	cases := []struct {
		pattern string
		want    string
		code    int
	}{
		{`a+b`, "", 1}, // literal plus: nothing contains "a+b"
		{`a\(a\)b`, "aab\n", 0},
		{`a\|b`, "aab\naxb\nb\n", 0}, // alternation with the backslash
		{`a\{2\}`, "aab\n", 0},
		{`(` + `a` + `)`, "", 1}, // literal parens match nothing here
	}
	for _, c := range cases {
		out, _, code := runGrep(t, []string{c.pattern}, input)
		if out != c.want || code != c.code {
			t.Errorf("grep %q = %d, %q; want %d, %q", c.pattern, code, out, c.code, c.want)
		}
	}
}

// TestExtendedStillERE: -E must not be rewritten, so a bare + stays the
// quantifier there.
func TestExtendedStillERE(t *testing.T) {
	out, _, code := runGrep(t, []string{"-E", "a+b"}, "aab\naxb\n")
	if out != "aab\n" || code != 0 {
		t.Fatalf("grep -E a+b = %d, %q", code, out)
	}
	if _, _, code := runGrep(t, []string{"-E", "a+b"}, "a+b\n"); code != 1 {
		t.Fatal("grep -E a+b matched the literal plus")
	}
}
