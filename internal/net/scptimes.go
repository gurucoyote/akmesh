//go:build !nonet

package net

import (
	"os"
	"strconv"
	"strings"
	"time"
)

// scpTimes is one T record of the scp protocol: the modification and access
// time of the entry that follows.
type scpTimes struct {
	mtime time.Time
	atime time.Time
}

// parseScpTimes reads "T<mtime> 0 <atime> 0". A malformed record gives nil, which
// the caller treats as "no times announced".
func parseScpTimes(header string) *scpTimes {
	fields := strings.Fields(header)
	if len(fields) < 4 {
		return nil
	}
	// The caller consumed the leading T already, so the first field is the
	// modification time in seconds; TrimPrefix keeps both call shapes working.
	seconds, err := strconv.ParseInt(strings.TrimPrefix(fields[0], "T"), 10, 64)
	if err != nil {
		return nil
	}
	atimeSecs, err := strconv.ParseInt(fields[2], 10, 64)
	if err != nil {
		return nil
	}
	mtime := time.Unix(seconds, 0)
	atime := time.Unix(atimeSecs, 0)
	return &scpTimes{mtime: mtime, atime: atime}
}

// applyScpTimes sets both times on a path. A failure is not worth stopping the
// transfer for; the file is there, which is what the caller asked for.
func applyScpTimes(path string, t *scpTimes) {
	_ = os.Chtimes(path, t.atime, t.mtime)
}
