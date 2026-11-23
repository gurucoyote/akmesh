package proc

import (
	"fmt"
	"os"
	"os/user"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"
)

// pgrepOpts is what pgrep and pkill share: the search, not the action.
type pgrepOpts struct {
	full    bool   // -f: match the whole command line, not the name
	exact   bool   // -x: the whole name has to match, not a part
	user    string // -u: only processes of this user
	ppid    int    // -P: only children of this pid
	newest  bool   // -n: only the youngest match
	oldest  bool   // -o: only the oldest match
	showPID bool   // -l/-a imply the pid is shown as well
	name    bool   // -l: print the name too
	args    bool   // -a: print the full command line
	count   bool   // -c: print the number of matches
	delim   string // -d: what separates two pids
	pattern string
}

// parsePgrepArgs reads the flags pgrep and pkill understand. allowSignal lets
// pkill take a leading -SIGNAL the way kill does; pgrep has no signal.
func parsePgrepArgs(args []string, allowSignal bool) (pgrepOpts, string, error) {
	o := pgrepOpts{delim: "\n"}
	sig := "TERM"
	var rest []string
	for i := 0; i < len(args); i++ {
		a := args[i]
		switch {
		case a == "-f" || a == "--full":
			o.full = true
		case a == "-x" || a == "--exact":
			o.exact = true
		case a == "-n" || a == "--newest":
			o.newest = true
		case a == "-o" || a == "--oldest":
			o.oldest = true
		case a == "-l" || a == "--list-name":
			o.name = true
		case a == "-a" || a == "--list-full":
			o.args = true
		case a == "-c" || a == "--count":
			o.count = true
		case a == "-u" || a == "--euid":
			if i+1 >= len(args) {
				return o, sig, fmt.Errorf("%s needs a user", a)
			}
			i++
			o.user = args[i]
		case a == "-P" || a == "--parent":
			if i+1 >= len(args) {
				return o, sig, fmt.Errorf("%s needs a pid", a)
			}
			i++
			n, err := strconv.Atoi(args[i])
			if err != nil {
				return o, sig, fmt.Errorf("%s: %q is not a pid", a, args[i])
			}
			o.ppid = n
		case a == "-d" || a == "--delimiter":
			if i+1 >= len(args) {
				return o, sig, fmt.Errorf("%s needs a separator", a)
			}
			i++
			o.delim = args[i]
		case a == "-s" || a == "--signal":
			if !allowSignal {
				return o, sig, fmt.Errorf("%s: unknown option", a)
			}
			if i+1 >= len(args) {
				return o, sig, fmt.Errorf("%s needs a signal", a)
			}
			i++
			sig = args[i]
		case a == "--":
			rest = append(rest, args[i+1:]...)
			i = len(args)
		case len(a) > 1 && a[0] == '-' && allowSignal:
			// pkill takes -9 and -TERM like kill; a lone -f is a flag above.
			sig = strings.TrimLeft(a, "-")
		case len(a) > 1 && a[0] == '-':
			return o, sig, fmt.Errorf("%q: unknown option", a)
		default:
			rest = append(rest, a)
		}
	}
	if len(rest) > 1 {
		return o, sig, fmt.Errorf("exactly one pattern is needed")
	}
	if len(rest) == 0 {
		// "pgrep -P 123" and "pgrep -u root" are how agents ask for a list, so
		// a missing pattern means "everything" - but only when a filter says
		// what is meant. A bare "pgrep" stays a usage error.
		if o.user == "" && o.ppid == 0 {
			return o, sig, fmt.Errorf("no pattern given")
		}
		return o, sig, nil
	}
	o.pattern = rest[0]
	return o, sig, nil
}

// pgrepSelect keeps the processes that match. It is a plain function over a
// list so that the matching can be tested without starting anything.
//
// The pattern is a regular expression, as in procps; -x demands that it covers
// the whole text, which is what makes "pgrep -x sleep" safe where plain
// "pgrep sleep" would also catch "mysleep".
func pgrepSelect(procs []procInfo, o pgrepOpts) []procInfo {
	var out []procInfo
	for _, p := range procs {
		if o.user != "" && !sameUser(p.User, o.user) {
			continue
		}
		if o.ppid != 0 && p.PPID != o.ppid {
			continue
		}
		// Windows names the image akmesh.exe, while an agent types "pgrep -x
		// akmesh": compare the name with and without the suffix, as killall
		// does. The command line is never trimmed, it carries arguments.
		texts := []string{p.Comm}
		if t := trimExe(p.Comm); t != p.Comm {
			texts = append(texts, t)
		}
		if o.full {
			texts = []string{p.command()}
		}
		matched := false
		for _, text := range texts {
			if text != "" && matchText(text, o) {
				matched = true
				break
			}
		}
		if !matched {
			continue
		}
		out = append(out, p)
	}
	if (o.newest || o.oldest) && len(out) > 1 {
		sort.SliceStable(out, func(i, j int) bool {
			if out[i].Started.Equal(out[j].Started) {
				return out[i].PID < out[j].PID
			}
			return out[i].Started.Before(out[j].Started)
		})
		if o.newest {
			out = out[len(out)-1:]
		} else {
			out = out[:1]
		}
	}
	return out
}

