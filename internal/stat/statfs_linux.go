//go:build linux

package stat

import (
	"github.com/gurucoyote/akmesh/internal/shared"

	"os"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"time"
)

// fileStatExtra is the part of a stat result that os.FileInfo does not carry:
// the numbers GNU stat prints as %i %h %u %g %b %d %t %T and the ctime.
type fileStatExtra struct {
	Inode, Links, UID, GID, Blocks, IOBlock, Device uint64
	RdevMajor, RdevMinor                            uint64
	Atime, Ctime                                    time.Time
}

func statExtraOf(info os.FileInfo) fileStatExtra {
	st, ok := info.Sys().(*syscall.Stat_t)
	if !ok {
		return fileStatExtra{}
	}
	major, minor := deviceParts(uint64(st.Rdev))
	return fileStatExtra{
		Inode:     uint64(st.Ino),
		Links:     uint64(st.Nlink),
		UID:       uint64(st.Uid),
		GID:       uint64(st.Gid),
		Blocks:    uint64(st.Blocks),
		IOBlock:   uint64(st.Blksize),
		Device:    uint64(st.Dev),
		RdevMajor: major,
		RdevMinor: minor,
		Atime:     time.Unix(st.Atim.Sec, st.Atim.Nsec),
		Ctime:     time.Unix(st.Ctim.Sec, st.Ctim.Nsec),
	}
}

// fileSystemStats fills stat -f from statfs(2), including the file system type
// read from /proc/self/mounts for the mount that contains the path.
func fileSystemStats(name string) (map[byte]string, error) {
	var st syscall.Statfs_t
	if err := syscall.Statfs(name, &st); err != nil {
		return nil, err
	}
	return map[byte]string{
		'n': name,
		'b': itoa(int64(st.Blocks)),
		'f': itoa(int64(st.Bfree)),
		'a': itoa(int64(st.Bavail)),
		'c': itoa(int64(st.Files)),
		'd': itoa(int64(st.Ffree)),
		's': itoa(int64(st.Bsize)),
		'S': itoa(int64(st.Frsize)),
		'l': itoa(int64(st.Namelen)),
		'i': itoa(int64(st.Fsid.X__val[0])),
		'T': mountType(name),
	}, nil
}

// mountType finds the file system type of the mount that holds name. The
// longest matching mount point wins, so /proc inside / is not shadowed by /.
func mountType(name string) string {
	abs, err := absPath(name)
	if err != nil {
		return "unknown"
	}
	data, err := os.ReadFile("/proc/self/mounts")
	if err != nil {
		return "unknown"
	}
	best, bestType := "", "unknown"
	for line := range strings.SplitSeq(string(data), "\n") {
		fields := strings.Fields(line)
		if len(fields) < 3 {
			continue
		}
		mount := unescapeMount(fields[1])
		if abs == mount || strings.HasPrefix(abs, strings.TrimSuffix(mount, "/")+"/") {
			if len(mount) >= len(best) {
				best, bestType = mount, fields[2]
			}
		}
	}
	return bestType
}

func itoa(n int64) string {
	if n == 0 {
		return "0"
	}
	neg := n < 0
	if neg {
		n = -n
	}
	var digits []byte
	for n > 0 {
		digits = append([]byte{byte('0' + n%10)}, digits...)
		n /= 10
	}
	if neg {
		return "-" + string(digits)
	}
	return string(digits)
}

// userName and groupName resolve the numeric owner to a name, as GNU stat does
// for %U %G. An unknown id stays a number instead of an invented name.
func userName(uid uint64) string {
	if name, ok := shared.ReadPasswd()[strconv.FormatUint(uid, 10)]; ok && name != "" {
		return name
	}
	return strconv.FormatUint(uid, 10)
}

func groupName(gid uint64) string {
	want := strconv.FormatUint(gid, 10)
	data, err := os.ReadFile("/etc/group")
	if err == nil {
		for line := range strings.SplitSeq(string(data), "\n") {
			parts := strings.Split(line, ":")
			if len(parts) >= 3 && parts[2] == want {
				return parts[0]
			}
		}
	}
	return want
}

func absPath(p string) (string, error) { return filepath.Abs(p) }

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
