package oauth_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/dylanbr0wn/shiet/internal/integration/oauth"
)

func TestExchangeAuthorizationCode_SlackAuthedUserToken(t *testing.T) {
	p := oauth.MustLookup(oauth.ProviderSlack)
	tokenServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"ok": true,
			"authed_user": map[string]any{
				"access_token": "xoxp-nested",
				"scope":        "channels:history,channels:read",
				"token_type":   "user",
			},
		})
	}))
	t.Cleanup(tokenServer.Close)

	tok, err := oauth.ExchangeAuthorizationCode(context.Background(), p, oauth.ClientCredentials{
		ClientID:     "slack-client",
		ClientSecret: "slack-secret",
	}, "https://broker.example/callback", "code-1", "verifier-1", oauth.ExchangeOptions{
		HTTPClient: tokenServer.Client(),
		TokenURL:   tokenServer.URL,
	})
	if err != nil {
		t.Fatal(err)
	}
	if tok.AccessToken != "xoxp-nested" {
		t.Fatalf("token: %+v", tok)
	}
	if tok.Scope != "channels:history,channels:read" {
		t.Fatalf("scope: %q", tok.Scope)
	}
}
