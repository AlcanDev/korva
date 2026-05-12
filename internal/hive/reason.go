package hive

import (
	"errors"
	"net"
	"net/url"
	"strings"
)

// ClassifyError maps a transport/server error to one of the canonical reason
// codes so downstream consumers (the Beacon dashboard, `korva status --hive`,
// the MCP status surface) can render a stable, localizable label instead of
// parsing free-form strings.
//
// Order matters: more specific signals (HTTP status codes embedded in the
// error string) are checked first, then transport-shaped errors as a
// fallback. An empty input returns ReasonNone.
func ClassifyError(err error) ReasonCode {
	if err == nil {
		return ReasonNone
	}
	msg := err.Error()
	lower := strings.ToLower(msg)

	// 1. Privacy/policy rejections are checked first so the word "forbidden"
	// in a privacy-filter message doesn't get routed to "auth_required".
	switch {
	case strings.Contains(lower, "rejected_privacy") || strings.Contains(lower, "policy"):
		return ReasonPolicyForbidden
	}

	// 2. Explicit HTTP status codes baked into the error string.
	switch {
	case strings.Contains(msg, "401") || strings.Contains(msg, "403"):
		return ReasonAuthRequired
	case strings.Contains(lower, "unauthorized") || strings.Contains(lower, "forbidden"):
		return ReasonAuthRequired
	case strings.Contains(msg, "410") || strings.Contains(msg, "426") || strings.Contains(msg, "501"):
		return ReasonServerUnsupported
	case strings.Contains(lower, "schema mismatch") || strings.Contains(lower, "unsupported"):
		return ReasonServerUnsupported
	case strings.Contains(msg, "500"), strings.Contains(msg, "502"),
		strings.Contains(msg, "503"), strings.Contains(msg, "504"),
		strings.Contains(lower, "internal server error"):
		return ReasonInternalError
	}

	// 2. Transport-shaped errors (DNS, connection refused, timeout). These
	// are what `http.Client` wraps into url.Error / net.OpError.
	var urlErr *url.Error
	var netErr net.Error
	if errors.As(err, &urlErr) {
		return ReasonTransportFailed
	}
	if errors.As(err, &netErr) {
		return ReasonTransportFailed
	}
	if strings.Contains(lower, "connection refused") ||
		strings.Contains(lower, "no such host") ||
		strings.Contains(lower, "timeout") ||
		strings.Contains(lower, "tcp") {
		return ReasonTransportFailed
	}

	// 3. Unknown: surface as transport so we keep retrying with backoff.
	return ReasonTransportFailed
}
