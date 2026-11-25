// Copyright 2018 the u-root Authors. All rights reserved
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// tr - translate or delete characters

// Synopsis:
//     tr [OPTION]... SET1 [SET2]

// Description:
//     Translate, squeeze, and/or delete characters from standard input, writing
//     to standard output.
//
//     -d, --delete: delete characters in SET1, do not translate
//
// SETs  are  specified  as  strings of characters. Most represent themselves.
// Interpreted sequences are:
//     \\        backslash
//     \a        audible BEL
//     \b        backspace
//     \f        form feed
//     \n        new line
//     \r        return
//     \t        horizontal tab
//     \v        vertical tab
//     [:alnum:] all letters and digits
//     [:alpha:] all letters
//     [:digit:] all digits
//     [:graph:] all printable characters
//     [:cntrl:] all control characters
//     [:lower:] all lower case letters
//     [:upper:] all upper case letters
//     [:space:] all whitespaces
//
// Forked from u-root 869f00f6b224ce02b1db51806159d6cd9c246f76, cmds/core/tr/tr.go.
// The rest of this file is the upstream source and keeps its copyright.
//
// Local changes, all of them for the way agents call tr:
//   - package tr and Run(args, stdin, stdout, stderr) instead of main();
//   - no flag package: an own parser, so the bundled -cd and -ds forms work;
//   - new: -s/--squeeze-repeats and -c/--complement. Without -c the complement
//     rule is GNU's: characters outside SET1 are translated with SET2 cycling,
//     and with -d they are what stays;

package tr

import (
	"bufio"
	"fmt"
	"io"
	"slices"
	"sort"
	"strings"
	"unicode"
)

type cmd struct {
	stdin    io.Reader
	stdout   io.Writer
	del      bool
	squeeze  bool
	tr       *transformer
	squeezed func(rune) bool
}

func command(in io.Reader, out io.Writer, args []string, p params) (*cmd, error) {
	tr, err := parse(args, p)
	if err != nil {
		return nil, err
	}

	c := &cmd{
		stdin:   in,
		stdout:  out,
		del:     p.del,
		squeeze: p.squeeze,
		tr:      tr,
	}
	if p.squeeze {
		// With -d the repeated characters of SET1 are squeezed, otherwise the
		// ones SET2 produces. Without -s nothing is squeezed.
		c.squeezed = squeezeSet(args, p)
	}
	return c, nil
}

// squeezeSet returns the test for "this rune may not repeat".
func squeezeSet(args []string, p params) func(rune) bool {
	if p.del || len(args) < 2 {
		set := args[0]
		return func(r rune) bool { return strings.ContainsRune(set, r) }
	}
	set := args[1]
	return func(r rune) bool {
		if strings.ContainsRune(set, r) {
			return true
		}
		// [:upper:] and [:lower:] stand for their whole class.
		if set == string(UPPER) {
			return sets[UPPER](r)
		}
		if set == string(LOWER) {
			return sets[LOWER](r)
		}
		return false
	}
}

const name = "tr"

var escapeChars = map[rune]rune{
	'\\': '\\',
	'a':  '\a',
	'b':  '\b',
	'f':  '\f',
	'n':  '\n',
	'r':  '\r',
	't':  '\t',
	'v':  '\v',
}

type Set string

const (
	ALPHA Set = "[:alpha:]"
	DIGIT Set = "[:digit:]"
	GRAPH Set = "[:graph:]"
	CNTRL Set = "[:cntrl:]"
	PUNCT Set = "[:punct:]"
	SPACE Set = "[:space:]"
	ALNUM Set = "[:alnum:]"
	LOWER Set = "[:lower:]"
	UPPER Set = "[:upper:]"
)

// The classes are the ASCII ones, as GNU has them in the C and C.UTF-8
// locales: "tr [:lower:] [:upper:]" leaves Ä as it is, because ä is not in the
// class there. Using unicode.IsUpper would also upper-case it and drift from
// the tool an agent knows (acmesh-49y).
func asciiRange(r rune, lo, hi rune) bool { return r >= lo && r <= hi }

