package cloud

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"strings"
)

// hashField produces a stable, opaque identifier for a field value
// using HMAC-SHA256 keyed on the per-installation salt.
//
// The output is the first 16 bytes (32 hex chars) of the digest.
// This is enough to deduplicate while keeping reverse lookup
// computationally infeasible without the salt.
//
// Empty inputs hash to the empty string (so they remain omitempty in JSON).
func hashField(value string, salt []byte) string {
	v := strings.TrimSpace(value)
	if v == "" {
		return ""
	}
	mac := hmac.New(sha256.New, salt)
	mac.Write([]byte(strings.ToLower(v)))
	sum := mac.Sum(nil)
	return hex.EncodeToString(sum[:16])
}
