package apkcmd

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"akmesh/internal/apkcore"
)

// ownBinaryTarget is where this binary sits inside root, or "" when there is
// nothing to protect. A package that carries akmesh installs it as
// /bin/<name>, so that is the path an upgrade of the core package replaces.
//
// The point of knowing it: replacing a broken binary takes away the tool that
// would repair it, so this one file is written aside, checked, and only then
// moved into place (acmesh-ieh3).
func ownBinaryTarget(root string) string {
	exe, err := os.Executable()
	if err != nil || exe == "" {
		return ""
	}
	base := filepath.Base(exe)
	switch root {
	case "", "/":
		return filepath.Join("/", "bin", base)
	}
	return filepath.Join(root, "bin", base)
}

// checkNewBinary runs the freshly extracted binary the way a user would: the
// dispatcher has to find a command and the command has to exit 0.
//
// "true" is the cheapest command that proves the binary still dispatches and
// still applies the policy; a truncated or wrongly linked file fails here
// instead of at the next login.
func checkNewBinary(path string) error {
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, path, "true")
	var errBuf bytes.Buffer
	cmd.Stdout = io.Discard
	cmd.Stderr = &errBuf
	if err := cmd.Run(); err != nil {
		msg := strings.TrimSpace(errBuf.String())
		if msg == "" {
			return err
		}
		return fmt.Errorf("%v: %s", err, msg)
	}
	return nil
}

// installCheckedBinary moves the checked file over the old one, or leaves
// everything as it was.
//
// The order is the whole point: until the check passes, the old binary is
// untouched, so a broken package cannot leave the system without a working
// tool. Running processes keep the old inode anyway; the replace matters for
// the next start.
func installCheckedBinary(cmdName, deferred, target string, stdout, stderr io.Writer) error {
	if err := checkNewBinary(deferred); err != nil {
		fmt.Fprintf(stderr, "%s: the new binary does not run (%v); %s stays as it was, the untested file is at %s\n",
			cmdName, err, target, deferred)
		return err
	}
	if err := os.Rename(deferred, target); err != nil {
		fmt.Fprintf(stderr, "%s: the checked binary could not be moved into place: %v (it stays at %s)\n",
			cmdName, err, deferred)
		return err
	}
	fmt.Fprintf(stdout, "  Replaced %s with a checked copy\n", target)
	return nil
}

// deferOwnBinary fills the extract options so that the running binary is
// written aside. The caller passes the variable the extraction fills in, so
// that it can see afterwards whether the binary was deferred at all.
func deferOwnBinary(root string, opts *apkcore.ExtractOptions, deferred *string) (target string, ok bool) {
	target = ownBinaryTarget(root)
	if target == "" {
		return "", false
	}
	if _, err := os.Stat(target); err != nil {
		// The root does not carry our binary yet: nothing to protect.
		return "", false
	}
	opts.DeferTarget = target
	opts.Deferred = deferred
	return target, true
}
