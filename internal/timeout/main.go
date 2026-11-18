// Package timeout implements timeout: run a command with a time limit.
package timeout

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strconv"
	"strings"

	"acmesh/internal/proc"
	"acmesh/internal/shared"
	"time"
)

type timeoutOptions struct {
	signal      string
	preserve    bool
	killAfter   time.Duration
	commandArgs []string
}

func parseTimeoutOptions(args []string) (timeoutOptions, error) {
	opts := timeoutOptions{signal: "TERM"}
	for i := 0; i < len(args); i++ {
		a := args[i]
		switch {
		case a == "--":
			opts.commandArgs = args[i+1:]
			return opts, nil
		case a == "--preserve-status":
			opts.preserve = true
		case a == "--foreground" || a == "-v" || a == "--verbose":
			// There are no process groups to hand out here; accepted so that
			// an existing script does not stop on it.
		case a == "-s" || a == "--signal":
			if i+1 >= len(args) {
				return opts, errors.New("option '--signal' requires an argument")
			}
			i++
			opts.signal = args[i]
		case strings.HasPrefix(a, "--signal="):
			opts.signal = strings.TrimPrefix(a, "--signal=")
		case strings.HasPrefix(a, "-s") && len(a) > 2:
			opts.signal = a[2:]
		case a == "-k" || a == "--kill-after":
			if i+1 >= len(args) {
				return opts, errors.New("option '--kill-after' requires an argument")
			}
			i++
			d, err := parseDuration(args[i])
			if err != nil {
				return opts, err
			}
			opts.killAfter = d
		case strings.HasPrefix(a, "--kill-after="):
			d, err := parseDuration(strings.TrimPrefix(a, "--kill-after="))
			if err != nil {
				return opts, err
			}
			opts.killAfter = d
		case strings.HasPrefix(a, "-k") && len(a) > 2:
			d, err := parseDuration(a[2:])
			if err != nil {
				return opts, err
			}
			opts.killAfter = d
		case strings.HasPrefix(a, "-") && len(a) > 1:
			return opts, fmt.Errorf("unrecognized option %q", a)
		default:
			opts.commandArgs = args[i:]
			return opts, nil
		}
	}
	return opts, errors.New("missing operand")
}

// runTimeout implements timeout [OPTION] DURATION COMMAND [ARG...].
// Exit codes follow GNU, measured in a Debian container: 124 on expiry, 137
// when SIGKILL was the signal, 128+signal with --preserve-status.
func Run(args []string, stdin *os.File, stdout, stderr *os.File) int {
	opts, err := parseTimeoutOptions(args)
	if err != nil {
		fmt.Fprintf(stderr, "timeout: %v\nTry 'timeout --help' for more information.\n", err)
		return 2
	}
	if len(opts.commandArgs) < 2 {
		fmt.Fprintln(stderr, "usage: timeout [OPTION] DURATION COMMAND [ARG...]")
		return 2
	}
	dur, err := parseDuration(opts.commandArgs[0])
	if err != nil {
		fmt.Fprintf(stderr, "timeout: %v\n", err)
		return 2
	}
	sigName, err := proc.NormaliseSignal(opts.signal)
	if err != nil {
		fmt.Fprintf(stderr, "timeout: %v\n", err)
		return 2
	}
	cmd := exec.Command(opts.commandArgs[1], opts.commandArgs[2:]...)
	cmd.Stdin = stdin
	cmd.Stdout = stdout
	cmd.Stderr = stderr
	if err := cmd.Start(); err != nil {
		fmt.Fprintf(stderr, "timeout: %v\n", err)
		return 127
	}
	done := make(chan error, 1)
	go func() { done <- cmd.Wait() }()
	select {
	case err := <-done:
		if err == nil {
			return 0
		}
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) {
			return exitErr.ExitCode()
		}
		fmt.Fprintf(stderr, "timeout: %v\n", err)
		return 1
	case <-time.After(dur):
		_ = proc.SignalProcess(cmd.Process.Pid, sigName)
		if opts.killAfter > 0 {
			select {
			case <-done:
			case <-time.After(opts.killAfter):
				_ = cmd.Process.Kill()
				<-done
			}
		} else {
			<-done
		}
		if opts.preserve {
			return 128 + proc.SignalNumber(sigName)
		}
		// GNU exits 137 when it killed with KILL and 124 for every other
		// signal. The name the caller asked for decides, not the platform's
		// mapping: on Windows TERM is an end as well, but a plain timeout is
		// still 124 there.
		requested := strings.ToUpper(strings.TrimPrefix(opts.signal, "SIG"))
		if requested == "KILL" || requested == "9" {
			return 137
		}
		return 124
	}
}

// parseDuration accepts Go durations and GNU suffixed seconds (30, 30s, 2m).
func parseDuration(s string) (time.Duration, error) {
	if s == "" {
		return 0, errors.New("empty duration")
	}
	if d, err := time.ParseDuration(s); err == nil {
		return d, nil
	}
	// GNU takes a bare number as seconds, fractions included ("0.2").
	if n, err := strconv.ParseFloat(s, 64); err == nil {
		if n < 0 {
			return 0, fmt.Errorf("invalid time interval %q", s)
		}
		return time.Duration(n * float64(time.Second)), nil
	}
	if n, err := shared.ParseSize(s, 0); err == nil {
		return time.Duration(n) * time.Second, nil
	}
	return 0, fmt.Errorf("bad duration %q", s)
}
