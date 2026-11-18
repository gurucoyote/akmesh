// Copyright 2023 the u-root Authors. All rights reserved
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// touch changes file access and modification times.
//
// Synopsis:
//
//	touch [-amc] [-d datetime] file...
//
// Description:
//
//	If a file does not exist, it will be created unless -c is specified.
//
// Options:
//
//	-a: change only the access time
//	-m: change only the modification time
//	-c: do not create any file if it does not exist
//	-d: use specified time instead of current time (RFC3339 format)
//
// Forked from u-root 869f00f6b224ce02b1db51806159d6cd9c246f76, cmds/core/touch/touch.go.
// The rest of this file is the upstream source and keeps its copyright.
//
// Local changes, all of them for the way agents call touch:
//   - package touch and Run(args, stdin, stdout, stderr) instead of main();
//   - no flag package and no unixflag: an own parser, so -t and -r work and -d
//     accepts the date forms people write ("2020-01-02 03:04:05" and
//     "202001020304", not only RFC3339);
//   - new: -t STAMP (GNU's [[CC]YY]MMDDhhmm[.ss]) and -r FILE (take the times
//     from another file);
//   - a file that cannot be touched is named on stderr, the rest still run, and
//     the exit status is 1.
package touch

import (
	"errors"
	"fmt"
	"io"
	"os"
	"strings"
	"time"
)

type params struct {
	time         time.Time
	access       bool
	modification bool
	create       bool
}

type cmd struct {
	params
	args []string
}

// parseStamp reads GNU's -t form: [[CC]YY]MMDDhhmm[.ss], where a two-digit year
// below 69 means 2069 and above means 19xx.
func parseStamp(stamp string) (time.Time, error) {
	seconds := "00"
	if i := strings.IndexByte(stamp, '.'); i >= 0 {
		seconds = stamp[i+1:]
		stamp = stamp[:i]
	}
	switch len(stamp) {
	case 8:
		stamp = time.Now().Format("2006") + stamp
	case 10, 12:
	default:
		return time.Time{}, fmt.Errorf("invalid date format %q", stamp)
	}
	full := stamp
	if len(stamp) == 10 {
		// A two-digit year: 69 and up means 19xx, below means 20xx.
		year := stamp[:2]
		var yy int
		fmt.Sscanf(year, "%d", &yy)
		if yy >= 69 {
			full = "19" + stamp
		} else {
			full = "20" + stamp
		}
	}
	if len(full) == 12 {
		// GNU reads the stamp in local time, so a container in UTC and a host
		// in CET must not disagree.
		if _, err := time.Parse("200601021504", full); err != nil {
			return time.Time{}, err
		}
		return time.ParseInLocation("20060102150405", full+seconds, time.Local)
	}
	return time.ParseInLocation("20060102", full, time.Local)
}

// parseDate reads what -d gets: RFC3339, "YYYY-MM-DD hh:mm[:ss]", a plain date,
// "hh:mm" for today, and the word "now".
func parseDate(value string) (time.Time, error) {
	value = strings.TrimSpace(value)
	if value == "now" {
		return time.Now(), nil
	}
	layouts := []string{
		time.RFC3339,
		"2006-01-02T15:04:05",
		"2006-01-02T15:04",
		"2006-01-02 15:04:05",
		"2006-01-02 15:04",
		"2006-01-02",
		"200601021504.05",
		"200601021504",
	}
	for _, layout := range layouts {
		if t, err := time.ParseInLocation(layout, value, time.Local); err == nil {
			return t, nil
		}
	}
	if t, err := time.ParseInLocation("15:04", value, time.Local); err == nil {
		now := time.Now()
		return time.Date(now.Year(), now.Month(), now.Day(), t.Hour(), t.Minute(), 0, 0, time.Local), nil
	}
	if t, err := parseStamp(value); err == nil {
		return t, nil
	}
	return time.Time{}, fmt.Errorf("invalid date format %q", value)
}

