package httpapi

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"connectrpc.com/connect"
	brokerv1 "github.com/dylanbr0wn/shiet/gen/shiet/broker/v1"
	brokerconfig "github.com/dylanbr0wn/shiet/internal/broker/config"
	"github.com/dylanbr0wn/shiet/internal/broker/observe"
	"github.com/dylanbr0wn/shiet/internal/broker/store"
)

func TestHealthAndReady(t *testing.T) {
	srv := Server{Config: testConfig(), Store: &memoryStore{}}

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

func TestRemovedRESTOperationsReturnNotFound(t *testing.T) {
	t.Parallel()

	srv := Server{Config: testConfig(), Store: &memoryStore{}}
	for _, path := range []string{
		"/v1/google/oauth/start",
		"/v1/google/oauth/handoff",
		"/v1/google/oauth/refresh",
		"/v1/google/oauth/revoke",
		"/v1/github/oauth/start",
		"/v1/github/oauth/handoff",
		"/v1/github/oauth/revoke",
	} {
		t.Run(path, func(t *testing.T) {
			recorder := httptest.NewRecorder()
			srv.Handler().ServeHTTP(recorder, httptest.NewRequest(http.MethodPost, path, strings.NewReader(`{}`)))
			if recorder.Code != http.StatusNotFound {
				t.Fatalf("%s status = %d, want 404", path, recorder.Code)
			}
		})
	}
}

func TestGoogleCallbackExchangesCodeAndCreatesHandoff(t *testing.T) {
	now := time.Date(2026, 7, 8, 12, 0, 0, 0, time.UTC)
	mem := &memoryStore{}
	tokenSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Fatalf("token method: %s", r.Method)
		}
		if err := r.ParseForm(); err != nil {
			t.Fatal(err)
		}
		if got := r.Form.Get("code"); got != "google-auth-code" {
			t.Fatalf("code: %q", got)
		}
		if got := r.Form.Get("client_secret"); got != "google-client-secret" {
			t.Fatalf("client_secret: %q", got)
		}
		if got := r.Form.Get("code_verifier"); got != "pkce-verifier" {
			t.Fatalf("code_verifier: %q", got)
		}
		if got := r.Form.Get("redirect_uri"); got != "https://auth.shiet.app/v1/google/oauth/callback" {
			t.Fatalf("redirect_uri: %q", got)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{
			"access_token":"access-1",
			"refresh_token":"refresh-1",
			"token_type":"Bearer",
			"expires_in":3600,
			"scope":"https://www.googleapis.com/auth/calendar.readonly"
		}`))
	}))
	t.Cleanup(tokenSrv.Close)

	srv := Server{Config: testConfig(), Store: mem, Clock: func() time.Time { return now }, HTTPClient: tokenSrv.Client(), GoogleTokenURL: tokenSrv.URL}
	if err := mem.SaveOAuthState(context.Background(), store.OAuthState{
		ID:                     "broker-state-1",
		DesktopSessionID:       "desktop-1",
		PKCEVerifier:           "pkce-verifier",
		PKCEChallenge:          "pkce-challenge",
		HandoffChallenge:       "challenge-1",
		DesktopHandoffRedirect: "http://127.0.0.1:9/oauth/handoff",
		Scopes:                 []string{"https://www.googleapis.com/auth/calendar.readonly"},
		ExpiresAt:              now.Add(5 * time.Minute),
	}); err != nil {
		t.Fatal(err)
	}

	recorder := httptest.NewRecorder()
	srv.Handler().ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/v1/google/oauth/callback?code=google-auth-code&state=broker-state-1", nil))
	if recorder.Code != http.StatusOK {
		t.Fatalf("status: got %d body %s", recorder.Code, recorder.Body.String())
	}
	body := recorder.Body.String()
	if !strings.Contains(body, "http://127.0.0.1:9/oauth/handoff?") || !strings.Contains(body, "broker_state=broker-state-1") || !strings.Contains(body, "handoff_code=") {
		t.Fatalf("expected bound loopback handoff link: %s", body)
	}
	if strings.Contains(body, "access-1") || strings.Contains(body, "refresh-1") {
		t.Fatal("callback page must not include Google token material")
	}
	if len(mem.handoffs) != 1 || mem.handoffs[0].CodeHash == "" || len(mem.handoffs[0].EncryptedTokenPayload) == 0 {
		t.Fatalf("handoff not persisted safely: %+v", mem.handoffs)
	}
	if !mem.handoffs[0].ExpiresAt.Equal(now.Add(2*time.Minute)) || mem.states[0].UsedAt == nil {
		t.Fatalf("callback side effects: handoff=%+v state=%+v", mem.handoffs[0], mem.states[0])
	}

	replay := httptest.NewRecorder()
	srv.Handler().ServeHTTP(replay, httptest.NewRequest(http.MethodGet, "/v1/google/oauth/callback?code=google-auth-code&state=broker-state-1", nil))
	if replay.Code != http.StatusBadRequest || len(mem.handoffs) != 1 {
		t.Fatalf("callback replay = %d, handoffs=%d", replay.Code, len(mem.handoffs))
	}
}

func TestGitHubCallbackHandsTokenOffThroughConnect(t *testing.T) {
	now := time.Date(2026, 7, 9, 12, 0, 0, 0, time.UTC)
	mem := &memoryStore{}
	tokenSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if err := r.ParseForm(); err != nil {
			t.Fatal(err)
		}
		for key, want := range map[string]string{
			"code": "github-auth-code", "client_id": "github-client-id", "client_secret": "github-client-secret",
			"code_verifier": "pkce-verifier", "redirect_uri": "https://auth.shiet.app/v1/github/oauth/callback",
		} {
			if got := r.Form.Get(key); got != want {
				t.Fatalf("%s: got %q want %q", key, got, want)
			}
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"access_token":"gho_access","token_type":"bearer","scope":"repo,read:user"}`))
	}))
	t.Cleanup(tokenSrv.Close)

	const verifier = "desktop-verifier"
	srv := Server{Config: testConfig(), Store: mem, Clock: func() time.Time { return now }, HTTPClient: tokenSrv.Client(), GitHubTokenURL: tokenSrv.URL}
	if err := mem.SaveOAuthState(context.Background(), store.OAuthState{
		ID: "github-state-1", Provider: "github", DesktopSessionID: "desktop-1", PKCEVerifier: "pkce-verifier",
		PKCEChallenge: "pkce-challenge", HandoffChallenge: pkceS256(verifier), DesktopHandoffRedirect: "http://127.0.0.1:9/oauth/handoff",
		Scopes: []string{"repo", "read:user"}, ExpiresAt: now.Add(5 * time.Minute),
	}); err != nil {
		t.Fatal(err)
	}

	callback := httptest.NewRecorder()
	srv.Handler().ServeHTTP(callback, httptest.NewRequest(http.MethodGet, "/v1/github/oauth/callback?code=github-auth-code&state=github-state-1", nil))
	if callback.Code != http.StatusOK || strings.Contains(callback.Body.String(), "gho_access") || !strings.Contains(callback.Body.String(), "finish connecting GitHub") {
		t.Fatalf("callback = %d %s", callback.Code, callback.Body.String())
	}
	code := handoffCodeFromCallbackPage(t, callback.Body.String())
	response, err := connectTestClient(srv).ExchangeHandoff(context.Background(), connect.NewRequest(&brokerv1.ExchangeHandoffRequest{
		Provider: brokerv1.Provider_PROVIDER_GITHUB, DesktopSessionId: "desktop-1", BrokerState: "github-state-1", HandoffCode: code, HandoffVerifier: verifier,
	}))
	if err != nil {
		t.Fatal(err)
	}
	if response.Msg.Provider != brokerv1.Provider_PROVIDER_GITHUB || response.Msg.Token.GetAccessToken() != "gho_access" || response.Msg.Token.GetRefreshToken() != "" {
		t.Fatalf("handoff response: %+v", response.Msg)
	}
}

