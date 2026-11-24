// Package apkguard is the rollback watcher decided in acmesh-o6ja: after an
// upgrade it runs the configured health checks, and if they fail it rolls back
// exactly once to the last known good version from the report
// (apkreport, acmesh-slyv). A second failure is reported and nothing more
// happens; a human takes over.
//
// It is a one-shot command, not a daemon: whoever supervises the caller runs it,
// and it leaves no state outside the files it is told to use.
package apkguard

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"akmesh/internal/apkreport"
)

type options struct {
	apk       string
	root      string
	report    string
	state     string
	kanal     string
	health    []string
	dryRun    bool
	noUpgrade bool
	host      string
}

// state remembers that the single automatic rollback was already used. Without
// it a supervisor restarting the command would roll back again and again.
type state struct {
	RollbackUsed bool     `json:"rollback_used"`
	Zeit         string   `json:"zeit,omitempty"`
	Pakete       []string `json:"pakete,omitempty"`
}

// Run is the command: apkguard [options].
func Run(args []string, stdout, stderr io.Writer) int {
	o := options{apk: "apk"}
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
		case "-o", "--report":
			o.report = next()
		case "--apk":
			o.apk = next()
		case "--state":
			o.state = next()
		case "--kanal", "--channel":
			o.kanal = next()
		case "--host":
			o.host = next()
		case "--health":
			o.health = append(o.health, next())
		case "-n", "--dry-run":
			o.dryRun = true
		case "--no-upgrade":
			o.noUpgrade = true
		case "-h", "--help":
			usage(stdout)
			return 0
		default:
			if strings.HasPrefix(a, "--") {
				fmt.Fprintf(stderr, "apkguard: unknown option %s\n", a)
				return 1
			}
			fmt.Fprintf(stderr, "apkguard: unexpected argument %q\n", a)
			return 1
		}
	}
	if o.report == "" {
		fmt.Fprintf(stderr, "apkguard: -o/--report is required: it is where the last known good version comes from\n")
		return 1
	}
	if o.state == "" {
		o.state = o.report + ".state"
	}
	if o.root == "" {
		o.root = os.Getenv("AKMESH_ROOT")
	}

	lastGood := apkreport.LastGood(o.report)
	st := readState(o.state)

	// 1. Upgrade, unless the caller already did it.
	upgradeExit := 0
	if !o.noUpgrade {
		code, err := runApk(o, stderr, "upgrade")
		if err != nil {
			fmt.Fprintf(stderr, "apkguard: apk upgrade: %v\n", err)
			code = 1
		}
		upgradeExit = code
	}

	// 2. What changed, measured from the report before we write the new state.
	changed := changedPackages(o, lastGood)

	// 3. Health checks.
	health := "ok"
	var failed string
	for _, c := range o.health {
		if err := runCheck(c, stderr); err != nil {
			health = "fail"
			failed = c
			break
		}
	}
	if upgradeExit != 0 && health == "ok" {
		health = "fail"
		failed = fmt.Sprintf("apk upgrade exit %d", upgradeExit)
	}

	// 4. Exactly one rollback, and only for packages that actually changed.
	rolledBack := []string{}
	if health == "fail" && len(changed) > 0 {
		if st.RollbackUsed {
			fmt.Fprintf(stderr, "apkguard: health failed (%s) and the automatic rollback was already used; standing still\n", failed)
		} else if o.dryRun {
			fmt.Fprintf(stderr, "apkguard: would roll back %s (dry run)\n", joinLines(changed))
		} else {
			ok := true
			for _, c := range changed {
				ver, have := lastGood[c.Paket]
				if !have {
					continue
				}
				if code, err := runApk(o, stderr, "add", c.Paket+"="+ver); err != nil || code != 0 {
					fmt.Fprintf(stderr, "apkguard: rollback of %s to %s failed\n", c.Paket, ver)
					ok = false
					continue
				}
				rolledBack = append(rolledBack, c.Paket+"="+ver)
			}
			if ok {
				st.RollbackUsed = true
				st.Zeit = time.Now().UTC().Format(time.RFC3339)
				st.Pakete = rolledBack
				if err := writeState(o.state, st); err != nil {
					fmt.Fprintf(stderr, "apkguard: %v\n", err)
				}
			}
		}
	}

	// 5. Report, then stop. The report is what the next run reads.
	healthCode := 0
	if health != "ok" {
		healthCode = 1
	}
	repArgs := []string{"-p", o.root, "-o", o.report, "--health", health, "--apk-exit", fmt.Sprint(upgradeExit), "-q"}
	if o.kanal != "" {
		repArgs = append(repArgs, "--kanal", o.kanal)
	}
	if o.host != "" {
		repArgs = append(repArgs, "--host", o.host)
	}
	if code := apkreport.Run(repArgs, stdout, stderr); code != 0 {
		fmt.Fprintf(stderr, "apkguard: could not write the report\n")
		return 1
	}
	if len(rolledBack) > 0 {
		fmt.Fprintf(stderr, "apkguard: rolled back %s\n", strings.Join(rolledBack, " "))
	}
	return healthCode
}

