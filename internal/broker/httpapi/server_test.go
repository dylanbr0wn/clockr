package httpapi

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/dylanbr0wn/shiet/internal/broker/codes"
	brokerconfig "github.com/dylanbr0wn/shiet/internal/broker/config"
	"github.com/dylanbr0wn/shiet/internal/broker/observe"
	"github.com/dylanbr0wn/shiet/internal/broker/ratelimit"
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

func TestStartGitHubOAuthPersistsProviderAndReturnsAuthURL(t *testing.T) {
	now := time.Date(2026, 7, 9, 12, 0, 0, 0, time.UTC)
	mem := &memoryStore{}
	srv := Server{Config: testConfig(), Store: mem, Clock: func() time.Time { return now }}
	body := bytes.NewBufferString(`{
		"desktop_session_id":"desktop-1",
		"handoff_challenge":"challenge-1",
		"app_version":"0.2.0",
		"platform":"darwin-arm64"
	}`)

	rr := httptest.NewRecorder()
	srv.Handler().ServeHTTP(rr, httptest.NewRequest(http.MethodPost, "/v1/github/oauth/start", body))
	if rr.Code != http.StatusCreated {
		t.Fatalf("status: got %d body %s", rr.Code, rr.Body.String())
	}
	var resp startResponse
	if err := json.NewDecoder(rr.Body).Decode(&resp); err != nil {
		t.Fatal(err)
	}
	if len(mem.states) != 1 || mem.states[0].Provider != "github" {
		t.Fatalf("github state not persisted: %+v", mem.states)
	}
	if got := strings.Join(mem.states[0].Scopes, ","); got != "repo,read:user" {
		t.Fatalf("scopes: got %q", got)
	}
	authURL, err := url.Parse(resp.AuthURL)
	if err != nil {
		t.Fatal(err)
	}
	if authURL.Scheme != "https" || authURL.Host != "github.com" || authURL.Path != "/login/oauth/authorize" {
		t.Fatalf("auth_url: %s", authURL)
	}
	q := authURL.Query()
	if q.Get("client_id") != "github-client-id" {
		t.Fatalf("client_id: %q", q.Get("client_id"))
	}
	if q.Get("redirect_uri") != "https://auth.shiet.app/v1/github/oauth/callback" {
		t.Fatalf("redirect_uri: %q", q.Get("redirect_uri"))
	}
	if q.Get("code_challenge_method") != "S256" || q.Get("code_challenge") == "" {
		t.Fatalf("missing PKCE query: %s", authURL.RawQuery)
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

	srv := Server{
		Config:         testConfig(),
		Store:          mem,
		Clock:          func() time.Time { return now },
		HTTPClient:     tokenSrv.Client(),
		GoogleTokenURL: tokenSrv.URL,
	}
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

	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/v1/google/oauth/callback?code=google-auth-code&state=broker-state-1", nil)
	srv.Handler().ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("status: got %d body %s", rr.Code, rr.Body.String())
	}
	body := rr.Body.String()
	if !strings.Contains(body, "http://127.0.0.1:9/oauth/handoff?") {
		t.Fatalf("expected loopback handoff link in body: %s", body)
	}
	if !strings.Contains(body, "broker_state=broker-state-1") {
		t.Fatalf("expected broker_state in body: %s", body)
	}
	if !strings.Contains(body, "handoff_code=") {
		t.Fatalf("expected handoff_code in body: %s", body)
	}
	if strings.Contains(body, "access-1") || strings.Contains(body, "refresh-1") {
		t.Fatal("callback page must not include Google token material")
	}
	if len(mem.handoffs) != 1 {
		t.Fatalf("handoffs: got %d", len(mem.handoffs))
	}
	handoff := mem.handoffs[0]
	if handoff.StateID != "broker-state-1" || handoff.DesktopSessionID != "desktop-1" {
		t.Fatalf("handoff binding: %+v", handoff)
	}
	if handoff.CodeHash == "" || len(handoff.EncryptedTokenPayload) == 0 {
		t.Fatal("expected handoff code hash and encrypted payload")
	}
	if !handoff.ExpiresAt.Equal(now.Add(2 * time.Minute)) {
		t.Fatalf("handoff expiry: %s", handoff.ExpiresAt)
	}
	if mem.states[0].UsedAt == nil {
		t.Fatal("expected oauth state marked used")
	}

	// Replay must not mint another handoff.
	rr2 := httptest.NewRecorder()
	srv.Handler().ServeHTTP(rr2, httptest.NewRequest(http.MethodGet, "/v1/google/oauth/callback?code=google-auth-code&state=broker-state-1", nil))
	if rr2.Code != http.StatusBadRequest {
		t.Fatalf("replay status: got %d body %s", rr2.Code, rr2.Body.String())
	}
	if len(mem.handoffs) != 1 {
		t.Fatalf("replay minted handoffs: %d", len(mem.handoffs))
	}
}

