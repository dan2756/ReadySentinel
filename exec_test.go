//go:build unix

package main

import (
	"bytes"
	"errors"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
	"testing"
)

// waitforCmd runs the test binary as if it were the waitfor binary.
func waitforCmd(args ...string) *exec.Cmd {
	cmd := exec.Command(os.Args[0], args...)
	cmd.Env = append(os.Environ(), "WAITFOR_TEST_MAIN=1")
	return cmd
}

func exitCode(t *testing.T, err error) int {
	t.Helper()
	if err == nil {
		return 0
	}
	var ee *exec.ExitError
	if !errors.As(err, &ee) {
		t.Fatal(err)
	}
	return ee.ExitCode()
}

func TestCommandRunsOnceReady(t *testing.T) {
	ln := listen(t)
	cmd := waitforCmd(ln.Addr().String(), "-q", "-t", "5s", "--", "sh", "-c", "echo hello; exit 7")
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	out, err := cmd.Output()
	if code := exitCode(t, err); code != 7 {
		t.Errorf("exit %d, want the command's 7; stderr: %s", code, stderr.String())
	}
	if string(out) != "hello\n" {
		t.Errorf("stdout = %q, want %q", out, "hello\n")
	}
}

func TestCommandSkippedOnTimeout(t *testing.T) {
	marker := filepath.Join(t.TempDir(), "ran")
	cmd := waitforCmd(closedAddr(t), "-q", "-t", "200ms", "-i", "20ms", "--", "touch", marker)
	if code := exitCode(t, cmd.Run()); code != exitTimeout {
		t.Errorf("exit %d, want %d", code, exitTimeout)
	}
	if _, err := os.Stat(marker); err == nil {
		t.Error("command ran even though the target timed out")
	}
}

func TestCommandCannotRun(t *testing.T) {
	ln := listen(t)
	script := filepath.Join(t.TempDir(), "script.sh")
	if err := os.WriteFile(script, []byte("#!/bin/sh\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	tests := []struct {
		command string
		want    int
	}{
		{"waitfor-no-such-command", exitNotFound},
		{filepath.Join(t.TempDir(), "missing"), exitNotFound},
		{script, exitCannotRun}, // not executable
	}
	for _, tt := range tests {
		cmd := waitforCmd(ln.Addr().String(), "-q", "--", tt.command)
		var stderr bytes.Buffer
		cmd.Stderr = &stderr
		if code := exitCode(t, cmd.Run()); code != tt.want {
			t.Errorf("%s: exit %d, want %d", tt.command, code, tt.want)
		}
		if !strings.HasPrefix(stderr.String(), "waitfor: ") {
			t.Errorf("%s: stderr = %q", tt.command, stderr.String())
		}
	}
}

func TestInterrupt(t *testing.T) {
	for _, tt := range []struct {
		sig  os.Signal
		want int
	}{
		{os.Interrupt, 130},
		{syscall.SIGTERM, 143},
	} {
		cmd := waitforCmd(closedAddr(t), "-i", "20ms")
		stderr, err := cmd.StderrPipe()
		if err != nil {
			t.Fatal(err)
		}
		if err := cmd.Start(); err != nil {
			t.Fatal(err)
		}
		// The first progress message is printed after the signal handler
		// is installed, so wait for it before sending the signal.
		buf := make([]byte, 256)
		n, err := stderr.Read(buf)
		if err != nil {
			cmd.Process.Kill()
			t.Fatal(err)
		}
		if err := cmd.Process.Signal(tt.sig); err != nil {
			t.Fatal(err)
		}
		rest, _ := io.ReadAll(stderr)
		code := exitCode(t, cmd.Wait())
		output := string(buf[:n]) + string(rest)
		if code != tt.want {
			t.Errorf("%v: exit %d, want %d; stderr: %s", tt.sig, code, tt.want, output)
		}
		if !strings.HasSuffix(output, "… interrupted\n") {
			t.Errorf("%v: stderr = %q", tt.sig, output)
		}
	}
}
