// Package supervise is the Aufpasser (acmesh-17s): it starts a child, passes its
// output through unchanged to this process's stdout and stderr, and in addition
// sends every line to syslog with a tag and a level.
//
// The point is that no program has to be changed: whatever runs in the container
// is logged, including foreign binaries. stdout stays data, stderr stays
// protocol - both are forwarded, but stdout goes to syslog as info and stderr as
// err, so logread does not show data as an error.
//
// Three modes, chosen with AKMESH_SUPERVISE:
//
//	dup      (default) pass through and forward - the duplication
//	forward  forward only, the child's output is not passed through
//	pass     pass through only, on the real descriptors, nothing is read
//
// "pass" exists for programs that insist on a real terminal or that buffer by
// descriptor; the price of dup is that a child sees a pipe instead of a tty.
package supervise

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"
	"sync"
	"sync/atomic"
)

// Mode is how a child's output is treated.
type Mode int

const (
	// ModeDup passes through and forwards (default).
	ModeDup Mode = iota
	// ModeForward sends lines to syslog and does not pass them through.
	ModeForward
	// ModePass leaves the descriptors alone and forwards nothing.
	ModePass
)

// EnvName is the variable that switches the mode.
const EnvName = "AKMESH_SUPERVISE"

// ModeFromEnv reads AKMESH_SUPERVISE. An unknown value is an error, not a silent
// default: a typo would otherwise log nothing without a word.
func ModeFromEnv() (Mode, error) {
	v := strings.ToLower(strings.TrimSpace(os.Getenv(EnvName)))
	switch v {
	case "", "dup", "1", "true", "yes":
		return ModeDup, nil
	case "forward", "only", "syslog":
		return ModeForward, nil
	case "pass", "passthrough", "direct", "0", "false", "no":
		return ModePass, nil
	}
	return ModeDup, fmt.Errorf("%s=%q is not a mode (dup, forward, pass)", EnvName, v)
}

// Sink receives the forwarded lines. TrySend must not block: it reports false
// when the line was dropped, and the caller counts it. A nil Sink means "no
// forwarding".
type Sink interface {
	TrySend(tag string, priority int, line string) bool
}

// Priorities used for the two streams: syslog info and err on facility user.
const (
	PriorityStdout = 14 // user.info
	PriorityStderr = 11 // user.err
)

// Config is what Start needs.
type Config struct {
	Mode    Mode
	Tag     string
	Sink    Sink
	MaxLine int // 0 = 4096
	// Out and Err are where lines are passed through to; nil means the
	// process's own stdout and stderr.
	Out io.Writer
	Err io.Writer
}

// Pipes is the wiring of a child's output: the two read ends, the goroutines and
// the counters. init uses it without exec.Cmd.Wait, because init reaps with
// Wait4(-1) itself; everything else uses Start.
type Pipes struct {
	wg      sync.WaitGroup
	outW    *os.File
	errW    *os.File
	dropped int64
}

// Attach creates the pipes and starts the readers, but does not start the child.
// The caller must start it and then call AfterStart.
func Attach(cmd *exec.Cmd, cfg Config) (*Pipes, error) {
	maxLine := cfg.MaxLine
	if maxLine == 0 {
		maxLine = 4096
	}
	var passOut, passErr io.Writer
	if cfg.Mode == ModeDup {
		passOut, passErr = cfg.Out, cfg.Err
		if passOut == nil {
			passOut = os.Stdout
		}
		if passErr == nil {
			passErr = os.Stderr
		}
	}
	// pass: the child writes on the real descriptors, nothing is read.
	if cfg.Mode == ModePass {
		if cmd.Stdout == nil {
			cmd.Stdout = cfg.Out
		}
		if cmd.Stderr == nil {
			cmd.Stderr = cfg.Err
		}
		return &Pipes{}, nil
	}
	outR, outW, err := os.Pipe()
	if err != nil {
		return nil, err
	}
	errR, errW, err := os.Pipe()
	if err != nil {
		outR.Close()
		outW.Close()
		return nil, err
	}
	cmd.Stdout = outW
	cmd.Stderr = errW
	p := &Pipes{outW: outW, errW: errW}
	p.wg.Add(2)
	go func() { defer p.wg.Done(); pump(outR, passOut, PriorityStdout, maxLine, cfg.Sink, &p.dropped, cfg.Tag) }()
	go func() { defer p.wg.Done(); pump(errR, passErr, PriorityStderr, maxLine, cfg.Sink, &p.dropped, cfg.Tag) }()
	return p, nil
}

