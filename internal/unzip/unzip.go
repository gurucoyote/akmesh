// Package unzip extracts zip archives with archive/zip: the deflate and store
// methods need nothing else, and Zip64 comes with the library. gzip and cpio do
// not read zip, and it is the second most used missing command (acmesh-f1e).
//
// What it does not do: encrypted entries (there is no zipcrypto in Go's
// standard library) and non-UTF-8 name guessing. Both are reported instead of
// producing half an archive.
package unzip

import (
	"archive/zip"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

// Run implements unzip [-l] [-o] [-q] [-p] [-d DIR] FILE [MEMBER...]
func Run(cmd string, args []string, stdin io.Reader, stdout, stderr io.Writer) int {
	list := false
	overwrite := false
	quiet := false
	toStdout := false
	dir := "."
	var rest []string
	for i := 0; i < len(args); i++ {
		a := args[i]
		switch {
		case a == "-l" || a == "--list":
			list = true
		case a == "-o" || a == "--overwrite":
			overwrite = true
		case a == "-q" || a == "--quiet":
			quiet = true
		case a == "-p":
			toStdout = true
			quiet = true
		case a == "-d":
			i++
			if i >= len(args) {
				fmt.Fprintln(stderr, "unzip: -d needs a directory")
				return 1
			}
			dir = args[i]
		case a == "--":
			rest = append(rest, args[i+1:]...)
			i = len(args)
		case len(a) > 1 && a[0] == '-':
			fmt.Fprintf(stderr, "unzip: unrecognized option '%s'\n", a)
			return 1
		default:
			rest = append(rest, a)
		}
	}
	if len(rest) == 0 {
		fmt.Fprintln(stderr, "unzip: usage: unzip [-l] [-o] [-q] [-d DIR] FILE [MEMBER...]")
		return 1
	}
	archive, members := rest[0], rest[1:]
	r, err := zip.OpenReader(archive)
	if err != nil {
		fmt.Fprintf(stderr, "unzip: %v\n", err)
		return 1
	}
	defer r.Close()

	want := func(name string) bool {
		if len(members) == 0 {
			return true
		}
		for _, m := range members {
			if m == name || strings.HasPrefix(name, strings.TrimSuffix(m, "/")+"/") {
				return true
			}
		}
		return false
	}

	if list {
		return listArchive(r, want, stdout, stderr)
	}

	code := 0
	for _, f := range r.File {
		if !want(f.Name) {
			continue
		}
		if f.FileInfo().IsDir() {
			if !toStdout {
				if err := os.MkdirAll(filepath.Join(dir, safeName(f.Name)), 0o755); err != nil {
					fmt.Fprintf(stderr, "unzip: %v\n", err)
					code = 1
				}
			}
			continue
		}
		if toStdout {
			if err := copyEntry(f, stdout); err != nil {
				fmt.Fprintf(stderr, "unzip: %s: %v\n", f.Name, err)
				code = 1
			}
			continue
		}
		target := filepath.Join(dir, safeName(f.Name))
		if !overwrite {
			if _, err := os.Lstat(target); err == nil {
				if !quiet {
					fmt.Fprintf(stderr, "unzip: %s already exists, not replaced (use -o)\n", target)
				}
				code = 1
				continue
			}
		}
		if err := extractOne(f, target); err != nil {
			fmt.Fprintf(stderr, "unzip: %s: %v\n", f.Name, err)
			code = 1
			continue
		}
		if !quiet {
			fmt.Fprintf(stdout, "  inflating: %s\n", target)
		}
	}
	return code
}

// listArchive prints the table unzip -l shows: lengths, date and name.
func listArchive(r *zip.ReadCloser, want func(string) bool, stdout, stderr io.Writer) int {
	var total uint64
	fmt.Fprintln(stdout, "  Length      Date    Time    Name")
	for _, f := range r.File {
		if !want(f.Name) {
			continue
		}
		total += f.UncompressedSize64
		fmt.Fprintf(stdout, "%8d  %s  %s\n", f.UncompressedSize64,
			f.Modified.Format("2006-01-02 15:04"), f.Name)
	}
	fmt.Fprintf(stdout, "%8d  %d file(s)\n", total, len(r.File))
	return 0
}

// copyEntry writes one member to a writer.
func copyEntry(f *zip.File, w io.Writer) error {
	rc, err := f.Open()
	if err != nil {
		return cleanErr(err)
	}
	defer rc.Close()
	_, err = io.Copy(w, rc)
	return cleanErr(err)
}

// extractOne writes one member to disk with its permissions and modification
// time, creating the directories it needs.
func extractOne(f *zip.File, target string) error {
	if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
		return err
	}
	mode := f.Mode()
	if mode == 0 || mode.IsDir() {
		mode = 0o644
	}
	out, err := os.OpenFile(target, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, mode.Perm())
	if err != nil {
		return err
	}
	rc, err := f.Open()
	if err != nil {
		out.Close()
		return cleanErr(err)
	}
	defer rc.Close()
	if _, err := io.Copy(out, rc); err != nil {
		out.Close()
		return cleanErr(err)
	}
	if err := out.Close(); err != nil {
		return err
	}
	// A file that says it is executable has to stay executable.
	if mode&0o111 != 0 {
		_ = os.Chmod(target, mode.Perm())
	}
	if !f.Modified.IsZero() {
		_ = os.Chtimes(target, f.Modified, f.Modified)
	}
	return nil
}

// safeName keeps an entry inside the target directory: an archive may carry
// "../" or an absolute path, and unpacking that is how a zip escapes its
// directory (zip slip).
func safeName(name string) string {
	name = strings.ReplaceAll(name, "\\", "/")
	parts := strings.Split(name, "/")
	var clean []string
	for _, p := range parts {
		switch p {
		case "", ".", "..":
			continue
		}
		// A Windows drive letter is not a directory.
		if len(p) == 2 && p[1] == ':' {
			continue
		}
		clean = append(clean, p)
	}
	return filepath.Join(clean...)
}

// cleanErr shortens the archive/zip error for an entry that cannot be read.
func cleanErr(err error) error {
	if err == nil {
		return nil
	}
	if err == io.ErrUnexpectedEOF {
		return fmt.Errorf("truncated entry")
	}
	return err
}
