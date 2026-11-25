package toolsvc

import (
	"os"

	"akmesh/internal/syslog"
)

// The three logging commands share one package but keep their own entry point,
// like BusyBox' applets: syslogd writes, logger sends, logread reads.
func runSyslogd(args []string, stdin *os.File, stdout, stderr *os.File) int {
	return syslog.RunSyslogd("syslogd", args, stdout, stderr)
}

func runLogger(args []string, stdin *os.File, stdout, stderr *os.File) int {
	return syslog.RunLogger("logger", args, stdin, stdout, stderr)
}

func runLogread(args []string, stdin *os.File, stdout, stderr *os.File) int {
	return syslog.RunLogread("logread", args, stdout, stderr)
}
