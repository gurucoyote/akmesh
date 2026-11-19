// Package stat implements stat, the file and filesystem numbers an agent reads:
// mode, size, inode, times, and the statfs view with -f. Numbers the platform
// cannot provide are reported as zero rather than invented.
package stat

import (
	"fmt"
	"os"
	"strings"

	"akmesh/internal/winmode"
)

func Run(args []string, stdin *os.File, stdout, stderr *os.File) int {
	format := ""
	formatSet := false
	printf := false
	terse := false
	fileSystem := false
	deref := false
	var files []string
	for i := 0; i < len(args); i++ {
		a := args[i]
		switch {
		case a == "--":
			files = append(files, args[i+1:]...)
			i = len(args)
		case a == "-c" || a == "--format":
			if i+1 >= len(args) {
				fmt.Fprintln(stderr, "stat: option requires an argument -- 'c'")
				return 2
			}
			i++
			format, formatSet = args[i], true
		case strings.HasPrefix(a, "--format="):
			format, formatSet = strings.TrimPrefix(a, "--format="), true
		case a == "--printf":
			if i+1 >= len(args) {
				fmt.Fprintln(stderr, "stat: option '--printf' requires an argument")
				return 2
			}
			i++
			format, formatSet, printf = args[i], true, true
		case strings.HasPrefix(a, "--printf="):
			format, formatSet, printf = strings.TrimPrefix(a, "--printf="), true, true
		case a == "-t" || a == "--terse":
			terse = true
		case a == "-f" || a == "--file-system":
			fileSystem = true
		case a == "-L" || a == "--dereference":
			deref = true
		case strings.HasPrefix(a, "-c") && !strings.HasPrefix(a, "--") && len(a) > 2:
			format, formatSet = a[2:], true
		case strings.HasPrefix(a, "-") && !strings.HasPrefix(a, "--") && len(a) > 2:
			// Combined short flags such as -tL or -Lf.
			for _, c := range a[1:] {
				switch c {
				case 't':
					terse = true
				case 'f':
					fileSystem = true
				case 'L':
					deref = true
				default:
					fmt.Fprintf(stderr, "stat: unknown option %q\n", "-"+string(c))
					return 2
				}
			}
		default:
			if strings.HasPrefix(a, "-") && a != "-" {
				fmt.Fprintf(stderr, "stat: unknown option %q\n", a)
				return 2
			}
			files = append(files, a)
		}
	}
	if printf && formatSet {
		format = unescapePrintf(format)
	}
	if len(files) == 0 {
		fmt.Fprintln(stderr, "stat: missing operand")
		return 2
	}
	code := 0
	for _, name := range files {
		if fileSystem {
			fs, err := fileSystemStats(name)
			if err != nil {
				fmt.Fprintf(stderr, "stat: %v\n", err)
				code = 1
				continue
			}
			switch {
			case formatSet:
				out := expandFsFormat(format, fs)
				if !printf {
					out += "\n"
				}
				fmt.Fprint(stdout, out)
			case terse:
				fmt.Fprint(stdout, terseFsStat(name, fs))
			default:
				fmt.Fprint(stdout, defaultFsStat(name, fs))
			}
			continue
		}
		var info os.FileInfo
		var err error
		if deref {
			info, err = os.Stat(name)
		} else {
			info, err = os.Lstat(name)
		}
		if err != nil {
			fmt.Fprintf(stderr, "stat: %v\n", err)
			code = 1
			continue
		}
		// On Windows os.Stat reports 0666 for every file; a mode that chmod
		// recorded in the ACL is the truth there (acmesh-m6x).
		info = winmode.Apply(name, info)
		d := statFields{name: name, info: info, extra: statExtraOf(info)}
		switch {
		case formatSet:
			out := expandStatFormat(format, d)
			if !printf {
				out += "\n"
			}
			fmt.Fprint(stdout, out)
		case terse:
			fmt.Fprint(stdout, terseStat(d))
		default:
			fmt.Fprint(stdout, defaultStat(d))
		}
	}
	return code
}

func fileTypeOf(info os.FileInfo) string {
	switch {
	case info.IsDir():
		return "directory"
	case info.Mode()&os.ModeSymlink != 0:
		return "symbolic link"
	case info.Mode().IsRegular():
		return "regular file"
	default:
		return "special file"
	}
}

// runTruncate implements truncate -s SIZE FILE...
// SIZE accepts K, M, G suffixes and a leading + or - for relative sizes.
