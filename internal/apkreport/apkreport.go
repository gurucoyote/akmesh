// Package apkreport writes one JSON line per installed package: what a host has,
// when, in which channel, and what the last update and health check said. It is
// the base for "last known good version" (ticket acmesh-slyv); the rollback
// watcher reads it.
//
// No host traces: the report is a plain file the caller names. Nothing goes to
// an event log, a registry or a cloud.
package apkreport

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"akmesh/internal/apkcore"
)

// Line is one record. Field names stay short because they end up in a file a
// human may read over ssh, and every line must be a complete JSON object.
type Line struct {
	Zeit     string `json:"zeit"`
	Host     string `json:"host"`
	Kanal    string `json:"kanal,omitempty"`
	Paket    string `json:"paket"`
	Version  string `json:"version"`
	Vorher   string `json:"vorher,omitempty"`
	Ergebnis string `json:"ergebnis"`
	Health   string `json:"health"`
	ApkExit  *int   `json:"apk_exit,omitempty"`
}

type options struct {
	root    string
	out     string
	host    string
	kanal   string
	health  string
	apkExit *int
	quiet   bool
}

// Run is the command: apkreport [options].
func Run(args []string, stdout, stderr io.Writer) int {
	o := options{health: "skip"}
	for i := 0; i < len(args); i++ {
		a := args[i]
		next := func() string {
			if i+1 >= len(args) {
				return ""
			}
			i++
			return args[i]
		}
		switch a {
		case "-p", "--root":
			o.root = next()
		case "-o", "--out":
			o.out = next()
		case "--host":
			o.host = next()
		case "--kanal", "--channel":
			o.kanal = next()
		case "--health":
			o.health = next()
		case "--apk-exit":
			n := 0
			if _, err := fmt.Sscanf(next(), "%d", &n); err != nil {
				fmt.Fprintf(stderr, "apkreport: --apk-exit needs a number\n")
				return 1
			}
			o.apkExit = &n
		case "-q", "--quiet":
			o.quiet = true
		case "-h", "--help":
			usage(stdout)
			return 0
		default:
			if strings.HasPrefix(a, "-") {
				fmt.Fprintf(stderr, "apkreport: unknown option %s\n", a)
				return 1
			}
			fmt.Fprintf(stderr, "apkreport: unexpected argument %q\n", a)
			return 1
		}
	}
	if o.host == "" {
		name, err := os.Hostname()
		if err != nil || name == "" {
			name = "unknown"
		}
		o.host = name
	}
	if o.root == "" {
		o.root = os.Getenv("AKMESH_ROOT")
	}
	if o.health != "ok" && o.health != "fail" && o.health != "skip" {
		fmt.Fprintf(stderr, "apkreport: --health needs ok, fail or skip\n")
		return 1
	}

	pkgs, err := apkcore.ReadInstalled(o.root)
	if err != nil {
		fmt.Fprintf(stderr, "apkreport: %v\n", err)
		return 1
	}
	kanals := worldChannels(o.root)
	vorher := LastGood(o.out)

	now := time.Now().UTC().Format(time.RFC3339)
	lines := make([]Line, 0, len(pkgs))
	for _, p := range pkgs {
		k := o.kanal
		if k == "" {
			k = kanals[p.Name]
		}
		l := Line{
			Zeit:     now,
			Host:     o.host,
			Kanal:    k,
			Paket:    p.Name,
			Version:  p.Version,
			Vorher:   vorher[p.Name],
			Ergebnis: resultFor(o.apkExit),
			Health:   o.health,
			ApkExit:  o.apkExit,
		}
		lines = append(lines, l)
	}
	sort.Slice(lines, func(i, j int) bool { return lines[i].Paket < lines[j].Paket })

	var out io.Writer = stdout
	if o.out != "" {
		if err := os.MkdirAll(filepath.Dir(o.out), 0o755); err != nil {
			fmt.Fprintf(stderr, "apkreport: %v\n", err)
			return 1
		}
		f, err := os.OpenFile(o.out, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644)
		if err != nil {
			fmt.Fprintf(stderr, "apkreport: %v\n", err)
			return 1
		}
		defer f.Close()
		out = f
	}
	w := bufio.NewWriter(out)
	for _, l := range lines {
		body, err := json.Marshal(l)
		if err != nil {
			fmt.Fprintf(stderr, "apkreport: %v\n", err)
			return 1
		}
		w.Write(body)
		w.WriteString("\n")
	}
	if err := w.Flush(); err != nil {
		fmt.Fprintf(stderr, "apkreport: %v\n", err)
		return 1
	}
	if !o.quiet && o.out != "" {
		fmt.Fprintf(stderr, "apkreport: %d Zeilen nach %s\n", len(lines), o.out)
	}
	return 0
}

func usage(w io.Writer) {
	fmt.Fprint(w, `usage: apkreport [-p DIR] [-o FILE] [--host NAME] [--kanal NAME]
                 [--health ok|fail|skip] [--apk-exit N] [-q]

  Writes one JSON line per installed package: time, host, channel, package,
  version, the version before this run, the apk result and the health result.

  -p/--root DIR   target tree (default: / or $AKMESH_ROOT)
  -o/--out FILE   append to this file (default: standard output); the file is
                  also where "vorher" comes from: the last line with health=ok
  --kanal NAME    channel for every line; default: the @tag of /etc/apk/world
  --health X      health result of this run: ok, fail or skip (default skip)
  --apk-exit N    exit code of the apk run that produced this state
`)
}

func resultFor(code *int) string {
	if code == nil {
		return "n/a"
	}
	if *code == 0 {
		return "ok"
	}
	return "fail"
}

// worldChannels maps a package name to the channel tag it was installed from:
// /etc/apk/world carries "pkg@stable" for these.
func worldChannels(root string) map[string]string {
	out := map[string]string{}
	entries, err := apkcore.ReadWorld(root)
	if err != nil {
		return out
	}
	for _, e := range entries {
		// The entry is "pkg", "pkg@testing" or "pkg=1.0.0-r0"; the channel is the
		// @tag, the constraint is not a channel.
		spec := e
		if before, after, ok := strings.Cut(spec, "@"); ok {
			out[apkcore.EntryName(before)] = after
		}
	}
	return out
}

// LastGood reads the last known good version per package out of an existing
// report: the newest line whose health is ok (or skip, which means "no check
// was run", so the state is not known to be bad).
func LastGood(path string) map[string]string {
	out := map[string]string{}
	if path == "" {
		return out
	}
	f, err := os.Open(path)
	if err != nil {
		return out
	}
	defer f.Close()
	sc := bufio.NewScanner(f)
	sc.Buffer(make([]byte, 0, 64*1024), 1024*1024)
	for sc.Scan() {
		var l Line
		if err := json.Unmarshal(sc.Bytes(), &l); err != nil {
			continue
		}
		if l.Health == "fail" {
			continue
		}
		if l.Paket == "" || l.Version == "" {
			continue
		}
		out[l.Paket] = l.Version
	}
	return out
}

// Installed reads the target tree's package list, so other commands (the
// rollback watcher) see the same picture this report writes.
func Installed(root string) ([]*apkcore.Installed, error) {
	return apkcore.ReadInstalled(root)
}