// matchText applies the pattern, with -x meaning "covers everything".
func matchText(text string, o pgrepOpts) bool {
	if o.pattern == "" {
		return true
	}
	expr := o.pattern
	if o.exact {
		expr = "^(?:" + expr + ")$"
	}
	re, err := regexp.Compile(expr)
	if err != nil {
		// A pattern that is no valid regular expression is tried literally, so
		// that a plain name with a dot in it still works.
		if o.exact {
			return strings.EqualFold(text, o.pattern)
		}
		return strings.Contains(strings.ToLower(text), strings.ToLower(o.pattern))
	}
	return re.MatchString(text)
}

// resolveUser turns a numeric uid into the name the process list carries, so
// that "pgrep -u 0" and "pgrep -u root" mean the same thing. A number that
// cannot be resolved stays as it is.
func resolveUser(want string) string {
	uid, err := strconv.Atoi(want)
	if err != nil {
		return want
	}
	u, err := user.LookupId(strconv.Itoa(uid))
	if err != nil || u.Username == "" {
		return want
	}
	// Some systems append the domain: DOMAIN\name or name@domain.
	name := u.Username
	if i := strings.LastIndexAny(name, "\\@"); i >= 0 && i+1 < len(name) {
		name = name[i+1:]
	}
	return name
}

// sameUser compares a process owner against -u, which may be a name or a
// number; Windows reports SIDs and no name for some processes, so a plain
// string compare is the last resort.
func sameUser(have, want string) bool {
	if strings.EqualFold(have, want) {
		return true
	}
	return strings.EqualFold(trimExe(have), trimExe(want))
}

// runPgrep prints the pids of the processes that match, one per line.
func runPgrep(args []string, stdin *os.File, stdout, stderr *os.File) int {
	o, _, err := parsePgrepArgs(args, false)
	o.user = resolveUser(o.user)
	if err != nil {
		fmt.Fprintf(stderr, "pgrep: %v\nusage: pgrep [-flxancv] [-u USER] [-P PPID] [-d SEP] PATTERN\n", err)
		return 2
	}
	procs, err := listProcesses()
	if err != nil {
		fmt.Fprintf(stderr, "pgrep: %v\n", err)
		return 1
	}
	found := pgrepSelect(procs, o)
	if len(found) == 0 {
		return 1
	}
	if o.count {
		fmt.Fprintf(stdout, "%d\n", len(found))
		return 0
	}
	var b strings.Builder
	for i, p := range found {
		if i > 0 {
			b.WriteString(o.delim)
		}
		b.WriteString(strconv.Itoa(p.PID))
		if o.name {
			b.WriteString(" ")
			b.WriteString(p.Comm)
		}
		if o.args {
			b.WriteString(" ")
			b.WriteString(p.command())
		}
	}
	b.WriteString("\n")
	fmt.Fprint(stdout, b.String())
	return 0
}

// runPkill sends a signal to every process that matches. The calling process is
// never signalled: "pkill -f akmesh" from an akmesh shell must not kill that
// shell.
func runPkill(args []string, stdin *os.File, stdout, stderr *os.File) int {
	o, sigName, err := parsePgrepArgs(args, true)
	o.user = resolveUser(o.user)
	if err != nil {
		fmt.Fprintf(stderr, "pkill: %v\nusage: pkill [-SIGNAL] [-f] [-x] [-u USER] [-P PPID] PATTERN\n", err)
		return 2
	}
	sig, err := normaliseSignal(sigName)
	if err != nil {
		fmt.Fprintf(stderr, "pkill: %v\n", err)
		return 2
	}
	procs, err := listProcesses()
	if err != nil {
		fmt.Fprintf(stderr, "pkill: %v\n", err)
		return 1
	}
	killed := 0
	failed := false
	for _, p := range pkillTargets(procs, o, os.Getpid()) {
		if err := signalProcess(p.PID, sig); err != nil {
			fmt.Fprintf(stderr, "pkill: %v\n", err)
			failed = true
			continue
		}
		killed++
	}
	if killed == 0 {
		return 1
	}
	if failed {
		return 1
	}
	return 0
}

// pkillTargets is what pkill signals: every match except the caller itself.
// A separate function so that the exclusion can be tested without sending a
// signal to anything - the first version of that test really did run
// "pkill -f .*" and took the whole process group down.
func pkillTargets(procs []procInfo, o pgrepOpts, self int) []procInfo {
	var out []procInfo
	for _, p := range pgrepSelect(procs, o) {
		if p.PID == self {
			continue
		}
		out = append(out, p)
	}
	return out
}

// waitGone waits until no process with this pid is left, so that killall -w
// means what a script expects. A process that does not react before the limit
// is reported instead of waiting forever: an agent must not hang.
func waitGone(pid int, limit time.Duration) bool {
	deadline := time.Now().Add(limit)
	for time.Now().Before(deadline) {
		procs, err := listProcesses()
		if err != nil {
			return false
		}
		alive := false
		for _, p := range procs {
			if p.PID == pid && !isZombie(p) {
				alive = true
				break
			}
		}
		if !alive {
			return true
		}
		time.Sleep(20 * time.Millisecond)
	}
	return false
}

// isZombie reports whether the process only waits to be reaped, which is as
// dead as it gets for the caller.
func isZombie(p procInfo) bool {
	return strings.HasPrefix(p.Stat, "Z")
}
