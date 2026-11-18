// Copyright 2012-2017 the u-root Authors. All rights reserved
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.
//
// grep searches file contents using regular expressions.
//
// Forked from u-root 869f00f6b224ce02b1db51806159d6cd9c246f76, cmds/core/grep/grep.go.
// The rest of this file is the upstream source and keeps its copyright.
//
// Local changes, all of them for the way agents call grep:
//   - package grep and Run(args, stdin, stdout, stderr) instead of main();
//   - no flag package and no unixflag: the parser accepts combined short
//     options (-rn) and the GNU long names itself, so the single dependency of
//     this file can go;
//   - new: -E and -r as synonyms GNU knows, -w, -o, -A NUM, -B NUM, -C NUM;
//   - an invalid pattern is reported and exits 2 (upstream panicked through
//     regexp.MustCompile);
//   - the exit status follows GNU: 0 if something matched, 1 if nothing did,
//     2 on a broken pattern or an unreadable file (upstream returned 0 when
//     nothing matched and 1 only in quiet mode);
//   - -c counts per file, and with several files the count carries the name;
//   - context lines use "-" as the separator and groups are separated by "--",
//     as GNU prints them.
package grep

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
)

// params is what the command line asked for.
type params struct {
	patterns        []string
	invert          bool
	recursive       bool
	caseInsensitive bool
	fixed           bool
	noShowMatch     bool // -l
	quiet           bool // -q
	count           bool // -c
	number          bool // -n
	noFilename      bool // -h
	wordMatch       bool // -w
	onlyMatch       bool // -o
	before, after   int
	files           []string
}

func usage(w io.Writer) {
	fmt.Fprintln(w, "usage: grep [-E] [-F] [-i] [-v] [-c] [-n] [-l] [-q] [-h] [-r] [-R]")
	fmt.Fprintln(w, "            [-w] [-o] [-A NUM] [-B NUM] [-C NUM] [-e PATTERN] PATTERN [FILE...]")
}

// parse reads the argument list the way GNU does: combined short options, long
// names with their own value, and -e for a pattern that starts with a dash.
func parse(args []string) (*params, error) {
	p := &params{}
	var files []string
	needValue := func(i *int, name string) (string, error) {
		if *i+1 >= len(args) {
			return "", fmt.Errorf("option %s needs a value", name)
		}
		*i++
		return args[*i], nil
	}
	setDepth := func(kind string, value string) error {
		n, err := strconv.Atoi(value)
		if err != nil || n < 0 {
			return fmt.Errorf("%s: invalid context length %q", kind, value)
		}
		switch kind {
		case "-A":
			p.after = n
		case "-B":
			p.before = n
		default:
			p.after, p.before = n, n
		}
		return nil
	}
	for i := 0; i < len(args); i++ {
		a := args[i]
		switch {
		case a == "--":
			files = append(files, args[i+1:]...)
			i = len(args)
		case a == "-e" || a == "--regexp":
			v, err := needValue(&i, a)
			if err != nil {
				return nil, err
			}
			p.patterns = append(p.patterns, v)
		case strings.HasPrefix(a, "--regexp="):
			p.patterns = append(p.patterns, strings.TrimPrefix(a, "--regexp="))
		case a == "-A" || a == "--after-context":
			v, err := needValue(&i, a)
			if err != nil {
				return nil, err
			}
			if err := setDepth("-A", v); err != nil {
				return nil, err
			}
		case a == "-B" || a == "--before-context":
			v, err := needValue(&i, a)
			if err != nil {
				return nil, err
			}
			if err := setDepth("-B", v); err != nil {
				return nil, err
			}
		case a == "-C" || a == "--context":
			v, err := needValue(&i, a)
			if err != nil {
				return nil, err
			}
			if err := setDepth("-C", v); err != nil {
				return nil, err
			}
		case a == "--after-context=" || strings.HasPrefix(a, "--after-context="):
			if err := setDepth("-A", strings.TrimPrefix(a, "--after-context=")); err != nil {
				return nil, err
			}
		case strings.HasPrefix(a, "--before-context="):
			if err := setDepth("-B", strings.TrimPrefix(a, "--before-context=")); err != nil {
				return nil, err
			}
		case strings.HasPrefix(a, "--context="):
			if err := setDepth("-C", strings.TrimPrefix(a, "--context=")); err != nil {
				return nil, err
			}
		case strings.HasPrefix(a, "--"):
			switch a {
			case "--ignore-case":
				p.caseInsensitive = true
			case "--invert-match":
				p.invert = true
			case "--recursive":
				p.recursive = true
			case "--files-with-matches":
				p.noShowMatch = true
			case "--count":
				p.count = true
			case "--line-number":
				p.number = true
			case "--no-filename":
				p.noFilename = true
			case "--quiet", "--silent":
				p.quiet = true
			case "--fixed-strings":
				p.fixed = true
			case "--word-regexp":
				p.wordMatch = true
			case "--only-matching":
				p.onlyMatch = true
			case "--extended-regexp", "--basic-regexp":
				// Go's regexp is extended already; accepted for the same reason
				// as sed -E: command lines carry it.
			default:
				return nil, fmt.Errorf("unknown option %q", a)
			}
		case strings.HasPrefix(a, "-") && len(a) > 1:
			// Combined short flags, "-rn" like "-r -n".
			rest := a[1:]
			for j := 0; j < len(rest); j++ {
				switch rest[j] {
				case 'i':
					p.caseInsensitive = true
				case 'v':
					p.invert = true
				case 'r', 'R':
					p.recursive = true
				case 'l':
					p.noShowMatch = true
				case 'c':
					p.count = true
				case 'n':
					p.number = true
				case 'h':
					p.noFilename = true
				case 'q':
					p.quiet = true
				case 's', 'F':
					// -s (silent about errors) and -F (fixed strings)
					if rest[j] == 'F' {
						p.fixed = true
					}
				case 'w':
					p.wordMatch = true
				case 'o':
					p.onlyMatch = true
				case 'E', 'e':
					// -e takes the rest of the token or the next argument.
					if j+1 < len(rest) {
						p.patterns = append(p.patterns, rest[j+1:])
						j = len(rest)
					} else {
						v, err := needValue(&i, "-e")
						if err != nil {
							return nil, err
						}
						p.patterns = append(p.patterns, v)
					}
				case 'A', 'B', 'C':
					v := rest[j+1:]
					if v == "" {
						got, err := needValue(&i, "-"+string(rest[j]))
						if err != nil {
							return nil, err
						}
						v = got
					}
					if err := setDepth("-"+string(rest[j]), v); err != nil {
						return nil, err
					}
					j = len(rest)
				default:
					return nil, fmt.Errorf("unknown option %q", "-"+string(rest[j]))
				}
			}
		default:
			files = append(files, a)
		}
	}
	if len(p.patterns) == 0 {
		if len(files) == 0 {
			return nil, fmt.Errorf("no pattern given")
		}
		p.patterns, files = files[:1], files[1:]
	}
	p.files = files
	return p, nil
}

