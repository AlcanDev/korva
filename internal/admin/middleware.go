package admin

import (
	"crypto/subtle"
	"net/http"
)

const headerName = "X-Admin-Key"

// Middleware returns an HTTP middleware that validates the X-Admin-Key header.
// If admin.key does not exist on this machine, all admin requests are rejected
// with 403 Forbidden — non-admin machines simply cannot use admin endpoints.
func Middleware(keyPath string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			provided := r.Header.Get(headerName)
			if provided == "" {
				http.Error(w, "missing "+headerName+" header", http.StatusUnauthorized)
				return
			}

			cfg, err := Load(keyPath)
			if err != nil {
				// Both "key not found" and "key unreadable/corrupt" are treated as
				// Forbidden — from the caller's perspective this machine has no valid key.
				http.Error(w, "admin operations are not available on this machine", http.StatusForbidden)
				return
			}

			if !secureEqual(cfg.Key, provided) {
				http.Error(w, "invalid admin key", http.StatusForbidden)
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}

// secureEqual compares two strings in constant time to prevent timing attacks.
func secureEqual(a, b string) bool {
	return subtle.ConstantTimeCompare([]byte(a), []byte(b)) == 1
}
