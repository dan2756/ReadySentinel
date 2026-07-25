package main

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"
)

// httpTarget is ready once a GET request returns an accepted status.
// Redirects are not followed: a 3xx is itself a sign of life, and the
// default range accepts it.
type httpTarget struct {
	url    string
	status statusRange
	client *http.Client
}

func newHTTPTarget(raw string, o *options) (target, error) {
	u, err := url.Parse(raw)
	if err != nil || u.Host == "" {
		return nil, fmt.Errorf("invalid URL %q", raw)
	}
	tr := http.DefaultTransport.(*http.Transport).Clone()
	tr.DisableKeepAlives = true // every attempt should open a fresh connection
	return &httpTarget{
		url:    raw,
		status: o.status,
		client: &http.Client{
			Transport: tr,
			CheckRedirect: func(*http.Request, []*http.Request) error {
				return http.ErrUseLastResponse
			},
		},
	}, nil
}

func (t *httpTarget) String() string { return t.url }

func (t *httpTarget) Check(ctx context.Context) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, t.url, nil)
	if err != nil {
		return err
	}
	req.Header.Set("User-Agent", "waitfor/"+versionString())
	resp, err := t.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if !t.status.contains(resp.StatusCode) {
		return fmt.Errorf("got %s", resp.Status)
	}
	return nil
}

// statusRange is an inclusive range of HTTP status codes, written as "200"
// or "200-299" on the command line.
type statusRange struct {
	lo, hi int
}

var defaultStatus = statusRange{200, 399}

func (r *statusRange) String() string {
	if r.lo == r.hi {
		return strconv.Itoa(r.lo)
	}
	return fmt.Sprintf("%d-%d", r.lo, r.hi)
}

func (r *statusRange) Set(s string) error {
	errBad := errors.New("want a status code like 200 or a range like 200-299")
	loStr, hiStr, isRange := strings.Cut(s, "-")
	lo, err := strconv.Atoi(loStr)
	if err != nil {
		return errBad
	}
	hi := lo
	if isRange {
		if hi, err = strconv.Atoi(hiStr); err != nil {
			return errBad
		}
	}
	if lo < 100 || hi > 599 || lo > hi {
		return errBad
	}
	r.lo, r.hi = lo, hi
	return nil
}

func (r statusRange) contains(code int) bool {
	return code >= r.lo && code <= r.hi
}
