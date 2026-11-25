//go:build !windows

package login

import (
	"bufio"
	"fmt"
	"io"

	"golang.org/x/term"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

// RunPasswd sets a password: it reads the new one twice, hashes it with SHA-512
// crypt and rewrites the user's line in /etc/shadow. Changing another user's
// password needs root; changing your own does not, as everywhere else
// (acmesh-a4d).
//
//	passwd [USER]
func RunPasswd(cmd string, args []string, stdin io.Reader, stdout, stderr io.Writer) int {
	var rest []string
	for _, a := range args {
		if len(a) > 1 && a[0] == '-' {
			fmt.Fprintf(stderr, "passwd: unrecognized option '%s'\n", a)
			return 1
		}
		rest = append(rest, a)
	}
	if len(rest) > 1 {
		fmt.Fprintln(stderr, "passwd: usage: passwd [USER]")
		return 1
	}
	name := ""
	if len(rest) == 1 {
		name = rest[0]
	} else {
		n, err := lookupUID(os.Geteuid())
		if err != nil {
			fmt.Fprintf(stderr, "passwd: %v\n", err)
			return 1
		}
		name = n
	}
	acct, err := lookup(name)
	if err != nil {
		fmt.Fprintf(stderr, "passwd: %v\n", err)
		return 1
	}
	if acct.UID != os.Geteuid() && os.Geteuid() != 0 {
		fmt.Fprintln(stderr, "passwd: only root can change another user's password")
		return 1
	}
	// The current password is asked for when a user changes their own; root is
	// not asked, which is what every passwd does.
	if acct.UID == os.Geteuid() && os.Geteuid() != 0 {
		if err := authenticate(acct.Name, stdin, stderr); err != nil {
			fmt.Fprintln(stderr, "passwd: Authentication failure")
			return 1
		}
	}
	first, second, err := askTwice(stdin, stdout)
	if err != nil {
		fmt.Fprintf(stderr, "passwd: %v\n", err)
		return 1
	}
	if first == "" {
		fmt.Fprintln(stderr, "passwd: no password supplied")
		return 1
	}
	if first != second {
		fmt.Fprintln(stderr, "passwd: the passwords do not match")
		return 1
	}
	salt, err := randomSalt(16)
	if err != nil {
		fmt.Fprintf(stderr, "passwd: %v\n", err)
		return 1
	}
	hash, err := shaCrypt(first, "$6$"+salt)
	if err != nil {
		fmt.Fprintf(stderr, "passwd: %v\n", err)
		return 1
	}
	if err := writeShadowHash(acct.Name, hash); err != nil {
		fmt.Fprintf(stderr, "passwd: %v\n", err)
		return 1
	}
	fmt.Fprintln(stdout, "passwd: password updated successfully")
	return 0
}

// writeShadowHash replaces the second field of one shadow line and leaves every
// other byte alone. The new file is written next to the old one and renamed, so
// an interrupted write cannot leave a half shadow file behind - that would lock
// every account out.
func writeShadowHash(name, hash string) error {
	data, err := os.ReadFile(shadowFile)
	if err != nil {
		return fmt.Errorf("cannot read %s: %v", shadowFile, err)
	}
	lines := strings.Split(string(data), "\n")
	found := false
	for i, line := range lines {
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		f := strings.Split(line, ":")
		if len(f) < 2 || f[0] != name {
			continue
		}
		f[1] = hash
		lines[i] = strings.Join(f, ":")
		found = true
		break
	}
	if !found {
		return fmt.Errorf("no shadow entry for %q", name)
	}
	dir := filepath.Dir(shadowFile)
	tmp, err := os.CreateTemp(dir, ".shadow-")
	if err != nil {
		return fmt.Errorf("cannot write in %s: %v", dir, err)
	}
	tmpName := tmp.Name()
	defer os.Remove(tmpName) // no-op after the rename succeeded
	if _, err := tmp.WriteString(strings.Join(lines, "\n")); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Chmod(0o600); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	return os.Rename(tmpName, shadowFile)
}

// lookupUID finds the account name for a numeric uid, for the case where passwd
// is called without a name.
func lookupUID(uid int) (string, error) {
	f, err := os.Open(passwdFile)
	if err != nil {
		return "", fmt.Errorf("cannot read %s: %v", passwdFile, err)
	}
	defer f.Close()
	data, err := io.ReadAll(f)
	if err != nil {
		return "", err
	}
	for line := range strings.SplitSeq(string(data), "\n") {
		fields := strings.Split(line, ":")
		if len(fields) < 4 {
			continue
		}
		if n, err := strconv.Atoi(fields[2]); err == nil && n == uid {
			return fields[0], nil
		}
	}
	return "", fmt.Errorf("no account with uid %d", uid)
}

// askTwice asks for the password and its repetition. Both lines come from one
// reader: two bufio.Reader on the same pipe swallow each other's input, which
// showed up as "passwd: EOF" when both lines arrived together.
func askTwice(stdin io.Reader, stdout io.Writer) (string, string, error) {
	if f, ok := stdin.(*os.File); ok && term.IsTerminal(int(f.Fd())) {
		fmt.Fprintf(stdout, "New password: ")
		first, err := term.ReadPassword(int(f.Fd()))
		fmt.Fprintln(stdout)
		if err != nil {
			return "", "", err
		}
		fmt.Fprintf(stdout, "Retype new password: ")
		second, err := term.ReadPassword(int(f.Fd()))
		fmt.Fprintln(stdout)
		if err != nil {
			return "", "", err
		}
		return string(first), string(second), nil
	}
	br := bufio.NewReader(stdin)
	fmt.Fprintf(stdout, "New password: ")
	first, err := br.ReadString('\n')
	fmt.Fprintln(stdout)
	if err != nil && first == "" {
		return "", "", err
	}
	fmt.Fprintf(stdout, "Retype new password: ")
	second, err := br.ReadString('\n')
	fmt.Fprintln(stdout)
	if err != nil && second == "" {
		return "", "", err
	}
	return strings.TrimRight(first, "\r\n"), strings.TrimRight(second, "\r\n"), nil
}
