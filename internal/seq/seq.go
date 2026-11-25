// Package seq implements seq, printing a sequence of numbers, the GNU way.
// seq prints a sequence of numbers the way GNU seq does.
//
// akmesh's own implementation: the u-root command rejects "-s," (the flag
// package reads it as an unknown flag), writes Go's raw number formatting, and
// reports a bad number with a timestamp (measured 24.11.2026, acmesh-efg).
//
// GNU prints the numbers with the precision of its arguments: "seq 1.0 3"
// gives 1.0 2.0 3.0, "seq 0.5 0.5 2" gives 0.5 1.0 1.5 2.0, and only plain
// integers when every argument is one.
package seq

import (
	"fmt"
	"io"
	"math"
	"strconv"
	"strings"
)

// decimals counts the digits after the decimal point in a literal.
func decimals(s string) int {
	if i := strings.IndexAny(s, "."); i >= 0 {
		return len(s) - i - 1
	}
	return 0
}

// formatNumber writes v with exactly d decimals.
func formatNumber(v float64, d int) string {
	return strconv.FormatFloat(v, 'f', d, 64)
}

// padZero pads s with leading zeros to n characters, after the minus sign.
func padZero(s string, n int) string {
	if len(s) >= n {
		return s
	}
	zeros := strings.Repeat("0", n-len(s))
	if strings.HasPrefix(s, "-") {
		return "-" + zeros + s[1:]
	}
	return zeros + s
}

// applyFormat renders one number through the -f format. Go's fmt takes the
// same verbs as printf, but a zero flag on %g (GNU's "x%02g") is not allowed
// there, so it is done here: the number is formatted and padded by hand.
func applyFormat(format string, v float64) string {
	i := strings.IndexByte(format, '%')
	if i < 0 {
		return format
	}
	tail := format[i:]
	end := strings.IndexAny(tail[1:], "feEgGaA")
	if end < 0 {
		return fmt.Sprintf(format, v)
	}
	spec := tail[:end+2]
	verb := spec[len(spec)-1]
	if verb == 'g' || verb == 'G' {
		body := strings.TrimPrefix(strings.TrimSuffix(spec, string(verb)), "%")
		zero := strings.Contains(body, "0") && !strings.Contains(body, ".")
		num := strconv.FormatFloat(v, 'g', -1, 64)
		if zero {
			w := 0
			for _, r := range body {
				if r >= '1' && r <= '9' {
					w = w*10 + int(r-'0')
				}
			}
			num = padZero(num, w)
		}
		return format[:i] + num + format[i+len(spec):]
	}
	return format[:i] + fmt.Sprintf(spec, v) + format[i+len(spec):]
}

