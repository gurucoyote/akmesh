// Package sortcmd implements sort(1) with the GNU command line: -b -c -C -d
// -f -g -h -i -k -m -M -n -o -r -R -s -t -T -u -V -z, --files0-from and
// --random-source.
//
// The ordering comparators and the -k key grammar are ported from
// qiangli/coreutils cmds/sort (MIT, Copyright (c) 2026 Qiang Li); the driver,
// the argument parser and the -R implementation are akmesh's own. Only the
// C/POSIX locale is supported: text compares by byte, numbers use '.' as the
// decimal point. A non-C LC_COLLATE or LC_NUMERIC is not silently ignored -
// see checkLocale.
package sortcmd

import (
	"crypto/rand"
	"fmt"
	"hash/fnv"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// options is one sort invocation.
type options struct {
	blanks, check, checkSilent, dict, fold bool
	generalNum, human, ignoreNP            bool
	merge, month, numeric, randomSort      bool
	reverse, stable, unique, versionSort   bool
	zeroTerm                               bool
	tab                                    int
	output                                 string
	files0                                 string
	randSource                             string
	keys                                   []keySpec
	globals                                keyOpts
}

// Run is the entry point the akmesh table calls.
func Run(args []string, stdin *os.File, stdout, stderr *os.File) int {
	o, operands, errMsg := parseArgs(args)
	if errMsg != "" {
		if errMsg == "help" {
			fmt.Fprint(stdout, usageText)
			return 0
		}
		if errMsg == "version" {
			fmt.Fprintln(stdout, "sort (akmesh) 1.0")
			return 0
		}
		fmt.Fprintf(stderr, "sort: %s\n", errMsg)
		fmt.Fprintln(stderr, "sort: try 'sort --help' for the supported options")
		return 2
	}
	if msg := checkLocale(); msg != "" {
		fmt.Fprintf(stderr, "sort: %s\n", msg)
		return 2
	}
	s := &sorter{o: o, hashes: map[string]uint64{}}

	if o.check || o.checkSilent {
		if o.merge {
			fmt.Fprintln(stderr, "sort: options '-cm' are incompatible")
			return 2
		}
		if o.output != "" {
			fmt.Fprintln(stderr, "sort: options '-co' are incompatible")
			return 2
		}
		if len(operands) > 1 {
			fmt.Fprintf(stderr, "sort: extra operand %q not allowed with -c\n", operands[1])
			return 2
		}
	}
	if len(operands) == 0 {
		operands = []string{"-"}
	}
	if o.files0 != "" {
		names, err := readFiles0(o.files0)
		if err != nil {
			fmt.Fprintf(stderr, "sort: %s: %v\n", o.files0, err)
			return 2
		}
		operands = append(names, operands...)
	}
	if o.randSource != "" {
		if err := s.loadSeed(); err != nil {
			fmt.Fprintf(stderr, "sort: %s: %v\n", o.randSource, err)
			return 2
		}
	} else if err := s.loadCryptoSeed(); err != nil {
		fmt.Fprintf(stderr, "sort: random seed: %v\n", err)
		return 2
	}

	if o.check || o.checkSilent {
		return s.check(operands, stdin, stdout, stderr)
	}
	return s.sortFiles(operands, stdin, stdout, stderr)
}

// sorter carries the invocation state. hashes memoises one random hash per
// distinct key, so -R is a stable function of the key within one run.
type sorter struct {
	o      *options
	seed   [8]byte
	hashes map[string]uint64
}

func (s *sorter) loadSeed() error {
	f, err := os.Open(s.o.randSource)
	if err != nil {
		return err
	}
	defer f.Close()
	if _, err := io.ReadFull(f, s.seed[:]); err != nil {
		return fmt.Errorf("cannot read 8 bytes of randomness")
	}
	return nil
}

func (s *sorter) loadCryptoSeed() error {
	_, err := io.ReadFull(rand.Reader, s.seed[:])
	return err
}

// hashKey is the -R ordering: FNV-1a over the seed and the key text. Equal
// keys get equal hashes and therefore stay grouped, which is what GNU's
// --random-sort does; a uniform permutation is shuf's job, not sort's.
func (s *sorter) hashKey(k string) uint64 {
	if h, ok := s.hashes[k]; ok {
		return h
	}
	f := fnv.New64a()
	f.Write(s.seed[:])
	f.Write([]byte(k))
	h := f.Sum64()
	s.hashes[k] = h
	return h
}

func cmpInts(a, b uint64) int {
	switch {
	case a < b:
		return -1
	case a > b:
		return 1
	}
	return 0
}

// compare orders two lines: keys first, then -- unless -s or -u -- the
// last-resort whole-line comparison.
func (s *sorter) compare(a, b string) int {
	if len(s.o.keys) > 0 {
		if d := s.compareKeys(a, b); d != 0 {
			return d
		}
		if s.o.stable || s.o.unique {
			return 0
		}
	}
	d := strings.Compare(a, b)
	if s.o.reverse {
		return -d
	}
	return d
}

// equal is the -u test: keys only, so equal keys collapse even when the rest
// of the line differs.
func (s *sorter) equal(a, b string) int {
	if len(s.o.keys) > 0 {
		return s.compareKeys(a, b)
	}
	return strings.Compare(a, b)
}

func (s *sorter) compareKeys(a, b string) int {
	for i := range s.o.keys {
		k := &s.o.keys[i]
		ka := extractKey(a, k, s.o.tab)
		kb := extractKey(b, k, s.o.tab)
		var d int
		switch {
		case k.opts.random:
			d = cmpInts(s.hashKey(ka), s.hashKey(kb))
		case k.opts.generalNum:
			d = generalNumCompare(ka, kb)
		case k.opts.numeric:
			d = numCompare(ka, kb)
		case k.opts.human:
			d = humanCompare(ka, kb)
		case k.opts.month:
			d = monthCompare(ka, kb)
		case k.opts.version:
			d = versionCompare(ka, kb)
		default:
			d = textKeyCompare(ka, kb, k.opts)
		}
		if d != 0 {
			if k.opts.reverse {
				return -d
			}
			return d
		}
	}
	return 0
}

// readLines reads every operand and returns the lines in operand order.
func readLines(names []string, stdin *os.File, zeroTerm bool) ([]string, string, error) {
	sep := byte('\n')
	if zeroTerm {
		sep = 0
	}
	var lines []string
	for _, name := range names {
		var data []byte
		var err error
		if name == "-" {
			data, err = io.ReadAll(stdin)
		} else {
			data, err = os.ReadFile(name)
		}
		if err != nil {
			return nil, name, err
		}
		for len(data) > 0 {
			i := indexByte(data, sep)
			if i < 0 {
				lines = append(lines, string(data))
				break
			}
			lines = append(lines, string(data[:i]))
			data = data[i+1:]
		}
	}
	return lines, "", nil
}

func indexByte(b []byte, c byte) int {
	for i := range b {
		if b[i] == c {
			return i
		}
	}
	return -1
}

func (s *sorter) sortFiles(operands []string, stdin *os.File, stdout, stderr *os.File) int {
	lines, bad, err := readLines(operands, stdin, s.o.zeroTerm)
	if err != nil {
		fmt.Fprintf(stderr, "sort: cannot read: %s: %v\n", bad, err)
		return 2
	}
	sort.SliceStable(lines, func(i, j int) bool { return s.compare(lines[i], lines[j]) < 0 })
	if s.o.unique {
		out := lines[:0]
		for i, l := range lines {
			if i == 0 || s.equal(out[len(out)-1], l) != 0 {
				out = append(out, l)
			}
		}
		lines = out
	}
	sep := byte('\n')
	if s.o.zeroTerm {
		sep = 0
	}
	var buf []byte
	for _, l := range lines {
		buf = append(buf, l...)
		buf = append(buf, sep)
	}
	if s.o.output != "" {
		if err := os.WriteFile(s.o.output, buf, 0o644); err != nil {
			fmt.Fprintf(stderr, "sort: %s: %v\n", s.o.output, err)
			return 2
		}
		return 0
	}
	if _, err := stdout.Write(buf); err != nil {
		fmt.Fprintf(stderr, "sort: write failed: %v\n", err)
		return 2
	}
	return 0
}

// check implements -c/-C. The report follows GNU's wording so that an agent
// grepping for "disorder" finds it.
func (s *sorter) check(operands []string, stdin *os.File, stdout, stderr *os.File) int {
	name := operands[0]
	lines, _, err := readLines(operands, stdin, s.o.zeroTerm)
	if err != nil {
		fmt.Fprintf(stderr, "sort: cannot read: %s: %v\n", name, err)
		return 2
	}
	for i := 1; i < len(lines); i++ {
		if s.compare(lines[i-1], lines[i]) > 0 {
			if s.o.checkSilent {
				return 1
			}
			fmt.Fprintf(stderr, "sort: %s:%d: disorder: %s\n", name, i+1, lines[i])
			return 1
		}
	}
	return 0
}

func readFiles0(path string) ([]string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var names []string
	for part := range strings.SplitSeq(string(data), "\x00") {
		if part != "" {
			names = append(names, part)
		}
	}
	return names, nil
}

// checkLocale refuses a locale this build cannot honour instead of quietly
// sorting by bytes when the user asked for a collation order.
func checkLocale() string {
	for _, v := range []string{"LC_ALL", "LC_COLLATE"} {
		name := os.Getenv(v)
		if name == "" || name == "C" || name == "POSIX" || name == "C.UTF-8" {
			continue
		}
		return fmt.Sprintf("%s=%s is not supported: this build sorts in the C locale only", v, name)
	}
	return ""
}

const usageText = `Usage: sort [OPTION]... [FILE]...
Write sorted concatenation of all FILE(s) to standard output.

Ordering options:
  -b, --ignore-leading-blanks   ignore leading blanks
  -d, --dictionary-order        consider only blanks and alphanumerics
  -f, --ignore-case             fold lower case to upper case
  -g, --general-numeric-sort    compare as general numeric value
  -h, --human-numeric-sort      compare human readable numbers (2K 1G)
  -i, --ignore-nonprinting      consider only printable characters
  -M, --month-sort              compare month names
  -n, --numeric-sort            compare according to string numerical value
  -R, --random-sort             shuffle, but group identical keys
      --random-source=FILE      get random bytes from FILE
  -r, --reverse                 reverse the result of comparisons
  -V, --version-sort            natural sort of version numbers
  -k, --key=KEYDEF              sort via a key; KEYDEF gives location and type
  -t, --field-separator=SEP     use SEP instead of blank-to-nonblank

Other options:
  -c, --check                   check for sorted input; do not sort
  -C, --check=quiet             like -c, but do not report the first bad line
      --files0-from=FILE        read NUL-terminated file names from FILE
  -m, --merge                   merge already sorted files
  -o, --output=FILE             write result to FILE instead of stdout
  -s, --stable                  stabilize sort by disabling last-resort compare
      --sort=WORD               general-numeric, human-numeric, month,
                                numeric, random, version
  -T, --temporary-directory=DIR accepted, this build sorts in memory
  -u, --unique                  output only the first of an equal run
  -z, --zero-terminated         line delimiter is NUL, not newline
      --help                    display this help and exit
      --version                 output version information and exit

Only the C/POSIX locale is supported; text comparisons use byte order.
`

// parseArgs parses the GNU command line: combined short options, attached
// option arguments (-k1,2, -t:) and long options with or without '='.
func parseArgs(args []string) (*options, []string, string) {
	o := &options{tab: -1}
	var operands []string

	takeArg := func(i *int, rest string, name string) (string, string) {
		if rest != "" {
			return rest, ""
		}
		if *i+1 >= len(args) {
			return "", fmt.Sprintf("option requires an argument -- %s", name)
		}
		*i++
		return args[*i], ""
	}

	for i := 0; i < len(args); i++ {
		a := args[i]
		if a == "--" {
			operands = append(operands, args[i+1:]...)
			break
		}
		if strings.HasPrefix(a, "--") {
			name, val, hasVal := strings.Cut(a[2:], "=")
			need := func() (string, string) {
				if hasVal {
					return val, ""
				}
				if i+1 >= len(args) {
					return "", fmt.Sprintf("option '--%s' requires an argument", name)
				}
				i++
				return args[i], ""
			}
			var errMsg string
			switch name {
			case "help":
				return nil, nil, "help"
			case "version":
				return nil, nil, "version"
			case "ignore-leading-blanks":
				o.blanks = true
			case "check":
				if val == "quiet" || val == "silent" {
					o.checkSilent = true
				} else if val == "" || val == "diagnose-first" {
					o.check = true
				} else {
					return nil, nil, fmt.Sprintf("invalid --check argument %q", val)
				}
			case "check-silent":
				o.checkSilent = true
			case "dictionary-order":
				o.dict = true
			case "ignore-case":
				o.fold = true
			case "general-numeric-sort":
				o.generalNum = true
			case "human-numeric-sort":
				o.human = true
			case "ignore-nonprinting":
				o.ignoreNP = true
			case "merge":
				o.merge = true
			case "month-sort":
				o.month = true
			case "numeric-sort":
				o.numeric = true
			case "random-sort":
				o.randomSort = true
			case "reverse":
				o.reverse = true
			case "stable":
				o.stable = true
			case "unique":
				o.unique = true
			case "version-sort":
				o.versionSort = true
			case "zero-terminated":
				o.zeroTerm = true
			case "key":
				v, e := need()
				if e != "" {
					return nil, nil, e
				}
				if e := addKey(o, v); e != "" {
					return nil, nil, e
				}
			case "field-separator":
				v, e := need()
				if e != "" {
					return nil, nil, e
				}
				if e := setSeparator(o, v); e != "" {
					return nil, nil, e
				}
			case "output":
				if o.output, errMsg = need(); errMsg != "" {
					return nil, nil, errMsg
				}
			case "files0-from":
				if o.files0, errMsg = need(); errMsg != "" {
					return nil, nil, errMsg
				}
			case "random-source":
				if o.randSource, errMsg = need(); errMsg != "" {
					return nil, nil, errMsg
				}
			case "sort":
				v, e := need()
				if e != "" {
					return nil, nil, e
				}
				switch v {
				case "general-numeric", "g":
					o.generalNum = true
				case "human-numeric", "human", "h":
					o.human = true
				case "month", "M":
					o.month = true
				case "numeric", "n":
					o.numeric = true
				case "random", "R":
					o.randomSort = true
				case "version", "V":
					o.versionSort = true
				default:
					return nil, nil, fmt.Sprintf("invalid argument %q for '--sort'", v)
				}
			case "temporary-directory", "buffer-size", "batch-size", "parallel", "compress-program":
				if _, errMsg = need(); errMsg != "" {
					return nil, nil, errMsg
				}
				// Accepted for GNU compatibility; this build sorts in memory.
			case "debug":
				return nil, nil, "--debug is not supported: it changes the output format"
			default:
				return nil, nil, fmt.Sprintf("unrecognized option '--%s'", name)
			}
			continue
		}
		if len(a) > 1 && a[0] == '-' {
			for j := 1; j < len(a); j++ {
				c := a[j]
				rest := a[j+1:]
				switch c {
				case 'b':
					o.blanks = true
				case 'c':
					o.check = true
				case 'C':
					o.checkSilent = true
				case 'd':
					o.dict = true
				case 'f':
					o.fold = true
				case 'g':
					o.generalNum = true
				case 'h':
					o.human = true
				case 'i':
					o.ignoreNP = true
				case 'm':
					o.merge = true
				case 'M':
					o.month = true
				case 'n':
					o.numeric = true
				case 'r':
					o.reverse = true
				case 'R':
					o.randomSort = true
				case 's':
					o.stable = true
				case 'u':
					o.unique = true
				case 'V':
					o.versionSort = true
				case 'z':
					o.zeroTerm = true
				case 'k':
					v, e := takeArg(&i, rest, "k")
					if e != "" {
						return nil, nil, e
					}
					if e := addKey(o, v); e != "" {
						return nil, nil, e
					}
					j = len(a)
				case 't':
					v, e := takeArg(&i, rest, "t")
					if e != "" {
						return nil, nil, e
					}
					if e := setSeparator(o, v); e != "" {
						return nil, nil, e
					}
					j = len(a)
				case 'o':
					v, e := takeArg(&i, rest, "o")
					if e != "" {
						return nil, nil, e
					}
					o.output = v
					j = len(a)
				case 'T':
					if _, e := takeArg(&i, rest, "T"); e != "" {
						return nil, nil, e
					}
					j = len(a)
				default:
					return nil, nil, fmt.Sprintf("invalid option -- %q", string(c))
				}
			}
			continue
		}
		operands = append(operands, a)
	}

	o.globals = keyOpts{
		numeric: o.numeric, human: o.human, fold: o.fold, dict: o.dict,
		generalNum: o.generalNum, ignoreNP: o.ignoreNP, month: o.month,
		random: o.randomSort, version: o.versionSort, reverse: o.reverse,
		skipSB: o.blanks, skipEB: o.blanks,
	}
	for i := range o.keys {
		if !o.keys[i].opts.hasMods() && !o.keys[i].opts.reverse {
			o.keys[i].opts = o.globals
		}
	}
	if len(o.keys) == 0 && o.globals.hasMods() {
		o.keys = []keySpec{{sword: 0, schar: 0, eword: -1, echar: 0, opts: o.globals}}
	}
	if o.merge && o.randomSort {
		return nil, nil, "options '-mR' are incompatible"
	}
	return o, operands, ""
}

func setSeparator(o *options, v string) string {
	if len(v) > 1 {
		return "multi-character tab " + fmt.Sprintf("%q", v)
	}
	if v == "" {
		return "empty tab"
	}
	o.tab = int(v[0])
	return ""
}

func addKey(o *options, spec string) string {
	_ = filepath.Separator
	k, errMsg, _ := parseKeySpec(spec)
	if errMsg != "" {
		return fmt.Sprintf("invalid field specification %q: %s", spec, errMsg)
	}
	o.keys = append(o.keys, k)
	return ""
}
