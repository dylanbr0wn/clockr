package oauth_test

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"

	"github.com/dylanbr0wn/clockr/internal/integration/oauth"
	"github.com/dylanbr0wn/clockr/internal/integration/secrets"
	"golang.org/x/oauth2"
)

func TestParseCallback(t *testing.T) {
	code, err := oauth.ParseCallback(
		"http://127.0.0.1:1234/oauth/callback?code=abc&state=xyz",
		"xyz",
	)
	if err != nil || code != "abc" {
		t.Fatalf("ParseCallback: code=%q err=%v", code, err)
	}

	_, err = oauth.ParseCallback(
		"http://127.0.0.1:1234/oauth/callback?code=abc&state=bad",
		"xyz",
	)
	if err == nil {
		t.Fatal("expected state mismatch")
	}
}

func TestProviderOAuth2Config(t *testing.T) {
	cfg := oauth.ProviderConfig{
		Provider:  "google",
		ClientID:  "client-id",
		AuthURL:   "https://accounts.google.com/o/oauth2/auth",
		TokenURL:  "https://oauth2.googleapis.com/token",
		AuthStyle: oauth2.AuthStyleInParams,
		Scopes:    []string{"calendar.readonly"},
	}.OAuth2Config("http://127.0.0.1:8080/oauth/callback")

	if cfg.ClientID != "client-id" {
		t.Fatalf("client id: %q", cfg.ClientID)
	}
	if cfg.RedirectURL != "http://127.0.0.1:8080/oauth/callback" {
		t.Fatalf("redirect: %q", cfg.RedirectURL)
	}
	if len(cfg.Scopes) != 1 || cfg.Scopes[0] != "calendar.readonly" {
		t.Fatalf("scopes: %#v", cfg.Scopes)
	}
	if cfg.Endpoint.AuthStyle != oauth2.AuthStyleInParams {
		t.Fatalf("auth style: %v", cfg.Endpoint.AuthStyle)
	}
}

func TestFlowAuthorizeExchangesCodeWithoutClientSecret(t *testing.T) {
	var form url.Values
	tokenServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		if err := r.ParseForm(); err != nil {
			t.Fatalf("parse form: %v", err)
		}
		form = cloneValues(r.PostForm)
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"access_token":  "access",
			"refresh_token": "refresh",
			"token_type":    "Bearer",
			"expires_in":    3600,
		})
	}))
	defer tokenServer.Close()

	store := secrets.NewMemoryStore()
	flow := oauth.Flow{
		Config: oauth.ProviderConfig{
			Provider:  "google",
			ClientID:  "client-id",
			AuthURL:   "https://accounts.google.com/o/oauth2/v2/auth",
			TokenURL:  tokenServer.URL,
			AuthStyle: oauth2.AuthStyleInParams,
			Scopes:    []string{"calendar.readonly"},
		},
		Store: store,
		OpenURL: func(rawURL string) error {
			authURL, err := url.Parse(rawURL)
			if err != nil {
				return err
			}
			q := authURL.Query()
			if q.Get("client_secret") != "" {
				t.Fatalf("auth request included client_secret: %q", q.Get("client_secret"))
			}
			if q.Get("client_id") != "client-id" {
				t.Fatalf("auth client id: %q", q.Get("client_id"))
			}
			if q.Get("code_challenge_method") != "S256" {
				t.Fatalf("code challenge method: %q", q.Get("code_challenge_method"))
			}
			if q.Get("code_challenge") == "" {
				t.Fatal("missing code challenge")
			}

			redirectURL, err := url.Parse(q.Get("redirect_uri"))
			if err != nil {
				return err
			}
			params := redirectURL.Query()
			params.Set("code", "auth-code")
			params.Set("state", q.Get("state"))
			redirectURL.RawQuery = params.Encode()

			resp, err := http.Get(redirectURL.String())
			if err != nil {
				return err
			}
			_, _ = io.Copy(io.Discard, resp.Body)
			return resp.Body.Close()
		},
	}

	result, err := flow.Authorize(t.Context(), "user@example.com")
	if err != nil {
		t.Fatal(err)
	}
	if result.Token.AccessToken != "access" || result.Token.RefreshToken != "refresh" {
		t.Fatalf("token: %+v", result.Token)
	}
	if form.Get("client_id") != "client-id" {
		t.Fatalf("exchange client id: %q", form.Get("client_id"))
	}
	if form.Get("client_secret") != "" {
		t.Fatalf("exchange included client_secret: %q", form.Get("client_secret"))
	}
	if form.Get("code") != "auth-code" {
		t.Fatalf("exchange code: %q", form.Get("code"))
	}
	if form.Get("code_verifier") == "" {
		t.Fatal("missing code verifier")
	}
	if form.Get("redirect_uri") == "" {
		t.Fatal("missing redirect uri")
	}

	stored, err := store.Get("google", "user@example.com")
	if err != nil {
		t.Fatal(err)
	}
	if stored.RefreshToken != "refresh" {
		t.Fatalf("stored token: %+v", stored)
	}
}

func cloneValues(values url.Values) url.Values {
	out := make(url.Values, len(values))
	for key, value := range values {
		out[key] = append([]string(nil), value...)
	}
	return out
}
