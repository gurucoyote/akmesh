// Package apkcmd is the apk subcommand: it installs packages from signed
// repositories into a tree, without C apk-tools and without the apko library
// (see internal/apkcore for the measurement that ruled the library out).
//
// The wording follows apk-tools, because an agent reads these lines:
// "(1/1) Installing name (ver)" and "OK: 43 B in 1 packages".
package apkcmd

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	"akmesh/internal/apkcore"
	"akmesh/internal/apkscript"
)

type options struct {
	root       string
	repos      []string
	allowUntr  bool
	noScripts  bool
	noCache    bool
	arch       string
	quiet      bool
	verbose    bool
	updateOnly bool
}

// Run answers "apk <subcommand>".
func Run(cmd string, args []string, stdin io.Reader, stdout, stderr io.Writer) int {
	if len(args) == 0 {
		fmt.Fprintf(stderr, "%s: no subcommand; try update, add, del, info, list, search or upgrade\n", cmd)
		return 1
	}
	// Options may stand before or after the subcommand ("apk --root X add y"
	// and "apk add --root X y" are both typed), so the parser collects the
	// operands and the first one names the subcommand.
	o, operands, err := parseOptions(args)
	if err != nil {
		fmt.Fprintf(stderr, "%s: %v\n", cmd, err)
		return 1
	}
	if len(operands) == 0 {
		fmt.Fprintf(stderr, "%s: no subcommand; try update, add, del, info, list, search or upgrade\n", cmd)
		return 1
	}
	sub := operands[0]
	operands = operands[1:]
	c, code := newClient(cmd, o, stderr)
	if code != 0 {
		return code
	}
	switch sub {
	case "update":
		return update(cmd, c, o, operands, stdout, stderr)
	case "add":
		return add(cmd, c, o, operands, stdout, stderr)
	case "del", "remove":
		return del(cmd, c, o, operands, stdout, stderr)
	case "upgrade":
		return upgrade(cmd, c, o, operands, stdout, stderr)
	case "info":
		return info(cmd, o, operands, stdout, stderr)
	case "list":
		return list(cmd, c, o, operands, stdout, stderr)
	case "search":
		return search(cmd, c, operands, stdout, stderr)
	default:
		fmt.Fprintf(stderr, "%s: unknown subcommand %q\n", cmd, sub)
		return 1
	}
}

func parseOptions(args []string) (options, []string, error) {
	o := options{root: os.Getenv("AKMESH_ROOT"), arch: arch()}
	var rest []string
	for i := 0; i < len(args); i++ {
		a := args[i]
		next := func() (string, error) {
			if i+1 >= len(args) {
				return "", fmt.Errorf("option %s needs a value", a)
			}
			i++
			return args[i], nil
		}
		switch {
		case a == "-p" || a == "--root":
			v, err := next()
			if err != nil {
				return o, nil, err
			}
			o.root = v
		case strings.HasPrefix(a, "--root="):
			o.root = strings.TrimPrefix(a, "--root=")
		case a == "-X" || a == "--repository":
			v, err := next()
			if err != nil {
				return o, nil, err
			}
			o.repos = append(o.repos, v)
		case strings.HasPrefix(a, "--repository="):
			o.repos = append(o.repos, strings.TrimPrefix(a, "--repository="))
		case a == "-a" || a == "--arch":
			v, err := next()
			if err != nil {
				return o, nil, err
			}
			o.arch = v
		case a == "--allow-untrusted":
			o.allowUntr = true
		case a == "--no-scripts":
			o.noScripts = true
		case a == "--no-cache":
			o.noCache = true
		case a == "-q" || a == "--quiet":
			o.quiet = true
		case a == "-v" || a == "--verbose":
			o.verbose = true
		case a == "--no-network":
			// Nothing is fetched from a cache yet; accepted so scripts keep working.
		case a == "--":
			rest = append(rest, args[i+1:]...)
			i = len(args)
		case strings.HasPrefix(a, "-") && a != "-":
			return o, nil, fmt.Errorf("unrecognized option '%s'", a)
		default:
			rest = append(rest, a)
		}
	}
	return o, rest, nil
}

func arch() string {
	switch {
	case os.Getenv("AKMESH_APK_ARCH") != "":
		return os.Getenv("AKMESH_APK_ARCH")
	default:
		return "x86_64"
	}
}

// newClient collects keys and repositories from the target root.
func newClient(cmd string, o options, stderr io.Writer) (*apkcore.Client, int) {
	keys, err := loadKeys(o.root)
	if err != nil {
		fmt.Fprintf(stderr, "%s: %v\n", cmd, err)
		return nil, 1
	}
	var repos []Repo
	for _, u := range o.repos {
		repos = append(repos, Repo{URL: u})
	}
	if len(repos) == 0 {
		repos, err = readRepos(o.root)
		if err != nil {
			fmt.Fprintf(stderr, "%s: %v\n", cmd, err)
			return nil, 1
		}
	}
	cache := ""
	if !o.noCache {
		cache = apkcore.CacheDir(o.root)
	}
	c := apkcore.NewClient(o.arch, keys, cache)
	for _, r := range repos {
		c.Repos = append(c.Repos, r.URL)
	}
	return c, 0
}

func loadKeys(root string) ([]*apkcore.Key, error) {
	var keys []*apkcore.Key
	for _, dir := range []string{apkcore.KeysDir(root), "/etc/apk/keys"} {
		entries, err := os.ReadDir(dir)
		if err != nil {
			continue
		}
		for _, e := range entries {
			if e.IsDir() {
				continue
			}
			data, err := os.ReadFile(filepath.Join(dir, e.Name()))
			if err != nil {
				continue
			}
			k, err := apkcore.LoadKey(e.Name(), data)
			if err != nil {
				continue
			}
			keys = append(keys, k)
		}
	}
	return keys, nil
}

