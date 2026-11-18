package diffpatch

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"

	"github.com/bluekeyes/go-gitdiff/gitdiff"
)

const contextLines = 3

// diffOptions are the switches an agent passes to diff. Only the ones that
// change the comparison or the output shape are here; the rest of GNU diff
// (side-by-side, colour, other output formats) is refused, not ignored.
type diffOptions struct {
	ignoreCase        bool
	ignoreAllSpace    bool
	ignoreSpaceChange bool
	brief             bool
	recursive         bool
	newFile           bool
	unified           bool
	labelA, labelB    string
}

// normalise is the form a line takes in the comparison; the printed line stays
// the original one, exactly as GNU diff prints it.
func (o diffOptions) normalise(s string) string {
	out := s
	if o.ignoreAllSpace {
		out = strings.Join(strings.Fields(out), "")
	} else if o.ignoreSpaceChange {
		out = strings.Join(strings.Fields(out), " ")
	}
	if o.ignoreCase {
		out = strings.ToLower(out)
	}
	return out
}

// runDiff prints a unified diff of two files or two directory trees. The hunk
// headers are computed from the actual line counts, because our own patch
// command (go-gitdiff) is strict: a header that announces one line too many is
// rejected.
func runDiff(args []string, stdin *os.File, stdout, stderr *os.File) int {
	opts := diffOptions{unified: true}
	haveLabelA := false
	var files []string
	for i := 0; i < len(args); i++ {
		a := args[i]
		switch {
		case a == "--":
			files = append(files, args[i+1:]...)
			i = len(args)
		case a == "-u" || a == "--unified":
			opts.unified = true
		case a == "-L" || a == "--label":
			// GNU takes one label per -L; the first names file A, the second B.
			if i+1 >= len(args) {
				fmt.Fprintln(stderr, "diff: option '-L' needs a label")
				return 2
			}
			i++
			if !haveLabelA {
				opts.labelA, haveLabelA = args[i], true
			} else {
				opts.labelB = args[i]
			}
		case a == "-i" || a == "--ignore-case":
			opts.ignoreCase = true
		case a == "-w" || a == "--ignore-all-space":
			opts.ignoreAllSpace = true
		case a == "-b" || a == "--ignore-space-change":
			opts.ignoreSpaceChange = true
		case a == "-q" || a == "--brief":
			opts.brief = true
		case a == "-r" || a == "--recursive":
			opts.recursive = true
		case a == "-N" || a == "--new-file":
			opts.newFile = true
		case strings.HasPrefix(a, "-") && !strings.HasPrefix(a, "--") && len(a) > 2:
			// Combined short flags such as -ur, -rq or -iN.
			for _, c := range a[1:] {
				switch c {
				case 'u':
					opts.unified = true
				case 'i':
					opts.ignoreCase = true
				case 'w':
					opts.ignoreAllSpace = true
				case 'b':
					opts.ignoreSpaceChange = true
				case 'q':
					opts.brief = true
				case 'r':
					opts.recursive = true
				case 'N':
					opts.newFile = true
				default:
					fmt.Fprintf(stderr, "diff: unknown option %q\n", "-"+string(c))
					return 2
				}
			}
		default:
			if strings.HasPrefix(a, "-") && a != "-" {
				fmt.Fprintf(stderr, "diff: unknown option %q\n", a)
				return 2
			}
			files = append(files, a)
		}
	}
	if len(files) != 2 {
		fmt.Fprintln(stderr, "usage: diff [-u] [-r] [-N] [-q] [-i] [-w] [-b] [-L LABEL -L LABEL] FILE1 FILE2")
		return 2
	}
	if !opts.unified {
		fmt.Fprintln(stderr, "diff: only unified output is supported")
		return 2
	}
	if opts.labelA == "" {
		opts.labelA = "a/" + files[0]
	}
	if opts.labelB == "" {
		opts.labelB = "b/" + files[0]
	}
	infoA, errA := os.Stat(files[0])
	infoB, errB := os.Stat(files[1])
	switch {
	case errA == nil && infoA.IsDir():
		return diffDirectories(files[0], files[1], opts, stdout, stderr)
	case errB == nil && infoB.IsDir():
		return diffDirectories(files[0], files[1], opts, stdout, stderr)
	}
	return diffFiles(files[0], files[1], opts.labelA, opts.labelB, opts, stdout, stderr)
}

