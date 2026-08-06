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
		"30s":   30 * time.Second,
		"500ms": 500 * time.Millisecond,
		"1m30s": 90 * time.Second,
		"2":     2 * time.Second,
		"1.5":   1500 * time.Millisecond,
		"0":     0,
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
}
