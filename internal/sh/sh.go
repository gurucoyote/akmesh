package sh

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"golang.org/x/term"

	"mvdan.cc/sh/v3/expand"
	"mvdan.cc/sh/v3/interp"
	"mvdan.cc/sh/v3/syntax"
)

// The startup files. They are variables so tests can point them at a temporary
// directory instead of changing /etc/profile on the machine that runs them.
var (
	etcProfile = "/etc/profile"
	homeEnv    = os.Getenv
)

// invocation is what the arguments asked for, before anything runs.
type invocation struct {
	command     string
	scriptFile  string
	positional  []string
	interactive bool
	login       bool
	showHelp    bool
}

// Run runs the shell that the policy applies to. It is mvdan's shell, so the
// language is POSIX-ish bash, but every file it opens, every program it starts
// and every child those programs start lives inside the policy the process
// applied before this function ran.
func Run(args []string, stdin io.Reader, stdout, stderr io.Writer) int {
	return RunAs(os.Args[0], args, stdin, stdout, stderr)
}

// RunAs is Run with an explicit argv[0]. A name that starts with a dash ("-sh")
// is the classic way of asking for a login shell, and the tests need it.
func RunAs(argv0 string, args []string, stdin io.Reader, stdout, stderr io.Writer) int {
	inv, ok := parseArgs(args, stdout, stderr)
	if !ok {
		return 2
	}
	if inv.showHelp {
		usage(stdout)
		return 0
	}
	if inv.command == "" && inv.scriptFile == "" && !inv.interactive && isTerminal(stdin) {
		inv.interactive = true
	}
	base := filepath.Base(argv0)
	login := inv.login || strings.HasPrefix(base, "-")
	bash := base == "bash" || base == "-bash"

	out := &switchWriter{w: stdout}
	// The shell gets the session's input itself, not a wrapper: interp wraps any
	// non-file reader in a pipe fed by io.Copy, and that copier reads ahead and
	// steals the lines the session was supposed to type (acmesh-51b). A *os.File
	// is used as it is, so the builtin `read` reads from the same place the
	// line loop does.
	shellIn := io.Reader(strings.NewReader(""))
	if _, isFile := stdin.(*os.File); isFile {
		// Only a file can be shared: anything else is wrapped in a pipe fed by
		// a copier that reads ahead.
		shellIn = stdin
	}
	if inv.command == "" && inv.scriptFile == "" && !inv.interactive {
		// The script is stdin and is read below, so the shell must not read it.
		shellIn = strings.NewReader("")
	}
	env := os.Environ()
	// $0 is the name the shell was called with; ash marks a login shell by a
	// leading dash. mvdan/sh takes $0 from the environment variable "0", which
	// the dispatcher never sets, so it read "gosh" (acmesh-51b).
	shellName := base
	if login && !strings.HasPrefix(shellName, "-") {
		shellName = "-" + shellName
	}
	env = append(env, "0="+shellName)
	if inv.interactive {
		// The parent process may export its own PS1; a shell must not inherit
		// it. Start from the ash default, and let the profile override it.
		env = withoutVar(env, "PS1")
		env = append(env, "PS1="+defaultPrompt())
	}
	// The tracer writes "+ " where ash writes PS4; the wrapper rewrites that
	// while xtrace is on (acmesh-89s9).
	var curRunner *interp.Runner
	traceErr := newPS4Writer(stderr, func() *interp.Runner { return curRunner })
	opts := []interp.RunnerOption{
		interp.StdIO(shellIn, out, traceErr),
		interp.Dir(workingDir()),
		interp.Env(expand.ListEnviron(env...)),
	}
	if len(inv.positional) > 0 {
		opts = append(opts, interp.Params(inv.positional...))
	}
	if inv.interactive {
		// Interactive makes the shell behave like one: it keeps the last exit
		// status, does not stop at the first error and knows PS1.
		opts = append(opts, interp.Interactive(true))
	}
	// Two builtins are answered here, in-process, because mvdan/sh lists them
	// and then reports "unsupported builtin": umask (a builtin in ash and in
	// every /etc/profile) and kill (acmesh-oxw). CallHandler holds one
	// function, so they are chained.
	opts = append(opts, interp.CallHandler(chainCall(killCall(out, stderr), umaskCall(out, stderr))))
	runner, err := interp.New(opts...)
	curRunner = runner
	if err != nil {
		fmt.Fprintf(stderr, "sh: %v\n", err)
		return 1
	}

	// Startup files run in the same runner as the session, so what they export
	// is what the session sees.
	for _, path := range startupFiles(login, bash, inv.interactive) {
		if code, stop := runStartup(runner, path, stderr); stop {
			return code
		}
	}

	switch {
	case inv.command != "":
		return finish(runner, runSource(runner, inv.command, "<command>"), stderr)
	case inv.scriptFile != "" && inv.scriptFile != "-":
		data, err := os.ReadFile(inv.scriptFile)
		if err != nil {
			fmt.Fprintf(stderr, "sh: %v\n", err)
			return 127
		}
		return finish(runner, runSource(runner, string(data), inv.scriptFile), stderr)
	case inv.interactive:
		// Ctrl-C must end the line, not the shell (acmesh-43qq).
		it := newInterrupter()
		defer it.stop()
		// A cancelled statement leaves the runner marked as exiting, so the
		// session needs a fresh one after a Ctrl-C. It carries the variables,
		// the directory and the positional parameters; shell functions are
		// lost, which is the price of interrupting a builtin loop.
		rebuild := func(old *interp.Runner) (*interp.Runner, error) {
			env := make([]string, 0, len(old.Vars))
			for name, v := range old.Vars {
				if !v.Set || v.Kind == expand.Indexed || v.Kind == expand.Associative {
					continue
				}
				env = append(env, name+"="+v.Str)
			}
			opts := []interp.RunnerOption{
				interp.StdIO(shellIn, out, traceErr),
				interp.Dir(old.Dir),
				interp.Env(expand.ListEnviron(env...)),
				interp.Interactive(true),
				interp.CallHandler(chainCall(killCall(out, stderr), umaskCall(out, stderr))),
			}
			if len(old.Params) > 0 {
				opts = append(opts, interp.Params(old.Params...))
			}
			return interp.New(opts...)
		}
		return interactiveLoop(runner, stdin, out, stderr, it, rebuild)
	default:
		data, err := io.ReadAll(stdin)
		if err != nil {
			fmt.Fprintf(stderr, "sh: %v\n", err)
			return 1
		}
		return finish(runner, runSource(runner, string(data), "<stdin>"), stderr)
	}
}