func TestGitHubCallbackCreatesExpiringOneTimeHandoff(t *testing.T) {
	now := time.Date(2026, 7, 9, 12, 0, 0, 0, time.UTC)
	mem := &memoryStore{}
	tokenSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Fatalf("token method: %s", r.Method)
		}
		if err := r.ParseForm(); err != nil {
			t.Fatal(err)
		}
		for key, want := range map[string]string{
			"code":          "github-auth-code",
			"client_id":     "github-client-id",
			"client_secret": "github-client-secret",
			"code_verifier": "pkce-verifier",
			"redirect_uri":  "https://auth.shiet.app/v1/github/oauth/callback",
		} {
			if got := r.Form.Get(key); got != want {
				t.Fatalf("%s: got %q want %q", key, got, want)
			}
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"access_token":"gho_access","token_type":"bearer","scope":"repo,read:user"}`))
	}))
	t.Cleanup(tokenSrv.Close)

	srv := Server{
		Config:         testConfig(),
		Store:          mem,
		Clock:          func() time.Time { return now },
		HTTPClient:     tokenSrv.Client(),
		GitHubTokenURL: tokenSrv.URL,
	}
	if err := mem.SaveOAuthState(context.Background(), store.OAuthState{
		ID:                     "github-state-1",
		Provider:               "github",
		DesktopSessionID:       "desktop-1",
		PKCEVerifier:           "pkce-verifier",
		PKCEChallenge:          "pkce-challenge",
		HandoffChallenge:       pkceS256("desktop-verifier"),
		DesktopHandoffRedirect: "http://127.0.0.1:9/oauth/handoff",
		Scopes:                 []string{"repo", "read:user"},
		ExpiresAt:              now.Add(5 * time.Minute),
	}); err != nil {
		t.Fatal(err)
	}

	rr := httptest.NewRecorder()
	srv.Handler().ServeHTTP(rr, httptest.NewRequest(http.MethodGet, "/v1/github/oauth/callback?code=github-auth-code&state=github-state-1", nil))
	if rr.Code != http.StatusOK {
		t.Fatalf("status: got %d body %s", rr.Code, rr.Body.String())
	}
	if strings.Contains(rr.Body.String(), "gho_access") {
		t.Fatal("callback page must not contain GitHub token material")
	}
	if !strings.Contains(rr.Body.String(), "finish connecting GitHub") || strings.Contains(rr.Body.String(), "Google Calendar") {
		t.Fatalf("callback page must use GitHub completion copy: %s", rr.Body.String())
	}
	if len(mem.handoffs) != 1 {
		t.Fatalf("handoffs: got %d", len(mem.handoffs))
	}
	handoff := mem.handoffs[0]
	if handoff.Provider != "github" {
		t.Fatalf("provider: got %q", handoff.Provider)
	}
	if !handoff.ExpiresAt.Equal(now.Add(2 * time.Minute)) {
		t.Fatalf("handoff expiry: %s", handoff.ExpiresAt)
	}

	code := handoffCodeFromCallbackPage(t, rr.Body.String())
	body := bytes.NewBufferString(`{
		"desktop_session_id":"desktop-1",
		"broker_state":"github-state-1",
		"handoff_code":"` + code + `",
		"handoff_verifier":"desktop-verifier"
	}`)
	rr2 := httptest.NewRecorder()
	srv.Handler().ServeHTTP(rr2, httptest.NewRequest(http.MethodPost, "/v1/github/oauth/handoff", body))
	if rr2.Code != http.StatusOK {
		t.Fatalf("handoff status: got %d body %s", rr2.Code, rr2.Body.String())
	}
	var resp handoffResponse
	if err := json.NewDecoder(rr2.Body).Decode(&resp); err != nil {
		t.Fatal(err)
	}
	if resp.Provider != "github" || resp.Token.AccessToken != "gho_access" || resp.Token.RefreshToken != "" {
		t.Fatalf("handoff response: %+v", resp)
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

	rr := httptest.NewRecorder()
	srv.Handler().ServeHTTP(rr, httptest.NewRequest(http.MethodGet, "/v1/google/oauth/callback?code=x&state=missing", nil))
	if rr.Code != http.StatusBadRequest {
		t.Fatalf("missing state status: got %d", rr.Code)
	}

	_ = mem.SaveOAuthState(context.Background(), store.OAuthState{
		ID:               "expired",
		DesktopSessionID: "desktop-1",
		PKCEVerifier:     "v",
		PKCEChallenge:    "c",
		HandoffChallenge: "h",
		Scopes:           []string{"scope"},
		ExpiresAt:        now.Add(-time.Second),
	})
	rr2 := httptest.NewRecorder()
	srv.Handler().ServeHTTP(rr2, httptest.NewRequest(http.MethodGet, "/v1/google/oauth/callback?code=x&state=expired", nil))
	if rr2.Code != http.StatusBadRequest {
		t.Fatalf("expired state status: got %d", rr2.Code)
	}
}

func TestHandoffExchangeReturnsTokensOnce(t *testing.T) {
	now := time.Date(2026, 7, 8, 12, 0, 0, 0, time.UTC)
	mem := &memoryStore{}
	srv := Server{Config: testConfig(), Store: mem, Clock: func() time.Time { return now }}

	verifier := "desktop-handoff-verifier"
	challenge := pkceS256(verifier)
	payload, err := encryptTokenPayload(testConfig().GoogleClientSecret, handoffAAD("broker-state-1", "desktop-1", challenge), tokenPayload{
		AccessToken:  "access-1",
		RefreshToken: "refresh-1",
		TokenType:    "Bearer",
		Expiry:       now.Add(time.Hour),
	})
	if err != nil {
		t.Fatal(err)
	}
	code := "handoff-code-1"
	_ = mem.SaveHandoff(context.Background(), store.HandoffRecord{
		CodeHash:              hashHandoffCode(code),
		StateID:               "broker-state-1",
		DesktopSessionID:      "desktop-1",
		HandoffChallenge:      challenge,
		EncryptedTokenPayload: payload,
		AccountHint:           "user@example.com",
		Scopes:                []string{"https://www.googleapis.com/auth/calendar.readonly"},
		ExpiresAt:             now.Add(2 * time.Minute),
	})

	body := bytes.NewBufferString(`{
		"desktop_session_id":"desktop-1",
		"broker_state":"broker-state-1",
		"handoff_code":"handoff-code-1",
		"handoff_verifier":"desktop-handoff-verifier"
	}`)
	rr := httptest.NewRecorder()
	srv.Handler().ServeHTTP(rr, httptest.NewRequest(http.MethodPost, "/v1/google/oauth/handoff", body))
	if rr.Code != http.StatusOK {
		t.Fatalf("status: got %d body %s", rr.Code, rr.Body.String())
	}
	var resp handoffResponse
	if err := json.NewDecoder(rr.Body).Decode(&resp); err != nil {
		t.Fatal(err)
	}
	if resp.Provider != "google" || resp.AccountHint != "user@example.com" {
		t.Fatalf("response meta: %+v", resp)
	}
	if resp.Token.AccessToken != "access-1" || resp.Token.RefreshToken != "refresh-1" {
		t.Fatalf("token: %+v", resp.Token)
	}

	rr2 := httptest.NewRecorder()
	srv.Handler().ServeHTTP(rr2, httptest.NewRequest(http.MethodPost, "/v1/google/oauth/handoff", bytes.NewBufferString(`{
		"desktop_session_id":"desktop-1",
		"broker_state":"broker-state-1",
		"handoff_code":"handoff-code-1",
		"handoff_verifier":"desktop-handoff-verifier"
	}`)))
	if rr2.Code != http.StatusBadRequest {
		t.Fatalf("replay status: got %d body %s", rr2.Code, rr2.Body.String())
	}
	var errResp errorResponse
	_ = json.NewDecoder(rr2.Body).Decode(&errResp)
	if errResp.Error != codes.HandoffAlreadyUsed {
		t.Fatalf("replay error: %+v", errResp)
	}
}

func TestHandoffExchangeRejectsMismatchExpiryAndBadVerifier(t *testing.T) {
	now := time.Date(2026, 7, 8, 12, 0, 0, 0, time.UTC)
	mem := &memoryStore{}
	srv := Server{Config: testConfig(), Store: mem, Clock: func() time.Time { return now }}
	verifier := "desktop-handoff-verifier"
	challenge := pkceS256(verifier)
	payload, err := encryptTokenPayload(testConfig().GoogleClientSecret, handoffAAD("broker-state-1", "desktop-1", challenge), tokenPayload{
		AccessToken: "access-1", TokenType: "Bearer", Expiry: now.Add(time.Hour),
	})
	if err != nil {
		t.Fatal(err)
	}
	save := func(code string, expires time.Time) {
		t.Helper()
		_ = mem.SaveHandoff(context.Background(), store.HandoffRecord{
			CodeHash:              hashHandoffCode(code),
			StateID:               "broker-state-1",
			DesktopSessionID:      "desktop-1",
			HandoffChallenge:      challenge,
			EncryptedTokenPayload: payload,
			Scopes:                []string{"scope"},
			ExpiresAt:             expires,
		})
	}

	save("code-mismatch", now.Add(time.Minute))
	rr := httptest.NewRecorder()
	srv.Handler().ServeHTTP(rr, httptest.NewRequest(http.MethodPost, "/v1/google/oauth/handoff", bytes.NewBufferString(`{
		"desktop_session_id":"other-desktop",
		"broker_state":"broker-state-1",
		"handoff_code":"code-mismatch",
		"handoff_verifier":"desktop-handoff-verifier"
	}`)))
	if rr.Code != http.StatusBadRequest {
		t.Fatalf("session mismatch status: %d", rr.Code)
	}
	var mismatch errorResponse
	_ = json.NewDecoder(rr.Body).Decode(&mismatch)
	if mismatch.Error != codes.HandoffStateMismatch {
		t.Fatalf("session mismatch error: %+v", mismatch)
	}
	// Binding mismatch must not burn the handoff.
	rrOK := httptest.NewRecorder()
	srv.Handler().ServeHTTP(rrOK, httptest.NewRequest(http.MethodPost, "/v1/google/oauth/handoff", bytes.NewBufferString(`{
		"desktop_session_id":"desktop-1",
		"broker_state":"broker-state-1",
		"handoff_code":"code-mismatch",
		"handoff_verifier":"desktop-handoff-verifier"
	}`)))
	if rrOK.Code != http.StatusOK {
		t.Fatalf("retry after mismatch status: %d body %s", rrOK.Code, rrOK.Body.String())
	}

	save("code-bad-verifier", now.Add(time.Minute))
	rr2 := httptest.NewRecorder()
	srv.Handler().ServeHTTP(rr2, httptest.NewRequest(http.MethodPost, "/v1/google/oauth/handoff", bytes.NewBufferString(`{
		"desktop_session_id":"desktop-1",
		"broker_state":"broker-state-1",
		"handoff_code":"code-bad-verifier",
		"handoff_verifier":"wrong-verifier"
	}`)))
	if rr2.Code != http.StatusBadRequest {
		t.Fatalf("verifier mismatch status: %d", rr2.Code)
	}

	save("code-expired", now.Add(-time.Second))
	rr3 := httptest.NewRecorder()
	srv.Handler().ServeHTTP(rr3, httptest.NewRequest(http.MethodPost, "/v1/google/oauth/handoff", bytes.NewBufferString(`{
		"desktop_session_id":"desktop-1",
		"broker_state":"broker-state-1",
		"handoff_code":"code-expired",
		"handoff_verifier":"desktop-handoff-verifier"
	}`)))
	if rr3.Code != http.StatusBadRequest {
		t.Fatalf("expired status: %d", rr3.Code)
	}
}

func TestRevokeGoogleOAuthSuccess(t *testing.T) {
	mem := &memoryStore{}
	var gotToken string
	revokeSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Fatalf("method: %s", r.Method)
		}
		if err := r.ParseForm(); err != nil {
			t.Fatal(err)
		}
		gotToken = r.Form.Get("token")
		if r.Form.Get("client_secret") != "" {
			t.Fatal("revoke must not send client_secret")
		}
		w.WriteHeader(http.StatusOK)
	}))
	t.Cleanup(revokeSrv.Close)

	srv := Server{
		Config:          testConfig(),
		Store:           mem,
		HTTPClient:      revokeSrv.Client(),
		GoogleRevokeURL: revokeSrv.URL,
	}

	rr := httptest.NewRecorder()
	srv.Handler().ServeHTTP(rr, httptest.NewRequest(http.MethodPost, "/v1/google/oauth/revoke", bytes.NewBufferString(`{
		"refresh_token":"refresh-to-revoke",
		"reason":"user_disconnect"
	}`)))
	if rr.Code != http.StatusOK {
		t.Fatalf("status: got %d body %s", rr.Code, rr.Body.String())
	}
	var resp revokeResponse
	if err := json.NewDecoder(rr.Body).Decode(&resp); err != nil {
		t.Fatal(err)
	}
	if !resp.Revoked {
		t.Fatal("expected revoked=true")
	}
	if gotToken != "refresh-to-revoke" {
		t.Fatalf("google token: got %q", gotToken)
	}
	if len(mem.states) != 0 || len(mem.handoffs) != 0 {
		t.Fatalf("revoke must not write store; states=%d handoffs=%d", len(mem.states), len(mem.handoffs))
	}
}

func TestRevokeGitHubOAuthUsesServerCredentialsWithoutPersistingToken(t *testing.T) {
	mem := &memoryStore{}
	var gotToken string
	revokeSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodDelete || r.URL.Path != "/applications/github-client-id/token" {
			t.Fatalf("request: %s %s", r.Method, r.URL.Path)
		}
		user, pass, ok := r.BasicAuth()
		if !ok || user != "github-client-id" || pass != "github-client-secret" {
			t.Fatalf("basic auth: user=%q pass=%q ok=%v", user, pass, ok)
		}
		var body struct {
			AccessToken string `json:"access_token"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Fatal(err)
		}
		gotToken = body.AccessToken
		w.WriteHeader(http.StatusNoContent)
	}))
	t.Cleanup(revokeSrv.Close)

	srv := Server{
		Config:          testConfig(),
		Store:           mem,
		HTTPClient:      revokeSrv.Client(),
		GitHubRevokeURL: revokeSrv.URL,
	}
	rr := httptest.NewRecorder()
	srv.Handler().ServeHTTP(rr, httptest.NewRequest(http.MethodPost, "/v1/github/oauth/revoke", bytes.NewBufferString(`{
		"access_token":"gho_revoke",
		"reason":"user_disconnect"
	}`)))
	if rr.Code != http.StatusOK {
		t.Fatalf("status: got %d body %s", rr.Code, rr.Body.String())
	}
	if gotToken != "gho_revoke" {
		t.Fatalf("access token: got %q", gotToken)
	}
	if len(mem.states) != 0 || len(mem.handoffs) != 0 {
		t.Fatalf("revoke must not write store; states=%d handoffs=%d", len(mem.states), len(mem.handoffs))
	}
}