// readRepos reads /etc/apk/repositories inside the root.
// Repo is one line of /etc/apk/repositories: a URL and an optional channel tag.
type Repo struct {
	Tag string
	URL string
}

func readRepos(root string) ([]Repo, error) {
	data, err := os.ReadFile(apkcore.RepoFilePath(root))
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	var out []Repo
	for _, line := range strings.Split(string(data), "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		// apk's syntax: "@tag url" or "url"; an "@tag" may also follow the url.
		var r Repo
		for _, field := range strings.Fields(line) {
			if strings.HasPrefix(field, "@") {
				r.Tag = strings.TrimPrefix(field, "@")
				continue
			}
			if r.URL == "" {
				r.URL = field
			}
		}
		if r.URL == "" {
			continue
		}
		out = append(out, r)
	}
	return out, nil
}

// loadIndexes fetches every repository index and refuses unsigned ones unless
// the caller said --allow-untrusted.
// requireRepos complains only for the subcommands that really need a
// repository; info and del work on an installed tree alone.
// requireRepos returns the repositories with their channel tags: -X wins, then
// /etc/apk/repositories (which is where "@stable https://..." lives).
func requireRepos(cmd string, o options, stderr io.Writer, c *apkcore.Client) []Repo {
	var repos []Repo
	for _, u := range o.repos {
		repos = append(repos, Repo{URL: u})
	}
	if len(repos) == 0 {
		if fromFile, err := readRepos(o.root); err == nil && len(fromFile) > 0 {
			repos = fromFile
		}
	}
	if len(repos) == 0 {
		fmt.Fprintf(stderr, "%s: no repositories configured (use -X or %s)\n", cmd, apkcore.RepoFilePath(o.root))
	}
	return repos
}

func loadIndexes(cmd string, c *apkcore.Client, o options, repos []Repo, stderr io.Writer) ([]*apkcore.Index, int) {
	var out []*apkcore.Index
	for _, repo := range repos {
		idx, err := c.Index(repo.URL)
		if err != nil {
			fmt.Fprintf(stderr, "%s: %v\n", cmd, err)
			return nil, 1
		}
		if (idx.Unsigned || idx.Untrusted) && !o.allowUntr {
			fmt.Fprintf(stderr, "%s: %s: UNTRUSTED signature\n", cmd, repo.URL)
			return nil, 1
		}
		if o.verbose {
			fmt.Fprintf(stderr, "%s: %s: %d packages%s\n", cmd, repo.URL, len(idx.Packages), sigNote(idx))
		}
		idx.Tag = repo.Tag
		out = append(out, idx)
	}
	return out, 0
}

func sigNote(idx *apkcore.Index) string {
	if len(idx.Keys) > 0 {
		return " (signed by " + strings.Join(idx.Keys, ", ") + ")"
	}
	return " (unsigned)"
}

// best finds a package by name (or provides) across the indexes.
// depSpec is one dependency as it appears in an index: "libcrypto3>=3.1.0",
// "openssh-keygen=10.3_p1-r2", "!openssh-client-krb5", "so:libz.so.1" or with a
// channel tag "pkg@testing". apk accepts all of these, so we must too.
type depSpec struct {
	name     string
	op       string
	ver      string
	tag      string
	conflict bool
	by       string // who asked for it, for the error message
}

func (d depSpec) wantedBy() string {
	if d.by == "" {
		return ""
	}
	return " for " + d.by
}

// parseDep splits a dependency string. The operators are the ones apk uses; "~"
// means "same major.minor", which is how Alpine pins a library to its soname.
func parseDep(spec string) depSpec {
	d := depSpec{}
	spec = strings.TrimSpace(spec)
	if strings.HasPrefix(spec, "!") {
		d.conflict = true
		spec = spec[1:]
	}
	if i := strings.Index(spec, "@"); i > 0 {
		d.tag = spec[i+1:]
		spec = spec[:i]
	}
	for _, op := range []string{">=", "<=", "=", ">", "<", "~"} {
		if i := strings.Index(spec, op); i > 0 {
			d.name = spec[:i]
			d.op = op
			d.ver = spec[i+len(op):]
			return d
		}
	}
	d.name = spec
	return d
}

// satisfies reports whether ver meets the constraint. An empty operator accepts
// anything, which is the common case.
func satisfies(ver, op, want string) bool {
	if op == "" || want == "" {
		return true
	}
	c := apkcore.CompareVersions(ver, want)
	switch op {
	case "=":
		return c == 0
	case ">=":
		return c >= 0
	case "<=":
		return c <= 0
	case ">":
		return c > 0
	case "<":
		return c < 0
	case "~":
		// Same major.minor: 3.5.9 satisfies 3.5, not 3.4.
		return c >= 0 && strings.HasPrefix(ver, strings.TrimSuffix(want, "-r")+".") ||
			ver == want
	}
	return true
}

