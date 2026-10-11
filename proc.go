package main

import "strings"

// procState extracts the one-letter state from the contents of
// /proc/PID/stat, which looks like "1234 (cmd name) S 1 1234 ...". The
// command name may contain spaces and parentheses itself, so the state is
// found after the last ')'.
func procState(stat string) (byte, bool) {
	i := strings.LastIndexByte(stat, ')')
	if i < 0 || i+2 >= len(stat) || stat[i+1] != ' ' {
		return 0, false
	}
	return stat[i+2], true
}
