//go:build !windows

package sh

import "os"

// elevated reports whether this process runs as root, the unix answer to the
// same question: there the rights are a uid, and 0 is all of them.
func elevated() bool { return os.Geteuid() == 0 }
