package main

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"runtime"
	"strings"
	"time"
)

// runUname implements uname with the options an agent needs: -s -n -r -m -a.
func runUname(args []string, stdin *os.File, stdout, stderr *os.File) int {
	all, sysname, nodename, release, machine := false, false, false, false, false
	for _, a := range args {
		switch a {
		case "-a", "--all":
			all = true
		case "-s", "--kernel-name":
			sysname = true
		case "-n", "--nodename":
			nodename = true
		case "-r", "--kernel-release":
			release = true
		case "-m", "--machine":
			machine = true
		default:
			fmt.Fprintf(stderr, "uname: unknown option %q\n", a)
			return 2
		}
	}
	if !all && !sysname && !nodename && !release && !machine {
		sysname = true
	}
	var parts []string
	if all || sysname {
		parts = append(parts, sysName())
	}
	if all || nodename {
		host, err := os.Hostname()
		if err != nil {
			host = "unknown"
		}
		parts = append(parts, host)
	}
	if all || release {
		parts = append(parts, kernelRelease())
	}
	if all || machine {
		parts = append(parts, machineName())
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

// kernelRelease returns the kernel release where the platform exposes one.
// Linux publishes it in /proc; Windows has no equivalent single string, so the
// field stays "-" instead of inventing a version.
func kernelRelease() string {
	if runtime.GOOS == "linux" {
		if data, err := os.ReadFile("/proc/sys/kernel/osrelease"); err == nil {
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

// runTimeout implements timeout DURATION COMMAND [ARG...]. On expiry the child
// is killed and the exit code is 124, like GNU timeout.
func runTimeout(args []string, stdin *os.File, stdout, stderr *os.File) int {
	if len(args) < 2 {
		fmt.Fprintln(stderr, "usage: timeout DURATION COMMAND [ARG...]")
		return 2
	}
	dur, err := parseDuration(args[0])
	if err != nil {
		fmt.Fprintf(stderr, "timeout: %v\n", err)
		return 2
	}
	cmd := exec.Command(args[1], args[2:]...)
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
		_ = cmd.Process.Kill()
		<-done
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
	if n, err := parseSize(s, 0); err == nil {
		return time.Duration(n) * time.Second, nil
	}
	return 0, fmt.Errorf("bad duration %q", s)
}
