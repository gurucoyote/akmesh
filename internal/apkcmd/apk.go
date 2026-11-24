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
		return update(cmd, c, o, stdout, stderr)
	case "add":
		return add(cmd, c, o, operands, stdout, stderr)
	case "del", "remove":
		return del(cmd, c, o, operands, stdout, stderr)
	case "upgrade":
		return upgrade(cmd, c, o, stdout, stderr)
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
	repos := o.repos
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
	c.Repos = repos
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
func readRepos(root string) ([]string, error) {
	data, err := os.ReadFile(apkcore.RepoFilePath(root))
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	var out []string
	for _, line := range strings.Split(string(data), "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		out = append(out, line)
	}
	return out, nil
}

// loadIndexes fetches every repository index and refuses unsigned ones unless
// the caller said --allow-untrusted.
// requireRepos complains only for the subcommands that really need a
// repository; info and del work on an installed tree alone.
func requireRepos(cmd string, o options, stderr io.Writer, c *apkcore.Client) []string {
	repos := o.repos
	if len(repos) == 0 {
		repos = c.Repos
	}
	if len(repos) == 0 {
		fmt.Fprintf(stderr, "%s: no repositories configured (use -X or %s)\n", cmd, apkcore.RepoFilePath(o.root))
	}
	return repos
}

func loadIndexes(cmd string, c *apkcore.Client, o options, repos []string, stderr io.Writer) ([]*apkcore.Index, int) {
	var out []*apkcore.Index
	for _, repo := range repos {
		idx, err := c.Index(repo)
		if err != nil {
			fmt.Fprintf(stderr, "%s: %v\n", cmd, err)
			return nil, 1
		}
		if (idx.Unsigned || idx.Untrusted) && !o.allowUntr {
			fmt.Fprintf(stderr, "%s: %s: UNTRUSTED signature\n", cmd, repo)
			return nil, 1
		}
		if o.verbose {
			fmt.Fprintf(stderr, "%s: %s: %d packages%s\n", cmd, repo, len(idx.Packages), sigNote(idx))
		}
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
func best(indexes []*apkcore.Index, name string) *apkcore.Package {
	var found *apkcore.Package
	for _, idx := range indexes {
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

func update(cmd string, c *apkcore.Client, o options, stdout, stderr io.Writer) int {
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
			fmt.Fprintf(stdout, "%s/%s/APKINDEX.tar.gz\n", strings.TrimSuffix(repo, "/"), o.arch)
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
	have := map[string]bool{}
	for _, p := range installed {
		have[p.Name] = true
	}

	// Resolve: requested names plus their depends, dependencies first.
	var order []*apkcore.Package
	seen := map[string]bool{}
	var resolve func(name string, top bool) int
	resolve = func(name string, top bool) int {
		if have[name] || seen[name] {
			return 0
		}
		p := best(indexes, name)
		if p == nil {
			fmt.Fprintf(stderr, "%s: %s (no such package)\n", cmd, name)
			return 1
		}
		if seen[p.Name] {
			return 0
		}
		seen[p.Name] = true
		for _, dep := range p.Depends {
			dep = strings.TrimPrefix(dep, "!")
			if strings.HasPrefix(dep, "so:") || strings.HasPrefix(dep, "cmd:") {
				// Virtual names: satisfied by provides, looked up the same way.
				if provider := best(indexes, dep); provider != nil {
					if code := resolve(provider.Name, false); code != 0 {
						return code
					}
					continue
				}
				fmt.Fprintf(stderr, "%s: %s requires %s, which no repository provides\n", cmd, p.Name, dep)
				return 1
			}
			if code := resolve(apkcore.EntryName(dep), false); code != 0 {
				return code
			}
		}
		order = append(order, p)
		return 0
	}
	for _, name := range names {
		if code := resolve(apkcore.EntryName(name), true); code != 0 {
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
	for i, p := range order {
		data, err := c.PackageData(p)
		if err != nil {
			fmt.Fprintf(stderr, "%s: %v\n", cmd, err)
			return 1
		}
		files, err := apkcore.Extract(data, p, apkcore.ExtractOptions{
			Root: o.root, NoScripts: o.noScripts, Now: time.Now(),
		})
		if err != nil {
			fmt.Fprintf(stderr, "%s: %v\n", cmd, err)
			return 1
		}
		installed = append(installed, &apkcore.Installed{Package: *p, Files: files})
		if !o.quiet {
			fmt.Fprintf(stdout, "(%d/%d) Installing %s (%s)\n", i+1, len(order), p.Name, p.Version)
		}
		added += p.Installed
	}
	for _, name := range names {
		base := apkcore.EntryName(name)
		if !worldSet[base] {
			world = append(world, name)
			worldSet[base] = true
		}
	}
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

func upgrade(cmd string, c *apkcore.Client, o options, stdout, stderr io.Writer) int {
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
	var names []string
	for _, p := range installed {
		newer := best(indexes, p.Name)
		if newer != nil && apkcore.CompareVersions(newer.Version, p.Version) > 0 {
			names = append(names, p.Name)
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
		repos := o.repos
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
