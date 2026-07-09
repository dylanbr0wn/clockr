package httpapi

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"
	"time"

	brokerconfig "github.com/dylanbr0wn/shiet/internal/broker/config"
	"github.com/dylanbr0wn/shiet/internal/broker/store"
)

func TestHealthAndReady(t *testing.T) {
	srv := Server{
		Config: testConfig(),
		Store:  &memoryStore{},
	}

	health := httptest.NewRecorder()
	srv.Handler().ServeHTTP(health, httptest.NewRequest(http.MethodGet, "/healthz", nil))
	if health.Code != http.StatusOK {
		t.Fatalf("health status: got %d", health.Code)
	}

	ready := httptest.NewRecorder()
	srv.Handler().ServeHTTP(ready, httptest.NewRequest(http.MethodGet, "/readyz", nil))
	if ready.Code != http.StatusOK {
		t.Fatalf("ready status: got %d body %s", ready.Code, ready.Body.String())
	}
}

func TestStartGoogleOAuthPersistsStateAndReturnsAuthURL(t *testing.T) {
	now := time.Date(2026, 7, 8, 12, 0, 0, 0, time.UTC)
	mem := &memoryStore{}
	srv := Server{
		Config: testConfig(),
		Store:  mem,
		Clock:  func() time.Time { return now },
	}
	body := bytes.NewBufferString(`{
		"desktop_session_id": "desktop-1",
		"handoff_challenge": "challenge-1",
		"app_version": "0.1.0",
		"platform": "darwin-arm64"
	}`)

	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/v1/google/oauth/start", body)
	req.RemoteAddr = "203.0.113.42:1234"
	srv.Handler().ServeHTTP(rr, req)

	if rr.Code != http.StatusCreated {
		t.Fatalf("status: got %d body %s", rr.Code, rr.Body.String())
	}
	var resp startResponse
	if err := json.NewDecoder(rr.Body).Decode(&resp); err != nil {
		t.Fatal(err)
	}
	if resp.BrokerState == "" {
		t.Fatal("expected broker state")
	}
	if !resp.ExpiresAt.Equal(now.Add(5 * time.Minute)) {
		t.Fatalf("expires_at: got %s", resp.ExpiresAt)
	}
	if len(mem.states) != 1 {
		t.Fatalf("stored states: got %d", len(mem.states))
	}
	state := mem.states[0]
	if state.ID != resp.BrokerState {
		t.Fatalf("state id: got %q want %q", state.ID, resp.BrokerState)
	}
	if state.PKCEVerifier == "" || state.PKCEChallenge == "" {
		t.Fatal("expected PKCE verifier and challenge")
	}
	if state.SourceIPBucket != "203.0.113.0/24" {
		t.Fatalf("source ip bucket: got %q", state.SourceIPBucket)
	}

	authURL, err := url.Parse(resp.AuthURL)
	if err != nil {
		t.Fatalf("parse auth_url: %v", err)
	}
	q := authURL.Query()
	if got := q.Get("client_id"); got != "google-client-id" {
		t.Fatalf("client_id: got %q", got)
	}
	if got := q.Get("redirect_uri"); got != "https://auth.shiet.app/v1/google/oauth/callback" {
		t.Fatalf("redirect_uri: got %q", got)
	}
	if got := q.Get("state"); got != resp.BrokerState {
		t.Fatalf("state: got %q want %q", got, resp.BrokerState)
	}
	if got := q.Get("code_challenge_method"); got != "S256" {
		t.Fatalf("code_challenge_method: got %q", got)
	}
}

func TestStartGoogleOAuthRejectsMissingBindingInputs(t *testing.T) {
	srv := Server{Config: testConfig(), Store: &memoryStore{}}

	rr := httptest.NewRecorder()
	srv.Handler().ServeHTTP(rr, httptest.NewRequest(http.MethodPost, "/v1/google/oauth/start", bytes.NewBufferString(`{}`)))
	if rr.Code != http.StatusBadRequest {
		t.Fatalf("status: got %d body %s", rr.Code, rr.Body.String())
	}
}

func testConfig() brokerconfig.Config {
	return brokerconfig.Config{
		ListenAddr:         ":8080",
		PublicOrigin:       "https://auth.shiet.app",
		GoogleClientID:     "google-client-id",
		GoogleClientSecret: "google-client-secret",
		DesktopHandoffURL:  "shiet://oauth/google/handoff",
		DatastoreDSN:       "file:broker.db",
		StateTTL:           5 * time.Minute,
		HandoffTTL:         2 * time.Minute,
		GoogleScopes:       []string{"https://www.googleapis.com/auth/calendar.readonly"},
	}
}

type memoryStore struct {
	states []store.OAuthState
}

func (m *memoryStore) Ping(context.Context) error {
	return nil
}

func (m *memoryStore) SaveOAuthState(_ context.Context, rec store.OAuthState) error {
	m.states = append(m.states, rec)
	return nil
}
