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
	n, err := strconv.Atoi(s)
	if err != nil || n <= 0 {
		return nil, fmt.Errorf("invalid pid %q", s)
	}
	return &pidTarget{pid: n}, nil
}

func (t *pidTarget) String() string { return "pid " + strconv.Itoa(t.pid) }
