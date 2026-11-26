// winget as one source among the apk repositories.
//
// apk keeps its meaning on Windows by delegating the part it cannot do itself:
// a line "winget" in etc/apk/repositories makes the Windows package manager a
// source beside the real apk repositories. Everything apk does itself stays as
// it is - keys, index, world, scripts - and only names no apk repository knows
// are handed to winget.
//
// The messages name the source and say where the software lands, because a
// person typing "apk add jq" should not have to wonder whether something just
// changed their Windows installation (asked 26.11.2026).
package apkcmd

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// wingetSource is one such line: what was written and what winget calls it.
type wingetSource struct {
	Line   string // "winget" or "winget:msstore", as in the file
	Source string // the value for winget --source
}

// parseWingetLine reports whether a repository line names winget.
func parseWingetLine(u string) (wingetSource, bool) {
	if u == "winget" {
		return wingetSource{Line: u, Source: "winget"}, true
	}
	if rest, ok := strings.CutPrefix(u, "winget:"); ok && rest != "" {
		return wingetSource{Line: u, Source: rest}, true
	}
	return wingetSource{}, false
}

// wingetSourcesOf picks the winget lines out of a repository list.
func wingetSourcesOf(repos []Repo) []wingetSource {
	var out []wingetSource
	for _, r := range repos {
		if src, ok := parseWingetLine(r.URL); ok {
			out = append(out, src)
		}
	}
	return out
}

// apkReposOnly drops the winget lines: they are not something to fetch an
// APKINDEX from.
func apkReposOnly(repos []Repo) []Repo {
	var out []Repo
	for _, r := range repos {
		if _, ok := parseWingetLine(r.URL); ok {
			continue
		}
		out = append(out, r)
	}
	return out
}

// wingetCommand is the program name, a variable so a test can put a stub there.
var wingetCommand = "winget"

// wingetPresent reports whether the package manager is reachable.
func wingetPresent() error {
	if _, err := exec.LookPath(wingetCommand); err != nil {
		return fmt.Errorf("the Windows package manager is not available here (%v)", err)
	}
	return nil
}

// wingetNote is the one shape of message this file writes, so every line about
// winget starts the same way and says whose work it is.
func wingetNote(w io.Writer, format string, args ...any) {
	fmt.Fprintf(w, "apk: winget: "+format+"\n", args...)
}

// runWinget runs the package manager and passes its output through: winget's
// text is localized, so it is never parsed into decisions - the exit code is.
func runWinget(args []string, stdout, stderr io.Writer) int {
	exe, err := exec.LookPath(wingetCommand)
	if err != nil {
		wingetNote(stderr, "cannot start: %v", err)
		return 127
	}
	c := exec.Command(exe, args...)
	c.Stdout = stdout
	c.Stderr = stderr
	c.Stdin = nil
	if err := c.Run(); err != nil {
		var ee *exec.ExitError
		if errors.As(err, &ee) {
			return ee.ExitCode()
		}
		wingetNote(stderr, "%v", err)
		return 1
	}
	return 0
}

// installFlags are the switches that make an unattended install possible and
// honest: agreements accepted, no questions asked.
func installFlags(src wingetSource) []string {
	return []string{
		"--source", src.Source,
		"--exact",
		"--silent",
		"--accept-package-agreements",
		"--accept-source-agreements",
		"--disable-interactivity",
	}
}

// wingetRow is one line of "winget search" output.
type wingetRow struct {
	Name    string
	ID      string
	Version string
}

// parseWingetSearch reads the table winget prints. Columns are separated by two
// spaces at least; the header line and the dashes are localized, so they are
// skipped by looking for the dashed rule.
func parseWingetSearch(out string) []wingetRow {
	var rows []wingetRow
	seenRule := false
	for _, line := range strings.Split(out, "\n") {
		line = strings.TrimRight(line, "\r")
		trimmed := strings.TrimSpace(line)
		if trimmed == "" {
			continue
		}
		if strings.HasPrefix(trimmed, "---") {
			seenRule = true
			continue
		}
		if !seenRule {
			continue
		}
		fields := splitOnGaps(trimmed)
		if len(fields) < 2 {
			continue
		}
		row := wingetRow{Name: fields[0], ID: fields[1]}
		if len(fields) > 2 {
			row.Version = fields[2]
		}
		rows = append(rows, row)
	}
	return rows
}