// chainCall runs the handlers in order until one of them rewrites the command.
// A handler that answers a builtin returns another command ("true" or "false"),
// so the next handler sees that and leaves it alone.
func chainCall(handlers ...interp.CallHandlerFunc) interp.CallHandlerFunc {
	return func(ctx context.Context, args []string) ([]string, error) {
		for _, h := range handlers {
			next, err := h(ctx, args)
			if err != nil {
				return next, err
			}
			args = next
		}
		return args, nil
	}
}

// parseArgs understands the flags a login shell is called with. Anything else
// stays an error: a shell that swallows every typo hides what went wrong.
func parseArgs(args []string, stdout, stderr io.Writer) (invocation, bool) {
	var inv invocation
	for i := range args {
		a := args[i]
		switch {
		case a == "-c":
			if i+1 >= len(args) {
				fmt.Fprintln(stderr, "sh: -c needs a command string")
				return inv, false
			}
			inv.command = args[i+1]
			inv.positional = args[i+2:]
			return inv, true
		case a == "-i":
			inv.interactive = true
		case a == "-l" || a == "--login":
			inv.login = true
		case a == "--":
			rest := args[i+1:]
			if len(rest) > 0 && inv.scriptFile == "" {
				inv.scriptFile = rest[0]
				inv.positional = append(inv.positional, rest[1:]...)
			} else {
				inv.positional = append(inv.positional, rest...)
			}
			return inv, true
		case a == "--help" || a == "-h":
			inv.showHelp = true
			return inv, true
		case strings.HasPrefix(a, "-") && a != "-":
			fmt.Fprintf(stderr, "sh: unsupported option %q\n", a)
			return inv, false
		default:
			if inv.scriptFile == "" {
				inv.scriptFile = a
				continue
			}
			inv.positional = append(inv.positional, a)
		}
	}
	return inv, true
}

