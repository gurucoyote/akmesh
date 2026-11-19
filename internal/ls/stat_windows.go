//go:build windows

// Local addition, no upstream source: what a Windows long listing can honestly
// show (acmesh-wx6).

package ls

import (
	"os"
	"os/user"
	"strconv"
	"time"
)

// inode and links do not exist on Windows. 0 and 1 are honest: a number read
// from nowhere would look real.
func (e entry) inode() uint64 { return 0 }

func (e entry) links() uint64 {
	if e.info.IsDir() {
		// A directory has at least itself and the parent's entry.
		return 2
	}
	return 1
}

// blocks falls back to the apparent size: Windows reports no allocation here,
// and a sparse file then counts at its full size.
func (e entry) blocks() int64 { return (e.size + 511) / 512 }

func (e entry) accessTime() (time.Time, bool) { return time.Time{}, false }
func (e entry) changeTime() (time.Time, bool) { return time.Time{}, false }

// ownerName is the account that owns the process, because Windows file ownership
// is a SID and not a uid; groupName has no counterpart at all.
func (l lister) ownerName(e entry) string {
	if l.o.numeric {
		return "0"
	}
	if u, err := user.Current(); err == nil {
		return u.Username
	}
	if name := os.Getenv("USERNAME"); name != "" {
		return name
	}
	return "-"
}

func (l lister) groupName(e entry) string {
	if l.o.numeric {
		return "0"
	}
	return "-"
}

var _ = strconv.Itoa