// AfterStart closes the parent's copies of the write ends, so the readers see EOF
// when the child dies.
func (p *Pipes) AfterStart() {
	if p.outW != nil {
		p.outW.Close()
	}
	if p.errW != nil {
		p.errW.Close()
	}
}

// Abort closes the write ends after a failed start.
func (p *Pipes) Abort() { p.AfterStart() }

// Wait waits for both readers, so no line of the child is lost.
func (p *Pipes) Wait() { p.wg.Wait() }

// Dropped is how many lines the sink refused.
func (p *Pipes) Dropped() int64 { return atomic.LoadInt64(&p.dropped) }

// Session is one supervised child started with Start.
type Session struct {
	cmd   *exec.Cmd
	pipes *Pipes
	code  int
	got   int64
}

// Start wires the output and starts the child. In pass mode the caller's own
// Stdout/Stderr on cmd are left untouched.
func Start(cmd *exec.Cmd, cfg Config) (*Session, error) {
	p, err := Attach(cmd, cfg)
	if err != nil {
		return nil, err
	}
	if err := cmd.Start(); err != nil {
		p.Abort()
		return nil, err
	}
	p.AfterStart()
	return &Session{cmd: cmd, pipes: p}, nil
}

// Wait waits for the child and then for the readers, and returns the exit code
// and how many lines were dropped.
func (s *Session) Wait() (int, int64) {
	err := s.cmd.Wait()
	s.pipes.Wait()
	code := 0
	if err != nil {
		if ee, ok := err.(*exec.ExitError); ok {
			code = ee.ExitCode()
		} else {
			code = 1
		}
	}
	s.code, s.got = code, s.pipes.Dropped()
	return s.code, s.got
}

// Code is the exit code seen by the last Wait.
func (s *Session) Code() int { return s.code }

// pump reads lines, passes them through when w is not nil and forwards them.
// A line longer than max is cut and marked; the rest of that line is discarded
// until its newline, so the stream stays line-aligned.
func pump(r *os.File, w io.Writer, priority int, maxLine int, sink Sink, dropped *int64, tag string) {
	defer r.Close()
	br := bufio.NewReaderSize(r, 32*1024)
	var acc []byte
	cut := false
	flush := func() {
		line := strings.TrimRight(string(acc), "\r\n")
		if cut {
			line += " (gekuerzt)"
		}
		acc = acc[:0]
		cut = false
		if w != nil {
			fmt.Fprintln(w, line)
		}
		if sink != nil && !sink.TrySend(tag, priority, line) {
			atomic.AddInt64(dropped, 1)
		}
	}
	for {
		frag, err := br.ReadSlice('\n')
		if len(frag) > 0 {
			room := maxLine - len(acc)
			switch {
			case room <= 0:
				cut = true
			case len(frag) > room:
				acc = append(acc, frag[:room]...)
				cut = true
			default:
				acc = append(acc, frag...)
			}
		}
		switch err {
		case nil:
			flush()
		case bufio.ErrBufferFull:
			// The line goes on; keep collecting up to the limit.
		case io.EOF:
			// The last line of a dying child often has no newline: deliver it.
			if len(acc) > 0 || cut {
				flush()
			}
			return
		default:
			if len(acc) > 0 || cut {
				flush()
			}
			return
		}
	}
}

// ChildEnv adds the mode back to a child's environment, so a nested command sees
// the same setting.
func ChildEnv(env []string, mode Mode) []string {
	out := make([]string, 0, len(env)+1)
	for _, kv := range env {
		if strings.HasPrefix(kv, EnvName+"=") {
			continue
		}
		out = append(out, kv)
	}
	return append(out, EnvName+"="+mode.String())
}

// String names the mode in a log line.
func (m Mode) String() string {
	switch m {
	case ModeForward:
		return "forward"
	case ModePass:
		return "pass"
	default:
		return "dup"
	}
}
