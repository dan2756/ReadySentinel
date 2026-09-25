package main

import (
	"bytes"
	"io"
	"log"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

func TestStatusRangeSet(t *testing.T) {
	good := map[string]statusRange{
		"200":     {200, 200},
		"200-299": {200, 299},
		"100-599": {100, 599},
	}
	for in, want := range good {
		var r statusRange
		if err := r.Set(in); err != nil || r != want {
			t.Errorf("Set(%q) = %v, %v; want %v", in, r, err, want)
		}
		if r.String() != in {
			t.Errorf("String() = %q, want %q", r.String(), in)
		}
	}
	for _, in := range []string{"", "ok", "99", "600", "300-200", "200-", "-200", "2xx"} {
		var r statusRange
		if err := r.Set(in); err == nil {
			t.Errorf("Set(%q) = %v, want error", in, r)
		}
	}
}

func TestParseTargetHTTP(t *testing.T) {
	for _, in := range []string{"http://localhost:8080/health", "https://example.com", "HTTP://example.com/"} {
		got, err := parseTarget(in, defaultOptions(t))
		if err != nil {
			t.Errorf("parseTarget(%q): %v", in, err)
			continue
		}
		if _, ok := got.(*httpTarget); !ok || got.String() != in {
			t.Errorf("parseTarget(%q) = %T %q", in, got, got)
		}
	}
	if _, err := parseTarget("http://", defaultOptions(t)); err == nil {
		t.Error(`parseTarget("http://"): want error`)
	}
}

func statusServer(t *testing.T) *httptest.Server {
	t.Helper()
	mux := http.NewServeMux()
	mux.HandleFunc("/ok", func(w http.ResponseWriter, r *http.Request) {
		if ua := r.Header.Get("User-Agent"); !strings.HasPrefix(ua, "waitfor/") {
			t.Errorf("User-Agent = %q", ua)
		}
		w.Write([]byte("ok"))
	})
	mux.HandleFunc("/moved", func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, "/down", http.StatusFound)
	})
	mux.HandleFunc("/down", func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "warming up", http.StatusServiceUnavailable)
	})
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	return srv
}

func newTestHTTPTarget(t *testing.T, url string, status statusRange) *httpTarget {
	t.Helper()
	o := defaultOptions(t)
	o.status = status
	tgt, err := newHTTPTarget(url, o)
	if err != nil {
		t.Fatal(err)
	}
	return tgt.(*httpTarget)
}

func TestHTTPTargetStatus(t *testing.T) {
	srv := statusServer(t)
	tests := []struct {
		path   string
		status statusRange
		ready  bool
	}{
		{"/ok", defaultStatus, true},
		{"/moved", defaultStatus, true}, // redirects are not followed
		{"/down", defaultStatus, false},
		{"/missing", defaultStatus, false},
		{"/moved", statusRange{200, 200}, false},
		{"/down", statusRange{500, 599}, true},
		{"/missing", statusRange{404, 404}, true},
	}
	for _, tt := range tests {
		tgt := newTestHTTPTarget(t, srv.URL+tt.path, tt.status)
		err := tgt.Check(testContext(t))
		if (err == nil) != tt.ready {
			t.Errorf("%s with --status %s: err = %v, want ready=%v", tt.path, &tt.status, err, tt.ready)
		}
	}
}

func TestHTTPTargetErrorMentionsStatus(t *testing.T) {
	srv := statusServer(t)
	err := newTestHTTPTarget(t, srv.URL+"/down", defaultStatus).Check(testContext(t))
	if err == nil || !strings.Contains(err.Error(), "503") {
		t.Errorf("err = %v, want it to mention 503", err)
	}
}

func TestHTTPTargetBecomesReady(t *testing.T) {
	var hits atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if hits.Add(1) < 3 {
			w.WriteHeader(http.StatusServiceUnavailable)
		}
	}))
	defer srv.Close()

	tgt := newTestHTTPTarget(t, srv.URL, defaultStatus)
	if pending := waitAll(testContext(t), []target{tgt}, fastPoll, nil); pending != nil {
		t.Fatalf("not ready: %v", pending[0].err)
	}
	if n := hits.Load(); n != 3 {
		t.Errorf("server saw %d requests, want 3", n)
	}
}

