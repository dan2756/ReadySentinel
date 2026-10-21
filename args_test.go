package main

import (
	"reflect"
	"testing"
	"time"
)

func TestParseArgsMixedOrder(t *testing.T) {
	o, err := parseArgs([]string{"db:5432", "-t", "30s", "cache:6379", "--quiet", "--interval=250ms"})
	if err != nil {
		t.Fatal(err)
	}
	if want := []string{"db:5432", "cache:6379"}; !reflect.DeepEqual(o.words, want) {
		t.Errorf("words = %q, want %q", o.words, want)
	}
	if o.timeout != 30*time.Second {
		t.Errorf("timeout = %v, want 30s", o.timeout)
	}
	if o.interval != 250*time.Millisecond {
		t.Errorf("interval = %v, want 250ms", o.interval)
	}
	if !o.quiet {
		t.Error("quiet not set")
	}
}

func TestParseArgsCommand(t *testing.T) {
	o, err := parseArgs([]string{"db:5432", "-t", "5s", "--", "npm", "start", "--", "-q"})
	if err != nil {
		t.Fatal(err)
	}
	if want := []string{"db:5432"}; !reflect.DeepEqual(o.words, want) {
		t.Errorf("words = %q, want %q", o.words, want)
	}
	if want := []string{"npm", "start", "--", "-q"}; !reflect.DeepEqual(o.command, want) {
		t.Errorf("command = %q, want %q", o.command, want)
	}
	if o.quiet {
		t.Error("-q after -- was parsed as a flag")
	}

	if _, err := parseArgs([]string{"db:5432", "--"}); err == nil {
		t.Error("empty command after --: want error")
	}
}

func TestParseArgsOperandLooksLikeFlag(t *testing.T) {
	o, err := parseArgs([]string{"--gone", "file", "-deploy.lock", "-t", "5s", "pid", "-5"})
	if err != nil {
		t.Fatal(err)
	}
	if want := []string{"file", "-deploy.lock", "pid", "-5"}; !reflect.DeepEqual(o.words, want) {
		t.Errorf("words = %q, want %q", o.words, want)
	}
	if !o.gone || o.timeout != 5*time.Second {
		t.Errorf("flags: gone=%v timeout=%v", o.gone, o.timeout)
	}
	// The operand is kept, and the pid parser still rejects it.
	if _, err := parseTargets(o.words, o); err == nil {
		t.Fatal("pid -5: want an invalid pid error")
	}

	o, err = parseArgs([]string{"file", "-ready", "--", "echo", "hi"})
	if err != nil {
		t.Fatal(err)
	}
	if want := []string{"file", "-ready"}; !reflect.DeepEqual(o.words, want) {
		t.Errorf("words = %q, want %q", o.words, want)
	}
	if want := []string{"echo", "hi"}; !reflect.DeepEqual(o.command, want) {
		t.Errorf("command = %q, want %q", o.command, want)
	}

	if _, err := parseArgs([]string{"-secret", "5432"}); err == nil {
		t.Error("a leading -secret is still an unknown flag")
	}
}

func TestParseArgsDefaults(t *testing.T) {
	o, err := parseArgs([]string{"5432"})
	if err != nil {
		t.Fatal(err)
	}
	if o.timeout != 0 || o.interval != 500*time.Millisecond || o.quiet {
		t.Errorf("unexpected defaults: %+v", o)
	}
}

func TestParseDuration(t *testing.T) {
	good := map[string]time.Duration{
		"30s":       30 * time.Second,
		"500ms":     500 * time.Millisecond,
		"1m30s":     90 * time.Second,
		"2":         2 * time.Second,
		"1.5":       1500 * time.Millisecond,
		"0":         0,
		"100000000": 100000000 * time.Second, // fits; about three years
	}
	for in, want := range good {
		got, err := parseDuration(in)
		if err != nil || got != want {
			t.Errorf("parseDuration(%q) = %v, %v; want %v", in, got, err, want)
		}
	}
	for _, in := range []string{"", "soon", "-1s", "1.2.3", "."} {
		if got, err := parseDuration(in); err == nil {
			t.Errorf("parseDuration(%q) = %v, want error", in, got)
		}
	}
	// Bigger than math.MaxInt64 nanoseconds. The conversion used to wrap
	// to a negative duration, which -t treats as "wait forever".
	for _, in := range []string{"100000000000", "9223372037"} {
		got, err := parseDuration(in)
		if err == nil || got < 0 {
			t.Errorf("parseDuration(%q) = %v, %v; want a too-large error", in, got, err)
		}
	}
}
