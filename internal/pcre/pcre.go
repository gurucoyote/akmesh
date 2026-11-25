// Package pcre is akmesh's thin wrapper around a pure-Go PCRE2 10.44 port
// (github.com/go-regexp/pcre2, BSD-3-Clause, no cgo).
//
// It exists because grep -P and rg -P have to behave like PCRE, not like Go's
// RE2 engine: \K, possessive quantifiers and lookbehind are exactly what agents
// write, and RE2 refuses them. docs/pcre-recherche.md holds the measurement that
// led here; this file keeps the dependency behind one door, so a later swap
// costs a single file.
//
// Everything here is bounded: a pattern that backtracks forever returns an
// error instead of hanging the binary, because a shell command that does not
// return is worse than one that says "match limit exceeded".
package pcre

import (
	"fmt"
	"unicode/utf8"

	"github.com/go-regexp/pcre2"
)

// Limits for one match. PCRE2's own default is 10,000,000 backtracking steps;
// for a command line tool that is already the sane choice, so we keep the count
// but add a wall clock budget on top, because a single step can be slow.
const (
	matchLimit = 10_000_000
	depthLimit = 10_000
	heapLimit  = 20_000_000
)

// Regexp is a compiled pattern. It is safe for use by one goroutine; grep and rg
// use one per invocation.
type Regexp struct {
	expr string
	utf  *pcre2.Code // UTF-8 and Unicode properties, the normal case
	raw  *pcre2.Code // byte mode, for input that is not valid UTF-8
	mc   *pcre2.MatchContext
}

// Compile compiles expr. The error text carries PCRE2's own message and the
// offset, the way grep -P reports a bad pattern.
func Compile(expr string) (*Regexp, error) {
	r := &Regexp{expr: expr}
	var err error
	if r.utf, err = compile(expr, pcre2.PCRE2_UTF|pcre2.PCRE2_UCP); err != nil {
		return nil, err
	}
	// The byte mode is only used when the input is not valid UTF-8; failing to
	// compile it is not an error, because then there is nothing to search with
	// and the caller gets PCRE2's message from the UTF path.
	r.raw, _ = compile(expr, 0)
	r.mc = pcre2.NewMatchContext()
	r.mc.SetMatchLimit(matchLimit)
	r.mc.SetDepthLimit(depthLimit)
	r.mc.SetHeapLimit(heapLimit)
	return r, nil
}

func compile(expr string, opts uint32) (*pcre2.Code, error) {
	code, errcode, offset := pcre2.Compile([]byte(expr), opts, nil)
	if code == nil {
		return nil, fmt.Errorf("%s at offset %d", pcre2.GetErrorMessage(errcode), offset)
	}
	return code, nil
}

// String returns the pattern, for error messages.
func (r *Regexp) String() string { return r.expr }

// Match reports whether the string contains a match.
func (r *Regexp) Match(s string) (bool, error) {
	start, _, err := r.match(s)
	if err != nil {
		return false, err
	}
	return start >= 0, nil
}

// FindAll returns the non-overlapping matches of s, at most n (n < 0 means all),
// in the order they appear. An empty match does not loop: it advances by one
// rune, as grep does.
func (r *Regexp) FindAll(s string, n int) ([]string, error) {
	var out []string
	pos := 0
	for n < 0 || len(out) < n {
		start, end, err := r.match(s[pos:])
		if err != nil {
			return out, err
		}
		if start < 0 {
			return out, nil
		}
		out = append(out, s[pos+start:pos+end])
		if end == start {
			// empty match: move past one rune so the loop terminates
			_, size := utf8.DecodeRuneInString(s[pos+end:])
			if size == 0 {
				return out, nil
			}
			pos += end + size
			continue
		}
		pos += end
	}
	return out, nil
}

// Find returns the byte offsets of the first match in s, and ok=false when the
// pattern does not match. Start and end are byte positions, like RE2's.
func (r *Regexp) Find(s string) (start, end int, ok bool, err error) {
	start, end, err = r.match(s)
	if err != nil || start < 0 {
		return -1, -1, false, err
	}
	return start, end, true, nil
}

// match returns the start and end offset of the first match in s, or -1 when
// there is none.
func (r *Regexp) match(s string) (int, int, error) {
	code := r.code(s)
	md := pcre2.NewMatchData(code)
	defer md.Free()
	rc := pcre2.Match(code, []byte(s), 0, 0, md, r.mc)
	if rc < 0 {
		if err := r.matchError(rc); err != nil {
			return -1, -1, err
		}
		return -1, -1, nil
	}
	ov := md.Ovector()
	return int(ov[0]), int(ov[1]), nil
}

// code picks the UTF or the byte variant for this subject.
func (r *Regexp) code(s string) *pcre2.Code {
	if utf8.ValidString(s) || r.raw == nil {
		return r.utf
	}
	return r.raw
}

// matchError turns a negative return code into an error, except for "no match".
func (r *Regexp) matchError(rc int) error {
	switch rc {
	case pcre2.PCRE2_ERROR_NOMATCH:
		return nil
	case pcre2.PCRE2_ERROR_MATCHLIMIT, pcre2.PCRE2_ERROR_DEPTHLIMIT, pcre2.PCRE2_ERROR_HEAPLIMIT:
		return fmt.Errorf("%s for pattern %q (the pattern backtracks too much)", pcre2.GetErrorMessage(rc), r.expr)
	}
	return fmt.Errorf("%s for pattern %q", pcre2.GetErrorMessage(rc), r.expr)
}
