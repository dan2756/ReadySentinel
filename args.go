package main

import (
	"errors"
	"flag"
	"io"
	"math"
	"strconv"
	"strings"
	"time"
)

type options struct {
	timeout        time.Duration
	interval       time.Duration
	attemptTimeout time.Duration
	quiet          bool
	any            bool
	help           bool
	version        bool

	status   statusRange // accepted HTTP status codes
	contains string      // HTTP bodies must contain this
	nonempty bool        // file targets must have content
	gone     bool        // file targets must disappear

	words   []string // positional arguments that describe targets
	command []string // what to run once ready, from after "--"
}

func (o *options) pollConfig() pollConfig {
	return pollConfig{
		interval:       o.interval,
		attemptTimeout: o.attemptTimeout,
		any:            o.any,
	}
}

// parseArgs parses the command line. Flags and targets may be mixed in any
// order: "waitfor db:5432 -t 30s cache:6379" is fine. Everything after the
// first "--" is the command to run.
func parseArgs(args []string) (*options, error) {
	o := &options{
		interval:       500 * time.Millisecond,
		attemptTimeout: 5 * time.Second,
		status:         defaultStatus,
	}

	fs := flag.NewFlagSet("waitfor", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	fs.Usage = func() {}
	for _, name := range []string{"t", "timeout"} {
		fs.Var(durationFlag{&o.timeout}, name, "")
	}
	for _, name := range []string{"i", "interval"} {
		fs.Var(durationFlag{&o.interval}, name, "")
	}
	fs.Var(durationFlag{&o.attemptTimeout}, "attempt-timeout", "")
	fs.BoolVar(&o.any, "any", false, "")
	fs.Var(&o.status, "status", "")
	fs.StringVar(&o.contains, "contains", "", "")
	fs.BoolVar(&o.nonempty, "nonempty", false, "")
	fs.BoolVar(&o.gone, "gone", false, "")
	for _, name := range []string{"q", "quiet"} {
		fs.BoolVar(&o.quiet, name, false, "")
	}
	for _, name := range []string{"h", "help"} {
		fs.BoolVar(&o.help, name, false, "")
	}
	fs.BoolVar(&o.version, "version", false, "")

	for i, a := range args {
		if a == "--" {
			o.command = args[i+1:]
			args = args[:i]
			if len(o.command) == 0 {
				return nil, errors.New("missing command after --")
			}
			break
		}
	}

	// The flag package stops at the first positional argument, so keep
	// going after each one.
	for {
		if err := fs.Parse(args); err != nil {
			return nil, err
		}
		args = fs.Args()
		if len(args) == 0 {
			break
		}
		word := args[0]
		o.words = append(o.words, word)
		args = args[1:]
		// "file" and "pid" take the next word as their operand even when it
		// starts with "-": that is a path or a pid, not a flag. The flag
		// package would otherwise reject "file -deploy.lock".
		if (word == "file" || word == "pid") && len(args) > 0 {
			o.words = append(o.words, args[0])
			args = args[1:]
		}
	}

	if o.interval <= 0 {
		return nil, errors.New("--interval must be greater than zero")
	}
	if o.nonempty && o.gone {
		return nil, errors.New("--nonempty and --gone cannot be combined")
	}
	return o, nil
}

// durationFlag is a flag.Value for durations that also accepts a bare
// number of seconds ("30" or "2.5").
type durationFlag struct{ d *time.Duration }

func (f durationFlag) String() string {
	if f.d == nil {
		return ""
	}
	return f.d.String()
}

func (f durationFlag) Set(s string) error {
	d, err := parseDuration(s)
	if err != nil {
		return err
	}
	*f.d = d
	return nil
}

func parseDuration(s string) (time.Duration, error) {
	errBad := errors.New("want a duration like 30s, 500ms or 2m")
	if s != "" && strings.Trim(s, "0123456789.") == "" {
		secs, err := strconv.ParseFloat(s, 64)
		if err != nil || math.IsNaN(secs) || math.IsInf(secs, 0) {
			return 0, errBad
		}
		// A bare number is seconds. Converting an out-of-range float to a
		// Duration wraps to a negative value, and a negative -t is treated
		// as "wait forever".
		ns := secs * float64(time.Second)
		if ns < 0 || ns > float64(math.MaxInt64) {
			return 0, errors.New("duration is too large")
		}
		d := time.Duration(ns)
		if d < 0 {
			return 0, errors.New("duration is too large")
		}
		return d, nil
	}
	d, err := time.ParseDuration(s)
	if err != nil {
		return 0, errBad
	}
	if d < 0 {
		return 0, errors.New("duration must not be negative")
	}
	return d, nil
}
