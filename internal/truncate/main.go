// Package truncate implements truncate, growing or shrinking a file to a size.

package truncate

import (
	"github.com/gurucoyote/akmesh/internal/shared"

	"fmt"
	"os"
	"strconv"
	"strings"
)

func Run(args []string, stdin *os.File, stdout, stderr *os.File) int {
	size := ""
	reference := ""
	noCreate := false
	var files []string
	for i := 0; i < len(args); i++ {
		switch {
		case args[i] == "-r" || args[i] == "--reference":
			if i+1 >= len(args) {
				fmt.Fprintln(stderr, "truncate: option '-r' needs a file")
				return 2
			}
			i++
			reference = args[i]
			continue
		case strings.HasPrefix(args[i], "--reference="):
			reference = strings.TrimPrefix(args[i], "--reference=")
			continue
		case strings.HasPrefix(args[i], "-r") && len(args[i]) > 2:
			reference = args[i][2:]
			continue
		}
		switch args[i] {
		case "-s", "--size":
			if i+1 >= len(args) {
				return 2
			}
			i++
			size = args[i]
		case "-c", "--no-create":
			noCreate = true
		default:
			if strings.HasPrefix(args[i], "-") && args[i] != "-" {
				fmt.Fprintf(stderr, "truncate: unknown option %q\n", args[i])
				return 2
			}
			files = append(files, args[i])
		}
	}
	if reference != "" {
		// -r takes the size from another file instead of -s.
		info, err := os.Stat(reference)
		if err != nil {
			fmt.Fprintf(stderr, "truncate: %v\n", err)
			return 1
		}
		size = strconv.FormatInt(info.Size(), 10)
	}
	if size == "" || len(files) == 0 {
		fmt.Fprintln(stderr, "usage: truncate -s SIZE FILE...")
		return 2
	}
	for _, name := range files {
		if err := truncateTo(name, size, noCreate); err != nil {
			fmt.Fprintf(stderr, "truncate: %v\n", err)
			return 1
		}
	}
	return 0
}

func truncateTo(name, size string, noCreate bool) error {
	info, err := os.Stat(name)
	if err != nil {
		if noCreate {
			// GNU: -c says "do not create", and a missing file is then not an
			// error either - it exits 0 and stays quiet.
			return nil
		}
		// GNU truncate creates the file unless -c was given.
		f, cerr := os.OpenFile(name, os.O_CREATE|os.O_WRONLY, 0o644)
		if cerr != nil {
			return err
		}
		f.Close()
		info, err = os.Stat(name)
		if err != nil {
			return err
		}
	}
	cur := info.Size()
	target, err := shared.ParseSize(size, cur)
	if err != nil {
		return err
	}
	if target < 0 {
		return fmt.Errorf("%s: negative size", name)
	}
	if target > cur {
		f, err := os.OpenFile(name, os.O_WRONLY, 0o644)
		if err != nil {
			return err
		}
		defer f.Close()
		if err := f.Truncate(target); err != nil {
			return err
		}
		return f.Sync()
	}
	return os.Truncate(name, target)
}