func TestRevokeGoogleOAuthAlreadyRevoked(t *testing.T) {
	revokeSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		_, _ = w.Write([]byte(`{"error":"invalid_token"}`))
	}))
	t.Cleanup(revokeSrv.Close)

	srv := Server{
		Config:          testConfig(),
		Store:           &memoryStore{},
		HTTPClient:      revokeSrv.Client(),
		GoogleRevokeURL: revokeSrv.URL,
	}

	rr := httptest.NewRecorder()
	srv.Handler().ServeHTTP(rr, httptest.NewRequest(http.MethodPost, "/v1/google/oauth/revoke", bytes.NewBufferString(`{
		"refresh_token":"already-gone"
	}`)))
	if rr.Code != http.StatusOK {
		t.Fatalf("status: got %d body %s", rr.Code, rr.Body.String())
	}
	var resp revokeResponse
	if err := json.NewDecoder(rr.Body).Decode(&resp); err != nil {
		t.Fatal(err)
	}
	if !resp.Revoked {
		t.Fatal("expected revoked=true for invalid_token")
	}
}

func TestRevokeGoogleOAuthMissingRefreshToken(t *testing.T) {
	srv := Server{Config: testConfig(), Store: &memoryStore{}}

	rr := httptest.NewRecorder()
	srv.Handler().ServeHTTP(rr, httptest.NewRequest(http.MethodPost, "/v1/google/oauth/revoke", bytes.NewBufferString(`{
		"reason":"user_disconnect"
	}`)))
	if rr.Code != http.StatusBadRequest {
		t.Fatalf("status: got %d body %s", rr.Code, rr.Body.String())
	}
	if !strings.Contains(rr.Body.String(), "refresh_token_required") {
		t.Fatalf("body: %s", rr.Body.String())
	}
}

