// Package uname implements uname, the system information an agent asks for
// first.

package uname

import (
	"fmt"
	"os"
	"runtime"
	"strings"

	"akmesh/internal/usage"
)

type unameFlags struct {
	all, sysname, nodename, release, version, machine, processor, platform, osname bool
}

// runUname implements uname with every field GNU has. An agent writes
// "uname -o" or "uname -a" from muscle memory, so -o, -p, -i and -v exist even
// where the platform has no honest value for them: then the field reads
// "unknown" or "-", never an invented version.
func Run(args []string, stdin *os.File, stdout, stderr *os.File) int {
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
				return usage.BadOption("uname", a, stderr, 1)
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
