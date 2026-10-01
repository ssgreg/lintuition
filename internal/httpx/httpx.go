// Package httpx is the HTTP transport shared by the remote classifier backends: https only (plain
// http only to loopback), redirects never followed, retries on 429, 5xx and transport errors with
// Retry-After honoured, every retry cleared with the run's budget, a per-minute rate limit, and
// errors that never carry a request or response body.
package httpx

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math"
	"math/rand/v2"
	"net"
	"net/http"
	"net/url"
	"strconv"
	"sync"
	"time"
)

// Client posts JSON to one endpoint.
type Client struct {
	endpoint string
	http     *http.Client
	retries  int
	limiter  *Limiter
	// Sleep waits; tests replace it.
	Sleep func(context.Context, time.Duration) error
}

// Options configure a Client; zero values take the defaults.
type Options struct {
	Timeout           string // per attempt, default 60s
	MaxRetries        *int   // default 4, 0 to 10
	RequestsPerMinute int    // default 1000
}

// New validates the endpoint and options.
func New(endpoint string, o Options) (*Client, error) {
	u, err := url.Parse(endpoint)
	if err != nil {
		return nil, fmt.Errorf("endpoint: %w", err)
	}
	// Plain HTTP only to a loopback address (a local model server, a test server, a local proxy).
	if u.Scheme != "https" && (u.Scheme != "http" || !IsLoopback(u.Hostname())) {
		return nil, fmt.Errorf("endpoint %q: https is required", endpoint)
	}
	c := &Client{endpoint: endpoint, retries: 4, Sleep: sleepCtx}
	timeout := 60 * time.Second
	if o.Timeout != "" {
		if timeout, err = time.ParseDuration(o.Timeout); err != nil || timeout <= 0 {
			return nil, fmt.Errorf("timeout %q: must be a positive duration", o.Timeout)
		}
	}
	c.http = &http.Client{
		Timeout: timeout,
		// Never follow a redirect: it would resend the body and the key to a place New did not check.
		CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse },
	}
	if o.MaxRetries != nil {
		if *o.MaxRetries < 0 || *o.MaxRetries > 10 {
			return nil, fmt.Errorf("max-retries %d: must be 0 to 10", *o.MaxRetries)
		}
		c.retries = *o.MaxRetries
	}
	rpm := o.RequestsPerMinute
	if rpm == 0 {
		rpm = 1000
	}
	if rpm < 0 {
		return nil, fmt.Errorf("requests-per-minute %d: must be positive", rpm)
	}
	c.limiter = NewLimiter(rpm)
	return c, nil
}

// Endpoint is the URL the client posts to.
func (c *Client) Endpoint() string { return c.endpoint }

// Local reports whether the endpoint is on this machine.
func (c *Client) Local() bool {
	u, err := url.Parse(c.endpoint)
	return err == nil && IsLoopback(u.Hostname())
}

// IsLoopback reports whether host is localhost or a loopback address.
func IsLoopback(host string) bool {
	if host == "localhost" {
		return true
	}
	ip := net.ParseIP(host)
	return ip != nil && ip.IsLoopback()
}

// APIError is a failure the service reported, including a redirect, which is never followed. It
// never carries the request or response body: a service may echo the input, and the input may be
// private.
type APIError struct {
	Status int
}

func (e APIError) Error() string {
	return fmt.Sprintf("HTTP %d %s", e.Status, http.StatusText(e.Status))
}

// Post sends the body with the headers, retrying 429, 5xx and transport errors. Every attempt after
// the first is first cleared with spend (the run's budget); a refusal stops the retries with its
// error.
//
// start, when set, is called right before every attempt goes out, after the rate limiter's wait;
// an error stops the call with that error.
func (c *Client) Post(ctx context.Context, body []byte, header http.Header, spend, start func() error) ([]byte, error) {
	var last error
	for attempt := 0; attempt <= c.retries; attempt++ {
		if attempt > 0 && spend != nil {
			if err := spend(); err != nil {
				return nil, fmt.Errorf("%w; retry not sent: %w", last, err)
			}
		}
		if err := c.limiter.Wait(ctx, c.Sleep); err != nil {
			return nil, err
		}
		if start != nil {
			if err := start(); err != nil {
				if last != nil {
					return nil, fmt.Errorf("%w; retry not sent: %w", last, err)
				}
				return nil, err
			}
		}
		raw, ra, err := c.once(ctx, body, header)
		if err == nil {
			return raw, nil
		}
		last = err
		var ae APIError
		retryable := !errors.As(err, &ae) || ae.Status == http.StatusTooManyRequests || ae.Status >= 500
		if !retryable || attempt == c.retries || ctx.Err() != nil {
			break
		}
		wait := ra
		if wait == 0 {
			// Exponential backoff with jitter: 0.5-1.5 s, 1-3 s, 2-6 s, ...
			base := time.Duration(1<<attempt) * time.Second
			wait = base/2 + time.Duration(rand.Int64N(int64(base)))
		}
		// The server's delay is honoured as given; the run's context bounds the total wait.
		if dl, ok := ctx.Deadline(); ok && time.Until(dl) < wait {
			return nil, fmt.Errorf("%w; the requested retry delay of %s exceeds the run's remaining time", last, wait.Round(time.Second))
		}
		if err := c.Sleep(ctx, wait); err != nil {
			return nil, err
		}
	}
	return nil, last
}

