//go:build !windows

package proc

import (
	"github.com/gurucoyote/akmesh/internal/shared"

	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

// clockTicks is the unit of the time fields in /proc/pid/stat. Linux uses 100
// for every architecture we target (USER_HZ).
const clockTicks = 100

// listProcesses reads /proc.
func listProcesses() ([]procInfo, error) {
	entries, err := os.ReadDir("/proc")
	if err != nil {
		return nil, err
	}
	bootTime := readBootTime()
	users := shared.ReadPasswd()
	memTotalKB := readMemTotalKB()
	now := time.Now()
	out := make([]procInfo, 0, len(entries))
	for _, e := range entries {
		pid, err := strconv.Atoi(e.Name())
		if err != nil {
			continue
		}
		info, ok := readProc(pid, bootTime, users, now)
		if !ok {
			continue
		}
		// procps prints %CPU as the lifetime average and %MEM as the resident
		// size over total memory. Both are derived here, once the numbers are in.
		if info.Elapsed > 0 && info.CPUTime >= 0 {
			info.CPUPercent = info.CPUTime.Seconds() / info.Elapsed.Seconds() * 100
		}
		if info.RssKB > 0 && memTotalKB > 0 {
			info.MemPercent = float64(info.RssKB) / float64(memTotalKB) * 100
		}
		out = append(out, info)
	}
	return out, nil
}

// readMemTotalKB is the MemTotal line of /proc/meminfo, the denominator procps
// uses for %MEM.
func readMemTotalKB() int64 {
	data, err := os.ReadFile("/proc/meminfo")
	if err != nil {
		return 0
	}
	for line := range strings.SplitSeq(string(data), "\n") {
		if !strings.HasPrefix(line, "MemTotal:") {
			continue
		}
		vals := strings.Fields(line)
		if len(vals) < 2 {
			return 0
		}
		kb, err := strconv.ParseInt(vals[1], 10, 64)
		if err != nil {
			return 0
		}
		return kb
	}
	return 0
}

func readBootTime() time.Time {
	data, err := os.ReadFile("/proc/stat")
	if err != nil {
		return time.Now()
	}
	for line := range strings.SplitSeq(string(data), "\n") {
		if !strings.HasPrefix(line, "btime ") {
			continue
		}
		secs, err := strconv.ParseInt(strings.TrimSpace(strings.TrimPrefix(line, "btime ")), 10, 64)
		if err != nil {
			continue
		}
		return time.Unix(secs, 0)
	}
	return time.Now()
}

func readProc(pid int, bootTime time.Time, users map[string]string, now time.Time) (procInfo, bool) {
	dir := filepath.Join("/proc", strconv.Itoa(pid))
	statData, err := os.ReadFile(filepath.Join(dir, "stat"))
	if err != nil {
		return procInfo{}, false
	}
	fields, comm, ok := splitStat(string(statData))
	if !ok {
		return procInfo{}, false
	}
	info := procInfo{
		PID: pid, Comm: comm,
		CPUPercent: unknown, MemPercent: unknown,
		VsizeKB: unknown, RssKB: unknown,
	}
	// After the parenthesised command name come: state ppid pgrp session
	// tty_nr tpgid flags minflt cminflt majflt cmajflt utime stime cutime
	// cstime priority nice num_threads itrealvalue starttime ...
	if len(fields) > 0 {
		info.Stat = fields[0]
	}
	if len(fields) > 1 {
		info.PPID, _ = strconv.Atoi(fields[1])
	}
	if len(fields) > 4 {
		info.TTY = ttyName(fields[4])
	}
	if len(fields) > 12 {
		utime, _ := strconv.ParseInt(fields[11], 10, 64)
		stime, _ := strconv.ParseInt(fields[12], 10, 64)
		info.CPUTime = time.Duration(utime+stime) * time.Second / clockTicks
	}
	if len(fields) > 19 {
		start, _ := strconv.ParseInt(fields[19], 10, 64)
		info.Started = bootTime.Add(time.Duration(start) * time.Second / clockTicks)
		info.Elapsed = now.Sub(info.Started)
	}
	// vsize in bytes follows starttime. RSS comes from statm, not from the rss
	// field of stat: the kernel's stat rss can lag behind VmRSS (and procps
	// prints the statm number), which showed up as a 144 KB difference in the
	// comparison against procps 4.0.4.
	if len(fields) > 20 {
		if vsize, err := strconv.ParseInt(fields[20], 10, 64); err == nil {
			info.VsizeKB = vsize / 1024
		}
	}
	if data, err := os.ReadFile(filepath.Join(dir, "statm")); err == nil {
		if vals := strings.Fields(string(data)); len(vals) > 1 {
			if resident, err := strconv.ParseInt(vals[1], 10, 64); err == nil {
				info.RssKB = resident * int64(os.Getpagesize()) / 1024
			}
		}
	}
	// argc/argv, NUL separated
	if data, err := os.ReadFile(filepath.Join(dir, "cmdline")); err == nil && len(data) > 0 {
		info.CmdLine = strings.TrimSpace(strings.ReplaceAll(strings.TrimRight(string(data), "\x00"), "\x00", " "))
	}
	if data, err := os.ReadFile(filepath.Join(dir, "status")); err == nil {
		for line := range strings.SplitSeq(string(data), "\n") {
			if !strings.HasPrefix(line, "Uid:") {
				continue
			}
			vals := strings.Fields(strings.TrimPrefix(line, "Uid:"))
			if len(vals) > 0 {
				if name, ok := users[vals[0]]; ok {
					info.User = name
				} else {
					info.User = vals[0]
				}
			}
			break
		}
	}
	return info, true
}

// splitStat splits /proc/pid/stat, whose second field is the command name in
// parentheses and may itself contain spaces and parentheses.
func splitStat(s string) (rest []string, comm string, ok bool) {
	open := strings.Index(s, "(")
	close := strings.LastIndex(s, ")")
	if open < 0 || close < open {
		return nil, "", false
	}
	comm = s[open+1 : close]
	rest = strings.Fields(s[close+1:])
	return rest, comm, true
}

// ttyName turns the device number of the controlling terminal into a name.
func ttyName(field string) string {
	dev, err := strconv.ParseInt(field, 10, 64)
	if err != nil || dev == 0 {
		return "?"
	}
	major := (dev >> 8) & 0xfff
	minor := (dev & 0xff) | ((dev >> 12) & 0xfff00)
	switch major {
	case 136, 137, 138, 139, 140, 141, 142, 143:
		return "pts/" + strconv.FormatInt(minor, 10)
	case 4:
		return "tty" + strconv.FormatInt(minor, 10)
	}
	return "?"
}
