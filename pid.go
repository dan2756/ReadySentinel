package main

import (
	"fmt"
	"strconv"
)

// pidTarget is ready once the process with the given pid has exited. The
// process does not have to be a child of waitfor.
type pidTarget struct {
	pid int
}

func parsePID(s string) (target, error) {
	// pid_t is 32 bits. A larger number would be truncated by kill(2), so
	// that 4294967297 checks pid 1 and 4294967295 checks every process.
	n, err := strconv.ParseInt(s, 10, 32)
	if err != nil || n <= 0 {
		return nil, fmt.Errorf("invalid pid %q", s)
	}
	return &pidTarget{pid: int(n)}, nil
}

func (t *pidTarget) String() string { return "pid " + strconv.Itoa(t.pid) }
