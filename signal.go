package main

import (
	"context"
	"errors"
	"os"
	"os/signal"
	"sync"
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

// signalCause reports the signal that cancelled ctx, if it was an interrupt.
func signalCause(ctx context.Context) *signalError {
	var sig *signalError
	if errors.As(context.Cause(ctx), &sig) {
		return sig
	}
	return nil
}

// newSignalContext is withSignals. Tests replace it to deliver an interrupt
// without signaling the test process.
var newSignalContext = withSignals

// beforeSignalSelect, if set, runs in the signal goroutine before it waits.
// Tests use it to hold that goroutine so a signal can sit in the buffer.
var beforeSignalSelect func()

// withSignals returns a context that is cancelled with a *signalError when
// SIGINT or SIGTERM arrives. Calling stop restores the default handling.
//
// stop waits until a signal that Notify has already queued is recorded.
// Cancelling first and letting the goroutine notice afterwards drops that
// signal: the first cancel wins, and a later cancel(signal) is a no-op.
func withSignals(parent context.Context) (ctx context.Context, stop func()) {
	ctx, cancel := context.WithCancelCause(parent)
	sigc := make(chan os.Signal, 1)
	signal.Notify(sigc, os.Interrupt, syscall.SIGTERM)
	stopped := make(chan struct{})
	done := make(chan struct{})
	hook := beforeSignalSelect
	go func() {
		defer close(done)
		if hook != nil {
			hook()
		}
		select {
		case s := <-sigc:
			cancel(&signalError{s})
		case <-stopped:
			// Prefer a signal already sitting in the buffer. Both this case
			// and the receive above can be ready together; either one records it.
			select {
			case s := <-sigc:
				cancel(&signalError{s})
			default:
				cancel(nil)
			}
		case <-ctx.Done():
			select {
			case s := <-sigc:
				cancel(&signalError{s})
			default:
			}
		}
	}()
	var once sync.Once
	return ctx, func() {
		signal.Stop(sigc)
		once.Do(func() { close(stopped) })
		<-done
	}
}
