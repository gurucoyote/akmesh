// Copyright 2019 the u-root Authors. All rights reserved
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.
//
// Create and extract tar archives.
//
// Forked from u-root 869f00f6b224ce02b1db51806159d6cd9c246f76,
// cmds/core/tar/tar.go, together with pkg/tarutil (tarutil/tarutil.go) and
// pkg/upath/safejoin.go. The rest of this file is the upstream source and keeps
// its copyright.
//
// Local changes, all of them for the way agents call tar:
//   - package tar and Run(args, stdin, stdout, stderr) instead of main();
//   - no flag package and no unixflag: an own parser, so the bundled form every
//     agent writes works - tar -czf x.tgz d, tar -tzf x.tgz, tar -xzf x.tgz;
//   - -z writes gzip; on reading the gzip magic is detected, so an archive can
//     be listed without knowing how it was packed (GNU does the same);
//   - new: -C DIR (create: files are taken from there, extract: written there),
//     --strip-components=N, --exclude=PATTERN, -f - for stdin/stdout;
//   - -v prints the names to stdout, as GNU does;
//   - a clear message and exit 2 for -j and -J, which this binary does not
//     have, instead of a wrong archive.
package tar

import (
	"archive/tar"
	"bufio"
	"compress/gzip"
	"errors"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"

	"acmesh/internal/forked/tar/tarutil"
)

type params struct {
	file      string
	files     []string
	create    bool
	extract   bool
	list      bool
	gzip      bool
	verbose   bool
	noRecurse bool
	dir       string
	strip     int
	exclude   []string
}

var (
	errEmptyFile            = errors.New("file is required")
	errMissingMandatoryFlag = errors.New("must supply at least one of: -c, -x, -t")
)

// parse reads the option string the way tar is called: one token with all the
// switches, the value of -f and -C directly after its letter or in the next
// argument.
func parse(args []string) (p params, rest []string, err error) {
	for i := 0; i < len(args); i++ {
		a := args[i]
		switch {
		case a == "--":
			return p, append(rest, args[i+1:]...), nil
		case a == "--create":
			p.create = true
		case a == "--extract" || a == "--get":
			p.extract = true
		case a == "--list":
			p.list = true
		case a == "--gzip" || a == "--gunzip":
			p.gzip = true
		case a == "--verbose":
			p.verbose = true
		case strings.HasPrefix(a, "--file="):
			p.file = strings.TrimPrefix(a, "--file=")
		case a == "--file":
			if i+1 >= len(args) {
				return p, nil, errors.New("--file needs a value")
			}
			i++
			p.file = args[i]
		case a == "--directory":
			if i+1 >= len(args) {
				return p, nil, errors.New("--directory needs a value")
			}
			i++
			p.dir = args[i]
		case strings.HasPrefix(a, "--directory="):
			p.dir = strings.TrimPrefix(a, "--directory=")
		case strings.HasPrefix(a, "--strip-components="):
			n, err := strconv.Atoi(strings.TrimPrefix(a, "--strip-components="))
			if err != nil || n < 0 {
				return p, nil, errors.New("invalid --strip-components value")
			}
			p.strip = n
		case strings.HasPrefix(a, "--exclude="):
			p.exclude = append(p.exclude, strings.TrimPrefix(a, "--exclude="))
		case a == "--exclude":
			if i+1 >= len(args) {
				return p, nil, errors.New("--exclude needs a value")
			}
			i++
			p.exclude = append(p.exclude, args[i])
		case a == "--no-recursion":
			p.noRecurse = true
		case strings.HasPrefix(a, "--"):
			return p, nil, fmt.Errorf("unknown option %q", a)
		case strings.HasPrefix(a, "-") && len(a) > 1:
			// The classic form: one token, letters in any order, -f and -C
			// carry their value.
			letters := a[1:]
			for j := 0; j < len(letters); j++ {
				switch letters[j] {
				case 'c':
					p.create = true
				case 'x':
					p.extract = true
				case 't':
					p.list = true
				case 'z':
					p.gzip = true
				case 'v':
					p.verbose = true
				case 'p':
					// keep permissions; the extractor does that already
				case 'j', 'J':
					return p, nil, fmt.Errorf("-%c is not supported: this tar reads and writes gzip only", letters[j])
				case 'f', 'C':
					value := letters[j+1:]
					if value == "" {
						if i+1 >= len(args) {
							return p, nil, fmt.Errorf("%s needs a value", "-"+string(letters[j]))
						}
						i++
						value = args[i]
					}
					if letters[j] == 'f' {
						p.file = value
					} else {
						p.dir = value
					}
					j = len(letters)
				default:
					return p, nil, fmt.Errorf("unknown option %q", "-"+string(letters[j]))
				}
			}
		default:
			rest = append(rest, a)
		}
	}
	return p, rest, nil
}

