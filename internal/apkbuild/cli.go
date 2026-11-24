package apkbuild

import (
	"encoding/base64"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"akmesh/internal/apkcore"
)

// Run is the command itself: apkbuild build|index|keygen.
func Run(args []string, stdout, stderr io.Writer) int {
	if len(args) == 0 {
		usage(stderr)
		return 1
	}
	switch args[0] {
	case "build":
		return runBuild(args[1:], stdout, stderr)
	case "index":
		return runIndex(args[1:], stdout, stderr)
	case "keygen":
		return runKeygen(args[1:], stdout, stderr)
	case "-h", "--help", "help":
		usage(stdout)
		return 0
	default:
		fmt.Fprintf(stderr, "apkbuild: unknown subcommand %q (build, index, keygen)\n", args[0])
		return 1
	}
}

func usage(w io.Writer) {
	fmt.Fprint(w, `usage: apkbuild <subcommand> [options]

  build  --manifest FILE --source DIR [-o DIR] [--key KEY] [--no-sign]
         builds one .apk from a manifest and a directory tree
  index  -o APKINDEX.tar.gz [?]PKGFILE... [--key KEY] [--no-sign]
         writes or extends a repository index
  keygen -o KEY [--bits 2048]
         writes an RSA key pair, KEY and KEY.pub

Manifest, one "key = value" per line:
  pkgname, pkgver, pkgrel, pkgdesc, url, license, maintainer, arch,
  depends, provides, replaces, source_date_epoch,
  script = .post-install:path/to/script
`)
}

// flags is a tiny parser: enough for the three subcommands, and it accepts
// options before or after operands like the rest of akmesh.
type flags struct {
	strs  map[string]string
	bools map[string]bool
	rest  []string
	known map[string]bool
	noVal map[string]bool
}

func newFlags(valueOpts, boolOpts []string) *flags {
	f := &flags{strs: map[string]string{}, bools: map[string]bool{}, known: map[string]bool{}, noVal: map[string]bool{}}
	for _, o := range valueOpts {
		f.known[o] = true
	}
	for _, o := range boolOpts {
		f.known[o] = true
		f.noVal[o] = true
	}
	return f
}

func (f *flags) parse(args []string) error {
	for i := 0; i < len(args); i++ {
		a := args[i]
		if a == "--" {
			f.rest = append(f.rest, args[i+1:]...)
			return nil
		}
		if !strings.HasPrefix(a, "-") || a == "-" {
			f.rest = append(f.rest, a)
			continue
		}
		name, val, hasVal := a, "", false
		if strings.HasPrefix(a, "--") {
			if eq := strings.Index(a, "="); eq >= 0 {
				name, val, hasVal = a[:eq], a[eq+1:], true
			}
		}
		if !f.known[name] {
			return fmt.Errorf("unknown option %s", name)
		}
		if f.noVal[name] {
			f.bools[name] = true
			continue
		}
		if !hasVal {
			if i+1 >= len(args) {
				return fmt.Errorf("option %s needs a value", name)
			}
			i++
			val = args[i]
		}
		f.strs[name] = val
	}
	return nil
}

func (f *flags) get(names ...string) string {
	for _, n := range names {
		if v, ok := f.strs[n]; ok {
			return v
		}
	}
	return ""
}

func runBuild(args []string, stdout, stderr io.Writer) int {
	f := newFlags([]string{"--manifest", "--source", "-o", "--out", "--key"}, []string{"--no-sign"})
	if err := f.parse(args); err != nil {
		fmt.Fprintf(stderr, "apkbuild: %v\n", err)
		return 1
	}
	manifestPath := f.get("--manifest")
	source := f.get("--source")
	if manifestPath == "" || source == "" {
		fmt.Fprintf(stderr, "apkbuild: build needs --manifest FILE and --source DIR\n")
		return 1
	}
	out := f.get("-o", "--out")
	if out == "" {
		out = "."
	}
	m, err := ReadManifest(manifestPath)
	if err != nil {
		fmt.Fprintf(stderr, "apkbuild: %v\n", err)
		return 1
	}
	key := f.get("--key")
	if f.bools["--no-sign"] {
		key = ""
	}
	res, err := Build(m, source, out, key)
	if err != nil {
		fmt.Fprintf(stderr, "apkbuild: %v\n", err)
		return 1
	}
	fmt.Fprintf(stdout, "%s (%d bytes, %d installed)\n", res.Path, res.Size, res.Installed)
	return 0
}