func TestRevokeGoogleOAuthGoogleFailure(t *testing.T) {
	revokeSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte(`oops`))
	}))
	t.Cleanup(revokeSrv.Close)

	srv := Server{
		Config:          testConfig(),
		Store:           &memoryStore{},
		HTTPClient:      revokeSrv.Client(),
		GoogleRevokeURL: revokeSrv.URL,
	}

	rr := httptest.NewRecorder()
	srv.Handler().ServeHTTP(rr, httptest.NewRequest(http.MethodPost, "/v1/google/oauth/revoke", bytes.NewBufferString(`{
		"refresh_token":"refresh-1"
	}`)))
	if rr.Code != http.StatusBadGateway {
		t.Fatalf("status: got %d body %s", rr.Code, rr.Body.String())
	}
	if !strings.Contains(rr.Body.String(), "google_revoke_failed") {
		t.Fatalf("body: %s", rr.Body.String())
	}
}

func testConfig() brokerconfig.Config {
	return brokerconfig.Config{
		ListenAddr:              ":8080",
		PublicOrigin:            "https://auth.shiet.app",
		GoogleClientID:          "google-client-id",
		GoogleClientSecret:      "google-client-secret",
		DesktopHandoffURL:       "shiet://oauth/google/handoff",
		GitHubClientID:          "github-client-id",
		GitHubClientSecret:      "github-client-secret",
		GitHubDesktopHandoffURL: "shiet://oauth/github/handoff",
		DatastoreDSN:            "file:broker.db",
		StateTTL:                5 * time.Minute,
		HandoffTTL:              2 * time.Minute,
		GoogleScopes:            []string{"https://www.googleapis.com/auth/calendar.readonly"},
		GitHubScopes:            []string{"repo", "read:user"},
	}
}

