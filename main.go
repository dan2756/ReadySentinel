// Command waitfor waits until TCP ports accept connections, HTTP endpoints
// answer, files appear or processes exit, then exits 0.
package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"runtime/debug"
)

// version is overridden at build time with -ldflags "-X main.version=v1.2.3".
var version = "dev"

const (
	exitReady       = 0
	exitTimeout     = 1
	exitUsage       = 2
	exitInterrupted = 130 // 128 + SIGINT; SIGTERM gives 143
)

const usage = `waitfor waits for a port, URL, file or process exit, then exits 0.

Usage:
  waitfor [flags] TARGET... [-- COMMAND [ARG...]]

All targets must be ready unless --any is given. With a COMMAND, waitfor
execs it once ready and exits with its status; on timeout it is not run.

Targets:
  5432, :5432         TCP port on localhost
  HOST:PORT           a TCP connection to HOST:PORT succeeds
  tcp://HOST:PORT     the same, spelled out
  http://..., https://...
                      a GET request returns an accepted status
  file PATH           PATH exists; "./PATH", "/PATH" or anything with a
                      slash is taken as a file too
  pid PID             process PID has exited (it need not be our child)

Flags:
  -t, --timeout DUR        give up after DUR (default 0: wait forever)
  -i, --interval DUR       delay between attempts (default 500ms, backing off to 4x)
      --attempt-timeout DUR
                           limit for a single attempt (default 5s)
      --any                succeed as soon as any one target is ready
      --status RANGE       accepted HTTP status: 200 or 200-299 (default 200-399)
      --nonempty           file targets must not be empty
      --gone               wait for file targets to disappear instead
  -q, --quiet              don't print progress to stderr
  -h, --help               show this help
      --version            print the version

Durations look like 30s, 1m30s or 500ms; a bare number means seconds.

Exit status:
  0    ready (or with COMMAND, the command's own status)
  1    timed out
  2    usage error
  126  COMMAND could not be run; 127 if it was not found
  130  interrupted (143 for SIGTERM)
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

	sigCtx, stopSignals := withSignals(context.Background())
	defer stopSignals()
	ctx := sigCtx
	if opts.timeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, opts.timeout)
		defer cancel()
	}

	rep := newReporter(stderr, opts.quiet, opts.any)
	rep.start(targets)
	if pending := waitAll(ctx, targets, opts.pollConfig(), rep.ready); len(pending) > 0 {
		var sig *signalError
		if errors.As(context.Cause(sigCtx), &sig) {
			rep.interrupted()
			return sig.exitCode()
		}
		rep.timedOut(pending)
		return exitTimeout
	}

	if len(opts.command) > 0 {
		stopSignals() // let the command see signals as usual
		return execCommand(opts.command, stderr)
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
