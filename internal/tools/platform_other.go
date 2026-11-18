//go:build !linux && !windows

package tools

import (
	"errors"
	"os"
	"time"
)

// Stubs for the wasm build. wasm has no processes, no signals and no statfs, so
// these commands report that they are unavailable instead of guessing. Only the
// symbols the compiler asks for live here; whatever the loader needs next gets
// added when a wasm build actually complains about it.

var errNoProcesses = errors.New("processes and signals are not available on this platform")

func normaliseSignal(name string) (string, error) { return "", errNoProcesses }

func signalProcess(pid int, name string) error { return errNoProcesses }

func signalNames() []string { return nil }

func signalNumber(string) int { return 0 }

func listProcesses() ([]procInfo, error) { return nil, errNoProcesses }

func readBootTime() time.Time { return time.Time{} }

func readPasswd() map[string]string { return map[string]string{} }

func readMemory() (memInfo, memInfo, error) { return memInfo{}, memInfo{}, errNoProcesses }

func readFileSystems() ([]diskRow, error) { return nil, errNoProcesses }

func inodeOf(info os.FileInfo) string { return "" }

func wholeSystemSync() error { return errNoProcesses }

// The storage helpers below are pure path arithmetic on Linux; on wasm there is
// no statfs to fill the tables with, so the numbers stay unavailable.
func absPath(p string) (string, error) { return p, nil }

func joinPath(root, name string) (string, error) { return root + "/" + name, nil }

func usageOf(info os.FileInfo, path string) int64 { return 0 }

func keyOf(info os.FileInfo) (fileKey, bool) { return fileKey{}, false }