var sets = map[Set]func(r rune) bool{
	ALNUM: func(r rune) bool {
		return asciiRange(r, '0', '9') || asciiRange(r, 'A', 'Z') || asciiRange(r, 'a', 'z')
	},
	ALPHA: func(r rune) bool { return asciiRange(r, 'A', 'Z') || asciiRange(r, 'a', 'z') },
	DIGIT: func(r rune) bool { return asciiRange(r, '0', '9') },
	GRAPH: func(r rune) bool { return r >= 0x21 && r <= 0x7e },
	CNTRL: func(r rune) bool { return r < 0x20 || r == 0x7f },
	PUNCT: func(r rune) bool {
		return r >= 0x21 && r <= 0x7e && !(asciiRange(r, '0', '9') || asciiRange(r, 'A', 'Z') || asciiRange(r, 'a', 'z'))
	},
	SPACE: func(r rune) bool { return r == ' ' || (r >= '\t' && r <= '\r') },
	LOWER: func(r rune) bool { return asciiRange(r, 'a', 'z') },
	UPPER: func(r rune) bool { return asciiRange(r, 'A', 'Z') },
}

type transformer struct {
	transform func(r rune) rune
}

func setToRune(s Set, outRune rune) *transformer {
	check := sets[s]
	return &transformer{
		transform: func(r rune) rune {
			if check(r) {
				return outRune
			}
			return r
		},
	}
}

func lowerToUpper() *transformer {
	return &transformer{
		transform: func(r rune) rune {
			// ASCII only, as GNU in the C and C.UTF-8 locales.
			if r >= 'a' && r <= 'z' {
				return r - 'a' + 'A'
			}
			return r
		},
	}
}

func upperToLower() *transformer {
	return &transformer{
		transform: func(r rune) rune {
			if r >= 'A' && r <= 'Z' {
				return r - 'A' + 'a'
			}
			return r
		},
	}
}

func runesToRunes(in []rune, out ...rune) *transformer {
	convs := make(map[rune]rune)
	l := len(out)
	for i, r := range in {
		ind := min(i, l-1)
		convs[r] = out[ind]
	}
	return &transformer{
		transform: func(r rune) rune {
			if outRune, ok := convs[r]; ok {
				return outRune
			}
			return r
		},
	}
}

func (c *cmd) run() error {
	in := bufio.NewReader(c.stdin)
	out := bufio.NewWriter(c.stdout)

	defer out.Flush()

	var prev rune = -1
	for {
		inRune, size, err := in.ReadRune()
		if inRune == unicode.ReplacementChar {
			// can skip error handling here, because
			// previous operation was in.ReadRune()
			in.UnreadRune()

			b, err := in.ReadByte()
			if err != nil {
				return fmt.Errorf("read error: %w", err)
			}

			if err := out.WriteByte(b); err != nil {
				return fmt.Errorf("write error: %w", err)
			}
		} else if size > 0 {
			if outRune := c.tr.transform(inRune); outRune != unicode.ReplacementChar {
				if c.squeezed != nil && outRune == prev && c.squeezed(outRune) {
					// keep reading; the repeat is dropped
				} else if _, err := out.WriteRune(outRune); err != nil {
					return fmt.Errorf("write error: %w", err)
				}
				prev = outRune
			}
		}

		if err != nil {
			if err == io.EOF {
				return nil
			}
			return err
		}
	}
}

