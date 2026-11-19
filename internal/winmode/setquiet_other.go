//go:build !windows

package winmode

import "io/fs"

// SetQuiet is a no-op where chmod(2) already wrote the mode.
func SetQuiet(string, fs.FileMode) error { return nil }