// bestSatisfying picks the newest package that meets the constraint, first by
// name and then through provides ("so:libcrypto.so.3=3").
func bestSatisfying(indexes []*apkcore.Index, d depSpec, tag string) *apkcore.Package {
	var found *apkcore.Package
	for _, idx := range indexes {
		if tag != "" && idx.Tag != tag {
			continue
		}
		for _, p := range idx.Packages {
			if p.Name != d.name || !satisfies(p.Version, d.op, d.ver) {
				continue
			}
			if found == nil || apkcore.CompareVersions(p.Version, found.Version) > 0 {
				found = p
			}
		}
	}
	if found != nil {
		return found
	}
	bestPrio := -1
	for _, idx := range indexes {
		if tag != "" && idx.Tag != tag {
			continue
		}
		for _, p := range idx.Packages {
			for _, prov := range p.Provides {
				prov = strings.TrimPrefix(prov, "!")
				name, ver := prov, ""
				if i := strings.Index(prov, "="); i > 0 {
					name, ver = prov[:i], prov[i+1:]
				}
				if name != d.name {
					continue
				}
				// A provider without its own version offers the package version.
				if ver == "" {
					ver = p.Version
				}
				if !satisfies(ver, d.op, d.ver) {
					continue
				}
				if found == nil || p.ProviderPrio > bestPrio ||
					(p.ProviderPrio == bestPrio && apkcore.CompareVersions(p.Version, found.Version) > 0) {
					found = p
					bestPrio = p.ProviderPrio
				}
			}
		}
	}
	return found
}

// best returns the newest package of that name, optionally only from one
// channel tag ("" means any repository).
func best(indexes []*apkcore.Index, name string) *apkcore.Package {
	return bestTagged(indexes, name, "")
}

func bestTagged(indexes []*apkcore.Index, name, tag string) *apkcore.Package {
	var found *apkcore.Package
	for _, idx := range indexes {
		if tag != "" && idx.Tag != tag {
			continue
		}
		for _, p := range idx.Packages {
			if p.Name != name {
				continue
			}
			if found == nil || apkcore.CompareVersions(p.Version, found.Version) > 0 {
				found = p
			}
		}
	}
	if found != nil {
		return found
	}
	// providers: a package may satisfy a name through provides.
	for _, idx := range indexes {
		for _, p := range idx.Packages {
			for _, prov := range p.Provides {
				if apkcore.EntryName(strings.TrimPrefix(prov, "!")) == name {
					if found == nil || p.ProviderPrio > found.ProviderPrio {
						found = p
					}
				}
			}
		}
	}
	return found
}

// replaceWorld puts spec in place of the existing entry for base (channel
// switch: "pkg@stable" becomes "pkg@testing").
func replaceWorld(world []string, base, spec string) []string {
	out := make([]string, 0, len(world))
	replaced := false
	for _, w := range world {
		if apkcore.EntryName(w) == base {
			if !replaced {
				out = append(out, spec)
				replaced = true
			}
			continue
		}
		out = append(out, w)
	}
	if !replaced {
		out = append(out, spec)
	}
	return out
}

// checkArch refuses a package built for another target. apk does the same; without
// it a Linux package installs into a Windows tree and the failure shows up much
// later (found 24.11.2026: arch = aarch64 was accepted on x86_64). "noarch" is
// the usual exception.
func checkArch(p *apkcore.Package, want string) error {
	if p.Arch == "" || p.Arch == "noarch" || want == "" || p.Arch == want {
		return nil
	}
	return fmt.Errorf("%s: package is for %s, this target is %s", p.FullName(), p.Arch, want)
}

// runStoredScript runs a deinstall script saved at install time, in our shell
// and with the same path rules as an install script.
func runStoredScript(root string, p *apkcore.Installed, kind string, stdout, stderr io.Writer) error {
	path := filepath.Join(apkcore.ScriptsDir(root), p.FullName()+kind)
	body, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	fmt.Fprintf(stdout, "  Executing %s%s\n", p.FullName(), kind)
	return apkscript.Run(apkscript.Script{Kind: kind, Body: body, Package: p.Name, Version: p.Version}, root, stdout, stderr)
}

// saveDeinstallScripts keeps .pre-deinstall/.post-deinstall for later, in
// /lib/apk/db/scripts of the target (see apkcore.ScriptsDir).
func saveDeinstallScripts(root string, p *apkcore.Package, scripts map[string][]byte) error {
	for _, kind := range []string{".pre-deinstall", ".post-deinstall"} {
		body, ok := scripts[kind]
		if !ok {
			continue
		}
		dir := apkcore.ScriptsDir(root)
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return err
		}
		path := filepath.Join(dir, p.FullName()+kind)
		if err := os.WriteFile(path, body, 0o755); err != nil {
			return err
		}
	}
	return nil
}

// splitTag splits "pkg@testing" into name and channel tag.
func splitTag(spec string) (string, string) {
	if i := strings.Index(spec, "@"); i > 0 {
		return spec[:i], spec[i+1:]
	}
	return spec, ""
}

// channelOf returns the tag a package was installed from, by looking at its
// world entry ("pkg@testing"). Empty means no channel.
func channelOf(world []string, name string) string {
	for _, w := range world {
		if apkcore.EntryName(w) == name {
			_, tag := splitTag(w)
			return tag
		}
	}
	return ""
}

// depPin returns the version of an "name=version" constraint, or "" when the
// constraint is absent or not an equality. Other operators are ticket acmesh-gk2i.
func depPin(spec string) string {
	if i := strings.Index(spec, "="); i > 0 {
		return strings.TrimPrefix(spec[i+1:], "=")
	}
	return ""
}

// exact finds the named package at exactly that version.
func exact(indexes []*apkcore.Index, name, version string) *apkcore.Package {
	return exactTagged(indexes, name, version, "")
}

func exactTagged(indexes []*apkcore.Index, name, version, tag string) *apkcore.Package {
	for _, idx := range indexes {
		if tag != "" && idx.Tag != tag {
			continue
		}
		for _, p := range idx.Packages {
			if p.Name == name && p.Version == version {
				return p
			}
		}
	}
	return nil
}

