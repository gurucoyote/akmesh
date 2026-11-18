//go:build !windows

package proc

import (
	"fmt"
	"strconv"
	"strings"
	"syscall"
)

// normaliseSignal maps a signal name or number onto a Linux signal.
func normaliseSignal(name string) (string, error) {
	clean := strings.ToUpper(strings.TrimSpace(name))
	clean = strings.TrimPrefix(clean, "SIG")
	if _, ok := signalTable[clean]; ok {
		return clean, nil
	}
	if n, err := strconv.Atoi(clean); err == nil {
		for name, sig := range signalTable {
			if int(sig) == n {
				return name, nil
			}
		}
	}
	return "", fmt.Errorf("unknown signal %q", name)
}

var signalTable = map[string]syscall.Signal{
	// Signal 0 sends nothing: kill -0 PID is the existence check, and GNU
	// accepts it under the name EXIT as well.
	"EXIT": 0,
	"HUP":  syscall.SIGHUP,
	"INT":  syscall.SIGINT,
	"QUIT": syscall.SIGQUIT,
	"ILL":  syscall.SIGILL,
	"TRAP": syscall.SIGTRAP,
	"ABRT": syscall.SIGABRT,
	"BUS":  syscall.SIGBUS,
	"FPE":  syscall.SIGFPE,
	"KILL": syscall.SIGKILL,
	"USR1": syscall.SIGUSR1,
	"SEGV": syscall.SIGSEGV,
	"USR2": syscall.SIGUSR2,
	"PIPE": syscall.SIGPIPE,
	"ALRM": syscall.SIGALRM,
	"TERM": syscall.SIGTERM,
	"CHLD": syscall.SIGCHLD,
	"CONT": syscall.SIGCONT,
	"STOP": syscall.SIGSTOP,
	"TSTP": syscall.SIGTSTP,
	"TTIN": syscall.SIGTTIN,
	"TTOU": syscall.SIGTTOU,
}

func signalProcess(pid int, name string) error {
	sig, ok := signalTable[name]
	if !ok {
		return fmt.Errorf("unsupported signal %q", name)
	}
	if err := syscall.Kill(pid, sig); err != nil {
		return fmt.Errorf("%d: %v", pid, err)
	}
	return nil
}

// signalNames is used by the help text of kill and killall.
func signalNumber(name string) int { return int(signalTable[name]) }

func signalNames() []string {
	names := make([]string, 0, len(signalTable))
	for name := range signalTable {
		names = append(names, name)
	}
	return names
}
