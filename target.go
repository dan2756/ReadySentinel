package main

import (
	"context"
	"errors"
	"fmt"
	"net"
	"strconv"
	"strings"
)

// A target is something waitfor can wait for.
type target interface {
	// Check makes a single attempt and returns nil if the target is ready.
	Check(ctx context.Context) error
	// String names the target in progress messages.
	String() string
}

// parseTargets turns positional arguments into targets. Most targets are a
// single word whose kind is guessed by parseTarget; "file PATH" and
// "pid PID" name their kind explicitly.
func parseTargets(words []string, o *options) ([]target, error) {
	var ts []target
	for i := 0; i < len(words); i++ {
		var t target
		var err error
		switch w := words[i]; w {
		case "file", "pid":
			if i+1 == len(words) {
				return nil, fmt.Errorf("%s: missing argument", w)
			}
			i++
			if w == "file" {
				t, err = newFileTarget(words[i], o)
			} else {
				t, err = parsePID(words[i])
			}
		default:
			t, err = parseTarget(w, o)
		}
		if err != nil {
			return nil, err
		}
		ts = append(ts, t)
	}
	if len(ts) == 0 {
		return nil, errors.New("nothing to wait for")
	}
	return ts, nil
}

// parseTarget works out what kind of target s describes.
func parseTarget(s string, o *options) (target, error) {
	lower := strings.ToLower(s)
	switch {
	case strings.HasPrefix(lower, "tcp://"):
		return parseHostPort(s[len("tcp://"):])
	case strings.HasPrefix(lower, "http://"), strings.HasPrefix(lower, "https://"):
		return newHTTPTarget(s, o)
	case strings.Contains(s, "://"):
		scheme, _, _ := strings.Cut(s, "://")
		return nil, fmt.Errorf("unsupported scheme %q in %q", scheme, s)
	case strings.Contains(s, "/"):
		// "./build/done", "/tmp/ready", "out/app.sock": host:port never
		// contains a slash, so this must be a path.
		return newFileTarget(s, o)
	case isDigits(s):
		return parseHostPort(":" + s)
	case strings.Contains(s, ":"):
		return parseHostPort(s)
	}
	return nil, fmt.Errorf("don't know how to wait for %q (want PORT, HOST:PORT, a URL, file PATH or pid PID)", s)
}

func parseHostPort(s string) (target, error) {
	host, port, err := net.SplitHostPort(s)
	if err != nil {
		return nil, fmt.Errorf("invalid address %q: want HOST:PORT", s)
	}
	if n, err := strconv.Atoi(port); err != nil || n < 1 || n > 65535 {
		return nil, fmt.Errorf("invalid port %q in %q", port, s)
	}
	if host == "" {
		host = "localhost"
	}
	return &tcpTarget{addr: net.JoinHostPort(host, port)}, nil
}

func isDigits(s string) bool {
	if s == "" {
		return false
	}
	for _, r := range s {
		if r < '0' || r > '9' {
			return false
		}
	}
	return true
}

// tcpTarget is ready once a TCP connection to addr succeeds.
type tcpTarget struct {
	addr string
}

func (t *tcpTarget) String() string { return t.addr }

func (t *tcpTarget) Check(ctx context.Context) error {
	var d net.Dialer
	conn, err := d.DialContext(ctx, "tcp", t.addr)
	if err != nil {
		return err
	}
	conn.Close()
	return nil
}