// reader opens the archive and peels gzip off when the data is gzipped, so -z
// is not needed for reading.
func (p params) reader(stdin io.Reader) (io.Reader, func() error, error) {
	var src io.Reader
	closeFn := func() error { return nil }
	if p.file == "-" || p.file == "" {
		src = stdin
	} else {
		f, err := os.Open(p.file)
		if err != nil {
			return nil, nil, err
		}
		src = f
		closeFn = f.Close
	}
	br := bufio.NewReader(src)
	if magic, err := br.Peek(2); err == nil && magic[0] == 0x1f && magic[1] == 0x8b {
		gz, err := gzip.NewReader(br)
		if err != nil {
			closeFn()
			return nil, nil, err
		}
		return gz, func() error { gz.Close(); return closeFn() }, nil
	}
	return br, closeFn, nil
}

// archiveWriter is a gzip stream on top of a file, closed in the right order.
type archiveWriter struct {
	gz *gzip.Writer
	f  *os.File
}

func (a *archiveWriter) Write(b []byte) (int, error) { return a.gz.Write(b) }

// writer opens the archive for writing, wrapped in gzip for -z.
func (p params) writer(stdout io.Writer) (io.Writer, error) {
	if p.file == "-" || p.file == "" {
		if p.gzip {
			return gzip.NewWriter(stdout), nil
		}
		return stdout, nil
	}
	f, err := os.Create(p.file)
	if err != nil {
		return nil, err
	}
	if p.gzip {
		return &archiveWriter{gz: gzip.NewWriter(f), f: f}, nil
	}
	return f, nil
}

// closeArchive closes a gzip wrapper first, the file after it.
func closeArchive(w io.Writer) {
	switch v := w.(type) {
	case *archiveWriter:
		v.gz.Close()
		v.f.Close()
	case *gzip.Writer:
		v.Close()
	case *os.File:
		v.Close()
	}
}

// run does what the switches asked for.
func (p params) run(stdin io.Reader, stdout, stderr io.Writer) int {
	modes := 0
	for _, on := range []bool{p.create, p.extract, p.list} {
		if on {
			modes++
		}
	}
	if modes > 1 {
		fmt.Fprintln(stderr, "tar: you may not specify more than one of -c, -x, -t")
		return 2
	}
	if modes == 0 {
		fmt.Fprintln(stderr, "tar: "+errMissingMandatoryFlag.Error())
		return 2
	}
	if p.file == "" {
		fmt.Fprintln(stderr, "tar: "+errEmptyFile.Error())
		return 2
	}
	opts := &tarutil.Opts{
		NoRecursion:     p.noRecurse,
		ChangeDirectory: p.dir,
		StripComponents: p.strip,
		Exclude:         p.exclude,
	}
	if p.verbose {
		opts.Filters = append(opts.Filters, tarutil.VerboseFilterTo(stdout))
	}

	switch {
	case p.create:
		if len(p.files) == 0 {
			fmt.Fprintln(stderr, "tar: Cowardly refusing to create an empty archive")
			return 2
		}
		out, err := p.writer(stdout)
		if err != nil {
			fmt.Fprintf(stderr, "tar: %v\n", err)
			return 2
		}
		if err := tarutil.CreateTar(out, p.files, opts); err != nil {
			fmt.Fprintf(stderr, "tar: %v\n", err)
			return 2
		}
		closeArchive(out)
	case p.list, p.extract:
		in, closeFn, err := p.reader(stdin)
		if err != nil {
			fmt.Fprintf(stderr, "tar: %v\n", err)
			return 2
		}
		defer closeFn()
		if p.list {
			if err := listArchive(in, stdout, p); err != nil {
				fmt.Fprintf(stderr, "tar: %v\n", err)
				return 2
			}
			return 0
		}
		dir := "."
		if len(p.files) > 0 {
			dir = p.files[0]
		}
		if err := tarutil.ExtractDir(in, dir, opts); err != nil {
			fmt.Fprintf(stderr, "tar: %v\n", err)
			return 2
		}
	}
	return 0
}

// listArchive prints the names, and with -v the header line GNU prints.
func listArchive(in io.Reader, out io.Writer, p params) error {
	tr := tar.NewReader(in)
	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			return nil
		}
		if err != nil {
			return err
		}
		if p.verbose {
			fmt.Fprintf(out, "%s %d/%d %7d %s %s\n", modeString(hdr.Mode), hdr.Uid, hdr.Gid, hdr.Size,
				hdr.ModTime.Format("2006-01-02 15:04"), hdr.Name)
			continue
		}
		fmt.Fprintln(out, hdr.Name)
	}
}

// modeString renders the mode the way GNU tar prints it in -tv.
func modeString(mode int64) string {
	const letters = "rwxrwxrwx"
	b := []byte("----------")
	switch {
	case mode&040000 != 0:
		b[0] = 'd'
	case mode&0120000 != 0:
		b[0] = 'l'
	}
	for i := 0; i < 9; i++ {
		if mode&(1<<uint(8-i)) != 0 {
			b[i+1] = letters[i]
		}
	}
	return string(b)
}

// Run implements tar for the acmesh dispatcher.
func Run(args []string, stdin io.Reader, stdout, stderr io.Writer) int {
	p, rest, err := parse(args)
	if err != nil {
		fmt.Fprintf(stderr, "tar: %v\n", err)
		return 2
	}
	p.files = rest
	return p.run(stdin, stdout, stderr)
}
