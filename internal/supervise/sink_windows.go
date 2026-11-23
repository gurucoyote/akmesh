//go:build windows

package supervise

import (
	"errors"
	"sync/atomic"
)

// SyslogSink exists on Windows only to say that there is no /dev/log. A program
// that needs a log file uses FileSink; nothing is registered with the host.
type SyslogSink struct {
	Socket  string
	tag     string
	sent    int64
	dropped int64
}

// NewSyslogSink keeps the API the same on both targets.
func NewSyslogSink(socket, tag string, q int) *SyslogSink {
	return &SyslogSink{Socket: socket, tag: tag}
}

// TrySend always refuses: no socket, so no line.
func (s *SyslogSink) TrySend(tag string, priority int, line string) bool {
	atomic.AddInt64(&s.dropped, 1)
	return false
}

// Start reports the missing socket once.
func (s *SyslogSink) Start() error {
	return errors.New("no /dev/log on Windows; use FileSink")
}

// Close does nothing.
func (s *SyslogSink) Close() {}

// Dropped is the number of refused lines.
func (s *SyslogSink) Dropped() int64 { return atomic.LoadInt64(&s.dropped) }

// Sent is always zero.
func (s *SyslogSink) Sent() int64 { return 0 }
