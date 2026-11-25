package sh

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// shellBreaks are the characters that end a word for completion: a space, the
// tab itself, and the operators a shell splits on.
func isWordBreak(r rune) bool {
	switch r {
	case ' ', '\t', ';', '|', '&', '(', ')', '<', '>':
		return true
	}
	return false
}

// completeLine completes the word under the cursor. The first word of a line is
// a command name and is looked for along PATH; every other word is a path,
// relative to the shell's current directory. A single candidate is filled in; a
// directory gets its slash; several candidates are listed and nothing is
// changed, which is what a person expects from Tab (25.11.2026).
func (e *editor) completeLine(buf []rune, pos int, prompt string) ([]rune, int) {
	start := pos
	for start > 0 && !isWordBreak(buf[start-1]) {
		start--
	}
	word := string(buf[start:pos])
	first := true
	for i := 0; i < start; i++ {
		if !isWordBreak(buf[i]) {
			first = false
			break
		}
	}
	var candidates []string
	if first {
		candidates = e.commands(word)
	} else {
		candidates = e.paths(word)
	}
	if len(candidates) == 0 {
		return buf, pos
	}
	sort.Strings(candidates)
	// The part all candidates share can be filled in without a decision.
	shared := candidates[0]
	full := false
	for _, c := range candidates[1:] {
		shared = commonPrefix(shared, c)
	}
	if shared == candidates[0] {
		full = true // one candidate, or one that is a prefix of all others
	}
	if len(shared) > len(word) {
		suffix := []rune(shared[len(word):])
		out := append([]rune{}, buf[:pos]...)
		out = append(out, suffix...)
		out = append(out, buf[pos:]...)
		pos += len(suffix)
		buf = out
		if full && len(candidates) == 1 && !strings.HasSuffix(shared, "/") {
			// A finished name: add the space a person would type next.
			out = append([]rune{}, buf[:pos]...)
			out = append(out, ' ')
			out = append(out, buf[pos:]...)
			buf = out
			pos++
		}
		return buf, pos
	}
	// Several candidates and nothing new to add: show the list, keep the line.
	fmt.Fprint(e.out, "\r\n"+strings.Join(candidates, "  ")+"\r\n")
	e.render(prompt, buf, pos)
	return buf, pos
}

// commonPrefix is the longest beginning two strings share.
func commonPrefix(a, b string) string {
	n := len(a)
	if len(b) < n {
		n = len(b)
	}
	i := 0
	for i < n && a[i] == b[i] {
		i++
	}
	return a[:i]
}

// commands lists the executable names on the shell's PATH that start with word.
func (e *editor) commands(word string) []string {
	seen := map[string]bool{}
	var out []string
	path := ""
	if e.getenv != nil {
		path = e.getenv("PATH")
	}
	for _, dir := range strings.Split(path, ":") {
		if dir == "" {
			continue
		}
		entries, err := os.ReadDir(dir)
		if err != nil {
			continue
		}
		for _, ent := range entries {
			name := ent.Name()
			if !strings.HasPrefix(name, word) || seen[name] {
				continue
			}
			info, err := ent.Info()
			if err != nil || info.IsDir() || info.Mode()&0o111 == 0 {
				continue
			}
			seen[name] = true
			out = append(out, name)
		}
	}
	return out
}

// paths lists the files and directories that start with word, seen from the
// shell's directory. Directories carry a trailing slash, so a second Tab
// continues inside them.
func (e *editor) paths(word string) []string {
	dir := "."
	if e.dir != nil {
		if d := e.dir(); d != "" {
			dir = d
		}
	}
	head, base := filepath.Split(word)
	// "~/" is the home directory, as a shell would read it.
	if strings.HasPrefix(word, "~/") {
		home := ""
		if e.getenv != nil {
			home = e.getenv("HOME")
		}
		if home != "" {
			head = strings.TrimPrefix(head, "~/")
			head = filepath.Join(home, head)
			if !strings.HasSuffix(head, "/") {
				head += "/"
			}
		}
	}
	lookup := head
	if lookup == "" {
		lookup = "."
	}
	if !filepath.IsAbs(lookup) {
		lookup = filepath.Join(dir, lookup)
	}
	entries, err := os.ReadDir(lookup)
	if err != nil {
		return nil
	}
	var out []string
	for _, ent := range entries {
		name := ent.Name()
		if !strings.HasPrefix(name, base) {
			continue
		}
		if strings.HasPrefix(name, ".") && !strings.HasPrefix(base, ".") {
			continue
		}
		cand := head + name
		if ent.IsDir() {
			cand += "/"
		}
		out = append(out, cand)
	}
	return out
}
