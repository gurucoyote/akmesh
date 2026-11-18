//go:build windows

package httpd

import "fmt"

// dropPrivileges says that -u has no meaning here, instead of pretending.
func dropPrivileges(spec string) error {
	if spec == "" {
		return nil
	}
	return fmt.Errorf("httpd: -u is not available on Windows")
}
