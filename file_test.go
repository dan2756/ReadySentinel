package main

import (
	"bytes"
	"context"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

func TestParseTargetFile(t *testing.T) {
	for _, in := range []string{"./build/done", "/tmp/ready", "../ready", "out/app.sock"} {
		got, err := parseTarget(in, defaultOptions(t))
		if err != nil {
			t.Errorf("parseTarget(%q): %v", in, err)
			continue
		}
		ft, ok := got.(*fileTarget)
		if !ok || ft.path != in {
			t.Errorf("parseTarget(%q) = %#v, want file target", in, got)
		}
	}
}

func TestParseTargetsFileKeyword(t *testing.T) {
	ts, err := parseTargets([]string{"file", "done", "5432", "file", "./x"}, defaultOptions(t))
	if err != nil {
		t.Fatal(err)
	}
	var names []string
	for _, tgt := range ts {
		names = append(names, tgt.String())
	}
	if got, want := strings.Join(names, ", "), "file done, localhost:5432, file ./x"; got != want {
		t.Errorf("targets = %s, want %s", got, want)
	}

	if _, err := parseTargets([]string{"5432", "file"}, defaultOptions(t)); err == nil {
		t.Error("file without a path: want error")
	}
	if _, err := parseTargets([]string{"file", ""}, defaultOptions(t)); err == nil {
		t.Error("file with an empty path: want error")
	}
}

func TestParseTargetSuggestsFile(t *testing.T) {
	_, err := parseTarget("done.flag", defaultOptions(t))
	if err == nil || !strings.Contains(err.Error(), `"file done.flag"`) {
		t.Errorf("err = %v, want a hint to use file", err)
	}
}

func TestParseArgsNonemptyAndGone(t *testing.T) {
	if _, err := parseArgs([]string{"--nonempty", "--gone", "./x"}); err == nil {
		t.Error("want an error for --nonempty with --gone")
	}
}

func TestFileTarget(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "done")
	exists := &fileTarget{path: path}
	nonempty := &fileTarget{path: path, nonempty: true}
	gone := &fileTarget{path: path, gone: true}
	ctx := testContext(t)

	check := func(step string, tgt *fileTarget, ready bool) {
		t.Helper()
		if err := tgt.Check(ctx); (err == nil) != ready {
			t.Errorf("%s: %s: err = %v, want ready=%v", step, tgt, err, ready)
		}
	}

	check("missing", exists, false)
	check("missing", nonempty, false)
	check("missing", gone, true)

	if err := os.WriteFile(path, nil, 0o644); err != nil {
		t.Fatal(err)
	}
	check("empty", exists, true)
	check("empty", nonempty, false)
	check("empty", gone, false)

	if err := os.WriteFile(path, []byte("ok\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	check("written", exists, true)
	check("written", nonempty, true)
	check("written", gone, false)
}

func TestFileTargetGoneSymlink(t *testing.T) {
	// A symlink lock ("ln -s $$ deploy.lock") points at something that
	// doesn't exist, but the lock is still held while the link is there.
	path := filepath.Join(t.TempDir(), "deploy.lock")
	if err := os.Symlink("12345", path); err != nil {
		t.Skip("cannot create symlinks:", err)
	}
	gone := &fileTarget{path: path, gone: true}
	if err := gone.Check(testContext(t)); err == nil {
		t.Error("dangling symlink counted as gone")
	}
	if err := os.Remove(path); err != nil {
		t.Fatal(err)
	}
	if err := gone.Check(testContext(t)); err != nil {
		t.Errorf("removed symlink: %v", err)
	}
}

func TestFileTargetGoneNotDir(t *testing.T) {
	file := filepath.Join(t.TempDir(), "f")
	if err := os.WriteFile(file, nil, 0o644); err != nil {
		t.Fatal(err)
	}
	// f is a regular file, so f/sub cannot exist.
	if err := (&fileTarget{path: filepath.Join(file, "sub"), gone: true}).Check(testContext(t)); err != nil {
		t.Errorf("path under a regular file: %v", err)
	}
}

func TestAwaitCheckPrefersFinishedResult(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	// Both the result and the cancellation are ready, so select may choose
	// either. A finished check must win every time.
	for i := 0; i < 200; i++ {
		ch := make(chan error, 1)
		ch <- nil
		err, done := awaitCheck(ctx, ch)
		if !done || err != nil {
			t.Fatalf("attempt %d: err=%v done=%v; finished check was dropped", i, err, done)
		}
	}

	ch := make(chan error)
	err, done := awaitCheck(ctx, ch)
	if done || !errors.Is(err, context.Canceled) {
		t.Fatalf("unfinished check: err=%v done=%v, want the cancellation", err, done)
	}
}

func TestFileTargetHungStat(t *testing.T) {
	var calls atomic.Int32
	release := make(chan struct{})
	orig := stat
	stat = func(string) (fs.FileInfo, error) {
		calls.Add(1)
		<-release // like a stat on a hung network mount
		return nil, fs.ErrNotExist
	}
	t.Cleanup(func() { stat = orig })

	tgt := &fileTarget{path: "/hung/mount/file"}
	cfg := fastPoll
	cfg.attemptTimeout = 20 * time.Millisecond
	ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
	defer cancel()
	start := time.Now()
	if pending := waitAll(ctx, []target{tgt}, cfg, nil); len(pending) != 1 {
		t.Fatalf("pending = %v, want the file", pending)
	}
	if elapsed := time.Since(start); elapsed > 2*time.Second {
		t.Errorf("waitAll took %v after a 200ms timeout", elapsed)
	}
	if n := calls.Load(); n != 1 {
		t.Errorf("stat called %d times, want 1: a stat that is still running should not be started again", n)
	}

	// Once the stat returns, the next attempt gets its result.
	close(release)
	if err := tgt.Check(testContext(t)); err == nil || err.Error() != "does not exist" {
		t.Errorf("err = %v, want the result of the hung stat", err)
	}
}

func TestFileTargetDirectory(t *testing.T) {
	dir := t.TempDir()
	tgt := &fileTarget{path: dir, nonempty: true}
	if err := tgt.Check(testContext(t)); err == nil {
		t.Error("empty directory counted as non-empty")
	}
	if err := os.WriteFile(filepath.Join(dir, "x"), nil, 0o644); err != nil {
		t.Fatal(err)
	}
	if err := tgt.Check(testContext(t)); err != nil {
		t.Errorf("directory with an entry: %v", err)
	}
}

func TestWaitForFileToAppearAndGo(t *testing.T) {
	path := filepath.Join(t.TempDir(), "ready")
	time.AfterFunc(50*time.Millisecond, func() { os.WriteFile(path, []byte("1"), 0o644) })
	if pending := waitAll(testContext(t), []target{&fileTarget{path: path}}, fastPoll, nil); pending != nil {
		t.Fatalf("file never appeared: %v", pending[0].err)
	}

	time.AfterFunc(50*time.Millisecond, func() { os.Remove(path) })
	if pending := waitAll(testContext(t), []target{&fileTarget{path: path, gone: true}}, fastPoll, nil); pending != nil {
		t.Fatalf("file never went away: %v", pending[0].err)
	}
}

func TestRunFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "done")
	var stdout, stderr bytes.Buffer
	if code := run([]string{"file", path, "-t", "100ms", "-i", "20ms"}, &stdout, &stderr); code != exitTimeout {
		t.Errorf("missing file: exit %d, want %d", code, exitTimeout)
	}
	if err := os.WriteFile(path, nil, 0o644); err != nil {
		t.Fatal(err)
	}
	stderr.Reset()
	if code := run([]string{path, "-t", "1s"}, &stdout, &stderr); code != exitReady {
		t.Errorf("existing file: exit %d; stderr: %s", code, stderr.String())
	}
	if !strings.HasPrefix(stderr.String(), "waiting for file "+path+"… ready after") {
		t.Errorf("stderr = %q", stderr.String())
	}
}
