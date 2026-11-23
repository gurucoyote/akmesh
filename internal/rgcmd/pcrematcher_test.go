package rgcmd

import (
	"strings"
	"testing"
)

// TestPCREEngine: -P, --pcre2 und --engine=pcre2 gehen an PCRE2, Muster, die RE2
// nicht kann, treffen damit, ohne -P bleiben sie ein Fehler (Exit 2), und
// --engine=auto faellt genau dann auf PCRE2 zurueck.
func TestPCREEngine(t *testing.T) {
	dir := fixture(t)
	cases := []struct {
		name string
		args []string
		want []string
		code int
	}{
		{"plain engine", []string{`beta alpha`, "."}, []string{"a.txt"}, 0},
		{"lookahead", []string{"-P", `beta(?= alpha)`, "."}, []string{"a.txt"}, 0},
		{"lookbehind", []string{"--pcre2", `(?<=beta )alpha`, "."}, []string{"a.txt"}, 0},
		{"engine pcre2", []string{"--engine=pcre2", `beta \Kalpha`, "."}, []string{"a.txt"}, 0},
		{"possessive", []string{"-P", `alpha++`, "."}, []string{"a.txt"}, 0},
		{"auto falls back", []string{"--engine=auto", `beta(?= alpha)`, "."}, []string{"a.txt"}, 0},
		{"default rejects", []string{`beta(?= alpha)`, "."}, nil, 2},
		{"engine default rejects", []string{"--engine=default", `beta(?= alpha)`, "."}, nil, 2},
		{"bad engine", []string{"--engine=nope", "alpha", "."}, nil, 2},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			out, stderr, code := runOurs(t, dir, c.args)
			if code != c.code {
				t.Fatalf("exit = %d, want %d (stderr %q)", code, c.code, stderr)
			}
			for _, file := range c.want {
				if !strings.Contains(out, file) {
					t.Fatalf("output %q does not name %s", out, file)
				}
			}
			if c.code == 2 && stderr == "" {
				t.Fatalf("an unsupported pattern must say why")
			}
		})
	}
}

// TestPCREWordAndCase: -w und -i muessen auch in der PCRE-Engine wirken, sonst
// liefert dieselbe Suche je nach Engine ein anderes Ergebnis.
func TestPCREWordAndCase(t *testing.T) {
	dir := fixture(t)
	out, stderr, code := runOurs(t, dir, []string{"-P", "-i", "-w", "ALPHA", "."})
	if code != 0 {
		t.Fatalf("exit = %d (stderr %q)", code, stderr)
	}
	if !strings.Contains(out, "a.txt") {
		t.Fatalf("output %q does not name a.txt", out)
	}
	// -w auf "alph" darf "alpha" nicht treffen.
	out, _, code = runOurs(t, dir, []string{"-P", "-w", "alph", "."})
	if code != 1 || out != "" {
		t.Fatalf("-w matched inside a word: %q (exit %d)", out, code)
	}
}