// run sets the times on every file.
func (c *cmd) run() error {
	var errs error
	for _, arg := range c.args {
		_, existsErr := os.Stat(arg)
		notExist := os.IsNotExist(existsErr)
		if notExist {
			if c.create {
				continue
			}

			f, err := os.Create(arg)
			if err != nil {
				errs = errors.Join(errs, err)
				continue
			}
			f.Close()
		}

		accessTime := time.Time{}
		if c.access || notExist {
			accessTime = c.time
		}
		modificationTime := time.Time{}
		if c.modification || notExist {
			modificationTime = c.time
		}

		err := os.Chtimes(arg, accessTime, modificationTime)
		if err != nil {
			errs = errors.Join(errs, err)
		}
	}

	return errs
}

// Run implements touch for the acmesh dispatcher.
func Run(args []string, stdin io.Reader, stdout, stderr io.Writer) int {
	var (
		access, modification, noCreate bool
		dateValue, stampValue, refFile string
		files                          []string
	)
	for i := 0; i < len(args); i++ {
		a := args[i]
		value := func(name string) (string, bool) {
			if len(a) > len(name) {
				return a[len(name):], true
			}
			if i+1 >= len(args) {
				fmt.Fprintf(stderr, "touch: option %s needs a value\n", name)
				return "", false
			}
			i++
			return args[i], true
		}
		switch {
		case a == "--":
			files = append(files, args[i+1:]...)
			i = len(args)
		case strings.HasPrefix(a, "--"):
			switch a {
			case "--no-create":
				noCreate = true
			case "--date", "--time":
				v, ok := value(a)
				if !ok {
					return 1
				}
				if a == "--time" {
					stampValue = v
				} else {
					dateValue = v
				}
			case "--reference":
				v, ok := value("--reference")
				if !ok {
					return 1
				}
				refFile = v
			default:
				fmt.Fprintf(stderr, "touch: unknown option %q\n", a)
				return 1
			}
		case strings.HasPrefix(a, "-") && len(a) > 1:
			letters := a[1:]
			for j := 0; j < len(letters); j++ {
				switch letters[j] {
				case 'a':
					access = true
				case 'm':
					modification = true
				case 'c':
					noCreate = true
				case 'h':
					// no dereference; os.Chtimes follows, so nothing to do here
				case 't', 'd', 'r':
					v, ok := value("-" + string(letters[j]))
					if !ok {
						return 1
					}
					switch letters[j] {
					case 't':
						stampValue = v
					case 'd':
						dateValue = v
					case 'r':
						refFile = v
					}
					j = len(letters)
				default:
					fmt.Fprintf(stderr, "touch: unknown option %q\n", "-"+string(letters[j]))
					return 1
				}
			}
		default:
			files = append(files, a)
		}
	}
	if len(files) == 0 {
		fmt.Fprintln(stderr, "touch: missing file operand")
		return 1
	}

	when := time.Now()
	switch {
	case stampValue != "":
		t, err := parseStamp(stampValue)
		if err != nil {
			fmt.Fprintf(stderr, "touch: %v\n", err)
			return 1
		}
		when = t
	case dateValue != "":
		t, err := parseDate(dateValue)
		if err != nil {
			fmt.Fprintf(stderr, "touch: invalid date format '%s'\n", dateValue)
			return 1
		}
		when = t
	case refFile != "":
		info, err := os.Stat(refFile)
		if err != nil {
			fmt.Fprintf(stderr, "touch: %v\n", err)
			return 1
		}
		when = info.ModTime()
	}

	c := &cmd{params: params{
		time:         when,
		access:       access || (!access && !modification),
		modification: modification || (!access && !modification),
		create:       noCreate,
	}, args: files}
	if err := c.run(); err != nil {
		fmt.Fprintf(stderr, "touch: %v\n", err)
		return 1
	}
	return 0
}
