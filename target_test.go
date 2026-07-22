package main

import (
	"context"
	"net"
	"testing"
	"time"
)

func TestParseTargetTCP(t *testing.T) {
	tests := []struct{ in, want string }{
		{"5432", "localhost:5432"},
		{":5432", "localhost:5432"},
		{"db:5432", "db:5432"},
		{"tcp://db:5432", "db:5432"},
		{"TCP://db:5432", "db:5432"},
		{"127.0.0.1:80", "127.0.0.1:80"},
		{"[::1]:5432", "[::1]:5432"},
	}
	for _, tt := range tests {
		got, err := parseTarget(tt.in)
		if err != nil {
			t.Errorf("parseTarget(%q): %v", tt.in, err)
			continue
		}
		if _, ok := got.(*tcpTarget); !ok {
			t.Errorf("parseTarget(%q) = %T, want *tcpTarget", tt.in, got)
		}
		if got.String() != tt.want {
			t.Errorf("parseTarget(%q) = %q, want %q", tt.in, got, tt.want)
		}
	}
}

func TestParseTargetErrors(t *testing.T) {
	for _, in := range []string{"0", "65536", "localhost", "db:", "db:http", "::1", "ftp://host:21"} {
		if got, err := parseTarget(in); err == nil {
			t.Errorf("parseTarget(%q) = %v, want error", in, got)
		}
	}
}

func TestParseTargetsEmpty(t *testing.T) {
	if _, err := parseTargets(nil); err == nil {
		t.Error("parseTargets(nil): want error")
	}
}

func TestTCPTarget(t *testing.T) {
	ln := listen(t)
	tgt := &tcpTarget{addr: ln.Addr().String()}
	if err := tgt.Check(testContext(t)); err != nil {
		t.Fatalf("Check on open port: %v", err)
	}

	closed := &tcpTarget{addr: closedAddr(t)}
	if err := closed.Check(testContext(t)); err == nil {
		t.Fatal("Check on closed port: want error")
	}
}

// listen starts a TCP listener on a random loopback port that accepts and
// immediately closes connections.
func listen(t *testing.T) net.Listener {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { ln.Close() })
	go func() {
		for {
			c, err := ln.Accept()
			if err != nil {
				return
			}
			c.Close()
		}
	}()
	return ln
}

// closedAddr returns a loopback address that nothing is listening on.
func closedAddr(t *testing.T) string {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	addr := ln.Addr().String()
	ln.Close()
	return addr
}

func testContext(t *testing.T) context.Context {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	t.Cleanup(cancel)
	return ctx
}
