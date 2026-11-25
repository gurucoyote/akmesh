package rgcmd

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"slices"
	"strings"
	"sync"

	"akmesh/internal/anymd"
)

// preprocessor turns a document into the text the search runs over. Two forms:
// the built-in any2md reads docx, pdf, epub, rtf and html in process, and any
// other command is run with the file's path as its last argument, its output
// read as the document. The original path stays the label of every hit, so a
// match names the file the caller knows.
//
// Line and column numbers refer to the converted text, not to the original
// file. That is the same thing rg --pre does, and the help text says it.
type preprocessor struct {
	command string // "" = built-in any2md
	globs   []string

	mu    sync.Mutex
	cache map[string]string
}

func newPreprocessor(cfg config) *preprocessor {
	if !cfg.pre {
		return nil
	}
	command := cfg.preCommand
	if command == "any2md" {
		command = ""
	}
	return &preprocessor{command: command, globs: cfg.preGlob, cache: map[string]string{}}
}

// applies reports whether this path goes through the preprocessor. Without
// --pre-glob that is the set of suffixes any2md reads; with it, the patterns
// decide (a leading '!' excludes).
func (p *preprocessor) applies(name string) bool {
	base := filepath.Base(name)
	if len(p.globs) == 0 {
		ext := strings.ToLower(strings.TrimPrefix(filepath.Ext(name), "."))
		return slices.Contains(anymd.Extensions, ext)
	}
	matched := false
	for _, pattern := range p.globs {
		negated := strings.HasPrefix(pattern, "!")
		pat := strings.TrimPrefix(pattern, "!")
		ok, err := path.Match(pat, base)
		if err != nil {
			// An invalid pattern was refused in parse; treat it as no match.
			continue
		}
		if ok {
			if negated {
				return false
			}
			matched = true
		}
	}
	return matched
}

// text returns the converted Markdown for the file. A conversion is done at
// most once per run; the cache is keyed by path.
func (p *preprocessor) text(name string) (string, error) {
	p.mu.Lock()
	if md, ok := p.cache[name]; ok {
		p.mu.Unlock()
		return md, nil
	}
	p.mu.Unlock()

	data, err := os.ReadFile(name)
	if err != nil {
		return "", err
	}
	md, err := p.convert(name, data)
	if err != nil {
		return "", err
	}
	p.mu.Lock()
	p.cache[name] = md
	p.mu.Unlock()
	return md, nil
}

// textBytes converts data that came out of an archive: there is no file to
// hand to an external command, so only the built-in any2md applies.
func (p *preprocessor) textBytes(name string, data []byte) (string, error) {
	return p.convert(name, data)
}

func (p *preprocessor) convert(name string, data []byte) (string, error) {
	if p.command == "" {
		md, err := anymd.Convert(data)
		if err != nil {
			return "", err
		}
		return md, nil
	}
	fields := strings.Fields(p.command)
	if len(fields) == 0 {
		return "", fmt.Errorf("preprocessor: empty command")
	}
	args := append(append([]string{}, fields[1:]...), name)
	cmd := exec.Command(fields[0], args...)
	var out, errOut bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &errOut
	if err := cmd.Run(); err != nil {
		msg := strings.TrimSpace(errOut.String())
		if msg == "" {
			msg = err.Error()
		}
		return "", fmt.Errorf("preprocessor %s: %s", fields[0], msg)
	}
	return out.String(), nil
}