type memoryStore struct {
	states   []store.OAuthState
	handoffs []store.HandoffRecord
}

func (m *memoryStore) Ping(context.Context) error {
	return nil
}

func (m *memoryStore) SaveOAuthState(_ context.Context, rec store.OAuthState) error {
	m.states = append(m.states, rec)
	return nil
}

func (m *memoryStore) ConsumeOAuthState(_ context.Context, id, provider string, now time.Time) (store.OAuthState, error) {
	for i := range m.states {
		rec := &m.states[i]
		if rec.ID != id {
			continue
		}
		if rec.UsedAt != nil {
			return store.OAuthState{}, store.ErrAlreadyUsed
		}
		if !now.Before(rec.ExpiresAt) {
			return store.OAuthState{}, store.ErrExpired
		}
		if providerOrGoogle(rec.Provider) != providerOrGoogle(provider) {
			return store.OAuthState{}, store.ErrMismatch
		}
		used := now
		rec.UsedAt = &used
		return *rec, nil
	}
	return store.OAuthState{}, store.ErrNotFound
}

func (m *memoryStore) SaveHandoff(_ context.Context, rec store.HandoffRecord) error {
	m.handoffs = append(m.handoffs, rec)
	return nil
}

func (m *memoryStore) ConsumeHandoff(_ context.Context, codeHash, provider, desktopSessionID, stateID, handoffChallenge string, now time.Time) (store.HandoffRecord, error) {
	for i := range m.handoffs {
		rec := &m.handoffs[i]
		if rec.CodeHash != codeHash {
			continue
		}
		if rec.UsedAt != nil {
			return store.HandoffRecord{}, store.ErrAlreadyUsed
		}
		if !now.Before(rec.ExpiresAt) {
			return store.HandoffRecord{}, store.ErrExpired
		}
		if providerOrGoogle(rec.Provider) != providerOrGoogle(provider) {
			return store.HandoffRecord{}, store.ErrMismatch
		}
		if rec.DesktopSessionID != desktopSessionID || rec.StateID != stateID || rec.HandoffChallenge != handoffChallenge {
			return store.HandoffRecord{}, store.ErrMismatch
		}
		out := *rec
		used := now
		rec.UsedAt = &used
		rec.EncryptedTokenPayload = nil
		return out, nil
	}
	return store.HandoffRecord{}, store.ErrNotFound
}

