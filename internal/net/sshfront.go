//go:build !nonet

package net

import (
	"bufio"
	"fmt"
	"io"
	"net"
	"os"
	"os/user"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"time"

	"golang.org/x/crypto/ssh"
	"golang.org/x/crypto/ssh/knownhosts"
	"golang.org/x/term"
)

// sshOptions is what a command line can set. It exists so ssh and scp share the
// same connection rules and the same config parsing.
type sshOptions struct {
	User        string
	Host        string
	Port        string
	Keys        []string
	ConfigFile  string
	KnownHosts  []string
	AcceptHosts bool // StrictHostKeyChecking=no / accept-new
	BatchMode   bool
	ConnectTime time.Duration
	Verbose     bool
	ForceTTY    bool
	NoTTY       bool
	Password    string
}

// sshConfigKey is the parsed ~/.ssh/config subset. Only the keys an agent needs
// are honoured, and unknown keys are ignored on purpose: a hand-written config on
// the machine it runs on should not make the tool refuse to start.
type sshConfigKey struct {
	Patterns  []string
	HostName  string
	Port      string
	User      string
	Identity  []string
	Strict    string
	KnownFile string
	Timeout   string
}

func defaultSSHConfigPath() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".ssh", "config")
}

// readSSHConfig parses the small subset of ssh_config this tool understands.
func readSSHConfig(path string) []sshConfigKey {
	f, err := os.Open(path)
	if err != nil {
		return nil
	}
	defer f.Close()
	var out []sshConfigKey
	var current *sshConfigKey
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		fields := strings.Fields(line)
		if len(fields) < 2 {
			continue
		}
		key := strings.ToLower(fields[0])
		value := strings.TrimSpace(strings.TrimPrefix(line, fields[0]))
		value = strings.Trim(value, `"`)
		if key == "host" {
			if current != nil {
				out = append(out, *current)
			}
			current = &sshConfigKey{Patterns: strings.Fields(value)}
			continue
		}
		if current == nil {
			continue
		}
		switch key {
		case "hostname":
			current.HostName = value
		case "port":
			current.Port = value
		case "user":
			current.User = value
		case "identityfile":
			current.Identity = append(current.Identity, value)
		case "stricthostkeychecking":
			current.Strict = strings.ToLower(value)
		case "userknownhostsfile":
			current.KnownFile = value
		case "connecttimeout":
			current.Timeout = value
		}
	}
	if current != nil {
		out = append(out, *current)
	}
	return out
}

// configFor returns the first block whose pattern matches the host.
func configFor(blocks []sshConfigKey, host string) *sshConfigKey {
	for i := range blocks {
		if slices.Contains(blocks[i].Patterns, host) {
			return &blocks[i]
		}
	}
	for i := range blocks {
		for _, pattern := range blocks[i].Patterns {
			if strings.Contains(pattern, "*") {
				suffix := strings.TrimPrefix(pattern, "*")
				if suffix == "" || strings.HasSuffix(host, suffix) {
					if suffix == "" && pattern != "*" {
						continue
					}
					return &blocks[i]
				}
			}
		}
	}
	return nil
}

