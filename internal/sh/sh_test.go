package sh

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// writeFile is a helper: a profile or script with the given content.
func writeFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

// withProfiles points the startup files at a temporary directory, so the test
// never touches the real /etc/profile, and restores them afterwards.
func withProfiles(t *testing.T) (etc, home string) {
	t.Helper()
	dir := t.TempDir()
	etc = filepath.Join(dir, "profile")
	home = filepath.Join(dir, "home")
	if err := os.MkdirAll(home, 0o755); err != nil {
		t.Fatal(err)
	}
	oldEtc, oldHome := etcProfile, homeEnv
	etcProfile = etc
	homeEnv = func(key string) string {
		if key == "HOME" {
			return home
		}
		return ""
	}
	t.Cleanup(func() { etcProfile, homeEnv = oldEtc, oldHome })
	return etc, home
}

// TestLoginReadsEtcProfileThenUserProfile is the point of acmesh-4g7: a login
// shell sees /etc/profile first and the user's profile after it. The order
// matters, because the user's file is allowed to override what etc set.
func TestLoginReadsEtcProfileThenUserProfile(t *testing.T) {
	etc, home := withProfiles(t)
	writeFile(t, etc, "echo etc\nPATH=/from-etc\n")
	writeFile(t, filepath.Join(home, ".profile"), "echo user\nPATH=/from-user\n")

	var out, errb bytes.Buffer
	code := RunAs("-sh", []string{"-c", `echo "path=$PATH"`}, strings.NewReader(""), &out, &errb)

	if code != 0 {
		t.Fatalf("exit %d, stderr %q", code, errb.String())
	}
	got := out.String()
	if !strings.Contains(got, "etc") || !strings.Contains(got, "user") {
		t.Fatalf("profiles did not run in order, output %q", got)
	}
	if strings.Index(got, "etc") > strings.Index(got, "user") {
		t.Fatalf("etc profile must run before the user profile, output %q", got)
	}
	if !strings.Contains(got, "path=/from-user") {
		t.Fatalf("user profile must win, output %q", got)
	}
}

// TestNonLoginDoesNotReadProfiles: a plain "sh -c" is not a login shell. If it
// read the profiles, every script call would re-run them.
func TestNonLoginDoesNotReadProfiles(t *testing.T) {
	etc, home := withProfiles(t)
	writeFile(t, etc, "echo etc\n")
	writeFile(t, filepath.Join(home, ".profile"), "echo user\n")

	var out, errb bytes.Buffer
	code := RunAs("sh", []string{"-c", "echo done"}, strings.NewReader(""), &out, &errb)
	if code != 0 {
		t.Fatalf("exit %d, stderr %q", code, errb.String())
	}
	if got := out.String(); got != "done\n" {
		t.Fatalf("output %q, want just done", got)
	}
}

// TestBashNameReadsBashProfile: when the shell is called as bash, the bash
// convention applies (.bash_profile), not ash's .profile. Reading both would
// run the same setup twice.
func TestBashNameReadsBashProfile(t *testing.T) {
	_, home := withProfiles(t)
	writeFile(t, filepath.Join(home, ".bash_profile"), "echo from-bash\n")
	writeFile(t, filepath.Join(home, ".profile"), "echo from-ash\n")

	var out, errb bytes.Buffer
	if code := RunAs("bash", []string{"-l", "-c", "true"}, strings.NewReader(""), &out, &errb); code != 0 {
		t.Fatalf("exit %d, stderr %q", code, errb.String())
	}
	got := out.String()
	if !strings.Contains(got, "from-bash") || strings.Contains(got, "from-ash") {
		t.Fatalf("want only .bash_profile, output %q", got)
	}
}

