package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
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