// buildSSHClient assembles authentication and host key checking. The result is
// the same for ssh and for every scp transfer.
func buildSSHClient(opts sshOptions, stderr *os.File) (*ssh.Client, error) {
	var auths []ssh.AuthMethod
	keys := opts.Keys
	if len(keys) == 0 {
		home, _ := os.UserHomeDir()
		for _, name := range []string{"id_ed25519", "id_ecdsa", "id_rsa"} {
			path := filepath.Join(home, ".ssh", name)
			if _, err := os.Stat(path); err == nil {
				keys = append(keys, path)
			}
		}
	}
	for _, path := range keys {
		data, err := os.ReadFile(path)
		if err != nil {
			if opts.Verbose {
				fmt.Fprintf(stderr, "ssh: cannot read key %s: %v\n", path, err)
			}
			continue
		}
		signer, err := ssh.ParsePrivateKey(data)
		if err != nil {
			// A key with a passphrase cannot be used without asking, and asking
			// is exactly what a non-interactive run must not do.
			fmt.Fprintf(stderr, "ssh: key %s needs a passphrase and is skipped\n", path)
			continue
		}
		auths = append(auths, ssh.PublicKeys(signer))
	}
	if opts.Password != "" {
		auths = append(auths, ssh.Password(opts.Password))
	}
	if len(auths) == 0 && !opts.BatchMode {
		// Only reached with a terminal; a script gets a clear error instead.
		if term.IsTerminal(int(os.Stdin.Fd())) {
			auths = append(auths, ssh.PasswordCallback(func() (string, error) {
				fmt.Fprint(stderr, "password: ")
				b, err := term.ReadPassword(int(os.Stdin.Fd()))
				fmt.Fprintln(stderr)
				return string(b), err
			}))
		}
	}
	if len(auths) == 0 {
		return nil, fmt.Errorf("no usable key and no password; a non-interactive run cannot ask for one")
	}

	callback, err := hostKeyCallback(opts, stderr)
	if err != nil {
		return nil, err
	}

	cfg := &ssh.ClientConfig{
		User:            opts.User,
		Auth:            auths,
		HostKeyCallback: callback,
		Timeout:         opts.ConnectTime,
	}
	addr := net.JoinHostPort(opts.Host, opts.Port)
	return ssh.Dial("tcp", addr, cfg)
}

// hostKeyCallback implements the three modes that matter: strict, no check, and
// accept-new (learn on first contact, complain if the key changes later).
func hostKeyCallback(opts sshOptions, stderr *os.File) (ssh.HostKeyCallback, error) {
	if opts.AcceptHosts {
		return ssh.InsecureIgnoreHostKey(), nil
	}
	files := opts.KnownHosts
	if len(files) == 0 {
		home, _ := os.UserHomeDir()
		files = []string{filepath.Join(home, ".ssh", "known_hosts")}
	}
	for _, name := range files {
		if _, err := os.Stat(name); err != nil {
			return nil, fmt.Errorf("known_hosts %s: %v (use -o StrictHostKeyChecking=no for a first contact, or -o UserKnownHostsFile=...)", name, err)
		}
	}
	return knownhosts.New(files...)
}