// TestMissingProfileIsNotAnError: a container has no /etc/profile at all, and
// the shell still has to start.
func TestMissingProfileIsNotAnError(t *testing.T) {
	etc, _ := withProfiles(t)
	os.Remove(etc)

	var out, errb bytes.Buffer
	code := RunAs("-sh", []string{"-c", "echo ok"}, strings.NewReader(""), &out, &errb)
	if code != 0 {
		t.Fatalf("exit %d, stderr %q", code, errb.String())
	}
	if strings.Contains(errb.String(), "profile") {
		t.Fatalf("missing profile reported as error: %q", errb.String())
	}
}

// TestProfileExitStopsTheStartup: "exit" in /etc/profile ends the session, as
// it does in ash and bash.
func TestProfileExitStopsTheStartup(t *testing.T) {
	etc, _ := withProfiles(t)
	writeFile(t, etc, "echo closing\nexit 7\n")

	var out, errb bytes.Buffer
	code := RunAs("-sh", []string{"-c", "echo reached"}, strings.NewReader(""), &out, &errb)
	if code != 7 {
		t.Fatalf("exit %d, want 7 (stderr %q)", code, errb.String())
	}
	if strings.Contains(out.String(), "reached") {
		t.Fatalf("command ran after exit in profile: %q", out.String())
	}
}

// TestInteractiveReadsLinesAndPrintsPrompt: without a script the shell has to
// stay open and run one statement per line - today it swallowed stdin to EOF.
func TestInteractiveReadsLinesAndPrintsPrompt(t *testing.T) {
	withProfiles(t)
	var out, errb bytes.Buffer
	code := RunAs("sh", []string{"-i"}, strings.NewReader("echo eins\necho zwei\n"), &out, &errb)
	if code != 0 {
		t.Fatalf("exit %d, stderr %q", code, errb.String())
	}
	got := out.String()
	if !strings.Contains(got, "eins") || !strings.Contains(got, "zwei") {
		t.Fatalf("both lines must run, output %q stderr %q", got, errb.String())
	}
	if !strings.Contains(got, "$ ") && !strings.Contains(got, "# ") {
		t.Fatalf("no prompt in output %q", got)
	}
}

// TestInteractiveKeepsGoingAfterAnError: a failing command in an interactive
// shell does not end the session.
func TestInteractiveKeepsGoingAfterAnError(t *testing.T) {
	withProfiles(t)
	var out, errb bytes.Buffer
	in := "command_that_does_not_exist\necho still-here\n"
	code := RunAs("sh", []string{"-i"}, strings.NewReader(in), &out, &errb)
	if code != 0 {
		t.Fatalf("exit %d, stderr %q", code, errb.String())
	}
	if !strings.Contains(out.String(), "still-here") {
		t.Fatalf("shell stopped after an error, output %q stderr %q", out.String(), errb.String())
	}
}

// TestInteractivePS1FromProfile: the prompt comes from PS1 when the profile
// sets one; the default is "$ " (or "# " for root).
func TestInteractivePS1FromProfile(t *testing.T) {
	etc, _ := withProfiles(t)
	writeFile(t, etc, "PS1='ak> '\n")

	var out, errb bytes.Buffer
	code := RunAs("-sh", []string{"-i"}, strings.NewReader("echo x\n"), &out, &errb)
	if code != 0 {
		t.Fatalf("exit %d, stderr %q", code, errb.String())
	}
	if !strings.Contains(out.String(), "ak> ") {
		t.Fatalf("PS1 from profile not used, output %q", out.String())
	}
}

// TestUnsupportedOptionStillFails: accepting -l must not turn the shell into
// one that swallows every typo.
func TestUnsupportedOptionStillFails(t *testing.T) {
	var out, errb bytes.Buffer
	if code := RunAs("sh", []string{"-Z"}, strings.NewReader(""), &out, &errb); code != 2 {
		t.Fatalf("exit %d, want 2", code)
	}
}

