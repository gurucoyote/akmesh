//go:build !windows

package akinit

import (
	"bytes"
	"os/exec"
	"strings"
	"sync"
	"syscall"
	"testing"
	"time"
)

func TestParseInittab(t *testing.T) {
	tab := `# comment
::sysinit:/etc/init.d/rcS
tty1::respawn:/sbin/getty 38400 tty1

job1:12345:once:/bin/echo hallo
::shutdown:/bin/umount -a -r
`
	entries, err := ParseInittab(strings.NewReader(tab))
	if err != nil {
		t.Fatal(err)
	}
	want := []struct {
		act action
		cmd string
	}{
		{sysinit, "/etc/init.d/rcS"},
		{respawn, "/sbin/getty 38400 tty1"},
		{once, "/bin/echo hallo"},
		{shutdown, "/bin/umount -a -r"},
	}
	if len(entries) != len(want) {
		t.Fatalf("got %d entries, want %d: %v", len(entries), len(want), entries)
	}
	for i, w := range want {
		if entries[i].Action != w.act || entries[i].Command != w.cmd {
			t.Errorf("entry %d = %v, want %s %q", i, entries[i], w.act, w.cmd)
		}
	}
	// The runlevel field is accepted and ignored, the id kept for messages.
	if entries[2].ID != "job1" {
		t.Errorf("id = %q, want job1", entries[2].ID)
	}
}

func TestParseInittabRefusesWhatItCannotRun(t *testing.T) {
	cases := map[string]string{
		"unknown action": "::booting:/bin/sh\n",
		"too few fields": "::respawn\n",
		"empty command":  "::respawn:   \n",
		"three fields":   "x:y:respawn\n",
	}
	for name, tab := range cases {
		if _, err := ParseInittab(strings.NewReader(tab)); err == nil {
			t.Errorf("%s: accepted %q", name, tab)
		}
	}
}

// TestRunOnceReturnsTheStatus: without a command that runs forever, init must
// hand back the service's exit code.
func TestRunOnceReturnsTheStatus(t *testing.T) {
	var out, errb bytes.Buffer
	code := Run("init", []string{"-c", writeTab(t, "::once:/bin/sh -c 'exit 7'\n")}, &out, &errb)
	if code != 7 {
		t.Fatalf("exit %d, want 7 (stderr %q)", code, errb.String())
	}
}

func TestSysinitRunsBeforeTheServices(t *testing.T) {
	dir := t.TempDir()
	tab := writeTab(t, "::sysinit:/bin/sh -c 'printf sysinit >> "+dir+"/order'\n"+
		"::once:/bin/sh -c 'printf once >> "+dir+"/order'\n")
	var out, errb bytes.Buffer
	if code := Run("init", []string{"-c", tab}, &out, &errb); code != 0 {
		t.Fatalf("exit %d, stderr %q", code, errb.String())
	}
	got := readFile(t, dir+"/order")
	if got != "sysinitonce" {
		t.Fatalf("order = %q, want sysinitonce", got)
	}
}

// TestRespawnRestarts gives the honest proof of the duty that matters: a service
// that ends is started again.
func TestRespawnRestarts(t *testing.T) {
	dir := t.TempDir()
	tab := writeTab(t, "::respawn:/bin/sh -c 'echo x >> "+dir+"/runs; exec sleep 0.1'\n")
	r := New(mustParse(t, tab), &bytes.Buffer{}, &bytes.Buffer{})
	r.Grace = 500 * time.Millisecond

	done := make(chan int, 1)
	go func() { done <- r.Run() }()

	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		if len(readFile(t, dir+"/runs")) >= 3 {
			break
		}
		time.Sleep(50 * time.Millisecond)
	}
	if n := len(readFile(t, dir+"/runs")); n < 3 {
		t.Fatalf("service ran %d times, want at least 3", n)
	}
	// SIGTERM must end it, even though a child is running.
	if err := syscall.Kill(syscall.Getpid(), syscall.SIGTERM); err != nil {
		t.Fatal(err)
	}
	select {
	case code := <-done:
		if code != 0 {
			t.Fatalf("exit %d after SIGTERM, want 0", code)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("init did not end after SIGTERM")
	}
}

// TestNoZombies: three children end at once; after they are reaped, none may be
// left as a Z line of THIS process. It is the same check the container probe
// does with ps, only through /proc directly.
func TestNoZombies(t *testing.T) {
	tab := writeTab(t, "::once:/bin/sh -c 'exit 0'\n::once:/bin/sh -c 'true'\n::once:/bin/sh -c 'exit 3'\n")
	var out, errb bytes.Buffer
	// The last service ended with 3, and init hands that back: with every
	// service gone there is nothing else to report.
	if code := Run("init", []string{"-c", tab}, &out, &errb); code != 3 {
		t.Fatalf("exit %d, want 3 (the last service), stderr %q", code, errb.String())
	}
	if z := zombiesOf(t); z != 0 {
		t.Fatalf("%d zombies left after init returned", z)
	}
}

// TestEnsureRespawnWaitsWhenAServiceKeepsDying pins the backoff, so a broken
// service cannot become a fork bomb.
func TestEnsureRespawnWaitsWhenAServiceKeepsDying(t *testing.T) {
	var sleeps []time.Duration
	var mu sync.Mutex
	r := New([]entry{{ID: "x", Action: respawn, Command: "/bin/true"}}, &bytes.Buffer{}, &bytes.Buffer{})
	r.RespawnDelay = 100 * time.Millisecond
	r.RespawnMax = time.Second
	r.sleep = func(d time.Duration) {
		mu.Lock()
		sleeps = append(sleeps, d)
		mu.Unlock()
	}
	// A service that never starts: spawn fails.
	r.spawn = func(entry) (*exec.Cmd, error) { return nil, errFake }
	r.mu.Lock()
	r.fastFails["/bin/true"] = 3
	r.mu.Unlock()
	r.ensureRespawn()
	mu.Lock()
	defer mu.Unlock()
	if len(sleeps) == 0 {
		t.Fatal("ensureRespawn did not wait before the third retry")
	}
	if sleeps[0] < 400*time.Millisecond {
		t.Fatalf("first wait %s, want the growing delay", sleeps[0])
	}
}
