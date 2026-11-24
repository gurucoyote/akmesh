package sh

import (
	"context"
	"os"
	"os/signal"
	"sync"
)

// interrupter keeps Ctrl-C inside the session. Without a handler Go's default
// action for SIGINT ends the process, so the user saw the shell disappear and
// the terminal keep echoing (measured 24.11.2026, acmesh-43qq).
//
// The interactive loop opens a context per statement. A signal cancels that
// context, which stops a running child and breaks a running builtin loop; the
// loop then prints a newline and a fresh prompt instead of dying.
type interrupter struct {
	ch   chan os.Signal
	quit chan struct{}
	wg   sync.WaitGroup

	mu     sync.Mutex
	cancel context.CancelFunc
	fired  bool
}

// newInterrupter starts listening. Only the interactive shell calls it: a
// script keeps the default, so Ctrl-C still ends it the way it always did.
func newInterrupter() *interrupter {
	it := &interrupter{ch: make(chan os.Signal, 1), quit: make(chan struct{})}
	signal.Notify(it.ch, os.Interrupt)
	it.wg.Add(1)
	go func() {
		defer it.wg.Done()
		for {
			select {
			case <-it.quit:
				return
			case sig := <-it.ch:
				it.handle(sig)
			}
		}
	}()
	return it
}

// handle records the signal and stops the statement that is running. It is a
// method of its own so a test can drive it without sending a real signal to
// the test process.
func (it *interrupter) handle(os.Signal) {
	it.mu.Lock()
	it.fired = true
	cancel := it.cancel
	it.mu.Unlock()
	if cancel != nil {
		cancel()
	}
}

// stop ends the listener and gives the terminal back its default handler.
func (it *interrupter) stop() {
	signal.Stop(it.ch)
	close(it.quit)
	it.wg.Wait()
}

// begin opens the context for one statement and returns a function that closes
// it and reports whether a Ctrl-C arrived while the statement ran.
func (it *interrupter) begin() (context.Context, func() bool) {
	ctx, cancel := context.WithCancel(context.Background())
	it.mu.Lock()
	it.cancel = cancel
	it.fired = false
	it.mu.Unlock()
	return ctx, func() bool {
		cancel()
		it.mu.Lock()
		defer it.mu.Unlock()
		it.cancel = nil
		fired := it.fired
		it.fired = false
		return fired
	}
}

// consume reports whether a signal arrived and clears the mark. It answers the
// case where Ctrl-C hits while the shell is reading a line: the terminal drops
// that line, and the loop wants a fresh prompt without an error message.
func (it *interrupter) consume() bool {
	it.mu.Lock()
	defer it.mu.Unlock()
	fired := it.fired
	it.fired = false
	return fired
}