// diffFiles compares one pair and returns the GNU exit status: 0 same, 1
// different, 2 trouble.
func diffFiles(nameA, nameB, labelA, labelB string, opts diffOptions, stdout, stderr *os.File) int {
	a, err := readLinesNoNL(nameA)
	if err != nil && !(opts.newFile && os.IsNotExist(err)) {
		fmt.Fprintf(stderr, "diff: %v\n", err)
		return 2
	}
	b, err := readLinesNoNL(nameB)
	if err != nil && !(opts.newFile && os.IsNotExist(err)) {
		fmt.Fprintf(stderr, "diff: %v\n", err)
		return 2
	}
	ops, err := diffLines(a, b, opts)
	if err != nil {
		fmt.Fprintf(stderr, "diff: %v\n", err)
		return 1
	}
	// Equal lines can still fill the script; what matters is whether any hunk
	// came out of it. Checking len(ops) alone printed empty headers for files
	// that differ only in case or spacing.
	hs := hunks(ops)
	if len(hs) == 0 {
		return 0
	}
	if opts.brief {
		fmt.Fprintf(stdout, "Files %s and %s differ\n", nameA, nameB)
		return 1
	}
	if opts.recursive {
		// GNU names the file pair before each diff so a reader can tell which
		// files the hunks belong to.
		flags := "diff -r"
		if opts.newFile {
			flags += " -N"
		}
		fmt.Fprintf(stdout, "%s %s %s\n", flags, nameA, nameB)
	}
	fmt.Fprintf(stdout, "--- %s\n+++ %s\n", labelA, labelB)
	for _, h := range hs {
		fmt.Fprintf(stdout, "@@ -%d,%d +%d,%d @@\n", h.aStart, h.aCount, h.bStart, h.bCount)
		for _, line := range h.lines {
			fmt.Fprintln(stdout, line)
		}
	}
	return 1
}

// diffDirectories walks both trees in one sorted pass, so the output order does
// not depend on the file system. A name that exists on one side only is
// "Only in ..." unless -N turns the missing side into an empty file.
func diffDirectories(dirA, dirB string, opts diffOptions, stdout, stderr *os.File) int {
	seen := map[string]bool{}
	for _, root := range []string{dirA, dirB} {
		entries, err := os.ReadDir(root)
		if err != nil {
			fmt.Fprintf(stderr, "diff: %v\n", err)
			return 2
		}
		for _, e := range entries {
			seen[e.Name()] = true
		}
	}
	names := make([]string, 0, len(seen))
	for name := range seen {
		names = append(names, name)
	}
	sort.Strings(names)

	code := 0
	for _, name := range names {
		pathA, pathB := filepath.Join(dirA, name), filepath.Join(dirB, name)
		infoA, errA := os.Stat(pathA)
		infoB, errB := os.Stat(pathB)
		switch {
		case errA == nil && errB == nil && infoA.IsDir() && infoB.IsDir():
			if c := diffDirectories(pathA, pathB, opts, stdout, stderr); c > code {
				code = c
			}
			continue
		case errA == nil && errB == nil && infoA.IsDir() != infoB.IsDir():
			fmt.Fprintf(stderr, "diff: %s and %s: one is a directory, the other a file\n", pathA, pathB)
			code = 2
			continue
		case (errA != nil || errB != nil) && !opts.newFile:
			only := dirA
			if errA != nil {
				only = dirB
			}
			fmt.Fprintf(stdout, "Only in %s: %s\n", only, name)
			if code < 1 {
				code = 1
			}
			continue
		}
		if !opts.recursive && (errA != nil || errB != nil) {
			fmt.Fprintf(stdout, "Only in %s: %s\n", dirA, name)
			if code < 1 {
				code = 1
			}
			continue
		}
		if c := diffFiles(pathA, pathB, pathA, pathB, opts, stdout, stderr); c > code {
			code = c
		}
	}
	return code
}

type opKind int

const (
	opEqual opKind = iota
	opDelete
	opInsert
)

type opLine struct {
	kind opKind
	text string
}

// diffLines produces a line edit script with a plain longest-common-subsequence
// table. Files with more than about 20 million cell pairs are refused instead
// of eating the memory.
func diffLines(a, b []string, opts diffOptions) ([]opLine, error) {
	// The comparison runs on normalised copies; the script keeps the original
	// lines so -i and -w change only what counts as equal.
	normA := make([]string, len(a))
	for i, s := range a {
		normA[i] = opts.normalise(s)
	}
	normB := make([]string, len(b))
	for i, s := range b {
		normB[i] = opts.normalise(s)
	}
	n, m := len(a), len(b)
	if n*m > 20000000 {
		return nil, fmt.Errorf("input too large for this diff (%d x %d lines)", n, m)
	}
	// table[i][j] = length of the LCS of a[i:] and b[j:]
	table := make([][]int32, n+1)
	for i := range table {
		table[i] = make([]int32, m+1)
	}
	for i := n - 1; i >= 0; i-- {
		for j := m - 1; j >= 0; j-- {
			if normA[i] == normB[j] {
				table[i][j] = table[i+1][j+1] + 1
				continue
			}
			if table[i+1][j] >= table[i][j+1] {
				table[i][j] = table[i+1][j]
			} else {
				table[i][j] = table[i][j+1]
			}
		}
	}
	var ops []opLine
	i, j := 0, 0
	for i < n && j < m {
		switch {
		case normA[i] == normB[j]:
			ops = append(ops, opLine{opEqual, a[i]})
			i++
			j++
		case table[i+1][j] >= table[i][j+1]:
			ops = append(ops, opLine{opDelete, a[i]})
			i++
		default:
			ops = append(ops, opLine{opInsert, b[j]})
			j++
		}
	}
	for ; i < n; i++ {
		ops = append(ops, opLine{opDelete, a[i]})
	}
	for ; j < m; j++ {
		ops = append(ops, opLine{opInsert, b[j]})
	}
	return ops, nil
}

