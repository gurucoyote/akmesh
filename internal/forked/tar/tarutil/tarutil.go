// Copyright 2019-2021 the u-root Authors. All rights reserved
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Package tarutil carries the u-root tar helpers, forked with the tar command.
// Local changes: ListArchive writes to the caller's writer, VerboseFilterTo
// takes one, and Opts knows StripComponents and Exclude.
package tarutil

import (
	"archive/tar"
	"bytes"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"strings"

	"akmesh/internal/forked/tar/upath"
)

// Opts contains options for creating and extracting tar files.
type Opts struct {
	// Filters are applied to each file while creating or extracting a tar
	// archive. The filter can modify the tar.Header struct. If the filter
	// returns false, the file is omitted.
	Filters []Filter

	// By default, when creating a tar archive, all directories are walked
	// to include all sub-directories. Set to true to prevent this
	// behavior.
	NoRecursion bool

	// Change to this directory before any operations. This is equivalent
	// to "tar -C DIR".
	ChangeDirectory string

	// StripComponents drops that many leading path elements while
	// extracting, like "tar --strip-components=N".
	StripComponents int

	// Exclude holds patterns; a path whose name or base name matches one is
	// not put into the archive, like "tar --exclude=PATTERN".
	Exclude []string

	// Self identifies the archive being written. A tar file inside the tree it
	// packs would otherwise be read while it grows, which produces an archive
	// that contains a truncated copy of itself (acmesh-vc3). GNU tar leaves it
	// out and warns.
	Self os.FileInfo

	// OnSelf is called with the path of the skipped archive, so the caller can
	// print the warning.
	OnSelf func(name string)

	// OnAbs is called once when a leading "/" has to be dropped from member
	// names, so the caller can print GNU's warning.
	OnAbs func()

	// OnSkip is called for a file whose header cannot be written (a Windows
	// special file, for instance); the archive continues without it.
	OnSkip func(name string, err error)
}

// excluded reports whether the path matches one of the exclude patterns.
func excluded(path string, patterns []string) bool {
	for _, pattern := range patterns {
		if ok, _ := filepath.Match(pattern, path); ok {
			return true
		}
		if ok, _ := filepath.Match(pattern, filepath.Base(path)); ok {
			return true
		}
	}
	return false
}

// stripPath removes n leading elements from a header name. It returns an empty
// string when nothing is left, and the caller skips that file.
func stripPath(name string, n int) string {
	parts := strings.Split(filepath.ToSlash(name), "/")
	if len(parts) <= n {
		return ""
	}
	return filepath.Join(parts[n:]...)
}

// passesFilters returns true if the given file passes all filters, false otherwise.
func passesFilters(hdr *tar.Header, filters []Filter) bool {
	for _, filter := range filters {
		if !filter(hdr) {
			return false
		}
	}
	return true
}

// applyToArchive applies function f to all files in the given archive
func applyToArchive(tarFile io.Reader, f func(tr *tar.Reader, hdr *tar.Header) error) error {
	tr := tar.NewReader(tarFile)
	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return err
		}
		if err := f(tr, hdr); err != nil {
			return err
		}
	}
	return nil
}

// ExtractToStdout writes the regular files of an archive to w instead of to
// disk, which is what "tar -xO" does (acmesh-vc3). An empty names list means
// every file; otherwise a file is taken when its stored name matches a name or
// starts with "name/", the way GNU tar matches.
func ExtractToStdout(r io.Reader, w io.Writer, opts *Opts, names []string) error {
	if opts == nil {
		opts = &Opts{}
	}
	wanted := func(stored string) bool {
		if len(names) == 0 {
			return true
		}
		clean := strings.TrimSuffix(stored, "/")
		for _, n := range names {
			// GNU drops the leading "/" from a member name, so a request for
			// the absolute path has to be matched without it.
			n = strings.TrimSuffix(filepath.ToSlash(n), "/")
			n = strings.TrimLeft(n, "/")
			if clean == n || strings.HasPrefix(clean, n+"/") {
				return true
			}
		}
		return false
	}
	return applyToArchive(r, func(tr *tar.Reader, hdr *tar.Header) error {
		if hdr.Typeflag != tar.TypeReg {
			return nil
		}
		if !passesFilters(hdr, opts.Filters) || !wanted(hdr.Name) {
			return nil
		}
		_, err := io.Copy(w, tr)
		return err
	})
}

