package tools

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"
)

// procInfo is what one platform can tell about one process. A field a platform
// does not know stays empty and is printed as "-", so the column layout is the
// same everywhere.
type procInfo struct {
	PID     int
	PPID    int
	User    string
	Stat    string
	CPUTime time.Duration
	Elapsed time.Duration
	TTY     string
	Comm    string
	CmdLine string
}

// psField is one column of the output.
type psField struct {
	name   string // name as typed in -eo
	header string
	right  bool // numbers are right aligned, text is left aligned
	value  func(p procInfo) string
}

var psFields = []psField{
	{"pid", "PID", true, func(p procInfo) string { return strconv.Itoa(p.PID) }},
	{"ppid", "PPID", true, func(p procInfo) string { return strconv.Itoa(p.PPID) }},
	{"user", "USER", false, func(p procInfo) string { return orDash(p.User) }},
	{"uid", "UID", false, func(p procInfo) string { return orDash(p.User) }},
	{"stat", "STAT", false, func(p procInfo) string { return orDash(p.Stat) }},
	{"time", "TIME", true, func(p procInfo) string { return formatCPUTime(p.CPUTime) }},
	{"etime", "ELAPSED", true, func(p procInfo) string { return formatElapsed(p.Elapsed) }},
	{"tty", "TTY", false, func(p procInfo) string { return orDash(p.TTY) }},
	{"comm", "COMM", false, func(p procInfo) string { return orDash(p.Comm) }},
	{"cmd", "CMD", false, func(p procInfo) string { return orDash(p.command()) }},
}

func (p procInfo) command() string {
	if p.CmdLine != "" {
		return p.CmdLine
	}
	return p.Comm
}

func orDash(s string) string {
	if s == "" {
		return "-"
	}
	return s
}

// formatCPUTime prints a duration the way ps does: MM:SS or HH:MM:SS.
func formatCPUTime(d time.Duration) string {
	if d < 0 {
		return "-"
	}
	total := int64(d / time.Second)
	h := total / 3600
	m := (total % 3600) / 60
	s := total % 60
	if h > 0 {
		return fmt.Sprintf("%02d:%02d:%02d", h, m, s)
	}
	return fmt.Sprintf("%02d:%02d", m, s)
}

// formatElapsed prints [[DD-]HH:]MM:SS.
func formatElapsed(d time.Duration) string {
	if d < 0 {
		return "-"
	}
	total := int64(d / time.Second)
	days := total / 86400
	total %= 86400
	h := total / 3600
	m := (total % 3600) / 60
	s := total % 60
	switch {
	case days > 0:
		return fmt.Sprintf("%d-%02d:%02d:%02d", days, h, m, s)
	case h > 0:
		return fmt.Sprintf("%02d:%02d:%02d", h, m, s)
	}
	return fmt.Sprintf("%02d:%02d", m, s)
}

// psRequest is the parsed command line.
type psRequest struct {
	all      bool
	full     bool // -f: user, ppid and start column
	bsd      bool // aux: the BSD layout agents parse
	fields   []psField
	pidOnly  []int
	hasPID   bool
	defaults bool
}

// columnsFor picks the columns for a request, the way procps does it.
func (r psRequest) columnsFor() []psField {
	if len(r.fields) > 0 {
		return r.fields
	}
	byName := func(names ...string) []psField {
		var out []psField
		for _, n := range names {
			if f, ok := psFieldByName(n); ok {
				out = append(out, f)
			}
		}
		return out
	}
	switch {
	case r.bsd:
		// USER PID %CPU %MEM VSZ RSS TTY STAT START TIME COMMAND is what aux
		// normally prints. We print the columns we can fill and say so in the
		// docs instead of inventing numbers for the rest.
		return byName("user", "pid", "stat", "tty", "time", "cmd")
	case r.full:
		return byName("user", "pid", "ppid", "stat", "etime", "tty", "time", "cmd")
	}
	return byName("pid", "tty", "time", "cmd")
}

func psFieldByName(name string) (psField, bool) {
	clean := strings.ToLower(strings.TrimSpace(name))
	switch clean {
	case "command":
		clean = "cmd"
	case "comm", "comm=":
		clean = "comm"
	case "args":
		clean = "cmd"
	}
	for _, f := range psFields {
		if f.name == clean {
			return f, true
		}
	}
	return psField{}, false
}

// parsePS understands the invocations agents actually type: ps, ps -e, ps -f,
// ps -ef, ps aux, ps -eo pid,ppid,cmd, ps -p 1234.
func parsePS(args []string) (psRequest, error) {
	req := psRequest{defaults: true}
	for i := 0; i < len(args); i++ {
		a := args[i]
		switch {
		case a == "aux" || a == "-aux":
			req.all, req.bsd, req.defaults = true, true, false
		case a == "-e" || a == "-A" || a == "--every" || a == "--all":
			req.all, req.defaults = true, false
		case a == "-f":
			req.full, req.defaults = true, false
		case a == "-ef" || a == "-fe":
			req.all, req.full, req.defaults = true, true, false
		case a == "-eo" || a == "-o" || a == "--format":
			if i+1 >= len(args) {
				return req, fmt.Errorf("%s needs a field list", a)
			}
			i++
			fields, err := parseFieldList(args[i])
			if err != nil {
				return req, err
			}
			req.fields = fields
			req.defaults = false
			req.full = false
		case strings.HasPrefix(a, "-eo="):
			fields, err := parseFieldList(strings.TrimPrefix(a, "-eo="))
			if err != nil {
				return req, err
			}
			req.fields = fields
			req.defaults, req.full = false, false
		case a == "-p" || a == "--pid":
			if i+1 >= len(args) {
				return req, fmt.Errorf("%s needs a pid", a)
			}
			i++
			pids, err := parsePIDs(args[i])
			if err != nil {
				return req, err
			}
			req.pidOnly = append(req.pidOnly, pids...)
			req.hasPID, req.defaults = true, false
		case strings.HasPrefix(a, "-p") && len(a) > 2:
			pids, err := parsePIDs(a[2:])
			if err != nil {
				return req, err
			}
			req.pidOnly = append(req.pidOnly, pids...)
			req.hasPID, req.defaults = true, false
		case a == "-x":
			// BSD style without the aux columns; the long command line is what
			// matters here, and we always print it when the platform has it.
			req.bsd, req.defaults = true, false
		case strings.HasPrefix(a, "-") && len(a) > 1:
			return req, unknownPSFlag(a)
		default:
			// bare pid list: ps 1234
			pids, err := parsePIDs(a)
			if err != nil {
				return req, unknownPSFlag(a)
			}
			req.pidOnly = append(req.pidOnly, pids...)
			req.hasPID, req.defaults = true, false
		}
	}
	return req, nil
}

