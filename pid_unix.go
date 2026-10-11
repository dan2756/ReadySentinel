//go:build unix

package main

import (
	"context"
	"errors"
	"syscall"
)

var errStillRunning = errors.New("still running")

// signalZero sends signal 0, which delivers nothing but reports whether the
// process exists. Tests replace it.
var signalZero = func(pid int) error { return syscall.Kill(pid, 0) }

func (t *pidTarget) Check(context.Context) error {
	err := signalZero(t.pid)
	switch {
	case errors.Is(err, syscall.ESRCH):
		return nil
	case err != nil && !errors.Is(err, syscall.EPERM):
		return err
	}
	// The process exists. (EPERM means we may not signal it, typically
	// because it belongs to another user.) A zombie has exited already and
	// is only waiting for its parent to reap it.
	if isZombie(t.pid) {
		return nil
	}
	return errStillRunning
}
