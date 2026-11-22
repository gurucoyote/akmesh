package jq

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/itchyny/gojq"

	"akmesh/internal/appname"
)

// jqExit codes, as in jq: 3 for a program that does not compile, 5 for an error
// while running.
const (
	jqExitCompile = 3
	jqExitRuntime = 5
)

// runJq is jq on top of gojq. The switches are the ones agents write: -r, -c,
// -e, -n, -s, -j, -R, --arg, --argjson, --slurpfile and -f.
func Run(args []string, stdin *os.File, stdout, stderr *os.File) int {
	var (
		rawOutput bool
		compact   bool
		exitStat  bool
		nullInput bool
		slurp     bool
		joinOut   bool
		rawInput  bool
		progFile  string
		varNames  []string
		varValues []any
		files     []string
		program   string
		haveProg  bool
		slurpVars = map[string]any{}
	)
	for i := 0; i < len(args); i++ {
		a := args[i]
		next := func(what string) (string, bool) {
			if i+1 >= len(args) {
				fmt.Fprintf(stderr, "jq: %s needs a value\n", what)
				return "", false
			}
			i++
			return args[i], true
		}
		switch {
		case a == "--help" || a == "-h":
			printJqHelp(stdout)
			return 0
		case a == "--version" || a == "-V":
			fmt.Fprintln(stdout, "jq-"+appname.Name+"-1.0 (gojq)")
			return 0
		case a == "-r" || a == "--raw-output":
			rawOutput = true
		case a == "-c" || a == "--compact-output":
			compact = true
		case a == "-e" || a == "--exit-status":
			exitStat = true
		case a == "-n" || a == "--null-input":
			nullInput = true
		case a == "-s" || a == "--slurp":
			slurp = true
		case a == "-j" || a == "--join-output":
			joinOut = true
			rawOutput = true
		case a == "-R" || a == "--raw-input":
			rawInput = true
		case a == "-f" || a == "--from-file":
			v, ok := next(a)
			if !ok {
				return 2
			}
			progFile = v
		case a == "--arg":
			name, ok := next("--arg")
			if !ok {
				return 2
			}
			value, ok := next("--arg")
			if !ok {
				return 2
			}
			varNames = append(varNames, "$"+name)
			varValues = append(varValues, value)
		case a == "--argjson":
			name, ok := next("--argjson")
			if !ok {
				return 2
			}
			text, ok := next("--argjson")
			if !ok {
				return 2
			}
			var parsed any
			if err := json.Unmarshal([]byte(text), &parsed); err != nil {
				fmt.Fprintf(stderr, "jq: invalid JSON for --argjson %s: %v\n", name, err)
				return 2
			}
			varNames = append(varNames, "$"+name)
			varValues = append(varValues, parsed)
		case a == "--slurpfile":
			name, ok := next("--slurpfile")
			if !ok {
				return 2
			}
			path, ok := next("--slurpfile")
			if !ok {
				return 2
			}
			values, err := readJSONValues(path)
			if err != nil {
				fmt.Fprintf(stderr, "jq: %v\n", err)
				return 2
			}
			slurpVars[name] = values
			varNames = append(varNames, "$"+name)
			varValues = append(varValues, values)
		case strings.HasPrefix(a, "-") && len(a) > 1 && !isJqUrl(a):
			// Combined short switches such as -rc.
			for _, ch := range a[1:] {
				switch ch {
				case 'r':
					rawOutput = true
				case 'c':
					compact = true
				case 'e':
					exitStat = true
				case 'n':
					nullInput = true
				case 's':
					slurp = true
				case 'j':
					joinOut = true
					rawOutput = true
				case 'R':
					rawInput = true
				default:
					fmt.Fprintf(stderr, "jq: unsupported option -%c\n", ch)
					return 2
				}
			}
		default:
			if !haveProg && progFile == "" {
				program = a
				haveProg = true
			} else {
				files = append(files, a)
			}
		}
	}

	if progFile != "" {
		data, err := os.ReadFile(progFile)
		if err != nil {
			fmt.Fprintf(stderr, "jq: %v\n", err)
			return 2
		}
		program = string(data)
	}
	if program == "" && !haveProg {
		fmt.Fprintln(stderr, "usage: jq [options] FILTER [FILE...]")
		return 2
	}

	query, err := gojq.Parse(program)
	if err != nil {
		fmt.Fprintf(stderr, "jq: error: %v\n", err)
		return jqExitCompile
	}
	opts := []gojq.CompilerOption{}
	if len(varNames) > 0 {
		opts = append(opts, gojq.WithVariables(varNames))
	}
	code, err := gojq.Compile(query, opts...)
	if err != nil {
		fmt.Fprintf(stderr, "jq: error: %v\n", err)
		return jqExitCompile
	}

	inputs, cleanup, err := collectInputs(files, rawInput, slurp, nullInput, stdin)
	if err != nil {
		fmt.Fprintf(stderr, "jq: %v\n", err)
		return 2
	}
	defer cleanup()

	lastOutput := false
	hadOutput := false
	for _, input := range inputs {
		iter := code.Run(input, varValues...)
		for {
			v, ok := iter.Next()
			if !ok {
				break
			}
			if e, isErr := v.(error); isErr {
				fmt.Fprintf(stderr, "jq: error: %v\n", e)
				return jqExitRuntime
			}
			hadOutput = true
			lastOutput = v != nil && v != false
			if err := writeJqValue(stdout, v, rawOutput, compact, joinOut); err != nil {
				fmt.Fprintf(stderr, "jq: %v\n", err)
				return 5
			}
		}
	}
	if exitStat {
		switch {
		case hadOutput && lastOutput:
			return 0
		case hadOutput && !lastOutput:
			return 1
		default:
			return 4
		}
	}
	return 0
}

