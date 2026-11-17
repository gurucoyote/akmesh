package tools

import (
	"bufio"
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/bluekeyes/go-gitdiff/gitdiff"
)

const contextLines = 3

// runDiff prints a unified diff of two files. The hunk headers are computed
// from the actual line counts, because our own patch command (go-gitdiff) is
// strict: a header that announces one line too many is rejected.
func runDiff(args []string, stdin *os.File, stdout, stderr *os.File) int {
	var files []string
	labelA, labelB := "", ""
	unified := true
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "-u", "--unified":
			unified = true
		case "-L", "--label":
			if i+2 >= len(args) {
				return 2
			}
			labelA = args[i+1]
			labelB = args[i+2]
			i += 2
		default:
			if strings.HasPrefix(args[i], "-") && args[i] != "-" {
				fmt.Fprintf(stderr, "diff: unknown option %q\n", args[i])
				return 2
			}
			files = append(files, args[i])
		}
	}
	if len(files) != 2 {
		fmt.Fprintln(stderr, "usage: diff [-L A -L B] FILE1 FILE2")
		return 2
	}
	if !unified {
		fmt.Fprintln(stderr, "diff: only unified output is supported")
		return 2
	}
	if labelA == "" {
		labelA = "a/" + files[0]
	}
	if labelB == "" {
		labelB = "b/" + files[0]
	}
	a, err := readLinesNoNL(files[0])
	if err != nil {
		fmt.Fprintf(stderr, "diff: %v\n", err)
		return 1
	}
	b, err := readLinesNoNL(files[1])
	if err != nil {
		fmt.Fprintf(stderr, "diff: %v\n", err)
		return 1
	}
	ops, err := diffLines(a, b)
	if err != nil {
		fmt.Fprintf(stderr, "diff: %v\n", err)
		return 1
	}
	if len(ops) == 0 {
		return 0
	}
	fmt.Fprintf(stdout, "--- %s\n+++ %s\n", labelA, labelB)
	for _, h := range hunks(ops) {
		fmt.Fprintf(stdout, "@@ -%d,%d +%d,%d @@\n", h.aStart, h.aCount, h.bStart, h.bCount)
		for _, line := range h.lines {
			fmt.Fprintln(stdout, line)
		}
	}
	return 1
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
func diffLines(a, b []string) ([]opLine, error) {
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
			if a[i] == b[j] {
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
		case a[i] == b[j]:
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

// runPatch applies a unified or git patch. Files are rewritten in place.
func runPatch(args []string, stdin *os.File, stdout, stderr *os.File) int {
	strip := 1
	dryRun := false
	var files []string
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "-p":
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
		case "--dry-run":
			dryRun = true
		case "-i", "--input":
			if i+1 >= len(args) {
				return 2
			}
			i++
			files = append(files, args[i])
		default:
			if strings.HasPrefix(args[i], "-") && args[i] != "-" {
				fmt.Fprintf(stderr, "patch: unknown option %q\n", args[i])
				return 2
			}
			files = append(files, args[i])
		}
	}
	var r *os.File = stdin
	if len(files) > 0 {
		f, err := os.Open(files[0])
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
	code := 0
	for _, file := range list {
		name := stripName(file.NewName, strip)
		if name == "" {
			name = stripName(file.OldName, strip)
		}
		if file.IsDelete {
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
