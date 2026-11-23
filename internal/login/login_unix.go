//go:build !windows

// Package login starts a session as another user: it reads /etc/passwd, drops
// privileges, sets the environment and runs the user's shell as a login shell.
//
// It is the second half of acmesh-4g7: the shell knows how to read
// /etc/profile and ~/.profile, but only a session that starts as the right user
// can offer that to the right home directory.
package login

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
)

// The paths and the two system calls are variables so that a test can point
// them at a temporary file and see what would have been called. Nothing else in
// this package guesses: a login that cannot set the ids fails, it does not fall
// back to the current user.
var (
	passwdFile = "/etc/passwd"
	setIDs     = setIDsReal
	execShell  = syscall.Exec
)

// Account is one line of /etc/passwd, the five fields a login needs.
type Account struct {
	Name  string
	UID   int
	GID   int
	Home  string
	Shell string
}

// Run is the command entry point.
func Run(args []string, stdin io.Reader, stdout, stderr io.Writer) int {
	name := ""
	force := false
	for i := 0; i < len(args); i++ {
		switch a := args[i]; {
		case a == "--help" || a == "-h":
			usage(stdout)
			return 0
		case a == "-f":
			force = true
		case a == "-p":
			fmt.Fprintln(stderr, "login: -p is accepted and has no effect: the environment is always kept")
		case a == "--":
			if i+1 < len(args) {
				name = args[i+1]
			}
			i = len(args)
		case strings.HasPrefix(a, "-"):
			fmt.Fprintf(stderr, "login: unsupported option %q\n", a)
			return 2
		default:
			name = a
		}
	}
	_ = stdin

	if name == "" {
		fmt.Fprintln(stderr, "login: name the user: login [-f] USER")
		return 1
	}
	acct, lookupErr := lookup(name)
	if !force {
		// The password is asked for before the name is judged: an unknown name
		// and a wrong password must look the same from the outside.
		authErr := authenticate(name, stdin, stderr)
		if lookupErr != nil || authErr != nil {
			if lookupErr != nil {
				fmt.Fprintln(stderr, "login: "+errIncorrect.Error())
			} else {
				fmt.Fprintf(stderr, "login: %v\n", authErr)
			}
			return 1
		}
	}
	if lookupErr != nil {
		fmt.Fprintf(stderr, "login: %v\n", lookupErr)
		return 1
	}
	if err := start(acct, stdout, stderr); err != nil {
		fmt.Fprintf(stderr, "login: %v\n", err)
		return 1
	}
	return 0
}

// lookup finds the account by name. An empty database is an error with a
// sentence a reader can act on, not a silent numeric fallback.
func lookup(name string) (Account, error) {
	f, err := os.Open(passwdFile)
	if err != nil {
		return Account{}, fmt.Errorf("cannot read %s: %v", passwdFile, err)
	}
	defer f.Close()
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		fields := strings.Split(line, ":")
		if len(fields) < 7 || fields[0] != name {
			continue
		}
		uid, err1 := strconv.Atoi(fields[2])
		gid, err2 := strconv.Atoi(fields[3])
		if err1 != nil || err2 != nil {
			return Account{}, fmt.Errorf("account %q has no numeric uid/gid", name)
		}
		shell := fields[6]
		if shell == "" {
			shell = "/bin/sh"
		}
		return Account{Name: fields[0], UID: uid, GID: gid, Home: fields[5], Shell: shell}, nil
	}
	if err := sc.Err(); err != nil {
		return Account{}, err
	}
	return Account{}, fmt.Errorf("unknown user %q", name)
}

// start switches to the account and hands the process to its shell.
func start(a Account, stdout, stderr io.Writer) error {
	if err := setIDs(a.UID, a.GID); err != nil {
		return err
	}
	home := a.Home
	if err := os.Chdir(home); err != nil {
		// A home that does not exist is a warning, not a failed login: the
		// session still has to start, in /.
		fmt.Fprintf(stderr, "login: %s: %v, starting in /\n", home, err)
		if err := os.Chdir("/"); err != nil {
			return err
		}
	}
	env := sessionEnv(a)
	argv, err := shellArgv(a.Shell)
	if err != nil {
		return err
	}
	_ = stdout
	return execShell(argv[0], argv, env)
}

// sessionEnv is the environment a login shell expects. The user's own values
// win over whatever the caller had, or a session could inherit another user's
// HOME.
func sessionEnv(a Account) []string {
	keep := map[string]bool{"HOME": true, "USER": true, "LOGNAME": true, "SHELL": true, "PATH": true}
	var env []string
	for _, kv := range os.Environ() {
		name, _, _ := strings.Cut(kv, "=")
		if !keep[name] {
			env = append(env, kv)
		}
	}
	path := os.Getenv("PATH")
	if path == "" {
		path = "/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin"
	}
	env = append(env,
		"HOME="+a.Home,
		"USER="+a.Name,
		"LOGNAME="+a.Name,
		"SHELL="+a.Shell,
		"PATH="+path,
	)
	return env
}

// shellArgv turns the shell field into the call. A shell this system reaches as
// one of our own names is called with -l, which is what makes it read
// /etc/profile and then the user's profile.
func shellArgv(shell string) ([]string, error) {
	if shell == "" {
		shell = "/bin/sh"
	}
	if _, err := os.Stat(shell); err != nil {
		return nil, fmt.Errorf("shell %s: %v", shell, err)
	}
	argv := []string{shell, "-l"}
	if base := filepath.Base(shell); base != "sh" && base != "bash" {
		// Only sh/bash readings exist here; another shell keeps its own flags.
		argv = []string{shell}
	}
	return argv, nil
}

func usage(w io.Writer) {
	fmt.Fprintln(w, "usage: login [-f] USER")
	fmt.Fprintln(w)
	fmt.Fprintln(w, "  -f    log in without a password (the only mode in this build)")
	fmt.Fprintln(w)
	fmt.Fprintln(w, "Reads /etc/passwd, switches to the user's uid/gid, sets HOME, USER,")
	fmt.Fprintln(w, "LOGNAME, SHELL and PATH and starts the shell with -l so that")
	fmt.Fprintln(w, "/etc/profile and ~/.profile are read.")
}

// setIDsReal drops root: group first, then supplementary groups (dropped), then
// the user. The order matters; a gid set after the uid cannot succeed.
func setIDsReal(uid, gid int) error {
	if os.Geteuid() != 0 {
		if os.Geteuid() == uid && os.Getegid() == gid {
			return nil
		}
		return fmt.Errorf("must be root to change to uid %d", uid)
	}
	if err := syscall.Setgroups([]int{gid}); err != nil {
		return err
	}
	if err := syscall.Setgid(gid); err != nil {
		return err
	}
	return syscall.Setuid(uid)
}
