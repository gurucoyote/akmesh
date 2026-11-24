package rgcmd

import (
	"fmt"
	"strconv"
	"strings"
)

// config is what the command line asks for, already resolved: no pointer
// fields, no "was it set" questions left for the search loop. The names and
// the defaults follow ripgrep's own flags, because an agent types those.
type config struct {
	patterns []string // -e, repeatable; without -e the first positional is the pattern
	paths    []string

	ignoreCase  bool
	smartCase   bool
	fixed       bool
	word        bool
	lineRegexp  bool
	invert      bool
	lineNumbers bool
	noFilename  bool

	filesWithMatches bool   // -l
	count            bool   // -c
	onlyMatching     bool   // -o
	files            bool   // --files
	pcre             bool   // -P, --pcre2
	engine           string // --engine default|pcre2|auto

	before   int
	after    int
	maxCount int
	maxDepth int

	globs    []string // -g
	iglobs   []string // --iglob
	types    []string // -t
	typesNot []string // -T

	hidden   bool
	noIgnore bool
	workers  int

	pre        bool
	preCommand string
	preGlob    []string

	// decompress/-z walks archives and compressed files; zmax is the number
	// of expansion stages, ugrep's default of 1 (1..99).
	decompress bool
	zmax       int
}

// parse reads args like the tool the agent already knows. The dialect rules
// that matter in practice: one-letter flags combine (-in, -cF), a value flag
// takes the rest of its cluster or the next argument (-A3, -A 3), and a long
// flag takes =value or the next argument (--glob=*.go).
func parse(args []string) (config, error) {
	cfg := config{zmax: 1}
	var positional []string

	// values are the single-letter flags that consume a value.
	takeValue := func(name string, rest string, i *int) (string, error) {
		if rest != "" {
			return strings.TrimPrefix(rest, "="), nil
		}
		if *i+1 >= len(args) {
			return "", fmt.Errorf("flag %s needs a value", name)
		}
		*i++
		return args[*i], nil
	}

	for i := 0; i < len(args); i++ {
		a := args[i]
		switch {
		case a == "--":
			positional = append(positional, args[i+1:]...)
			i = len(args)
		case strings.HasPrefix(a, "--"):
			name, rest, hasValue := strings.Cut(a[2:], "=")
			value := func() (string, error) {
				if hasValue {
					return rest, nil
				}
				if i+1 >= len(args) {
					return "", fmt.Errorf("flag --%s needs a value", name)
				}
				i++
				return args[i], nil
			}
			var err error
			switch name {
			case "ignore-case":
				cfg.ignoreCase = true
			case "smart-case":
				cfg.smartCase = true
			case "fixed-strings":
				cfg.fixed = true
			case "word-regexp":
				cfg.word = true
			case "line-regexp":
				cfg.lineRegexp = true
			case "invert-match":
				cfg.invert = true
			case "line-number":
				cfg.lineNumbers = true
			case "no-line-number":
				cfg.lineNumbers = false
			case "no-filename":
				cfg.noFilename = true
			case "with-filename":
				cfg.noFilename = false
			case "files-with-matches":
				cfg.filesWithMatches = true
			case "count":
				cfg.count = true
			case "count-matches":
				cfg.count = true
			case "only-matching":
				cfg.onlyMatching = true
			case "files":
				cfg.files = true
			case "pcre2":
				cfg.pcre = true
			case "engine":
				var v string
				if v, err = value(); err == nil {
					switch v {
					case "default", "pcre2", "auto":
						cfg.engine = v
					default:
						err = fmt.Errorf("invalid value %q for --engine: expected default, pcre2 or auto", v)
					}
				}
			case "hidden":
				cfg.hidden = true
			case "no-ignore":
				cfg.noIgnore = true
			case "after-context":
				var v string
				if v, err = value(); err == nil {
					cfg.after, err = intValue("--after-context", v)
				}
			case "before-context":
				var v string
				if v, err = value(); err == nil {
					cfg.before, err = intValue("--before-context", v)
				}
			case "context":
				var v string
				if v, err = value(); err == nil {
					var n int
					n, err = intValue("--context", v)
					cfg.before, cfg.after = n, n
				}
			case "max-count":
				var v string
				if v, err = value(); err == nil {
					cfg.maxCount, err = intValue("--max-count", v)
				}
			case "max-depth":
				var v string
				if v, err = value(); err == nil {
					cfg.maxDepth, err = intValue("--max-depth", v)
				}
			case "glob":
				var v string
				if v, err = value(); err == nil {
					cfg.globs = append(cfg.globs, v)
				}
			case "iglob":
				var v string
				if v, err = value(); err == nil {
					cfg.iglobs = append(cfg.iglobs, v)
				}
			case "type":
				var v string
				if v, err = value(); err == nil {
					cfg.types = append(cfg.types, v)
				}
			case "type-not":
				var v string
				if v, err = value(); err == nil {
					cfg.typesNot = append(cfg.typesNot, v)
				}
			case "threads":
				var v string
				if v, err = value(); err == nil {
					cfg.workers, err = intValue("--threads", v)
				}
			case "pre":
				cfg.pre = true
				cfg.preCommand = "any2md"
				if hasValue {
					if rest != "" {
						cfg.preCommand = rest
					}
				} else if i+1 < len(args) && !strings.HasPrefix(args[i+1], "-") {
					// rg's own shape: --pre COMMAND. The built-in name
					// "any2md" is recognized, like any other command.
					i++
					cfg.preCommand = args[i]
				}
			case "pre-glob":
				var v string
				if v, err = value(); err == nil {
					cfg.preGlob = append(cfg.preGlob, v)
				}
			case "decompress":
				cfg.decompress = true
			case "zmax":
				var v string
				if v, err = value(); err == nil {
					cfg.zmax, err = zmaxValue(v)
				}
			default:
				return cfg, fmt.Errorf("unrecognized flag --%s", name)
			}
			if err != nil {
				return cfg, err
			}
		case len(a) > 1 && a[0] == '-':
			cluster := a[1:]
			for j := 0; j < len(cluster); j++ {
				c := cluster[j]
				rest := cluster[j+1:]
				switch c {
				case 'i':
					cfg.ignoreCase = true
				case 'S':
					cfg.smartCase = true
				case 'F':
					cfg.fixed = true
				case 'P':
					// -P/--pcre2: search with PCRE2, which ripgrep only does
					// with the pcre2 feature compiled in.
					cfg.pcre = true
				case 'w':
					cfg.word = true
				case 'x':
					cfg.lineRegexp = true
				case 'v':
					cfg.invert = true
				case 'z':
					cfg.decompress = true
				case 'n':
					cfg.lineNumbers = true
				case 'N':
					cfg.lineNumbers = false
				case 'l':
					cfg.filesWithMatches = true
				case 'c':
					cfg.count = true
				case 'o':
					cfg.onlyMatching = true
				case 'H':
					cfg.noFilename = false
				case 'A', 'B', 'C', 'm', 'd', 'g', 't', 'T', 'j', 'e':
					v, err := takeValue("-"+string(c), rest, &i)
					if err != nil {
						return cfg, err
					}
					j = len(cluster) // the value ends this cluster
					switch c {
					case 'A':
						cfg.after, err = intValue("-A", v)
					case 'B':
						cfg.before, err = intValue("-B", v)
					case 'C':
						var n int
						n, err = intValue("-C", v)
						cfg.before, cfg.after = n, n
					case 'm':
						cfg.maxCount, err = intValue("-m", v)
					case 'd':
						cfg.maxDepth, err = intValue("-d", v)
					case 'g':
						cfg.globs = append(cfg.globs, v)
					case 't':
						cfg.types = append(cfg.types, v)
					case 'T':
						cfg.typesNot = append(cfg.typesNot, v)
					case 'j':
						cfg.workers, err = intValue("-j", v)
					case 'e':
						cfg.patterns = append(cfg.patterns, v)
					}
					if err != nil {
						return cfg, err
					}
				default:
					return cfg, fmt.Errorf("unrecognized flag -%c", c)
				}
			}
		default:
			positional = append(positional, a)
		}
	}

	if len(cfg.patterns) == 0 && !cfg.files {
		if len(positional) == 0 {
			return cfg, fmt.Errorf("no pattern given")
		}
		cfg.patterns = []string{positional[0]}
		positional = positional[1:]
	}
	cfg.paths = positional
	return cfg, nil
}

func intValue(flag, v string) (int, error) {
	n, err := strconv.Atoi(v)
	if err != nil || n < 0 {
		return 0, fmt.Errorf("invalid value for %s: %q", flag, v)
	}
	return n, nil
}

// zmaxValue accepts ugrep's range, 1 to 99. Both ends are refused with the
// plain words ugrep uses, because 0 and 100 are the mistakes callers make.
func zmaxValue(v string) (int, error) {
	n, err := strconv.Atoi(v)
	if err != nil {
		return 0, fmt.Errorf("invalid value for --zmax: %q", v)
	}
	if n < 1 {
		return 0, fmt.Errorf("invalid value for --zmax: %d (must be 1..99)", n)
	}
	if n > 99 {
		return 0, fmt.Errorf("--zmax %d exceeds the upper limit of 99", n)
	}
	return n, nil
}
