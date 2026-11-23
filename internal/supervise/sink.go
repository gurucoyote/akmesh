package supervise

import (
	"fmt"
	"io"
	"sync"
	"sync/atomic"
	"time"
)

// formatLine builds an RFC 3164 line. It is a small copy of syslog.Format on
// purpose: internal/syslog is one package for the three logging commands, and the
// supervisor must not drag them into every program that uses it.
func formatLine(priority int, tag, msg string) string {
	if tag == "" {
		tag = "supervise"
	}
	now := time.Now()
	return fmt.Sprintf("<%d>%s %2d %02d:%02d:%02d %s: %s\n", priority, now.Format("Jan"), now.Day(),
		now.Hour(), now.Minute(), now.Second(), tag, msg)
}

// FileSink appends to a file (Windows, where there is no /dev/log).
type FileSink struct {
	mu      sync.Mutex
	w       io.Writer
	dropped int64
}

// NewFileSink wraps a writer.
func NewFileSink(w io.Writer) *FileSink { return &FileSink{w: w} }

// TrySend writes the line; a failing writer counts as dropped but never blocks
// the child for long.
func (f *FileSink) TrySend(tag string, priority int, line string) bool {
	if tag == "" {
		tag = "supervise"
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	if _, err := io.WriteString(f.w, formatLine(priority, tag, line)); err != nil {
		atomic.AddInt64(&f.dropped, 1)
		return false
	}
	return true
}

// Dropped is how many writes failed.
func (f *FileSink) Dropped() int64 { return atomic.LoadInt64(&f.dropped) }