// splitOnGaps splits on runs of two or more spaces, which is how winget pads its
// table. A single space can belong to a name ("RipGrep GNU").
func splitOnGaps(s string) []string {
	var out []string
	var cur strings.Builder
	gap := 0
	for _, r := range s {
		if r == ' ' {
			gap++
			continue
		}
		if gap >= 2 && cur.Len() > 0 {
			out = append(out, cur.String())
			cur.Reset()
		} else if gap == 1 && cur.Len() > 0 {
			cur.WriteRune(' ')
		}
		gap = 0
		cur.WriteRune(r)
	}
	if cur.Len() > 0 {
		out = append(out, cur.String())
	}
	return out
}

// wingetResolve turns a name a person typed into a package id. An id already
// contains a dot and goes straight through; anything else is searched, and only
// a single unambiguous match is taken. Several matches are listed and the
// command stops - guessing here would install something else than asked.
func wingetResolve(src wingetSource, name string, stdout, stderr io.Writer) (string, int) {
	if strings.Contains(name, ".") {
		return name, 0
	}
	wingetNote(stdout, "searching %s for %q", src.Source, name)
	var buf strings.Builder
	code := runWinget([]string{"search", name, "--source", src.Source,
		"--accept-source-agreements", "--disable-interactivity"}, &buf, stderr)
	if code != 0 {
		return "", code
	}
	var matches []wingetRow
	for _, row := range parseWingetSearch(buf.String()) {
		if strings.EqualFold(row.Name, name) || idTailMatches(row.ID, name) {
			matches = append(matches, row)
		}
	}
	switch len(matches) {
	case 1:
		return matches[0].ID, 0
	case 0:
		rows := parseWingetSearch(buf.String())
		if len(rows) == 0 {
			wingetNote(stderr, "%s: no package of that name", name)
			return "", 1
		}
		wingetNote(stderr, "%s: no exact match; these are the candidates:", name)
		printRows(stderr, rows, 10)
		wingetNote(stderr, "give the id, for example \"apk add %s\"", rows[0].ID)
		return "", 1
	default:
		wingetNote(stderr, "%s: more than one package; give the id:", name)
		printRows(stderr, matches, 10)
		return "", 1
	}
}

// idTailMatches reports whether the last part of an id equals the name,
// so "winget add ripgrep" finds BurntSushi.ripgrep.MSVC only if it is alone.
func idTailMatches(id, name string) bool {
	parts := strings.Split(id, ".")
	if len(parts) == 0 {
		return false
	}
	if strings.EqualFold(parts[len(parts)-1], name) {
		return true
	}
	return len(parts) > 1 && strings.EqualFold(parts[len(parts)-2], name)
}

func printRows(w io.Writer, rows []wingetRow, limit int) {
	for i, row := range rows {
		if i == limit {
			fmt.Fprintf(w, "  ... and %d more\n", len(rows)-limit)
			return
		}
		if row.Version != "" {
			fmt.Fprintf(w, "  %s  %s  %s\n", row.ID, row.Version, row.Name)
			continue
		}
		fmt.Fprintf(w, "  %s  %s\n", row.ID, row.Name)
	}
}

// wingetInstall installs the given names through winget, saying before each one
// where it goes and why apk is not the one writing the files.
func wingetInstall(cmd string, src wingetSource, names []string, stdout, stderr io.Writer) int {
	for _, name := range names {
		id, code := wingetResolve(src, name, stdout, stderr)
		if code != 0 {
			return code
		}
		wingetNote(stdout, "%s is not an apk package; %s installs %s", name, src.Line, id)
		wingetNote(stdout, "this changes this Windows machine (winget's own folders), not the apk root")
		args := append([]string{"install", "--id", id}, installFlags(src)...)
		if code := runWinget(args, stdout, stderr); code != 0 {
			wingetNote(stderr, "%s: winget ended with %d", id, code)
			return code
		}
	}
	return 0
}

