//go:build windows

package disk

import (
	"fmt"
	"os"
	"path/filepath"
	"syscall"
	"unsafe"
)

var (
	kernel32       = syscall.NewLazyDLL("kernel32.dll")
	procMemStatus  = kernel32.NewProc("GlobalMemoryStatusEx")
	procDriveMask  = kernel32.NewProc("GetLogicalDrives")
	procDiskFreeEx = kernel32.NewProc("GetDiskFreeSpaceExW")
)

type memStatusEx struct {
	Length               uint32
	MemoryLoad           uint32
	TotalPhys            uint64
	AvailPhys            uint64
	TotalPageFile        uint64
	AvailPageFile        uint64
	TotalVirtual         uint64
	AvailVirtual         uint64
	AvailExtendedVirtual uint64
}

// readMemory asks GlobalMemoryStatusEx. Windows has no equivalent of "buff/
// cache", so those cells stay unknown and print as "-" instead of a guess.
func readMemory() (mem memInfo, swap memInfo, err error) {
	var m memStatusEx
	m.Length = uint32(unsafe.Sizeof(m))
	ok, _, callErr := procMemStatus.Call(uintptr(unsafe.Pointer(&m)))
	if ok == 0 {
		return mem, swap, fmt.Errorf("GlobalMemoryStatusEx: %v", callErr)
	}
	mem = memInfo{
		Total:     int64(m.TotalPhys),
		Free:      int64(m.AvailPhys),
		Used:      int64(m.TotalPhys - m.AvailPhys),
		Shared:    -1, // not exposed by GlobalMemoryStatusEx
		BuffCache: -1,
		Available: int64(m.AvailPhys),
		Known:     false,
	}
	// The page file numbers in GlobalMemoryStatusEx are commit limit and commit
	// available, not page file usage; subtracting the physical totals gives a
	// number that looks like swap but is not. Windows cannot answer this from
	// that structure, so the row stays unknown instead of wrong.
	swap = memInfo{Total: -1, Used: -1, Free: -1, Available: -1, Known: false}
	return mem, swap, nil
}

// readFileSystems walks the drive letters. A drive that is not ready (an empty
// card reader, for instance) is skipped instead of failing the whole command.
func readFileSystems() ([]diskRow, error) {
	mask, _, _ := procDriveMask.Call()
	if mask == 0 {
		return nil, fmt.Errorf("GetLogicalDrives returned no drives")
	}
	var rows []diskRow
	for i := 0; i < 26; i++ {
		if mask&(1<<uint(i)) == 0 {
			continue
		}
		letter := string(rune('A' + i))
		root := letter + ":\\"
		ptr, err := syscall.UTF16PtrFromString(root)
		if err != nil {
			continue
		}
		var freeToCaller, total, totalFree uint64
		ok, _, _ := procDiskFreeEx.Call(uintptr(unsafe.Pointer(ptr)),
			uintptr(unsafe.Pointer(&freeToCaller)),
			uintptr(unsafe.Pointer(&total)),
			uintptr(unsafe.Pointer(&totalFree)))
		if ok == 0 || total == 0 {
			continue
		}
		percent := 0
		if total > 0 {
			percent = int((total - totalFree) * 100 / total)
		}
		rows = append(rows, diskRow{
			Filesystem: letter + ":",
			Size:       int64(total),
			Used:       int64(total - totalFree),
			Avail:      int64(freeToCaller),
			UsePercent: percent,
			MountedOn:  root,
		})
	}
	return rows, nil
}

// usageOf returns the file size. Windows allocation size would need a handle per
// file, which costs more than it explains for an agent.
func usageOf(info os.FileInfo, path string) int64 { return info.Size() }

// usageOfB: Windows has no block count here, so both modes report the apparent
// size; a directory counts as zero with -b, as in GNU du.
func usageOfB(info os.FileInfo, path string, apparent bool) int64 {
	if apparent && info.IsDir() {
		return 0
	}
	return info.Size()
}

// devOf is not available on Windows; -x therefore has no effect there.
func devOf(info os.FileInfo) (uint64, bool) { return 0, false }

func keyOf(info os.FileInfo) (fileKey, bool) { return fileKey{}, false }

func absPath(p string) (string, error) { return filepath.Abs(p) }

func joinPath(root, name string) (string, error) { return filepath.Join(root, name), nil }
