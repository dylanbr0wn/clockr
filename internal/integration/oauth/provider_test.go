package oauth_test

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/dylanbr0wn/shiet/internal/integration/oauth"
)

func TestLookupProviders(t *testing.T) {
	google, ok := oauth.Lookup(oauth.ProviderGoogle)
	if !ok {
		t.Fatal("expected google provider")
	}
	if google.DisplayName != "Google" || google.AuthURLHost != "accounts.google.com" {
		t.Fatalf("google descriptor: %+v", google)
	}
	if !google.Capabilities.Refresh || !google.Capabilities.Revoke {
		t.Fatalf("google capabilities: %+v", google.Capabilities)
	}
	if strings.Contains(google.AuthURL, "client_secret") || google.TokenURL == "" {
		t.Fatal("descriptor must stay credential-free with real endpoints")
	}

	github, ok := oauth.Lookup(oauth.ProviderGitHub)
	if !ok {
		t.Fatal("expected github provider")
	}
	if github.Capabilities.Refresh {
		t.Fatal("github must not advertise refresh")
	}
	if err := github.ValidateAuthorizationURL("https://github.com/login/oauth/authorize?client_id=x"); err != nil {
		t.Fatal(err)
	}
	if err := google.ValidateAuthorizationURL("https://evil.example/o/oauth2/v2/auth"); err == nil {
		t.Fatal("expected host rejection")
	}

	slack, ok := oauth.Lookup(oauth.ProviderSlack)
	if !ok {
		t.Fatal("expected slack provider")
	}
	if slack.ScopeParam != "user_scope" {
		t.Fatalf("slack scope param: %q", slack.ScopeParam)
	}
	if err := slack.ValidateAuthorizationURL("https://slack.com/oauth/v2/authorize?client_id=x"); err != nil {
		t.Fatal(err)
	}
}

func TestBuildAuthorizationURLAndExchangeSharedContract(t *testing.T) {
	providers := []string{oauth.ProviderGoogle, oauth.ProviderGitHub, oauth.ProviderSlack}
	for _, id := range providers {
		t.Run(id, func(t *testing.T) {
			p := oauth.MustLookup(id)
			var gotForm url.Values
			var gotAccept string
			tokenServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				body, _ := io.ReadAll(r.Body)
				gotForm, _ = url.ParseQuery(string(body))
				gotAccept = r.Header.Get("Accept")
				w.Header().Set("Content-Type", "application/json")
				_ = json.NewEncoder(w).Encode(map[string]any{
					"access_token":  "access-" + id,
					"refresh_token": "refresh-" + id,
					"token_type":    "bearer",
					"expires_in":    3600,
					"scope":         strings.Join(p.DefaultScopes, " "),
				})
			}))
			t.Cleanup(tokenServer.Close)

			creds := oauth.ClientCredentials{ClientID: "client-" + id, ClientSecret: "secret-" + id}
			authURL, err := oauth.BuildAuthorizationURL(p, creds, "https://broker.example/callback", "state-1", "challenge-1", nil)
			if err != nil {
				t.Fatal(err)
			}
			u, err := url.Parse(authURL)
			if err != nil {
				t.Fatal(err)
			}
			if u.Scheme != "https" || u.Host != p.AuthURLHost {
				t.Fatalf("auth url host: %s", authURL)
			}
			q := u.Query()
			if q.Get("client_id") != creds.ClientID || q.Get("code_challenge") != "challenge-1" {
				t.Fatalf("auth query: %v", q)
			}
			if id == oauth.ProviderGoogle {
				if q.Get("access_type") != "offline" || q.Get("prompt") != "consent" {
					t.Fatalf("google auth params missing: %v", q)
				}
			}

			tok, err := oauth.ExchangeAuthorizationCode(context.Background(), p, creds, "https://broker.example/callback", "code-1", "verifier-1", oauth.ExchangeOptions{
				HTTPClient: tokenServer.Client(),
				TokenURL:   tokenServer.URL,
			})
			if err != nil {
				t.Fatal(err)
			}
			if tok.AccessToken != "access-"+id {
				t.Fatalf("token: %+v", tok)
			}
			if gotForm.Get("client_id") != creds.ClientID || gotForm.Get("client_secret") != creds.ClientSecret {
				t.Fatalf("form: %v", gotForm)
			}
			if gotForm.Get("code") != "code-1" || gotForm.Get("code_verifier") != "verifier-1" {
				t.Fatalf("pkce form: %v", gotForm)
			}
			if id == oauth.ProviderGitHub && gotAccept != "application/json" {
				t.Fatalf("github accept header: %q", gotAccept)
			}
			if id == oauth.ProviderSlack {
				if gotAccept != "application/json" {
					t.Fatalf("slack accept header: %q", gotAccept)
				}
				if q.Get("user_scope") == "" {
					t.Fatalf("slack user_scope missing: %v", q)
				}
				if q.Get("scope") != "" {
					t.Fatalf("slack must not use scope param: %v", q)
				}
			}
		})
	}
}