func TestHTTPTargetContains(t *testing.T) {
	var hits atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if hits.Add(1) < 3 {
			io.WriteString(w, `{"status":"starting"}`)
			return
		}
		io.WriteString(w, `{"status":"ok"}`)
	}))
	defer srv.Close()

	tgt := newTestHTTPTarget(t, srv.URL, defaultStatus)
	tgt.contains = `"status":"ok"`
	if err := tgt.Check(testContext(t)); err == nil || !strings.Contains(err.Error(), "does not contain") {
		t.Errorf("first attempt: err = %v, want a body mismatch", err)
	}
	if pending := waitAll(testContext(t), []target{tgt}, fastPoll, nil); pending != nil {
		t.Fatalf("not ready: %v", pending[0].err)
	}
	if n := hits.Load(); n != 3 {
		t.Errorf("server saw %d requests, want 3", n)
	}
}

func TestHTTPTargetContainsNeedsStatus(t *testing.T) {
	srv := statusServer(t)
	tgt := newTestHTTPTarget(t, srv.URL+"/down", defaultStatus)
	tgt.contains = "warming up"
	if err := tgt.Check(testContext(t)); err == nil || !strings.Contains(err.Error(), "503") {
		t.Errorf("err = %v, want the status to be checked first", err)
	}
}

func TestHTTPTargetContainsLimit(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write(bytes.Repeat([]byte("x"), maxBody))
		io.WriteString(w, "ready")
	}))
	defer srv.Close()

	tgt := newTestHTTPTarget(t, srv.URL, defaultStatus)
	tgt.contains = "ready"
	if err := tgt.Check(testContext(t)); err == nil {
		t.Error("matched text past the body size limit")
	}
}

func TestHTTPTargetAttemptTimeout(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		select {
		case <-r.Context().Done():
		case <-time.After(5 * time.Second):
		}
	}))
	defer srv.Close()

	tgt := newTestHTTPTarget(t, srv.URL, defaultStatus)
	start := time.Now()
	if err := attempt(testContext(t), tgt, 50*time.Millisecond); err == nil {
		t.Fatal("want a timeout error")
	}
	if elapsed := time.Since(start); elapsed > 2*time.Second {
		t.Errorf("attempt took %v, want about 50ms", elapsed)
	}
}

func TestHTTPSTarget(t *testing.T) {
	srv := httptest.NewUnstartedServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
	srv.Config.ErrorLog = log.New(io.Discard, "", 0) // quiet the expected handshake failure
	srv.StartTLS()
	defer srv.Close()

	tgt := newTestHTTPTarget(t, srv.URL, defaultStatus)
	if err := tgt.Check(testContext(t)); err == nil {
		t.Error("untrusted certificate: want error")
	}
	tgt.client.Transport = srv.Client().Transport
	if err := tgt.Check(testContext(t)); err != nil {
		t.Errorf("trusted certificate: %v", err)
	}
}

func TestRunHTTP(t *testing.T) {
	srv := statusServer(t)
	var stdout, stderr bytes.Buffer
	if code := run([]string{srv.URL + "/ok", "-t", "5s"}, &stdout, &stderr); code != exitReady {
		t.Errorf("/ok: exit %d; stderr: %s", code, stderr.String())
	}
	stderr.Reset()
	if code := run([]string{srv.URL + "/down", "--status", "200-299", "-t", "200ms", "-i", "20ms"}, &stdout, &stderr); code != exitTimeout {
		t.Errorf("/down: exit %d, want %d; stderr: %s", code, exitTimeout, stderr.String())
	}
	stderr.Reset()
	if code := run([]string{srv.URL + "/ok", "--contains", "ok", "-t", "5s"}, &stdout, &stderr); code != exitReady {
		t.Errorf("/ok --contains ok: exit %d; stderr: %s", code, stderr.String())
	}
	stderr.Reset()
	if code := run([]string{srv.URL + "/ok", "--contains", "nope", "-t", "200ms", "-i", "20ms"}, &stdout, &stderr); code != exitTimeout {
		t.Errorf("/ok --contains nope: exit %d, want %d", code, exitTimeout)
	}
	stderr.Reset()
	if code := run([]string{srv.URL + "/ok", "--status", "2xx"}, &stdout, &stderr); code != exitUsage {
		t.Errorf("--status 2xx: exit %d, want %d", code, exitUsage)
	}
}
