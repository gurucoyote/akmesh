// Package apkscript runs the install scripts of an apk package with akmesh's own
// shell interpreter instead of handing them to a host shell.
//
// Two properties matter:
//
//   - No root needed. Every path the script opens is mapped into the target
//     root, so "/etc/foo" from the script becomes "<root>/etc/foo" on disk. The
//     script cannot touch the host by writing absolute paths.
//   - Root only where it is unavoidable. A script that calls an external binary
//     (ldconfig, update-ca-certificates) needs that binary and its view of the
//     filesystem; that still runs through chroot, which needs root. Without
//     root such a call fails with a clear message instead of silently running
//     against the host.
package apkscript

import (
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"mvdan.cc/sh/v3/expand"
	"mvdan.cc/sh/v3/interp"
	"mvdan.cc/sh/v3/syntax"
)

// Script is one install script as it travels in a package.
type Script struct {
	Kind    string // ".post-install", ".pre-upgrade", ...
	Body    []byte
	Package string
	Version string
}

// Run executes one script against the target root.
func Run(s Script, root string, stdout, stderr io.Writer) error {
	file, err := syntax.NewParser().Parse(strings.NewReader(string(s.Body)), s.Kind)
	if err != nil {
		return fmt.Errorf("%s: %v", s.Kind, err)
	}
	if root == "" {
		root = "/"
	}
	root = filepath.Clean(root)
	// A script run in the target expects the usual empty layout (busybox's
	// post-install links into /usr/bin, many scripts write /tmp). Create what is
	// missing; a device node only as root.
	PrepareRoot(root)
	// Builtins such as mkdir, cp and ln reach the kernel directly, so the open
	// handler alone cannot keep them inside the root. Absolute path literals are
	// rewritten to sit under the root; that is the "root prefix" for scripts.
	if root != "/" {
		PrefixPaths(file, root)
	}

	r, err := interp.New(
		interp.StdIO(strings.NewReader(""), stdout, stderr),
		interp.Dir(root),
		interp.Env(expand.ListEnviron(
			"PATH=/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin",
			"AKMESH_ROOT="+root,
			"PKG_NAME="+s.Package,
			"PKG_VERSION="+s.Version,
		)),
		interp.OpenHandler(rooted(root)),
		interp.ExecHandler(chrootExec(root, stderr)),
		// apk passes the package version as $1 to its scripts.
		interp.Params(s.Version),
	)
	if err != nil {
		return err
	}
	if err := r.Run(context.Background(), file); err != nil {
		return fmt.Errorf("%s: %v", s.Kind, err)
	}
	return nil
}

// rooted maps a path from the script into the target root and refuses anything
// that would climb out of it.
func rooted(root string) interp.OpenHandlerFunc {
	return func(ctx context.Context, path string, flag int, perm os.FileMode) (io.ReadWriteCloser, error) {
		// A target tree rarely has device nodes yet (they need root and mknod),
		// but package scripts redirect to /dev/null all the time. Those names
		// point at the host's device instead of failing.
		if root != "/" && strings.HasPrefix(path, root+"/") {
			path = strings.TrimPrefix(path, root)
		}
		switch path {
		case "/dev/null":
			return os.OpenFile(os.DevNull, flag, perm)
		case "/dev/zero":
			return os.OpenFile("/dev/zero", flag, perm)
		}
		real, err := resolve(root, interp.HandlerCtx(ctx).Dir, path)
		if err != nil {
			return nil, err
		}
		return os.OpenFile(real, flag, perm)
	}
}

// PrepareRoot creates the standard directory skeleton a chroot needs.
func PrepareRoot(root string) {
	if root == "" || root == "/" {
		return
	}
	for _, d := range []string{"bin", "sbin", "usr/bin", "usr/sbin", "usr/lib", "etc", "var/tmp", "tmp"} {
		os.MkdirAll(filepath.Join(root, d), 0o755)
	}
	os.Chmod(filepath.Join(root, "tmp"), 0o1777)
	os.MkdirAll(filepath.Join(root, "dev"), 0o755)
	null := filepath.Join(root, "dev/null")
	if _, err := os.Stat(null); err != nil && os.Geteuid() == 0 {
		if f, err := os.OpenFile(null, os.O_CREATE, 0o666); err == nil {
			f.Close()
		}
		if err := mknodNull(null); err != nil {
			os.Remove(null)
		}
	}
}

