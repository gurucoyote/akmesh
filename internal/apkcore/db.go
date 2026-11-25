package apkcore

import (
	"archive/tar"
	"bufio"
	"bytes"
	"compress/gzip"
	"crypto/sha1"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"akmesh/internal/winmode"
)

// Layout inside a target root. The paths are functions, not constants, because
// every command can point them at a tree instead of /.
func PasswdPath(root string) string    { return joinRoot(root, "/etc/passwd") }
func GroupPath(root string) string     { return joinRoot(root, "/etc/group") }
func WorldPath(root string) string     { return joinRoot(root, "/etc/apk/world") }
func RepoFilePath(root string) string  { return joinRoot(root, "/etc/apk/repositories") }
func KeysDir(root string) string       { return joinRoot(root, "/etc/apk/keys") }
func InstalledPath(root string) string { return joinRoot(root, "/lib/apk/db/installed") }
func CacheDir(root string) string      { return joinRoot(root, "/var/cache/apk") }

// ScriptsDir keeps the deinstall scripts of an installed package. apk fetches the
// package again when removing; a tree that may be offline cannot, so the scripts
// are stored at install time (our own directory, apk does not read it).
func ScriptsDir(root string) string { return joinRoot(root, "/lib/apk/db/scripts") }

func joinRoot(root, path string) string {
	if root == "" || root == "/" {
		return path
	}
	return filepath.Join(root, filepath.FromSlash(path))
}

// Installed is one entry of /lib/apk/db/installed.
type Installed struct {
	Package
	Files []InstalledFile
}

// InstalledFile is one F:/R:/Z:/a: block.
type InstalledFile struct {
	Dir  string
	Base string
	Hash string // Q1... of the content, empty for directories and links
	Mode int64
	UID  int64
	GID  int64
}

// Path returns the file's path inside the target root.
func (f InstalledFile) Path(root string) string {
	return joinRoot(root, filepath.Join("/", f.Dir, f.Base))
}

// ReadInstalled parses the install database. A missing database is empty.
func ReadInstalled(root string) ([]*Installed, error) {
	path := InstalledPath(root)
	data, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, nil
		}
		return nil, err
	}
	var out []*Installed
	var cur *Installed
	flush := func() {
		if cur != nil && cur.Name != "" {
			out = append(out, cur)
		}
		cur = nil
	}
	for _, line := range strings.Split(string(data), "\n") {
		line = strings.TrimRight(line, "\r")
		if line == "" {
			flush()
			continue
		}
		if len(line) < 2 || line[1] != ':' {
			continue
		}
		if cur == nil {
			cur = &Installed{}
		}
		val := line[2:]
		switch line[0] {
		case 'P':
			cur.Name = val
		case 'V':
			cur.Version = val
		case 'A':
			cur.Arch = val
		case 'S':
			cur.Size = atoi64(val)
		case 'I':
			cur.Installed = atoi64(val)
		case 'C':
			cur.Hash = val
		case 'T':
			cur.Description = val
		case 'U':
			cur.URL = val
		case 'L':
			cur.License = val
		case 'o':
			cur.Origin = val
		case 'm':
			cur.Maintainer = val
		case 't':
			cur.BuildTime = atoi64(val)
		case 'c':
			cur.Commit = val
		case 'D':
			cur.Depends = splitWords(val)
		case 'p':
			cur.Provides = splitWords(val)
		case 'r':
			cur.Replaces = splitWords(val)
		case 'k':
			cur.ProviderPrio = int(atoi64(val))
		case 'F':
			cur.Files = append(cur.Files, InstalledFile{Dir: val})
		case 'R':
			if n := len(cur.Files); n > 0 {
				cur.Files[n-1].Base = val
			}
		case 'a':
			if n := len(cur.Files); n > 0 {
				parts := strings.Split(val, ":")
				if len(parts) >= 3 {
					cur.Files[n-1].UID = atoi64(parts[0])
					cur.Files[n-1].GID = atoi64(parts[1])
					cur.Files[n-1].Mode = atoi64(parts[2])
				}
			}
		}
		if line[0] == 'Z' && len(cur.Files) > 0 && cur.Files[len(cur.Files)-1].Hash == "" {
			// The file hash line follows a: and is uppercase Z as well; the
			// package-level hash is the first Z of a record, before any F.
			cur.Files[len(cur.Files)-1].Hash = val
		}
	}
	flush()
	return out, nil
}