func update(cmd string, c *apkcore.Client, o options, args []string, stdout, stderr io.Writer) int {
	// apk update takes no packages; apk 3.0.9 ignores a path here and still
	// succeeds. We say what the path is instead of ignoring it silently, and
	// keep the exit code, so a script does not turn red for nothing.
	for _, a := range args {
		if strings.HasSuffix(a, ".apk") {
			fmt.Fprintf(stderr, "%s: %s is a local package, update does not need it (install it with apk add)\n", cmd, a)
		}
	}
	if len(o.repos) == 0 {
		if fromFile, err := readRepos(o.root); err == nil && len(fromFile) == 0 {
			// apk warns here, it does not fail; a path argument changes nothing.
			fmt.Fprintf(stderr, "%s: WARNING: no repositories configured\n", cmd)
			return 0
		}
	}
	repos := requireRepos(cmd, o, stderr, c)
	indexes, code := loadIndexes(cmd, c, options{root: o.root, allowUntr: o.allowUntr, verbose: o.verbose, noCache: true}, repos, stderr)
	if code != 0 {
		return code
	}
	seen := map[string]bool{}
	total := 0
	for _, idx := range indexes {
		for _, p := range idx.Packages {
			total++
			seen[p.Name] = true
		}
	}
	if !o.quiet {
		for _, repo := range repos {
			fmt.Fprintf(stdout, "%s/%s/APKINDEX.tar.gz\n", strings.TrimSuffix(repo.URL, "/"), o.arch)
		}
		fmt.Fprintf(stdout, "OK: %d distinct packages available\n", len(seen))
	}
	return 0
}

