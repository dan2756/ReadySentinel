package main

import (
	"bytes"
	"context"
	"errors"
	"net"
	"net/url"
	"os"
	"syscall"
	"testing"
	"time"
)

func TestReason(t *testing.T) {
	refused := &net.OpError{Op: "dial", Net: "tcp", Err: &os.SyscallError{Syscall: "connect", Err: syscall.ECONNREFUSED}}
	tests := []struct {
		err  error
		want string
	}{
		{refused, "connection refused"},
		{&url.Error{Op: "Get", URL: "http://localhost:8080/", Err: refused}, "connection refused"},
		{&net.DNSError{Name: "db", Err: "no such host", Server: "127.0.0.53:53", IsNotFound: true}, "lookup db: no such host"},
		{&url.Error{Op: "Get", URL: "http://db/", Err: &net.OpError{Op: "dial", Net: "tcp", Err: &net.DNSError{Name: "db", Err: "no such host"}}}, "lookup db: no such host"},
		{context.DeadlineExceeded, "no response"},
		{&url.Error{Op: "Get", URL: "http://x/", Err: context.DeadlineExceeded}, "no response"},
		{&url.Error{Op: "Get", URL: "https://x/", Err: errors.New("tls: bad certificate")}, "tls: bad certificate"},
		{errors.New("got 503 Service Unavailable"), "got 503 Service Unavailable"},
		{nil, "unknown error"},
	}
	for _, tt := range tests {
		if got := reason(tt.err); got != tt.want {
			t.Errorf("reason(%v) = %q, want %q", tt.err, got, tt.want)
		}
	}
}

func TestReasonFromRealDial(t *testing.T) {
	err := (&tcpTarget{addr: closedAddr(t)}).Check(testContext(t))
	if got := reason(err); got != "connection refused" {
		t.Errorf("reason(%v) = %q, want %q", err, got, "connection refused")
	}
}

func TestReporterTimedOut(t *testing.T) {
	a := &fakeTarget{name: "db:5432"}
	b := &fakeTarget{name: "http://api/health"}
	pending := []notReady{
		{a, errors.New("connection refused")},
		{b, errors.New("got 503 Service Unavailable")},
	}

	var buf bytes.Buffer
	r := newReporter(&buf, false, false)
	r.start([]target{a, b})
	r.started = r.started.Add(-30 * time.Second)
	r.timedOut(pending)
	want := "waiting for db:5432, http://api/health…\n" +
		"timed out after 30.0s; not ready:\n" +
		"  db:5432: connection refused\n" +
		"  http://api/health: got 503 Service Unavailable\n"
	if buf.String() != want {
		t.Errorf("output:\n%s\nwant:\n%s", buf.String(), want)
	}

	buf.Reset()
	r = newReporter(&buf, false, false)
	r.start([]target{a})
	r.started = r.started.Add(-90 * time.Second)
	r.timedOut(pending[:1])
	if want := "waiting for db:5432… timed out after 1m30s (connection refused)\n"; buf.String() != want {
		t.Errorf("output = %q, want %q", buf.String(), want)
	}
}

func TestFormatDuration(t *testing.T) {
	tests := map[time.Duration]string{
		0:                                "0.0s",
		2400 * time.Millisecond:          "2.4s",
		59 * time.Second:                 "59.0s",
		59940 * time.Millisecond:         "59.9s",
		59950 * time.Millisecond:         "1m0s", // %.1f would print "60.0s"
		59999 * time.Millisecond:         "1m0s",
		72 * time.Second:                 "1m12s",
		time.Hour + 500*time.Millisecond: "1h0m1s",
	}
	for d, want := range tests {
		if got := formatDuration(d); got != want {
			t.Errorf("formatDuration(%v) = %q, want %q", d, got, want)
		}
	}
}
