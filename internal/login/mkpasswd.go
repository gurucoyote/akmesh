package login

import (
	"bufio"
	"crypto/rand"
	"fmt"
	"io"
	"strings"
)

// RunMkpasswd prints a crypt hash for a password: "mkpasswd -m sha-512" is what
// scripts use when they fill a shadow file (acmesh-a4d).
//
//	mkpasswd [-m sha-512|sha-256] [-s SALT] [PASSWORD]
func RunMkpasswd(cmd string, args []string, stdin io.Reader, stdout, stderr io.Writer) int {
	method := "sha-512"
	salt := ""
	var rest []string
	for i := 0; i < len(args); i++ {
		switch a := args[i]; {
		case a == "-m" || a == "--method":
			i++
			if i >= len(args) {
				fmt.Fprintln(stderr, "mkpasswd: -m needs a method")
				return 1
			}
			method = args[i]
		case a == "-s" || a == "--salt":
			i++
			if i >= len(args) {
				fmt.Fprintln(stderr, "mkpasswd: -s needs a salt")
				return 1
			}
			salt = args[i]
		case a == "--":
			rest = append(rest, args[i+1:]...)
			i = len(args)
		case len(a) > 1 && a[0] == '-':
			fmt.Fprintf(stderr, "mkpasswd: unrecognized option '%s'\n", a)
			return 1
		default:
			rest = append(rest, a)
		}
	}
	password := ""
	if len(rest) > 0 {
		password = rest[0]
	} else {
		line, err := readOneLine(stdin)
		if err != nil && err != io.EOF {
			fmt.Fprintf(stderr, "mkpasswd: %v\n", err)
			return 1
		}
		password = line
	}
	if password == "" {
		fmt.Fprintln(stderr, "mkpasswd: no password given")
		return 1
	}
	prefix, err := methodPrefix(method)
	if err != nil {
		fmt.Fprintf(stderr, "mkpasswd: %v\n", err)
		return 1
	}
	if salt == "" {
		salt, err = randomSalt(16)
		if err != nil {
			fmt.Fprintf(stderr, "mkpasswd: %v\n", err)
			return 1
		}
	}
	hash, err := shaCrypt(password, prefix+salt)
	if err != nil {
		fmt.Fprintf(stderr, "mkpasswd: %v\n", err)
		return 1
	}
	fmt.Fprintln(stdout, hash)
	return 0
}

// methodPrefix maps the names mkpasswd accepts onto crypt settings. Only the two
// SHA crypt forms are built in: md5 and yescrypt would need code this binary
// does not carry, and a wrong hash in a shadow file locks people out.
func methodPrefix(method string) (string, error) {
	m := strings.ToLower(strings.ReplaceAll(method, "_", "-"))
	switch m {
	case "sha-512", "sha512", "6":
		return "$6$", nil
	case "sha-256", "sha256", "5":
		return "$5$", nil
	}
	return "", fmt.Errorf("method %q is not supported (sha-512 and sha-256 are)", method)
}

// readOneLine reads a single line, which is how mkpasswd takes a password on
// standard input.
func readOneLine(r io.Reader) (string, error) {
	if r == nil {
		return "", io.EOF
	}
	line, err := bufio.NewReader(r).ReadString('\n')
	return strings.TrimRight(line, "\r\n"), err
}

// randomSalt builds a salt of the shape crypt wants: the alphabet is the crypt
// base64, and every byte is taken from crypto/rand.
func randomSalt(n int) (string, error) {
	const alphabet = "./0123456789ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz"
	buf := make([]byte, n)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	out := make([]byte, n)
	for i, b := range buf {
		out[i] = alphabet[int(b)%len(alphabet)]
	}
	return string(out), nil
}
