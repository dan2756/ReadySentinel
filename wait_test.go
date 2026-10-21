package main

import (
	"context"
	"fmt"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

// fakeTarget fails readyAfter times, then succeeds. A negative readyAfter
// means it is never ready.
type fakeTarget struct {
	name       string
	readyAfter int32
	calls      atomic.Int32
}

func (f *fakeTarget) String() string { return f.name }

func (f *fakeTarget) Check(ctx context.Context) error {
	n := f.calls.Add(1)
	if f.readyAfter >= 0 && n > f.readyAfter {
		return nil
	}
	return fmt.Errorf("attempt %d failed", n)
}

// blockingTarget never answers until its context is done.
type blockingTarget struct{}

func (blockingTarget) String() string { return "blocking" }

func (blockingTarget) Check(ctx context.Context) error {
	<-ctx.Done()
	return ctx.Err()
}

var fastPoll = pollConfig{interval: 5 * time.Millisecond, attemptTimeout: time.Second}

func TestWaitAllReady(t *testing.T) {
	a := &fakeTarget{name: "a", readyAfter: 0}
	b := &fakeTarget{name: "b", readyAfter: 3}
	var ready []string
	pending := waitAll(testContext(t), []target{a, b}, fastPoll, func(t target) {
		ready = append(ready, t.String())
	})
	if pending != nil {
		t.Fatalf("pending = %v, want nil", pending)
	}
	if got := b.calls.Load(); got != 4 {
		t.Errorf("b checked %d times, want 4", got)
	}
	if strings.Join(ready, ",") != "a,b" {
		t.Errorf("ready order = %v, want [a b]", ready)
	}
}

func TestWaitAllTimeout(t *testing.T) {
	a := &fakeTarget{name: "a", readyAfter: 0}
	b := &fakeTarget{name: "b", readyAfter: -1}
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	start := time.Now()
	pending := waitAll(ctx, []target{a, b}, fastPoll, nil)
	if elapsed := time.Since(start); elapsed > 2*time.Second {
		t.Errorf("waitAll took %v after a 100ms timeout", elapsed)
	}
	if len(pending) != 1 || pending[0].target != b {
		t.Fatalf("pending = %v, want only b", pending)
	}
	if pending[0].err == nil || !strings.Contains(pending[0].err.Error(), "failed") {
		t.Errorf("last error = %v, want the target's own error", pending[0].err)
	}
}

func TestWaitAllAny(t *testing.T) {
	never := &fakeTarget{name: "never", readyAfter: -1}
	soon := &fakeTarget{name: "soon", readyAfter: 2}
	cfg := fastPoll
	cfg.any = true

	var ready []string
	pending := waitAll(testContext(t), []target{never, soon}, cfg, func(t target) {
		ready = append(ready, t.String())
	})
	if pending != nil {
		t.Fatalf("pending = %v, want nil", pending)
	}
	if strings.Join(ready, ",") != "soon" {
		t.Errorf("ready = %v, want [soon]", ready)
	}
}

func TestWaitAllAnyTimeout(t *testing.T) {
	a := &fakeTarget{name: "a", readyAfter: -1}
	b := &fakeTarget{name: "b", readyAfter: -1}
	cfg := fastPoll
	cfg.any = true
	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()
	if pending := waitAll(ctx, []target{a, b}, cfg, nil); len(pending) != 2 {
		t.Errorf("pending = %v, want both targets", pending)
	}
}

func TestAttemptTimeout(t *testing.T) {
	start := time.Now()
	err := attempt(context.Background(), blockingTarget{}, 20*time.Millisecond)
	if err == nil {
		t.Fatal("want an error from a timed-out attempt")
	}
	if elapsed := time.Since(start); elapsed > time.Second {
		t.Errorf("attempt took %v, want about 20ms", elapsed)
	}
}

func TestNextDelay(t *testing.T) {
	base := 500 * time.Millisecond
	want := []time.Duration{750 * time.Millisecond, 1125 * time.Millisecond, 1687500 * time.Microsecond, 2 * time.Second, 2 * time.Second}
	d := base
	for i, w := range want {
		d = nextDelay(d, base)
		if d != w {
			t.Fatalf("step %d: delay = %v, want %v", i+1, d, w)
		}
	}
}

func TestNextDelayNoOverflow(t *testing.T) {
	// 900000h fits in a Duration, but four times that does not. The old
	// cap wrapped negative and the poll loop would spin.
	base := 900000 * time.Hour
	d := base
	for i := 0; i < 5; i++ {
		d = nextDelay(d, base)
		if d <= 0 {
			t.Fatalf("step %d: delay = %v, want positive", i+1, d)
		}
	}
}
