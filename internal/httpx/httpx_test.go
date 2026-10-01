package httpx

import (
	"context"
	"net/http"
	"testing"
	"time"
)

func TestRetryAfter(t *testing.T) {
	now := time.Date(2026, 10, 1, 12, 0, 0, 0, time.UTC)
	for v, want := range map[string]time.Duration{
		"120":  120 * time.Second,
		"1.5":  1500 * time.Millisecond,
		"0":    0,
		"-3":   0,
		"soon": 0,
		now.Add(2 * time.Minute).Format(http.TimeFormat):  2 * time.Minute,
		now.Add(-2 * time.Minute).Format(http.TimeFormat): 0,
	} {
		if got := RetryAfter(v, now); got != want {
			t.Errorf("RetryAfter(%q) = %v, want %v", v, got, want)
		}
	}
}

func TestLimiterSpacesRequests(t *testing.T) {
	l := NewLimiter(60) // one a second
	now := time.Unix(0, 0)
	l.Now = func() time.Time { return now }
	var waits []time.Duration
	sleep := func(_ context.Context, d time.Duration) error { waits = append(waits, d); return nil }
	for range 3 {
		l.Wait(context.Background(), sleep)
	}
	if len(waits) != 2 || waits[0] != time.Second || waits[1] != 2*time.Second {
		t.Fatalf("waits %v", waits)
	}
}

func TestEndpointRules(t *testing.T) {
	for url, ok := range map[string]bool{"https://api.example.com/v1": true, "http://127.0.0.1:11434/v1": true, "http://localhost:8080": true, "http://api.example.com/v1": false, "ftp://x": false} {
		if _, err := New(url, Options{}); (err == nil) != ok {
			t.Errorf("%s: err %v", url, err)
		}
	}
	c, _ := New("http://127.0.0.1:1/v1", Options{})
	if !c.Local() {
		t.Error("loopback is local")
	}
	_ = http.StatusOK
}
