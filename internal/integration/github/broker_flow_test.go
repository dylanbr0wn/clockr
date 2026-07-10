package github_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/dylanbr0wn/shiet/internal/integration/github"
)

func TestBrokerFlowAuthorizeUsesGitHubRoutes(t *testing.T) {
	broker := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/v1/github/oauth/start":
			var req map[string]string
			if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
				t.Fatal(err)
			}
			go func() {
				time.Sleep(20 * time.Millisecond)
				resp, err := http.Get(req["desktop_handoff_redirect"] + "?broker_state=state-1&handoff_code=code-1")
				if err == nil {
					_ = resp.Body.Close()
				}
			}()
			w.WriteHeader(http.StatusCreated)
			_ = json.NewEncoder(w).Encode(map[string]any{
				"auth_url":     "https://github.com/login/oauth/authorize?state=state-1",
				"broker_state": "state-1",
				"expires_at":   time.Now().Add(time.Minute),
			})
		case "/v1/github/oauth/handoff":
			_ = json.NewEncoder(w).Encode(map[string]any{
				"provider": "github",
				"scope":    []string{"repo", "read:user"},
				"token":    map[string]any{"access_token": "gho_access", "token_type": "bearer"},
			})
		default:
			http.NotFound(w, r)
		}
	}))
	t.Cleanup(broker.Close)

	var opened string
	flow := &github.BrokerFlow{
		BaseURL:    broker.URL,
		HTTPClient: broker.Client(),
		OpenURL:    func(raw string) error { opened = raw; return nil },
		AppVersion: "0.2.0",
		Platform:   "test",
	}
	result, err := flow.Authorize(context.Background(), "github")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(opened, "https://github.com/login/oauth/authorize") {
		t.Fatalf("opened: %q", opened)
	}
	if result.Provider != "github" || result.Token.AccessToken != "gho_access" || result.Token.RefreshToken != "" {
		t.Fatalf("result: %+v", result)
	}
}