func handoffCodeFromCallbackPage(t *testing.T, body string) string {
	t.Helper()
	start := strings.Index(body, "handoff_code=")
	if start < 0 {
		t.Fatalf("handoff code missing from callback page: %s", body)
	}
	start += len("handoff_code=")
	end := strings.IndexAny(body[start:], "&\"<")
	if end < 0 {
		return body[start:]
	}
	return body[start : start+end]
}

func TestGoogleCallbackRejectsExpiredOrMissingState(t *testing.T) {
	now := time.Date(2026, 7, 8, 12, 0, 0, 0, time.UTC)
	mem := &memoryStore{}
	srv := Server{Config: testConfig(), Store: mem, Clock: func() time.Time { return now }}

	missing := httptest.NewRecorder()
	srv.Handler().ServeHTTP(missing, httptest.NewRequest(http.MethodGet, "/v1/google/oauth/callback?code=x&state=missing", nil))
	if missing.Code != http.StatusBadRequest {
		t.Fatalf("missing state status: got %d", missing.Code)
	}

	_ = mem.SaveOAuthState(context.Background(), store.OAuthState{ID: "expired", DesktopSessionID: "desktop-1", PKCEVerifier: "v", PKCEChallenge: "c", HandoffChallenge: "h", Scopes: []string{"scope"}, ExpiresAt: now.Add(-time.Second)})
	expired := httptest.NewRecorder()
	srv.Handler().ServeHTTP(expired, httptest.NewRequest(http.MethodGet, "/v1/google/oauth/callback?code=x&state=expired", nil))
	if expired.Code != http.StatusBadRequest {
		t.Fatalf("expired state status: got %d", expired.Code)
	}
}

