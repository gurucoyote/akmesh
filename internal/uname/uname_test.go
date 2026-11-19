package uname

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

// capture runs the command with stdin, stdout and stderr as real files. The
// test used to live in internal/tools, which left this package at 0 % coverage
// and gremlins blind to its mutants (acmesh-i2t).
func capture(t *testing.T, args []string) (string, string, int) {
	t.Helper()
	dir := t.TempDir()
	inPath := filepath.Join(dir, "stdin")
	if err := os.WriteFile(inPath, nil, 0o600); err != nil {
		t.Fatal(err)
	}
	inF, err := os.Open(inPath)
	if err != nil {
		t.Fatal(err)
	}
	defer inF.Close()
	outPath := filepath.Join(dir, "stdout")
	outF, err := os.Create(outPath)
	if err != nil {
		t.Fatal(err)
	}
	defer outF.Close()
	errPath := filepath.Join(dir, "stderr")
	errF, err := os.Create(errPath)
	if err != nil {
		t.Fatal(err)
	}
	defer errF.Close()
	code := Run(args, inF, outF, errF)
	out, err := os.ReadFile(outPath)
	if err != nil {
		t.Fatal(err)
	}
	errOut, err := os.ReadFile(errPath)
	if err != nil {
		t.Fatal(err)
	}
	return string(out), string(errOut), code
}

// TestFlagsAnswerWithSomething: -o, -p, -i and -v exist on every platform. A
// field the kernel has no answer for reads "unknown" or "-", never an invented
// version, and the command still exits 0.
func TestFlagsAnswerWithSomething(t *testing.T) {
	out, errOut, code := capture(t, []string{"-o"})
	if code != 0 || strings.TrimSpace(out) == "" {
		t.Fatalf("uname -o = %d, %q; want an operating system name", code, out)
	}
	if errOut != "" {
		t.Fatalf("uname -o wrote %q to stderr", errOut)
	}
	for _, flag := range []string{"-p", "-i", "-v"} {
		out, _, code := capture(t, []string{flag})
		if code != 0 {
			t.Fatalf("uname %s exit %d", flag, code)
		}
		if strings.TrimSpace(out) == "" {
			t.Fatalf("uname %s printed nothing; GNU prints unknown or -", flag)
		}
	}
}

// TestKernelNameAndMachine: -s names the kernel and -m the machine, and -a
// carries both. A field name is not a free text: an agent greps "Linux" or
// "x86_64" here.
func TestKernelNameAndMachine(t *testing.T) {
	out, _, code := capture(t, []string{"-s"})
	if code != 0 || strings.TrimSpace(out) == "" {
		t.Fatalf("uname -s = %d, %q", code, out)
	}
	sys := strings.TrimSpace(out)

	out, _, _ = capture(t, []string{"-m"})
	machine := strings.TrimSpace(out)
	if machine == "" {
		t.Fatalf("uname -m printed nothing")
	}

	all, _, code := capture(t, []string{"-a"})
	if code != 0 {
		t.Fatalf("uname -a exit %d", code)
	}
	for _, want := range []string{sys, machine} {
		if !strings.Contains(all, want) {
			t.Fatalf("uname -a = %q misses %q", all, want)
		}
	}
}

// TestReportsTheRealMachine: every field is compared against the fact the
// runtime publishes, not merely against "non-empty". A mutation that renames the
// machine or forgets the GNU/Linux spelling has to die here (acmesh-i2t).
func TestReportsTheRealMachine(t *testing.T) {
	wantSys := map[string]string{"linux": "Linux", "windows": "Windows_NT", "darwin": "Darwin"}[runtime.GOOS]
	out, _, _ := capture(t, []string{"-s"})
	if wantSys != "" && strings.TrimSpace(out) != wantSys {
		t.Fatalf("uname -s = %q, want %q", strings.TrimSpace(out), wantSys)
	}

	wantMachine := map[string]string{"amd64": "x86_64", "arm64": "aarch64", "386": "i686"}[runtime.GOARCH]
	out, _, _ = capture(t, []string{"-m"})
	if wantMachine != "" && strings.TrimSpace(out) != wantMachine {
		t.Fatalf("uname -m = %q, want %q for %s", strings.TrimSpace(out), wantMachine, runtime.GOARCH)
	}

	wantOS := wantSys
	if runtime.GOOS == "linux" {
		wantOS = "GNU/Linux"
	}
	out, _, _ = capture(t, []string{"-o"})
	if wantOS != "" && strings.TrimSpace(out) != wantOS {
		t.Fatalf("uname -o = %q, want %q", strings.TrimSpace(out), wantOS)
	}

	// -p is "unknown" on Linux by GNU's own convention: the kernel does not
	// publish it, and an invented value would be worse.
	if runtime.GOOS == "linux" {
		out, _, _ = capture(t, []string{"-p"})
		if strings.TrimSpace(out) != "unknown" {
			t.Fatalf("uname -p = %q, want unknown like GNU on Linux", strings.TrimSpace(out))
		}
	}

	// -r reads the running kernel's release; it must match the file the kernel
	// publishes, and must not be the "-" placeholder on Linux.
	if runtime.GOOS == "linux" {
		raw, err := os.ReadFile("/proc/sys/kernel/osrelease")
		if err == nil {
			want := strings.TrimSpace(string(raw))
			out, _, _ = capture(t, []string{"-r"})
			if strings.TrimSpace(out) != want {
				t.Fatalf("uname -r = %q, want %q from /proc", strings.TrimSpace(out), want)
			}
		}
	}
}

// TestCombinedShortFlags: -sm is the shape an agent writes, and it must equal
// -s -m.
func TestCombinedShortFlags(t *testing.T) {
	combined, _, code := capture(t, []string{"-sm"})
	if code != 0 {
		t.Fatalf("uname -sm exit %d", code)
	}
	separate, _, _ := capture(t, []string{"-s", "-m"})
	if combined != separate {
		t.Fatalf("-sm = %q, -s -m = %q: the combined form differs", combined, separate)
	}
}

// TestUnknownOptionIsRefused: GNU's wording, its Try line and exit 1
// (acmesh-4qw). Measured on coreutils 9.1.
func TestUnknownOptionIsRefused(t *testing.T) {
	_, errOut, code := capture(t, []string{"--bogus"})
	if code != 1 {
		t.Fatalf("uname --bogus exit %d, want 1", code)
	}
	want := "uname: unrecognized option '--bogus'\nTry 'uname --help' for more information.\n"
	if errOut != want {
		t.Fatalf("stderr %q, want %q", errOut, want)
	}
}