func unknownPSFlag(a string) error {
	return fmt.Errorf("unsupported option %q (supported: -e, -A, -f, -ef, aux, -eo FIELDS, -p PID)", a)
}

func parsePIDs(s string) ([]int, error) {
	var out []int
	for _, part := range strings.Split(s, ",") {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		pid, err := strconv.Atoi(part)
		if err != nil {
			return nil, fmt.Errorf("bad pid %q", part)
		}
		out = append(out, pid)
	}
	if len(out) == 0 {
		return nil, fmt.Errorf("no pid given")
	}
	return out, nil
}

func parseFieldList(s string) ([]psField, error) {
	var out []psField
	for _, name := range strings.FieldsFunc(s, func(r rune) bool { return r == ',' || r == ' ' }) {
		// accept the header= form, e.g. pid=PID
		if idx := strings.Index(name, "="); idx >= 0 {
			name = name[:idx]
		}
		f, ok := psFieldByName(name)
		if !ok {
			return nil, fmt.Errorf("unknown field %q (supported: pid, ppid, user, uid, stat, time, etime, tty, comm, cmd)", name)
		}
		out = append(out, f)
	}
	if len(out) == 0 {
		return nil, fmt.Errorf("empty field list")
	}
	return out, nil
}

// runPS lists processes. Linux reads /proc, Windows asks Toolhelp32; the
// columns and the flag parsing are shared, so the same call looks the same on
// both platforms.
func runPS(args []string, stdin *os.File, stdout, stderr *os.File) int {
	if len(args) == 1 && (args[0] == "-h" || args[0] == "--help") {
		fmt.Fprintln(stdout, "usage: ps [-e|-A|-f|-ef|aux|-x] [-eo FIELDS] [-p PID[,PID]]")
		fmt.Fprintln(stdout, "fields: pid ppid user uid stat time etime tty comm cmd")
		return 0
	}
	req, err := parsePS(args)
	if err != nil {
		fmt.Fprintf(stderr, "ps: %v\n", err)
		return 2
	}
	procs, err := listProcesses()
	if err != nil {
		fmt.Fprintf(stderr, "ps: %v\n", err)
		return 1
	}
	selected := selectProcs(procs, req)
	printTable(stdout, req.columnsFor(), selected)
	return 0
}

// selectProcs applies the filters of the request.
func selectProcs(procs []procInfo, req psRequest) []procInfo {
	if req.hasPID {
		want := map[int]bool{}
		for _, pid := range req.pidOnly {
			want[pid] = true
		}
		var out []procInfo
		for _, p := range procs {
			if want[p.PID] {
				out = append(out, p)
			}
		}
		return out
	}
	if req.all {
		return procs
	}
	// Without a selector, procps shows the processes of the current terminal.
	// We show those of the current user, which is the useful subset in a
	// controlled userland where there is often no terminal at all.
	self := os.Getpid()
	me := ""
	for _, p := range procs {
		if p.PID == self {
			me = p.User
			break
		}
	}
	var out []procInfo
	for _, p := range procs {
		if me != "" && p.User == me {
			out = append(out, p)
		}
	}
	if len(out) == 0 {
		return procs
	}
	return out
}

// printTable prints the header and the rows with aligned columns.
func printTable(stdout *os.File, fields []psField, procs []procInfo) {
	widths := make([]int, len(fields))
	for i, f := range fields {
		widths[i] = len(f.header)
	}
	rows := make([][]string, 0, len(procs))
	for _, p := range procs {
		row := make([]string, len(fields))
		for i, f := range fields {
			row[i] = f.value(p)
			if len(row[i]) > widths[i] {
				widths[i] = len(row[i])
			}
		}
		rows = append(rows, row)
	}
	writeRow := func(cells []string) {
		var b strings.Builder
		for i, cell := range cells {
			if i > 0 {
				b.WriteString(" ")
			}
			if fields[i].right {
				b.WriteString(strings.Repeat(" ", widths[i]-len(cell)))
				b.WriteString(cell)
				continue
			}
			b.WriteString(cell)
			if i < len(cells)-1 {
				b.WriteString(strings.Repeat(" ", widths[i]-len(cell)))
			}
		}
		fmt.Fprintln(stdout, b.String())
	}
	header := make([]string, len(fields))
	for i, f := range fields {
		header[i] = f.header
	}
	writeRow(header)
	for _, row := range rows {
		writeRow(row)
	}
}