func TestMetricsEndpoint(t *testing.T) {
	metrics := observe.NewMetrics()
	metrics.IncAuthStart()
	srv := Server{Config: testConfig(), Store: &memoryStore{}, Metrics: metrics}
	recorder := httptest.NewRecorder()
	srv.Handler().ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/metrics", nil))
	if recorder.Code != http.StatusOK || !strings.Contains(recorder.Body.String(), "broker_auth_starts_total 1") {
		t.Fatalf("metrics = %d %s", recorder.Code, recorder.Body.String())
	}
}

func testConfig() brokerconfig.Config {
	return brokerconfig.Config{
		ListenAddr: ":8080", PublicOrigin: "https://auth.shiet.app", GoogleClientID: "google-client-id", GoogleClientSecret: "google-client-secret",
		DesktopHandoffURL: "shiet://oauth/google/handoff", GitHubClientID: "github-client-id", GitHubClientSecret: "github-client-secret",
		GitHubDesktopHandoffURL: "shiet://oauth/github/handoff", DatastoreDSN: "file:broker.db", StateTTL: 5 * time.Minute, HandoffTTL: 2 * time.Minute,
		GoogleScopes: []string{"https://www.googleapis.com/auth/calendar.readonly"}, GitHubScopes: []string{"repo", "read:user"},
	}
}

type memoryStore struct {
	states   []store.OAuthState
	handoffs []store.HandoffRecord
}

func (m *memoryStore) Ping(context.Context) error { return nil }

func (m *memoryStore) SaveOAuthState(_ context.Context, record store.OAuthState) error {
	m.states = append(m.states, record)
	return nil
}

func (m *memoryStore) ConsumeOAuthState(_ context.Context, id, provider string, now time.Time) (store.OAuthState, error) {
	for index := range m.states {
		record := &m.states[index]
		if record.ID != id {
			continue
		}
		if record.UsedAt != nil {
			return store.OAuthState{}, store.ErrAlreadyUsed
		}
		if !now.Before(record.ExpiresAt) {
			return store.OAuthState{}, store.ErrExpired
		}
		if providerOrGoogle(record.Provider) != providerOrGoogle(provider) {
			return store.OAuthState{}, store.ErrMismatch
		}
		used := now
		record.UsedAt = &used
		return *record, nil
	}
	return store.OAuthState{}, store.ErrNotFound
}

func (m *memoryStore) SaveHandoff(_ context.Context, record store.HandoffRecord) error {
	m.handoffs = append(m.handoffs, record)
	return nil
}

func (m *memoryStore) ConsumeHandoff(_ context.Context, codeHash, provider, desktopSessionID, stateID, handoffChallenge string, now time.Time) (store.HandoffRecord, error) {
	for index := range m.handoffs {
		record := &m.handoffs[index]
		if record.CodeHash != codeHash {
			continue
		}
		if record.UsedAt != nil {
			return store.HandoffRecord{}, store.ErrAlreadyUsed
		}
		if !now.Before(record.ExpiresAt) {
			return store.HandoffRecord{}, store.ErrExpired
		}
		if providerOrGoogle(record.Provider) != providerOrGoogle(provider) || record.DesktopSessionID != desktopSessionID || record.StateID != stateID || record.HandoffChallenge != handoffChallenge {
			return store.HandoffRecord{}, store.ErrMismatch
		}
		output := *record
		used := now
		record.UsedAt = &used
		record.EncryptedTokenPayload = nil
		return output, nil
	}
	return store.HandoffRecord{}, store.ErrNotFound
}