// compile turns the patterns into one matcher. -w wraps each pattern in word
// boundaries, -i adds the case flag, -F uses plain substring search.
func (p *params) compile() (*regexp.Regexp, error) {
	if p.fixed {
		quoted := make([]string, len(p.patterns))
		for i, pattern := range p.patterns {
			quoted[i] = regexp.QuoteMeta(pattern)
		}
		expr := strings.Join(quoted, "|")
		if p.wordMatch {
			expr = `\b(?:` + expr + `)\b`
		}
		if p.caseInsensitive {
			expr = "(?i)" + expr
		}
		return regexp.Compile(expr)
	}
	expr := strings.Join(p.patterns, "|")
	if p.wordMatch {
		expr = `\b(?:` + expr + `)\b`
	}
	if p.caseInsensitive {
		expr = "(?i)" + expr
	}
	return regexp.Compile(expr)
}

// printer writes the matches of one input.
type printer struct {
	p      *params
	re     *regexp.Regexp
	out    *bufio.Writer
	name   string
	show   bool
	before []string
	after  int
	last   int
	groups int
	hits   int
}

func (pr *printer) prefix(num int, context bool) {
	if pr.show {
		pr.out.WriteString(pr.name)
		if context {
			pr.out.WriteByte('-')
		} else {
			pr.out.WriteByte(':')
		}
	}
	if pr.p.number {
		pr.out.WriteString(strconv.Itoa(num))
		if context {
			pr.out.WriteByte('-')
		} else {
			pr.out.WriteByte(':')
		}
	}
}

func (pr *printer) emit(line string, num int, context bool) {
	if pr.p.count || pr.p.noShowMatch || pr.p.quiet {
		return
	}
	pr.prefix(num, context)
	pr.out.WriteString(line)
	pr.out.WriteByte('\n')
	pr.last = num
}

