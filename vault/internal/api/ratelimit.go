package api

import (
	"context"
	"net"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"
)

// ipState holds the fixed-window request count for one IP address.
type ipState struct {
	mu      sync.Mutex
	count   int
	resetAt time.Time
}

// RateLimiter implements a per-IP fixed-window rate limiter using only stdlib.
// Suitable for the vault HTTP server (local or LAN deployment).
type RateLimiter struct {
	entries sync.Map
	limit   int
	window  time.Duration
}

// NewRateLimiter creates a limiter allowing at most limit requests per window per IP.
func NewRateLimiter(limit int, window time.Duration) *RateLimiter {
	return &RateLimiter{limit: limit, window: window}
}

// check increments the counter for ip and returns (allowed, remaining, resetAt).
// remaining may be negative when the limit is exceeded.
func (rl *RateLimiter) check(ip string) (allowed bool, remaining int, resetAt time.Time) {
	raw, _ := rl.entries.LoadOrStore(ip, &ipState{})
	s := raw.(*ipState) //nolint:forcetypeassert

	s.mu.Lock()
	defer s.mu.Unlock()

	now := time.Now()
	if now.After(s.resetAt) {
		s.count = 0
		s.resetAt = now.Add(rl.window)
	}
	s.count++
	remaining = rl.limit - s.count
	return s.count <= rl.limit, remaining, s.resetAt
}

// Allow returns true if the IP is within its quota for the current window.
func (rl *RateLimiter) Allow(ip string) bool {
	ok, _, _ := rl.check(ip)
	return ok
}

// StartCleanup launches a background goroutine that removes stale per-IP entries
// once per interval. Without this, the sync.Map would grow unboundedly in
// deployments exposed to many distinct IPs (e.g. reverse-proxy behind Cloudflare).
// The goroutine stops when ctx is canceled.
func (rl *RateLimiter) StartCleanup(ctx context.Context, interval time.Duration) {
	go func() {
		t := time.NewTicker(interval)
		defer t.Stop()
		for {
			select {
			case <-t.C:
				now := time.Now()
				rl.entries.Range(func(k, v any) bool {
					s := v.(*ipState) //nolint:forcetypeassert
					s.mu.Lock()
					expired := now.After(s.resetAt)
					s.mu.Unlock()
					if expired {
						rl.entries.Delete(k)
					}
					return true
				})
			case <-ctx.Done():
				return
			}
		}
	}()
}

// Middleware wraps h with IP-based rate limiting and injects X-RateLimit-*
// headers on every response so API clients can back off proactively.
//
// Headers set on every response:
//
//	X-RateLimit-Limit:     maximum requests per window
//	X-RateLimit-Remaining: requests remaining in the current window (min 0)
//	X-RateLimit-Reset:     Unix timestamp when the window resets
//
// When the limit is exceeded, HTTP 429 is returned with Retry-After set.
func (rl *RateLimiter) Middleware(h http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ip := clientIP(r)
		allowed, remaining, resetAt := rl.check(ip)

		if remaining < 0 {
			remaining = 0
		}
		w.Header().Set("X-RateLimit-Limit", strconv.Itoa(rl.limit))
		w.Header().Set("X-RateLimit-Remaining", strconv.Itoa(remaining))
		w.Header().Set("X-RateLimit-Reset", strconv.FormatInt(resetAt.Unix(), 10))

		if !allowed {
			w.Header().Set("Content-Type", "application/json")
			w.Header().Set("Retry-After", "60")
			w.WriteHeader(http.StatusTooManyRequests)
			_, _ = w.Write([]byte(`{"error":"rate limit exceeded — slow down"}`))
			return
		}
		h.ServeHTTP(w, r)
	})
}

// clientIP extracts the real client IP from a request.
// X-Forwarded-For is trusted for the first hop (reverse-proxy deployments).
func clientIP(r *http.Request) string {
	if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
		// Trim to just the first address in the comma-list.
		if i := strings.IndexByte(xff, ','); i != -1 {
			xff = xff[:i]
		}
		if ip := net.ParseIP(strings.TrimSpace(xff)); ip != nil {
			return ip.String()
		}
	}
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return host
}
