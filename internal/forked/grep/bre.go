// Local addition, no upstream source: BRE translation layer for akmesh's grep.

package grep

// BRE, the default pattern language of GNU grep. Without -E, grep reads the
// pattern as a Basic Regular Expression: +, ?, |, ( ) and { } are ordinary
// characters there, and their special meaning is written with a backslash.
// Go's regexp only speaks ERE, so this file rewrites the pattern before it is
// compiled.
//
// The measurement that made this necessary is in docs/abweichungen-gnu.md
// (acmesh-4gi): "grep 'a+b'" matched "aab" here and nothing in GNU.

import "strings"

// toERE rewrites a BRE pattern into the ERE spelling Go's regexp understands.
// Character classes are copied unchanged; a backslash before an ordinary
// character stays, because it means the same in both languages.
func toERE(pattern string) string {
	var b strings.Builder
	b.Grow(len(pattern) + 8)
	inClass := false
	// atStartOfBranch says the next * is a literal: in BRE a leading * is an
	// ordinary character, in ERE it is a syntax error.
	atStartOfBranch := true
	for i := 0; i < len(pattern); i++ {
		c := pattern[i]
		if inClass {
			b.WriteByte(c)
			if c == ']' {
				inClass = false
			}
			continue
		}
		switch c {
		case '\\':
			if i+1 >= len(pattern) {
				b.WriteByte('\\')
				continue
			}
			n := pattern[i+1]
			i++
			switch n {
			case '(', ')', '|', '+', '?', '{', '}':
				// GNU BRE extension: these grow their ERE meaning
				b.WriteByte(n)
			case '<', '>':
				// GNU word boundaries; Go knows \b
				b.WriteString(`\b`)
			default:
				b.WriteByte('\\')
				b.WriteByte(n)
			}
			atStartOfBranch = false
		case '[':
			// copy the whole class, including a leading ^ or ]
			b.WriteByte(c)
			inClass = true
			if i+1 < len(pattern) && pattern[i+1] == '^' {
				i++
				b.WriteByte('^')
			}
			if i+1 < len(pattern) && pattern[i+1] == ']' {
				i++
				b.WriteByte(']')
			}
			continue
		case '(', ')', '|', '+', '?', '{', '}':
			// ordinary in BRE, metacharacter in ERE: escape it
			b.WriteByte('\\')
			b.WriteByte(c)
			atStartOfBranch = false
		case '*':
			if atStartOfBranch {
				b.WriteString(`\*`)
			} else {
				b.WriteByte('*')
			}
			atStartOfBranch = false
		case '^':
			b.WriteByte('^')
			atStartOfBranch = true
		case '$':
			b.WriteByte('$')
			atStartOfBranch = false
		case '.':
			b.WriteByte('.')
			atStartOfBranch = false
		default:
			b.WriteByte(c)
			atStartOfBranch = false
		}
	}
	return b.String()
}