// scan reads one file (or stdin) and prints what matches.
func (pr *printer) scan(r io.Reader) error {
	sc := bufio.NewScanner(r)
	sc.Buffer(make([]byte, 0, 64*1024), 8*1024*1024)
	num := 0
	for sc.Scan() {
		num++
		line := sc.Text()
		match := pr.re.MatchString(line) != pr.p.invert
		if !match {
			if pr.after > 0 {
				// Inside the after-context of an earlier match.
				pr.emit(line, num, true)
				pr.after--
			}
			if pr.p.before > 0 {
				pr.before = append(pr.before, line)
				if len(pr.before) > pr.p.before {
					pr.before = pr.before[1:]
				}
			}
			continue
		}
		pr.hits++
		if pr.p.quiet {
			return nil
		}
		if pr.p.noShowMatch && !pr.p.count {
			// -l: the file name is printed once by the caller, one hit is enough.
			return nil
		}
		if pr.p.count {
			pr.after = 0
			pr.before = pr.before[:0]
			continue
		}
		// Only lines that were not already printed as after-context of the
		// previous match: those sit in the ring and would appear twice.
		first := num - len(pr.before)
		if pr.last > 0 && first <= pr.last {
			first = pr.last + 1
		}
		// A group is separated from the previous one only when a line is really
		// missing between them. Without context there are no groups to
		// separate, which is why GNU prints no "--" for plain -r.
		if (pr.p.before > 0 || pr.p.after > 0) && pr.groups > 0 && pr.last != 0 && first > pr.last+1 {
			fmt.Fprintln(pr.out, "--")
		}
		pr.groups++
		for i := first - (num - len(pr.before)); i < len(pr.before); i++ {
			pr.emit(pr.before[i], num-len(pr.before)+i, true)
		}
		pr.before = pr.before[:0]
		pr.after = pr.p.after
		if pr.p.onlyMatch {
			for _, m := range pr.re.FindAllString(line, -1) {
				pr.prefix(num, false)
				pr.out.WriteString(m)
				pr.out.WriteByte('\n')
			}
			pr.last = num
			continue
		}
		pr.prefix(num, false)
		pr.out.WriteString(line)
		pr.out.WriteByte('\n')
		pr.last = num
	}
	return sc.Err()
}

func (p *params) run(stdin io.Reader, stdout, stderr io.Writer) int {
	re, err := p.compile()
	if err != nil {
		fmt.Fprintf(stderr, "grep: %v\n", err)
		return 2
	}
	// -r names the file even for one start path, as GNU does.
	show := (len(p.files) > 1 || p.recursive) && !p.noFilename
	code := 1
	matched := func(pr *printer) {
		if pr.hits > 0 {
			code = 0
		}
	}

	if len(p.files) == 0 {
		pr := &printer{p: p, re: re, out: bufio.NewWriter(stdout), name: "(standard input)", show: false}
		if err := pr.scan(stdin); err != nil {
			fmt.Fprintf(stderr, "grep: %v\n", err)
		}
		pr.out.Flush()
		if p.count {
			fmt.Fprintf(stdout, "%d\n", pr.hits)
		}
		matched(pr)
		return code
	}

	walkOne := func(name string) error {
		pr := &printer{p: p, re: re, out: bufio.NewWriter(stdout), name: name, show: show}
		f, err := os.Open(name)
		if err != nil {
			return err
		}
		defer f.Close()
		pr.name = name
		if err := pr.scan(f); err != nil {
			return err
		}
		pr.out.Flush()
		if p.count {
			if show {
				fmt.Fprintf(stdout, "%s:%d\n", name, pr.hits)
			} else {
				fmt.Fprintf(stdout, "%d\n", pr.hits)
			}
		}
		matched(pr)
		return nil
	}

	for _, path := range p.files {
		info, err := os.Stat(path)
		if err != nil {
			fmt.Fprintf(stderr, "grep: %v\n", err)
			code = 2
			continue
		}
		if !info.IsDir() {
			if err := walkOne(path); err != nil {
				fmt.Fprintf(stderr, "grep: %v\n", err)
				code = 2
			}
			continue
		}
		if !p.recursive {
			fmt.Fprintf(stderr, "grep: %s: Is a directory\n", path)
			code = 2
			continue
		}
		err = filepath.Walk(path, func(name string, fi os.FileInfo, err error) error {
			if path == "." && fi != nil && name != "." {
				name = "./" + name
			}
			if err != nil {
				fmt.Fprintf(stderr, "grep: %v\n", err)
				return nil
			}
			if fi.IsDir() {
				return nil
			}
			if err := walkOne(name); err != nil {
				fmt.Fprintf(stderr, "grep: %v\n", err)
				code = 2
			}
			return nil
		})
		if err != nil {
			fmt.Fprintf(stderr, "grep: %v\n", err)
			code = 2
		}
	}
	return code
}

// Run implements grep for the acmesh dispatcher.
func Run(args []string, stdin io.Reader, stdout, stderr io.Writer) int {
	p, err := parse(args)
	if err != nil {
		fmt.Fprintf(stderr, "grep: %v\n", err)
		usage(stderr)
		return 2
	}
	return p.run(stdin, stdout, stderr)
}
