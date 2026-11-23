package disk

import (
	"fmt"
	"os"
	"sort"
	"strconv"
	"strings"
)

// humanBytes formats a size the way df -h and du -h do: 1.5K, 23M, 1.2G.
// Units are powers of 1024, and a value below ten of its unit keeps one
// decimal, so the column stays readable.
func humanBytes(n int64) string {
	units := []string{"", "K", "M", "G", "T", "P"}
	if n < 1024 {
		return fmt.Sprintf("%d", n)
	}
	value := float64(n)
	idx := 0
	for value >= 1024 && idx < len(units)-1 {
		value /= 1024
		idx++
	}
	if value < 10 {
		return fmt.Sprintf("%.1f%s", value, units[idx])
	}
	return fmt.Sprintf("%.0f%s", value, units[idx])
}

// sizeFormatter returns a function for the size columns of one command,
// depending on -h or an explicit unit.
func sizeFormatter(human bool, unit int64) func(int64) string {
	if human {
		return humanBytes
	}
	if unit <= 0 {
		unit = 1024
	}
	return func(n int64) string { return fmt.Sprintf("%d", n/unit) }
}

// parseUnitFlags reads the size switches shared by df, du and free.
// It returns human=true for -h, or a unit in bytes for -k/-m/-g, and the
// remaining arguments.
// splitShortFlags expands combined short options the way getopt does, because
// agents write -sh and -la without thinking: -sh becomes -s -h. A flag that
// needs a value takes the rest of the cluster as that value (-d2 is -d 2,
// -t ext4 is left as it is). Long options and a lone "-" stay untouched.
func splitShortFlags(args []string, takesValue string) []string {
	out := make([]string, 0, len(args))
	for _, a := range args {
		if len(a) < 3 || a[0] != '-' || a[1] == '-' {
			out = append(out, a)
			continue
		}
		cluster := a[1:]
		for i := 0; i < len(cluster); i++ {
			c := cluster[i]
			if strings.IndexByte(takesValue, c) >= 0 {
				if i+1 < len(cluster) {
					out = append(out, "-"+string(c), cluster[i+1:])
				} else {
					out = append(out, "-"+string(c))
				}
				break
			}
			out = append(out, "-"+string(c))
		}
	}
	return out
}

func parseUnitFlags(args []string) (human bool, unit int64, rest []string, err error) {
	unit = 1024 // default is 1K blocks, as in df and du
	for _, a := range args {
		switch a {
		case "-h", "--human-readable":
			human = true
		case "-k", "--kilo":
			human, unit = false, 1024
		case "-m", "--mega":
			human, unit = false, 1024*1024
		case "-g", "--giga":
			human, unit = false, 1024*1024*1024
		case "-b", "--bytes":
			human, unit = false, 1
		default:
			rest = append(rest, a)
		}
	}
	return human, unit, rest, nil
}

// memInfo is one row of free.
type memInfo struct {
	Total     int64
	Used      int64
	Free      int64
	Shared    int64
	BuffCache int64
	Available int64
	Known     bool // false means the platform cannot tell (Windows swap detail)
}

