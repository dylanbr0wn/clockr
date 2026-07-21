package log

import (
	"context"
	"errors"
	"fmt"
	"net"
	"testing"
)

func TestReason(t *testing.T) {
	timeoutErr := context.DeadlineExceeded
	canceledErr := context.Canceled
	dnsErr := &net.DNSError{Err: "no such host", Name: "example.invalid", IsNotFound: true}

	cases := []struct {
		name string
		err  error
		want string
	}{
		{"nil", nil, ReasonUnknown},
		{"unauthorized literal", errors.New("unauthorized after token refresh"), ReasonUnauthorized},
		{"401 status", fmt.Errorf("list models: 401 Unauthorized: %s", `{"error":"bad_api_key_ghp_xxx"}`), ReasonUnauthorized},
		{"invalid_grant", errors.New("token refresh: invalid_grant"), ReasonUnauthorized},
		{"invalid oauth token + body", fmt.Errorf("invalid GitHub OAuth token (github api /user: %s)", `{"message":"Bad credentials"}`), ReasonUnauthorized},
		{"needs reauth", errors.New("calendar account needs re-authentication"), ReasonUnauthorized},
		{"forbidden", errors.New("google api /calendars: 403 Forbidden"), ReasonForbidden},
		{"not found", errors.New("resource not found"), ReasonNotFound},
		{"404 status", errors.New("list models: 404 Not Found: missing"), ReasonNotFound},
		{"rate limited", errors.New("slack api: rate_limited"), ReasonRateLimited},
		{"429 status", errors.New("validate model: 429 Too Many Requests"), ReasonRateLimited},
		{"not configured", errors.New("Google OAuth broker is not configured"), ReasonInvalidConfig},
		{"token store required", errors.New("token store is required"), ReasonInvalidConfig},
		{"deadline", timeoutErr, ReasonNetwork},
		{"canceled", canceledErr, ReasonUnknown},
		{"dns", dnsErr, ReasonNetwork},
		{"wrapped network", fmt.Errorf("github user: %w", dnsErr), ReasonNetwork},
		{"unknown", errors.New("something went sideways"), ReasonUnknown},
		{"token-shaped unknown still codes", fmt.Errorf("boom: ghp_abcdefghijklmnopqrstuvwxyz012345"), ReasonUnknown},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := Reason(tc.err)
			if got != tc.want {
				t.Fatalf("Reason(%v)=%q want %q", tc.err, got, tc.want)
			}
		})
	}
}

func TestReason_doesNotPreferSubstringFalsePositives(t *testing.T) {
	// "not configured" must not become not_found via "not".
	if got := Reason(errors.New("Slack OAuth is not configured")); got != ReasonInvalidConfig {
		t.Fatalf("got %q want %s", got, ReasonInvalidConfig)
	}
}
