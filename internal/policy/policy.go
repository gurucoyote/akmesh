// Package policy turns a small JSON file into real restrictions on the process
// it runs in, so that a harness can decide once what the shell inside may touch
// instead of trusting every command to check.
//
// On Linux the rules are handed to Landlock, which the kernel then enforces for
// the process and for every child it starts. There is nothing to intercept and
// nothing to guess: a path outside the list fails with permission denied, no
// matter which tool asks.
package policy

import (
	"encoding/json"
	"fmt"
	"os"
	"path"
	"path/filepath"
	"strings"
	"sync"
)

// File is the on-disk shape. Paths are prefixes: a directory grants everything
// below it, a file grants that file.
type File struct {
	Version int `json:"version"`
	// Read lists paths the process may read and list.
	Read []string `json:"read"`
	// Write lists paths the process may change. Write implies read.
	Write []string `json:"write"`
	// Exec lists paths whose programs may be started.
	Exec []string `json:"exec"`
	// Network lists hosts the HTTP tools may talk to, "host" or "host:port".
	Network []string `json:"network"`
	// SystemAccess allows reading and running the usual system directories, so
	// that programs can be started at all. Defaults to true.
	SystemAccess *bool `json:"system_access"`
}

// Policy is a loaded, validated policy.
type Policy struct {
	read    []string
	write   []string
	exec    []string
	network []string
	source  string
}

// systemPaths are read and run by default; without them no program can start.
var systemPaths = []string{
	"/usr", "/bin", "/sbin", "/lib", "/lib32", "/lib64",
	"/etc/ld.so.cache", "/etc/localtime", "/proc/self", "/dev/null",
	"/dev/urandom", "/dev/random", "/dev/tty", "/dev/pts",
}

var (
	activeMu  sync.Mutex
	active    *Policy
	activeErr error
)

// Active returns the policy that was applied to this process, or nil when none
// was configured.
func Active() *Policy {
	activeMu.Lock()
	defer activeMu.Unlock()
	return active
}

// InitFromEnv loads ACMESH_POLICY and applies it. A missing variable means no
// policy, which keeps the binary usable as a plain busybox. A variable that
// points at a broken file is a hard error: running unconfined after a typo in a
// policy would be the worst possible failure mode.
func InitFromEnv() error {
	activeMu.Lock()
	defer activeMu.Unlock()
	activeErr = nil
	path := os.Getenv("ACMESH_POLICY")
	if path == "" {
		active = nil
		return nil
	}
	p, err := LoadFile(path)
	if err != nil {
		activeErr = err
		return err
	}
	if err := p.Enforce(); err != nil {
		activeErr = fmt.Errorf("policy %s: %w", path, err)
		return activeErr
	}
	p.source = path
	active = p
	return nil
}

// InitError reports why the policy could not be applied, for commands that want
// to explain themselves.
func InitError() error {
	activeMu.Lock()
	defer activeMu.Unlock()
	return activeErr
}

// LoadFile reads and validates a policy file.
func LoadFile(path string) (*Policy, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	return Parse(data, path)
}

// Parse turns JSON into a policy and checks it for mistakes that would silently
// open more than intended.
func Parse(data []byte, source string) (*Policy, error) {
	var f File
	dec := json.NewDecoder(strings.NewReader(string(data)))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&f); err != nil {
		return nil, fmt.Errorf("%s: invalid policy: %w", source, err)
	}
	if f.Version != 1 {
		return nil, fmt.Errorf("%s: unsupported policy version %d (this build knows version 1)", source, f.Version)
	}
	if len(f.Read) == 0 && len(f.Write) == 0 {
		return nil, fmt.Errorf("%s: a policy without read or write paths would block everything, add at least one", source)
	}
	systemAccess := true
	if f.SystemAccess != nil {
		systemAccess = *f.SystemAccess
	}
	p := &Policy{source: source}
	var bad []string
	for _, item := range f.Read {
		if !filepath.IsAbs(item) {
			bad = append(bad, item)
			continue
		}
		p.read = append(p.read, item)
	}
	for _, item := range f.Write {
		if !filepath.IsAbs(item) {
			bad = append(bad, item)
			continue
		}
		p.write = append(p.write, item)
	}
	for _, item := range f.Exec {
		if !filepath.IsAbs(item) {
			bad = append(bad, item)
			continue
		}
		p.exec = append(p.exec, item)
	}
	if len(bad) > 0 {
		return nil, fmt.Errorf("%s: paths must be absolute: %s", source, strings.Join(bad, ", "))
	}
	if systemAccess {
		p.read = append(p.read, systemPaths...)
		p.exec = append(p.exec, systemPaths...)
	}
	// The running binary must stay readable and runnable, otherwise a shell
	// inside the policy cannot even start its own commands.
	if self, err := os.Executable(); err == nil {
		if resolved, err := filepath.EvalSymlinks(self); err == nil {
			self = resolved
		}
		p.read = append(p.read, self)
		p.exec = append(p.exec, self)
	}
	p.network = append(p.network, f.Network...)
	return p, nil
}

// Source is the file the policy came from.
func (p *Policy) Source() string { return p.source }

// ReadPaths, WritePaths, ExecPaths and Hosts are for the policy command and for
// documentation.
func (p *Policy) ReadPaths() []string  { return append([]string(nil), p.read...) }
func (p *Policy) WritePaths() []string { return append([]string(nil), p.write...) }
func (p *Policy) ExecPaths() []string  { return append([]string(nil), p.exec...) }
func (p *Policy) Hosts() []string      { return append([]string(nil), p.network...) }

// AllowHost reports whether an HTTP tool may reach this host. An empty network
// list blocks everything, which is the safe default for a policy that says
// nothing about the network.
func (p *Policy) AllowHost(hostport string) bool {
	if p == nil {
		return true
	}
	host := hostport
	port := ""
	if idx := strings.LastIndex(hostport, ":"); idx > 0 && !strings.HasSuffix(hostport, "]") {
		host = hostport[:idx]
		port = hostport[idx+1:]
	}
	host = strings.ToLower(strings.TrimSuffix(strings.TrimPrefix(host, "["), "]"))
	for _, entry := range p.network {
		wantHost := entry
		wantPort := ""
		if idx := strings.LastIndex(entry, ":"); idx > 0 && !strings.HasSuffix(entry, "]") {
			wantHost = entry[:idx]
			wantPort = entry[idx+1:]
		}
		wantHost = strings.ToLower(strings.TrimSuffix(strings.TrimPrefix(wantHost, "["), "]"))
		if matchHost(wantHost, host) && (wantPort == "" || wantPort == port) {
			return true
		}
	}
	return false
}

// matchHost supports a leading "*." wildcard and exact names.
func matchHost(pattern, host string) bool {
	if pattern == host {
		return true
	}
	if strings.HasPrefix(pattern, "*.") {
		suffix := pattern[1:] // ".example.com"
		return strings.HasSuffix(host, suffix) && len(host) > len(suffix)
	}
	if _, err := path.Match(pattern, host); err == nil {
		if ok, err := path.Match(pattern, host); err == nil && ok {
			return true
		}
	}
	return false
}

// CheckHost is the gate the HTTP tools call before connecting.
func CheckHost(hostport string) error {
	p := Active()
	if p == nil {
		return nil
	}
	if p.AllowHost(hostport) {
		return nil
	}
	return fmt.Errorf("policy %s does not allow network access to %s", p.source, hostport)
}
