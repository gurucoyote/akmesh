package proc

// The signal helpers are exported for timeout, which has to know the same
// names: two tables would drift apart and one command would accept a signal the
// other refuses.

// NormaliseSignal turns "SIGTERM", "term" or "15" into the canonical name.
func NormaliseSignal(name string) (string, error) { return normaliseSignal(name) }

// SignalNumber returns the number of a signal name, 0 when the platform has no
// such number.
func SignalNumber(name string) int { return signalNumber(name) }

// SignalProcess sends the named signal to one process. Signals a platform has
// no equivalent for are refused instead of being mapped to a kill.
func SignalProcess(pid int, name string) error { return signalProcess(pid, name) }
