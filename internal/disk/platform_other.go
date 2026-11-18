//go:build !linux && !windows

package disk

// absPath keeps the name as given where the platform has no absolute paths to
// resolve; the callers only use it to report a name.
func absPath(p string) (string, error) { return p, nil }