// TestInteractiveKeepsShellState: variables set on one line must still be there
// on the next. A reset between lines would silently wipe them.
func TestInteractiveKeepsShellState(t *testing.T) {
	withProfiles(t)
	var out, errb bytes.Buffer
	code := RunAs("sh", []string{"-i"}, strings.NewReader("x=5\necho x=$x\n"), &out, &errb)
	if code != 0 {
		t.Fatalf("exit %d, stderr %q", code, errb.String())
	}
	if !strings.Contains(out.String(), "x=5") {
		t.Fatalf("state lost between lines, output %q", out.String())
	}
}

// TestHelpEndsZero: the help audit asks every command for --help and -h and
// fails on a non-zero exit; a help text that ends with 2 is not help.
func TestHelpEndsZero(t *testing.T) {
	for _, flag := range []string{"--help", "-h"} {
		var out, errb bytes.Buffer
		if code := RunAs("sh", []string{flag}, strings.NewReader(""), &out, &errb); code != 0 {
			t.Fatalf("%s exit %d, want 0", flag, code)
		}
		if !strings.Contains(out.String(), "usage: sh") {
			t.Fatalf("%s printed %q, want a usage line", flag, out.String())
		}
	}
}

// TestPromptEscapesAreExpanded: a profile sets PS1 to "\u@\h:\w\$"; printing
// that literally is what a shell must not do.
func TestPromptEscapesAreExpanded(t *testing.T) {
	t.Setenv("USER", "martin")
	got := expandPrompt(`\u@\h:\w\$`)
	if strings.Contains(got, "\\") {
		t.Fatalf("escapes not expanded: %q", got)
	}
	if !strings.HasPrefix(got, "martin@") {
		t.Fatalf("user missing: %q", got)
	}
	if !strings.HasSuffix(got, "#") && !strings.HasSuffix(got, "$") {
		t.Fatalf("prompt does not end in a prompt sign: %q", got)
	}
	if got := expandPrompt(`\[\]x`); got != "x" {
		t.Fatalf("readline markers kept: %q", got)
	}
}

// TestDollarZeroIsTheInvokedName: the dispatcher sets argv[0] to the binary, so
// $0 came out as "gosh" in every shell. ash uses the name it was called with
// and marks a login shell with a leading dash (acmesh-51b).
func TestDollarZeroIsTheInvokedName(t *testing.T) {
	var out, errb strings.Builder
	RunAs("sh", []string{"-c", `echo "$0"`}, strings.NewReader(""), &out, &errb)
	if got := strings.TrimSpace(out.String()); got != "sh" {
		t.Fatalf("$0 = %q, want sh", got)
	}
	out.Reset()
	RunAs("-sh", []string{"-c", `echo "$0"`}, strings.NewReader(""), &out, &errb)
	if got := strings.TrimSpace(out.String()); got != "-sh" {
		t.Fatalf("login $0 = %q, want -sh", got)
	}
}

// TestReadReadsInTheSession: read used to find nothing because the session and
// the shell each had their own reader (acmesh-51b). A pipe is an *os.File, so
// this is the same path a terminal takes.
func TestReadReadsInTheSession(t *testing.T) {
	pr, pw, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	defer pr.Close()
	if _, err := pw.WriteString("read a line\nhallo welt\necho A=$a line=$line\nexit\n"); err != nil {
		t.Fatal(err)
	}
	pw.Close()
	var out, errb strings.Builder
	RunAs("sh", []string{"-i"}, pr, &out, &errb)
	if got := out.String(); !strings.Contains(got, "A=hallo line=welt") {
		t.Fatalf("read found nothing: %q (stderr %q)", got, errb.String())
	}
}

