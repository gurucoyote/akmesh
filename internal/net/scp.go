//go:build !nonet

package net

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"golang.org/x/crypto/ssh"
)

// runScp copies files over the same connection rules as ssh. It speaks the scp
// protocol, so the remote side only needs the scp that comes with OpenSSH.
func runScp(args []string, stdin *os.File, stdout, stderr *os.File) int {
	opts := sshOptions{Port: "22", ConfigFile: defaultSSHConfigPath(), BatchMode: true}
	recursive := false
	preserve := false
	quiet := false
	var paths []string
	overrides := map[string]string{}
	for i := 0; i < len(args); i++ {
		a := args[i]
		next := func(what string) (string, bool) {
			if i+1 >= len(args) {
				fmt.Fprintf(stderr, "scp: %s needs a value\n", what)
				return "", false
			}
			i++
			return args[i], true
		}
		switch {
		case a == "--help" || a == "-h":
			printScpHelp(stdout)
			return 0
		case a == "-p":
			// As in OpenSSH scp: -p preserves times, -P sets the port.
			preserve = true
		case a == "-P":
			v, ok := next("-P")
			if !ok {
				return 1
			}
			opts.Port = v
		case a == "-i":
			v, ok := next("-i")
			if !ok {
				return 1
			}
			opts.Keys = append(opts.Keys, v)
		case a == "-F":
			v, ok := next("-F")
			if !ok {
				return 1
			}
			opts.ConfigFile = v
		case a == "-o":
			v, ok := next("-o")
			if !ok {
				return 1
			}
			name, value, ok := strings.Cut(v, "=")
			if !ok {
				fmt.Fprintf(stderr, "scp: -o needs Key=Value, got %q\n", v)
				return 1
			}
			overrides[strings.ToLower(strings.TrimSpace(name))] = strings.TrimSpace(value)
		case a == "-r":
			recursive = true
		case a == "-rp" || a == "-pr":
			recursive = true
			preserve = true
		case a == "-q":
			quiet = true
		case a == "-v":
			opts.Verbose = true
		case a == "-C":
			// The channel compression of OpenSSH is not implemented in
			// x/crypto/ssh, and the scp protocol itself has none.
			fmt.Fprintln(stderr, "scp: -C (compression) is not supported, the ssh library cannot negotiate it")
			return 1
		case a == "-l":
			fmt.Fprintln(stderr, "scp: -l (bandwidth limit) is not supported")
			return 1
		case strings.HasPrefix(a, "-") && len(a) > 1:
			fmt.Fprintf(stderr, "scp: unsupported option %q\n", a)
			return 1
		default:
			paths = append(paths, a)
		}
	}
	_ = quiet
	if len(paths) < 2 {
		fmt.Fprintln(stderr, "usage: scp [options] SOURCE... DEST")
		return 1
	}
	dst := paths[len(paths)-1]
	srcs := paths[:len(paths)-1]

	dstHost, dstPath := splitRemote(dst)
	if dstHost == "" {
		// Local destination: every source must be remote.
		for _, src := range srcs {
			host, remotePath := splitRemote(src)
			if host == "" {
				fmt.Fprintf(stderr, "scp: %s: local to local copy makes no sense here\n", src)
				return 1
			}
			if err := applyHostOptions(&opts, host, overrides); err != nil {
				fmt.Fprintf(stderr, "scp: %v\n", err)
				return 1
			}
			if code := downloadOne(opts, remotePath, dstPath, recursive, preserve, stderr); code != 0 {
				return code
			}
		}
		return 0
	}
	for _, src := range srcs {
		if host, _ := splitRemote(src); host != "" {
			fmt.Fprintln(stderr, "scp: remote to remote is not supported, copy through this machine")
			return 1
		}
	}
	if err := applyHostOptions(&opts, dstHost, overrides); err != nil {
		fmt.Fprintf(stderr, "scp: %v\n", err)
		return 1
	}
	if opts.Verbose {
		fmt.Fprintf(stderr, "scp: %s@%s:%s\n", opts.User, opts.Host, opts.Port)
	}
	client, err := buildSSHClient(opts, stderr)
	if err != nil {
		fmt.Fprintf(stderr, "scp: %v\n", err)
		return 1
	}
	defer client.Close()
	for _, src := range srcs {
		if code := uploadOne(client, src, dstPath, recursive, preserve, stderr); code != 0 {
			return code
		}
	}
	return 0
}

