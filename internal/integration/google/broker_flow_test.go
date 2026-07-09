package google_test

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/dylanbr0wn/shiet/internal/integration/google"
)

func TestBrokerFlowAuthorizeSuccess(t *testing.T) {
	var (
		mu            sync.Mutex
		gotStart      map[string]any
		gotHandoff    map[string]any
		handoffCalled bool
	)

	broker := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodPost && r.URL.Path == "/v1/google/oauth/start":
			var req map[string]any
			if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
				t.Fatal(err)
			}
			mu.Lock()
			gotStart = req
			mu.Unlock()
			redirect, _ := req["desktop_handoff_redirect"].(string)
			if redirect == "" {
				t.Fatal("expected desktop_handoff_redirect")
			}
			// Simulate broker callback completing by hitting the desktop handoff URL.
			go func() {
				time.Sleep(20 * time.Millisecond)
				resp, err := http.Get(redirect + "?broker_state=state-1&handoff_code=code-1")
				if err != nil {
					t.Errorf("desktop handoff hit: %v", err)
					return
				}
				_ = resp.Body.Close()
			}()
			w.WriteHeader(http.StatusCreated)
			_ = json.NewEncoder(w).Encode(map[string]any{
				"auth_url":     "https://accounts.google.com/o/oauth2/v2/auth?state=state-1",
				"broker_state": "state-1",
				"expires_at":   time.Now().Add(time.Minute).UTC().Format(time.RFC3339),
			})
		case r.Method == http.MethodPost && r.URL.Path == "/v1/google/oauth/handoff":
			var req map[string]any
			if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
				t.Fatal(err)
			}
			mu.Lock()
			gotHandoff = req
			handoffCalled = true
			mu.Unlock()
			_ = json.NewEncoder(w).Encode(map[string]any{
				"provider":     "google",
				"account_hint": "user@example.com",
				"scope":        []string{"https://www.googleapis.com/auth/calendar.readonly"},
				"token": map[string]any{
					"access_token":  "access-token",
					"refresh_token": "refresh-token",
					"token_type":    "Bearer",
					"expiry":        time.Date(2026, 7, 8, 13, 0, 0, 0, time.UTC),
				},
			})
		default:
			http.NotFound(w, r)
		}
	}))
	t.Cleanup(broker.Close)

	var opened string
	flow := &google.BrokerFlow{
		BaseURL:    broker.URL,
		HTTPClient: broker.Client(),
		OpenURL: func(u string) error {
			opened = u
			return nil
		},
		AppVersion: "0.1.0",
		Platform:   "test",
	}

	result, err := flow.Authorize(context.Background(), "user@example.com")
	if err != nil {
		t.Fatal(err)
	}
	if opened == "" || !strings.Contains(opened, "accounts.google.com") {
		t.Fatalf("expected Google auth URL opened, got %q", opened)
	}
	if result.Token.AccessToken != "access-token" || result.Token.RefreshToken != "refresh-token" {
		t.Fatalf("token: %+v", result.Token)
	}

	mu.Lock()
	defer mu.Unlock()
	if !handoffCalled {
		t.Fatal("expected handoff exchange")
	}
	if gotStart["desktop_session_id"] == "" || gotStart["handoff_challenge"] == "" {
		t.Fatalf("start payload: %#v", gotStart)
	}
	if gotHandoff["broker_state"] != "state-1" || gotHandoff["handoff_code"] != "code-1" {
		t.Fatalf("handoff payload: %#v", gotHandoff)
	}
	if gotHandoff["handoff_verifier"] == "" {
		t.Fatal("expected handoff_verifier")
	}
	if gotHandoff["desktop_session_id"] != gotStart["desktop_session_id"] {
		t.Fatalf("session mismatch start=%v handoff=%v", gotStart["desktop_session_id"], gotHandoff["desktop_session_id"])
	}
}

