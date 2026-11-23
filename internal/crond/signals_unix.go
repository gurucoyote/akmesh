//go:build !windows

package crond

import (
	"os"
	"os/signal"
	"syscall"
)

// jobProcAttr puts each job in its own process group, so a stop reaches the
// whole pipeline a shell started (_sh -c "a | b").
func jobProcAttr() *syscall.SysProcAttr {
	return &syscall.SysProcAttr{Setpgid: true}
}

// signalJob signals the job's process group first and the pid when that fails
// (a group can already be gone while the leader still reports).
func signalJob(pid int, hard bool) {
	sig := syscall.SIGTERM
	if hard {
		sig = syscall.SIGKILL
	}
	if err := syscall.Kill(-pid, sig); err != nil {
		_ = syscall.Kill(pid, sig)
	}
}

var (
	reloadCh chan os.Signal
	stopCh   chan os.Signal
)

func signalChannels() (chan os.Signal, chan os.Signal) {
	reloadCh = make(chan os.Signal, 1)
	stopCh = make(chan os.Signal, 2)
	signal.Notify(reloadCh, syscall.SIGHUP)
	signal.Notify(stopCh, syscall.SIGTERM, syscall.SIGINT, syscall.SIGQUIT)
	return reloadCh, stopCh
}

func stopSignals() {
	if reloadCh != nil {
		signal.Stop(reloadCh)
	}
	if stopCh != nil {
		signal.Stop(stopCh)
	}
}
