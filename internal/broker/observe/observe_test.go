package observe

import (
	"bytes"
	"fmt"
	"strings"
	"testing"

	"github.com/dylanbr0wn/shiet/internal/broker/codes"
)

func TestLoggerRedactsInJSONOutput(t *testing.T) {
	var buf bytes.Buffer
	log := NewLogger(&buf)
	log.Info().
		Str("surface", "refresh").
		Str("refresh_token", "1//should-not-appear-in-logs").
		Str("client_secret", "google-client-secret").
		Str("outcome", "failure").
		Msg("broker_event")
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
	m.IncHandoffFailure(codes.OutcomeAlreadyUsed)
	m.IncHandoffFailure(codes.OutcomeAlreadyUsed)
	m.IncRateLimited(codes.SurfaceStart)
	m.IncKillSwitch(codes.SurfaceRefresh)
	m.IncQuotaRisk(codes.QuotaHandoffReplay)

	if m.HandoffFailureCount(codes.OutcomeAlreadyUsed) != 2 {
		t.Fatalf("handoff fail count: %d", m.HandoffFailureCount(codes.OutcomeAlreadyUsed))
	}
	if m.RateLimitedCount(codes.SurfaceStart) != 1 {
		t.Fatalf("rate limited: %d", m.RateLimitedCount(codes.SurfaceStart))
	}
	if m.KillSwitchCount(codes.SurfaceRefresh) != 1 {
		t.Fatalf("kill switch: %d", m.KillSwitchCount(codes.SurfaceRefresh))
	}

	var buf bytes.Buffer
	m.WritePrometheus(&buf)
	text := buf.String()
	for _, want := range []string{
		"broker_auth_starts_total 1",
		fmt.Sprintf("broker_handoff_failures_total{reason=%q} 2", codes.OutcomeAlreadyUsed),
		fmt.Sprintf("broker_rate_limited_total{surface=%q} 1", codes.SurfaceStart),
		fmt.Sprintf("broker_kill_switch_total{surface=%q} 1", codes.SurfaceRefresh),
		fmt.Sprintf("broker_quota_risk_total{signal=%q} 1", codes.QuotaHandoffReplay),
	} {
		if !strings.Contains(text, want) {
			t.Fatalf("metrics missing %q in:\n%s", want, text)
		}
	}
}