func TestSystemProfilePrefersTheNamedFile(t *testing.T) {
	// The portable Windows bundle has no /etc/profile; the launcher names its
	// own profile through AKMESH_PROFILE.
	dir := t.TempDir()
	named := filepath.Join(dir, "profile.sh")
	if err := os.WriteFile(named, []byte("MYVAR=from-named\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	oldEtc := etcProfile
	etcProfile = filepath.Join(dir, "no-such-profile")
	t.Setenv("AKMESH_PROFILE", named)
	t.Cleanup(func() { etcProfile = oldEtc })
	if got := systemProfile(); got != named {
		t.Fatalf("systemProfile: got %q, want %q", got, named)
	}
}

func TestSystemProfileFallsBackToEtcProfile(t *testing.T) {
	dir := t.TempDir()
	etc := filepath.Join(dir, "profile")
	if err := os.WriteFile(etc, []byte("MYVAR=from-etc\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	oldEtc := etcProfile
	etcProfile = etc
	t.Setenv("AKMESH_PROFILE", "")
	t.Cleanup(func() { etcProfile = oldEtc })
	if got := systemProfile(); got != etc {
		t.Fatalf("systemProfile: got %q, want %q", got, etc)
	}
}

// On Windows there is no USER in the environment, only USERNAME; the shell used
// to call the session "root" while it ran as an ordinary user (26.11.2026).
func TestFillUserEnvTakesTheWindowsName(t *testing.T) {
	env := fillUserEnv([]string{"USERNAME=Spernau", "HOME=C:\\Users\\Spernau"})
	got := map[string]string{}
	for _, kv := range env {
		if k, v, ok := strings.Cut(kv, "="); ok {
			got[k] = v
		}
	}
	if got["USER"] != "Spernau" {
		t.Fatalf("USER = %q, want Spernau (%v)", got["USER"], env)
	}
	if got["LOGNAME"] != "Spernau" {
		t.Fatalf("LOGNAME = %q, want Spernau", got["LOGNAME"])
	}
}

func TestFillUserEnvKeepsAnExistingUser(t *testing.T) {
	env := fillUserEnv([]string{"USER=martin", "USERNAME=Spernau"})
	for _, kv := range env {
		if strings.HasPrefix(kv, "USER=") && kv != "USER=martin" {
			t.Fatalf("USER was overwritten: %q", kv)
		}
		if strings.HasPrefix(kv, "LOGNAME=") && kv != "LOGNAME=martin" {
			t.Fatalf("LOGNAME does not match USER: %q", kv)
		}
	}
}

// The prompt's \u has to name the Windows user too, not "root".
func TestPromptExpansionUsesTheWindowsName(t *testing.T) {
	if old, had := os.LookupEnv("USER"); had {
		os.Unsetenv("USER")
		t.Cleanup(func() { os.Setenv("USER", old) })
	}
	t.Setenv("USERNAME", "Spernau")
	if got := expandPromptIn(`\u$ `, ""); got != "Spernau$ " {
		t.Fatalf("prompt = %q, want %q", got, "Spernau$ ")
	}
	// The sign depends on the uid of the test process (# as root, $ otherwise).
	if got := defaultPrompt("/tmp", "", "/home/x"); !strings.HasPrefix(got, "\x1b]0;") || !strings.Contains(got, "\nSpernau") {
		t.Fatalf("default prompt = %q", got)
	}
}

// Windows has no root: an elevated token is all "als Administrator" gives you.
// The sign is the only place a session can say so.
func TestPromptSignFollowsTheRights(t *testing.T) {
	old := isElevated
	t.Cleanup(func() { isElevated = old })

	isElevated = func() bool { return true }
	if got := expandPromptIn(`\$ `, ""); got != "# " {
		t.Fatalf("elevated \\$ = %q, want %q", got, "# ")
	}
	if got := defaultPrompt("/tmp", "martin", "/home/martin"); !strings.HasSuffix(got, "martin# ") {
		t.Fatalf("elevated prompt = %q", got)
	}

	isElevated = func() bool { return false }
	if got := expandPromptIn(`\$ `, ""); got != "$ " {
		t.Fatalf("plain \\$ = %q, want %q", got, "$ ")
	}
	if got := defaultPrompt("/tmp", "martin", "/home/martin"); !strings.HasSuffix(got, "martin$ ") {
		t.Fatalf("plain prompt = %q", got)
	}
}
