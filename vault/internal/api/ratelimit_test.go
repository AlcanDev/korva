package api

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestRateLimiter_Allow(t *testing.T) {
	rl := NewRateLimiter(5, time.Minute)

	// First 5 requests should be allowed.
	for i := 0; i < 5; i++ {
		if !rl.Allow("127.0.0.1") {
			t.Fatalf("request %d should be allowed, got denied", i+1)
		}
	}
	// 6th request should be denied.
	if rl.Allow("127.0.0.1") {
		t.Fatal("6th request should be denied, got allowed")
	}
}

func TestRateLimiter_IsolatedPerIP(t *testing.T) {
	rl := NewRateLimiter(2, time.Minute)

	// Exhaust IP A.
	rl.Allow("10.0.0.1")
	rl.Allow("10.0.0.1")
	if rl.Allow("10.0.0.1") {
		t.Fatal("IP A: 3rd request should be denied")
	}

	// IP B should still be allowed.
	if !rl.Allow("10.0.0.2") {
		t.Fatal("IP B: first request should be allowed")
	}
}

func TestRateLimiter_WindowReset(t *testing.T) {
	// Use a tiny window so we can force a reset without sleeping.
	rl := NewRateLimiter(1, time.Millisecond)

	if !rl.Allow("127.0.0.1") {
		t.Fatal("first request should be allowed")
	}
	if rl.Allow("127.0.0.1") {
		t.Fatal("second request should be denied (same window)")
	}

	// Wait for the window to expire.
	time.Sleep(5 * time.Millisecond)

	if !rl.Allow("127.0.0.1") {
		t.Fatal("first request in new window should be allowed")
	}
}

func TestRateLimiter_Middleware_429(t *testing.T) {
	rl := NewRateLimiter(1, time.Minute)
	inner := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	h := rl.Middleware(inner)

	// First request: passes through.
	r1 := httptest.NewRequest("GET", "/", nil)
	r1.RemoteAddr = "127.0.0.1:12345"
	w1 := httptest.NewRecorder()
	h.ServeHTTP(w1, r1)
	if w1.Code != http.StatusOK {
		t.Fatalf("want 200, got %d", w1.Code)
	}

	// Second request: rate-limited.
	r2 := httptest.NewRequest("GET", "/", nil)
	r2.RemoteAddr = "127.0.0.1:12346"
	w2 := httptest.NewRecorder()
	h.ServeHTTP(w2, r2)
	if w2.Code != http.StatusTooManyRequests {
		t.Fatalf("want 429, got %d", w2.Code)
	}
	if hdr := w2.Header().Get("Retry-After"); hdr == "" {
		t.Error("want Retry-After header, got none")
	}
}

func TestClientIP_RemoteAddr(t *testing.T) {
	r := httptest.NewRequest("GET", "/", nil)
	r.RemoteAddr = "192.168.1.10:54321"
	if got := clientIP(r); got != "192.168.1.10" {
		t.Fatalf("want 192.168.1.10, got %s", got)
	}
}

func TestClientIP_XForwardedFor(t *testing.T) {
	r := httptest.NewRequest("GET", "/", nil)
	r.RemoteAddr = "10.0.0.1:9999"
	r.Header.Set("X-Forwarded-For", "203.0.113.5, 10.0.0.1")
	if got := clientIP(r); got != "203.0.113.5" {
		t.Fatalf("want 203.0.113.5, got %s", got)
	}
}
