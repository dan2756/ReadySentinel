package main

import (
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os/exec"
)

// Exit codes for a command that could not be run, as in POSIX shells.
const (
	exitCannotRun = 126
	exitNotFound  = 127
)

func commandError(w io.Writer, name string, err error) int {
	var ee *exec.Error
	if !errors.As(err, &ee) {
		err = fmt.Errorf("%s: %w", name, err)
	}
	fmt.Fprintf(w, "waitfor: %v\n", err)
	if errors.Is(err, exec.ErrNotFound) || errors.Is(err, fs.ErrNotExist) {
		return exitNotFound
	}
	return exitCannotRun
}