func atoi64(s string) int64 {
	var n int64
	if _, err := fmt.Sscanf(s, "%d", &n); err != nil {
		return 0
	}
	return n
}

// WriteInstalled rewrites the database. apk's own order is alphabetical by
// package name, which keeps diffs small and matches what a reader expects.
// dedupeByName keeps the last occurrence of every package name, in order of the
// first appearance.
func dedupeByName(pkgs []*Installed) []*Installed {
	last := map[string]*Installed{}
	var order []string
	for _, p := range pkgs {
		if _, seen := last[p.Name]; !seen {
			order = append(order, p.Name)
		}
		last[p.Name] = p
	}
	out := make([]*Installed, 0, len(order))
	for _, n := range order {
		out = append(out, last[n])
	}
	return out
}

func WriteInstalled(root string, pkgs []*Installed) error {
	// One entry per name, last wins: add appends the new entry and must not
	// leave the old version behind as a second block (measured 24.11.2026: a
	// downgrade produced three P: lines for one package).
	pkgs = dedupeByName(pkgs)
	path := InstalledPath(root)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	var b strings.Builder
	for _, p := range pkgs {
		fmt.Fprintf(&b, "C:%s\n", p.Hash)
		fmt.Fprintf(&b, "P:%s\n", p.Name)
		fmt.Fprintf(&b, "V:%s\n", p.Version)
		fmt.Fprintf(&b, "A:%s\n", p.Arch)
		fmt.Fprintf(&b, "S:%d\n", p.Size)
		fmt.Fprintf(&b, "I:%d\n", p.Installed)
		if p.Description != "" {
			fmt.Fprintf(&b, "T:%s\n", p.Description)
		}
		if p.URL != "" {
			fmt.Fprintf(&b, "U:%s\n", p.URL)
		}
		if p.License != "" {
			fmt.Fprintf(&b, "L:%s\n", p.License)
		}
		if p.Origin != "" {
			fmt.Fprintf(&b, "o:%s\n", p.Origin)
		}
		if p.Maintainer != "" {
			fmt.Fprintf(&b, "m:%s\n", p.Maintainer)
		}
		if p.BuildTime > 0 {
			fmt.Fprintf(&b, "t:%d\n", p.BuildTime)
		}
		if p.Commit != "" {
			fmt.Fprintf(&b, "c:%s\n", p.Commit)
		}
		if len(p.Depends) > 0 {
			fmt.Fprintf(&b, "D:%s\n", strings.Join(p.Depends, " "))
		}
		if len(p.Provides) > 0 {
			fmt.Fprintf(&b, "p:%s\n", strings.Join(p.Provides, " "))
		}
		if len(p.Replaces) > 0 {
			fmt.Fprintf(&b, "r:%s\n", strings.Join(p.Replaces, " "))
		}
		if p.ProviderPrio > 0 {
			fmt.Fprintf(&b, "k:%d\n", p.ProviderPrio)
		}
		for _, f := range p.Files {
			fmt.Fprintf(&b, "F:%s\n", f.Dir)
			fmt.Fprintf(&b, "R:%s\n", f.Base)
			if f.Hash != "" {
				fmt.Fprintf(&b, "Z:%s\n", f.Hash)
			}
			fmt.Fprintf(&b, "a:%d:%d:%o\n", f.UID, f.GID, f.Mode)
		}
		b.WriteString("\n")
	}
	return writeFileAtomic(path, []byte(b.String()), 0o644)
}

// ReadWorld reads /etc/apk/world: one constraint per line.
func ReadWorld(root string) ([]string, error) {
	data, err := os.ReadFile(WorldPath(root))
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, nil
		}
		return nil, err
	}
	var out []string
	sc := bufio.NewScanner(strings.NewReader(string(data)))
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		out = append(out, line)
	}
	return out, sc.Err()
}

