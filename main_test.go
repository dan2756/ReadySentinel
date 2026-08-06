package main

import (
	"bytes"
	"os"
	"regexp"
	"strings"
	"testing"
	"time"
)

// TestMain lets the test binary double as a helper process: with
// WAITFOR_TEST_MAIN=1 it behaves exactly like the waitfor binary, and with
// WAITFOR_TEST_SLEEP set it just sleeps that long and exits.
func TestMain(m *testing.M) {
	if os.Getenv("WAITFOR_TEST_MAIN") == "1" {
		os.Exit(run(os.Args[1:], os.Stdout, os.Stderr))
	}
	if s := os.Getenv("WAITFOR_TEST_SLEEP"); s != "" {
		d, _ := time.ParseDuration(s)
		time.Sleep(d)
		os.Exit(0)
	}
	os.Exit(m.Run())
}

func TestRunReady(t *testing.T) {
	ln := listen(t)
	var stdout, stderr bytes.Buffer
	code := run([]string{ln.Addr().String(), "-t", "5s"}, &stdout, &stderr)
	if code != exitReady {
		t.Fatalf("exit %d, want 0; stderr: %s", code, stderr.String())
	}
	re := regexp.MustCompile(`^waiting for 127\.0\.0\.1:\d+… ready after \d+\.\ds\n$`)
	if !re.MatchString(stderr.String()) {
		t.Errorf("stderr = %q", stderr.String())
	}
}

func TestRunSeveralTargets(t *testing.T) {
	a, b := listen(t), listen(t)
	var stdout, stderr bytes.Buffer
	code := run([]string{a.Addr().String(), b.Addr().String(), "-t", "5s"}, &stdout, &stderr)
	if code != exitReady {
		t.Fatalf("exit %d, want 0; stderr: %s", code, stderr.String())
	}
	if n := strings.Count(stderr.String(), "ready after"); n != 2 {
		t.Errorf("stderr reports %d ready targets, want 2:\n%s", n, stderr.String())
	}
}

func TestRunAny(t *testing.T) {
	open, closed := listen(t), closedAddr(t)
	var stdout, stderr bytes.Buffer
	code := run([]string{"--any", closed, open.Addr().String(), "-t", "5s"}, &stdout, &stderr)
	if code != exitReady {
		t.Fatalf("exit %d, want 0; stderr: %s", code, stderr.String())
	}
	want := "waiting for any of " + closed + ", " + open.Addr().String() + "…\n" + open.Addr().String() + " ready after"
	if !strings.HasPrefix(stderr.String(), want) {
		t.Errorf("stderr = %q, want prefix %q", stderr.String(), want)
	}

	stderr.Reset()
	if code := run([]string{closed, open.Addr().String(), "-t", "200ms", "-i", "20ms"}, &stdout, &stderr); code != exitTimeout {
		t.Errorf("without --any: exit %d, want %d", code, exitTimeout)
	}
	if !strings.Contains(stderr.String(), "still waiting for "+closed+"\n") {
		t.Errorf("stderr = %q, want it to name the closed port", stderr.String())
	}
}

func TestRunTimeout(t *testing.T) {
	addr := closedAddr(t)
	var stdout, stderr bytes.Buffer
	code := run([]string{addr, "-t", "200ms", "-i", "20ms"}, &stdout, &stderr)
	if code != exitTimeout {
		t.Fatalf("exit %d, want %d", code, exitTimeout)
	}
	if !strings.Contains(stderr.String(), "timed out after") {
		t.Errorf("stderr = %q, want a timeout message", stderr.String())
	}
}

func TestRunQuiet(t *testing.T) {
	ln := listen(t)
	var stdout, stderr bytes.Buffer
	if code := run([]string{"-q", ln.Addr().String()}, &stdout, &stderr); code != exitReady {
		t.Fatalf("exit %d, want 0", code)
	}
	if stderr.Len() != 0 || stdout.Len() != 0 {
		t.Errorf("quiet run printed stdout=%q stderr=%q", stdout.String(), stderr.String())
	}
}

func TestRunUsageErrors(t *testing.T) {
	for _, args := range [][]string{
		{},
		{"--bogus", "5432"},
		{"not-a-target"},
		{"-t", "soon", "5432"},
		{"-i", "0", "5432"},
	} {
		var stdout, stderr bytes.Buffer
		if code := run(args, &stdout, &stderr); code != exitUsage {
			t.Errorf("run(%q) = %d, want %d", args, code, exitUsage)
		}
		if !strings.HasPrefix(stderr.String(), "waitfor: ") {
			t.Errorf("run(%q) stderr = %q", args, stderr.String())
		}
	}
}

func TestRunHelpAndVersion(t *testing.T) {
	for _, flag := range []string{"-h", "--help"} {
		var stdout, stderr bytes.Buffer
		if code := run([]string{flag}, &stdout, &stderr); code != exitReady {
			t.Errorf("%s: exit %d", flag, code)
		}
		if !strings.Contains(stdout.String(), "Usage:") {
			t.Errorf("%s: stdout = %q", flag, stdout.String())
		}
	}
	var stdout, stderr bytes.Buffer
	if code := run([]string{"--version"}, &stdout, &stderr); code != exitReady {
		t.Errorf("--version: exit %d", code)
	}
	if !strings.HasPrefix(stdout.String(), "waitfor ") {
		t.Errorf("--version: stdout = %q", stdout.String())
	}
}
