// Package cmdlist prints the command list the binary answers when it is called
// with a name it does not carry. The list used to be one flat alphabet from
// gobusybox; here it is ours: the groups, their order and their wording.
//
// Why grouped: an agent that mistypes a name reads this text, and a flat list of
// a hundred lines puts the document commands between "df" and "echo". The group
// is the second thing it learns about the tool, after the failure itself.
//
// The classification is a table, not a guess from the name. A name that is not
// in the table lands in "Other", and TestEveryCommandIsPlaced turns that into a
// failure for every command we build, so a new command forces a decision
// instead of quietly appearing at the end.
package cmdlist

import (
	"fmt"
	"io"
	"sort"
	"strings"
)

// The group titles, in the order the list prints them. Documents come first:
// that is the block this binary is asked about most, and the one where a wrong
// guess costs the most (cat on a PDF puts binary into the agent's context).
const (
	Documents = "Documents (any2md reads PDF, docx, EPUB, RTF or HTML by its bytes; the *2md names are the same reader with the format fixed):"
	Files     = "Files and text:"
	System    = "System:"
	Network   = "Network:"
	Shell     = "Shell:"
	Other     = "Other:"
)

// order is the order of the blocks in the output.
var order = []string{Documents, Files, System, Network, Shell, Other}

// groups maps a command name to its block. Adding a command means adding a line
// here; forgetting it fails TestEveryCommandIsPlaced.
var groups = map[string]string{
	// Documents: one reader (any2md) and the names that pin it, plus the writers
	// and the HTML readers that cannot go through Markdown.
	"any2md":    Documents,
	"pdf2md":    Documents,
	"docx2md":   Documents,
	"epub2md":   Documents,
	"rtf2md":    Documents,
	"html2md":   Documents,
	"md2txt-l":  Documents,
	"md2pdf":    Documents,
	"md2docx":   Documents,
	"md2epub":   Documents,
	"html2docx": Documents,
	"html2epub": Documents,
	"epub2html": Documents,
	"pdf2html":  Documents,

	// Shell.
	"sh":   Shell,
	"gosh": Shell,

	// Files and text.
	"awk":       Files,
	"base64":    Files,
	"basename":  Files,
	"cat":       Files,
	"chmod":     Files,
	"cmp":       Files,
	"comm":      Files,
	"cp":        Files,
	"cpio":      Files,
	"cut":       Files,
	"dd":        Files,
	"diff":      Files,
	"dirname":   Files,
	"echo":      Files,
	"file":      Files,
	"find":      Files,
	"grep":      Files,
	"gzip":      Files,
	"head":      Files,
	"hexdump":   Files,
	"iconv":     Files,
	"jq":        Files,
	"ln":        Files,
	"ls":        Files,
	"md5sum":    Files,
	"mkdir":     Files,
	"mktemp":    Files,
	"more":      Files,
	"mv":        Files,
	"nl":        Files,
	"patch":     Files,
	"readlink":  Files,
	"realpath":  Files,
	"rg":        Files,
	"rm":        Files,
	"sed":       Files,
	"seq":       Files,
	"sha1sum":   Files,
	"sha256sum": Files,
	"sha512sum": Files,
	"shasum":    Files,
	"shuf":      Files,
	"sort":      Files,
	"split":     Files,
	"stat":      Files,
	"strings":   Files,
	"sum":       Files,
	"tac":       Files,
	"tail":      Files,
	"tar":       Files,
	"tee":       Files,
	"touch":     Files,
	"tr":        Files,
	"truncate":  Files,
	"uniq":      Files,
	"wc":        Files,
	"xargs":     Files,
	"yes":       Files,

	// System.
	"certinfo":  Files, // a certificate is a file
	"date":      System,
	"df":        System,
	"du":        System,
	"env":       System,
	"false":     System,
	"free":      System,
	"hostname":  System,
	"id":        System,
	"kill":      System,
	"killall":   System,
	"man":       System,
	"mkcert":    System,
	"policy":    System,
	"printenv":  System,
	"ps":        System,
	"pwd":       System,
	"sleep":     System,
	"sync":      System,
	"time":      System,
	"timeout":   System,
	"tracedeps": System,
	"true":      System,
	"uname":     System,
	"uptime":    System,
	"verify":    System,
	"which":     System,

	// Network.
	"curl":   Network,
	"httpd":  Network,
	"netcat": Network,
	"ping":   Network,
	"scp":    Network,
	"ssh":    Network,
	"wget":   Network,
}

// ByGroup returns the names per block, with the blocks in printing order and an
// empty block left out. A name that is not in the table is in Other, so the list
// never loses a command because someone forgot to classify it.
func ByGroup(names []string) (titles []string, blocks [][]string) {
	per := map[string][]string{}
	for _, n := range names {
		n = strings.TrimSpace(n)
		if n == "" {
			continue
		}
		g, ok := groups[n]
		if !ok {
			g = Other
		}
		per[g] = append(per[g], n)
	}
	for _, title := range order {
		block := per[title]
		if len(block) == 0 {
			continue
		}
		sort.Strings(block)
		titles = append(titles, title)
		blocks = append(blocks, block)
	}
	return titles, blocks
}

// Print writes the whole list: one line per name, a blank line between blocks,
// so that a screenreader reads a block as one thing and does not run the
// commands together.
func Print(w io.Writer, names []string) error {
	titles, blocks := ByGroup(names)
	if _, err := fmt.Fprintln(w, "Supported commands are:"); err != nil {
		return err
	}
	for i, title := range titles {
		if i > 0 {
			if _, err := fmt.Fprintln(w); err != nil {
				return err
			}
		}
		if _, err := fmt.Fprintln(w, title); err != nil {
			return err
		}
		for _, n := range blocks[i] {
			if _, err := fmt.Fprintf(w, " - %s\n", n); err != nil {
				return err
			}
		}
	}
	return nil
}

// Unplaced reports the names the table does not know. The build gate uses it:
// every command we ship has to have a decided block.
func Unplaced(names []string) []string {
	var out []string
	for _, n := range names {
		n = strings.TrimSpace(n)
		if n == "" {
			continue
		}
		if _, ok := groups[n]; !ok {
			out = append(out, n)
		}
	}
	sort.Strings(out)
	return out
}