// WriteWorld replaces the world file. An empty world means an empty file, which
// apk treats as "nothing wanted" rather than as an error.
func WriteWorld(root string, entries []string) error {
	path := WorldPath(root)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	body := ""
	if len(entries) > 0 {
		body = strings.Join(entries, "\n") + "\n"
	}
	return writeFileAtomic(path, []byte(body), 0o644)
}

// EntryName strips a constraint from a world line: "x>1" becomes "x".
func EntryName(entry string) string {
	// "@" is the channel tag ("pkg@testing"), the others are version constraints.
	for _, sep := range []string{"<", ">", "=", "~", "@"} {
		if i := strings.Index(entry, sep); i > 0 {
			return entry[:i]
		}
	}
	return entry
}

func writeFileAtomic(path string, data []byte, mode os.FileMode) error {
	tmp, err := os.CreateTemp(filepath.Dir(path), ".apk-*")
	if err != nil {
		return err
	}
	name := tmp.Name()
	defer os.Remove(name)
	if _, err := tmp.Write(data); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	if err := os.Chmod(name, mode); err != nil {
		return err
	}
	return os.Rename(name, path)
}

// Scripts reports which of a package's install scripts exist, so a caller can
// warn before installing. Stage one runs none of them.
func Scripts(entries map[string][]byte) []string {
	var out []string
	for _, name := range []string{".pre-install", ".post-install", ".pre-upgrade", ".post-upgrade", ".pre-deinstall", ".post-deinstall", ".trigger"} {
		if _, ok := entries[name]; ok {
			out = append(out, name)
		}
	}
	return out
}

// gzipReader opens one gzip member of a stream.
func gzipReader(raw []byte) *gzip.Reader {
	gz, err := gzip.NewReader(bytes.NewReader(raw))
	if err != nil {
		return nil
	}
	gz.Multistream(false)
	return gz
}

// ScriptBodies returns the install scripts of a package (.pre-install,
// .post-install, .trigger, ...) with their bodies, keyed by made-up name. The
// caller decides whether to run them (apkcmd runs them through akmesh's own
// shell, apkscript).
func ScriptBodies(data []byte, pkg *Package) (map[string][]byte, error) {
	streams, err := splitStreams(data)
	if err != nil {
		return nil, err
	}
	for i := range streams {
		s := &streams[i]
		if s.isSignature() {
			continue
		}
		if _, ok := s.entries[".PKGINFO"]; ok {
			out := map[string][]byte{}
			for _, name := range Scripts(s.entries) {
				out[name] = s.entries[name]
			}
			return out, nil
		}
	}
	return nil, fmt.Errorf("%s: no control segment", pkg.FullName())
}

// ExtractOptions controls what lands on disk.
type ExtractOptions struct {
	Root       string
	NoScripts  bool
	Skipped    []string
	Now        time.Time
	TotalBytes int64

	// DeferTarget is an absolute path inside Root whose file must not be
	// replaced while it is extracted: it is written next to itself as
	// DeferTarget+".apk-new" and named in Deferred, so the caller can check the
	// new file and only then move it into place. Used for the running binary,
	// where a broken replacement would take away the tool that repairs it
	// (acmesh-ieh3).
	DeferTarget string
	Deferred    *string
}

// Extract writes the data stream of a package into the root and returns the
// file records for the installed database.
// pkgInfoTriggers reads the "triggers = " lines out of a .PKGINFO body.
func pkgInfoTriggers(body []byte) []string {
	var out []string
	for _, line := range strings.Split(string(body), "\n") {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "triggers = ") {
			out = append(out, strings.Fields(strings.TrimPrefix(line, "triggers = "))...)
		}
	}
	return out
}

