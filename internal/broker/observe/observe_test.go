package observe

import (
	"bytes"
	"log/slog"
	"strings"
	"testing"
)

func TestRedactAttrsScrubsTokensAndSecrets(t *testing.T) {
	attrs := []slog.Attr{
		slog.String("event", "refresh_failed"),
		slog.String("refresh_token", "1//secret-refresh-token-value"),
		slog.String("client_secret", "super-secret-value"),
		slog.String("handoff_code", "abc123handoff"),
		slog.String("access_token", "ya29.a0AfH6SMB-example-token"),
		slog.String("reason", "invalid_grant"),
	}
	out := RedactAttrs(attrs)
	byKey := map[string]string{}
	for _, a := range out {
		byKey[a.Key] = a.Value.String()
	}
	if byKey["event"] != "refresh_failed" {
		t.Fatalf("event: %q", byKey["event"])
	}
	if byKey["reason"] != "invalid_grant" {
		t.Fatalf("reason: %q", byKey["reason"])
	}
	for _, key := range []string{"refresh_token", "client_secret", "handoff_code", "access_token"} {
		if byKey[key] != "[redacted]" {
			t.Fatalf("%s: got %q want [redacted]", key, byKey[key])
		}
	}
}

func TestLoggerRedactsInJSONOutput(t *testing.T) {
	var buf bytes.Buffer
	log := NewLogger(&buf)
	log.Info("broker_event",
		"surface", "refresh",
		"refresh_token", "1//should-not-appear-in-logs",
		"client_secret", "google-client-secret",
		"outcome", "failure",
	)
	got := buf.String()
	if strings.Contains(got, "1//should-not-appear") {
		t.Fatalf("refresh token leaked: %s", got)
	}
	if strings.Contains(got, "google-client-secret") {
		t.Fatalf("client secret leaked: %s", got)
	}
	if !strings.Contains(got, "[redacted]") {
		t.Fatalf("expected redaction marker: %s", got)
	}
	if !strings.Contains(got, `"outcome":"failure"`) {
		t.Fatalf("expected safe field preserved: %s", got)
	}
}

func TestMetricsCountersAndPrometheus(t *testing.T) {
	m := NewMetrics()
	m.IncAuthStart()
	m.IncHandoffFailure("already_used")
	m.IncHandoffFailure("already_used")
	m.IncRateLimited("start")
	m.IncKillSwitch("refresh")
	m.IncQuotaRisk("handoff_replay")

	if m.HandoffFailureCount("already_used") != 2 {
		t.Fatalf("handoff fail count: %d", m.HandoffFailureCount("already_used"))
	}
	if m.RateLimitedCount("start") != 1 {
		t.Fatalf("rate limited: %d", m.RateLimitedCount("start"))
	}
	if m.KillSwitchCount("refresh") != 1 {
		t.Fatalf("kill switch: %d", m.KillSwitchCount("refresh"))
	}

	var buf bytes.Buffer
	m.WritePrometheus(&buf)
	text := buf.String()
	for _, want := range []string{
		"broker_auth_starts_total 1",
		`broker_handoff_failures_total{reason="already_used"} 2`,
		`broker_rate_limited_total{surface="start"} 1`,
		`broker_kill_switch_total{surface="refresh"} 1`,
		`broker_quota_risk_total{signal="handoff_replay"} 1`,
	} {
		if !strings.Contains(text, want) {
			t.Fatalf("metrics missing %q in:\n%s", want, text)
		}
	}
}