// runSSH is the ssh front: options, a destination and an optional remote command.
func runSSH(args []string, stdin *os.File, stdout, stderr *os.File) int {
	opts := sshOptions{
		Port:        "22",
		ConfigFile:  defaultSSHConfigPath(),
		BatchMode:   !term.IsTerminal(int(stdin.Fd())),
		ConnectTime: 0,
	}
	var dest string
	var remoteCommand []string
	var overrides = map[string]string{}
	for i := 0; i < len(args); i++ {
		a := args[i]
		next := func(what string) (string, bool) {
			if i+1 >= len(args) {
				fmt.Fprintf(stderr, "ssh: %s needs a value\n", what)
				return "", false
			}
			i++
			return args[i], true
		}
		switch {
		case a == "--help" || a == "-h":
			printSSHHelp(stdout)
			return 0
		case a == "-V":
			fmt.Fprintln(stdout, "ssh (akmesh, golang.org/x/crypto/ssh)")
			return 0
		case a == "-p":
			v, ok := next("-p")
			if !ok {
				return 255
			}
			opts.Port = v
		case strings.HasPrefix(a, "-p") && len(a) > 2:
			opts.Port = a[2:]
		case a == "-i":
			v, ok := next("-i")
			if !ok {
				return 255
			}
			opts.Keys = append(opts.Keys, v)
		case strings.HasPrefix(a, "-i") && len(a) > 2:
			opts.Keys = append(opts.Keys, a[2:])
		case a == "-F":
			v, ok := next("-F")
			if !ok {
				return 255
			}
			opts.ConfigFile = v
		case a == "-l":
			v, ok := next("-l")
			if !ok {
				return 255
			}
			opts.User = v
		case a == "-o":
			v, ok := next("-o")
			if !ok {
				return 255
			}
			name, value, ok := strings.Cut(v, "=")
			if !ok {
				fmt.Fprintf(stderr, "ssh: -o needs Key=Value, got %q\n", v)
				return 255
			}
			overrides[strings.ToLower(strings.TrimSpace(name))] = strings.TrimSpace(value)
		case a == "-v":
			opts.Verbose = true
		case a == "-t":
			opts.ForceTTY = true
		case a == "-T":
			opts.NoTTY = true
		case a == "-N", a == "-q", a == "-C", a == "-A", a == "-X":
			fmt.Fprintf(stderr, "ssh: %s is not supported by this build\n", a)
			return 255
		case strings.HasPrefix(a, "-") && len(a) > 1:
			fmt.Fprintf(stderr, "ssh: unsupported option %q\n", a)
			return 255
		default:
			if dest == "" {
				dest = a
			} else {
				remoteCommand = append(remoteCommand, args[i:]...)
				i = len(args)
			}
		}
	}
	if dest == "" {
		fmt.Fprintln(stderr, "usage: ssh [options] [user@]host[:port] [command...]")
		return 255
	}
	if err := applyDestination(&opts, dest); err != nil {
		fmt.Fprintf(stderr, "ssh: %v\n", err)
		return 255
	}
	if err := applyConfig(&opts); err != nil {
		fmt.Fprintf(stderr, "ssh: %v\n", err)
		return 255
	}
	applyOverrides(&opts, overrides, stderr)

	if opts.Verbose {
		fmt.Fprintf(stderr, "ssh: %s@%s:%s\n", opts.User, opts.Host, opts.Port)
	}
	client, err := buildSSHClient(opts, stderr)
	if err != nil {
		fmt.Fprintf(stderr, "ssh: %v\n", err)
		return 255
	}
	defer client.Close()

	session, err := client.NewSession()
	if err != nil {
		fmt.Fprintf(stderr, "ssh: %v\n", err)
		return 255
	}
	defer session.Close()
	session.Stdin = stdin
	session.Stdout = stdout
	session.Stderr = stderr

	useTTY := opts.ForceTTY || (!opts.NoTTY && len(remoteCommand) == 0 && term.IsTerminal(int(stdin.Fd())))
	if useTTY {
		width, height := 80, 24
		if w, h, err := term.GetSize(int(stdout.Fd())); err == nil && w > 0 {
			width, height = w, h
		}
		if err := session.RequestPty(os.Getenv("TERM"), height, width, ssh.TerminalModes{
			ssh.ECHO: 1, ssh.TTY_OP_ISPEED: 14400, ssh.TTY_OP_OSPEED: 14400,
		}); err != nil {
			fmt.Fprintf(stderr, "ssh: pty: %v\n", err)
			return 255
		}
	}
	var runErr error
	if len(remoteCommand) > 0 {
		runErr = session.Run(strings.Join(remoteCommand, " "))
	} else {
		runErr = session.Shell()
		if runErr == nil {
			runErr = session.Wait()
		}
	}
	if runErr == nil {
		return 0
	}
	if exitErr, ok := runErr.(*ssh.ExitError); ok {
		// The remote status is the status: this is what a script branches on.
		if exitErr.Signal() != "" && exitErr.ExitStatus() == -1 {
			fmt.Fprintf(stderr, "ssh: remote command killed by %s\n", exitErr.Signal())
			return 255
		}
		return exitErr.ExitStatus()
	}
	fmt.Fprintf(stderr, "ssh: %v\n", runErr)
	return 255
}

// applyDestination splits [user@]host[:port] out of the positional argument.
func applyDestination(opts *sshOptions, dest string) error {
	dest = strings.TrimPrefix(dest, "ssh://")
	if idx := strings.LastIndex(dest, "@"); idx >= 0 {
		opts.User = dest[:idx]
		dest = dest[idx+1:]
	}
	host, port, err := net.SplitHostPort(dest)
	if err == nil {
		opts.Host = host
		opts.Port = port
	} else {
		opts.Host = strings.TrimSuffix(strings.TrimPrefix(dest, "["), "]")
	}
	if opts.Host == "" {
		return fmt.Errorf("no host given")
	}
	if opts.User == "" {
		if u, err := user.Current(); err == nil {
			opts.User = u.Username
		}
		if opts.User == "" {
			opts.User = "root"
		}
	}
	return nil
}

