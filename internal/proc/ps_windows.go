//go:build windows

package proc

import (
	"time"
	"unsafe"

	"golang.org/x/sys/windows"
)

// listProcesses asks Toolhelp32 for the process table. The command line is not
// part of that table: it lives in the target process's PEB, which would mean
// reading foreign memory. That is deliberately not done here (see
// docs/ps-kill-einschaetzung.md); the cmd column falls back to the executable
// name.
func listProcesses() ([]procInfo, error) {
	snap, err := windows.CreateToolhelp32Snapshot(windows.TH32CS_SNAPPROCESS, 0)
	if err != nil {
		return nil, err
	}
	defer windows.CloseHandle(snap)

	var entry windows.ProcessEntry32
	entry.Size = uint32(unsafe.Sizeof(entry))
	if err := windows.Process32First(snap, &entry); err != nil {
		return nil, err
	}
	now := time.Now()
	var out []procInfo
	for {
		info := procInfo{
			PID:  int(entry.ProcessID),
			PPID: int(entry.ParentProcessID),
			Comm: windows.UTF16ToString(entry.ExeFile[:]),
			TTY:  "?",
			Stat: "?",
			// Negative means "unknown"; the table prints those as "-".
			CPUTime: -1,
			Elapsed: -1,
		}
		fillProcessDetail(&info, now)
		out = append(out, info)
		if err := windows.Process32Next(snap, &entry); err != nil {
			break
		}
	}
	return out, nil
}

// filetimeDuration reads a FILETIME as a duration (100ns units).
func filetimeDuration(ft windows.Filetime) time.Duration {
	ticks := uint64(ft.HighDateTime)<<32 | uint64(ft.LowDateTime)
	return time.Duration(ticks) * 100 * time.Nanosecond
}

// fillProcessDetail adds the fields Toolhelp32 does not carry. Every one of them
// needs a handle, and not every handle can be opened: a protected process still
// shows up in the table, only with less detail.
func fillProcessDetail(info *procInfo, now time.Time) {
	const access = windows.PROCESS_QUERY_LIMITED_INFORMATION
	h, err := windows.OpenProcess(access, false, uint32(info.PID))
	if err != nil {
		return
	}
	defer windows.CloseHandle(h)

	var creation, exit, kernel, user windows.Filetime
	if err := windows.GetProcessTimes(h, &creation, &exit, &kernel, &user); err == nil {
		// Filetime.Nanoseconds() subtracts the 1601 epoch, which is right for a
		// point in time and wrong for a duration. CPU time is a duration, so it
		// is read from the raw 100ns counter.
		info.CPUTime = filetimeDuration(kernel) + filetimeDuration(user)
		started := time.Unix(0, creation.Nanoseconds())
		if !started.IsZero() {
			info.Elapsed = now.Sub(started)
		}
	}

	var token windows.Token
	if err := windows.OpenProcessToken(h, windows.TOKEN_QUERY, &token); err != nil {
		return
	}
	defer token.Close()
	tu, err := token.GetTokenUser()
	if err != nil {
		return
	}
	account, domain, _, err := tu.User.Sid.LookupAccount("")
	if err != nil {
		info.User = tu.User.Sid.String()
		return
	}
	if domain != "" {
		info.User = domain + `\` + account
	} else {
		info.User = account
	}
}