func (c *Client) once(ctx context.Context, body []byte, header http.Header) ([]byte, time.Duration, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.endpoint, bytes.NewReader(body))
	if err != nil {
		return nil, 0, err
	}
	for k, vs := range header {
		for _, v := range vs {
			req.Header.Add(k, v)
		}
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := c.http.Do(req)
	if err != nil {
		// url.Error names the endpoint and the cause; credentials travel in headers and are not in it.
		return nil, 0, err
	}
	defer func() { _ = resp.Body.Close() }() // read to the end or the limit; nothing to report on close
	raw, err := io.ReadAll(io.LimitReader(resp.Body, 8<<20))
	if err != nil {
		return nil, 0, err
	}
	if resp.StatusCode != http.StatusOK {
		return nil, RetryAfter(resp.Header.Get("Retry-After"), time.Now()), APIError{Status: resp.StatusCode}
	}
	return raw, 0, nil
}

// NoDuplicateKeys refuses a response with a repeated key in any JSON object (two "answers", two
// answers to one question, two "choice" fields): a decoder would silently keep the last one.
func NoDuplicateKeys(raw []byte) error {
	dec := json.NewDecoder(bytes.NewReader(raw))
	var walk func() error
	walk = func() error {
		t, err := dec.Token()
		if err != nil {
			return err
		}
		switch t {
		case json.Delim('{'):
			seen := map[string]bool{}
			for dec.More() {
				kt, err := dec.Token()
				if err != nil {
					return err
				}
				k, _ := kt.(string)
				if seen[k] {
					return errDuplicate
				}
				seen[k] = true
				if err := walk(); err != nil {
					return err
				}
			}
			_, err = dec.Token()
			return err
		case json.Delim('['):
			for dec.More() {
				if err := walk(); err != nil {
					return err
				}
			}
			_, err = dec.Token()
			return err
		}
		return nil
	}
	if err := walk(); err != nil {
		if errors.Is(err, errDuplicate) {
			return errors.New("the response repeats a key in one object")
		}
		return errors.New("the response is not valid JSON")
	}
	return nil
}

var errDuplicate = errors.New("duplicate key")

// RetryAfter parses Retry-After in both forms HTTP allows (RFC 9110, 10.2.3): delay seconds or an
// HTTP date. Zero means none was given, or the date has passed.
func RetryAfter(v string, now time.Time) time.Duration {
	if v == "" {
		return 0
	}
	if s, err := strconv.ParseFloat(v, 64); err == nil {
		if s <= 0 || math.IsNaN(s) || math.IsInf(s, 0) {
			return 0
		}
		return time.Duration(s * float64(time.Second))
	}
	if t, err := http.ParseTime(v); err == nil && t.After(now) {
		return t.Sub(now)
	}
	return 0
}

func sleepCtx(ctx context.Context, d time.Duration) error {
	t := time.NewTimer(d)
	defer t.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-t.C:
		return nil
	}
}

// Limiter spaces requests evenly to a rate per minute.
type Limiter struct {
	mu       sync.Mutex
	interval time.Duration
	next     time.Time
	// Now is the clock; tests replace it.
	Now func() time.Time
}

func NewLimiter(rpm int) *Limiter {
	return &Limiter{interval: time.Minute / time.Duration(rpm), Now: time.Now}
}

func (l *Limiter) Wait(ctx context.Context, sleep func(context.Context, time.Duration) error) error {
	l.mu.Lock()
	now := l.Now()
	at := l.next
	if at.Before(now) {
		at = now
	}
	l.next = at.Add(l.interval)
	l.mu.Unlock()
	if d := at.Sub(now); d > 0 {
		return sleep(ctx, d)
	}
	return nil
}
