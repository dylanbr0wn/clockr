// Package observe provides structured logging and in-process metrics for the
// OAuth broker without recording secrets or token material.
package observe

import (
	"fmt"
	"io"
	"net/http"
	"sort"
	"strings"
	"sync"
	"sync/atomic"

	applog "github.com/dylanbr0wn/shiet/internal/log"
	"github.com/rs/zerolog"
)

// Metrics holds Prometheus-style counters for broker abuse signals.
type Metrics struct {
	authStarts       atomic.Int64
	authStartFails   atomic.Int64
	callbackOutcomes sync.Map // reason -> *atomic.Int64
	handoffFails     sync.Map
	refreshFails     sync.Map
	revokeOutcomes   sync.Map
	rateLimited      sync.Map
	killSwitch       sync.Map
	quotaRisk        sync.Map
	refreshOK        atomic.Int64
	handoffOK        atomic.Int64
	revokeOK         atomic.Int64
}

// NewMetrics returns an empty metrics registry.
func NewMetrics() *Metrics {
	return &Metrics{}
}

func (m *Metrics) IncAuthStart() {
	if m == nil {
		return
	}
	m.authStarts.Add(1)
}

func (m *Metrics) IncAuthStartFail() {
	if m == nil {
		return
	}
	m.authStartFails.Add(1)
}

func (m *Metrics) IncHandoffOK() {
	if m == nil {
		return
	}
	m.handoffOK.Add(1)
}

func (m *Metrics) IncRefreshOK() {
	if m == nil {
		return
	}
	m.refreshOK.Add(1)
}

func (m *Metrics) IncRevokeOK() {
	if m == nil {
		return
	}
	m.revokeOK.Add(1)
}

func (m *Metrics) IncCallback(reason string) {
	if m == nil {
		return
	}
	incLabeled(&m.callbackOutcomes, reason)
}

func (m *Metrics) IncHandoffFailure(reason string) {
	if m == nil {
		return
	}
	incLabeled(&m.handoffFails, reason)
}

func (m *Metrics) IncRefreshFailure(reason string) {
	if m == nil {
		return
	}
	incLabeled(&m.refreshFails, reason)
}

func (m *Metrics) IncRevokeOutcome(reason string) {
	if m == nil {
		return
	}
	incLabeled(&m.revokeOutcomes, reason)
}

func (m *Metrics) IncRateLimited(surface string) {
	if m == nil {
		return
	}
	incLabeled(&m.rateLimited, surface)
}

func (m *Metrics) IncKillSwitch(surface string) {
	if m == nil {
		return
	}
	incLabeled(&m.killSwitch, surface)
}

func (m *Metrics) IncQuotaRisk(signal string) {
	if m == nil {
		return
	}
	incLabeled(&m.quotaRisk, signal)
}

func (m *Metrics) HandoffFailureCount(reason string) int64 {
	if m == nil {
		return 0
	}
	return labeledCount(&m.handoffFails, reason)
}

func (m *Metrics) RateLimitedCount(surface string) int64 {
	if m == nil {
		return 0
	}
	return labeledCount(&m.rateLimited, surface)
}

func (m *Metrics) KillSwitchCount(surface string) int64 {
	if m == nil {
		return 0
	}
	return labeledCount(&m.killSwitch, surface)
}

func incLabeled(m *sync.Map, label string) {
	label = sanitizeLabel(label)
	actual, _ := m.LoadOrStore(label, &atomic.Int64{})
	actual.(*atomic.Int64).Add(1)
}

func labeledCount(m *sync.Map, label string) int64 {
	v, ok := m.Load(sanitizeLabel(label))
	if !ok {
		return 0
	}
	return v.(*atomic.Int64).Load()
}

func sanitizeLabel(label string) string {
	label = strings.TrimSpace(label)
	if label == "" {
		return "unknown"
	}
	var b strings.Builder
	for _, r := range label {
		switch {
		case r >= 'a' && r <= 'z', r >= 'A' && r <= 'Z', r >= '0' && r <= '9', r == '_', r == '-':
			b.WriteRune(r)
		default:
			b.WriteByte('_')
		}
	}
	return b.String()
}

// WritePrometheus writes text exposition format to w.
func (m *Metrics) WritePrometheus(w io.Writer) {
	if m == nil {
		return
	}
	_, _ = fmt.Fprintf(w, "broker_auth_starts_total %d\n", m.authStarts.Load())
	_, _ = fmt.Fprintf(w, "broker_auth_start_failures_total %d\n", m.authStartFails.Load())
	_, _ = fmt.Fprintf(w, "broker_handoff_success_total %d\n", m.handoffOK.Load())
	_, _ = fmt.Fprintf(w, "broker_refresh_success_total %d\n", m.refreshOK.Load())
	_, _ = fmt.Fprintf(w, "broker_revoke_success_total %d\n", m.revokeOK.Load())
	writeLabeled(w, "broker_callback_outcomes_total", "reason", &m.callbackOutcomes)
	writeLabeled(w, "broker_handoff_failures_total", "reason", &m.handoffFails)
	writeLabeled(w, "broker_refresh_failures_total", "reason", &m.refreshFails)
	writeLabeled(w, "broker_revoke_outcomes_total", "reason", &m.revokeOutcomes)
	writeLabeled(w, "broker_rate_limited_total", "surface", &m.rateLimited)
	writeLabeled(w, "broker_kill_switch_total", "surface", &m.killSwitch)
	writeLabeled(w, "broker_quota_risk_total", "signal", &m.quotaRisk)
}

func writeLabeled(w io.Writer, name, labelKey string, m *sync.Map) {
	type pair struct {
		label string
		n     int64
	}
	var pairs []pair
	m.Range(func(key, value any) bool {
		pairs = append(pairs, pair{label: key.(string), n: value.(*atomic.Int64).Load()})
		return true
	})
	sort.Slice(pairs, func(i, j int) bool { return pairs[i].label < pairs[j].label })
	for _, p := range pairs {
		_, _ = fmt.Fprintf(w, "%s{%s=%q} %d\n", name, labelKey, p.label, p.n)
	}
}

// Handler serves GET /metrics.
func (m *Metrics) Handler() http.HandlerFunc {
	return func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/plain; version=0.0.4; charset=utf-8")
		m.WritePrometheus(w)
	}
}

// NewLogger returns a JSON zerolog logger with ADR-0001 secret redaction
// (shared internal/log package).
func NewLogger(w io.Writer) zerolog.Logger {
	return applog.New(w)
}