// applyHostOptions resolves config and -o for one destination host.
func applyHostOptions(opts *sshOptions, remote string, overrides map[string]string) error {
	if opts.Port == "" {
		opts.Port = "22"
	}
	// applyDestination keeps a user in the destination and otherwise falls back
	// to the current user, exactly as ssh does.
	if err := applyDestination(opts, remote); err != nil {
		return err
	}
	if err := applyConfig(opts); err != nil {
		return err
	}
	applyOverrides(opts, overrides, os.Stderr)
	return nil
}

// splitRemote separates "user@host:path" from a local path. A colon inside the
// first component means remote, as in scp; a path such as ./a:b is local. The
// user stays attached, so the same parsing as ssh decides it.
func splitRemote(arg string) (remote, p string) {
	if idx := strings.Index(arg, ":"); idx > 0 {
		head := arg[:idx]
		if !strings.Contains(head, "/") && !strings.Contains(head, string(os.PathSeparator)) {
			return head, arg[idx+1:]
		}
	}
	return "", arg
}

// uploadOne sends a file, or a tree with -r, to the remote directory.
func uploadOne(client *ssh.Client, src, dstDir string, recursive, preserve bool, stderr *os.File) int {
	info, err := os.Stat(src)
	if err != nil {
		fmt.Fprintf(stderr, "scp: %v\n", err)
		return 1
	}
	if info.IsDir() && !recursive {
		fmt.Fprintf(stderr, "scp: %s is a directory, use -r\n", src)
		return 1
	}
	session, err := client.NewSession()
	if err != nil {
		fmt.Fprintf(stderr, "scp: %v\n", err)
		return 1
	}
	defer session.Close()
	writer, err := session.StdinPipe()
	if err != nil {
		fmt.Fprintf(stderr, "scp: %v\n", err)
		return 1
	}
	reader, err := session.StdoutPipe()
	if err != nil {
		fmt.Fprintf(stderr, "scp: %v\n", err)
		return 1
	}
	session.Stderr = stderr
	remote := "scp -t "
	if recursive {
		// The sink refuses directories unless it was told they may come.
		remote = "scp -t -r "
	}
	if err := session.Start(remote + shellQuote(dstDir)); err != nil {
		fmt.Fprintf(stderr, "scp: %v\n", err)
		return 1
	}
	ack := newScpAck(reader)
	if err := ack.read(); err != nil {
		fmt.Fprintf(stderr, "scp: remote refused: %v\n", err)
		return 1
	}
	if err := sendTree(writer, ack, src, info, recursive, preserve); err != nil {
		fmt.Fprintf(stderr, "scp: %v\n", err)
		return 1
	}
	writer.Close()
	if err := session.Wait(); err != nil {
		fmt.Fprintf(stderr, "scp: %v\n", err)
		return 1
	}
	return 0
}

// sendTree writes the C and D records for one path and its children.
func sendTree(w io.Writer, ack *scpAck, src string, info os.FileInfo, recursive, preserve bool) error {
	if preserve {
		// A T record announces the times of the entry that follows.
		atime := info.ModTime()
		if st, ok := statTimes(info); ok {
			atime = st.atime
		}
		if _, err := fmt.Fprintf(w, "T%d 0 %d 0\n", info.ModTime().Unix(), atime.Unix()); err != nil {
			return err
		}
		if err := ack.read(); err != nil {
			return err
		}
	}
	if info.IsDir() {
		if _, err := fmt.Fprintf(w, "D%04o 0 %s\n", info.Mode().Perm(), filepath.Base(src)); err != nil {
			return err
		}
		if err := ack.read(); err != nil {
			return err
		}
		entries, err := os.ReadDir(src)
		if err != nil {
			return err
		}
		for _, entry := range entries {
			childInfo, err := entry.Info()
			if err != nil {
				return err
			}
			child := filepath.Join(src, entry.Name())
			if err := sendTree(w, ack, child, childInfo, recursive, preserve); err != nil {
				return err
			}
		}
		if _, err := io.WriteString(w, "E\n"); err != nil {
			return err
		}
		return ack.read()
	}
	if _, err := fmt.Fprintf(w, "C%04o %d %s\n", info.Mode().Perm(), info.Size(), filepath.Base(src)); err != nil {
		return err
	}
	if err := ack.read(); err != nil {
		return err
	}
	f, err := os.Open(src)
	if err != nil {
		return err
	}
	defer f.Close()
	written, err := io.Copy(w, io.LimitReader(f, info.Size()))
	if err != nil {
		return err
	}
	if written != info.Size() {
		return fmt.Errorf("%s: read %d of %d bytes", src, written, info.Size())
	}
	if _, err := w.Write([]byte{0}); err != nil {
		return err
	}
	return ack.read()
}

