package tools

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"runtime"
	"strconv"
	"strings"
	"time"
)

// unameFlags holds one bit per output field, in the order GNU prints them.
type unameFlags struct {
	all, sysname, nodename, release, version, machine, processor, platform, osname bool
}

// runUname implements uname with every field GNU has. An agent writes
// "uname -o" or "uname -a" from muscle memory, so -o, -p, -i and -v exist even
// where the platform has no honest value for them: then the field reads
// "unknown" or "-", never an invented version.
func runUname(args []string, stdin *os.File, stdout, stderr *os.File) int {
	var f unameFlags
	for _, a := range args {
		switch {
		case a == "--":
		case strings.HasPrefix(a, "--"):
			switch a {
			case "--all":
				f.all = true
			case "--kernel-name":
				f.sysname = true
			case "--nodename":
				f.nodename = true
			case "--kernel-release":
				f.release = true
			case "--kernel-version":
				f.version = true
			case "--machine":
				f.machine = true
			case "--processor":
				f.processor = true
			case "--hardware-platform":
				f.platform = true
			case "--operating-system":
				f.osname = true
			default:
				fmt.Fprintf(stderr, "uname: unrecognized option %q\nTry 'uname --help' for more information.\n", a)
				return 2
			}
		case strings.HasPrefix(a, "-") && a != "-":
			// Combined short options, "-sn" like "-s -n".
			for _, c := range a[1:] {
				switch c {
				case 'a':
					f.all = true
				case 's':
					f.sysname = true
				case 'n':
					f.nodename = true
				case 'r':
					f.release = true
				case 'v':
					f.version = true
				case 'm':
					f.machine = true
				case 'p':
					f.processor = true
				case 'i':
					f.platform = true
				case 'o':
					f.osname = true
				default:
					fmt.Fprintf(stderr, "uname: invalid option -- '%c'\nTry 'uname --help' for more information.\n", c)
					return 2
				}
			}
		default:
			fmt.Fprintf(stderr, "uname: extra operand %q\n", a)
			return 2
		}
	}
	if !f.all && !f.sysname && !f.nodename && !f.release && !f.version &&
		!f.machine && !f.processor && !f.platform && !f.osname {
		f.sysname = true
	}
	var parts []string
	if f.all || f.sysname {
		parts = append(parts, sysName())
	}
	if f.all || f.nodename {
		host, err := os.Hostname()
		if err != nil {
			host = "unknown"
		}
		parts = append(parts, host)
	}
	if f.all || f.release {
		parts = append(parts, kernelRelease())
	}
	if f.all || f.version {
		parts = append(parts, kernelVersion())
	}
	if f.all || f.machine {
		parts = append(parts, machineName())
	}
	if f.all || f.processor {
		parts = append(parts, processorName())
	}
	if f.all || f.platform {
		parts = append(parts, "unknown")
	}
	if f.all || f.osname {
		parts = append(parts, osName())
	}
	fmt.Fprintln(stdout, strings.Join(parts, " "))
	return 0
}

func sysName() string {
	switch runtime.GOOS {
	case "windows":
		return "Windows_NT"
	case "linux":
		return "Linux"
	case "darwin":
		return "Darwin"
	default:
		return runtime.GOOS
	}
}

func machineName() string {
	switch runtime.GOARCH {
	case "amd64":
		return "x86_64"
	case "arm64":
		return "aarch64"
	case "386":
		return "i686"
	default:
		return runtime.GOARCH
	}
}

// processorName is "unknown" on Linux, exactly like GNU uname -p: the kernel
// does not publish it, and a guess would be worse than the word.
func processorName() string { return "unknown" }

// osName is GNU's -o: the operating system name, not the kernel name.
func osName() string {
	if runtime.GOOS == "linux" {
		return "GNU/Linux"
	}
	return sysName()
}

// kernelRelease returns the kernel release where the platform exposes one.
// Linux publishes it in /proc; Windows has no equivalent single string, so the
// field stays "-" instead of inventing a version.
func kernelRelease() string {
	return procLine("/proc/sys/kernel/osrelease")
}

// kernelVersion is GNU's -v: the build string of the running kernel.
func kernelVersion() string {
	return procLine("/proc/sys/kernel/version")
}

func procLine(path string) string {
	if runtime.GOOS == "linux" {
		if data, err := os.ReadFile(path); err == nil {
			if text := strings.TrimSpace(string(data)); text != "" {
				return text
			}
		}
	}
	return "-"
}

// runHostname prints the machine name.
func runHostname(args []string, stdin *os.File, stdout, stderr *os.File) int {
	if len(args) > 0 {
		fmt.Fprintln(stderr, "hostname: setting the name is not supported")
		return 2
	}
	host, err := os.Hostname()
	if err != nil {
		fmt.Fprintf(stderr, "hostname: %v\n", err)
		return 1
	}
	fmt.Fprintln(stdout, host)
	return 0
}

// timeoutOptions is what GNU timeout accepts before the duration.
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
func runTimeout(args []string, stdin *os.File, stdout, stderr *os.File) int {
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
	sigName, err := normaliseSignal(opts.signal)
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
		_ = signalProcess(cmd.Process.Pid, sigName)
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
			return 128 + signalNumber(sigName)
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
	if n, err := parseSize(s, 0); err == nil {
		return time.Duration(n) * time.Second, nil
	}
	return 0, fmt.Errorf("bad duration %q", s)
}
