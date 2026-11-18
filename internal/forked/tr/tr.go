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
			return unicode.IsUpper(r)
		}
		if set == string(LOWER) {
			return unicode.IsLower(r)
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

var sets = map[Set]func(r rune) bool{
	ALNUM: func(r rune) bool {
		return unicode.IsLetter(r) || unicode.IsDigit(r)
	},

	ALPHA: unicode.IsLetter,
	DIGIT: unicode.IsDigit,
	GRAPH: unicode.IsGraphic,
	CNTRL: unicode.IsControl,
	PUNCT: unicode.IsPunct,
	SPACE: unicode.IsSpace,
	LOWER: unicode.IsLower,
	UPPER: unicode.IsUpper,
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
			return unicode.ToUpper(r)
		},
	}
}

func upperToLower() *transformer {
	return &transformer{
		transform: func(r rune) rune {
			return unicode.ToLower(r)
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
		return nil, fmt.Errorf("missing operand")
	case narg == 1 && del:
		// -d alone needs one SET; -ds needs two, as GNU says.
		if p.squeeze {
			return nil, fmt.Errorf("missing operand after '%s'\nTwo strings must be given when both deleting and squeezing repeats.\nTry 'tr --help' for more information.", args[0])
		}
	case narg == 1 && !del && !p.squeeze:
		return nil, fmt.Errorf("missing operand after '%s'\nTwo strings must be given when translating.\nTry 'tr --help' for more information.", args[0])
	case narg > 1 && del:
		return nil, fmt.Errorf("extra operand '%s'", args[0])
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
	sorted := append([]rune(nil), arg1...)
	sort.Slice(sorted, func(i, j int) bool { return sorted[i] < sorted[j] })
	return &transformer{transform: func(r rune) rune {
		if inside[r] {
			return r
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
	var out []rune
	var escape bool
	for _, r := range s {
		if escape {
			v, ok := escapeChars[r]
			if !ok {
				return nil, fmt.Errorf("unknown escape sequence '\\%c'", r)
			}
			out = append(out, v)
			escape = false
			continue
		}

		if r == '\\' {
			escape = true
			continue
		}

		out = append(out, r)
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

// Run implements tr for the acmesh dispatcher.
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
