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
	"time"

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
	// The default is GNU's normal format; -u asks for unified output.
	opts := diffOptions{}
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
		fmt.Fprintln(stderr, "usage: diff [-u] [-r] [-N] [-q] [-i] [-w] [-b] [-L LABEL -L LABEL] FILE1 FILE2\n       without -u the normal format is written, as GNU does")
		return 2
	}
	if opts.labelA == "" {
		opts.labelA = files[0]
	}
	if opts.labelB == "" {
		opts.labelB = files[1]
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
// stamp is the timestamp column of a unified diff header. A file that does not
// exist is the epoch, which is what GNU prints for a side that only -N invented.
func stamp(name string) string {
	when := time.Unix(0, 0).UTC()
	if info, err := os.Stat(name); err == nil {
		when = info.ModTime()
	}
	return when.Format("2006-01-02 15:04:05.000000000 -0700")
}

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
	if !opts.unified {
		// GNU's normal format is the default: "2c2" and the two lines, no
		// header. An agent types "diff a b" and expects this (measured
		// 24.11.2026 against GNU diff 3.10 in the reference image).
		writeNormal(stdout, hs)
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
	// GNU puts the file's timestamp behind the name, separated by a tab, so
	// patch -p0 can find the file without being told its name (measured
	// 23.11.2026: "--- d1.txt\t2026-11-23 19:44:46.581456399 +0000").
	fmt.Fprintf(stdout, "--- %s\t%s\n+++ %s\t%s\n", labelA, stamp(nameA), labelB, stamp(nameB))
	for _, h := range hs {
		fmt.Fprintf(stdout, "@@ -%d,%d +%d,%d @@\n", h.aStart, h.aCount, h.bStart, h.bCount)
		for _, line := range h.lines {
			fmt.Fprintln(stdout, line)
		}
	}
	return 1
}

// numRange writes "N" for a single line and "N,M" for a span, the way GNU
// normal format names a range.
func numRange(from, to int) string {
	if from == to {
		return strconv.Itoa(from)
	}
	return strconv.Itoa(from) + "," + strconv.Itoa(to)
}

// writeNormal prints the change groups of the hunks in GNU's normal format:
// "2c2" with "<" and ">", "2d1" for a deletion, "1a2" for an addition.
func writeNormal(stdout io.Writer, hs []hunk) {
	for _, h := range hs {
		aLine, bLine := h.aStart, h.bStart
		for i := 0; i < len(h.lines); {
			if h.lines[i][0] == ' ' {
				aLine++
				bLine++
				i++
				continue
			}
			var dels, adds []string
			for ; i < len(h.lines) && h.lines[i][0] != ' '; i++ {
				switch h.lines[i][0] {
				case '-':
					dels = append(dels, h.lines[i][1:])
				case '+':
					adds = append(adds, h.lines[i][1:])
				}
			}
			switch {
			case len(dels) > 0 && len(adds) > 0:
				fmt.Fprintf(stdout, "%sc%s\n", numRange(aLine, aLine+len(dels)-1), numRange(bLine, bLine+len(adds)-1))
			case len(dels) > 0:
				fmt.Fprintf(stdout, "%sd%d\n", numRange(aLine, aLine+len(dels)-1), bLine-1)
			default:
				fmt.Fprintf(stdout, "%da%s\n", aLine-1, numRange(bLine, bLine+len(adds)-1))
			}
			for _, d := range dels {
				fmt.Fprintf(stdout, "< %s\n", d)
			}
			if len(dels) > 0 && len(adds) > 0 {
				fmt.Fprintln(stdout, "---")
			}
			for _, a := range adds {
				fmt.Fprintf(stdout, "> %s\n", a)
			}
			aLine += len(dels)
			bLine += len(adds)
		}
	}
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

// headerNames collects the "---" names of a patch, in order. A name of
// /dev/null means "file does not exist yet" and stays empty.
func headerNames(text string) []string {
	var names []string
	var pending string
	for _, line := range strings.Split(text, "\n") {
		switch {
		case strings.HasPrefix(line, "--- "):
			pending = strings.TrimSpace(strings.TrimPrefix(line, "--- "))
			if i := strings.IndexByte(pending, '\t'); i >= 0 {
				pending = pending[:i]
			}
			if pending == "/dev/null" {
				pending = ""
			}
		case strings.HasPrefix(line, "+++ "):
			names = append(names, pending)
			pending = ""
		}
	}
	return names
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
	// The parser behind gitdiff files both names under the post-image name for
	// a plain unified diff ("--- a.txt" / "+++ b.txt" comes back as b.txt
	// twice). patch has to write the pre-image file, so the "---" names are
	// read from the raw patch as well and win (acmesh-bwv).
	raw, err := io.ReadAll(r)
	if err != nil {
		fmt.Fprintf(stderr, "patch: %v\n", err)
		return 1
	}
	oldNames := headerNames(string(raw))
	list, _, err := gitdiff.Parse(bufio.NewReader(strings.NewReader(string(raw))))
	if err != nil {
		fmt.Fprintf(stderr, "patch: %v\n", err)
		return 1
	}
	if len(list) == 0 {
		fmt.Fprintln(stderr, "patch: no patch found in the input")
		return 2
	}
	code := 0
	for idx, file := range list {
		if reverse {
			file = reverseFile(file)
		}
		if idx < len(oldNames) && oldNames[idx] != "" {
			file.OldName, file.NewName = oldNames[idx], file.NewName
		}
		name := target
		if name == "" {
			// patch modifies the file the "---" line names; the "+++" name
			// is the result, which does not exist yet on a create patch.
			// Preferring the new name broke plain `diff a b | patch` once the
			// labels became real file names (acmesh-bwv).
			name = stripName(file.OldName, strip)
			if name == "" {
				name = stripName(file.NewName, strip)
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
