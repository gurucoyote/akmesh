//go:build linux

package shared

import (
	"os"
	"strings"
)

// ReadPasswd maps user ids to names. A missing /etc/passwd only costs the name:
// callers fall back to printing the number.
func ReadPasswd() map[string]string {
	users := map[string]string{}
	data, err := os.ReadFile("/etc/passwd")
	if err != nil {
		return users
	}
	for _, line := range strings.Split(string(data), "\n") {
		parts := strings.Split(line, ":")
		if len(parts) < 3 {
			continue
		}
		users[parts[2]] = parts[0]
	}
	return users
}
