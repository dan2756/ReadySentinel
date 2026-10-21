package main

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
)

// maxBody caps how much of a response body --contains searches.
const maxBody = 1 << 20

// httpTarget is ready once a GET request returns an accepted status and, if
// contains is set, a body containing that text. Redirects are not followed:
// a 3xx is itself a sign of life, and the default range accepts it.
type httpTarget struct {
	url      string
	status   statusRange
	contains string
	client   *http.Client
}

func newHTTPTarget(raw string, o *options) (target, error) {
	u, err := url.Parse(raw)
	if err != nil || u.Host == "" {
		return nil, fmt.Errorf("invalid URL %q", raw)
	}
	// url.Parse accepts ports it can never dial. A TCP target rejects those
	// up front; an HTTP target used to retry them until --timeout.
	if port := u.Port(); port != "" {
		n, nerr := strconv.Atoi(port)
		if nerr != nil || n < 1 || n > 65535 {
			return nil, fmt.Errorf("invalid port %q in %q", port, raw)
		}
	}
	tr := http.DefaultTransport.(*http.Transport).Clone()
	tr.DisableKeepAlives = true // every attempt should open a fresh connection
	return &httpTarget{
		url:      raw,
		status:   o.status,
		contains: o.contains,
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
	if t.contains != "" {
		found, err := bodyContains(resp.Body, t.contains)
		switch {
		case found:
			return nil
		case errors.Is(err, context.DeadlineExceeded):
			// There was a response, so don't let this be reported as
			// "no response": its body was still coming when time ran out.
			return fmt.Errorf("body does not contain %q; it was still being sent", t.contains)
		case err != nil:
			return fmt.Errorf("reading body: %w", err)
		}
		return fmt.Errorf("body does not contain %q", t.contains)
	}
	return nil
}

// bodyContains reports whether the first maxBody bytes of r contain text. It
// returns as soon as text has been read, so a body that is still being
// streamed can match.
func bodyContains(r io.Reader, text string) (bool, error) {
	r = io.LimitReader(r, maxBody)
	var body []byte
	buf := make([]byte, 32<<10)
	for {
		n, err := r.Read(buf)
		if n > 0 {
			// Search only the new bytes, plus enough of the old ones to
			// catch a match split across two reads.
			from := max(0, len(body)-len(text)+1)
			body = append(body, buf[:n]...)
			if bytes.Contains(body[from:], []byte(text)) {
				return true, nil
			}
		}
		if err == io.EOF {
			return false, nil
		}
		if err != nil {
			return false, err
		}
	}
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