func Extract(data []byte, pkg *Package, opts ExtractOptions) ([]InstalledFile, []string, error) {
	streams, err := splitStreams(data)
	if err != nil {
		return nil, nil, err
	}
	var dataStream *stream
	var controlEntries map[string][]byte
	for i := range streams {
		s := &streams[i]
		if s.isSignature() {
			continue
		}
		if _, ok := s.entries[".PKGINFO"]; ok {
			controlEntries = s.entries
			// The paths a package watches live in .PKGINFO, and the index has no
			// field for them; take them from the package file itself
			// (acmesh-8h9j).
			if pkg != nil && len(pkg.Triggers) == 0 {
				pkg.Triggers = pkgInfoTriggers(s.entries[".PKGINFO"])
			}
			continue
		}
		dataStream = s
	}
	if dataStream == nil {
		return nil, nil, fmt.Errorf("%s: no data segment", pkg.FullName())
	}
	if !opts.NoScripts {
		if scripts := Scripts(controlEntries); len(scripts) > 0 {
			return nil, nil, fmt.Errorf("%s: package has install scripts (%s) and this akmesh runs none; use --no-scripts to install the files only",
				pkg.FullName(), strings.Join(scripts, " "))
		}
	}
	if err := extractTar(dataStream.raw, &opts, pkg); err != nil {
		return nil, nil, err
	}
	return dataStream.files(opts.Root), opts.Skipped, nil
}

// files lists what the data stream contained, with the hashes the installed
// database needs. It reads the stream a second time on purpose: the write pass
// must not hold every file in memory.
func (s *stream) files(root string) []InstalledFile {
	gz := gzipReader(s.raw)
	if gz == nil {
		return nil
	}
	defer gz.Close()
	tr := tar.NewReader(gz)
	var out []InstalledFile
	for {
		hdr, err := tr.Next()
		if err != nil {
			break
		}
		dir := ""
		base := hdr.Name
		if i := strings.LastIndex(strings.TrimSuffix(hdr.Name, "/"), "/"); i >= 0 {
			dir = strings.TrimSuffix(hdr.Name, "/")[:i]
			base = strings.TrimSuffix(hdr.Name, "/")[i+1:]
		}
		f := InstalledFile{Dir: dir, Base: base, Mode: int64(hdr.Mode), UID: int64(hdr.Uid), GID: int64(hdr.Gid)}
		if hdr.Typeflag == tar.TypeReg {
			sum := sha1.New()
			if _, err := io.Copy(sum, tr); err == nil {
				f.Hash = "Q1" + base64.StdEncoding.EncodeToString(sum.Sum(nil))
			}
		}
		out = append(out, f)
	}
	return out
}

func extractTar(raw []byte, opts *ExtractOptions, pkg *Package) error {
	gz := gzipReader(raw)
	if gz == nil {
		return fmt.Errorf("%s: unreadable data segment", pkg.FullName())
	}
	defer gz.Close()
	tr := tar.NewReader(gz)
	root := opts.Root
	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			return nil
		}
		if err != nil {
			return fmt.Errorf("%s: %v", pkg.FullName(), err)
		}
		target := joinRoot(root, filepath.Join("/", hdr.Name))
		// dest is where this member actually landed; it differs from target
		// only for the deferred running binary.
		dest := target
		mode := os.FileMode(hdr.Mode & 0o7777)
		switch hdr.Typeflag {
		case tar.TypeDir:
			if err := os.MkdirAll(target, mode|0o700); err != nil {
				return err
			}
			if err := os.Chmod(target, mode); err != nil {
				return err
			}
		case tar.TypeSymlink:
			if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
				return err
			}
			if err := os.RemoveAll(target); err != nil {
				return err
			}
			if err := os.Symlink(hdr.Linkname, target); err != nil {
				// Windows has no symlinks without a privilege (developer mode or
				// admin); say so instead of leaving the file out silently.
				return fmt.Errorf("%s: symlinks are not available here (%v); a package for this target must ship plain files", hdr.Name, err)
			}
		case tar.TypeLink:
			if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
				return err
			}
			link := joinRoot(root, filepath.Join("/", hdr.Linkname))
			if err := os.RemoveAll(target); err != nil {
				return err
			}
			if err := os.Link(link, target); err != nil {
				return err
			}
		case tar.TypeReg:
			dir := filepath.Dir(target)
			if err := os.MkdirAll(dir, 0o755); err != nil {
				return err
			}
			written := target
			if opts.DeferTarget != "" && filepath.Clean(target) == filepath.Clean(opts.DeferTarget) {
				// The running binary is written beside itself; the caller checks
				// it and renames it over the old one (acmesh-ieh3).
				written = target + ".apk-new"
				if opts.Deferred != nil {
					*opts.Deferred = written
				}
			}
			if err := writeMember(dir, written, mode, tr); err != nil {
				return err
			}
			dest = written
			// What actually landed on disk is the installed size apk reports.
			pkg.Installed += hdr.Size
			if !opts.Now.IsZero() {
				_ = os.Chtimes(dest, opts.Now, opts.Now)
			}
		case tar.TypeChar, tar.TypeBlock, tar.TypeFifo:
			// apk creates these with mknod; that needs privileges we do not
			// claim. Record the skip instead of failing the whole package.
			opts.Skipped = append(opts.Skipped, hdr.Name)
			continue
		default:
			continue
		}
		// Ownership is best effort: without root the files stay ours, and apk
		// behaves the same way with --usermode.
		if hdr.Typeflag != tar.TypeSymlink {
			_ = os.Lchown(dest, hdr.Uid, hdr.Gid)
		}
		// Windows has no permission bits; record them in the ACL the way our
		// chmod does, so an installed 0755 file counts as executable there too.
		if hdr.Typeflag != tar.TypeSymlink {
			_ = winmode.SetQuiet(dest, fs.FileMode(hdr.Mode).Perm())
		}
	}
}

