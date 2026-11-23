package rgcmd

import (
	"fmt"
	"strings"
	"sync"

	"akmesh/internal/pcre"

	"git.site.quack-lab.dev/dave/gripgrep/match"
)

// pcreMatcher is a ripgrep matcher backed by PCRE2, for "-P" and
// "--engine=pcre2". ripgrep itself needs the "pcre2" cargo feature for this;
// akmesh carries the pure-Go port from internal/pcre.
//
// The patterns are compiled once. Every call takes the mutex, because the
// wrapper holds a PCRE2 match context per compiled pattern and ripgrep shares a
// matcher between walk workers; serialising is the honest choice over a race.
type pcreMatcher struct {
	mu    sync.Mutex
	res   []*pcre.Regexp
	whole bool
}

// newPCREMatcher compiles each pattern on its own and combines them by
// leftmost match, the way the default engine joins them with "|" (a plain join
// would break a pattern that carries an alternation-free inline flag at its
// start, and PCRE2 has no top-level flag scoping across a join).
func newPCREMatcher(cfg match.Config) (match.Matcher, error) {
	m := &pcreMatcher{}
	for _, pat := range cfg.Patterns {
		expr := pat
		if cfg.Word {
			expr = `\b(?:` + expr + `)\b`
		}
		if cfg.CaseMode == match.CaseInsensitive || cfg.CaseMode == match.CaseSmart {
			// Smart case is resolved by the caller; rg's own rule is "a
			// lowercase pattern means case-insensitive", which the caller
			// already folded into CaseMode.
			expr = `(?i)` + expr
		}
		re, err := pcre.Compile(expr)
		if err != nil {
			return nil, fmt.Errorf("PCRE2: %v", err)
		}
		m.res = append(m.res, re)
	}
	if cfg.LineRegexp {
		// -x: the whole line has to match, which rg expresses by anchoring.
		m.whole = true
	}
	return m, nil
}

// FindCandidate reports the first match at or after start. Every hit is
// Confirmed: this matcher has no literal prefilter, so there is nothing to
// verify later.
func (m *pcreMatcher) FindCandidate(buf []byte, start int) (int, match.CandidateKind, bool) {
	s, _, ok, err := m.find(buf, start)
	if err != nil || !ok {
		return 0, match.Confirmed, false
	}
	return s, match.Confirmed, true
}

// Verify answers the yes/no question for one line.
func (m *pcreMatcher) Verify(line []byte) bool {
	_, _, ok, err := m.find(line, 0)
	return err == nil && ok
}

// Find returns the byte bounds of the leftmost match in line.
func (m *pcreMatcher) Find(line []byte) (int, int, bool) {
	s, e, ok, err := m.find(line, 0)
	if err != nil || !ok {
		return 0, 0, false
	}
	return s, e, true
}

// NonMatchingLineTerm is false: a PCRE2 pattern may well match across a
// newline (\\n in the pattern, or (?s)), so the searcher has to go line by line
// instead of scanning whole buffers.
func (m *pcreMatcher) NonMatchingLineTerm() bool { return false }

// find runs every compiled pattern over buf[start:] and keeps the leftmost
// match, which is what a joined alternation would report.
func (m *pcreMatcher) find(buf []byte, start int) (int, int, bool, error) {
	if start > len(buf) {
		return -1, -1, false, nil
	}
	s := asString(buf[start:])
	m.mu.Lock()
	defer m.mu.Unlock()
	bestStart, bestEnd, found := -1, -1, false
	for _, re := range m.res {
		st, en, ok, err := re.Find(s)
		if err != nil {
			return -1, -1, false, err
		}
		if !ok {
			continue
		}
		if m.whole && (st != 0 || en != len(s)) {
			continue
		}
		if !found || st < bestStart {
			bestStart, bestEnd, found = st, en, true
		}
	}
	if !found {
		return -1, -1, false, nil
	}
	return bestStart + start, bestEnd + start, true, nil
}

// asString converts without a copy the way the hot path needs it; PCRE2 wants a
// string, so the copy is unavoidable here, but it is one allocation per call
// and the wrapper's limits keep the work bounded.
func asString(b []byte) string { return string(b) }

var _ match.Matcher = (*pcreMatcher)(nil)

// engineFor resolves --engine and -P into the engine name the builder uses.
func engineFor(pcreFlag bool, engine string) string {
	switch engine {
	case "pcre2":
		return "pcre2"
	case "default":
		if pcreFlag {
			return "pcre2"
		}
		return "default"
	case "auto":
		if pcreFlag {
			return "pcre2"
		}
		return "auto"
	}
	if pcreFlag {
		return "pcre2"
	}
	return ""
}

// looksLikePCREOnly reports whether a pattern uses a construct RE2 rejects, so
// --engine=auto can fall back to PCRE2 the way ripgrep does.
func looksLikePCREOnly(pattern string) bool {
	for _, s := range []string{"(?=", "(?!", "(?<=", "(?<!", "\\K", "(?>", "\\p{", "++", "*+"} {
		if strings.Contains(pattern, s) {
			return true
		}
	}
	return false
}