// PrefixPaths rewrites absolute path literals in the parsed script to live under
// root: "/etc/foo" becomes "<root>/etc/foo". Quoted strings, variables and
// command substitution keep their text; only literal words are touched.
func PrefixPaths(node syntax.Node, root string) {
	syntax.Walk(node, func(n syntax.Node) bool {
		lit, ok := n.(*syntax.Lit)
		if !ok || !strings.HasPrefix(lit.Value, "/") || strings.HasPrefix(lit.Value, root+"/") {
			return true
		}
		if lit.Value == "/" {
			lit.Value = root
			return true
		}
		lit.Value = root + lit.Value
		return true
	})
}

// resolve turns a script path into a host path. A relative path is taken
// against the shell's current directory (which starts at the root and moves
// with cd), an absolute one against the root itself.
func resolve(root, cwd, path string) (string, error) {
	if !filepath.IsAbs(path) {
		if cwd == "" {
			cwd = root
		}
		full := filepath.Clean(filepath.Join(cwd, path))
		if !contains(root, full) {
			return "", fmt.Errorf("apkscript: path %q leaves the target root", path)
		}
		return full, nil
	}
	return Inside(root, path)
}

// contains reports whether full lies inside root.
func contains(root, full string) bool {
	if root == "/" {
		return true
	}
	return full == root || strings.HasPrefix(full, root+string(filepath.Separator))
}

// Inside resolves a script path against the target root. An absolute path is
// taken relative to the root; a relative one is taken relative to the current
// directory, which Run keeps at the root. Paths that leave the root are an error.
func Inside(root, path string) (string, error) {
	if root != "/" && (path == root || strings.HasPrefix(path, root+string(filepath.Separator))) {
		return filepath.Clean(path), nil
	}
	clean := filepath.Clean("/" + path)
	full := filepath.Join(root, clean)
	rel, err := filepath.Rel(root, full)
	if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return "", fmt.Errorf("apkscript: path %q leaves the target root", path)
	}
	return full, nil
}

// chrootExec runs external commands inside the target root. That needs root; as
// a normal user the call is refused with a message that names the command, so a
// script fails loudly instead of touching the host.
func chrootExec(root string, stderr io.Writer) interp.ExecHandlerFunc {
	return func(ctx context.Context, args []string) error {
		hc := interp.HandlerCtx(ctx)
		if root == "" || root == "/" {
			cmd := exec.Command(args[0], args[1:]...)
			cmd.Stdin, cmd.Stdout, cmd.Stderr = hc.Stdin, hc.Stdout, hc.Stderr
			if err := cmd.Run(); err != nil {
				if ee, ok := err.(*exec.ExitError); ok {
					return interp.NewExitStatus(uint8(ee.ExitCode()))
				}
				return err
			}
			return nil
		}
		if os.Geteuid() != 0 {
			fmt.Fprintf(stderr, "apkscript: %s is not a shell builtin, running it needs root (chroot into %s); use --no-scripts to install the files only\n", args[0], root)
			return interp.NewExitStatus(127)
		}
		// The script's absolute paths carry the root prefix (see PrefixPaths);
		// inside the chroot the path must be the bare one again.
		bin := args[0]
		if strings.HasPrefix(bin, root+"/") {
			bin = strings.TrimPrefix(bin, root)
		}
		if !strings.HasPrefix(bin, "/") {
			// PATH lookup inside the target, as a shell in that tree would do.
			found := ""
			for _, dir := range []string{"/usr/local/sbin", "/usr/local/bin", "/usr/sbin", "/usr/bin", "/sbin", "/bin"} {
				if st, err := os.Stat(filepath.Join(root, dir, bin)); err == nil && !st.IsDir() {
					found = dir + "/" + bin
					break
				}
			}
			if found == "" {
				fmt.Fprintf(stderr, "apkscript: %s is not in the target root's PATH, so the script cannot run it; use --no-scripts to install the files only\n", bin)
				return interp.NewExitStatus(127)
			}
			bin = found
		}
		full, err := Inside(root, bin)
		if err != nil {
			fmt.Fprintf(stderr, "apkscript: %v\n", err)
			return interp.NewExitStatus(127)
		}
		if _, err := os.Stat(full); err != nil {
			fmt.Fprintf(stderr, "apkscript: %s is not part of the target root, so the script cannot run it; use --no-scripts to install the files only\n", args[0])
			return interp.NewExitStatus(127)
		}
		cmd := exec.Command("chroot", append([]string{root, bin}, args[1:]...)...)
		cmd.Stdin = hc.Stdin
		cmd.Stdout = hc.Stdout
		cmd.Stderr = hc.Stderr
		if err := cmd.Run(); err != nil {
			if ee, ok := err.(*exec.ExitError); ok {
				return interp.NewExitStatus(uint8(ee.ExitCode()))
			}
			return err
		}
		return nil
	}
}