// ListArchive lists the contents of the given tar archive.
func ListArchive(tarFile io.Reader, w io.Writer) error {
	return applyToArchive(tarFile, func(tr *tar.Reader, hdr *tar.Header) error {
		fmt.Fprintln(w, hdr.Name)
		return nil
	})
}

// ExtractDir extracts all the contents of the tar file to the given directory.
func ExtractDir(tarFile io.Reader, dir string, opts *Opts) error {
	if opts == nil {
		opts = &Opts{}
	}

	// Simulate a "cd" to another directory.
	if !filepath.IsAbs(dir) {
		dir = filepath.Join(opts.ChangeDirectory, dir)
	}

	fi, err := os.Stat(dir)
	if os.IsNotExist(err) {
		if err := os.Mkdir(dir, os.ModePerm); err != nil {
			return fmt.Errorf("could not create directory %s: %w", dir, err)
		}
	} else if err != nil || !fi.IsDir() {
		return fmt.Errorf("could not stat directory %s: %w", dir, err)
	}

	return applyToArchive(tarFile, func(tr *tar.Reader, hdr *tar.Header) error {
		if !passesFilters(hdr, opts.Filters) {
			return nil
		}
		if opts.StripComponents > 0 {
			name := stripPath(hdr.Name, opts.StripComponents)
			if name == "" {
				return nil
			}
			hdr.Name = name
		}
		return createFileInRoot(hdr, tr, dir)
	})
}

// CreateTar creates a new tar file with all the contents of a directory.
func CreateTar(tarFile io.Writer, files []string, opts *Opts) error {
	if opts == nil {
		opts = &Opts{}
	}

	tw := tar.NewWriter(tarFile)
	for _, bFile := range files {
		// Simulate a "cd" to another directory. There are 3 parts to
		// the file path:
		// a) The path passed to ChangeDirectory
		// b) The path passed in files
		// c) The path in the current walk
		// I prefixed corresponding a/b/c onto the variable name as an
		// aid. For example abFile is the filepath of a+b.
		abFile := filepath.Join(opts.ChangeDirectory, bFile)
		if filepath.IsAbs(bFile) {
			// "cd" does nothing if the file is absolute.
			abFile = bFile
		}

		walk := filepath.Walk
		if opts.NoRecursion {
			// This "walk" function does not recurse.
			walk = func(root string, walkFn filepath.WalkFunc) error {
				fi, err := os.Lstat(root)
				return walkFn(root, fi, err)
			}
		}

		err := walk(abFile, func(abcPath string, info os.FileInfo, err error) error {
			if err != nil {
				return err
			}

			// The record should not contain the ChangeDirectory
			// path, so we need to derive bc from abc.
			bcPath, relErr := filepath.Rel(opts.ChangeDirectory, abcPath)
			if relErr != nil {
				// Rel cannot express a relative path to an absolute source (or
				// the other way round); the absolute name is the answer GNU
				// gives too, minus the leading slash.
				bcPath = abcPath
			}
			if filepath.IsAbs(bFile) {
				// "cd" does nothing if the file is absolute.
				bcPath = abcPath
			}
			if filepath.IsAbs(bcPath) || strings.HasPrefix(bcPath, "/") {
				// GNU drops the leading "/" and says so once. On Windows the
				// volume name ("C:") goes with it, because an archive entry
				// cannot carry a drive letter.
				if opts.OnAbs != nil {
					opts.OnAbs()
				}
				bcPath = strings.TrimLeft(filepath.ToSlash(bcPath), "/")
				if vol := filepath.VolumeName(bcPath); vol != "" {
					bcPath = strings.TrimLeft(strings.TrimPrefix(bcPath, vol), "/")
				}
			}

			// GNU keeps the spelling of the argument: "tar -cf x.tar ." stores
			// "./a.txt", and "tar -cf x.tar dir" stores "dir/a.txt".
			if bFile == "." || strings.HasPrefix(filepath.ToSlash(bFile), "./") {
				if bcPath == "." {
					bcPath = "./"
				} else if !strings.HasPrefix(bcPath, "./") {
					bcPath = "./" + bcPath
				}
			}

			if opts.Self != nil && os.SameFile(opts.Self, info) {
				// The archive must not be packed into itself.
				if opts.OnSelf != nil {
					opts.OnSelf(bcPath)
				}
				return nil
			}

			var symlink string
			if info.Mode()&os.ModeSymlink == os.ModeSymlink {
				if symlink, err = os.Readlink(abcPath); err != nil {
					return err
				}
			}
			hdr, err := tar.FileInfoHeader(info, symlink)
			if err != nil {
				return err
			}
			// A tar archive stores "/" separators whatever the host does;
			// "d\a.txt" from a Windows build would be unreadable by GNU tar.
			hdr.Name = filepath.ToSlash(bcPath)
			if hdr.Typeflag == tar.TypeDir && !strings.HasSuffix(hdr.Name, "/") {
				// GNU stores a directory as "dir/", and "." as "./".
				hdr.Name += "/"
			}
			if excluded(bcPath, opts.Exclude) {
				return nil
			}
			if !passesFilters(hdr, opts.Filters) {
				return nil
			}
			switch hdr.Typeflag {
			case tar.TypeLink, tar.TypeSymlink, tar.TypeChar, tar.TypeBlock, tar.TypeDir, tar.TypeFifo:
				if err := tw.WriteHeader(hdr); err != nil {
					if opts.OnSkip != nil {
						opts.OnSkip(bcPath, err)
						return nil
					}
					return err
				}
			default:
				f, err := os.Open(abcPath)
				if err != nil {
					return err
				}
				defer f.Close()

				var r io.Reader = f
				if hdr.Size == 0 {
					// Some files don't report their size correctly
					// (ex: procfs), so we use an intermediary
					// buffer to determine size.
					b := &bytes.Buffer{}
					if _, err := io.Copy(b, f); err != nil {
						return err
					}
					hdr.Size = int64(b.Len())
					r = b
				}

				if err := tw.WriteHeader(hdr); err != nil {
					return err
				}
				if _, err := io.Copy(tw, r); err != nil {
					return err
				}
			}
			return nil
		})
		if err != nil {
			return err
		}
	}
	return tw.Close()
}