func add(cmd string, c *apkcore.Client, o options, names []string, stdout, stderr io.Writer) int {
	if len(names) == 0 {
		fmt.Fprintf(stderr, "%s: add: no packages given\n", cmd)
		return 1
	}

	// "apk add ./pkg.apk": a package that lies on disk needs no repository.
	localData := map[string][]byte{}
	var local []*apkcore.Package
	var wanted []string
	for _, spec := range names {
		if strings.HasSuffix(spec, ".apk") {
			if st, err := os.Stat(spec); err == nil && !st.IsDir() {
				data, err := os.ReadFile(spec)
				if err != nil {
					fmt.Fprintf(stderr, "%s: %v\n", cmd, err)
					return 1
				}
				keys, err := loadKeys(o.root)
				if err != nil {
					fmt.Fprintf(stderr, "%s: %v\n", cmd, err)
					return 1
				}
				p, trusted, err := apkcore.ParsePackageFile(data, spec, keys)
				if err != nil {
					fmt.Fprintf(stderr, "%s: %v\n", cmd, err)
					return 1
				}
				if !trusted && !o.allowUntr {
					fmt.Fprintf(stderr, "%s: %s: UNTRUSTED signature (use --allow-untrusted)\n", cmd, spec)
					return 1
				}
				if err := checkArch(p, o.arch); err != nil {
					fmt.Fprintf(stderr, "%s: %v\n", cmd, err)
					return 1
				}
				local = append(local, p)
				localData[p.Name] = data
				continue
			}
		}
		wanted = append(wanted, spec)
	}
	repos := requireRepos(cmd, o, stderr, c)
	indexes, code := loadIndexes(cmd, c, o, repos, stderr)
	if code != 0 && len(local) == 0 {
		return code
	}
	if code != 0 {
		// A local package without repositories: only its own files, no deps from
		// a repo. The dependency check below reports what is missing.
		indexes = nil
	}
	installed, err := apkcore.ReadInstalled(o.root)
	if err != nil {
		fmt.Fprintf(stderr, "%s: %v\n", cmd, err)
		return 1
	}
	have := map[string]bool{}
	installedVers := map[string]string{}
	for _, p := range installed {
		have[p.Name] = true
		installedVers[p.Name] = p.Version
	}

	// Resolve: requested names plus their depends, dependencies first.
	var order []*apkcore.Package
	seen := map[string]bool{}
	// resolve installs one dependency spec. "=" pins exactly that version; >=, <=,
	// >, < and ~ filter the newest match; a leading "!" is a conflict.
	var resolve func(d depSpec, top bool) int
	resolve = func(d depSpec, top bool) int {
		if d.conflict {
			if have[d.name] || seen[d.name] {
				fmt.Fprintf(stderr, "%s: %s must not be installed (%s conflicts with it)\n", cmd, d.name, d.by)
				return 1
			}
			return 0
		}
		if seen[d.name] {
			return 0
		}
		// An installed package is left alone unless the constraint asks for another
		// version or the repository has a newer one (that is what upgrade relies on).
		if have[d.name] {
			switch {
			case d.op == "":
				if newer := bestTagged(indexes, d.name, d.tag); newer == nil ||
					apkcore.CompareVersions(newer.Version, installedVers[d.name]) <= 0 {
					return 0
				}
			case satisfies(installedVers[d.name], d.op, d.ver):
				return 0
			}
		}
		var p *apkcore.Package
		if d.op == "=" {
			p = exactTagged(indexes, d.name, d.ver, d.tag)
		} else {
			p = bestSatisfying(indexes, d, d.tag)
		}
		if p != nil {
			if err := checkArch(p, o.arch); err != nil {
				fmt.Fprintf(stderr, "%s: %v\n", cmd, err)
				return 1
			}
		}
		if p == nil {
			switch {
			case d.op == "=":
				fmt.Fprintf(stderr, "%s: %s=%s (no such version)\n", cmd, d.name, d.ver)
			case d.op != "":
				fmt.Fprintf(stderr, "%s: %s (no package satisfies %s%s)%s\n", cmd, d.name, d.op, d.ver, d.wantedBy())
			case strings.HasPrefix(d.name, "so:") || strings.HasPrefix(d.name, "cmd:"):
				fmt.Fprintf(stderr, "%s: %s requires %s, which no repository provides\n", cmd, d.by, d.name)
			default:
				fmt.Fprintf(stderr, "%s: %s (no such package)\n", cmd, d.name)
			}
			return 1
		}
		if seen[p.Name] {
			return 0
		}
		seen[p.Name] = true
		for _, dep := range p.Depends {
			sub := parseDep(dep)
			sub.by = p.Name
			if code := resolve(sub, false); code != 0 {
				return code
			}
		}
		order = append(order, p)
		return 0
	}
	for _, lp := range local {
		if have[lp.Name] && installedVers[lp.Name] == lp.Version {
			continue
		}
		seen[lp.Name] = true
		for _, dep := range lp.Depends {
			sub := parseDep(dep)
			sub.by = lp.Name
			if code := resolve(sub, false); code != 0 {
				return code
			}
		}
		order = append(order, lp)
	}
	for _, spec := range wanted {
		d := parseDep(spec)
		d.by = "the command line"
		if code := resolve(d, true); code != 0 {
			return code
		}
	}
	if len(order) == 0 {
		if !o.quiet {
			fmt.Fprintf(stdout, "OK: nothing to do\n")
		}
		return 0
	}

	world, err := apkcore.ReadWorld(o.root)
	if err != nil {
		fmt.Fprintf(stderr, "%s: %v\n", cmd, err)
		return 1
	}
	worldSet := map[string]bool{}
	for _, w := range world {
		worldSet[apkcore.EntryName(w)] = true
	}

	var added int64
	// Triggers run at the end and only for the packages whose paths were
	// touched (acmesh-8h9j); the list of written files is what they are judged by.
	var newFiles []apkcore.InstalledFile
	for i, p := range order {
		if !o.quiet {
			// apk's wording: Installing, Upgrading, Downgrading - agents read it.
			switch {
			case have[p.Name] && apkcore.CompareVersions(p.Version, installedVers[p.Name]) < 0:
				fmt.Fprintf(stdout, "(%d/%d) Downgrading %s (%s -> %s)\n", i+1, len(order), p.Name, installedVers[p.Name], p.Version)
			case have[p.Name] && apkcore.CompareVersions(p.Version, installedVers[p.Name]) > 0:
				fmt.Fprintf(stdout, "(%d/%d) Upgrading %s (%s -> %s)\n", i+1, len(order), p.Name, installedVers[p.Name], p.Version)
			default:
				fmt.Fprintf(stdout, "(%d/%d) Installing %s (%s)\n", i+1, len(order), p.Name, p.Version)
			}
		}
		data, ok := localData[p.Name]
		if !ok {
			var err error
			data, err = c.PackageData(p)
			if err != nil {
				fmt.Fprintf(stderr, "%s: %v\n", cmd, err)
				return 1
			}
		}
		// Install scripts run through akmesh's own shell with every path mapped
		// into the target root (apkscript); --no-scripts keeps files only.
		scripts, err := apkcore.ScriptBodies(data, p)
		if err != nil {
			fmt.Fprintf(stderr, "%s: %v\n", cmd, err)
			return 1
		}
		// Removal happens without the package file, so its deinstall scripts are
		// stored in the tree now; apk re-downloads instead, which a tree without
		// network cannot.
		if err := saveDeinstallScripts(o.root, p, scripts); err != nil {
			fmt.Fprintf(stderr, "%s: %v\n", cmd, err)
			return 1
		}
		// A fresh install runs the install scripts, a replacement the upgrade ones.
		installKind, doneKind := ".pre-install", ".post-install"
		if have[p.Name] {
			installKind, doneKind = ".pre-upgrade", ".post-upgrade"
		}
		if !o.noScripts {
			for _, kind := range []string{installKind} {
				if body, ok := scripts[kind]; ok {
					if err := apkscript.Run(apkscript.Script{Kind: kind, Body: body, Package: p.Name, Version: p.Version}, o.root, stdout, stderr); err != nil {
						fmt.Fprintf(stderr, "%s: %s failed: %v\n", cmd, p.FullName(), err)
						return 1
					}
				}
			}
		}
		files, skippedDev, err := apkcore.Extract(data, p, apkcore.ExtractOptions{
			Root: o.root, NoScripts: true, Now: time.Now(),
		})
		if err != nil {
			fmt.Fprintf(stderr, "%s: %v\n", cmd, err)
			return 1
		}
		if len(skippedDev) > 0 {
			fmt.Fprintf(stderr, "%s: %s: %d device node(s) not created (%s); that needs root and mknod\n",
				cmd, p.FullName(), len(skippedDev), strings.Join(skippedDev, " "))
		}
		if !o.noScripts {
			if body, ok := scripts[doneKind]; ok {
				if !o.quiet {
					fmt.Fprintf(stdout, "  Executing %s%s\n", p.FullName(), doneKind)
				}
				if err := apkscript.Run(apkscript.Script{Kind: doneKind, Body: body, Package: p.Name, Version: p.Version}, o.root, stdout, stderr); err != nil {
					fmt.Fprintf(stderr, "%s: %s failed: %v\n", cmd, p.FullName(), err)
					return 1
				}
			}
			// A trigger belongs to its package and runs later, when another
			// package writes below its paths. The paths come from "triggers" in
			// .PKGINFO, the action from the control file .trigger - as in apk;
			// both are kept in our script directory because apk's installed
			// database has no key for them.
			if body, ok := scripts[".trigger"]; ok {
				if err := saveStoredScript(o.root, p.FullName(), ".trigger", body); err != nil {
					fmt.Fprintf(stderr, "%s: %v\n", cmd, err)
					return 1
				}
			}
			if len(p.Triggers) > 0 {
				paths := strings.Join(p.Triggers, "\n") + "\n"
				if err := saveStoredScript(o.root, p.FullName(), ".trigger-paths", []byte(paths)); err != nil {
					fmt.Fprintf(stderr, "%s: %v\n", cmd, err)
					return 1
				}
			}
		}
		newFiles = append(newFiles, files...)
		installed = append(installed, &apkcore.Installed{Package: *p, Files: files})
		added += p.Installed
	}
	// A local file is recorded by package name, not by its path.
	for _, lp := range local {
		if !worldSet[lp.Name] {
			world = append(world, lp.Name)
			worldSet[lp.Name] = true
		}
	}
	for _, spec := range wanted {
		base := apkcore.EntryName(spec)
		if worldSet[base] {
			// A channel switch replaces the world line, it does not add one.
			world = replaceWorld(world, base, spec)
			continue
		}
		world = append(world, spec)
		worldSet[base] = true
	}
	if !o.noScripts {
		if err := runTriggers(cmd, o, newFiles, stdout, stderr); err != nil {
			fmt.Fprintf(stderr, "%s: %v\n", cmd, err)
			return 1
		}
	}
	installed = applyReplaces(cmd, o, installed, stdout, stderr)
	if err := apkcore.WriteInstalled(o.root, installed); err != nil {
		fmt.Fprintf(stderr, "%s: %v\n", cmd, err)
		return 1
	}
	if err := apkcore.WriteWorld(o.root, world); err != nil {
		fmt.Fprintf(stderr, "%s: %v\n", cmd, err)
		return 1
	}
	if !o.quiet {
		fmt.Fprintf(stdout, "OK: %s in %s\n", human(added), plural(len(order), "package"))
	}
	return 0
}

