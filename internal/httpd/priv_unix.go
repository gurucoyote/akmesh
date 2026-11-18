//go:build !windows

package httpd

import (
	"fmt"
	"os/user"
	"strconv"
	"syscall"
)

// dropPrivileges switches to USER[:GROUP] after the port is bound, which is how
// busybox httpd handles -u. An empty spec changes nothing.
func dropPrivileges(spec string) error {
	if spec == "" {
		return nil
	}
	name, groupName := spec, ""
	for i := 0; i < len(spec); i++ {
		if spec[i] == ':' {
			name, groupName = spec[:i], spec[i+1:]
			break
		}
	}
	u, err := user.Lookup(name)
	if err != nil {
		return fmt.Errorf("httpd: %v", err)
	}
	uid, err := strconv.Atoi(u.Uid)
	if err != nil {
		return err
	}
	gid, err := strconv.Atoi(u.Gid)
	if err != nil {
		return err
	}
	if groupName != "" {
		g, err := user.LookupGroup(groupName)
		if err != nil {
			return fmt.Errorf("httpd: %v", err)
		}
		if gid, err = strconv.Atoi(g.Gid); err != nil {
			return err
		}
	}
	if err := syscall.Setgid(gid); err != nil {
		return fmt.Errorf("httpd: setgid: %v", err)
	}
	if err := syscall.Setuid(uid); err != nil {
		return fmt.Errorf("httpd: setuid: %v", err)
	}
	return nil
}
