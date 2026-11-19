//go:build windows

// Local addition, no upstream source: reads the command line of a process
// through ProcessCommandLineInformation (acmesh-94e.22).

package proc

import (
	"errors"
	"fmt"
	"unsafe"

	"golang.org/x/sys/windows"
)

// The command line is not in the Toolhelp32 table. It can be had in two ways:
// read the target's PEB and walk to the ProcessParameters (needs
// PROCESS_VM_READ, breaks on 32-bit processes in a 64-bit system, and a bad
// pointer takes the whole process down), or ask ntdll for
// ProcessCommandLineInformation, which the kernel copies into our own buffer.
// The second way needs only PROCESS_QUERY_LIMITED_INFORMATION, the same right ps
// already uses for the times, and cannot read foreign memory. It exists since
// Windows 8.1 and answers STATUS_INVALID_INFO_CLASS on older systems, where the
// cmd column keeps the executable name.
const (
	processCommandLineInformation = 60
	// statusInfoLengthMismatch is 0xC0000004: "you gave me no buffer".
	statusInfoLengthMismatch = 0xC0000004
	// maxCommandLine bounds the buffer: 32 KiB is the Windows limit for a
	// command line, anything larger is a broken answer, not a long one.
	maxCommandLine = 32 * 1024
)

var procNtQueryInformationProcess = windows.NewLazySystemDLL("ntdll.dll").NewProc("NtQueryInformationProcess")

// unicodeString is UNICODE_STRING: length in bytes, capacity in bytes, and a
// pointer into the same buffer the kernel filled.
type unicodeString struct {
	Length        uint16
	MaximumLength uint16
	Buffer        *uint16
}

// errNoCommandLine reports a process whose command line is empty or not
// readable; ps keeps the executable name then.
var errNoCommandLine = errors.New("no command line")

// processCommandLine returns the command line of pid, as the target sees it:
// the image path first, then the arguments, in one string.
func processCommandLine(pid int) (string, error) {
	h, err := windows.OpenProcess(windows.PROCESS_QUERY_LIMITED_INFORMATION, false, uint32(pid))
	if err != nil {
		return "", err
	}
	defer windows.CloseHandle(h)

	// First call asks for the size, second one fills the buffer.
	var size uint32
	status, _, _ := procNtQueryInformationProcess.Call(
		uintptr(h), processCommandLineInformation, 0, 0, uintptr(unsafe.Pointer(&size)))
	if status != statusInfoLengthMismatch {
		return "", fmt.Errorf("command line of %d: status 0x%x", pid, status)
	}
	if size == 0 || size > maxCommandLine {
		return "", errNoCommandLine
	}
	buf := make([]byte, size)
	status, _, _ = procNtQueryInformationProcess.Call(
		uintptr(h), processCommandLineInformation,
		uintptr(unsafe.Pointer(&buf[0])), uintptr(len(buf)), uintptr(unsafe.Pointer(&size)))
	if status != 0 {
		return "", fmt.Errorf("command line of %d: status 0x%x", pid, status)
	}

	us := (*unicodeString)(unsafe.Pointer(&buf[0]))
	if us.Buffer == nil || us.Length == 0 {
		return "", errNoCommandLine
	}
	// The string sits inside buf, behind the UNICODE_STRING header; the offset is
	// computed and checked rather than trusted.
	base := uintptr(unsafe.Pointer(&buf[0]))
	start := uintptr(unsafe.Pointer(us.Buffer))
	end := start + uintptr(us.Length)
	if start < base || end > base+uintptr(len(buf)) || us.Length%2 != 0 {
		return "", errNoCommandLine
	}
	text := make([]uint16, us.Length/2)
	copy(text, unsafe.Slice(us.Buffer, us.Length/2))
	return windows.UTF16ToString(text), nil
}
