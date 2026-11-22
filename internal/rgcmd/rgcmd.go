// Package rgcmd is akmesh's rg: a ripgrep-shaped search over the files a
// harness has, built on gripgrep's library facade instead of its 1945-line
// command line.
//
// Two differences to ripgrep are on purpose and are stated in the help text:
// PCRE2, --type-add/--type-clear and --encoding are not built, and the global
// gitignore file (core.excludesFile) is not read; per-directory .gitignore,
// .ignore and .rgignore are.
package rgcmd

import (
	"fmt"
	"io"
	"os"
	"sync"
	"sync/atomic"

	"git.site.quack-lab.dev/dave/gripgrep/filetype"
	"git.site.quack-lab.dev/dave/gripgrep/glob"
	"git.site.quack-lab.dev/dave/gripgrep/match"
	"git.site.quack-lab.dev/dave/gripgrep/printer"
	"git.site.quack-lab.dev/dave/gripgrep/search"
	"git.site.quack-lab.dev/dave/gripgrep/walk"
)

// Run is the whole command: parse, search, print, exit code. Exit codes are
// rg's: 0 at least one match, 1 no match, 2 any error (bad pattern, unreadable
// file, bad flag).
func Run(args []string, stdin io.Reader, stdout, stderr io.Writer) (code int) {
	// grepgrep pools scratch state and has panic sites ported from ripgrep's
	// unreachable!(); a fuzz run over the public builder found none of them
	// reachable (see acmesh-55u), so this recover is a last resort, not a
	// safety net with a proof. It also only covers this goroutine: a panic in
	// one of gripgrep's own walk workers still ends the process.
	defer func() {
		if r := recover(); r != nil {
			fmt.Fprintf(stderr, "rg: internal error: %v\n", r)
			code = 2
		}
	}()

	cfg, err := parse(args)
	if err != nil {
		fmt.Fprintf(stderr, "rg: %v\n", err)
		return 2
	}

	globSet, requireMatch, err := buildGlobs(cfg)
	if err != nil {
		fmt.Fprintf(stderr, "rg: %v\n", err)
		return 2
	}
	typesMatcher, err := buildTypes(cfg)
	if err != nil {
		fmt.Fprintf(stderr, "rg: %v\n", err)
		return 2
	}

	// --files needs no pattern and no matcher: it is a walk, not a search.
	if cfg.files {
		return listFiles(cfg, globSet, requireMatch, typesMatcher, stdout, stderr)
	}

	matcher, err := buildMatcher(cfg)
	if err != nil {
		fmt.Fprintf(stderr, "rg: %v\n", err)
		return 2
	}

	// No PATH and a redirected stdin: rg reads the stream, not the current
	// directory. A terminal stdin keeps rg's default of searching ".".
	if len(cfg.paths) == 0 && !stdinIsTerminal(stdin) {
		return searchStdin(cfg, matcher, stdin, stdout, stderr)
	}

	return searchPaths(cfg, matcher, globSet, requireMatch, typesMatcher, stdout, stderr)
}

func buildMatcher(cfg config) (match.Matcher, error) {
	cm := match.CaseSensitive
	switch {
	case cfg.smartCase:
		cm = match.CaseSmart
	case cfg.ignoreCase:
		cm = match.CaseInsensitive
	}
	return match.New(match.Config{
		Patterns:   cfg.patterns,
		CaseMode:   cm,
		Word:       cfg.word,
		Fixed:      cfg.fixed,
		LineRegexp: cfg.lineRegexp,
	})
}

// buildGlobs turns -g/--iglob into the walk's override set. The polarity is
// ripgrep's: a plain pattern is an include filter, one with '!' an exclude.
func buildGlobs(cfg config) (*glob.Set, bool, error) {
	if len(cfg.globs) == 0 && len(cfg.iglobs) == 0 {
		return nil, false, nil
	}
	var b glob.Builder
	requireMatch := false
	addOne := func(p string, ci bool) {
		stripped, negated := cutPrefix(p, "!")
		add := b.Add
		if ci {
			add = b.AddCI
		}
		if negated {
			add(stripped)
			return
		}
		requireMatch = true
		add("!" + p)
	}
	for _, p := range cfg.globs {
		addOne(p, false)
	}
	for _, p := range cfg.iglobs {
		addOne(p, true)
	}
	set, err := b.Build()
	if err != nil {
		return nil, false, fmt.Errorf("invalid glob: %w", err)
	}
	return set, requireMatch, nil
}