// wingetUninstall removes names through winget.
func wingetUninstall(src wingetSource, names []string, stdout, stderr io.Writer) int {
	for _, name := range names {
		id, code := wingetResolve(src, name, stdout, stderr)
		if code != 0 {
			return code
		}
		wingetNote(stdout, "%s was installed by winget, not by apk; %s removes it", name, src.Line)
		if code := runWinget([]string{"uninstall", "--id", id, "--source", src.Source, "--exact", "--disable-interactivity"}, stdout, stderr); code != 0 {
			wingetNote(stderr, "%s: winget ended with %d", id, code)
			return code
		}
	}
	return 0
}

// wingetShow prints what winget knows about a package.
func wingetShow(src wingetSource, names []string, stdout, stderr io.Writer) int {
	for _, name := range names {
		id, code := wingetResolve(src, name, stdout, stderr)
		if code != 0 {
			return code
		}
		wingetNote(stdout, "%s is not an apk package; these are winget's details:", name)
		if code := runWinget([]string{"show", "--id", id, "--source", src.Source, "--exact", "--disable-interactivity"}, stdout, stderr); code != 0 {
			return code
		}
	}
	return 0
}

// wingetSearchPrints asks winget and passes its table through, with a line that
// names the source first.
func wingetSearchPrints(src wingetSource, words []string, stdout, stderr io.Writer) int {
	for _, word := range words {
		wingetNote(stdout, "searching %s for %q (the Windows package manager)", src.Source, word)
		if code := runWinget([]string{"search", word, "--source", src.Source,
			"--accept-source-agreements", "--disable-interactivity"}, stdout, stderr); code != 0 {
			return code
		}
	}
	return 0
}

// wingetUpdate refreshes winget's own source index.
func wingetUpdate(src wingetSource, stdout, stderr io.Writer) int {
	wingetNote(stdout, "refreshing %s (the Windows package manager's index)", src.Line)
	return runWinget([]string{"source", "update", "--source", src.Source, "--disable-interactivity"}, stdout, stderr)
}

// wingetUpgrade upgrades every winget package. Only used when apk was asked for
// it explicitly (--all), because it touches programs apk never installed.
func wingetUpgrade(src wingetSource, stdout, stderr io.Writer) int {
	wingetNote(stdout, "upgrading every package %s knows (this touches programs apk never installed)", src.Line)
	return runWinget([]string{"upgrade", "--all", "--source", src.Source,
		"--silent", "--accept-package-agreements", "--accept-source-agreements", "--disable-interactivity"}, stdout, stderr)
}

// exportedPackage is what "winget export" writes, as far as apk needs it.
type exportedPackage struct {
	Sources []struct {
		Packages []struct {
			ID      string `json:"PackageIdentifier"`
			Version string `json:"Version"`
		} `json:"Packages"`
	} `json:"Sources"`
}

// wingetInstalled lists what winget has installed, through its JSON export: the
// one machine-readable answer it gives without parsing localized text.
func wingetInstalled(src wingetSource, stdout, stderr io.Writer) int {
	dir, err := os.MkdirTemp("", "apk-winget-")
	if err != nil {
		wingetNote(stderr, "%v", err)
		return 1
	}
	defer os.RemoveAll(dir)
	file := filepath.Join(dir, "export.json")
	wingetNote(stdout, "listing what %s installed", src.Line)
	if code := runWinget([]string{"export", "-o", file, "--source", src.Source, "--disable-interactivity"}, io.Discard, stderr); code != 0 {
		return code
	}
	data, err := os.ReadFile(file)
	if err != nil {
		wingetNote(stderr, "%v", err)
		return 1
	}
	var ex exportedPackage
	if err := json.Unmarshal(data, &ex); err != nil {
		wingetNote(stderr, "cannot read the export: %v", err)
		return 1
	}
	n := 0
	for _, s := range ex.Sources {
		for _, p := range s.Packages {
			// winget export carries the id but no version (measured 26.11.2026),
			// so a version is printed only when it is really there.
			if p.Version != "" {
				fmt.Fprintf(stdout, "%s-%s [%s]\n", p.ID, p.Version, src.Line)
			} else {
				fmt.Fprintf(stdout, "%s [%s]\n", p.ID, src.Line)
			}
			n++
		}
	}
	if n == 0 {
		wingetNote(stdout, "nothing installed through this source")
	}
	return 0
}