// usage answers --help. It also carries the one decision a reader will ask
// about: which startup file this shell reads.
func usage(w io.Writer) {
	fmt.Fprintln(w, "usage: sh [-c COMMAND] [-i] [-l|--login] [SCRIPT [ARGS...]]")
	fmt.Fprintln(w, "runs mvdan/sh inside the policy of this process")
	fmt.Fprintln(w)
	fmt.Fprintln(w, "  -c COMMAND   run COMMAND and exit")
	fmt.Fprintln(w, "  -i           interactive: read one statement per line")
	fmt.Fprintln(w, "  -l, --login  login shell: read /etc/profile, then the user profile")
	fmt.Fprintln(w, "               (.profile; .bash_profile only when called as bash)")
	fmt.Fprintln(w, "               $0 is -sh then; the dispatcher cannot call the shell as -sh,")
	fmt.Fprintln(w, "               so -l/--login is the way to a login shell")
}

// startupFiles is the list of files a shell reads before its first command, in
// order. A login shell reads /etc/profile and then the user's file; an
// interactive shell that is not a login shell reads $ENV, as POSIX says. A
// plain script call reads nothing at all.
func startupFiles(login, bash, interactive bool) []string {
	var files []string
	if login {
		files = append(files, etcProfile)
		if home := homeEnv("HOME"); home != "" {
			name := ".profile"
			if bash {
				// bash's own convention; reading both would run the setup twice.
				name = ".bash_profile"
			}
			files = append(files, filepath.Join(home, name))
		}
		return files
	}
	if interactive {
		if env := homeEnv("ENV"); env != "" {
			files = append(files, env)
		}
	}
	return files
}

// runStartup runs one startup file. A missing file is not an error (a container
// has no /etc/profile), an unreadable one is reported but does not end the
// session, and "exit" in a profile really ends it.
func runStartup(r *interp.Runner, path string, stderr io.Writer) (int, bool) {
	data, err := os.ReadFile(path)
	if err != nil {
		if !os.IsNotExist(err) {
			fmt.Fprintf(stderr, "sh: %v\n", err)
		}
		return 0, false
	}
	err = runSource(r, string(data), path)
	if r.Exited() {
		return exitCode(err), true
	}
	if err != nil {
		fmt.Fprintf(stderr, "sh: %v\n", err)
	}
	return 0, false
}