// isJqUrl keeps a lone "-" and things that look like URLs from being eaten as
// switches.
func isJqUrl(a string) bool { return a == "-" || strings.Contains(a, "://") }

func writeJqValue(stdout *os.File, v any, raw, compact, join bool) error {
	if s, ok := v.(string); ok && raw {
		if _, err := stdout.WriteString(s); err != nil {
			return err
		}
		if !join {
			_, err := stdout.WriteString("\n")
			return err
		}
		return nil
	}
	data, err := gojq.Marshal(v)
	if err != nil {
		return err
	}
	if !compact {
		var buf bytes.Buffer
		if err := json.Indent(&buf, data, "", "  "); err != nil {
			buf.Reset()
			buf.Write(data)
		}
		data = buf.Bytes()
	}
	if _, err := stdout.Write(data); err != nil {
		return err
	}
	if !join {
		_, err = stdout.WriteString("\n")
	}
	return err
}

// collectInputs gathers the input values: JSON documents from files or stdin,
// one string per line with -R, an array with -s, a single null with -n.
func collectInputs(files []string, raw, slurp, nullInput bool, stdin *os.File) ([]any, func(), error) {
	cleanup := func() {}
	if nullInput {
		return []any{nil}, cleanup, nil
	}
	var reader io.Reader = stdin
	if len(files) > 0 {
		var bufs []string
		for _, f := range files {
			data, err := os.ReadFile(f)
			if err != nil {
				return nil, cleanup, err
			}
			bufs = append(bufs, string(data))
		}
		reader = strings.NewReader(strings.Join(bufs, "\n"))
	}
	if raw {
		data, err := io.ReadAll(reader)
		if err != nil {
			return nil, cleanup, err
		}
		if slurp {
			return []any{string(data)}, cleanup, nil
		}
		var out []any
		scanner := bufio.NewScanner(bytes.NewReader(data))
		scanner.Buffer(make([]byte, 0, 64*1024), 8*1024*1024)
		for scanner.Scan() {
			out = append(out, scanner.Text())
		}
		if len(out) == 0 {
			out = append(out, "")
		}
		return out, cleanup, nil
	}
	dec := json.NewDecoder(reader)
	dec.UseNumber()
	var out []any
	for {
		var v any
		err := dec.Decode(&v)
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, cleanup, fmt.Errorf("invalid JSON: %v", err)
		}
		out = append(out, normalizeNumbers(v))
	}
	if slurp {
		return []any{out}, cleanup, nil
	}
	return out, cleanup, nil
}

// readJSONValues reads every JSON value in a file, for --slurpfile.
func readJSONValues(path string) ([]any, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	dec := json.NewDecoder(bytes.NewReader(data))
	dec.UseNumber()
	var out []any
	for {
		var v any
		err := dec.Decode(&v)
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("%s: invalid JSON: %v", path, err)
		}
		out = append(out, normalizeNumbers(v))
	}
	return out, nil
}

// normalizeNumbers turns json.Number into int or float64, which is what gojq
// accepts and what jq prints.
func normalizeNumbers(v any) any {
	switch x := v.(type) {
	case map[string]any:
		for k, item := range x {
			x[k] = normalizeNumbers(item)
		}
		return x
	case []any:
		for i, item := range x {
			x[i] = normalizeNumbers(item)
		}
		return x
	case json.Number:
		if n, err := x.Int64(); err == nil {
			return int(n)
		}
		if f, err := x.Float64(); err == nil {
			return f
		}
		return x.String()
	}
	return v
}

func printJqHelp(stdout *os.File) {
	fmt.Fprint(stdout, `usage: jq [options] FILTER [FILE...]

  -r, --raw-output      strings without quotes
  -c, --compact-output  one line per value
  -j, --join-output     like -r, no newline after each value
  -e, --exit-status     0 if the last value is truthy, 1 if false or null, 4 if none
  -n, --null-input      run the filter without input
  -s, --slurp           read all input into one array
  -R, --raw-input       each input line is a string
  -f, --from-file FILE  read the filter from FILE
      --arg NAME VALUE      string variable
      --argjson NAME JSON   JSON variable
      --slurpfile NAME FILE array variable from a file

Short switches may be combined: -rc. Built on gojq.
`)
}