type hunk struct {
	aStart, aCount int
	bStart, bCount int
	lines          []string
}

// hunks groups the edit script into unified hunks with context. Two changes
// belong to the same hunk when they are at most 2*context lines apart.
func hunks(ops []opLine) []hunk {
	var changed []int
	for i, op := range ops {
		if op.kind != opEqual {
			changed = append(changed, i)
		}
	}
	if len(changed) == 0 {
		return nil
	}
	var groups [][2]int
	start := changed[0]
	last := changed[0]
	for _, idx := range changed[1:] {
		if idx-last > 2*contextLines {
			groups = append(groups, [2]int{start, last})
			start = idx
		}
		last = idx
	}
	groups = append(groups, [2]int{start, last})

	out := make([]hunk, 0, len(groups))
	for _, g := range groups {
		from := g[0] - contextLines
		if from < 0 {
			from = 0
		}
		to := g[1] + contextLines
		if to > len(ops)-1 {
			to = len(ops) - 1
		}
		h := hunk{}
		aLine, bLine := 1, 1
		for k := 0; k < from; k++ {
			if ops[k].kind != opInsert {
				aLine++
			}
			if ops[k].kind != opDelete {
				bLine++
			}
		}
		h.aStart, h.bStart = aLine, bLine
		for k := from; k <= to; k++ {
			switch ops[k].kind {
			case opEqual:
				h.aCount++
				h.bCount++
				h.lines = append(h.lines, " "+ops[k].text)
			case opDelete:
				h.aCount++
				h.lines = append(h.lines, "-"+ops[k].text)
			case opInsert:
				h.bCount++
				h.lines = append(h.lines, "+"+ops[k].text)
			}
		}
		out = append(out, h)
	}
	return out
}

