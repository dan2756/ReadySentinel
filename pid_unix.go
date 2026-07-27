//go:build unix

package main

import (
	"context"
	"errors"
	"syscall"
)

// Check sends signal 0, which delivers nothing but reports whether the
// process still exists.
func (t *pidTarget) Check(context.Context) error {
	if err := syscall.Kill(t.pid, 0); err != nil {
		// No such process: it has exited.
		return nil
	}
	return errors.New("still running")
}
