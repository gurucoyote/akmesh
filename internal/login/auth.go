//go:build !windows

package login

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	"golang.org/x/term"
)

// errIncorrect is one answer for every failure that would tell a stranger which
// names exist on the machine.
var errIncorrect = errors.New("incorrect name or password")

// failDelay slows a wrong password down. A variable, so a test does not wait two
// seconds per case.
var failDelay = 2 * time.Second

// readPasswordFn reads the password without echo on a terminal. A variable for
// the same reason.
var readPasswordFn = readPassword

// authenticate asks for the password and checks it against /etc/shadow.
//
// The order is deliberate: the password is read even when the account is
// unknown, so a wrong name and a wrong password behave the same way from the
// outside.
func authenticate(name string, stdin io.Reader, stderr io.Writer) error {
	e, shadowErr := readShadow(name)

	fmt.Fprintf(stderr, "Password: ")
	pw, err := readPasswordFn(stdin)
	fmt.Fprintln(stderr)
	if err != nil {
		return errIncorrect
	}
	if shadowErr != nil {
		time.Sleep(failDelay)
		return errIncorrect
	}
	switch e.State {
	case "locked":
		return errors.New("account is locked")
	case "expired":
		return errors.New("account has expired")
	case "no-password":
		return errors.New("account has no password, so nothing can be checked")
	}
	ok, verr := verifyPassword(e.Hash, pw)
	if verr != nil {
		return verr
	}
	if !ok {
		time.Sleep(failDelay)
		return errIncorrect
	}
	return nil
}

// readPassword reads one line with the terminal echo switched off. Without a
// terminal (a pipe, a test) it reads the line as it comes.
func readPassword(stdin io.Reader) (string, error) {
	if f, ok := stdin.(*os.File); ok && term.IsTerminal(int(f.Fd())) {
		b, err := term.ReadPassword(int(f.Fd()))
		if err != nil {
			return "", err
		}
		return string(b), nil
	}
	line, err := bufio.NewReader(stdin).ReadString('\n')
	if err != nil && line == "" {
		return "", err
	}
	return strings.TrimRight(line, "\r\n"), nil
}

// openShadow opens the shadow database. Kept here so that the path stays one
// decision in one file.
func openShadow() (io.ReadCloser, error) {
	f, err := os.Open(shadowFile)
	if err != nil {
		return nil, fmt.Errorf("cannot read %s: %v", shadowFile, err)
	}
	return f, nil
}

func newScanner(r io.Reader) *bufio.Scanner {
	return bufio.NewScanner(r)
}
