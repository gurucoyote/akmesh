package rgcmd

import (
	"bufio"
	"bytes"
	"io"
	"strconv"
	"sync/atomic"

	"git.site.quack-lab.dev/dave/gripgrep/printer"
	"git.site.quack-lab.dev/dave/gripgrep/search"
)

// binaryTracker wraps a printer sink with ripgrep's binary-file rules. It is
// ported from gripgrep's internal/engine/matchtracker.go (MIT), trimmed to
// what this command needs: no stats, no JSON, no quiet mode.
//
// The rules, as measured against rg 15.1.0:
//   - A discovered file (BinaryQuit) prints the matches found before its first
//     NUL and then one warning line.
//   - A file named on the command line (BinaryConvert) prints one summary line
//     instead of its matching lines.
//   - -c and -l never print either message, and a discovered binary file
//     contributes nothing to them at all.
//
// The tracker also records whether anything matched, which is what the exit
// code is built on (a Sink's Finish cannot report that back).
type binaryTracker struct {
	search.Sink
	matched *atomic.Bool
	// standard is true for the line-printing sink; -c/-l are summaries and
	// take the different branch in Finish.
	standard       bool
	binMode        search.BinaryMode
	showPath       bool
	contextEnabled bool
	dest           *printer.Dest
	searcher       *search.Searcher

	foundBinary       bool
	foundBinaryOffset int64
}

func (t *binaryTracker) Matched(m *search.Match) (bool, error) {
	if t.standard {
		t.noteLineNUL(m.Offset, m.Line)
		if t.binaryConvertSuppressed(m.Offset + int64(len(m.Line))) {
			return true, nil
		}
	}
	return t.Sink.Matched(m)
}

func (t *binaryTracker) Context(c *search.Ctx) (bool, error) {
	if t.standard {
		t.noteLineNUL(c.Offset, c.Line)
		if t.binaryConvertSuppressed(c.Offset + int64(len(c.Line))) {
			return true, nil
		}
	}
	return t.Sink.Context(c)
}

func (t *binaryTracker) Finish(path string, stats *search.Stats) error {
	if t.binMode == search.BinaryQuit && stats.Binary {
		if !t.standard {
			return nil
		}
		if stats.Matched {
			t.matched.Store(true)
		}
		if err := t.Sink.Finish(path, stats); err != nil {
			return err
		}
		if !stats.Matched {
			return nil
		}
		return writeBinaryQuitWarning(t.dest, path, stats.BinaryOffset, t.showPath, t.contextEnabled)
	}
	if t.binMode == search.BinaryConvert {
		if offset, ok := t.effectiveBinaryOffset(); ok {
			if stats.Matched {
				t.matched.Store(true)
			}
			if err := t.Sink.Finish(path, stats); err != nil {
				return err
			}
			if !stats.Matched || !t.standard {
				return nil
			}
			return writeBinaryMessage(t.dest, path, offset, t.showPath, t.contextEnabled)
		}
	}
	if stats.Matched {
		t.matched.Store(true)
	}
	return t.Sink.Finish(path, stats)
}

func (t *binaryTracker) noteLineNUL(lineOffset int64, line []byte) {
	if t.binMode != search.BinaryConvert || t.foundBinary || t.searcher.HasBinaryOffset() {
		return
	}
	if i := bytes.IndexByte(line, 0); i >= 0 {
		t.foundBinary = true
		t.foundBinaryOffset = lineOffset + int64(i)
	}
}

func (t *binaryTracker) effectiveBinaryOffset() (int64, bool) {
	if t.searcher.HasBinaryOffset() {
		return t.searcher.BinaryOffset(), true
	}
	if t.foundBinary {
		return t.foundBinaryOffset, true
	}
	return 0, false
}

// binaryConvertSuppressed withholds every line of a named binary file when its
// NUL sits in the first read buffer, and every line from the NUL on otherwise.
func (t *binaryTracker) binaryConvertSuppressed(lineEnd int64) bool {
	if t.binMode != search.BinaryConvert {
		return false
	}
	binOffset, ok := t.effectiveBinaryOffset()
	if !ok {
		return false
	}
	return binOffset < int64(search.DefaultBufferSize) || lineEnd > binOffset
}

func binarySeparator(contextEnabled bool) []byte {
	if contextEnabled {
		return []byte("--\n")
	}
	return nil
}

func writeBinaryMessage(dest *printer.Dest, path string, offset int64, showPath, contextEnabled bool) error {
	var buf []byte
	if showPath {
		buf = append(buf, path...)
		buf = append(buf, ':', ' ')
	}
	buf = append(buf, `binary file matches (found "\0" byte around offset `...)
	buf = strconv.AppendInt(buf, offset, 10)
	buf = append(buf, ")\n"...)
	return dest.WriteBlock(buf, binarySeparator(contextEnabled))
}

func writeBinaryQuitWarning(dest *printer.Dest, path string, offset int64, showPath, contextEnabled bool) error {
	var buf []byte
	if showPath {
		buf = append(buf, path...)
		buf = append(buf, ':', ' ')
	}
	buf = append(buf, `WARNING: stopped searching binary file after match (found "\0" byte around offset `...)
	buf = strconv.AppendInt(buf, offset, 10)
	buf = append(buf, ")\n"...)
	return dest.WriteBlock(buf, binarySeparator(contextEnabled))
}

// stripBOM drops a leading UTF-8 byte order mark, so a pattern at the start of
// the first line still matches. rg does the same.
func stripBOM(r io.Reader) io.Reader {
	br := bufio.NewReader(r)
	if head, err := br.Peek(3); err == nil && bytes.Equal(head, []byte{0xEF, 0xBB, 0xBF}) {
		_, _ = br.Discard(3)
	}
	return br
}