func runIndex(args []string, stdout, stderr io.Writer) int {
	f := newFlags([]string{"-o", "--output", "--key", "--repo"}, []string{"--no-sign"})
	if err := f.parse(args); err != nil {
		fmt.Fprintf(stderr, "apkbuild: %v\n", err)
		return 1
	}
	out := f.get("-o", "--output")
	if out == "" {
		fmt.Fprintf(stderr, "apkbuild: index needs -o APKINDEX.tar.gz\n")
		return 1
	}
	key := f.get("--key")
	if f.bools["--no-sign"] {
		key = ""
	}
	// Start from an existing index in the same directory, so "index" extends a
	// repository instead of wiping it.
	var entries []IndexEntry
	if old, err := ReadIndexFile(out); err == nil {
		entries = old
	}
	// Key by name and version: a repository keeps old versions so a host can
	// pin or roll back (measured 24.11.2026, acmesh-o6ja).
	byName := map[string]IndexEntry{}
	for _, e := range entries {
		byName[e.Name+"-"+e.Version] = e
	}
	when := time.Now()
	for _, path := range f.rest {
		e, err := entryForPackage(path, when)
		if err != nil {
			fmt.Fprintf(stderr, "apkbuild: %v\n", err)
			return 1
		}
		byName[e.Name+"-"+e.Version] = e
	}
	names := make([]string, 0, len(byName))
	for key := range byName {
		names = append(names, key)
	}
	sort.Strings(names)
	all := make([]IndexEntry, 0, len(names))
	for _, name := range names {
		all = append(all, byName[name])
	}
	size, err := WriteIndex(all, out, key)
	if err != nil {
		fmt.Fprintf(stderr, "apkbuild: %v\n", err)
		return 1
	}
	fmt.Fprintf(stdout, "%s (%d bytes, %d packages)\n", out, size, len(all))
	return 0
}

// entryForPackage reads .PKGINFO and the control hash out of a built package, so
// the index describes the file that is on disk rather than a manifest that may
// have changed since.
func entryForPackage(path string, when time.Time) (IndexEntry, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return IndexEntry{}, err
	}
	streams, err := apkcore.Split(data)
	if err != nil {
		return IndexEntry{}, fmt.Errorf("%s: %v", filepath.Base(path), err)
	}
	var pkginfo string
	var controlSHA string
	var e IndexEntry
	gotControl := false
	for _, s := range streams {
		if body, ok := s.Entries[".PKGINFO"]; ok {
			pkginfo = string(body)
			controlSHA = "Q1" + base64.StdEncoding.EncodeToString(sha1Sum(s.Raw))
			gotControl = true
		}
	}
	if !gotControl {
		return IndexEntry{}, fmt.Errorf("%s: no .PKGINFO", filepath.Base(path))
	}
	info, err := os.Stat(path)
	if err != nil {
		return IndexEntry{}, err
	}
	size := info.Size()
	e = IndexEntry{
		Name:        fieldOf(pkginfo, "pkgname"),
		Version:     fieldOf(pkginfo, "pkgver"),
		Arch:        fieldOf(pkginfo, "arch"),
		Size:        size,
		Installed:   atoi64(fieldOf(pkginfo, "size")),
		ControlSHA:  controlSHA,
		Description: fieldOf(pkginfo, "pkgdesc"),
		URL:         fieldOf(pkginfo, "url"),
		License:     fieldOf(pkginfo, "license"),
		Maintainer:  fieldOf(pkginfo, "packager"),
		BuildTime:   atoi64(fieldOf(pkginfo, "builddate")),
		Depends:     strings.Fields(fieldOf(pkginfo, "depend")),
		Provides:    strings.Fields(fieldOf(pkginfo, "provides")),
		Replaces:    strings.Fields(fieldOf(pkginfo, "replaces")),
	}
	if e.BuildTime == 0 {
		e.BuildTime = when.Unix()
	}
	return e, nil
}

func fieldOf(pkginfo, key string) string {
	for _, line := range strings.Split(pkginfo, "\n") {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "#") {
			continue
		}
		parts := strings.SplitN(line, "=", 2)
		if len(parts) == 2 && strings.TrimSpace(parts[0]) == key {
			return strings.TrimSpace(parts[1])
		}
	}
	return ""
}

func runKeygen(args []string, stdout, stderr io.Writer) int {
	f := newFlags([]string{"-o", "--out", "--bits"}, nil)
	if err := f.parse(args); err != nil {
		fmt.Fprintf(stderr, "apkbuild: %v\n", err)
		return 1
	}
	out := f.get("-o", "--out")
	if out == "" {
		out = "local.rsa"
	}
	bits := 2048
	if v := f.get("--bits"); v != "" {
		n, err := atoiErr(v)
		if err != nil || n < 2048 {
			fmt.Fprintf(stderr, "apkbuild: --bits needs at least 2048\n")
			return 1
		}
		bits = int(n)
	}
	kp, err := GenerateKey(out, bits)
	if err != nil {
		fmt.Fprintf(stderr, "apkbuild: %v\n", err)
		return 1
	}
	fmt.Fprintf(stdout, "%s and %s (%d bits)\n", out, kp.Name, bits)
	return 0
}

func atoiErr(s string) (int64, error) {
	var n int64
	_, err := fmt.Sscanf(s, "%d", &n)
	return n, err
}