// writeMember writes one regular file of a package next to its target and moves
// it into place in the last step (acmesh-vrt1).
//
// Why not in place: until 25.11.2026 the target was deleted and then written, so
// a crash or a full disk left it missing or half done, and a file that is
// currently being executed could not be written at all ("text file busy").
// rename(2) replaces even a running executable - the running process keeps its
// old inode - and either the old file or the complete new one is visible, never
// something in between.
func writeMember(dir, target string, mode os.FileMode, r io.Reader) error {
	// A directory in the way is the one case rename cannot handle; remove it
	// first, exactly like the old code did for every target.
	if st, err := os.Lstat(target); err == nil && st.IsDir() {
		if err := os.RemoveAll(target); err != nil {
			return err
		}
	}
	tmp, err := os.CreateTemp(dir, ".apk-*")
	if err != nil {
		return err
	}
	name := tmp.Name()
	// On success the rename below consumes the name; every other path removes it.
	defer os.Remove(name)
	// 0700 while writing, the package's mode right before the rename, so a
	// half-written file is never executable or readable by others.
	if err := tmp.Chmod(0o600); err != nil {
		tmp.Close()
		return err
	}
	if _, err := io.Copy(tmp, r); err != nil {
		tmp.Close()
		return err
	}
	// Sync before the rename: otherwise a crash can leave the name pointing at
	// an empty file on disk even though the rename itself was durable.
	if err := tmp.Sync(); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	if err := os.Chmod(name, mode); err != nil {
		return err
	}
	return os.Rename(name, target)
}

// RemoveFiles deletes the files of an installed package and prunes the
// directories that become empty, deepest first.
func RemoveFiles(root string, files []InstalledFile) (removed, kept int, err error) {
	dirs := map[string]bool{}
	for _, f := range files {
		path := f.Path(root)
		if f.Base == "" {
			continue
		}
		st, statErr := os.Lstat(path)
		if statErr != nil {
			continue
		}
		if st.IsDir() {
			dirs[path] = true
			continue
		}
		if err := os.Remove(path); err != nil && !errors.Is(err, os.ErrNotExist) {
			return removed, kept, err
		}
		removed++
	}
	// Deepest first, so a parent is only tried after its children.
	var ordered []string
	for d := range dirs {
		ordered = append(ordered, d)
	}
	sortByDepth(ordered)
	for _, d := range ordered {
		if err := os.Remove(d); err != nil {
			kept++
		}
	}
	return removed, kept, nil
}

func sortByDepth(paths []string) {
	for i := 1; i < len(paths); i++ {
		for j := i; j > 0 && strings.Count(paths[j], string(os.PathSeparator)) > strings.Count(paths[j-1], string(os.PathSeparator)); j-- {
			paths[j], paths[j-1] = paths[j-1], paths[j]
		}
	}
}

// IsRoot reports whether we can change ownership; without it the installer
// works like apk --usermode.
func IsRoot() bool { return os.Geteuid() == 0 }

// ErrPermission is returned for the cases apk reports as "Permission denied".
var ErrPermission = syscall.EACCES
