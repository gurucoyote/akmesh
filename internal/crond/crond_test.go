package crond

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestParseCrontabKeepsLinesAndEnv(t *testing.T) {
	tab := `# comment
SHELL=/bin/sh
PATH=/usr/bin:/bin

*/5 * * * * echo five
@daily /bin/backup
@every 10s echo ten seconds
0 3 * * * run --a --b
`
	got, err := ParseCrontab(strings.NewReader(tab), "/etc/crontab")
	if err != nil {
		t.Fatal(err)
	}
	if len(got.Env) != 2 || got.Env[1] != "PATH=/usr/bin:/bin" {
		t.Fatalf("env = %v", got.Env)
	}
	want := []struct{ spec, cmd string }{
		{"*/5 * * * *", "echo five"},
		{"@daily", "/bin/backup"},
		{"@every 10s", "echo ten seconds"},
		{"0 3 * * *", "run --a --b"},
	}
	if len(got.Jobs) != len(want) {
		t.Fatalf("%d jobs, want %d: %+v", len(got.Jobs), len(want), got.Jobs)
	}
	for i, w := range want {
		if got.Jobs[i].Spec != w.spec || got.Jobs[i].Command != w.cmd {
			t.Errorf("job %d = %q / %q, want %q / %q", i, got.Jobs[i].Spec, got.Jobs[i].Command, w.spec, w.cmd)
		}
		if got.Jobs[i].Line == 0 {
			t.Errorf("job %d has no line number", i)
		}
	}
	if got.Jobs[0].Env == nil || len(got.Jobs[0].Env) != 2 {
		t.Errorf("job did not inherit the file's variables: %v", got.Jobs[0].Env)
	}
}

func TestParseCrontabRefusesBrokenLines(t *testing.T) {
	cases := []string{
		"* * * echo\n",
		"*/5 * * * *\n",
		"@daily\n",
	}
	for _, c := range cases {
		if _, err := ParseCrontab(strings.NewReader(c), "tab"); err == nil {
			t.Errorf("accepted %q", c)
		}
	}
}

// TestFiveFieldsAndCommand pins the split: a command may start with a number and
// must still be the command, and sub-minute schedules come from @every.
func TestFiveFieldsAndCommand(t *testing.T) {
	spec, cmd, ok := splitSpec("0 0 * * * 5 echo x")
	if !ok {
		t.Fatal("not split")
	}
	if spec != "0 0 * * *" || cmd != "5 echo x" {
		t.Fatalf("got %q / %q", spec, cmd)
	}
	spec, cmd, _ = splitSpec("@every 10s echo x")
	if spec != "@every 10s" || cmd != "echo x" {
		t.Fatalf("@every: got %q / %q", spec, cmd)
	}
}

// TestRunJobReportsExitCode runs a real job through the binary's own shell and
// checks the two markers every cron log needs: start with time, end with code.
func TestRunJobReportsExitCode(t *testing.T) {
	var out, errb bytes.Buffer
	d := &daemon{Stdout: &out, Stderr: &errb, Now: time.Now, Self: "/bin/sh"}
	// /bin/sh -c is what shellWords produces when Self is a shell itself.
	d.Shell = func(command string) []string { return []string{"/bin/sh", "-c", command} }
	d.runJob(Job{Spec: "* * * * *", Command: "echo hallo; exit 3", Source: "test", Line: 7})
	text := out.String()
	if !strings.Contains(text, "start echo hallo; exit 3") {
		t.Errorf("no start marker: %q", text)
	}
	if !strings.Contains(text, "(exit 3)") {
		t.Errorf("no exit code: %q", text)
	}
	if !strings.Contains(text, "hallo") {
		t.Errorf("job output missing: %q", text)
	}
}

// TestJobEnvironmentIsInherited: supercronic's point is that the crontab
// variables reach the job.
func TestJobEnvironmentIsInherited(t *testing.T) {
	var out, errb bytes.Buffer
	d := &daemon{Stdout: &out, Stderr: &errb, Now: time.Now, Self: "/bin/sh"}
	d.Shell = func(command string) []string { return []string{"/bin/sh", "-c", command} }
	d.runJob(Job{Spec: "* * * * *", Command: "echo A=$AUSDERCRONTAB", Source: "test", Line: 1,
		Env: []string{"AUSDERCRONTAB=gesetzt"}})
	if !strings.Contains(out.String(), "A=gesetzt") {
		t.Fatalf("environment not inherited: %q", out.String())
	}
}

// TestBadSpecIsReportedWithLine: a wrong expression must be named, not skipped.
func TestBadSpecIsReportedWithLine(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "crontab")
	content := "99 * * * * echo nein\n"
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
	var out, errb bytes.Buffer
	d := &daemon{Files: []string{path}, Stdout: &out, Stderr: &errb, Now: time.Now, Self: "/bin/sh"}
	sched, _, err := d.start()
	if err != nil {
		t.Fatal(err)
	}
	defer sched.Stop()
	if !strings.Contains(errb.String(), "line 1") {
		t.Fatalf("report without line number: %q", errb.String())
	}
}

// TestCronDirSkipsBackups: cron ignores dotfiles and editor leftovers.
func TestCronDirSkipsBackups(t *testing.T) {
	dir := t.TempDir()
	for _, name := range []string{"job", ".hidden", "job~"} {
		if err := os.WriteFile(filepath.Join(dir, name), []byte("* * * * * true\n"), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	got := CronDir(dir)
	if len(got) != 1 || filepath.Base(got[0]) != "job" {
		t.Fatalf("CronDir = %v", got)
	}
	if CronDir(filepath.Join(dir, "missing")) != nil {
		t.Fatal("a missing directory must give no files, not an error")
	}
}

// TestEverySchedulesSubMinute proves that "@every 10s" is understood by the
// expression parser, not only by our splitter.
func TestEverySchedulesSubMinute(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "crontab")
	if err := os.WriteFile(path, []byte("@every 10s echo tick\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	var out, errb bytes.Buffer
	d := &daemon{Files: []string{path}, Stdout: &out, Stderr: &errb, Self: "/bin/sh"}
	sched, jobs, err := d.start()
	if err != nil {
		t.Fatal(err)
	}
	defer sched.Stop()
	if errb.Len() != 0 {
		t.Fatalf("parser refused @every: %q", errb.String())
	}
	if jobs != 1 || len(sched.Entries()) != 1 {
		t.Fatalf("jobs %d, entries %d, want 1 and 1", jobs, len(sched.Entries()))
	}
}

// TestReloadSeesANewFileInTheDirectory pins the reason a reload exists: a file
// dropped into /etc/cron.d after the start must be picked up. It was not, when
// the file list was frozen at startup.
func TestReloadSeesANewFileInTheDirectory(t *testing.T) {
	dir := t.TempDir()
	var out, errb bytes.Buffer
	d := &daemon{Files: []string{filepath.Join(dir, "crontab")}, Dir: dir, Stdout: &out, Stderr: &errb, Self: "/bin/sh"}
	if err := os.WriteFile(filepath.Join(dir, "crontab"), []byte("@every 5s echo a\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	sched, jobs, err := d.start()
	if err != nil {
		t.Fatal(err)
	}
	if jobs != 1 {
		t.Fatalf("first start: %d jobs", jobs)
	}
	sched.Stop()

	if err := os.WriteFile(filepath.Join(dir, "neu"), []byte("@every 5s echo b\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	sched2, jobs2, err := d.start()
	if err != nil {
		t.Fatal(err)
	}
	defer sched2.Stop()
	if jobs2 != 2 {
		t.Fatalf("after adding a file: %d jobs, want 2", jobs2)
	}
}