func cutPrefix(s, prefix string) (string, bool) {
	if len(s) >= len(prefix) && s[:len(prefix)] == prefix {
		return s[len(prefix):], true
	}
	return s, false
}

// walkOptions is the file-set half of the search: which files exist at all.
// It is built here once and used both by --files and by the search.
func walkOptions(cfg config, globSet *glob.Set, requireMatch bool, typesMatcher *filetype.Matcher) walk.Options {
	opts := walk.Options{
		Hidden:            cfg.hidden,
		NoIgnoreDot:       cfg.noIgnore,
		NoIgnoreExclude:   cfg.noIgnore,
		NoIgnoreParent:    cfg.noIgnore,
		NoIgnoreVcs:       cfg.noIgnore,
		Threads:           cfg.workers,
		Globs:             globSet,
		GlobsRequireMatch: requireMatch,
		Types:             typesMatcher,
	}
	if cfg.maxDepth > 0 {
		d := cfg.maxDepth
		opts.MaxDepth = &d
	}
	return opts
}

func stdinIsTerminal(stdin io.Reader) bool {
	f, ok := stdin.(*os.File)
	if !ok {
		return false
	}
	info, err := f.Stat()
	if err != nil {
		return false
	}
	return info.Mode()&os.ModeCharDevice != 0
}

// sinkFor builds the output side of one worker. Every sink is single
// goroutine, like search.Searcher, so each pooled worker owns its own.
func sinkFor(cfg config, matcher match.Matcher, dest *printer.Dest, showPath bool) (search.Sink, bool) {
	switch {
	case cfg.filesWithMatches:
		p := printer.NewFilesWithMatches(dest)
		return p, false
	case cfg.count:
		p := printer.NewCount(dest)
		p.ShowPath = showPath
		p.OnlyMatching = cfg.onlyMatching
		p.Matcher = matcher
		return p, false
	default:
		p := printer.NewStandard(dest)
		p.Matcher = matcher
		p.ShowPath = showPath
		p.OnlyMatching = cfg.onlyMatching
		p.ContextEnabled = cfg.before > 0 || cfg.after > 0
		return p, true
	}
}

type worker struct {
	searcher *search.Searcher
	sink     search.Sink
	standard bool
}

func newWorkerFactory(cfg config, matcher match.Matcher, dest *printer.Dest, showPath bool) func() *worker {
	return func() *worker {
		sink, standard := sinkFor(cfg, matcher, dest, showPath)
		return &worker{
			searcher: search.New(search.Searcher{
				Matcher:       matcher,
				Invert:        cfg.invert,
				LineNumbers:   cfg.lineNumbers,
				BeforeContext: cfg.before,
				AfterContext:  cfg.after,
				MaxCount:      intPtr(cfg.maxCount),
			}),
			sink:     sink,
			standard: standard,
		}
	}
}

func intPtr(n int) *int {
	if n == 0 {
		return nil
	}
	return &n
}

