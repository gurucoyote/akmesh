//go:build windows

package policy

import "errors"

// ErrUnsupported is returned when a policy is configured on a system that cannot
// enforce it. Callers decide whether that is fatal; the shell refuses to start,
// because pretending to be confined is worse than not starting.
var ErrUnsupported = errors.New("a filesystem policy cannot be enforced on windows yet (landlock is a linux feature)")

// Enforce always fails on Windows: there is no kernel facility that restricts
// path access for a process tree the way Landlock does.
func (p *Policy) Enforce() error { return ErrUnsupported }

// Supported reports why enforcement is unavailable.
func Supported() (bool, string) { return false, ErrUnsupported.Error() }

// ApplyToChild is the hook a future Job Object based implementation would fill.
// It exists so the call sites do not need build tags.
func (p *Policy) ApplyToChild() error { return ErrUnsupported }
