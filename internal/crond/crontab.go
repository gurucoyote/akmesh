// Package crond runs time jobs the way a container expects it: the environment
// of the crontab is inherited, output goes to stdout and stderr, SIGTERM stops
// it cleanly and SIGHUP reloads the files (acmesh-qgf).
//
// Why our own: u-root has no cron at all, and supercronic - which has exactly
// this behaviour - costs 704 KB as a library and binds its packages to logrus
// and a Prometheus registry. The expressions come from robfig/cron v3 (MIT, no
// transitive dependencies, measured +184 KB); the crontab file, the environment,
// the reload and the running of jobs are ours.
//
// The crontab format is Vixie/BusyBox with the one addition containers want:
// a seconds field is allowed as the first field. Both of these work:
//
//	*/5 * * * *   echo five minutes
//	@every 10s    echo every ten seconds
//
// and the descriptors @hourly, @daily, @weekly, @monthly, @yearly, @every DURATION
// and @reboot. There is no seconds field: see splitSpec for why.
package crond

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// Job is one line of a crontab.
type Job struct {
	Spec    string // the timing, 5 or 6 fields, or a @descriptor
	Command string
	Line    int
	Source  string // file it came from, "command line" for -c
	Env     []string
}

// Crontab is a parsed file: the jobs and the variables they run with.
type Crontab struct {
	Jobs []Job
	Env  []string
}

// ParseCrontab reads one crontab. Variable lines are KEY=value (no spaces
// around =), comments start with "#", a trailing comment after a command stays
// part of the command - Vixie does the same, and dropping it would silently
// change what the job does.
func ParseCrontab(r io.Reader, source string) (*Crontab, error) {
	tab := &Crontab{}
	scanner := bufio.NewScanner(r)
	scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)
	line := 0
	for scanner.Scan() {
		line++
		text := strings.TrimSpace(scanner.Text())
		if text == "" || strings.HasPrefix(text, "#") {
			continue
		}
		// A variable: NAME=value, the name without spaces or <>
		if eq := strings.Index(text, "="); eq > 0 && !strings.ContainsAny(text[:eq], " \t") {
			name := text[:eq]
			if isVarName(name) {
				value := strings.TrimSpace(text[eq+1:])
				value = strings.Trim(value, `"'`)
				entry := name + "=" + value
				tab.Env = append(tab.Env, entry)
				continue
			}
		}
		spec, command, ok := splitSpec(text)
		if !ok {
			return nil, fmt.Errorf("%s line %d: not a crontab line: %q", source, line, text)
		}
		if command == "" {
			return nil, fmt.Errorf("%s line %d: no command after the time", source, line)
		}
		tab.Jobs = append(tab.Jobs, Job{Spec: spec, Command: command, Line: line, Source: source})
	}
	if err := scanner.Err(); err != nil {
		return nil, err
	}
	for i := range tab.Jobs {
		tab.Jobs[i].Env = tab.Env
	}
	return tab, nil
}

// isVarName reports whether s looks like a shell variable name.
func isVarName(s string) bool {
	if s == "" {
		return false
	}
	for i, c := range s {
		switch {
		case c >= 'a' && c <= 'z', c >= 'A' && c <= 'Z', c == '_':
		case c >= '0' && c <= '9' && i > 0:
		default:
			return false
		}
	}
	return true
}

// splitSpec cuts a line into its time part and its command. A @descriptor is the
// whole time; otherwise the first five fields are the time and the rest is the
// command, exactly like Vixie and BusyBox.
//
// There is no seconds field: a sixth time field cannot be told apart from a
// command that starts with a number ("0 0 * * * 5 cmd"), and guessing would
// silently move jobs. Sub-minute schedules use "@every 10s" instead, which the
// expression parser understands and which reads as what it does.
func splitSpec(text string) (spec, command string, ok bool) {
	if strings.HasPrefix(text, "@") {
		// @every carries its duration: "@every 10s cmd" is spec + command.
		if after, ok0 := strings.CutPrefix(text, "@every "); ok0 {
			fields := strings.SplitN(after, " ", 2)
			if len(fields) < 2 {
				return "", "", false
			}
			return "@every " + fields[0], strings.TrimSpace(fields[1]), true
		}
		fields := strings.SplitN(text, " ", 2)
		if len(fields) < 2 {
			return "", "", false
		}
		return fields[0], strings.TrimSpace(fields[1]), true
	}
	fields := strings.Fields(text)
	if len(fields) < 6 {
		return "", "", false
	}
	return strings.Join(fields[:5], " "), strings.Join(fields[5:], " "), true
}

// ReadFiles reads and merges crontabs in order; later files override variables
// from earlier ones, as Vixie does.
func ReadFiles(paths []string) (*Crontab, error) {
	out := &Crontab{}
	envIndex := map[string]int{}
	for _, path := range paths {
		f, err := os.Open(path)
		if err != nil {
			if os.IsNotExist(err) {
				continue
			}
			return nil, err
		}
		tab, err := ParseCrontab(f, path)
		f.Close()
		if err != nil {
			return nil, err
		}
		for _, kv := range tab.Env {
			name := kv[:strings.Index(kv, "=")]
			if i, ok := envIndex[name]; ok {
				out.Env[i] = kv
				continue
			}
			envIndex[name] = len(out.Env)
			out.Env = append(out.Env, kv)
		}
		out.Jobs = append(out.Jobs, tab.Jobs...)
	}
	for i := range out.Jobs {
		out.Jobs[i].Env = out.Env
	}
	sort.SliceStable(out.Jobs, func(i, j int) bool {
		if out.Jobs[i].Source != out.Jobs[j].Source {
			return out.Jobs[i].Source < out.Jobs[j].Source
		}
		return out.Jobs[i].Line < out.Jobs[j].Line
	})
	return out, nil
}

// CronDir lists sorted the files of a /etc/cron.d style directory, skipping
// dotfiles and backups the way cron does.
func CronDir(dir string) []string {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil
	}
	var out []string
	for _, e := range entries {
		name := e.Name()
		if e.IsDir() || strings.HasPrefix(name, ".") || strings.HasSuffix(name, "~") {
			continue
		}
		out = append(out, filepath.Join(dir, name))
	}
	sort.Strings(out)
	return out
}
