//go:build !windows

package login

import (
	"fmt"
	"io"
	"os"
	"os/exec"
)

// RunSu is su: verify the target user's password (unless the caller already is
// root), then run that user's shell in this process. An agent that needs one
// command as another user writes "su -c 'cmd' user"; the one-shot form is what
// su is used for here, not a lasting session (acmesh-a4d).
//
//	su [-] [-c COMMAND] [USER]
func RunSu(cmd string, args []string, stdin io.Reader, stdout, stderr io.Writer) int {
	command := ""
	session := false
	var rest []string
	for i := 0; i < len(args); i++ {
		switch a := args[i]; {
		case a == "-" || a == "-l" || a == "--login":
			session = true
		case a == "-c" || a == "--command":
			i++
			if i >= len(args) {
				fmt.Fprintln(stderr, "su: -c needs a command")
				return 1
			}
			command = args[i]
		case a == "--":
			rest = append(rest, args[i+1:]...)
			i = len(args)
		case len(a) > 1 && a[0] == '-':
			fmt.Fprintf(stderr, "su: unrecognized option '%s'\n", a)
			return 1
		default:
			rest = append(rest, a)
		}
	}
	name := "root"
	if len(rest) > 0 {
		name = rest[0]
	}
	acct, err := lookup(name)
	if err != nil {
		fmt.Fprintf(stderr, "su: %v\n", err)
		return 1
	}
	if os.Geteuid() != 0 {
		if err := authenticate(acct.Name, stdin, stderr); err != nil {
			fmt.Fprintf(stderr, "su: Authentication failure\n")
			return 1
		}
	}
	if err := setIDs(acct.UID, acct.GID); err != nil {
		fmt.Fprintf(stderr, "su: %v\n", err)
		return 1
	}
	if session || command != "" {
		// A login shell starts in the user's home, an agent's "su -c" does not
		// care but gets it anyway: the environment has to match the user.
		if acct.Home != "" {
			_ = os.Chdir(acct.Home)
		}
		os.Setenv("HOME", acct.Home)
		os.Setenv("USER", acct.Name)
		os.Setenv("LOGNAME", acct.Name)
		os.Setenv("SHELL", acct.Shell)
	}
	if command != "" {
		// -c goes through the shell, as su always did.
		return runShellCommand(acct.Shell, command, stdin, stdout, stderr)
	}
	argv := []string{acct.Shell}
	if session {
		// A login shell is called with a leading dash in argv[0], which is how
		// it knows to read the profile files.
		argv[0] = "-" + acct.Shell
	}
	if err := execShell(acct.Shell, argv, os.Environ()); err != nil {
		fmt.Fprintf(stderr, "su: %v\n", err)
		return 1
	}
	return 0
}

// runShellCommand runs one command through the user's shell and returns its exit
// code, which is what "su -c" has to hand back to the script.
func runShellCommand(shell, command string, stdin io.Reader, stdout, stderr io.Writer) int {
	if shell == "" {
		shell = "/bin/sh"
	}
	cmd := exec.Command(shell, "-c", command)
	// exec.Command wants *os.File for these; a caller that passes a plain
	// reader (a test, a pipe) gets the process' own stdin instead.
	if f, ok := stdin.(*os.File); ok {
		cmd.Stdin = f
	}
	cmd.Stdout, cmd.Stderr = stdout, stderr
	if err := cmd.Run(); err != nil {
		if ee, ok := err.(*exec.ExitError); ok {
			return ee.ExitCode()
		}
		fmt.Fprintf(stderr, "su: %v\n", err)
		return 1
	}
	return 0
}
