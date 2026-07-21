package log

import (
	"context"
	"errors"
	"net"
	"strings"
)

// Fixed reason codes for failure logs (DYL-180). Prefer these over err.Error()
// so tokens/bodies/query strings never reach the writer.
const (
	ReasonUnauthorized  = "unauthorized"
	ReasonForbidden     = "forbidden"
	ReasonNotFound      = "not_found"
	ReasonRateLimited   = "rate_limited"
	ReasonNetwork       = "network"
	ReasonInvalidConfig = "invalid_config"
	ReasonUnknown       = "unknown"
)

// Reason maps an error to a fixed reason code for structured logs.
// Classification may inspect err.Error(); callers must never log that string.
func Reason(err error) string {
	if err == nil {
		return ReasonUnknown
	}

	var netErr net.Error
	if errors.As(err, &netErr) {
		return ReasonNetwork
	}
	var dnsErr *net.DNSError
	if errors.As(err, &dnsErr) {
		return ReasonNetwork
	}
	var opErr *net.OpError
	if errors.As(err, &opErr) {
		return ReasonNetwork
	}
	if errors.Is(err, context.DeadlineExceeded) {
		return ReasonNetwork
	}
	if errors.Is(err, context.Canceled) {
		return ReasonUnknown
	}

	msg := strings.ToLower(err.Error())

	switch {
	case containsAny(msg, "rate_limited", "rate limit", "too many requests", " 429 ", "429 ", ": 429"):
		return ReasonRateLimited
	case containsAny(msg,
		"unauthorized",
		"invalid_grant",
		"invalid personal access token",
		"invalid github oauth token",
		"invalid slack oauth token",
		"invalid bitbucket oauth token",
		"bad credentials",
		"needs re-authentication",
		"needs reauth",
		" 401 ",
		"401 ",
		": 401",
	):
		return ReasonUnauthorized
	case containsAny(msg, "forbidden", " 403 ", "403 ", ": 403"):
		return ReasonForbidden
	case containsAny(msg, "not configured", "is required", "invalid_config", "missing required"):
		return ReasonInvalidConfig
	case containsAny(msg, "not found", " 404 ", "404 ", ": 404"):
		return ReasonNotFound
	case containsAny(msg,
		"connection refused",
		"connection reset",
		"no such host",
		"i/o timeout",
		"tls handshake",
		"dial tcp",
		"network is unreachable",
		"temporary failure in name resolution",
	):
		return ReasonNetwork
	default:
		return ReasonUnknown
	}
}

func containsAny(msg string, needles ...string) bool {
	for _, n := range needles {
		if strings.Contains(msg, n) {
			return true
		}
	}
	return false
}
