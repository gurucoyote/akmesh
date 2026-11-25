package sh

import (
	"bytes"
	"io"
	"os"
	"strings"

	"mvdan.cc/sh/v3/interp"
)

// ps4Writer turns the tracer's hard-coded "+ " into the shell's PS4.
//
// mvdan/sh writes every traced command with "+ " and never looks at PS4 (see
// interp/trace.go). ash writes PS4, so a profile that sets PS4 to something else
// would be ignored. This writer sees the same stream as the commands' stderr and
// rewrites only the leading "+ " of a line while xtrace is on, so ordinary
// output that happens to contain a plus is left alone.
type ps4Writer struct {
	w      io.Writer
	runner func() *interp.Runner
	buf    bytes.Buffer
}

func newPS4Writer(w io.Writer, runner func() *interp.Runner) *ps4Writer {
	return &ps4Writer{w: w, runner: runner}
}

// ps4 is the expanded prefix: $PS4 with the escape sequences we also expand in
// PS1, defaulting to "+ " like ash.
func (p *ps4Writer) ps4() string {
	r := p.runner()
	raw := "+ "
	if r != nil {
		if v, ok := r.Vars["PS4"]; ok && v.String() != "" {
			raw = v.String()
		} else if r.Env != nil {
			if v := r.Env.Get("PS4"); v.String() != "" {
				raw = v.String()
			}
		}
	}
	// $VAR and ${VAR} inside PS4, read without running the shell.
	raw = os.Expand(raw, func(name string) string {
		if r != nil {
			if v, ok := r.Vars[name]; ok {
				return v.String()
			}
			if r.Env != nil {
				return r.Env.Get(name).String()
			}
		}
		return ""
	})
	return expandPrompt(raw)
}

func (p *ps4Writer) Write(b []byte) (int, error) {
	// mvdan does not expose whether xtrace is on ("$-" is not among its
	// variables), so the decision rests on PS4 alone: with the default "+ "
	// there is nothing to rewrite, and with a different PS4 every traced line
	// starts with "+ ". The cost is that a command printing a line that begins
	// with "+ " would see the prefix replaced - documented in
	// docs/measurements.md, and only while PS4 differs from the default.
	if p.ps4() == "+ " {
		return p.w.Write(b)
	}
	p.buf.Write(b)
	for {
		line, err := p.buf.ReadBytes('\n')
		if err != nil {
			// No complete line yet.
			p.buf.Write(line)
			break
		}
		prefix := p.ps4()
		if strings.HasPrefix(string(line), "+ ") && prefix != "+ " {
			line = append([]byte(prefix), line[2:]...)
		}
		if _, werr := p.w.Write(line); werr != nil {
			return len(b), werr
		}
	}
	return len(b), nil
}