// runPatch applies a unified or git patch. The GNU forms are understood:
//
//	patch [options] [ORIGINALFILE [PATCHFILE]]
//
// so both `patch -p1 < x.diff` and `patch w.txt < x.diff` reach the same code.
// Files are rewritten in place, a dry run says what would happen.
func runPatch(args []string, stdin *os.File, stdout, stderr *os.File) int {
	strip := 0
	dryRun := false
	reverse := false
	forward := false
	target := ""
	patchFile := ""
	for i := 0; i < len(args); i++ {
		a := args[i]
		if len(a) > 2 && strings.HasPrefix(a, "-p") && a[2] != '-' {
			// Attached form, patch -p1.
			n, err := strconv.Atoi(a[2:])
			if err != nil || n < 0 {
				fmt.Fprintln(stderr, "patch: bad -p value")
				return 2
			}
			strip = n
			continue
		}
		switch a {
		case "-R", "--reverse":
			reverse = true
			continue
		case "-N", "--forward":
			// Skip a patch that was already applied instead of failing on it.
			forward = true
			continue
		case "-f", "--force":
			// We never ask an interactive question, so -f changes nothing else.
			continue
		case "--dry-run":
			dryRun = true
			continue
		case "-p", "--strip":
			if i+1 >= len(args) {
				return 2
			}
			i++
			n, err := strconv.Atoi(args[i])
			if err != nil || n < 0 {
				fmt.Fprintln(stderr, "patch: bad -p value")
				return 2
			}
			strip = n
			continue
		case "-i", "--input":
			if i+1 >= len(args) {
				return 2
			}
			i++
			patchFile = args[i]
			continue
		}
		if strings.HasPrefix(a, "-") && a != "-" {
			fmt.Fprintf(stderr, "patch: unknown option %q\n", a)
			return 2
		}
		switch {
		case target == "":
			target = a
		case patchFile == "":
			patchFile = a
		default:
			fmt.Fprintf(stderr, "patch: extra operand %q\n", a)
			return 2
		}
	}
	var r *os.File = stdin
	if patchFile != "" {
		f, err := os.Open(patchFile)
		if err != nil {
			fmt.Fprintf(stderr, "patch: %v\n", err)
			return 1
		}
		defer f.Close()
		r = f
	}
	list, _, err := gitdiff.Parse(bufio.NewReader(r))
	if err != nil {
		fmt.Fprintf(stderr, "patch: %v\n", err)
		return 1
	}
	if len(list) == 0 {
		fmt.Fprintln(stderr, "patch: no patch found in the input")
		return 2
	}
	code := 0
	for _, file := range list {
		if reverse {
			file = reverseFile(file)
		}
		name := target
		if name == "" {
			name = stripName(file.NewName, strip)
			if name == "" {
				name = stripName(file.OldName, strip)
			}
			// Without an explicit -p, GNU falls back to the file name alone
			// when the full path is not there. That makes a/foo patches apply
			// after `cd` into the source directory.
			if strip == 0 && name != "" {
				if _, err := os.Stat(name); err != nil {
					if base := stripName(name, len(strings.Split(strings.ReplaceAll(name, "\\", "/"), "/"))-1); base != "" {
						if _, err := os.Stat(base); err == nil {
							name = base
						}
					}
				}
			}
		}
		if file.IsDelete {
			if name == "" {
				fmt.Fprintln(stderr, "patch: cannot delete without a file name")
				code = 1
				continue
			}
			if dryRun {
				fmt.Fprintf(stdout, "would delete %s\n", name)
				continue
			}
			if err := os.Remove(name); err != nil {
				fmt.Fprintf(stderr, "patch: %v\n", err)
				code = 1
			}
			continue
		}
		src, err := os.Open(name)
		if err != nil {
			fmt.Fprintf(stderr, "patch: %v\n", err)
			code = 1
			continue
		}
		var out strings.Builder
		if err := gitdiff.Apply(&out, src, file); err != nil {
			// With -N a patch that is reversed or already applied is skipped
			// with GNU's wording, instead of being reported as broken input.
			if forward {
				if _, seekErr := src.Seek(0, io.SeekStart); seekErr == nil {
					var probe strings.Builder
					if gitdiff.Apply(&probe, src, reverseFile(file)) == nil {
						fmt.Fprintf(stdout, "Reversed (or previously applied) patch detected!  Skipping patch.\n")
						src.Close()
						code = 1
						continue
					}
				}
			}
			fmt.Fprintf(stderr, "patch: %s: %v\n", name, err)
			src.Close()
			code = 1
			continue
		}
		src.Close()
		if dryRun {
			fmt.Fprintf(stdout, "would patch %s (%d bytes)\n", name, out.Len())
			continue
		}
		info, err := os.Stat(name)
		mode := os.FileMode(0o644)
		if err == nil {
			mode = info.Mode().Perm()
		}
		if err := os.WriteFile(name, []byte(out.String()), mode); err != nil {
			fmt.Fprintf(stderr, "patch: %v\n", err)
			code = 1
		}
	}
	return code
}

// reverseFile swaps the two sides of a parsed patch, which is what patch -R
// needs: old and new names, old and new line numbers, and add against delete.
func reverseFile(f *gitdiff.File) *gitdiff.File {
	out := *f
	out.OldName, out.NewName = f.NewName, f.OldName
	out.OldMode, out.NewMode = f.NewMode, f.OldMode
	out.IsNew, out.IsDelete = f.IsDelete, f.IsNew
	out.BinaryFragment, out.ReverseBinaryFragment = f.ReverseBinaryFragment, f.BinaryFragment
	out.TextFragments = make([]*gitdiff.TextFragment, len(f.TextFragments))
	for i, frag := range f.TextFragments {
		nf := *frag
		nf.OldPosition, nf.NewPosition = frag.NewPosition, frag.OldPosition
		nf.OldLines, nf.NewLines = frag.NewLines, frag.OldLines
		nf.LinesAdded, nf.LinesDeleted = frag.LinesDeleted, frag.LinesAdded
		nf.Lines = make([]gitdiff.Line, len(frag.Lines))
		for j, line := range frag.Lines {
			switch line.Op {
			case gitdiff.OpAdd:
				line.Op = gitdiff.OpDelete
			case gitdiff.OpDelete:
				line.Op = gitdiff.OpAdd
			}
			nf.Lines[j] = line
		}
		out.TextFragments[i] = &nf
	}
	return &out
}

func stripName(name string, count int) string {
	if name == "" {
		return ""
	}
	parts := strings.Split(strings.ReplaceAll(name, "\\", "/"), "/")
	if count >= len(parts) {
		return ""
	}
	return strings.Join(parts[count:], "/")
}

func readLinesNoNL(name string) ([]string, error) {
	data, err := os.ReadFile(name)
	if err != nil {
		return nil, err
	}
	text := string(data)
	if text == "" {
		return nil, nil
	}
	text = strings.TrimSuffix(text, "\n")
	text = strings.TrimSuffix(text, "\r")
	return strings.Split(text, "\n"), nil
}
