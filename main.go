// Command waitfor waits until TCP ports accept connections or HTTP endpoints
// answer, then exits 0.
package main

import (
	"context"
	"fmt"
	"io"
	"os"
	"runtime/debug"
)

// version is overridden at build time with -ldflags "-X main.version=v1.2.3".
var version = "dev"

const (
	exitReady   = 0
	exitTimeout = 1
	exitUsage   = 2
)

const usage = `waitfor waits until ports and URLs are ready, then exits 0.

Usage:
  waitfor [flags] TARGET...

Targets:
  5432, :5432         TCP port on localhost
  HOST:PORT           a TCP connection to HOST:PORT succeeds
  tcp://HOST:PORT     the same, spelled out
  http://..., https://...
                      a GET request returns an accepted status

Flags:
  -t, --timeout DUR        give up after DUR (default 0: wait forever)
  -i, --interval DUR       delay between attempts (default 500ms, backing off to 4x)
      --attempt-timeout DUR
                           limit for a single attempt (default 5s)
      --status RANGE       accepted HTTP status: 200 or 200-299 (default 200-399)
  -q, --quiet              don't print progress to stderr
  -h, --help               show this help
      --version            print the version

Durations look like 30s, 1m30s or 500ms; a bare number means seconds.

Exit status: 0 ready, 1 timed out, 2 usage error.
`

func main() {
	os.Exit(run(os.Args[1:], os.Stdout, os.Stderr))
}

func run(args []string, stdout, stderr io.Writer) int {
	opts, err := parseArgs(args)
	if err != nil {
		return usageError(stderr, err)
	}
	switch {
	case opts.help:
		fmt.Fprint(stdout, usage)
		return exitReady
	case opts.version:
		fmt.Fprintf(stdout, "waitfor %s\n", versionString())
		return exitReady
	}

	targets, err := parseTargets(opts.words, opts)
	if err != nil {
		return usageError(stderr, err)
	}

	ctx := context.Background()
	if opts.timeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, opts.timeout)
		defer cancel()
	}

	rep := newReporter(stderr, opts.quiet)
	rep.start(targets)
	if pending := waitAll(ctx, targets, opts.pollConfig(), rep.ready); len(pending) > 0 {
		rep.timedOut(pending)
		return exitTimeout
	}
	return exitReady
}

func usageError(w io.Writer, err error) int {
	fmt.Fprintf(w, "waitfor: %v\nTry 'waitfor --help' for more information.\n", err)
	return exitUsage
}

func versionString() string {
	if version != "dev" {
		return version
	}
	if bi, ok := debug.ReadBuildInfo(); ok && bi.Main.Version != "" && bi.Main.Version != "(devel)" {
		return bi.Main.Version
	}
	return version
}
