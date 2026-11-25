// Package clear implements the clear command: it clears the terminal.
//
// The terminal is cleared with ANSI sequences and not with terminfo, because
// every terminal Windows and Linux put in front of this shell understands them:
// Windows Terminal, xterm, the Windows console with VT enabled. A terminfo
// lookup would need a database the image does not carry, and would leave a
// person staring at an uncleared screen when it is missing. Without a terminal
// (a pipe) the sequences go to stdout and do nothing harmful - GNU clear is
// silent about that too. (25.11.2026)
package clear

import (
	"fmt"
	"io"
	"os"
)

const usageText = `usage: clear [-x]
clears the screen

  -x, --no-save-lines  do not clear the scrollback`

// Run clears the screen. -x leaves the scrollback alone, as GNU clear does.
func Run(args []string, stdin, stdout, stderr *os.File) int {
	keepScrollback := false
	for _, a := range args {
		switch a {
		case "-x", "--no-save-lines":
			keepScrollback = true
		case "-h", "--help":
			fmt.Fprintln(stdout, usageText)
			return 0
		case "":
			// Nothing to do; an empty argument is not an error in GNU clear either.
		default:
			fmt.Fprintf(stderr, "clear: unrecognised option %q\n", a)
			fmt.Fprintln(stderr, usageText)
			return 2
		}
	}
	seq := "\x1b[H\x1b[2J"
	if !keepScrollback {
		// The scrollback is what a person sees when they scroll up; GNU clear
		// removes it as well, so a cleared screen is really clear.
		seq += "\x1b[3J"
	}
	seq += "\x1b[H"
	if _, err := io.WriteString(stdout, seq); err != nil {
		fmt.Fprintf(stderr, "clear: %v\n", err)
		return 1
	}
	return 0
}