func parse(args []string, p params) (*transformer, error) {
	del := p.del
	narg := len(args)

	switch {
	case narg == 0:
		return nil, fmt.Errorf("missing operand\nTry 'tr --help' for more information.")
	case narg == 1 && del:
		// -d alone needs one SET; -ds needs two, as GNU says.
		if p.squeeze {
			return nil, fmt.Errorf("missing operand after '%s'\nTwo strings must be given when both deleting and squeezing repeats.\nTry 'tr --help' for more information.", args[0])
		}
	case narg == 1 && !del && !p.squeeze:
		return nil, fmt.Errorf("missing operand after '%s'\nTwo strings must be given when translating.\nTry 'tr --help' for more information.", args[0])
	case narg > 1 && del:
		return nil, fmt.Errorf("extra operand '%s'\nTry 'tr --help' for more information.", args[0])
	case narg > 2:
		return nil, fmt.Errorf("extra operand '%s'", args[1])
	case narg == 1 && !del && p.squeeze:
		// "tr -s SET" only squeezes; it translates nothing.
		return &transformer{transform: func(r rune) rune { return r }}, nil
	}

	set1 := Set(args[0])
	arg1, err := unescape(set1)
	if err != nil {
		return nil, err
	}
	arg1, err = expandClasses(arg1)
	if err != nil {
		return nil, err
	}
	arg1 = expandRanges(arg1)

	var set2 Set
	if del {
		set2 = Set(unicode.ReplacementChar)
	} else {
		set2 = Set(args[1])
	}

	if set1 == LOWER && set2 == UPPER {
		return lowerToUpper(), nil
	}
	if set1 == UPPER && set2 == LOWER {
		return upperToLower(), nil
	}

	if (set2 == LOWER || set2 == UPPER) && (set1 != LOWER && set1 != UPPER) ||
		(set1 == LOWER && set2 == LOWER) || (set1 == UPPER && set2 == UPPER) {
		return nil, fmt.Errorf("misaligned [:upper:] and/or [:lower:] construct")
	}

	if _, ok := sets[set2]; ok {
		return nil, fmt.Errorf(`the only character classes that may appear in SET2 are 'upper' and 'lower'`)
	}

	arg2, err := unescape(set2)
	if err != nil {
		return nil, err
	}
	arg2, err = expandClasses(arg2)
	if err != nil {
		return nil, err
	}
	arg2, err = expandRepeats(arg2)
	if err != nil {
		return nil, err
	}
	arg2 = expandRanges(arg2)
	if len(arg2) == 0 {
		return nil, fmt.Errorf("SET2 must be non-empty")
	}
	inner := runesToRunes(arg1, arg2...)
	if _, ok := sets[set1]; ok {
		inner = setToRune(set1, arg2[0])
	}
	if !p.complement {
		return inner, nil
	}
	// Complement: every rune outside SET1 is translated. The index of a rune
	// inside the complement decides which SET2 rune it gets, counted in rune
	// order, which is how GNU cycles SET2.
	inside := make(map[rune]bool, len(arg1))
	for _, r := range arg1 {
		inside[r] = true
	}
	// A class as SET1 is not the literal text "[:digit:]": membership has to
	// come from the class test, or -cd '[:digit:]' would keep nothing and
	// delete the digits too (acmesh-49y).
	classSet, isClass := sets[set1]
	inSet := func(r rune) bool {
		if isClass {
			return classSet(r)
		}
		return inside[r]
	}
	sorted := append([]rune(nil), arg1...)
	slices.Sort(sorted)
	return &transformer{transform: func(r rune) rune {
		if inSet(r) {
			return r
		}
		if isClass {
			// Without a list of members there is no index into SET2; GNU
			// repeats the last character, which is all a class caller wants.
			return arg2[len(arg2)-1]
		}
		// SET2 is extended by repeating its last character, so everything
		// beyond the second character maps to it - GNU's long-standing rule.
		before := sort.Search(len(sorted), func(i int) bool { return sorted[i] >= r })
		if index := int(r) - before; index < len(arg2) {
			return arg2[index]
		}
		return arg2[len(arg2)-1]
	}}, nil
}

// expandRanges turns "a-z" into the runes between a and z. Without it a range
// is three literal characters, which is not what any caller means.
// expandRepeats turns the [c*n] form of SET2 into n copies of c, as GNU does:
// "tr a '[b*2]c'" maps a to b, not to c. An empty count means zero copies
// ("[b*]c" leaves c alone), and a char before the star is required - "[-*3]"
// and "[*3]" stay the literal text. A count that is not a number is refused with
// GNU's sentence.
func expandRepeats(in []rune) ([]rune, error) {
	out := make([]rune, 0, len(in))
	for i := 0; i < len(in); i++ {
		if i+2 < len(in) && in[i] == '[' && in[i+2] == '*' {
			end := i + 3
			for end < len(in) && in[end] != ']' {
				end++
			}
			if end < len(in) {
				count, err := repeatCount(string(in[i+3 : end]))
				if err != nil {
					return nil, err
				}
				for range count {
					out = append(out, in[i+1])
				}
				i = end
				continue
			}
		}
		out = append(out, in[i])
	}
	return out, nil
}

// repeatCount reads the n of [c*n]: empty means 0, digits are the count.
func repeatCount(s string) (int, error) {
	if s == "" {
		return 0, nil
	}
	n := 0
	for _, r := range s {
		if r < '0' || r > '9' {
			return 0, fmt.Errorf("invalid repeat count '%s' in [c*n] construct", s)
		}
		n = n*10 + int(r-'0')
		if n > 1<<20 {
			return 0, fmt.Errorf("invalid repeat count '%s' in [c*n] construct", s)
		}
	}
	return n, nil
}

