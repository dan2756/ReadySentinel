//go:build unix

package main

import (
	"bytes"
	"errors"
	"os"
	"os/exec"
	"strconv"
	"syscall"
	"testing"
	"time"
)

// startSleeper starts a child process that sleeps for d, then exits. Like a
// well-behaved parent, the test reaps it as soon as it exits; exited is
// closed once that has happened.
func startSleeper(t *testing.T, d time.Duration) (pid int, exited <-chan struct{}) {
	t.Helper()
	cmd := exec.Command(os.Args[0])
	cmd.Env = append(os.Environ(), "WAITFOR_TEST_SLEEP="+d.String())
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	done := make(chan struct{})
	go func() {
		cmd.Wait()
		close(done)
	}()
	t.Cleanup(func() {
		cmd.Process.Kill()
		<-done
	})
	return cmd.Process.Pid, done
}

func TestParsePID(t *testing.T) {
	ts, err := parseTargets([]string{"pid", "1234"}, defaultOptions(t))
	if err != nil {
		t.Fatal(err)
	}
	if len(ts) != 1 || ts[0].String() != "pid 1234" {
		t.Errorf("targets = %v", ts)
	}
	for _, words := range [][]string{{"pid"}, {"pid", "abc"}, {"pid", "0"}, {"pid", "-5"}} {
		if _, err := parseTargets(words, defaultOptions(t)); err == nil {
			t.Errorf("parseTargets(%q): want error", words)
		}
	}
}

func TestPIDTargetRunning(t *testing.T) {
	tgt := &pidTarget{pid: os.Getpid()}
	if err := tgt.Check(testContext(t)); err == nil {
		t.Error("our own process reported as exited")
	}
}

func TestPIDTargetSignalErrors(t *testing.T) {
	orig := signalZero
	t.Cleanup(func() { signalZero = orig })

	tests := []struct {
		err   error
		ready bool
	}{
		{nil, false},
		{syscall.EPERM, false}, // exists, owned by someone else
		{syscall.ESRCH, true},
		{syscall.EINVAL, false},
	}
	for _, tt := range tests {
		signalZero = func(int) error { return tt.err }
		err := (&pidTarget{pid: 42}).Check(testContext(t))
		if (err == nil) != tt.ready {
			t.Errorf("kill returned %v: Check = %v, want ready=%v", tt.err, err, tt.ready)
		}
		if tt.err == syscall.EINVAL && !errors.Is(err, syscall.EINVAL) {
			t.Errorf("unexpected kill error was not passed on: %v", err)
		}
	}
}

func TestPIDTargetOtherUser(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("root may signal every process")
	}
	// pid 1 (init, systemd or launchd) is owned by root, so kill(1, 0)
	// fails with EPERM.
	if err := (&pidTarget{pid: 1}).Check(testContext(t)); err == nil {
		t.Error("pid 1 reported as exited")
	}
}

func TestPIDTargetExits(t *testing.T) {
	pid, exited := startSleeper(t, 200*time.Millisecond)
	tgt := &pidTarget{pid: pid}
	if err := tgt.Check(testContext(t)); err == nil {
		t.Fatal("sleeping child reported as exited")
	}
	if pending := waitAll(testContext(t), []target{tgt}, fastPoll, nil); pending != nil {
		t.Fatalf("child never exited: %v", pending[0].err)
	}
	select {
	case <-exited:
	case <-time.After(2 * time.Second):
		t.Error("waitAll returned but the child is still running")
	}
}

func TestPIDTargetAlreadyGone(t *testing.T) {
	pid, exited := startSleeper(t, 0)
	<-exited
	if err := (&pidTarget{pid: pid}).Check(testContext(t)); err != nil {
		t.Errorf("exited child: %v", err)
	}
}

func TestRunPID(t *testing.T) {
	pid, _ := startSleeper(t, 100*time.Millisecond)
	var stdout, stderr bytes.Buffer
	code := run([]string{"pid", strconv.Itoa(pid), "-t", "5s", "-i", "20ms"}, &stdout, &stderr)
	if code != exitReady {
		t.Fatalf("exit %d; stderr: %s", code, stderr.String())
	}
}
