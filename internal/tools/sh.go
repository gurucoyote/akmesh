package tools

import (
	"context"
	"fmt"
	"io"
	"os"
	"strings"

	"mvdan.cc/sh/v3/expand"
	"mvdan.cc/sh/v3/interp"
	"mvdan.cc/sh/v3/syntax"
)

// runSh runs the shell that the policy applies to. It is mvdan's shell, so the
// language is POSIX-ish bash, but every file it opens, every program it starts
// and every child those programs start lives inside the policy the process
// applied before this function ran.
func runSh(args []string, stdin *os.File, stdout, stderr *os.File) int {
	command := ""
	var positional []string
	var scriptFile string
	interactive := false
	for i := 0; i < len(args); i++ {
		a := args[i]
		if a == "-c" {
			if i+1 >= len(args) {
				fmt.Fprintln(stderr, "sh: -c needs a command string")
				return 2
			}
			command = args[i+1]
			positional = args[i+2:]
			i = len(args)
			continue
		}
		if a == "-i" {
			interactive = true
			continue
		}
		if a == "--help" {
			fmt.Fprintln(stdout, "usage: sh [-c COMMAND] [SCRIPT [ARGS...]]")
			fmt.Fprintln(stdout, "runs mvdan/sh inside the policy of this process")
			return 0
		}
		if strings.HasPrefix(a, "-") && a != "-" {
			fmt.Fprintf(stderr, "sh: unsupported option %q\n", a)
			return 2
		}
		if command == "" && scriptFile == "" {
			scriptFile = a
			continue
		}
		positional = append(positional, a)
	}
	_ = interactive

	var src, name string
	switch {
	case command != "":
		src, name = command, "<command>"
	case scriptFile != "" && scriptFile != "-":
		data, err := os.ReadFile(scriptFile)
		if err != nil {
			fmt.Fprintf(stderr, "sh: %v\n", err)
			return 127
		}
		src, name = string(data), scriptFile
	default:
		data, err := io.ReadAll(stdin)
		if err != nil {
			fmt.Fprintf(stderr, "sh: %v\n", err)
			return 1
		}
		src, name = string(data), "<stdin>"
	}

	dir, err := os.Getwd()
	if err != nil {
		dir = "/"
	}
	opts := []interp.RunnerOption{
		interp.StdIO(stdin, stdout, stderr),
		interp.Dir(dir),
		interp.Env(expand.ListEnviron(os.Environ()...)),
	}
	if len(positional) > 0 {
		opts = append(opts, interp.Params(positional...))
	}
	runner, err := interp.New(opts...)
	if err != nil {
		fmt.Fprintf(stderr, "sh: %v\n", err)
		return 1
	}
	file, err := syntax.NewParser().Parse(strings.NewReader(src), name)
	if err != nil {
		fmt.Fprintf(stderr, "sh: %v\n", err)
		return 2
	}
	if err := runner.Run(context.Background(), file); err != nil {
		if status, ok := interp.IsExitStatus(err); ok {
			return int(status)
		}
		fmt.Fprintf(stderr, "sh: %v\n", err)
		return 1
	}
	return 0
}