// Run is the command.
func Run(args []string, stdin io.Reader, stdout, stderr io.Writer) int {
	sep := "\n"
	format := ""
	sameWidth := false
	var operands []string
	bad := func(err error) int {
		fmt.Fprintf(stderr, "seq: %v\n", err)
		fmt.Fprintf(stderr, "Try 'seq --help' for more information.\n")
		return 1
	}
	take := func(i *int, glued, opt string) (string, error) {
		if glued != "" {
			return glued, nil
		}
		*i++
		if *i >= len(args) {
			return "", fmt.Errorf("option requires an argument -- '%s'", opt)
		}
		return args[*i], nil
	}
	for i := 0; i < len(args); i++ {
		a := args[i]
		switch {
		case a == "--":
			operands = append(operands, args[i+1:]...)
			i = len(args)
		case a == "--help":
			fmt.Fprint(stdout, `Usage: seq [OPTION]... LAST
  or:  seq [OPTION]... FIRST LAST
  or:  seq [OPTION]... FIRST INCREMENT LAST
Print numbers from FIRST to LAST, in steps of INCREMENT.

  -f, --format=FORMAT      use printf style floating point FORMAT
  -s, --separator=STRING   separate numbers with STRING (default newline)
  -w, --equal-width        pad numbers with leading zeros to equal width
`)
			return 0
		case a == "-w" || a == "--equal-width":
			sameWidth = true
		case a == "-s" || a == "--separator" || strings.HasPrefix(a, "--separator=") || (strings.HasPrefix(a, "-s") && len(a) > 2):
			glued := ""
			switch {
			case strings.HasPrefix(a, "--separator="):
				glued = strings.TrimPrefix(a, "--separator=")
			case a != "-s":
				glued = a[2:]
			}
			v, err := take(&i, glued, "s")
			if err != nil {
				return bad(err)
			}
			sep = v
		case a == "-f" || a == "--format" || strings.HasPrefix(a, "--format=") || (strings.HasPrefix(a, "-f") && len(a) > 2):
			glued := ""
			switch {
			case strings.HasPrefix(a, "--format="):
				glued = strings.TrimPrefix(a, "--format=")
			case a != "-f":
				glued = a[2:]
			}
			v, err := take(&i, glued, "f")
			if err != nil {
				return bad(err)
			}
			format = v
		case len(a) > 1 && a[0] == '-' && a != "-" && !isNumber(a):
			return bad(fmt.Errorf("invalid option -- '%s'", strings.TrimPrefix(a, "-")))
		default:
			operands = append(operands, a)
		}
	}
	if len(operands) == 0 {
		return bad(fmt.Errorf("missing operand"))
	}
	if len(operands) > 3 {
		return bad(fmt.Errorf("extra operand: '%s'", operands[3]))
	}
	first, step, last := 1.0, 1.0, 0.0
	var d int
	texts := operands
	switch len(operands) {
	case 1:
		v, err := parse(operands[0])
		if err != nil {
			return bad(err)
		}
		last = v
		d = decimals(operands[0])
	case 2:
		f, err := parse(operands[0])
		if err != nil {
			return bad(err)
		}
		l, err := parse(operands[1])
		if err != nil {
			return bad(err)
		}
		first, last = f, l
		d = max(decimals(operands[0]), decimals(operands[1]))
	default:
		f, err := parse(operands[0])
		if err != nil {
			return bad(err)
		}
		s, err := parse(operands[1])
		if err != nil {
			return bad(err)
		}
		l, err := parse(operands[2])
		if err != nil {
			return bad(err)
		}
		first, step, last = f, s, l
		d = max(decimals(operands[0]), max(decimals(operands[1]), decimals(operands[2])))
	}

	// The width that -w pads to: the longest of the endpoints as printed.
	w := 0
	if sameWidth {
		for _, t := range texts {
			if n, err := parse(t); err == nil {
				if l := len(formatNumber(n, d)); l > w {
					w = l
				}
			}
		}
	}
	render := func(v float64) string {
		if format != "" {
			return applyFormat(format, v)
		}
		s := formatNumber(v, d)
		if sameWidth {
			s = padZero(s, w)
		}
		return s
	}

	// float steps drift; run on the count of steps and multiply, as GNU does.
	if step == 0 {
		return bad(fmt.Errorf("invalid Zero increment value: '0'"))
	}
	n := max(int(math.Floor((last-first)/step+1e-9))+1, 0)
	var b strings.Builder
	for i := range n {
		if i > 0 {
			b.WriteString(sep)
		}
		b.WriteString(render(first + float64(i)*step))
	}
	if n > 0 {
		b.WriteString("\n")
	}
	fmt.Fprint(stdout, b.String())
	return 0
}

// parse reads a number the way GNU reads one, with its own error text.
func parse(s string) (float64, error) {
	v, err := strconv.ParseFloat(s, 64)
	if err != nil {
		return 0, fmt.Errorf("invalid floating point argument: '%s'", s)
	}
	return v, nil
}

// isNumber reports whether s is a number, so that "-3" is an operand and not
// an option.
func isNumber(s string) bool {
	if s == "" {
		return false
	}
	_, err := strconv.ParseFloat(s, 64)
	return err == nil
}
