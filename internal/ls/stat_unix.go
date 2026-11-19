//go:build !windows

// Local addition, no upstream source: the numbers a long listing shows, from
// the platform's own stat (acmesh-wx6).

package ls

import (
	"os/user"
	"strconv"
	"syscall"
	"time"
)

func (e entry) inode() uint64 {
	if st, ok := e.info.Sys().(*syscall.Stat_t); ok {
		return st.Ino
	}
	return 0
}

func (e entry) links() uint64 {
	if st, ok := e.info.Sys().(*syscall.Stat_t); ok {
		return uint64(st.Nlink)
	}
	return 1
}

// blocks is the allocated space in 512-byte units, which is what GNU sums for
// the total line. Allocated, not apparent size: a sparse file counts small.
func (e entry) blocks() int64 {
	if st, ok := e.info.Sys().(*syscall.Stat_t); ok {
		return st.Blocks
	}
	return (e.size + 511) / 512
}

func (e entry) accessTime() (time.Time, bool) {
	if st, ok := e.info.Sys().(*syscall.Stat_t); ok {
		return time.Unix(st.Atim.Sec, st.Atim.Nsec), true
	}
	return time.Time{}, false
}

func (e entry) changeTime() (time.Time, bool) {
	if st, ok := e.info.Sys().(*syscall.Stat_t); ok {
		return time.Unix(st.Ctim.Sec, st.Ctim.Nsec), true
	}
	return time.Time{}, false
}

func (l lister) ownerName(e entry) string {
	uid, _, ok := e.ids()
	if !ok {
		return "-"
	}
	if l.o.numeric {
		return strconv.FormatUint(uint64(uid), 10)
	}
	if u, err := user.LookupId(strconv.FormatUint(uint64(uid), 10)); err == nil {
		return u.Username
	}
	return strconv.FormatUint(uint64(uid), 10)
}

func (l lister) groupName(e entry) string {
	_, gid, ok := e.ids()
	if !ok {
		return "-"
	}
	if l.o.numeric {
		return strconv.FormatUint(uint64(gid), 10)
	}
	if g, err := user.LookupGroupId(strconv.FormatUint(uint64(gid), 10)); err == nil {
		return g.Name
	}
	return strconv.FormatUint(uint64(gid), 10)
}

func (e entry) ids() (uid, gid uint32, ok bool) {
	if st, ok := e.info.Sys().(*syscall.Stat_t); ok {
		return st.Uid, st.Gid, true
	}
	return 0, 0, false
}
