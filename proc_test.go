package main

import "testing"

func TestProcState(t *testing.T) {
	tests := []struct {
		stat  string
		state byte
		ok    bool
	}{
		{"1234 (sleep) S 1 1234 1234 0 -1 4194560 104 0 0 0", 'S', true},
		{"42 (defunct) Z 1 42 42 0 -1", 'Z', true},
		{"77 (my (odd) name) R 1 77", 'R', true},
		{"9 (a) b) D 1", 'D', true},
		{"", 0, false},
		{"1234 (sleep)", 0, false},
		{"1234 sleep S 1", 0, false},
	}
	for _, tt := range tests {
		state, ok := procState(tt.stat)
		if state != tt.state || ok != tt.ok {
			t.Errorf("procState(%q) = %q, %v; want %q, %v", tt.stat, state, ok, tt.state, tt.ok)
		}
	}
}
