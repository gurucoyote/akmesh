//go:build !windows

package sh

import (
	"context"
	"fmt"
	"io"
	"strconv"
	"strings"
	"syscall"

	"mvdan.cc/sh/v3/interp"
)

// killCall answers the kill builtin. mvdan/sh lists kill among the POSIX
// builtins and then answers "unsupported builtin" - and because it is treated as
// a builtin, the shell never looks for /bin/kill either, so a script could not
// stop a process at all (acmesh-oxw). Answering it here keeps the process in the
// shell and needs no external file.
func killCall(out *switchWriter, stderr io.Writer) interp.CallHandlerFunc {
	return func(ctx context.Context, args []string) ([]string, error) {
		if len(args) == 0 || args[0] != "kill" {
			return args, nil
		}
		if runKill(args[1:], out, stderr) {
			return []string{"true"}, nil
		}
		return []string{"false"}, nil
	}
}

// runKill understands what ash and bash understand for the common cases:
// kill [-s sigspec | -n signum | -sigspec] pid ..., and kill -l [signum].
func runKill(args []string, out *switchWriter, stderr io.Writer) bool {
	var sig syscall.Signal = syscall.SIGTERM
	var pids []string
	for i := 0; i < len(args); i++ {
		a := args[i]
		switch {
		case a == "-l" || a == "-L":
			return listSignals(args[i+1:], out, stderr)
		case a == "-s" || a == "-n":
			if i+1 >= len(args) {
				fmt.Fprintf(stderr, "kill: %s needs a signal\n", a)
				return false
			}
			i++
			s, ok := parseSignal(args[i])
			if !ok {
				fmt.Fprintf(stderr, "kill: %s: invalid signal\n", args[i])
				return false
			}
			sig = s
		case len(a) > 1 && a[0] == '-' && (isNumber(a[1:]) || !isFlagLike(a[1:])):
			// -TERM, -9 and -0 all arrive here as a bare flag: in kill a
			// leading dash always means a signal, never a negative pid.
			s, ok := parseSignal(a[1:])
			if !ok {
				fmt.Fprintf(stderr, "kill: %s: invalid signal\n", a[1:])
				return false
			}
			sig = s
		case a == "--":
			pids = append(pids, args[i+1:]...)
			i = len(args)
		default:
			pids = append(pids, a)
		}
	}
	if len(pids) == 0 {
		fmt.Fprintln(stderr, "kill: usage: kill [-s sigspec | -n signum | -sigspec] pid ...")
		return false
	}
	ok := true
	for _, p := range pids {
		n, err := strconv.Atoi(p)
		if err != nil {
			fmt.Fprintf(stderr, "kill: %s: arguments must be process or job IDs\n", p)
			ok = false
			continue
		}
		// 0 is the caller's process group, negative numbers are process
		// groups: kill passes both through unchanged, as ash does.
		if err := syscall.Kill(n, sig); err != nil {
			if err == syscall.ESRCH {
				fmt.Fprintf(stderr, "kill: %d: no such process\n", n)
			} else {
				fmt.Fprintf(stderr, "kill: %d: %v\n", n, err)
			}
			ok = false
		}
	}
	return ok
}

// parseSignal accepts a number or a name, with or without the SIG prefix and in
// either case, which is what scripts actually write.
func parseSignal(s string) (syscall.Signal, bool) {
	if n, err := strconv.Atoi(s); err == nil {
		if n < 0 || n > 64 {
			return 0, false
		}
		return syscall.Signal(n), true
	}
	name := strings.ToUpper(strings.TrimPrefix(strings.ToUpper(s), "SIG"))
	if n, ok := signalNumbers[name]; ok {
		return syscall.Signal(n), true
	}
	return 0, false
}

// signalNumbers is the name table of Linux, which is what the target is.
var signalNumbers = map[string]int{
	"HUP": 1, "INT": 2, "QUIT": 3, "ILL": 4, "TRAP": 5, "ABRT": 6, "BUS": 7,
	"FPE": 8, "KILL": 9, "USR1": 10, "SEGV": 11, "USR2": 12, "PIPE": 13,
	"ALRM": 14, "TERM": 15, "STKFLT": 16, "CHLD": 17, "CONT": 18, "STOP": 19,
	"TSTP": 20, "TTIN": 21, "TTOU": 22, "URG": 23, "XCPU": 24, "XFSZ": 25,
	"VTALRM": 26, "PROF": 27, "WINCH": 28, "IO": 29, "PWR": 30, "SYS": 31,
}

// signalNames is the reverse order for kill -l, indexed by number.
var signalNames = []string{
	0: "EXIT", 1: "HUP", 2: "INT", 3: "QUIT", 4: "ILL", 5: "TRAP", 6: "ABRT",
	7: "BUS", 8: "FPE", 9: "KILL", 10: "USR1", 11: "SEGV", 12: "USR2",
	13: "PIPE", 14: "ALRM", 15: "TERM", 16: "STKFLT", 17: "CHLD", 18: "CONT",
	19: "STOP", 20: "TSTP", 21: "TTIN", 22: "TTOU", 23: "URG", 24: "XCPU",
	25: "XFSZ", 26: "VTALRM", 27: "PROF", 28: "WINCH", 29: "IO", 30: "PWR",
	31: "SYS",
}

func listSignals(args []string, out *switchWriter, stderr io.Writer) bool {
	if len(args) == 0 {
		var b strings.Builder
		for n := 1; n < len(signalNames); n++ {
			b.WriteString(strconv.Itoa(n))
			b.WriteString(") SIG")
			b.WriteString(signalNames[n])
			if n%8 == 0 {
				b.WriteString("\n")
			} else {
				b.WriteString("\t")
			}
		}
		b.WriteString("\n")
		out.WriteString(b.String())
		return true
	}
	for _, a := range args {
		if n, err := strconv.Atoi(a); err == nil {
			if n < 0 || n >= len(signalNames) {
				fmt.Fprintf(stderr, "kill: %s: invalid signal\n", a)
				return false
			}
			fmt.Fprintf(stderr, "%s\n", signalNames[n])
			continue
		}
		name := strings.ToUpper(strings.TrimPrefix(strings.ToUpper(a), "SIG"))
		n, ok := signalNumbers[name]
		if !ok {
			fmt.Fprintf(stderr, "kill: %s: invalid signal\n", a)
			return false
		}
		fmt.Fprintf(stderr, "%d\n", n)
	}
	return true
}

// isFlagLike reports whether the text after the dash could be an option rather
// than a signal name. "0" and "KILL" are signals; "s" and "n" are options.
func isFlagLike(s string) bool {
	switch s {
	case "s", "n", "l", "L":
		return true
	}
	return false
}

func isNumber(s string) bool {
	if s == "" {
		return false
	}
	for i := 0; i < len(s); i++ {
		if s[i] < '0' || s[i] > '9' {
			return false
		}
	}
	return true
}