// runFree prints memory and swap usage.
func runFree(args []string, stdin *os.File, stdout, stderr *os.File) int {
	args = splitShortFlags(args, "")
	total, wide := false, false
	var kept []string
	for _, a := range args {
		switch a {
		case "-t", "--total":
			total = true
		case "-w", "--wide":
			wide = true
		default:
			kept = append(kept, a)
		}
	}
	human, unit, rest, _ := parseUnitFlags(kept)
	for _, a := range rest {
		if strings.HasPrefix(a, "-") {
			fmt.Fprintf(stderr, "free: unknown option %q\n", a)
			return 2
		}
		fmt.Fprintf(stderr, "free: unexpected argument %q\n", a)
		return 2
	}
	mem, swap, err := readMemory()
	if err != nil {
		fmt.Fprintf(stderr, "free: %v\n", err)
		return 1
	}
	format := sizeFormatter(human, unit)
	// The header uses the same layout as the rows, so the columns line up in
	// both modes. -w widens by one column; buffers and cache stay one number
	// because /proc/meminfo is read as a pair here.
	width := 11
	if wide {
		width = 13
	}
	field := "%" + fmt.Sprintf("%d", width) + "s"
	fmt.Fprintf(stdout, "%-7s "+field+" "+field+" "+field+" "+field+" "+field+" "+field+"\n",
		"", "total", "used", "free", "shared", "buff/cache", "available")
	row := func(label string, m memInfo) {
		cells := []string{label}
		values := []int64{m.Total, m.Used, m.Free, m.Shared, m.BuffCache, m.Available}
		for _, v := range values {
			if v < 0 {
				cells = append(cells, "-")
				continue
			}
			cells = append(cells, format(v))
		}
		fmt.Fprintf(stdout, "%-7s "+field+" "+field+" "+field+" "+field+" "+field+" "+field+"\n",
			cells[0], cells[1], cells[2], cells[3], cells[4], cells[5], cells[6])
	}
	row("Mem:", mem)
	row("Swap:", swap)
	if total {
		// GNU's -t line: Mem plus Swap, field by field.
		sum := mem
		sum.Total += swap.Total
		sum.Used += swap.Used
		sum.Free += swap.Free
		sum.Available += swap.Available
		if mem.Shared > 0 {
			sum.Shared = mem.Shared
		}
		if mem.BuffCache > 0 {
			sum.BuffCache = mem.BuffCache
		}
		row("Total:", sum)
	}
	return 0
}

// diskRow is one line of df.
type diskRow struct {
	Filesystem string
	Type       string
	Size       int64
	Used       int64
	Avail      int64
	UsePercent int
	Inodes     int64
	IUsed      int64
	IFree      int64
	MountedOn  string
}

// runDf prints file system usage. -h is what people ask for first, so it is
// supported from the start.
func runDf(args []string, stdin *os.File, stdout, stderr *os.File) int {
	human, unit, rest, _ := parseUnitFlags(splitShortFlags(args, "t"))
	inodes, showType, posix := false, false, false
	var paths []string
	for _, a := range rest {
		switch a {
		case "-i", "--inodes":
			inodes = true
			continue
		case "-T", "--print-type":
			showType = true
			continue
		case "-P", "--portability":
			posix = true
			continue
		}
		if strings.HasPrefix(a, "-") && a != "-" && !strings.HasPrefix(a, "--") {
			// combined short options such as -iP
			ok := true
			for _, c := range a[1:] {
				switch c {
				case 'i':
					inodes = true
				case 'T':
					showType = true
				case 'P':
					posix = true
				default:
					ok = false
				}
			}
			if ok {
				continue
			}
		}
		if strings.HasPrefix(a, "-") && a != "-" {
			fmt.Fprintf(stderr, "df: unknown option %q\n", a)
			return 2
		}
		paths = append(paths, a)
	}
	rows, err := readFileSystems()
	if err != nil {
		fmt.Fprintf(stderr, "df: %v\n", err)
		return 1
	}
	var missing []string
	if len(paths) > 0 {
		rows, missing = filterFileSystems(rows, paths)
		for _, p := range missing {
			fmt.Fprintf(stderr, "df: %s: not on a mounted file system\n", p)
		}
	}
	format := sizeFormatter(human, unit)

	// Column titles follow GNU: -P renames the size column to 1024-blocks and
	// the percentage to Capacity, -i switches the whole table to inodes.
	sizeLabel := "1K-blocks"
	if posix {
		sizeLabel = "1024-blocks"
	}
	header := []string{"Filesystem"}
	if showType {
		header = append(header, "Type")
	}
	if inodes {
		header = append(header, "Inodes", "IUsed", "IFree", "IUse%")
	} else if posix {
		header = append(header, sizeLabel, "Used", "Available", "Capacity")
	} else {
		header = append(header, sizeLabel, "Used", "Avail", "Use%")
	}
	header = append(header, "Mounted on")
	printDfRow(stdout, header, true)

	for _, r := range rows {
		cells := []string{r.Filesystem}
		if showType {
			cells = append(cells, dashIfEmpty(r.Type))
		}
		if inodes {
			used := r.IUsed
			percent := 0
			if r.Inodes > 0 {
				percent = int(used * 100 / r.Inodes)
			}
			cells = append(cells, countOrDash(r.Inodes), countOrDash(used), countOrDash(r.IFree), fmt.Sprintf("%d%%", percent))
		} else if posix {
			cells = append(cells, blockCells(r, format, true)...)
		} else {
			cells = append(cells, blockCells(r, format, false)...)
		}
		cells = append(cells, r.MountedOn)
		printDfRow(stdout, cells, false)
	}
	if len(missing) > 0 {
		// A caller that asked for a path must not read an empty table as an
		// empty disk.
		return 1
	}
	return 0
}

