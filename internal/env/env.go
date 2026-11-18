package env

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strings"
)

// runEnv implements env [-i] [-u NAME]... [NAME=VALUE]... [COMMAND [ARG]...].
//
// Without a command it prints the environment; with one it starts the command
// with the adjusted environment and returns its exit code. This is the one
// command here that starts another process, so it passes the standard streams
// through untouched.
func Run(args []string, stdin *os.File, stdout, stderr *os.File) int {
	var (
		clearEnv  bool
		unset     []string
		assigns   []string
		command   []string
		ignoreAll bool
	)
	for i := 0; i < len(args); i++ {
		a := args[i]
		switch {
		case a == "-i" || a == "--ignore-environment" || a == "-":
			clearEnv = true
		case a == "-0" || a == "--null":
			ignoreAll = true
		case a == "-u" || a == "--unset":
			if i+1 >= len(args) {
				fmt.Fprintln(stderr, "env: -u needs a name")
				return 2
			}
			i++
			unset = append(unset, args[i])
		case strings.HasPrefix(a, "-u") && len(a) > 2:
			unset = append(unset, a[2:])
		case strings.HasPrefix(a, "--unset="):
			unset = append(unset, strings.TrimPrefix(a, "--unset="))
		case a == "--":
			command = append(command, args[i+1:]...)
			i = len(args)
		case strings.HasPrefix(a, "-") && a != "-":
			fmt.Fprintf(stderr, "env: unknown option %q\n", a)
			return 2
		case strings.Contains(a, "="):
			assigns = append(assigns, a)
		default:
			command = append(command, args[i:]...)
			i = len(args)
		}
	}
	_ = ignoreAll

	base := os.Environ()
	var result []string
	if !clearEnv {
		for _, kv := range base {
			name, _, _ := strings.Cut(kv, "=")
			if contains(unset, name) {
				continue
			}
			result = append(result, kv)
		}
	}
	for _, kv := range assigns {
		name, _, _ := strings.Cut(kv, "=")
		result = append(result, kv)
		// A later assignment to the same name replaces the earlier one.
		result = dedupeEnv(result, name)
	}
	for _, name := range unset {
		result = removeEnv(result, name)
	}

	if len(command) == 0 {
		for _, kv := range result {
			fmt.Fprintln(stdout, kv)
		}
		return 0
	}

	path, err := exec.LookPath(command[0])
	if err != nil {
		fmt.Fprintf(stderr, "env: %v\n", err)
		return 127
	}
	cmd := exec.Command(path, command[1:]...)
	cmd.Env = result
	cmd.Stdin = stdin
	cmd.Stdout = stdout
	cmd.Stderr = stderr
	if err := cmd.Run(); err != nil {
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) {
			return exitErr.ExitCode()
		}
		fmt.Fprintf(stderr, "env: %v\n", err)
		return 126
	}
	return 0
}

func contains(list []string, name string) bool {
	for _, item := range list {
		if item == name {
			return true
		}
	}
	return false
}

func removeEnv(env []string, name string) []string {
	out := env[:0]
	for _, kv := range env {
		if key, _, _ := strings.Cut(kv, "="); key == name {
			continue
		}
		out = append(out, kv)
	}
	return out
}

// dedupeEnv keeps only the last entry for name, which is the one we just added.
func dedupeEnv(env []string, name string) []string {
	last := ""
	seenLast := false
	for i := len(env) - 1; i >= 0; i-- {
		key, _, _ := strings.Cut(env[i], "=")
		if key == name && !seenLast {
			last = env[i]
			seenLast = true
			break
		}
	}
	if !seenLast {
		return env
	}
	out := make([]string, 0, len(env))
	for _, kv := range env {
		if key, _, _ := strings.Cut(kv, "="); key == name {
			continue
		}
		out = append(out, kv)
	}
	return append(out, last)
}
