package sedawk

import (
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/benhoyt/goawk/interp"
	"github.com/benhoyt/goawk/parser"
	"github.com/rwtodd/Go.Sed/sed"
)

// runSed implements sed through the Go.Sed engine: -e, -f, -n, -i and -E.
//
// The engine is forked (scripts/apply-go-sed-fork.sh): a basic regular
// expression is translated to Go's syntax, so \(...\)/\1 and & work, the
// leftmost longest match wins, and "." matches a newline. Without -E the
// pattern is read the way GNU sed reads it; with -E the operators are written
// unescaped and nothing is translated. What stays open: back references inside
// the search pattern (\(a\)\1) need an engine with backtracking, and the error
// says so.
func runSed(args []string, stdin *os.File, stdout, stderr *os.File) int {
	var programs []string
	quiet := false
	extended := false
	inPlace := false
	var files []string
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "-e", "--expression":
			if i+1 >= len(args) {
				fmt.Fprintln(stderr, "sed: -e needs an argument")
				return 2
			}
			i++
			programs = append(programs, args[i])
		case "-f", "--file":
			if i+1 >= len(args) {
				fmt.Fprintln(stderr, "sed: -f needs an argument")
				return 2
			}
			i++
			data, err := os.ReadFile(args[i])
			if err != nil {
				fmt.Fprintf(stderr, "sed: %v\n", err)
				return 1
			}
			programs = append(programs, string(data))
		case "-E", "-r", "--regexp-extended":
			extended = true
		case "-n", "--quiet", "--silent":
			quiet = true
		case "-i", "--in-place":
			inPlace = true
		default:
			if strings.HasPrefix(args[i], "-") && args[i] != "-" {
				fmt.Fprintf(stderr, "sed: unknown option %q\n", args[i])
				return 2
			}
			files = append(files, args[i])
		}
	}
	if len(programs) == 0 {
		if len(files) == 0 {
			fmt.Fprintln(stderr, "sed: no program")
			return 2
		}
		programs = append(programs, files[0])
		files = files[1:]
	}
	program := strings.Join(programs, "\n")
	var eng *sed.Engine
	var err error
	switch {
	case quiet && extended:
		eng, err = sed.NewQuietExtended(strings.NewReader(program))
	case quiet:
		eng, err = sed.NewQuiet(strings.NewReader(program))
	case extended:
		eng, err = sed.NewExtended(strings.NewReader(program))
	default:
		eng, err = sed.New(strings.NewReader(program))
	}
	if err != nil {
		fmt.Fprintf(stderr, "sed: %v\n", err)
		return 1
	}
	if len(files) == 0 {
		if _, err := io.Copy(stdout, eng.Wrap(stdin)); err != nil {
			fmt.Fprintf(stderr, "sed: %v\n", err)
			return 1
		}
		return 0
	}
	code := 0
	for _, name := range files {
		if name == "-" {
			io.Copy(stdout, eng.Wrap(stdin))
			continue
		}
		f, err := os.Open(name)
		if err != nil {
			fmt.Fprintf(stderr, "sed: %v\n", err)
			code = 1
			continue
		}
		if !inPlace {
			if _, err := io.Copy(stdout, eng.Wrap(f)); err != nil {
				fmt.Fprintf(stderr, "sed: %v\n", err)
				code = 1
			}
			f.Close()
			continue
		}
		var sb strings.Builder
		if _, err := io.Copy(&sb, eng.Wrap(f)); err != nil {
			fmt.Fprintf(stderr, "sed: %v\n", err)
			f.Close()
			code = 1
			continue
		}
		f.Close()
		info, _ := os.Stat(name)
		mode := os.FileMode(0o644)
		if info != nil {
			mode = info.Mode().Perm()
		}
		if err := os.WriteFile(name, []byte(sb.String()), mode); err != nil {
			fmt.Fprintf(stderr, "sed: %v\n", err)
			code = 1
		}
	}
	return code
}

// runAwk implements awk through goawk: -F, -v, -f, program and files.
func runAwk(args []string, stdin *os.File, stdout, stderr *os.File) int {
	var progFiles, vars []string
	fieldSep := ""
	var positional []string
	for i := 0; i < len(args); i++ {
		a := args[i]
		attached := ""
		if len(a) > 2 && (a[:2] == "-F" || a[:2] == "-v" || a[:2] == "-f") {
			a, attached = a[:2], a[2:]
		}
		switch a {
		case "-F":
			if attached == "" {
				if i+1 >= len(args) {
					fmt.Fprintln(stderr, "awk: -F needs an argument")
					return 2
				}
				i++
				attached = args[i]
			}
			fieldSep = attached
		case "-v":
			if attached == "" {
				if i+1 >= len(args) {
					fmt.Fprintln(stderr, "awk: -v needs an argument")
					return 2
				}
				i++
				attached = args[i]
			}
			name, value, ok := strings.Cut(attached, "=")
			if !ok {
				fmt.Fprintf(stderr, "awk: -v wants name=value, got %q\n", attached)
				return 2
			}
			vars = append(vars, name, value)
		case "-f":
			if attached == "" {
				if i+1 >= len(args) {
					fmt.Fprintln(stderr, "awk: -f needs an argument")
					return 2
				}
				i++
				attached = args[i]
			}
			data, err := os.ReadFile(attached)
			if err != nil {
				fmt.Fprintf(stderr, "awk: %v\n", err)
				return 1
			}
			progFiles = append(progFiles, string(data))
		default:
			if strings.HasPrefix(args[i], "-") && args[i] != "-" && len(positional) == 0 {
				fmt.Fprintf(stderr, "awk: unknown option %q\n", args[i])
				return 2
			}
			positional = append(positional, args[i])
		}
	}
	source := strings.Join(progFiles, "\n")
	if source == "" {
		if len(positional) == 0 {
			fmt.Fprintln(stderr, "awk: no program")
			return 2
		}
		source = positional[0]
		positional = positional[1:]
	}
	prog, err := parser.ParseProgram([]byte(source), &parser.ParserConfig{})
	if err != nil {
		fmt.Fprintf(stderr, "awk: %v\n", err)
		return 2
	}
	if fieldSep != "" {
		vars = append(vars, "FS", fieldSep)
	}
	cfg := &interp.Config{
		Stdin:  stdin,
		Output: stdout,
		Error:  stderr,
		Argv0:  "awk",
		Args:   positional,
		Vars:   vars,
	}
	status, err := interp.ExecProgram(prog, cfg)
	if err != nil {
		fmt.Fprintf(stderr, "awk: %v\n", err)
	}
	return status
}
