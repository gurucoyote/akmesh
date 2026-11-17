//go:build linux

package policy

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"unsafe"

	"golang.org/x/sys/unix"
)

// Landlock access bits, in the order the kernel defines them.
const (
	accessExecute    = 1 << 0
	accessWriteFile  = 1 << 1
	accessReadFile   = 1 << 2
	accessReadDir    = 1 << 3
	accessRemoveDir  = 1 << 4
	accessRemoveFile = 1 << 5
	accessMakeChar   = 1 << 6
	accessMakeDir    = 1 << 7
	accessMakeReg    = 1 << 8
	accessMakeSock   = 1 << 9
	accessMakeFifo   = 1 << 10
	accessMakeBlock  = 1 << 11
	accessMakeSym    = 1 << 12
	accessRefer      = 1 << 13
	accessTruncate   = 1 << 14
)

// ruleTypePathBeneath is the only rule type this code uses: a path and the
// rights granted below it.
const ruleTypePathBeneath = 1

type rulesetAttr struct {
	HandledAccessFS      uint64
	HandledAccessNetwork uint64
	Scoped               uint64
}

type pathBeneathAttr struct {
	AllowedAccess uint64
	ParentFD      int32
	pad           uint32
}

// abiVersion asks the kernel which Landlock generation it speaks. Zero means the
// syscall is missing or the kernel was built without Landlock.
func abiVersion() (int, error) {
	ver, _, errno := unix.Syscall(unix.SYS_LANDLOCK_CREATE_RULESET, 0, 0, 1 /* LANDLOCK_CREATE_RULESET_VERSION */)
	if errno != 0 {
		return 0, fmt.Errorf("landlock is not available on this kernel: %v", errno)
	}
	return int(ver), nil
}

// Enforce applies the policy to the current process. From here on every file
// access outside the listed paths fails for this process and for all children,
// enforced by the kernel rather than by our own code.
func (p *Policy) Enforce() error {
	abi, err := abiVersion()
	if err != nil {
		return err
	}
	// Bits that do not exist yet in this ABI cannot be handled; asking for them
	// makes the kernel refuse the whole ruleset.
	handled := uint64(accessExecute | accessWriteFile | accessReadFile | accessReadDir |
		accessRemoveDir | accessRemoveFile | accessMakeChar | accessMakeDir |
		accessMakeReg | accessMakeSock | accessMakeFifo | accessMakeBlock | accessMakeSym)
	if abi >= 2 {
		handled |= accessRefer
	}
	if abi >= 3 {
		handled |= accessTruncate
	}

	attr := rulesetAttr{HandledAccessFS: handled}
	fd, _, errno := unix.Syscall(unix.SYS_LANDLOCK_CREATE_RULESET,
		uintptr(unsafe.Pointer(&attr)), unsafe.Sizeof(attr), 0)
	if errno != 0 {
		return fmt.Errorf("landlock create ruleset: %v", errno)
	}
	defer unix.Close(int(fd))

	// Read only, on the read list.
	readRights := uint64(accessReadFile | accessReadDir)
	writeRights := readRights | accessWriteFile | accessMakeReg | accessMakeDir |
		accessMakeSym | accessRemoveFile | accessRemoveDir
	if abi >= 2 {
		writeRights |= accessRefer
	}
	if abi >= 3 {
		writeRights |= accessTruncate
	}
	execRights := readRights | accessExecute

	added := 0
	for _, dir := range p.read {
		if ok, err := addRule(int(fd), dir, readRights); err != nil {
			return err
		} else if ok {
			added++
		}
	}
	for _, dir := range p.exec {
		if ok, err := addRule(int(fd), dir, execRights); err != nil {
			return err
		} else if ok {
			added++
		}
	}
	for _, dir := range p.write {
		if ok, err := addRule(int(fd), dir, writeRights); err != nil {
			return err
		} else if ok {
			added++
		}
	}
	if added == 0 {
		return fmt.Errorf("policy listed no path that exists; it would lock the process out of every file")
	}

	// Landlock refuses to apply without no_new_privs.
	if err := unix.Prctl(unix.PR_SET_NO_NEW_PRIVS, 1, 0, 0, 0); err != nil {
		return fmt.Errorf("set no_new_privs: %v", err)
	}
	if _, _, errno := unix.Syscall(unix.SYS_LANDLOCK_RESTRICT_SELF, fd, 0, 0); errno != 0 {
		return fmt.Errorf("landlock restrict self: %v", errno)
	}
	return nil
}

// addRule grants rights below one path. A path that does not exist is reported
// and skipped: a policy naming a directory that has not been created yet should
// not silently weaken what is enforced, but it also must not stop the process.
func addRule(rulesetFD int, path string, rights uint64) (bool, error) {
	info, err := os.Stat(path)
	if err != nil {
		if !isSystemPath(path) {
			fmt.Fprintf(os.Stderr, "policy: skipping %s: %v\n", path, err)
		}
		return false, nil
	}
	// A path that resolves into /proc/self/fd points at an inherited
	// descriptor (a pipe, usually), which Landlock cannot take a rule for.
	if resolved, err := filepath.EvalSymlinks(path); err == nil {
		if strings.HasPrefix(resolved, "/proc/self/fd") {
			return false, nil
		}
		path = resolved
	}
	flags := unix.O_PATH | unix.O_CLOEXEC
	if !info.IsDir() {
		// A regular file is only reachable through its parent, so the rule is
		// added on the file itself with O_PATH which the kernel accepts.
		flags = unix.O_PATH | unix.O_CLOEXEC
	}
	fd, err := unix.Open(path, flags, 0)
	if err != nil {
		return false, fmt.Errorf("open %s: %v", path, err)
	}
	defer unix.Close(fd)
	granted := rights
	if !info.IsDir() {
		granted &= accessExecute | accessWriteFile | accessReadFile | accessTruncate
	}
	rule := pathBeneathAttr{AllowedAccess: granted, ParentFD: int32(fd)}
	if _, _, errno := unix.Syscall6(unix.SYS_LANDLOCK_ADD_RULE, uintptr(rulesetFD),
		ruleTypePathBeneath, uintptr(unsafe.Pointer(&rule)), 0, 0, 0); errno != 0 {
		return false, fmt.Errorf("landlock add rule for %s: %v", path, errno)
	}
	return true, nil
}

// isSystemPath keeps the default system directories quiet when one of them is
// absent; a missing path from the policy file itself is worth a warning.
func isSystemPath(path string) bool {
	for _, item := range systemPaths {
		if item == path {
			return true
		}
	}
	return false
}

// Supported reports whether this kernel can enforce a policy, for the policy
// command.
func Supported() (bool, string) {
	abi, err := abiVersion()
	if err != nil {
		return false, err.Error()
	}
	return true, fmt.Sprintf("landlock ABI %d", abi)
}
