//go:build windows

package proc

import (
	"fmt"
	"strings"

	"golang.org/x/sys/windows"
)

// normaliseSignal narrows the signal names to the two Windows has: there is no
// SIGTERM that a process could handle, only an end.
func normaliseSignal(name string) (string, error) {
	clean := strings.ToUpper(strings.TrimSpace(name))
	clean = strings.TrimPrefix(clean, "SIG")
	switch clean {
	case "0", "EXIT":
		// Signal 0 is the existence check: nothing is sent, the handle decides.
		return "EXIT", nil
	case "TERM", "15", "KILL", "9", "QUIT", "3", "INT", "2":
		return "KILL", nil
	}
	return "", fmt.Errorf("signal %q is not supported on Windows (only KILL/TERM)", name)
}

func signalProcess(pid int, name string) error {
	if name == "EXIT" {
		h, err := windows.OpenProcess(windows.PROCESS_QUERY_LIMITED_INFORMATION, false, uint32(pid))
		if err != nil {
			return fmt.Errorf("%d: %v", pid, err)
		}
		windows.CloseHandle(h)
		return nil
	}
	h, err := windows.OpenProcess(windows.PROCESS_TERMINATE, false, uint32(pid))
	if err != nil {
		return fmt.Errorf("%d: %v", pid, err)
	}
	defer windows.CloseHandle(h)
	if err := windows.TerminateProcess(h, 1); err != nil {
		return fmt.Errorf("%d: %v", pid, err)
	}
	return nil
}

func signalNames() []string { return []string{"KILL", "TERM"} }

func signalNumber(name string) int {
	switch name {
	case "EXIT":
		return 0
	case "KILL":
		return 9
	case "TERM":
		return 15
	}
	return 0
}
