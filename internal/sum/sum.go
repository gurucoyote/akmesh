// Package sum implements md5sum, sha1sum, sha256sum and sha512sum, plus the
// -c form that checks a list of checksums against the files it names.
//
// The checksum family lives in its own package so that a build which carries
// one of them carries only this code (acmesh-mq1). The commands used to be one
// entry in internal/tools, whose table pulls in every other tool.
package sum

import (
	"bufio"
	"crypto/md5"
	"crypto/sha1"
	"crypto/sha256"
	"crypto/sha512"
	"encoding/hex"
	"fmt"
	"hash"
	"io"
	"os"
	"strings"
)

// Run executes one checksum command. kind is "md5", "sha1", "sha256" or
// "sha512"; the output format is "<hex>  <name>", with "-" for standard input.
func Run(kind string, args []string, stdin io.Reader, stdout, stderr io.Writer) int {
	var files []string
	check := false
	quiet := false
	for _, a := range args {
		switch a {
		case "-c", "--check":
			check = true
		case "-b", "--binary", "-t", "--text":
			// text and binary mode are the same here
		case "--quiet", "--status":
			quiet = true
		default:
			if len(a) > 0 && a[0] == '-' && a != "-" {
				fmt.Fprintf(stderr, "%ssum: unknown option %q\n", kind, a)
				return 2
			}
			files = append(files, a)
		}
	}
	if check {
		if len(files) == 0 {
			files = []string{"-"}
		}
		return checkFiles(kind, files, stdin, stdout, stderr, quiet)
	}
	if len(files) == 0 {
		files = []string{"-"}
	}
	code := 0
	for _, name := range files {
		sum, err := fileSum(kind, name, stdin)
		if err != nil {
			fmt.Fprintf(stderr, "%ssum: %v\n", kind, err)
			code = 1
			continue
		}
		fmt.Fprintf(stdout, "%s  %s\n", sum, name)
	}
	return code
}

// checkFiles reads "hex  name" lines and verifies them. It prints the same
// three words GNU prints and exits 1 when a checksum did not match, so a script
// that checks `sha256sum -c` behaves the same as with the real tool.
func checkFiles(kind string, lists []string, stdin io.Reader, stdout, stderr io.Writer, quiet bool) int {
	failed := 0
	unreadable := 0
	checked := 0
	for _, list := range lists {
		var r io.Reader = stdin
		if list != "-" {
			f, err := os.Open(list)
			if err != nil {
				fmt.Fprintf(stderr, "%ssum: %v\n", kind, err)
				return 2
			}
			defer f.Close()
			r = f
		}
		scanner := bufio.NewScanner(r)
		scanner.Buffer(make([]byte, 0, 64*1024), 8*1024*1024)
		before := checked
		for scanner.Scan() {
			line := strings.TrimSpace(scanner.Text())
			if line == "" {
				continue
			}
			want, name, ok := splitLine(line)
			if !ok {
				fmt.Fprintf(stderr, "%ssum: %s: improperly formatted checksum line\n", kind, list)
				continue
			}
			checked++
			got, err := fileSum(kind, name, stdin)
			if err != nil {
				// An unreadable file is not a wrong checksum: GNU counts the
				// two apart and says which it was.
				if !quiet {
					fmt.Fprintf(stdout, "%s: FAILED open or read\n", name)
					fmt.Fprintf(stderr, "%ssum: %s: %s\n", kind, name, gnuMessage(err))
				}
				unreadable++
				continue
			}
			if !strings.EqualFold(got, want) {
				if !quiet {
					fmt.Fprintf(stdout, "%s: FAILED\n", name)
				}
				failed++
				continue
			}
			if !quiet {
				fmt.Fprintf(stdout, "%s: OK\n", name)
			}
		}
		if err := scanner.Err(); err != nil {
			fmt.Fprintf(stderr, "%ssum: %v\n", kind, err)
			return 2
		}
		// GNU names the list that had no usable line: "sha256sum: file: no
		// properly formatted checksum lines found", and 'standard input' for
		// stdin (quoted, because it carries a space).
		if checked == before && !quiet {
			fmt.Fprintf(stderr, "%ssum: %s: no properly formatted checksum lines found\n", kind, listName(list))
		}
	}
	if failed > 0 && !quiet {
		fmt.Fprintf(stderr, "%ssum: WARNING: %d computed checksum%s did NOT match\n",
			kind, failed, plural(failed))
	}
	if unreadable > 0 && !quiet {
		fmt.Fprintf(stderr, "%ssum: WARNING: %d listed file%s could not be read\n",
			kind, unreadable, pluralS(unreadable))
		if failed == 0 {
			// GNU counts the missing files as failures for the exit status.
			return 1
		}
	}
	if failed > 0 || unreadable > 0 {
		return 1
	}
	if checked == 0 {
		return 1
	}
	return 0
}

// listName renders the list the way GNU does in its messages: the file as
// given, and 'standard input' (with quotes) when the list came from stdin.
func listName(list string) string {
	if list == "-" {
		return "'standard input'"
	}
	if strings.ContainsAny(list, " 	'\"") {
		return "'" + list + "'"
	}
	return list
}

// splitLine reads one line of a checksum list. GNU writes the name after two
// spaces or after " *", and keeps a leading "*" (binary mode) away from it.
func splitLine(line string) (string, string, bool) {
	if i := strings.Index(line, "  "); i > 0 {
		return line[:i], line[i+2:], true
	}
	if i := strings.Index(line, " *"); i > 0 {
		return line[:i], line[i+2:], true
	}
	return "", "", false
}

// gnuMessage renders an open error the way GNU does: "name: No such file or
// directory" instead of Go's "open name: no such file or directory".
func gnuMessage(err error) string {
	if os.IsNotExist(err) {
		return "No such file or directory"
	}
	if os.IsPermission(err) {
		return "Permission denied"
	}
	return err.Error()
}

func pluralS(n int) string {
	if n == 1 {
		return ""
	}
	return "s"
}

func plural(n int) string {
	if n == 1 {
		return ""
	}
	return "s"
}

func fileSum(kind, name string, stdin io.Reader) (string, error) {
	var r io.Reader = stdin
	if name != "-" {
		f, err := os.Open(name)
		if err != nil {
			return "", err
		}
		defer f.Close()
		r = f
	}
	h, err := newHash(kind)
	if err != nil {
		return "", err
	}
	if _, err := io.Copy(h, r); err != nil {
		return "", err
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}

func newHash(kind string) (hash.Hash, error) {
	switch kind {
	case "md5":
		return md5.New(), nil
	case "sha1":
		return sha1.New(), nil
	case "sha256":
		return sha256.New(), nil
	case "sha512":
		return sha512.New(), nil
	}
	return nil, fmt.Errorf("unknown hash %q", kind)
}
