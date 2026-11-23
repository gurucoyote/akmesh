//go:build windows

package crond

import (
	"os"
	"os/signal"
	"syscall"
)

// Windows has no process groups to signal and no SIGHUP. A job is stopped by
// killing it, and the schedule is fixed for the life of the process - reloading
// would be a host trick, and this program leaves no traces (AGENTS.md).
func jobProcAttr() *syscall.SysProcAttr { return nil }

// signalJob stops one job process.
func signalJob(pid int, hard bool) {
	p, err := os.FindProcess(pid)
	if err != nil {
		return
	}
	_ = p.Kill()
}

var stopCh chan os.Signal

func signalChannels() (chan os.Signal, chan os.Signal) {
	reloadCh := make(chan os.Signal, 1) // never fires on Windows
	stopCh = make(chan os.Signal, 2)
	signal.Notify(stopCh, os.Interrupt)
	return reloadCh, stopCh
}

func stopSignals() {
	if stopCh != nil {
		signal.Stop(stopCh)
	}
}