// triggerPaths reads the paths a .trigger names, one per line, as apk writes
// them ("usr/share/icons/*" or a directory).
func triggerPaths(body []byte) []string {
	var out []string
	for _, line := range strings.Split(string(body), "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		out = append(out, line)
	}
	return out
}

// saveStoredScript writes a script next to the deinstall scripts, so a later
// run can find it without the package file (apk keeps its triggers the same way).
func saveStoredScript(root, fullName, kind string, body []byte) error {
	dir := apkcore.ScriptsDir(root)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(dir, fullName+kind), body, 0o755)
}

// runTriggers runs every stored .trigger whose paths cover a file this run
// wrote. apk behaves the same way: the trigger belongs to a package that is
// already installed, and a later install under its paths wakes it up.
func runTriggers(cmd string, o options, written []apkcore.InstalledFile, stdout, stderr io.Writer) error {
	if o.noScripts {
		return nil
	}
	dir := apkcore.ScriptsDir(o.root)
	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	for _, e := range entries {
		name := e.Name()
		if !strings.HasSuffix(name, ".trigger-paths") {
			continue
		}
		raw, err := os.ReadFile(filepath.Join(dir, name))
		if err != nil {
			return err
		}
		paths := triggerPaths(raw)
		if len(paths) == 0 || !triggerTouched(paths, written) {
			continue
		}
		owner := strings.TrimSuffix(name, ".trigger-paths")
		body, err := os.ReadFile(filepath.Join(dir, owner+".trigger"))
		if err != nil {
			// Paths without an action: nothing to run (a package may name paths
			// and rely on its own scripts; apk would have no script either).
			continue
		}
		if !o.quiet {
			fmt.Fprintf(stdout, "  Executing %s.trigger\n", owner)
		}
		pkg := apkcore.EntryName(owner)
		if err := apkscript.Run(apkscript.Script{Kind: ".trigger", Body: body, Package: pkg}, o.root, stdout, stderr); err != nil {
			return fmt.Errorf("%s.trigger failed: %v", owner, err)
		}
	}
	return nil
}

// triggerTouched reports whether one of the written files lies at or below a
// trigger path.
func triggerTouched(paths []string, written []apkcore.InstalledFile) bool {
	for _, f := range written {
		p := "/" + strings.TrimPrefix(f.Dir+"/"+f.Base, "/")
		p = strings.ReplaceAll(p, "//", "/")
		for _, t := range paths {
			t = "/" + strings.TrimPrefix(strings.TrimSuffix(t, "/"), "/")
			if p == t || strings.HasPrefix(p, t+"/") {
				return true
			}
		}
	}
	return false
}

