package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"net/url"
	"os"
	"strings"
	"time"
)

// reporter prints progress to stderr. With a single target everything fits
// on one line:
//
//	waiting for localhost:5432… ready after 2.4s
//
// With several targets each one reports on its own line. On timeout, each
// target that was not ready is listed with the reason its last attempt
// failed.
type reporter struct {
	w       io.Writer
	quiet   bool
	any     bool
	single  bool
	started time.Time
}

func newReporter(w io.Writer, quiet, anyOf bool) *reporter {
	return &reporter{w: w, quiet: quiet, any: anyOf}
}

func (r *reporter) printf(format string, args ...any) {
	if !r.quiet {
		fmt.Fprintf(r.w, format, args...)
	}
}

func (r *reporter) start(ts []target) {
	r.started = time.Now()
	r.single = len(ts) == 1
	if r.single {
		r.printf("waiting for %s… ", ts[0])
		return
	}
	names := make([]string, len(ts))
	for i, t := range ts {
		names[i] = t.String()
	}
	of := ""
	if r.any {
		of = "any of "
	}
	r.printf("waiting for %s%s…\n", of, strings.Join(names, ", "))
}

func (r *reporter) ready(t target) {
	elapsed := formatDuration(time.Since(r.started))
	if r.single {
		r.printf("ready after %s\n", elapsed)
		return
	}
	r.printf("%s ready after %s\n", t, elapsed)
}

func (r *reporter) timedOut(pending []notReady) {
	elapsed := formatDuration(time.Since(r.started))
	if r.single {
		r.printf("timed out after %s (%s)\n", elapsed, reason(pending[0].err))
		return
	}
	what := "not ready"
	if r.any {
		what = "none ready"
	}
	var b strings.Builder
	fmt.Fprintf(&b, "timed out after %s; %s:\n", elapsed, what)
	for _, p := range pending {
		fmt.Fprintf(&b, "  %s: %s\n", p.target, reason(p.err))
	}
	r.printf("%s", b.String())
}

func (r *reporter) interrupted() {
	r.printf("interrupted\n")
}

// reason shortens an attempt's error for the timeout message, so that
//
//	Get "http://localhost:8080/": dial tcp [::1]:8080: connect: connection refused
//
// is reported as just "connection refused".
func reason(err error) string {
	var dnsErr *net.DNSError
	var sysErr *os.SyscallError
	var netErr net.Error
	var urlErr *url.Error
	switch {
	case err == nil:
		return "unknown error"
	case errors.As(err, &dnsErr):
		return "lookup " + dnsErr.Name + ": " + dnsErr.Err
	case errors.As(err, &sysErr):
		return sysErr.Err.Error()
	case errors.Is(err, context.DeadlineExceeded), errors.As(err, &netErr) && netErr.Timeout():
		return "no response"
	case errors.As(err, &urlErr):
		return urlErr.Err.Error()
	}
	return err.Error()
}

// formatDuration prints short durations as "2.4s" and longer ones as "1m12s".
func formatDuration(d time.Duration) string {
	if d < time.Minute {
		return fmt.Sprintf("%.1fs", d.Seconds())
	}
	return d.Round(time.Second).String()
}
