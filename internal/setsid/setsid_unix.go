//go:build !windows

// Package setsid starts a program in its own session, so that closing the
// terminal it was started from does not take it down. An agent uses it for
// anything that has to outlive the shell call that started it (acmesh-vgr).
package setsid

import (
	"fmt"
	"os"
	"os/exec"
	"syscall"
)

// Run starts PROG in a new session and waits, like the util-linux setsid.
//
//	setsid [-c] [-w] PROG [ARGS...]
//
// -w waits for the program and returns its exit code (default when not
// detached); -c keeps the current working directory, which is the default here
// anyway - the flag is accepted so that scripts do not fail on it.
func Run(cmd string, args []string, stdin *os.File, stdout, stderr *os.File) int {
	wait := true
	var rest []string
	for i, a := range args {
		switch a {
		case "-w", "--wait":
			wait = true
			continue
		case "-c", "--ctty":
			continue
		case "-f", "--fork":
			wait = false
			continue
		case "--":
			rest = append(rest, args[i+1:]...)
		default:
			rest = append(rest, args[i:]...)
		}
		break
	}
	if len(rest) == 0 {
		fmt.Fprintln(stderr, "setsid: usage: setsid [-w] [-f] PROGRAM [ARGS...]")
		return 127
	}
	child := exec.Command(rest[0], rest[1:]...)
	child.Stdin, child.Stdout, child.Stderr = stdin, stdout, stderr
	child.SysProcAttr = &syscall.SysProcAttr{Setsid: true}
	if err := child.Start(); err != nil {
		fmt.Fprintf(stderr, "setsid: %v\n", err)
		return 127
	}
	if !wait {
		return 0
	}
	if err := child.Wait(); err != nil {
		if ee, ok := err.(*exec.ExitError); ok {
			return ee.ExitCode()
		}
		fmt.Fprintf(stderr, "setsid: %v\n", err)
		return 1
	}
	return 0
}