func TestBrokerFlowAuthorizeHandoffFailures(t *testing.T) {
	cases := []struct {
		name    string
		code    string
		wantErr error
	}{
		{name: "replay", code: "handoff_already_used", wantErr: google.ErrHandoffReplay},
		{name: "expired", code: "handoff_expired", wantErr: google.ErrHandoffExpired},
		{name: "state mismatch", code: "handoff_state_mismatch", wantErr: google.ErrHandoffStateMismatch},
		{name: "broker error", code: "rate_limited", wantErr: google.ErrBrokerRejected},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			broker := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				switch {
				case r.URL.Path == "/v1/google/oauth/start":
					var req map[string]any
					_ = json.NewDecoder(r.Body).Decode(&req)
					redirect, _ := req["desktop_handoff_redirect"].(string)
					go func() {
						time.Sleep(20 * time.Millisecond)
						resp, err := http.Get(redirect + "?broker_state=state-1&handoff_code=code-1")
						if err == nil {
							_ = resp.Body.Close()
						}
					}()
					w.WriteHeader(http.StatusCreated)
					_ = json.NewEncoder(w).Encode(map[string]any{
						"auth_url":     "https://accounts.google.com/o/oauth2/v2/auth",
						"broker_state": "state-1",
					})
				case r.URL.Path == "/v1/google/oauth/handoff":
					w.WriteHeader(http.StatusBadRequest)
					_ = json.NewEncoder(w).Encode(map[string]string{"error": tc.code})
				default:
					http.NotFound(w, r)
				}
			}))
			t.Cleanup(broker.Close)

			flow := &google.BrokerFlow{
				BaseURL:    broker.URL,
				HTTPClient: broker.Client(),
				OpenURL:    func(string) error { return nil },
			}
			_, err := flow.Authorize(context.Background(), "user@example.com")
			if err == nil {
				t.Fatal("expected error")
			}
			if !errors.Is(err, tc.wantErr) {
				t.Fatalf("want %v, got %v", tc.wantErr, err)
			}
		})
	}
}

func TestBrokerFlowAuthorizeStartUnavailable(t *testing.T) {
	broker := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusServiceUnavailable)
		_ = json.NewEncoder(w).Encode(map[string]string{"error": "datastore_unavailable"})
	}))
	t.Cleanup(broker.Close)

	flow := &google.BrokerFlow{
		BaseURL:    broker.URL,
		HTTPClient: broker.Client(),
		OpenURL:    func(string) error { return nil },
	}
	_, err := flow.Authorize(context.Background(), "user@example.com")
	if !errors.Is(err, google.ErrBrokerUnavailable) {
		t.Fatalf("want ErrBrokerUnavailable, got %v", err)
	}
}

func TestBrokerFlowRevokeSuccess(t *testing.T) {
	var got map[string]any
	broker := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.URL.Path != "/v1/google/oauth/revoke" {
			http.NotFound(w, r)
			return
		}
		if err := json.NewDecoder(r.Body).Decode(&got); err != nil {
			t.Fatal(err)
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"revoked": true})
	}))
	t.Cleanup(broker.Close)

	flow := &google.BrokerFlow{
		BaseURL:    broker.URL,
		HTTPClient: broker.Client(),
	}
	if err := flow.Revoke(context.Background(), "refresh-token"); err != nil {
		t.Fatal(err)
	}
	if got["refresh_token"] != "refresh-token" {
		t.Fatalf("payload: %#v", got)
	}
	if got["reason"] != "user_disconnect" {
		t.Fatalf("reason: %#v", got["reason"])
	}
}

func TestBrokerFlowRevokeUnavailable(t *testing.T) {
	broker := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadGateway)
		_ = json.NewEncoder(w).Encode(map[string]string{"error": "google_revoke_failed"})
	}))
	t.Cleanup(broker.Close)

	flow := &google.BrokerFlow{
		BaseURL:    broker.URL,
		HTTPClient: broker.Client(),
	}
	err := flow.Revoke(context.Background(), "refresh-token")
	if !errors.Is(err, google.ErrBrokerUnavailable) {
		t.Fatalf("want ErrBrokerUnavailable, got %v", err)
	}
}