// applyConfig fills what the command line left open from ~/.ssh/config.
func applyConfig(opts *sshOptions) error {
	blocks := readSSHConfig(opts.ConfigFile)
	block := configFor(blocks, opts.Host)
	if block == nil {
		return nil
	}
	if block.HostName != "" {
		opts.Host = block.HostName
	}
	if block.Port != "" && opts.Port == "22" {
		opts.Port = block.Port
	}
	if block.User != "" && opts.User == "" {
		opts.User = block.User
	}
	if len(opts.Keys) == 0 && len(block.Identity) > 0 {
		for _, path := range block.Identity {
			if strings.HasPrefix(path, "~") {
				home, _ := os.UserHomeDir()
				path = filepath.Join(home, path[1:])
			}
			opts.Keys = append(opts.Keys, path)
		}
	}
	if block.Strict != "" && (block.Strict == "no" || block.Strict == "accept-new") {
		opts.AcceptHosts = true
	}
	if block.KnownFile != "" && len(opts.KnownHosts) == 0 {
		for item := range strings.FieldsSeq(block.KnownFile) {
			if item == "/dev/null" {
				opts.AcceptHosts = true
				continue
			}
			opts.KnownHosts = append(opts.KnownHosts, item)
		}
	}
	if block.Timeout != "" && opts.ConnectTime == 0 {
		if secs, err := strconv.Atoi(block.Timeout); err == nil && secs > 0 {
			opts.ConnectTime = time.Duration(secs) * time.Second
		}
	}
	return nil
}

// applyOverrides applies -o Key=Value, which wins over the config file.
func applyOverrides(opts *sshOptions, overrides map[string]string, stderr *os.File) {
	for name, value := range overrides {
		switch name {
		case "port":
			opts.Port = value
		case "identityfile":
			opts.Keys = append(opts.Keys, value)
		case "user":
			opts.User = value
		case "stricthostkeychecking":
			if value == "no" || value == "accept-new" {
				opts.AcceptHosts = true
			}
		case "userknownhostsfile":
			for item := range strings.FieldsSeq(value) {
				if item == "/dev/null" {
					opts.AcceptHosts = true
					continue
				}
				opts.KnownHosts = append(opts.KnownHosts, item)
			}
		case "batchmode":
			opts.BatchMode = value == "yes" || value == "true"
		case "connecttimeout":
			if secs, err := strconv.Atoi(value); err == nil && secs > 0 {
				opts.ConnectTime = time.Duration(secs) * time.Second
			}
		case "logLevel", "loglevel":
			opts.Verbose = strings.EqualFold(value, "verbose") || strings.EqualFold(value, "debug")
		case "pubkeyauthentication", "passwordauthentication":
			// accepted and ignored: the build uses key auth, password only with a tty
		default:
			fmt.Fprintf(stderr, "ssh: ignoring unsupported option %s\n", name)
		}
	}
}

func printSSHHelp(stdout *os.File) {
	fmt.Fprint(stdout, `usage: ssh [options] [user@]host[:port] [command...]

  -p PORT         port (also user@host:port)
  -l USER         user
  -i KEY          private key, repeatable
  -o Key=Value    Port, User, IdentityFile, StrictHostKeyChecking=no|accept-new,
                  UserKnownHostsFile, BatchMode, ConnectTimeout
  -F FILE         config file (default ~/.ssh/config)
  -t              force a terminal
  -T              no terminal
  -v              verbose

Non-interactive use: with stdin not a terminal there is no pty and no password
prompt; a missing key is an error, not a question. The remote exit status is
returned as the exit status of this command.
`)
}

// pipeThrough is the helper scp uses: run a command over an existing client and
// hand back its streams.
func pipeThrough(client *ssh.Client, command string, stderr io.Writer) (*ssh.Session, io.WriteCloser, io.Reader, error) {
	session, err := client.NewSession()
	if err != nil {
		return nil, nil, nil, err
	}
	writer, err := session.StdinPipe()
	if err != nil {
		session.Close()
		return nil, nil, nil, err
	}
	reader, err := session.StdoutPipe()
	if err != nil {
		session.Close()
		return nil, nil, nil, err
	}
	session.Stderr = stderr
	if err := session.Start(command); err != nil {
		session.Close()
		return nil, nil, nil, err
	}
	return session, writer, reader, nil
}