func joinLines(ls []apkreport.Line) string {
	parts := make([]string, 0, len(ls))
	for _, l := range ls {
		parts = append(parts, l.Paket)
	}
	return strings.Join(parts, " ")
}

func usage(w io.Writer) {
	fmt.Fprint(w, `usage: apkguard -o FILE [--health CMD]... [options]

  Upgrade, check health, roll back once if the check fails, then stop.

  -o/--report FILE   report file (required): read for the last known good
                     version, written after every run
  --health CMD       health command, repeatable; non-zero exit is a failure
  --apk PATH         apk to call for upgrade and rollback (default: apk)
  -p/--root DIR      target tree (default: / or $AKMESH_ROOT)
  --state FILE       remembers that the rollback was used
                     (default: <report>.state); a second run does not roll back
  --kanal NAME       channel recorded in the report
  --no-upgrade       skip the upgrade, only check and roll back
  -n/--dry-run       say what would be rolled back, change nothing
`)
}

// changedPackages compares the install database with the last good versions.
func changedPackages(o options, lastGood map[string]string) []apkreport.Line {
	pkgs, err := apkreport.Installed(o.root)
	if err != nil {
		return nil
	}
	var out []apkreport.Line
	for _, p := range pkgs {
		old, have := lastGood[p.Name]
		if !have || old == p.Version {
			continue
		}
		out = append(out, apkreport.Line{Paket: p.Name, Version: p.Version, Vorher: old})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Paket < out[j].Paket })
	return out
}

func runApk(o options, stderr io.Writer, args ...string) (int, error) {
	full := append([]string{}, args...)
	if o.root != "" {
		full = append([]string{"-p", o.root}, full...)
	}
	if o.dryRun {
		fmt.Fprintf(stderr, "apkguard: would run %s %s\n", o.apk, strings.Join(full, " "))
		return 0, nil
	}
	cmd := exec.Command(o.apk, full...)
	cmd.Stdout = stderr
	cmd.Stderr = stderr
	err := cmd.Run()
	if err == nil {
		return 0, nil
	}
	if ee, ok := err.(*exec.ExitError); ok {
		return ee.ExitCode(), nil
	}
	return 1, err
}

// runCheck runs a health command through sh -c so that the caller may write a
// pipeline. The command is the operator's, not the agent's.
func runCheck(c string, stderr io.Writer) error {
	if strings.TrimSpace(c) == "" {
		return fmt.Errorf("empty health command")
	}
	cmd := exec.Command("sh", "-c", c)
	cmd.Stdout = stderr
	cmd.Stderr = stderr
	err := cmd.Run()
	if err != nil {
		fmt.Fprintf(stderr, "apkguard: health check failed: %s\n", c)
	}
	return err
}

func readState(path string) state {
	var st state
	f, err := os.Open(path)
	if err != nil {
		return st
	}
	defer f.Close()
	json.NewDecoder(bufio.NewReader(f)).Decode(&st)
	return st
}

func writeState(path string, st state) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	body, err := json.Marshal(st)
	if err != nil {
		return err
	}
	return os.WriteFile(path, append(body, '\n'), 0o644)
}
