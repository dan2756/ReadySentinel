//go:build unix

package main

import (
	"io"
	"os"
	"os/exec"
	"syscall"
)

// execCommand replaces waitfor with argv, like a shell's exec: the command
// keeps waitfor's pid, stdio and environment, and its exit status becomes
// ours. It only returns if the command could not be started.
func execCommand(argv []string, stderr io.Writer) int {
	path, err := exec.LookPath(argv[0])
	if err != nil {
		return commandError(stderr, argv[0], err)
	}
	err = syscall.Exec(path, argv, os.Environ())
	return commandError(stderr, argv[0], err)
}
