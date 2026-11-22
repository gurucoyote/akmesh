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
