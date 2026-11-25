//go:build !windows

package supervise

import (
	"net"
	"sync/atomic"
	"time"
)

// SyslogSink sends lines to a /dev/log style socket without ever blocking the
// reader: a buffered channel, a goroutine that writes, and a counter for what was
// dropped. A syslog daemon that stops reading must not stop the child - that is
// the rule in acmesh-17s.
type SyslogSink struct {
	Socket  string
	tag     string
	ch      chan string
	dropped int64
	// sent counts the lines the socket took.
	sent int64
}

// NewSyslogSink builds a sink with a queue of q lines (0 = 256).
func NewSyslogSink(socket, tag string, q int) *SyslogSink {
	if q == 0 {
		q = 256
	}
	return &SyslogSink{Socket: socket, tag: tag, ch: make(chan string, q)}
}

// TrySend queues one line; false means the queue is full and the line is lost.
// The tag argument wins over the sink's default, so one sink serves many children.
func (s *SyslogSink) TrySend(tag string, priority int, line string) bool {
	if tag == "" {
		tag = s.tag
	}
	msg := formatLine(priority, tag, line)
	select {
	case s.ch <- msg:
		return true
	default:
		atomic.AddInt64(&s.dropped, 1)
		return false
	}
}

// Dropped is how many lines the queue refused.
func (s *SyslogSink) Dropped() int64 { return atomic.LoadInt64(&s.dropped) }

// Sent is how many lines the socket accepted.
func (s *SyslogSink) Sent() int64 { return atomic.LoadInt64(&s.sent) }

// Start begins writing in the background. It deliberately does not open the
// socket: init and crond create their sink before the log daemon is up, so the
// connection is made on the first line and retried while the daemon is missing.
func (s *SyslogSink) Start() error {
	go func() {
		var conn net.Conn
		defer func() {
			if conn != nil {
				conn.Close()
			}
		}()
		for msg := range s.ch {
			if conn == nil {
				// The daemon may still be starting: give it half a second
				// before the line is given up, so the first lines of the first
				// service are not the ones that disappear.
				for range 5 {
					c, err := net.Dial("unixgram", s.Socket)
					if err == nil {
						conn = c
						break
					}
					time.Sleep(100 * time.Millisecond)
				}
				if conn == nil {
					atomic.AddInt64(&s.dropped, 1)
					continue
				}
			}
			if _, err := conn.Write([]byte(msg)); err != nil {
				conn.Close()
				conn = nil
				atomic.AddInt64(&s.dropped, 1)
				continue
			}
			atomic.AddInt64(&s.sent, 1)
		}
	}()
	return nil
}

// Close stops the writer after the queued lines are written.
func (s *SyslogSink) Close() {
	close(s.ch)
}