// applyReplaces removes the packages a newly installed package replaces, the
// way apk does: the replaced package disappears, its files stay only where the
// new package ships them too (acmesh-8h9j).
func applyReplaces(cmd string, o options, installed []*apkcore.Installed, stdout, stderr io.Writer) []*apkcore.Installed {
	newest := map[string]*apkcore.Installed{}
	for _, p := range installed {
		newest[p.Name] = p
	}
	// Which new package replaces which name.
	replaceBy := map[string]*apkcore.Installed{}
	for _, p := range installed {
		for _, rep := range p.Replaces {
			name := apkcore.EntryName(rep)
			if name != "" && name != p.Name {
				replaceBy[name] = p
			}
		}
	}
	if len(replaceBy) == 0 {
		return installed
	}
	keep := installed[:0]
	for _, old := range installed {
		np := replaceBy[old.Name]
		if np == nil || np.Name == old.Name {
			keep = append(keep, old)
			continue
		}
		shipped := map[string]bool{}
		for _, f := range np.Files {
			shipped[f.Dir+"/"+f.Base] = true
		}
		var gone []apkcore.InstalledFile
		for _, f := range old.Files {
			if !shipped[f.Dir+"/"+f.Base] {
				gone = append(gone, f)
			}
		}
		removed, keptCount, err := apkcore.RemoveFiles(o.root, gone)
		if err != nil {
			fmt.Fprintf(stderr, "%s: replacing %s: %v\n", cmd, old.Name, err)
		}
		if !o.quiet {
			fmt.Fprintf(stdout, "  Replacing %s (%s) with %s (%s removed, %s kept)\n",
				old.Name, old.Version, np.Name, plural(removed, "file"), plural(keptCount, "file"))
		}
	}
	return keep
}

func del(cmd string, c *apkcore.Client, o options, names []string, stdout, stderr io.Writer) int {
	if len(names) == 0 {
		fmt.Fprintf(stderr, "%s: del: no packages given\n", cmd)
		return 1
	}
	installed, err := apkcore.ReadInstalled(o.root)
	if err != nil {
		fmt.Fprintf(stderr, "%s: %v\n", cmd, err)
		return 1
	}
	byName := map[string]*apkcore.Installed{}
	for _, p := range installed {
		byName[p.Name] = p
	}
	// Refuse to remove something another installed package depends on.
	for _, name := range names {
		if _, ok := byName[name]; !ok {
			fmt.Fprintf(stderr, "%s: %s: not installed\n", cmd, name)
			return 1
		}
		for _, other := range installed {
			if other.Name == name {
				continue
			}
			for _, dep := range other.Depends {
				if apkcore.EntryName(strings.TrimPrefix(dep, "!")) == name {
					fmt.Fprintf(stderr, "%s: %s is required by %s\n", cmd, name, other.Name)
					return 1
				}
			}
		}
	}
	var freed int64
	i := 0
	for _, name := range names {
		p := byName[name]
		if !o.noScripts {
			if err := runStoredScript(o.root, p, ".pre-deinstall", stdout, stderr); err != nil {
				fmt.Fprintf(stderr, "%s: %s failed: %v\n", cmd, p.FullName(), err)
				return 1
			}
		}
		if _, kept, err := apkcore.RemoveFiles(o.root, p.Files); err != nil {
			fmt.Fprintf(stderr, "%s: %v\n", cmd, err)
			return 1
		} else if kept > 0 && o.verbose {
			fmt.Fprintf(stderr, "%s: %s: %d directories kept (not empty)\n", cmd, name, kept)
		}
		freed += p.Installed
		i++
		if !o.quiet {
			fmt.Fprintf(stdout, "(%d/%d) Purging %s (%s)\n", i, len(names), p.Name, p.Version)
		}
		if !o.noScripts {
			if err := runStoredScript(o.root, p, ".post-deinstall", stdout, stderr); err != nil {
				fmt.Fprintf(stderr, "%s: %s failed: %v\n", cmd, p.FullName(), err)
				return 1
			}
		}
		for _, kind := range []string{".pre-deinstall", ".post-deinstall"} {
			_ = os.Remove(filepath.Join(apkcore.ScriptsDir(o.root), p.FullName()+kind))
		}
	}
	// Drop from the database and from the world file.
	remove := map[string]bool{}
	for _, name := range names {
		remove[name] = true
	}
	var keep []*apkcore.Installed
	for _, p := range installed {
		if !remove[p.Name] {
			keep = append(keep, p)
		}
	}
	if err := apkcore.WriteInstalled(o.root, keep); err != nil {
		fmt.Fprintf(stderr, "%s: %v\n", cmd, err)
		return 1
	}
	world, err := apkcore.ReadWorld(o.root)
	if err != nil {
		fmt.Fprintf(stderr, "%s: %v\n", cmd, err)
		return 1
	}
	var keptWorld []string
	for _, w := range world {
		if !remove[apkcore.EntryName(w)] {
			keptWorld = append(keptWorld, w)
		}
	}
	if err := apkcore.WriteWorld(o.root, keptWorld); err != nil {
		fmt.Fprintf(stderr, "%s: %v\n", cmd, err)
		return 1
	}
	if !o.quiet {
		fmt.Fprintf(stdout, "OK: %s freed\n", human(freed))
	}
	return 0
}