// downloadOne fetches a file or tree from the remote side.
func downloadOne(opts sshOptions, remotePath, localPath string, recursive, preserve bool, stderr *os.File) int {
	if opts.Verbose {
		fmt.Fprintf(stderr, "scp: %s@%s:%s\n", opts.User, opts.Host, opts.Port)
	}
	client, err := buildSSHClient(opts, stderr)
	if err != nil {
		fmt.Fprintf(stderr, "scp: %v\n", err)
		return 1
	}
	defer client.Close()
	session, err := client.NewSession()
	if err != nil {
		fmt.Fprintf(stderr, "scp: %v\n", err)
		return 1
	}
	defer session.Close()
	stdinPipe, err := session.StdinPipe()
	if err != nil {
		fmt.Fprintf(stderr, "scp: %v\n", err)
		return 1
	}
	stdoutPipe, err := session.StdoutPipe()
	if err != nil {
		fmt.Fprintf(stderr, "scp: %v\n", err)
		return 1
	}
	session.Stderr = stderr
	sourceCmd := "scp -f "
	if preserve {
		// The source announces times only when it was asked to preserve them.
		sourceCmd = "scp -f -p "
	}
	if recursive {
		sourceCmd += "-r "
	}
	if err := session.Start(sourceCmd + shellQuote(remotePath)); err != nil {
		fmt.Fprintf(stderr, "scp: %v\n", err)
		return 1
	}
	reader := bufio.NewReader(stdoutPipe)
	ack := newScpAckFromWriter(stdinPipe)
	// The remote source waits for one acknowledgement before it sends anything.
	if err := ack.send(); err != nil {
		fmt.Fprintf(stderr, "scp: %v\n", err)
		return 1
	}
	var dirStack []string
	var pending *scpTimes
	target := localPath
	localIsDir := false
	if info, err := os.Stat(localPath); err == nil && info.IsDir() {
		// An existing directory is the parent for everything that arrives.
		target = localPath
		localIsDir = true
	}
	for {
		code, err := reader.ReadByte()
		if err == io.EOF {
			break
		}
		if err != nil {
			fmt.Fprintf(stderr, "scp: %v\n", err)
			return 1
		}
		switch code {
		case 0, '\n', '\r':
			// A stray newline shows up after the last record of a recursive
			// transfer; it carries nothing.
			continue
		case 1, 2:
			message, _ := reader.ReadString('\n')
			fmt.Fprintf(stderr, "scp: remote: %s", message)
			return 1
		case 'T':
			header, err := reader.ReadString('\n')
			if err != nil {
				fmt.Fprintf(stderr, "scp: %v\n", err)
				return 1
			}
			pending = parseScpTimes(header)
			if err := ack.send(); err != nil {
				return 1
			}
		case 'D':
			header, err := reader.ReadString('\n')
			if err != nil {
				fmt.Fprintf(stderr, "scp: %v\n", err)
				return 1
			}
			fields := strings.Fields(header)
			if len(fields) < 3 {
				fmt.Fprintf(stderr, "scp: bad directory header %q\n", header)
				return 1
			}
			name := fields[2]
			parent := target
			if len(dirStack) > 0 {
				parent = dirStack[len(dirStack)-1]
			}
			if parent == "" {
				parent = "."
			}
			dir := filepath.Join(parent, name)
			if err := os.MkdirAll(dir, 0o755); err != nil {
				fmt.Fprintf(stderr, "scp: %v\n", err)
				return 1
			}
			if _, err := os.Stat(filepath.Join(parent)); err != nil {
				fmt.Fprintf(stderr, "scp: %v\n", err)
				return 1
			}
			if pending != nil {
				applyScpTimes(dir, pending)
				pending = nil
			}
			dirStack = append(dirStack, dir)
			if target == "" {
				target = parent
			}
			if err := ack.send(); err != nil {
				return 1
			}
		case 'E':
			if len(dirStack) > 0 {
				dirStack = dirStack[:len(dirStack)-1]
			}
			if err := ack.send(); err != nil {
				return 1
			}
		case 'C':
			header, err := reader.ReadString('\n')
			if err != nil {
				fmt.Fprintf(stderr, "scp: %v\n", err)
				return 1
			}
			fields := strings.Fields(header)
			if len(fields) < 3 {
				fmt.Fprintf(stderr, "scp: bad file header %q\n", header)
				return 1
			}
			size, err := strconv.ParseInt(fields[1], 10, 64)
			if err != nil {
				fmt.Fprintf(stderr, "scp: bad size %q\n", fields[1])
				return 1
			}
			mode := uint32(0o644)
			if m, err := strconv.ParseUint(fields[0], 8, 32); err == nil {
				mode = uint32(m)
			}
			name := fields[2]
			parent := target
			if len(dirStack) > 0 {
				parent = dirStack[len(dirStack)-1]
			}
			if parent == "" {
				parent = "."
			}
			localFile := localPath
			if localIsDir || len(dirStack) > 0 {
				localFile = filepath.Join(parent, name)
			}
			if err := ack.send(); err != nil {
				return 1
			}
			f, err := os.OpenFile(localFile, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, os.FileMode(mode))
			if err != nil {
				fmt.Fprintf(stderr, "scp: %v\n", err)
				return 1
			}
			if _, err := io.CopyN(f, reader, size); err != nil {
				f.Close()
				fmt.Fprintf(stderr, "scp: %v\n", err)
				return 1
			}
			f.Close()
			if pending != nil {
				applyScpTimes(localFile, pending)
				pending = nil
			}
			status, err := reader.ReadByte()
			if err != nil {
				fmt.Fprintf(stderr, "scp: %v\n", err)
				return 1
			}
			if status != 0 {
				message, _ := reader.ReadString('\n')
				fmt.Fprintf(stderr, "scp: remote: %s", message)
				return 1
			}
			if err := ack.send(); err != nil {
				return 1
			}
		default:
			fmt.Fprintf(stderr, "scp: unexpected protocol byte %q\n", code)
			return 1
		}
	}
	if err := session.Wait(); err != nil {
		fmt.Fprintf(stderr, "scp: %v\n", err)
		return 1
	}
	return 0
}