// blockCells is the size part of one df row. -P always reports blocks, so the
// formatting of -h does not apply there.
func blockCells(r diskRow, format func(int64) string, posix bool) []string {
	if posix {
		return []string{
			fmt.Sprintf("%d", r.Size/1024),
			fmt.Sprintf("%d", r.Used/1024),
			fmt.Sprintf("%d", r.Avail/1024),
			fmt.Sprintf("%d%%", r.UsePercent),
		}
	}
	return []string{format(r.Size), format(r.Used), format(r.Avail), fmt.Sprintf("%d%%", r.UsePercent)}
}

func printDfRow(stdout *os.File, cells []string, header bool) {
	if len(cells) == 0 {
		return
	}
	line := fmt.Sprintf("%-24s", cells[0])
	for i, c := range cells[1:] {
		last := i == len(cells)-2
		if last {
			line += " " + c
			continue
		}
		line += fmt.Sprintf(" %9s", c)
	}
	fmt.Fprintln(stdout, line)
}

func countOrDash(n int64) string {
	if n < 0 {
		return "-"
	}
	return fmt.Sprintf("%d", n)
}

func dashIfEmpty(s string) string {
	if s == "" {
		return "-"
	}
	return s
}

// filterFileSystems keeps the rows that cover the given paths and reports the
// paths no mount point covers.
func filterFileSystems(rows []diskRow, paths []string) (out []diskRow, missing []string) {
	for _, p := range paths {
		abs, err := absPath(p)
		if err != nil {
			missing = append(missing, p)
			continue
		}
		best := -1
		for i, r := range rows {
			mount := r.MountedOn
			if !strings.HasSuffix(mount, string(os.PathSeparator)) {
				mount += string(os.PathSeparator)
			}
			if strings.HasPrefix(abs+string(os.PathSeparator), mount) {
				if best < 0 || len(r.MountedOn) > len(rows[best].MountedOn) {
					best = i
				}
			}
		}
		if best >= 0 {
			out = append(out, rows[best])
			continue
		}
		missing = append(missing, p)
	}
	return out, missing
}

