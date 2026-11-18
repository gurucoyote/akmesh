//go:build !linux && !windows

package proc

import (
	"errors"
	"time"
)

// Stubs for the wasm build: no processes, no signals. The commands report that
// they are unavailable instead of guessing.

var errNoProcesses = errors.New("processes and signals are not available on this platform")

func normaliseSignal(name string) (string, error) { return "", errNoProcesses }

func signalProcess(pid int, name string) error { return errNoProcesses }

func signalNames() []string { return nil }

func signalNumber(string) int { return 0 }

func listProcesses() ([]procInfo, error) { return nil, errNoProcesses }

func readBootTime() time.Time { return time.Time{} }

func readMemory() (memInfo, memInfo, error) { return memInfo{}, memInfo{}, errNoProcesses }