// searchPaths walks the PATH arguments and searches every regular file.
func searchPaths(cfg config, matcher match.Matcher, globSet *glob.Set, requireMatch bool, typesMatcher *filetype.Matcher, stdout, stderr io.Writer) int {
	showPath, missing := wantPaths(cfg)
	if missing != "" {
		fmt.Fprintf(stderr, "rg: %s: No such file or directory\n", missing)
		return 2
	}
	if cfg.noFilename {
		showPath = false
	}

	dest := printer.NewDest(stdout)
	newWorker := newWorkerFactory(cfg, matcher, dest, showPath)
	var pool sync.Pool
	pool.New = func() any { return newWorker() }

	var anyMatched, anyErr atomic.Bool
	reportErr := func(path string, err error) {
		fmt.Fprintf(stderr, "rg: %s: %s\n", path, err)
		anyErr.Store(true)
	}

	roots := cfg.paths
	visit := func(e *walk.Entry) walk.WalkState {
		if e.Err != nil {
			reportErr(e.Path, e.Err)
			return walk.Continue
		}
		if e.Type != walk.TypeFile {
			return walk.Continue
		}
		w := pool.Get().(*worker)
		defer pool.Put(w)

		// A file named on the command line is read as text (rg's
		// BinaryConvert); a discovered one stops at its first NUL.
		w.searcher.BinaryMode = search.BinaryQuit
		if e.Depth == 0 {
			w.searcher.BinaryMode = search.BinaryConvert
		}

		tracked := &binaryTracker{
			Sink:           w.sink,
			matched:        &anyMatched,
			standard:       w.standard,
			binMode:        w.searcher.BinaryMode,
			showPath:       showPath,
			contextEnabled: cfg.before > 0 || cfg.after > 0,
			dest:           dest,
			searcher:       w.searcher,
		}

		f, err := os.Open(e.Path)
		if err != nil {
			reportErr(e.Path, err)
			return walk.Continue
		}
		defer f.Close()
		if err := w.searcher.Search(e.Path, stripBOM(f), tracked); err != nil {
			reportErr(e.Path, err)
		}
		return walk.Continue
	}

	if err := walk.Walk(roots, walkOptions(cfg, globSet, requireMatch, typesMatcher), visit); err != nil {
		fmt.Fprintf(stderr, "rg: %v\n", err)
		return 2
	}
	if anyErr.Load() {
		return 2
	}
	if anyMatched.Load() {
		return 0
	}
	return 1
}

// searchStdin searches the piped stream. The label "<stdin>" never appears:
// with one input and no path, rg prints bare lines.
func searchStdin(cfg config, matcher match.Matcher, stdin io.Reader, stdout, stderr io.Writer) int {
	dest := printer.NewDest(stdout)
	sink, standard := sinkFor(cfg, matcher, dest, false)
	s := search.New(search.Searcher{
		Matcher:       matcher,
		Invert:        cfg.invert,
		LineNumbers:   cfg.lineNumbers,
		BeforeContext: cfg.before,
		AfterContext:  cfg.after,
		MaxCount:      intPtr(cfg.maxCount),
	})
	var matched atomic.Bool
	tracked := &binaryTracker{
		Sink:           sink,
		matched:        &matched,
		standard:       standard,
		binMode:        search.BinaryNone,
		contextEnabled: cfg.before > 0 || cfg.after > 0,
		dest:           dest,
		searcher:       s,
	}
	if err := s.Search("<stdin>", stdin, tracked); err != nil {
		fmt.Fprintf(stderr, "rg: %v\n", err)
		return 2
	}
	if matched.Load() {
		return 0
	}
	return 1
}

// listFiles is --files: the walked file set, no search.
func listFiles(cfg config, globSet *glob.Set, requireMatch bool, typesMatcher *filetype.Matcher, stdout, stderr io.Writer) int {
	roots := cfg.paths
	if len(roots) == 0 {
		roots = []string{""}
	}
	var mu sync.Mutex
	found := 0
	var anyErr atomic.Bool
	visit := func(e *walk.Entry) walk.WalkState {
		if e.Err != nil {
			fmt.Fprintf(stderr, "rg: %s: %s\n", e.Path, e.Err)
			anyErr.Store(true)
			return walk.Continue
		}
		if e.Type != walk.TypeFile {
			return walk.Continue
		}
		mu.Lock()
		fmt.Fprintln(stdout, e.Path)
		found++
		mu.Unlock()
		return walk.Continue
	}
	if err := walk.Walk(roots, walkOptions(cfg, globSet, requireMatch, typesMatcher), visit); err != nil {
		fmt.Fprintf(stderr, "rg: %v\n", err)
		return 2
	}
	if anyErr.Load() {
		return 2
	}
	if found > 0 {
		return 0
	}
	return 1
}

// wantPaths decides whether output carries the path prefix, like rg: one
// explicitly named file gets bare lines, everything else (several paths, a
// directory, the default ".") gets paths. missing is the PATH that cannot be
// stat'ed, so the caller can report it instead of searching nothing.
func wantPaths(cfg config) (show bool, missing string) {
	if len(cfg.paths) == 0 {
		return true, ""
	}
	if len(cfg.paths) > 1 {
		return true, ""
	}
	info, err := os.Stat(cfg.paths[0])
	if err != nil {
		return false, cfg.paths[0]
	}
	return !info.Mode().IsRegular(), ""
}
