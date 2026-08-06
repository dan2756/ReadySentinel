//go:build !unix

package main

import (
	"errors"
	"io"
	"os"
	"os/exec"
)

// execCommand runs argv as a child process and returns its exit status.
// Platforms without exec(2) can't replace the current process.
func execCommand(argv []string, stderr io.Writer) int {
	cmd := exec.Command(argv[0], argv[1:]...)
	cmd.Stdin, cmd.Stdout, cmd.Stderr = os.Stdin, os.Stdout, os.Stderr
	err := cmd.Run()
	var ee *exec.ExitError
	if errors.As(err, &ee) {
		return ee.ExitCode()
	}
	if err != nil {
		return commandError(stderr, argv[0], err)
	}
	return 0
}
