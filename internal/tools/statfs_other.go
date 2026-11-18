//go:build !linux

package tools

import (
	"errors"
	"os"
	"strconv"
	"time"
)

// Windows and wasm have no st_ino, st_nlink, uid, gid, blocks or statfs. The
// numbers stay zero and stat -f answers with an error, because a wrong number
// is worse for an agent than a missing one.

type fileStatExtra struct {
	Inode, Links, UID, GID, Blocks, IOBlock, Device uint64
	RdevMajor, RdevMinor                            uint64
	Atime, Ctime                                    time.Time
}

func statExtraOf(info os.FileInfo) fileStatExtra { return fileStatExtra{} }

var errNoStatfs = errors.New("file system statistics are not available on this platform")

func fileSystemStats(name string) (map[byte]string, error) { return nil, errNoStatfs }

// userName and groupName fall back to the number: there is no passwd file to
// read, and inventing a name would be worse than the id GNU would print.
func userName(uid uint64) string { return strconv.FormatUint(uid, 10) }

func groupName(gid uint64) string { return strconv.FormatUint(gid, 10) }