// runDu sums the usage of directory trees.
//
//	du [-h] [-s] [-a] [-c] [-k|-m|-g] [PATH...]
func runDu(args []string, stdin *os.File, stdout, stderr *os.File) int {
	human, unit, rest, _ := parseUnitFlags(splitShortFlags(args, "d"))
	rest = splitShortFlags(rest, "d")
	summary := false
	all := false
	grandTotal := false
	maxDepth := -1
	var paths []string
	for i := 0; i < len(rest); i++ {
		a := rest[i]
		switch {
		case a == "-d" || a == "--max-depth":
			if i+1 >= len(rest) {
				fmt.Fprintln(stderr, "du: option '--max-depth' requires an argument")
				return 2
			}
			i++
			n, err := strconv.Atoi(rest[i])
			if err != nil || n < 0 {
				fmt.Fprintf(stderr, "du: invalid maximum depth %q\n", rest[i])
				return 2
			}
			maxDepth = n
			continue
		case strings.HasPrefix(a, "--max-depth="):
			v := strings.TrimPrefix(a, "--max-depth=")
			n, err := strconv.Atoi(v)
			if err != nil || n < 0 {
				fmt.Fprintf(stderr, "du: invalid maximum depth %q\n", v)
				return 2
			}
			maxDepth = n
			continue
		case strings.HasPrefix(a, "-d") && len(a) > 2:
			n, err := strconv.Atoi(a[2:])
			if err != nil || n < 0 {
				fmt.Fprintf(stderr, "du: invalid maximum depth %q\n", a[2:])
				return 2
			}
			maxDepth = n
			continue
		}
		switch a {
		case "-s", "--summarize":
			summary = true
		case "-a", "--all":
			all = true
		case "-c", "--total":
			grandTotal = true
		default:
			if strings.HasPrefix(a, "-") {
				fmt.Fprintf(stderr, "du: unknown option %q\n", a)
				return 2
			}
			paths = append(paths, a)
		}
	}
	if len(paths) == 0 {
		paths = []string{"."}
	}
	format := sizeFormatter(human, unit)
	seen := map[fileKey]bool{}
	var total int64
	code := 0
	for _, root := range paths {
		info, err := os.Lstat(root)
		if err != nil {
			fmt.Fprintf(stderr, "du: %v\n", err)
			code = 1
			continue
		}
		if !info.IsDir() {
			size := usageOf(info, root)
			fmt.Fprintf(stdout, "%s\t%s\n", format(size), root)
			total += size
			continue
		}
		size, err := walkUsage(stdout, root, format, summary, all, seen, 0, maxDepth)
		if err != nil {
			fmt.Fprintf(stderr, "du: %v\n", err)
			code = 1
		}
		// The path itself is always reported; -s only suppresses the children,
		// which is how GNU du behaves.
		fmt.Fprintf(stdout, "%s\t%s\n", format(size), root)
		total += size
	}
	if grandTotal {
		fmt.Fprintf(stdout, "%s\ttotal\n", format(total))
	}
	return code
}

// walkUsage sums one tree depth first and prints each directory (and each file
// with -a). Hard links are counted once, which is what GNU du does.
func walkUsage(stdout *os.File, root string, format func(int64) string, summary, all bool, seen map[fileKey]bool, depth, maxDepth int) (int64, error) {
	entries, err := os.ReadDir(root)
	if err != nil {
		return 0, err
	}
	sort.Slice(entries, func(i, j int) bool { return entries[i].Name() < entries[j].Name() })
	var total int64
	for _, e := range entries {
		full, err := joinPath(root, e.Name())
		if err != nil {
			continue
		}
		info, err := os.Lstat(full)
		if err != nil {
			continue
		}
		if info.IsDir() {
			size, err := walkUsage(stdout, full, format, summary, all, seen, depth+1, maxDepth)
			if err != nil {
				continue
			}
			total += size
			// -d N prints children up to depth N; the root itself is depth 0.
			if !summary && (maxDepth < 0 || depth+1 <= maxDepth) {
				fmt.Fprintf(stdout, "%s\t%s\n", format(size), full)
			}
			continue
		}
		size := usageOf(info, full)
		if key, ok := keyOf(info); ok {
			if seen[key] {
				continue
			}
			seen[key] = true
		}
		total += size
		if all && !summary && (maxDepth < 0 || depth+1 <= maxDepth) {
			fmt.Fprintf(stdout, "%s\t%s\n", format(size), full)
		}
	}
	return total, nil
}

// fileKey identifies a file for hard link detection.
type fileKey struct {
	Dev uint64
	Ino uint64
}