func TestRefreshGoogleOAuthReturnsTokensWithoutPersisting(t *testing.T) {
	now := time.Date(2026, 7, 8, 12, 0, 0, 0, time.UTC)
	mem := &memoryStore{}
	var gotForm url.Values
	tokenSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if err := r.ParseForm(); err != nil {
			t.Fatalf("parse form: %v", err)
		}
		gotForm = cloneURLValues(r.PostForm)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{
			"access_token":"access-fresh",
			"refresh_token":"refresh-rotated",
			"token_type":"Bearer",
			"expires_in":3600
		}`))
	}))
	defer tokenSrv.Close()

	srv := Server{
		Config:         testConfig(),
		Store:          mem,
		Clock:          func() time.Time { return now },
		GoogleTokenURL: tokenSrv.URL,
	}

	rr := httptest.NewRecorder()
	srv.Handler().ServeHTTP(rr, httptest.NewRequest(http.MethodPost, "/v1/google/oauth/refresh", bytes.NewBufferString(`{
		"refresh_token":"refresh-old",
		"scope":["https://www.googleapis.com/auth/calendar.readonly"],
		"app_version":"0.1.0",
		"platform":"darwin-arm64"
	}`)))
	if rr.Code != http.StatusOK {
		t.Fatalf("status: got %d body %s", rr.Code, rr.Body.String())
	}

	var resp refreshResponse
	if err := json.NewDecoder(rr.Body).Decode(&resp); err != nil {
		t.Fatal(err)
	}
	if resp.AccessToken != "access-fresh" {
		t.Fatalf("access_token: %q", resp.AccessToken)
	}
	if resp.RefreshToken != "refresh-rotated" {
		t.Fatalf("refresh_token: %q", resp.RefreshToken)
	}
	if resp.TokenType != "Bearer" {
		t.Fatalf("token_type: %q", resp.TokenType)
	}
	if !resp.Expiry.Equal(now.Add(time.Hour)) {
		t.Fatalf("expiry: got %s", resp.Expiry)
	}

	if gotForm.Get("grant_type") != "refresh_token" {
		t.Fatalf("grant_type: %q", gotForm.Get("grant_type"))
	}
	if gotForm.Get("refresh_token") != "refresh-old" {
		t.Fatalf("refresh_token form: %q", gotForm.Get("refresh_token"))
	}
	if gotForm.Get("client_id") != "google-client-id" {
		t.Fatalf("client_id: %q", gotForm.Get("client_id"))
	}
	if gotForm.Get("client_secret") != "google-client-secret" {
		t.Fatalf("client_secret: %q", gotForm.Get("client_secret"))
	}
	if len(mem.states) != 0 || len(mem.handoffs) != 0 {
		t.Fatalf("store mutated: states=%d handoffs=%d", len(mem.states), len(mem.handoffs))
	}
}

func TestRefreshGoogleOAuthInvalidGrant(t *testing.T) {
	tokenSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		_, _ = w.Write([]byte(`{"error":"invalid_grant","error_description":"Token has been expired or revoked."}`))
	}))
	defer tokenSrv.Close()

	srv := Server{Config: testConfig(), Store: &memoryStore{}, GoogleTokenURL: tokenSrv.URL}
	rr := httptest.NewRecorder()
	srv.Handler().ServeHTTP(rr, httptest.NewRequest(http.MethodPost, "/v1/google/oauth/refresh", bytes.NewBufferString(`{
		"refresh_token":"bad-refresh"
	}`)))
	if rr.Code != http.StatusBadRequest {
		t.Fatalf("status: got %d body %s", rr.Code, rr.Body.String())
	}
	var errResp errorResponse
	_ = json.NewDecoder(rr.Body).Decode(&errResp)
	if errResp.Error != codes.InvalidRefreshToken {
		t.Fatalf("error: %+v", errResp)
	}
}

func TestRefreshGoogleOAuthGoogleUnavailable(t *testing.T) {
	tokenSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte(`{"error":"server_error"}`))
	}))
	defer tokenSrv.Close()

	srv := Server{Config: testConfig(), Store: &memoryStore{}, GoogleTokenURL: tokenSrv.URL}
	rr := httptest.NewRecorder()
	srv.Handler().ServeHTTP(rr, httptest.NewRequest(http.MethodPost, "/v1/google/oauth/refresh", bytes.NewBufferString(`{
		"refresh_token":"refresh-old"
	}`)))
	if rr.Code != http.StatusBadGateway {
		t.Fatalf("status: got %d body %s", rr.Code, rr.Body.String())
	}
	var errResp errorResponse
	_ = json.NewDecoder(rr.Body).Decode(&errResp)
	if errResp.Error != "google_token_refresh_failed" {
		t.Fatalf("error: %+v", errResp)
	}
}

func TestRefreshGoogleOAuthRequiresRefreshToken(t *testing.T) {
	srv := Server{Config: testConfig(), Store: &memoryStore{}}
	rr := httptest.NewRecorder()
	srv.Handler().ServeHTTP(rr, httptest.NewRequest(http.MethodPost, "/v1/google/oauth/refresh", bytes.NewBufferString(`{}`)))
	if rr.Code != http.StatusBadRequest {
		t.Fatalf("status: got %d body %s", rr.Code, rr.Body.String())
	}
}

func TestStartGoogleOAuthAuthDisabled(t *testing.T) {
	cfg := testConfig()
	cfg.AuthDisabled = true
	metrics := observe.NewMetrics()
	srv := Server{Config: cfg, Store: &memoryStore{}, Metrics: metrics}

	rr := httptest.NewRecorder()
	srv.Handler().ServeHTTP(rr, httptest.NewRequest(http.MethodPost, "/v1/google/oauth/start", bytes.NewBufferString(`{
		"desktop_session_id":"desktop-1",
		"handoff_challenge":"challenge-1"
	}`)))
	if rr.Code != http.StatusForbidden {
		t.Fatalf("status: got %d body %s", rr.Code, rr.Body.String())
	}
	var errResp errorResponse
	_ = json.NewDecoder(rr.Body).Decode(&errResp)
	if errResp.Error != codes.AuthDisabled {
		t.Fatalf("error: %+v", errResp)
	}
	if metrics.KillSwitchCount(codes.SurfaceStart) != 1 {
		t.Fatalf("kill switch metric: %d", metrics.KillSwitchCount(codes.SurfaceStart))
	}
}

func TestRefreshGoogleOAuthRefreshDisabled(t *testing.T) {
	cfg := testConfig()
	cfg.RefreshDisabled = true
	metrics := observe.NewMetrics()
	srv := Server{Config: cfg, Store: &memoryStore{}, Metrics: metrics}

	rr := httptest.NewRecorder()
	srv.Handler().ServeHTTP(rr, httptest.NewRequest(http.MethodPost, "/v1/google/oauth/refresh", bytes.NewBufferString(`{
		"refresh_token":"refresh-old"
	}`)))
	if rr.Code != http.StatusForbidden {
		t.Fatalf("status: got %d body %s", rr.Code, rr.Body.String())
	}
	var errResp errorResponse
	_ = json.NewDecoder(rr.Body).Decode(&errResp)
	if errResp.Error != codes.RefreshDisabled {
		t.Fatalf("error: %+v", errResp)
	}
	if metrics.KillSwitchCount(codes.SurfaceRefresh) != 1 {
		t.Fatalf("kill switch metric: %d", metrics.KillSwitchCount(codes.SurfaceRefresh))
	}
}

func TestStartGoogleOAuthAppVersionDisabled(t *testing.T) {
	cfg := testConfig()
	cfg.DisabledAppVersions = []string{"0.1.0"}
	srv := Server{Config: cfg, Store: &memoryStore{}, Metrics: observe.NewMetrics()}

	rr := httptest.NewRecorder()
	srv.Handler().ServeHTTP(rr, httptest.NewRequest(http.MethodPost, "/v1/google/oauth/start", bytes.NewBufferString(`{
		"desktop_session_id":"desktop-1",
		"handoff_challenge":"challenge-1",
		"app_version":"0.1.0"
	}`)))
	if rr.Code != http.StatusForbidden {
		t.Fatalf("status: got %d body %s", rr.Code, rr.Body.String())
	}
	if !strings.Contains(rr.Body.String(), codes.AppVersionDisabled) {
		t.Fatalf("body: %s", rr.Body.String())
	}
}

func TestStartGoogleOAuthRateLimited(t *testing.T) {
	now := time.Date(2026, 7, 9, 12, 0, 0, 0, time.UTC)
	metrics := observe.NewMetrics()
	lim := ratelimit.New(time.Minute, func() time.Time { return now })
	srv := Server{
		Config:  testConfig(),
		Store:   &memoryStore{},
		Clock:   func() time.Time { return now },
		Limiter: lim,
		Metrics: metrics,
	}

	body := `{"desktop_session_id":"desktop-1","handoff_challenge":"challenge-1"}`
	for i := 0; i < limitStart; i++ {
		rr := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodPost, "/v1/google/oauth/start", bytes.NewBufferString(body))
		req.RemoteAddr = "203.0.113.42:1234"
		srv.Handler().ServeHTTP(rr, req)
		if rr.Code != http.StatusCreated {
			t.Fatalf("request %d status: got %d body %s", i+1, rr.Code, rr.Body.String())
		}
	}

	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/v1/google/oauth/start", bytes.NewBufferString(body))
	req.RemoteAddr = "203.0.113.42:1234"
	srv.Handler().ServeHTTP(rr, req)
	if rr.Code != http.StatusTooManyRequests {
		t.Fatalf("status: got %d body %s", rr.Code, rr.Body.String())
	}
	if !strings.Contains(rr.Body.String(), codes.RateLimited) {
		t.Fatalf("body: %s", rr.Body.String())
	}
	if metrics.RateLimitedCount(codes.SurfaceStart) != 1 {
		t.Fatalf("rate limited metric: %d", metrics.RateLimitedCount(codes.SurfaceStart))
	}
}

func TestHandoffFailureCountedForMonitoring(t *testing.T) {
	metrics := observe.NewMetrics()
	srv := Server{Config: testConfig(), Store: &memoryStore{}, Metrics: metrics}

	rr := httptest.NewRecorder()
	srv.Handler().ServeHTTP(rr, httptest.NewRequest(http.MethodPost, "/v1/google/oauth/handoff", bytes.NewBufferString(`{
		"desktop_session_id":"desktop-1",
		"broker_state":"missing",
		"handoff_code":"code-1",
		"handoff_verifier":"verifier-1"
	}`)))
	if rr.Code != http.StatusBadRequest {
		t.Fatalf("status: got %d body %s", rr.Code, rr.Body.String())
	}
	if metrics.HandoffFailureCount(codes.OutcomeNotFound) != 1 {
		t.Fatalf("handoff failure metric: %d", metrics.HandoffFailureCount(codes.OutcomeNotFound))
	}
}

func TestMetricsEndpoint(t *testing.T) {
	metrics := observe.NewMetrics()
	metrics.IncAuthStart()
	srv := Server{Config: testConfig(), Store: &memoryStore{}, Metrics: metrics}

	rr := httptest.NewRecorder()
	srv.Handler().ServeHTTP(rr, httptest.NewRequest(http.MethodGet, "/metrics", nil))
	if rr.Code != http.StatusOK {
		t.Fatalf("status: got %d", rr.Code)
	}
	if !strings.Contains(rr.Body.String(), "broker_auth_starts_total 1") {
		t.Fatalf("body: %s", rr.Body.String())
	}
}

func cloneURLValues(values url.Values) url.Values {
	out := make(url.Values, len(values))
	for key, value := range values {
		out[key] = append([]string(nil), value...)
	}
	return out
}
