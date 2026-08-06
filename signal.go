package main

import (
	"context"
	"os"
	"os/signal"
	"syscall"
)

// signalError is the cancellation cause when waitfor is interrupted.
type signalError struct {
	sig os.Signal
}

func (e *signalError) Error() string { return "received " + e.sig.String() }

// exitCode follows the shell convention of 128 + the signal number.
func (e *signalError) exitCode() int {
	if s, ok := e.sig.(syscall.Signal); ok {
		return 128 + int(s)
	}
	return exitInterrupted
}

// withSignals returns a context that is cancelled with a *signalError when
// SIGINT or SIGTERM arrives. Calling stop restores the default handling.
func withSignals(parent context.Context) (ctx context.Context, stop func()) {
	ctx, cancel := context.WithCancelCause(parent)
	sigc := make(chan os.Signal, 1)
	signal.Notify(sigc, os.Interrupt, syscall.SIGTERM)
	go func() {
		select {
		case s := <-sigc:
			cancel(&signalError{s})
		case <-ctx.Done():
		}
	}()
	return ctx, func() {
		signal.Stop(sigc)
		cancel(nil)
	}
}
