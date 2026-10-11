package main

import (
	"os"
	"strconv"
)

// isZombie reports whether pid has exited but has not been reaped by its
// parent yet. Such a process still answers kill(pid, 0).
func isZombie(pid int) bool {
	b, err := os.ReadFile("/proc/" + strconv.Itoa(pid) + "/stat")
	if err != nil {
		return false
	}
	state, ok := procState(string(b))
	return ok && (state == 'Z' || state == 'X')
}