func expandRanges(in []rune) []rune {
	out := make([]rune, 0, len(in))
	for i := 0; i < len(in); i++ {
		if i+2 < len(in) && in[i+1] == '-' {
			for r := in[i]; r <= in[i+2]; r++ {
				out = append(out, r)
			}
			i += 2
			continue
		}
		out = append(out, in[i])
	}
	return out
}

func unescape(s Set) ([]rune, error) {
	rs := []rune(s)
	var out []rune
	for i := 0; i < len(rs); i++ {
		r := rs[i]
		if r != '\\' {
			out = append(out, r)
			continue
		}
		i++
		if i >= len(rs) {
			// A lone backslash at the end stands for itself, as in GNU.
			out = append(out, '\\')
			break
		}
		c := rs[i]
		if c >= '0' && c <= '7' {
			// An octal escape: \0 is a NUL byte, \12 a newline, \101 an "A".
			// GNU reads up to three digits; the value is taken modulo 256
			// (measured 24.11.2026, acmesh-bly: "tr '\0' '@'" replaces NUL).
			v := int(c - '0')
			for k := 0; k < 2 && i+1 < len(rs) && rs[i+1] >= '0' && rs[i+1] <= '7'; k++ {
				i++
				v = v*8 + int(rs[i]-'0')
			}
			out = append(out, rune(byte(v)))
			continue
		}
		v, ok := escapeChars[c]
		if !ok {
			return nil, fmt.Errorf("unknown escape sequence '\\%c'", c)
		}
		out = append(out, v)
	}
	return out, nil
}

// classChars returns the characters of a named class in ascending order.
func classChars(name Set) ([]rune, bool) {
	f, ok := sets[name]
	if !ok {
		return nil, false
	}
	var out []rune
	for r := range rune(256) {
		if f(r) {
			out = append(out, r)
		}
	}
	return out, true
}

// expandClasses replaces every [:name:] inside a set with that class's
// characters. GNU honours a class wherever it appears, not only when the whole
// set is one class: "tr '[:upper:][:digit:]' 'a'" turns both A and 1 into "a"
// (measured 24.11.2026, acmesh-3qy).
func expandClasses(in []rune) ([]rune, error) {
	var out []rune
	for i := 0; i < len(in); i++ {
		if in[i] == '[' && i+1 < len(in) && in[i+1] == ':' {
			end := -1
			for j := i + 2; j+1 < len(in); j++ {
				if in[j] == ':' && in[j+1] == ']' {
					end = j
					break
				}
			}
			if end > 0 {
				name := Set(string(in[i : end+2]))
				chars, ok := classChars(name)
				if !ok {
					return nil, fmt.Errorf("invalid character class '%s'", string(in[i:end+2]))
				}
				out = append(out, chars...)
				i = end + 1
				continue
			}
		}
		out = append(out, in[i])
	}
	return out, nil
}

// params is what the command line asked for.
type params struct {
	del        bool
	squeeze    bool
	complement bool
}

// parseFlags reads -d, -s and -c, bundled as -ds or -cd like GNU allows.
func parseFlags(args []string) (params, []string, error) {
	var p params
	var rest []string
	for _, a := range args {
		switch {
		case a == "--":
			return p, append(rest, args[1:]...), nil
		case strings.HasPrefix(a, "--"):
			switch a {
			case "--delete":
				p.del = true
			case "--squeeze-repeats":
				p.squeeze = true
			case "--complement":
				p.complement = true
			default:
				return p, nil, fmt.Errorf("unknown option %q", a)
			}
		case strings.HasPrefix(a, "-") && len(a) > 1:
			for _, c := range a[1:] {
				switch c {
				case 'd':
					p.del = true
				case 's':
					p.squeeze = true
				case 'c', 'C':
					p.complement = true
				default:
					return p, nil, fmt.Errorf("unknown option %q", "-"+string(c))
				}
			}
		default:
			rest = append(rest, a)
		}
	}
	return p, rest, nil
}

// Run implements tr for the akmesh dispatcher.
func Run(args []string, stdin io.Reader, stdout, stderr io.Writer) int {
	p, rest, err := parseFlags(args)
	if err != nil {
		fmt.Fprintf(stderr, "tr: %v\n", err)
		return 1
	}
	c, err := command(stdin, stdout, rest, p)
	if err != nil {
		fmt.Fprintf(stderr, "tr: %v\n", err)
		return 1
	}
	if err := c.run(); err != nil {
		fmt.Fprintf(stderr, "tr: %v\n", err)
		return 1
	}
	return 0
}
