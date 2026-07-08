package oauth_test

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

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

func TestFlowAuthorizeExchangesCodeWithOptionalClientSecret(t *testing.T) {
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

	callbackBody := make(chan string, 1)
	callbackErr := make(chan error, 1)
	store := secrets.NewMemoryStore()
	flow := oauth.Flow{
		Config: oauth.ProviderConfig{
			Provider:     "google",
			ClientID:     "client-id",
			ClientSecret: "client-secret",
			AuthURL:      "https://accounts.google.com/o/oauth2/v2/auth",
			TokenURL:     tokenServer.URL,
			AuthStyle:    oauth2.AuthStyleInParams,
			Scopes:       []string{"calendar.readonly"},
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

			go fetchCallbackPage(redirectURL.String(), callbackBody, callbackErr)
			return nil
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
	if form.Get("client_secret") != "client-secret" {
		t.Fatalf("exchange client secret: %q", form.Get("client_secret"))
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

	body := awaitCallbackBody(t, callbackBody, callbackErr)
	if !strings.Contains(body, "Authorization complete") {
		t.Fatalf("callback body: %q", body)
	}
}

func TestFlowAuthorizeShowsExchangeFailureOnCallbackPage(t *testing.T) {
	testCases := []struct {
		name        string
		code        string
		description string
	}{
		{
			name:        "invalid client",
			code:        "invalid_client",
			description: "Unauthorized",
		},
		{
			name:        "web client missing secret",
			code:        "invalid_request",
			description: "client_secret is missing.",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			runExchangeFailureCallbackTest(t, tc.code, tc.description)
		})
	}
}

func runExchangeFailureCallbackTest(t *testing.T, code, description string) {
	t.Helper()
	tokenServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		_ = json.NewEncoder(w).Encode(map[string]string{
			"error":             code,
			"error_description": description,
		})
	}))
	defer tokenServer.Close()

	callbackBody := make(chan string, 1)
	callbackErr := make(chan error, 1)
	flow := oauth.Flow{
		Config: oauth.ProviderConfig{
			Provider:  "google",
			ClientID:  "web-client-id",
			AuthURL:   "https://accounts.google.com/o/oauth2/v2/auth",
			TokenURL:  tokenServer.URL,
			AuthStyle: oauth2.AuthStyleInParams,
			Scopes:    []string{"calendar.readonly"},
		},
		Store: secrets.NewMemoryStore(),
		OpenURL: func(rawURL string) error {
			authURL, err := url.Parse(rawURL)
			if err != nil {
				return err
			}
			q := authURL.Query()
			redirectURL, err := url.Parse(q.Get("redirect_uri"))
			if err != nil {
				return err
			}
			params := redirectURL.Query()
			params.Set("code", "auth-code")
			params.Set("state", q.Get("state"))
			redirectURL.RawQuery = params.Encode()

			go fetchCallbackPage(redirectURL.String(), callbackBody, callbackErr)
			return nil
		},
	}

	_, err := flow.Authorize(t.Context(), "user@example.com")
	if err == nil {
		t.Fatal("expected exchange error")
	}
	if !strings.Contains(err.Error(), "CLOCKR_GOOGLE_CLIENT_SECRET") {
		t.Fatalf("error should mention client secret configuration: %v", err)
	}

	body := awaitCallbackBody(t, callbackBody, callbackErr)
	if !strings.Contains(body, "CLOCKR_GOOGLE_CLIENT_SECRET") {
		t.Fatalf("callback body should mention client secret configuration: %q", body)
	}
}

func fetchCallbackPage(rawURL string, bodyCh chan<- string, errCh chan<- error) {
	resp, err := http.Get(rawURL)
	if err != nil {
		errCh <- err
		return
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		errCh <- err
		return
	}
	bodyCh <- string(body)
}

func awaitCallbackBody(t *testing.T, bodyCh <-chan string, errCh <-chan error) string {
	t.Helper()
	select {
	case body := <-bodyCh:
		return body
	case err := <-errCh:
		t.Fatal(err)
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for callback page")
	}
	return ""
}

func cloneValues(values url.Values) url.Values {
	out := make(url.Values, len(values))
	for key, value := range values {
		out[key] = append([]string(nil), value...)
	}
	return out
}
