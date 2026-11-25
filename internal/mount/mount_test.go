//go:build linux

package mount

import (
	"reflect"
	"testing"

	"golang.org/x/sys/unix"
)

// The argv shape is the whole point of this command: WSL's MountDrvfsEntry
// reads Argv[1] as the source, Argv[2] as the target and Argv[4] as the option
// string. A wrong shape there fails at boot with a message that says nothing
// about mount.
func TestDrvfsArgs(t *testing.T) {
	got := drvfsArgs("C:", "/mnt/c", "uid=1000,gid=1000")
	want := []string{"mount.drvfs", "C:", "/mnt/c", "-o", "uid=1000,gid=1000"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("drvfsArgs = %q, want %q", got, want)
	}
}

// Without options the "-o" must be absent, not empty: /init would otherwise see
// Argv[4] missing and drop the option string, which is correct - but an empty
// fourth argument would make Argc 5 with an empty Options pointer.
func TestDrvfsArgsWithoutOptions(t *testing.T) {
	got := drvfsArgs("D:", "/mnt/d", "")
	want := []string{"mount.drvfs", "D:", "/mnt/d"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("drvfsArgs = %q, want %q", got, want)
	}
}

func TestSplitOptions(t *testing.T) {
	flags, data := splitOptions([]string{"ro,nosuid,size=64k"})
	if flags&unix.MS_RDONLY == 0 || flags&unix.MS_NOSUID == 0 {
		t.Errorf("flags = %#x, want MS_RDONLY|MS_NOSUID", flags)
	}
	if data != "size=64k" {
		t.Errorf("data = %q, want size=64k (unknown options belong to the filesystem)", data)
	}
}

// "ro,rw" ends read-write: the last spelling wins, as in mount(8).
func TestSplitOptionsClears(t *testing.T) {
	flags, _ := splitOptions([]string{"ro,rw"})
	if flags&unix.MS_RDONLY != 0 {
		t.Errorf("flags = %#x, want MS_RDONLY cleared by rw", flags)
	}
}
