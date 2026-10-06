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
	case err == nil, errors.Is(err, syscall.EPERM):
		// EPERM: the process exists but we may not signal it, typically
		// because it belongs to another user.
		return errStillRunning
	case errors.Is(err, syscall.ESRCH):
		return nil
	default:
		return err
	}
}
