package apkcmd

import (
	"io"
	"os"
	"os/exec"

	"github.com/gurucoyote/akmesh/internal/apkscript"
)

// ownTool lets a package script run a command we already carry, in our own
// process tree: the binary calls itself with the command name as its argument
// ("akmesh mkdir <path>"), which the dispatcher understands for our commands and
// for the u-root applets alike. The paths in the arguments were already mapped
// into the target root by apkscript.PrefixPaths, so nothing outside the tree is
// touched.
//
// Only tools that work on files and text are allowed. A daemon, a shell or a
// network command in a package script would do things we cannot reason about, so
// it stays refused and the script gets the honest message instead.
var ownToolAllowed = map[string]bool{
	// our table
	"basename": true, "cat": true, "chmod": true, "chown": true, "comm": true,
	"cp": true, "cut": true, "df": true, "diff": true, "dirname": true,
	"du": true, "file": true, "find": true, "grep": true, "head": true,
	"hexdump": true, "iconv": true, "id": true, "jq": true, "ln": true,
	"ls": true, "mkdir": true, "mv": true, "nl": true, "od": true,
	"patch": true, "rm": true, "rmdir": true, "sed": true, "seq": true,
	"sha1sum": true, "sha256sum": true, "sha512sum": true, "shuf": true,
	"sort": true, "split": true, "stat": true, "sum": true, "sync": true,
	"tac": true, "tail": true, "tar": true, "touch": true, "tr": true,
	"truncate": true, "uname": true, "uniq": true, "wc": true, "xargs": true,
	"awk": true, "rg": true, "hostname": true, "pwd": true, "echo": true,
	"true": true, "false": true, "env": true, "base64": true, "unzip": true,
}

// runOwnTool carries out one such call and reports whether the name was one of
// ours at all, so apkscript can fall back to its own messages.
func runOwnTool(name string, args []string, stdin io.Reader, stdout, stderr io.Writer) (int, bool) {
	if !ownToolAllowed[name] {
		return 0, false
	}
	self, err := os.Executable()
	if err != nil {
		return 1, true
	}
	cmd := exec.Command(self, append([]string{name}, args...)...)
	cmd.Stdin, cmd.Stdout, cmd.Stderr = stdin, stdout, stderr
	err = cmd.Run()
	if ee, ok := err.(*exec.ExitError); ok {
		return ee.ExitCode(), true
	}
	if err != nil {
		return 1, true
	}
	return 0, true
}

func init() {
	apkscript.OwnTool = runOwnTool
}
