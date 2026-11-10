//go:build unix

package main

import (
	"context"
	"os"
	"os/signal"
	"syscall"
	"testing"
	"time"
)

func TestStopObservesQueuedSignal(t *testing.T) {
	entered := make(chan struct{})
	release := make(chan struct{})
	orig := beforeSignalSelect
	beforeSignalSelect = func() {
		close(entered)
		<-release
	}
	t.Cleanup(func() {
		beforeSignalSelect = orig
		select {
		case <-release:
		default:
			close(release)
		}
	})

	ctx, stop := withSignals(context.Background())
	// Release the hook before stop waits on the goroutine. Defers run
	// before Cleanup, so stop must not be deferred ahead of that release.
	t.Cleanup(func() {
		select {
		case <-release:
		default:
			close(release)
		}
		stop()
	})
	select {
	case <-entered:
	case <-time.After(2 * time.Second):
		t.Fatal("signal goroutine did not start")
	}

	// A second registration receives the same SIGINT. Notify delivers to
	// every channel before returning, and this one was registered second,
	// so a receive here means withSignals' buffer already holds the signal.
	// The handler goroutine is still parked in the hook, so it has not
	// consumed that buffer yet.
	seen := make(chan os.Signal, 1)
	signal.Notify(seen, os.Interrupt)
	defer signal.Stop(seen)

	if err := syscall.Kill(os.Getpid(), syscall.SIGINT); err != nil {
		t.Fatal(err)
	}
	select {
	case <-seen:
	case <-time.After(2 * time.Second):
		t.Fatal("SIGINT was not delivered")
	}

	// Let the goroutine reach its select only after stop has asked it to
	// finish, which is when a queued signal used to be overwritten.
	go func() {
		time.Sleep(20 * time.Millisecond)
		close(release)
	}()
	stop()
	sig := signalCause(ctx)
	if sig == nil {
		t.Fatal("signal queued before stop was dropped")
	}
	if sig.exitCode() != 130 {
		t.Fatalf("exit code %d, want 130", sig.exitCode())
	}
}
