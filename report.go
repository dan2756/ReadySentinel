package main

import (
	"fmt"
	"io"
	"strings"
	"time"
)

// reporter prints progress to stderr. With a single target everything fits
// on one line:
//
//	waiting for localhost:5432… ready after 2.4s
//
// With several targets each one reports on its own line.
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
	msg := "timed out after " + formatDuration(time.Since(r.started))
	if !r.single {
		names := make([]string, len(pending))
		for i, p := range pending {
			names[i] = p.target.String()
		}
		if r.any {
			msg += "; none of " + strings.Join(names, ", ") + " became ready"
		} else {
			msg += "; still waiting for " + strings.Join(names, ", ")
		}
	}
	r.printf("%s\n", msg)
}

func (r *reporter) interrupted() {
	r.printf("interrupted\n")
}

// formatDuration prints short durations as "2.4s" and longer ones as "1m12s".
func formatDuration(d time.Duration) string {
	if d < time.Minute {
		return fmt.Sprintf("%.1fs", d.Seconds())
	}
	return d.Round(time.Second).String()
}
