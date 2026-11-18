package tools

import (
	"bufio"
	"fmt"
	"os"
	"sort"
	"strconv"
	"strings"
)

// runKill sends a signal to one or more processes:
//
//	kill [-SIGNAL] PID...
//	kill -s SIGNAL PID...
//
// The default is TERM, as in any shell. Signals a platform does not have are an
// error instead of a silent no-op.
func runKill(args []string, stdin *os.File, stdout, stderr *os.File) int {
	sigName := "TERM"
	var pids []int
	for i := 0; i < len(args); i++ {
		a := args[i]
		switch {
		case a == "-l" || a == "--list":
			return runKillList(args[i+1:], stdout, stderr)
		case a == "-s" || a == "--signal":
			if i+1 >= len(args) {
				fmt.Fprintln(stderr, "kill: -s needs a signal")
				return 2
			}
			i++
			sigName = args[i]
		case strings.HasPrefix(a, "--signal="):
			sigName = strings.TrimPrefix(a, "--signal=")
		case strings.HasPrefix(a, "-") && len(a) > 1 && a != "-9" && !isNumber(a[1:]):
			sigName = strings.ToUpper(strings.TrimPrefix(a, "-"))
		case a == "-9":
			sigName = "KILL"
		case strings.HasPrefix(a, "-") && len(a) > 1:
			sigName = strings.ToUpper(a[1:])
		default:
			pid, err := strconv.Atoi(a)
			if err != nil {
				fmt.Fprintf(stderr, "kill: bad pid %q\n", a)
				return 2
			}
			pids = append(pids, pid)
		}
	}
	if len(pids) == 0 {
		fmt.Fprintln(stderr, "usage: kill [-SIGNAL] PID...")
		return 2
	}
	sig, err := normaliseSignal(sigName)
	if err != nil {
		fmt.Fprintf(stderr, "kill: %v\n", err)
		return 2
	}
	code := 0
	for _, pid := range pids {
		if err := signalProcess(pid, sig); err != nil {
			fmt.Fprintf(stderr, "kill: %v\n", err)
			code = 1
		}
	}
	return code
}

// runKillList is kill -l: names in signal order, or one translation.
// "kill -l 9" prints KILL, "kill -l KILL" prints 9, as the POSIX form does.
func runKillList(rest []string, stdout, stderr *os.File) int {
	if len(rest) == 0 {
		names := signalNames()
		sort.Slice(names, func(i, j int) bool { return signalNumber(names[i]) < signalNumber(names[j]) })
		for _, name := range names {
			fmt.Fprintln(stdout, name)
		}
		return 0
	}
	arg := rest[0]
	name, err := normaliseSignal(arg)
	if err != nil {
		fmt.Fprintf(stderr, "kill: %v\n", err)
		return 2
	}
	if _, err := strconv.Atoi(arg); err == nil {
		fmt.Fprintln(stdout, name)
		return 0
	}
	fmt.Fprintln(stdout, signalNumber(name))
	return 0
}

func isNumber(s string) bool {
	if s == "" {
		return false
	}
	for _, r := range s {
		if r < '0' || r > '9' {
			return false
		}
	}
	return true
}

// runKillall ends every process whose name matches:
//
//	killall [-i] [-q] [-SIGNAL] NAME...
//
// The name is compared case-insensitively and without the .exe suffix, so
// "killall notepad" works on Windows and Linux alike. The calling process is
// never killed.
func runKillall(args []string, stdin *os.File, stdout, stderr *os.File) int {
	sigName := "TERM"
	quiet := false
	interactive := false
	var names []string
	for i := 0; i < len(args); i++ {
		a := args[i]
		switch {
		case a == "-q" || a == "--quiet":
			quiet = true
		case a == "-i" || a == "--interactive":
			interactive = true
		case a == "-s" || a == "--signal":
			if i+1 >= len(args) {
				fmt.Fprintln(stderr, "killall: -s needs a signal")
				return 2
			}
			i++
			sigName = args[i]
		case strings.HasPrefix(a, "-") && len(a) > 1:
			sigName = strings.ToUpper(strings.TrimLeft(a, "-"))
		default:
			names = append(names, a)
		}
	}
	if len(names) == 0 {
		fmt.Fprintln(stderr, "usage: killall [-i] [-q] [-SIGNAL] NAME...")
		return 2
	}
	sig, err := normaliseSignal(sigName)
	if err != nil {
		fmt.Fprintf(stderr, "killall: %v\n", err)
		return 2
	}
	procs, err := listProcesses()
	if err != nil {
		fmt.Fprintf(stderr, "killall: %v\n", err)
		return 1
	}
	self := os.Getpid()
	reader := bufio.NewReader(stdin)
	killed := 0
	for _, p := range procs {
		if p.PID == self {
			continue
		}
		if !matchesName(p, names) {
			continue
		}
		if interactive {
			fmt.Fprintf(stderr, "kill %s(%d)? ", p.Comm, p.PID)
			answer, _ := reader.ReadString('\n')
			answer = strings.ToLower(strings.TrimSpace(answer))
			if answer != "y" && answer != "yes" {
				continue
			}
		}
		if err := signalProcess(p.PID, sig); err != nil {
			if !quiet {
				fmt.Fprintf(stderr, "killall: %v\n", err)
			}
			continue
		}
		killed++
	}
	if killed == 0 {
		if !quiet {
			fmt.Fprintf(stderr, "killall: no process found for %s\n", strings.Join(names, ", "))
		}
		return 1
	}
	return 0
}

// matchesName compares without a trailing .exe and case-insensitively.
func matchesName(p procInfo, names []string) bool {
	candidates := []string{p.Comm}
	if p.CmdLine != "" {
		candidates = append(candidates, firstWord(p.CmdLine))
	}
	for _, cand := range candidates {
		base := strings.ToLower(trimExe(cand))
		for _, want := range names {
			if base == strings.ToLower(trimExe(want)) {
				return true
			}
		}
	}
	return false
}

func trimExe(s string) string {
	if strings.HasSuffix(strings.ToLower(s), ".exe") {
		return s[:len(s)-4]
	}
	return s
}

func firstWord(s string) string {
	if idx := strings.IndexAny(s, " \t"); idx >= 0 {
		return s[:idx]
	}
	return s
}