func upgrade(cmd string, c *apkcore.Client, o options, args []string, stdout, stderr io.Writer) int {
	// "apk upgrade paket" (and a local "./paket.apk") upgrades just that: add
	// does the comparison, the wording and the channel, so it is reused. Real
	// apk refuses a file path here ("Package '/p.apk' not found", apk 3.0.9);
	// accepting it is an extension, not a contradiction.
	if len(args) > 0 {
		return add(cmd, c, o, args, stdout, stderr)
	}
	repos := requireRepos(cmd, o, stderr, c)
	indexes, code := loadIndexes(cmd, c, o, repos, stderr)
	if code != 0 {
		return code
	}
	installed, err := apkcore.ReadInstalled(o.root)
	if err != nil {
		fmt.Fprintf(stderr, "%s: %v\n", cmd, err)
		return 1
	}
	world, err := apkcore.ReadWorld(o.root)
	if err != nil {
		fmt.Fprintf(stderr, "%s: %v\n", cmd, err)
		return 1
	}
	// An upgrade stays inside the channel the package was installed from: a pin
	// to @testing is not moved to stable (or the other way) by apk upgrade.
	var names []string
	for _, p := range installed {
		tag := channelOf(world, p.Name)
		newer := bestTagged(indexes, p.Name, tag)
		if newer != nil && apkcore.CompareVersions(newer.Version, p.Version) > 0 {
			if tag != "" {
				names = append(names, p.Name+"@"+tag)
			} else {
				names = append(names, p.Name)
			}
		}
	}
	if len(names) == 0 {
		if !o.quiet {
			fmt.Fprintf(stdout, "OK: nothing to upgrade\n")
		}
		return 0
	}
	// Reuse add: it replaces the files and rewrites the entry.
	return add(cmd, c, o, names, stdout, stderr)
}

func info(cmd string, o options, names []string, stdout, stderr io.Writer) int {
	installed, err := apkcore.ReadInstalled(o.root)
	if err != nil {
		fmt.Fprintf(stderr, "%s: %v\n", cmd, err)
		return 1
	}
	if len(names) == 0 {
		var list []string
		for _, p := range installed {
			list = append(list, p.Name)
		}
		sort.Strings(list)
		for _, n := range list {
			fmt.Fprintln(stdout, n)
		}
		return 0
	}
	code := 0
	for _, name := range names {
		var p *apkcore.Installed
		for _, ip := range installed {
			if ip.Name == name {
				p = ip
			}
		}
		if p == nil {
			fmt.Fprintf(stderr, "%s: %s: not installed\n", cmd, name)
			code = 1
			continue
		}
		fmt.Fprintf(stdout, "%s-%s description:\n%s\n", p.Name, p.Version, p.Description)
		fmt.Fprintf(stdout, "%s-%s webpage:\n%s\n", p.Name, p.Version, p.URL)
		fmt.Fprintf(stdout, "%s-%s installed size:\n%d\n", p.Name, p.Version, p.Installed)
		if len(p.Depends) > 0 {
			fmt.Fprintf(stdout, "%s-%s depends on:\n%s\n", p.Name, p.Version, strings.Join(p.Depends, " "))
		}
	}
	return code
}

func list(cmd string, c *apkcore.Client, o options, patterns []string, stdout, stderr io.Writer) int {
	installed, err := apkcore.ReadInstalled(o.root)
	if err != nil {
		fmt.Fprintf(stderr, "%s: %v\n", cmd, err)
		return 1
	}
	var lines []string
	seen := map[string]bool{}
	for _, p := range installed {
		if matchAny(patterns, p.Name, p.Description) {
			lines = append(lines, fmt.Sprintf("%s-%s %s installed", p.Name, p.Version, p.Arch))
			seen[p.Name] = true
		}
	}
	if len(o.repos) > 0 || len(patterns) > 0 {
		var repos []Repo
		for _, u := range o.repos {
			repos = append(repos, Repo{URL: u})
		}
		if len(repos) == 0 {
			repos, _ = readRepos(o.root)
		}
		indexes, code := loadIndexes(cmd, c, o, repos, stderr)
		if code != 0 {
			return code
		}
		for _, idx := range indexes {
			for _, p := range idx.Packages {
				if seen[p.Name] || !matchAny(patterns, p.Name, p.Description) {
					continue
				}
				lines = append(lines, fmt.Sprintf("%s-%s %s %s", p.Name, p.Version, p.Arch, idx.Repo))
			}
		}
	}
	sort.Strings(lines)
	for _, l := range lines {
		fmt.Fprintln(stdout, l)
	}
	return 0
}

func search(cmd string, c *apkcore.Client, words []string, stdout, stderr io.Writer) int {
	if len(words) == 0 {
		fmt.Fprintf(stderr, "%s: search: no pattern given\n", cmd)
		return 1
	}
	repos, _ := readRepos("")
	if len(repos) == 0 {
		fmt.Fprintf(stderr, "%s: search needs repositories (-X or /etc/apk/repositories)\n", cmd)
		return 1
	}
	o := options{root: "", allowUntr: false}
	indexes, code := loadIndexes(cmd, c, o, repos, stderr)
	if code != 0 {
		return code
	}
	for _, idx := range indexes {
		for _, p := range idx.Packages {
			if matchAny(words, p.Name, p.Description) {
				fmt.Fprintf(stdout, "%s-%s - %s\n", p.Name, p.Version, p.Description)
			}
		}
	}
	return 0
}

// matchAny is apk's list filter: a plain substring test over name and text.
func matchAny(patterns []string, name, text string) bool {
	if len(patterns) == 0 {
		return true
	}
	for _, pat := range patterns {
		for _, part := range strings.Split(pat, ",") {
			part = strings.TrimSpace(part)
			if part == "" {
				continue
			}
			if strings.Contains(name, part) || strings.Contains(text, part) {
				return true
			}
		}
	}
	return false
}

// human prints sizes the way apk does: bytes under 1 KiB, then KiB and MiB.
func human(n int64) string {
	switch {
	case n < 1024:
		return strconv.FormatInt(n, 10) + " B"
	case n < 1024*1024:
		return fmt.Sprintf("%.1f KiB", float64(n)/1024)
	default:
		return fmt.Sprintf("%.1f MiB", float64(n)/(1024*1024))
	}
}

func plural(n int, word string) string {
	if n == 1 {
		return "1 " + word
	}
	return strconv.Itoa(n) + " " + word + "s"
}