// interactiveLoop reads statements until EOF, prints the prompt before each
// one and keeps the session alive after a failing command. It reads lines
// itself instead of using the parser's sequence API: a test with a plain
// strings.Reader has to behave exactly like a terminal.
func interactiveLoop(r *interp.Runner, stdin io.Reader, out *switchWriter, stderr io.Writer, it *interrupter, rebuild func(*interp.Runner) (*interp.Runner, error)) int {
	// One byte at a time: a buffered reader would read past the line, and a
	// command like `read` would find the input already eaten (acmesh-51b).
	reader := &lineReader{r: stdin}
	var pending strings.Builder
	// At a real terminal the line editor takes over: cursor keys, history and
	// Ctrl-C work while typing. A pipe or a test reader keeps the plain reader,
	// so nothing about the non-interactive behaviour changes.
	f, editing := useEditor(stdin)
	var ed *editor
	if editing {
		ed = newEditor(f, out)
	} else {
		out.WriteString(promptString(r, out))
	}
	for {
		var line string
		var readErr error
		if ed != nil {
			line, readErr = ed.readLine(promptString(r, out))
			if readErr == errLineCancelled {
				pending.Reset()
				continue
			}
		} else {
			line, readErr = reader.readLine()
		}
		if readErr != nil && it != nil && it.consume() {
			// Ctrl-C while typing: the terminal threw the line away, so start
			// a fresh one instead of ending the session.
			pending.Reset()
			out.WriteString("\n")
			out.WriteString(promptString(r, out))
			continue
		}
		if line == "" && readErr != nil {
			return 0
		}
		pending.WriteString(line)
		src := pending.String()
		file, err := syntax.NewParser().Parse(strings.NewReader(src), "<stdin>")
		if err != nil {
			if syntax.IsIncomplete(err) {
				if ed != nil {
					// The editor prints its own prompt on the next call: PS2, the
					// continuation prompt, not the main one (acmesh-89s9).
					if readErr != nil {
						return 0
					}
					pending.WriteString("\n")
					ed.setPrompt(promptString2(r, out))
					continue
				}
				out.WriteString(promptString2(r, out))
				if readErr != nil {
					return 0
				}
				continue
			}
			fmt.Fprintf(stderr, "sh: %v\n", err)
			pending.Reset()
			out.WriteString(promptString(r, out))
			if readErr != nil {
				return 0
			}
			continue
		}
		pending.Reset()
		ctx, done := context.Background(), func() bool { return false }
		if it != nil {
			ctx, done = it.begin()
		}
		runErr := runStatements(ctx, r, file)
		interrupted := done()
		if interrupted && r.Exited() && rebuild != nil {
			fresh, err := rebuild(r)
			if err != nil {
				fmt.Fprintf(stderr, "sh: %v\n", err)
				return 1
			}
			r = fresh
		}
		if r.Exited() {
			return exitCode(runErr)
		}
		if interrupted || interruptedBySignal(runErr) {
			// A newline, then a new prompt: the cancelled statement is not an
			// error to report, it is what the user asked for. $? is 130, the
			// status ash leaves after a Ctrl-C.
			if file, err := syntax.NewParser().Parse(strings.NewReader("(exit 130)"), "<interrupt>"); err == nil {
				_ = runStatements(context.Background(), r, file)
			}
			out.WriteString("\n")
		} else if runErr != nil {
			fmt.Fprintf(stderr, "sh: %v\n", runErr)
		}
		out.WriteString(promptString(r, out))
		if readErr != nil {
			return 0
		}
	}
}

// interruptedBySignal reports whether a command ended because the terminal
// sent it SIGINT. ash prints nothing there, only a new prompt.
func interruptedBySignal(err error) bool {
	status, ok := interp.IsExitStatus(err)
	return ok && status == 130
}

// promptString is PS1 when the profile set one, otherwise the ash default. The
// value is expanded by the shell itself, so a profile can use $USER or $PWD.
// promptString2 is PS2, the prompt of a continued line (ash default "> ").
func promptString2(r *interp.Runner, out *switchWriter) string {
	var buf strings.Builder
	prevOut := out.swap(&buf)
	_ = runSource(r, `printf '%s' "$PS2"`, "<prompt>")
	out.swap(prevOut)
	if s := buf.String(); s != "" {
		return expandPrompt(s)
	}
	return "> "
}

func promptString(r *interp.Runner, out *switchWriter) string {
	var buf strings.Builder
	prevOut := out.swap(&buf)
	// printf writes to the buffer and reads no input, so the shell may keep the
	// session's reader here.
	_ = runSource(r, `printf '%s' "$PS1"`, "<prompt>")
	out.swap(prevOut)
	if s := buf.String(); s != "" {
		return expandPrompt(s)
	}
	return defaultPrompt()
}

// expandPrompt turns the PS1 escapes ash understands into text. Without this a
// profile that sets PS1 to "\[..\]\u@\h:\w\$" shows that string literally.
func expandPrompt(p string) string {
	var b strings.Builder
	for i := 0; i < len(p); i++ {
		c := p[i]
		if c == '\\' && i+1 < len(p) {
			i++
			switch p[i] {
			case 'u':
				b.WriteString(envOr("USER", "root"))
			case 'h':
				h, _ := os.Hostname()
				if k := strings.IndexByte(h, '.'); k > 0 {
					h = h[:k]
				}
				b.WriteString(h)
			case 'w':
				wd, _ := os.Getwd()
				home := envOr("HOME", "")
				if home != "" && strings.HasPrefix(wd, home) {
					wd = "~" + wd[len(home):]
				}
				b.WriteString(wd)
			case 'W':
				wd, _ := os.Getwd()
				if k := strings.LastIndexByte(wd, '/'); k >= 0 && k+1 < len(wd) {
					wd = wd[k+1:]
				}
				b.WriteString(wd)
			case '$':
				if os.Geteuid() == 0 {
					b.WriteByte('#')
				} else {
					b.WriteByte('$')
				}
			case 'n':
				b.WriteByte('\n')
			case '\\':
				b.WriteByte('\\')
			case '[', ']':
				// Readline markers: they carry no text.
			default:
				b.WriteByte(p[i])
			}
			continue
		}
		b.WriteByte(c)
	}
	return b.String()
}

