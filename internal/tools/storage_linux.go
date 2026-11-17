//go:build !windows

package tools

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
)

// readMemory reads /proc/meminfo.
func readMemory() (mem memInfo, swap memInfo, err error) {
	data, err := os.ReadFile("/proc/meminfo")
	if err != nil {
		return mem, swap, err
	}
	values := map[string]int64{}
	for _, line := range strings.Split(string(data), "\n") {
		key, rest, ok := strings.Cut(line, ":")
		if !ok {
			continue
		}
		fields := strings.Fields(rest)
		if len(fields) == 0 {
			continue
		}
		n, err := strconv.ParseInt(fields[0], 10, 64)
		if err != nil {
			continue
		}
		values[key] = n * 1024 // /proc/meminfo counts KiB
	}
	mem = memInfo{
		Total:     values["MemTotal"],
		Free:      values["MemFree"],
		Shared:    values["Shmem"],
		BuffCache: values["Buffers"] + values["Cached"] + values["SReclaimable"],
		Available: values["MemAvailable"],
		Known:     true,
	}
	mem.Used = mem.Total - mem.Free - mem.BuffCache
	if mem.Used < 0 {
		mem.Used = 0
	}
	if mem.Available == 0 {
		mem.Available = mem.Free + mem.BuffCache
	}
	swapTotal := values["SwapTotal"]
	swapFree := values["SwapFree"]
	swap = memInfo{
		Total:     swapTotal,
		Free:      swapFree,
		Used:      swapTotal - swapFree,
		Available: swapFree,
		Known:     true,
	}
	return mem, swap, nil
}

// readFileSystems reads /proc/self/mountinfo and asks statfs for each mount.
func readFileSystems() ([]diskRow, error) {
	data, err := os.ReadFile("/proc/self/mountinfo")
	if err != nil {
		return nil, err
	}
	seen := map[string]bool{}
	var rows []diskRow
	for _, line := range strings.Split(string(data), "\n") {
		if strings.TrimSpace(line) == "" {
			continue
		}
		fields := strings.Fields(line)
		if len(fields) < 5 {
			continue
		}
		device := fields[2] // major:minor
		mount := unescapeMount(fields[4])
		source := ""
		for i, f := range fields {
			if f == "-" && i+2 < len(fields) {
				source = unescapeMount(fields[i+2])
				break
			}
		}
		if source == "" {
			source = device
		}
		if seen[device] {
			continue
		}
		var st syscall.Statfs_t
		if err := syscall.Statfs(mount, &st); err != nil {
			continue
		}
		seen[device] = true
		size := int64(st.Blocks) * int64(st.Bsize)
		avail := int64(st.Bavail) * int64(st.Bsize)
		free := int64(st.Bfree) * int64(st.Bsize)
		used := (int64(st.Blocks) - int64(st.Bfree)) * int64(st.Bsize)
		percent := 0
		if size > 0 {
			percent = int((size - free) * 100 / size)
		}
		rows = append(rows, diskRow{
			Filesystem: source,
			Size:       size,
			Used:       used,
			Avail:      avail,
			UsePercent: percent,
			MountedOn:  mount,
		})
	}
	return rows, nil
}

// unescapeMount turns the octal escapes of mountinfo back into characters.
func unescapeMount(s string) string {
	if !strings.Contains(s, "\\") {
		return s
	}
	var b strings.Builder
	for i := 0; i < len(s); i++ {
		if s[i] == '\\' && i+3 < len(s) {
			if n, err := strconv.ParseUint(s[i+1:i+4], 8, 8); err == nil {
				b.WriteByte(byte(n))
				i += 3
				continue
			}
		}
		b.WriteByte(s[i])
	}
	return b.String()
}

// usageOf returns the disk blocks a file occupies, which is what du reports by
// default. Sparse files therefore count with their real usage, not their size.
func usageOf(info os.FileInfo, path string) int64 {
	if st, ok := info.Sys().(*syscall.Stat_t); ok {
		blocks := int64(st.Blocks) * 512
		if blocks > 0 {
			return blocks
		}
	}
	return info.Size()
}

func keyOf(info os.FileInfo) (fileKey, bool) {
	st, ok := info.Sys().(*syscall.Stat_t)
	if !ok {
		return fileKey{}, false
	}
	if st.Nlink < 2 {
		return fileKey{}, false
	}
	return fileKey{Dev: uint64(st.Dev), Ino: uint64(st.Ino)}, true
}

func absPath(p string) (string, error) { return filepath.Abs(p) }

func joinPath(root, name string) (string, error) {
	return filepath.Join(root, name), nil
}

var _ = fmt.Sprintf // keep fmt available for platform tweaks
