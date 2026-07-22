package main

import (
	"context"
	"sort"
	"time"
)

const (
	// After each failed attempt the delay grows by backoffFactor, up to
	// backoffCap times the configured interval.
	backoffFactor = 1.5
	backoffCap    = 4
)

type pollConfig struct {
	interval       time.Duration
	attemptTimeout time.Duration
}

// notReady is a target that was still not ready when time ran out, with the
// error from its last attempt.
type notReady struct {
	target target
	err    error
}

// waitAll polls every target concurrently until all of them are ready or ctx
// is done. onReady, if non-nil, is called as each target becomes ready. It
// returns the targets that were not ready in time, or nil.
func waitAll(ctx context.Context, ts []target, cfg pollConfig, onReady func(target)) []notReady {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	type result struct {
		i   int
		err error
	}
	results := make(chan result, len(ts)) // buffered so pollers never block
	for i, t := range ts {
		go func() { results <- result{i, poll(ctx, t, cfg)} }()
	}

	var failed []result
	for range ts {
		r := <-results
		if r.err != nil {
			failed = append(failed, r)
			continue
		}
		if onReady != nil {
			onReady(ts[r.i])
		}
	}
	if len(failed) == 0 {
		return nil
	}
	sort.Slice(failed, func(a, b int) bool { return failed[a].i < failed[b].i })
	pending := make([]notReady, len(failed))
	for k, r := range failed {
		pending[k] = notReady{ts[r.i], r.err}
	}
	return pending
}

// poll checks t until it is ready (returning nil) or ctx is done (returning
// the error from the last attempt).
func poll(ctx context.Context, t target, cfg pollConfig) error {
	delay := cfg.interval
	var last error
	for {
		err := attempt(ctx, t, cfg.attemptTimeout)
		if err == nil {
			return nil
		}
		if ctx.Err() != nil {
			if last == nil {
				last = err
			}
			return last
		}
		last = err

		timer := time.NewTimer(delay)
		select {
		case <-ctx.Done():
			timer.Stop()
			return last
		case <-timer.C:
		}
		delay = nextDelay(delay, cfg.interval)
	}
}

func attempt(ctx context.Context, t target, timeout time.Duration) error {
	if timeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, timeout)
		defer cancel()
	}
	return t.Check(ctx)
}

// nextDelay backs off gently from the base interval.
func nextDelay(cur, base time.Duration) time.Duration {
	return min(time.Duration(float64(cur)*backoffFactor), backoffCap*base)
}