func envOr(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

// defaultPrompt is what ash prints when no profile set PS1.
func defaultPrompt() string {
	if os.Geteuid() == 0 {
		return "# "
	}
	return "$ "
}

// withoutVar returns the environment with every definition of name removed.
func withoutVar(env []string, name string) []string {
	prefix := name + "="
	out := env[:0:0]
	for _, kv := range env {
		if !strings.HasPrefix(kv, prefix) {
			out = append(out, kv)
		}
	}
	return out
}

// runSource parses and runs one chunk of shell code.
func runSource(r *interp.Runner, src, name string) error {
	file, err := syntax.NewParser().Parse(strings.NewReader(src), name)
	if err != nil {
		return err
	}
	return runStatements(context.Background(), r, file)
}

// runStatements runs one statement at a time. Passing a whole *syntax.File to
// Runner.Run implies an exit - the runner is then spent, and the next line of
// an interactive session would do nothing. Statements keep the session alive,
// and the variables, options and directory with it.
func runStatements(ctx context.Context, r *interp.Runner, file *syntax.File) error {
	var last error
	for _, stmt := range file.Stmts {
		last = r.Run(ctx, stmt)
		if r.Exited() {
			return last
		}
		if last != nil {
			if _, ok := interp.IsExitStatus(last); !ok {
				return last
			}
		}
	}
	return last
}

// finish turns the last error into the process exit code.
func finish(_ *interp.Runner, err error, stderr io.Writer) int {
	if err == nil {
		return 0
	}
	if status, ok := interp.IsExitStatus(err); ok {
		return int(status)
	}
	fmt.Fprintf(stderr, "sh: %v\n", err)
	return 1
}

func exitCode(err error) int {
	if status, ok := interp.IsExitStatus(err); ok {
		return int(status)
	}
	return 0
}

func workingDir() string {
	dir, err := os.Getwd()
	if err != nil {
		return "/"
	}
	return dir
}

// isTerminal decides whether a call without a command is interactive: a shell
// on a terminal waits for input, a shell on a pipe reads it to the end.
func isTerminal(r io.Reader) bool {
	f, ok := r.(*os.File)
	if !ok {
		return false
	}
	return term.IsTerminal(int(f.Fd()))
}

// switchWriter lets the shell's own output go somewhere else for a moment. The
// prompt is expanded by the shell, and that needs the same runner without
// leaking the output into the session.
type switchWriter struct {
	mu sync.Mutex
	w  io.Writer
}

func (s *switchWriter) Write(p []byte) (int, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.w.Write(p)
}

func (s *switchWriter) WriteString(str string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	_, _ = io.WriteString(s.w, str)
}

func (s *switchWriter) swap(w io.Writer) io.Writer {
	s.mu.Lock()
	defer s.mu.Unlock()
	prev := s.w
	s.w = w
	return prev
}

// lineReader reads one line at a time and never past the newline, so the shell
// and the session can share one input without either stealing from the other.
type lineReader struct {
	r io.Reader
}

func (l *lineReader) readLine() (string, error) {
	var b strings.Builder
	var one [1]byte
	for {
		n, err := l.r.Read(one[:])
		if n > 0 {
			if one[0] == '\n' {
				return b.String(), nil
			}
			b.WriteByte(one[0])
		}
		if err != nil {
			if b.Len() > 0 {
				return b.String(), nil
			}
			return "", err
		}
	}
}

// switchReader does the same for input: the prompt needs the shell to run
// without touching the session's stdin.
type switchReader struct {
	mu sync.Mutex
	r  io.Reader
}

func (s *switchReader) Read(p []byte) (int, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.r.Read(p)
}

func (s *switchReader) swap(r io.Reader) io.Reader {
	s.mu.Lock()
	defer s.mu.Unlock()
	prev := s.r
	s.r = r
	return prev
}