// scpAck reads the one byte acknowledgements of the scp protocol.
type scpAck struct {
	reader *bufio.Reader
}

func newScpAck(r io.Reader) *scpAck { return &scpAck{reader: bufio.NewReader(r)} }

func (a *scpAck) read() error {
	code, err := a.reader.ReadByte()
	if err != nil {
		return err
	}
	if code == 0 {
		return nil
	}
	message, _ := a.reader.ReadString('\n')
	return fmt.Errorf("%s", strings.TrimSpace(message))
}

// scpAckFromWriter sends the acknowledgements for the download side.
type scpAckFromWriter struct {
	writer io.Writer
}

func newScpAckFromWriter(w io.Writer) *scpAckFromWriter { return &scpAckFromWriter{writer: w} }

func (a *scpAckFromWriter) send() error {
	_, err := a.writer.Write([]byte{0})
	return err
}

// shellQuote wraps a path for the remote shell.
func shellQuote(s string) string {
	return "'" + strings.ReplaceAll(s, "'", `'"'"'`) + "'"
}

func printScpHelp(stdout *os.File) {
	fmt.Fprint(stdout, `usage: scp [options] SOURCE... DEST

  -p PORT         port (also host:path with -P PORT)
  -P PORT        port, as in OpenSSH scp
  -i KEY          private key
  -o Key=Value    same keys as ssh
  -F FILE         config file
  -p              preserve modification and access times
  -r              recursive
  -q              quiet
  -v              verbose

A path with a colon before the first slash is remote: host:file, user@host:file.
`)
}