func createFileInRoot(hdr *tar.Header, r io.Reader, rootDir string) error {
	fi := hdr.FileInfo()
	path, err := upath.SafeFilepathJoin(rootDir, hdr.Name)
	if err != nil {
		// The behavior is to skip files which are unsafe due to
		// zipslip, but continue extracting everything else.
		log.Printf("Warning: Skipping file %q due to: %v", hdr.Name, err)
		return nil
	}

	switch fi.Mode() & os.ModeType {
	case os.ModeSymlink:
		// TODO: support symlinks
		return fmt.Errorf("symlinks not yet supported: %q", path)

	case os.FileMode(0):
		f, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, fi.Mode().Perm())
		if err != nil {
			return err
		}
		if _, err := io.Copy(f, r); err != nil {
			f.Close()
			return err
		}
		if err := f.Close(); err != nil {
			return err
		}

	case os.ModeDir:
		if err := os.MkdirAll(path, fi.Mode()&os.ModePerm); err != nil {
			return err
		}

	case os.ModeDevice:
		// TODO: support block device
		return fmt.Errorf("block device not yet supported: %q", path)

	case os.ModeCharDevice:
		// TODO: support char device
		return fmt.Errorf("char device not yet supported: %q", path)

	default:
		return fmt.Errorf("%q: Unknown type %#o", path, fi.Mode()&os.ModeType)
	}

	if err := os.Chmod(path, fi.Mode()&os.ModePerm); err != nil {
		return fmt.Errorf("error setting mode %#o on %q: %w",
			fi.Mode()&os.ModePerm, path, err)
	}
	// TODO: also set ownership, etc...
	return nil
}

// Filter is applied to each file while creating or extracting a tar archive.
// The filter can modify the tar.Header struct. If the filter returns false,
// the file is omitted.
type Filter func(hdr *tar.Header) bool

// VerboseFilter prints the name of every file.
// VerboseFilterTo prints each name to w while the archive is written or read.
func VerboseFilterTo(w io.Writer) Filter {
	return func(hdr *tar.Header) bool {
		fmt.Fprintln(w, hdr.Name)
		return true
	}
}

func VerboseFilter(hdr *tar.Header) bool {
	fmt.Println(hdr.Name)
	return true
}

// VerboseLogFilter logs the name of every file.
func VerboseLogFilter(hdr *tar.Header) bool {
	log.Println(hdr.Name)
	return true
}

// SafeFilter filters out all files which are not regular and not directories.
// It also sets appropriate permissions.
func SafeFilter(hdr *tar.Header) bool {
	if hdr.Typeflag == tar.TypeDir {
		hdr.Mode = 0o770
		return true
	}
	if hdr.Typeflag == tar.TypeReg {
		hdr.Mode = 0o660
		return true
	}
	return false
}
