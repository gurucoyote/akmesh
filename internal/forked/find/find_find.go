// Copyright 2015-2017 the u-root Authors. All rights reserved
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.
//
// Package find searches for files in a directory hierarchy recursively.
//
// find can filter out files by file names, paths, and modes.
//
// Forked from u-root 869f00f6b224ce02b1db51806159d6cd9c246f76, pkg/find/find.go.
// The rest of this file is the upstream source and keeps its copyright.
//
// Local changes:
//   - package find in this repository, no u-root module import;
//   - the matcher results are combined in one loop with a single push helper, so
//     a directory at maxdepth is reported before the walk stops descending;
//   - pkg/ls is gone together with -l: File.String prints mode, size and path;
//   - WithMaxDepth and WithMinDepth decide in the walk itself, so a directory
//     below maxdepth is never entered (GNU -maxdepth, -mindepth);
//   - a symlink is not followed, as with GNU find without -L (fs.WalkDir
//     already behaves that way; noted because it is part of the contract).
package find

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
)

var ErrInvalidRegexp = errors.New("invalid regular expression")

// File is a found file.
type File struct {
	// Name is the path as the walk produced it, relative to the root.
	Name string
	Err  error
}

// String implements a fmt.Stringer for File: mode, size and path on one line.
func (f *File) String() string {
	// If the walk already captured an error, show that.
	// This avoids an unneeded stat, which will also
	// take an error.
	if f.Err != nil {
		return fmt.Sprintf("%s: %v", f.Name, f.Err)
	}

	// If the stat fails, show that it failed.
	fi, err := os.Lstat(f.Name)
	if err != nil {
		return fmt.Sprintf("%s: %v", f.Name, err)
	}
	return fmt.Sprintf("%s %8d %s", fi.Mode().String(), fi.Size(), f.Name)
}

// A Matcher returns true if the string/fs.DirEntry match some criteria.
// It will return an error if it had an error of some type.
type Matcher func(string, fs.DirEntry) (bool, error)

// Finder controls show files are walked.
type Finder struct {
	root         string
	matchers     []Matcher
	files        chan *File
	debug        func(string, ...any)
	ignoreErrors bool
	// maxDepth limits how deep the walk descends (0 = no limit), minDepth how
	// deep a match has to be before it is reported (0 = the root counts too).
	maxDepth int
	minDepth int
}

func New() *Finder {
	return &Finder{
		debug: func(string, ...any) {},
		files: make(chan *File, 128),
	}
}

type Set func(*Finder)

// WithRoot sets a root path for the file finder. Only descendants of the root
// will be returned on the channel.
func WithRoot(rootPath string) Set {
	return func(f *Finder) {
		f.root = rootPath
	}
}

// WithoutError filters out files with errors from being sent on the channel.
func WithoutError() Set {
	return func(f *Finder) {
		f.ignoreErrors = true
	}
}

// WithMaxDepth stops the walk below the given depth, the root being 0.
func WithMaxDepth(depth int) Set {
	return func(f *Finder) {
		f.maxDepth = depth
	}
}

// WithMinDepth suppresses matches above the given depth, the root being 0.
func WithMinDepth(depth int) Set {
	return func(f *Finder) {
		f.minDepth = depth
	}
}

// Find finds files according to the settings and matchers given.
// It can be called with no opts, but requires a context.
// e.g. names := Find(context.Background())
// or
//
//	names := Find(ctx,
//	  WithRoot("/boot"),
//	  WithFilenameMatch("sda[0-9]"),
//	  WithDebugLog(log.Printf),
//	)
func Find(ctx context.Context, opt ...Set) <-chan *File {
	f := New()
	return RunFind(ctx, f, opt...)
}

// depthOf counts the path segments between root and name the way GNU counts:
// the root itself is 0, its children are 1.
func depthOf(root, name string) int {
	rel, err := filepath.Rel(root, name)
	if err != nil || rel == "." {
		return 0
	}
	return strings.Count(rel, string(filepath.Separator)) + 1
}

// RunFind runs a Finder
func RunFind(ctx context.Context, f *Finder, opt ...Set) <-chan *File {

	for _, o := range opt {
		if o != nil {
			o(f)
		}
	}

	go func(f *Finder) {
		_ = filepath.WalkDir(f.root, func(n string, de fs.DirEntry, err error) error {
			f.debug("walk to %v, de %v, err %v", f, de, err)
			if err != nil && f.ignoreErrors {
				// Don't send file on channel if user doesn't want them.
				return nil
			}

			depth := depthOf(f.root, n)
			// A directory at maxDepth is still reported but not entered: its
			// children would be one level deeper than GNU find shows.
			skipDepth := f.maxDepth > 0 && err == nil && de.IsDir() && depth >= f.maxDepth
			next := func() error {
				if skipDepth {
					return filepath.SkipDir
				}
				return nil
			}

			file := &File{
				Name: n,
				Err:  err,
			}

			if err != nil {
				return push(f, ctx, file)
			}
			if depth < f.minDepth {
				// Too shallow to be reported, but the walk goes on.
				return next()
			}
			for _, m := range f.matchers {
				ok, merr := m(n, de)
				if merr != nil {
					f.debug("%s: err on matching: %v", n, merr)
					return next()
				}
				if !ok {
					f.debug("%q:", n)
					return next()
				}
			}
			f.debug("Found: %s", n)
			if perr := push(f, ctx, file); perr != nil {
				return perr
			}
			return next()
		})
		f.debug("Done, close chan")
		close(f.files)
	}(f)

	f.debug("return from Find")
	return f.files
}

// push sends a found file to the result channel, so the walk blocks instead of
// losing entries when the reader is slow.
func push(f *Finder, ctx context.Context, file *File) error {
	select {
	case <-ctx.Done():
		f.debug("ctx says done")
		return fmt.Errorf("should never be returned to user: stop walking")
	case f.files <- file:
		f.debug("pushed file")
		return nil
	}
}
