package github_test

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/dylanbr0wn/shiet/internal/db"
	"github.com/dylanbr0wn/shiet/internal/db/sqlc"
	"github.com/dylanbr0wn/shiet/internal/integration/connection"
	"github.com/dylanbr0wn/shiet/internal/integration/github"
	"github.com/dylanbr0wn/shiet/internal/integration/oauth"
	"github.com/dylanbr0wn/shiet/internal/integration/secrets"
	"github.com/dylanbr0wn/shiet/internal/service"
)

type stubAuthorizer struct {
	result oauth.Result
	err    error
}

type stubRevoker struct {
	token string
}

func (s *stubRevoker) Revoke(_ context.Context, token string) error {
	s.token = token
	return nil
}

func (s stubAuthorizer) Authorize(context.Context, string) (oauth.Result, error) {
	return s.result, s.err
}

func newProviderEnv(t *testing.T, handler http.Handler) (*github.Provider, *connection.Registry, *sqlc.Queries) {
	t.Helper()
	path := t.TempDir() + "/test.db"
	conn, err := db.Open(path)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	t.Cleanup(func() { _ = conn.Close() })
	if err := db.Migrate(conn); err != nil {
		t.Fatalf("migrate: %v", err)
	}

	server := httptest.NewServer(handler)
	t.Cleanup(server.Close)

	store := secrets.NewMemoryStore()
	reg := connection.NewRegistry(conn)
	q := sqlc.New(conn)

	return &github.Provider{
		Store:    store,
		Registry: reg,
		Queries:  q,
		BaseURL:  server.URL,
		HTTP:     server.Client(),
	}, reg, q
}

func newProviderStorageEnv(t *testing.T) (*github.Provider, *connection.Registry) {
	t.Helper()
	path := t.TempDir() + "/test.db"
	conn, err := db.Open(path)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	t.Cleanup(func() { _ = conn.Close() })
	if err := db.Migrate(conn); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	reg := connection.NewRegistry(conn)
	return &github.Provider{Store: secrets.NewMemoryStore(), Registry: reg}, reg
}

func TestConnect_RejectsEmptyPAT(t *testing.T) {
	p, _, _ := newProviderEnv(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.NotFound(w, r)
	}))
	_, err := p.Connect(context.Background(), "  ")
	if err == nil {
		t.Fatal("expected error for empty PAT")
	}
	if !strings.Contains(err.Error(), "personal access token") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestOAuthConfigSupportsLocalBYOCredentials(t *testing.T) {
	cfg := github.OAuthConfig("client-id", "client-secret")
	if cfg.Provider != service.ProviderGitHub || cfg.AuthURL != "https://github.com/login/oauth/authorize" || cfg.TokenURL != "https://github.com/login/oauth/access_token" {
		t.Fatalf("oauth config: %+v", cfg)
	}
	if got := strings.Join(cfg.Scopes, ","); got != "repo" {
		t.Fatalf("scopes: %q", got)
	}
}

func TestOAuthAvailableSeparatesPATOnlyLocalMode(t *testing.T) {
	p := &github.Provider{AuthMode: "local"}
	if p.OAuthAvailable() {
		t.Fatal("PAT-only local mode must not advertise browser OAuth")
	}
	p.Config = github.OAuthConfig("client-id", "client-secret")
	if !p.OAuthAvailable() {
		t.Fatal("BYO credentials should enable local browser OAuth")
	}
	p.AuthMode = "broker"
	p.BrokerBaseURL = "https://auth.shiet.app"
	if !p.OAuthAvailable() {
		t.Fatal("broker URL should enable broker browser OAuth")
	}
}

func TestConnect_BrokerModeStoresOAuthTokenUnderVerifiedGitHubLogin(t *testing.T) {
	p, _, _ := newProviderEnv(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/user":
			_ = json.NewEncoder(w).Encode(map[string]any{"login": "octocat", "name": "The Octocat"})
		case "/user/repos":
			_ = json.NewEncoder(w).Encode([]map[string]any{})
		default:
			http.NotFound(w, r)
		}
	}))
	p.AuthMode = "broker"
	p.Authorizer = stubAuthorizer{result: oauth.Result{
		Provider: service.ProviderGitHub,
		Token:    secrets.Token{AccessToken: "gho_broker", TokenType: "Bearer"},
		Scopes:   []string{"repo", "read:user"},
	}}

	conn, err := p.Connect(context.Background(), "")
	if err != nil {
		t.Fatal(err)
	}
	if conn.AccountID != "octocat" || conn.AccountLabel != "The Octocat" {
		t.Fatalf("connection: %+v", conn)
	}
	token, err := p.Store.Get(service.ProviderGitHub, "octocat")
	if err != nil {
		t.Fatal(err)
	}
	if token.AccessToken != "gho_broker" || token.RefreshToken != "" || token.CredentialSource != secrets.CredentialSourceBroker {
		t.Fatalf("token: %+v", token)
	}
}

func TestConnect_RejectsInvalidPAT(t *testing.T) {
	p, _, _ := newProviderEnv(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/user" {
			http.Error(w, `{"message":"Bad credentials"}`, http.StatusUnauthorized)
			return
		}
		http.NotFound(w, r)
	}))
	_, err := p.Connect(context.Background(), "ghp_bad")
	if err == nil {
		t.Fatal("expected invalid token error")
	}
	if !strings.Contains(err.Error(), "invalid personal access token") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestConnect_SyncFailureRollsBack(t *testing.T) {
	p, reg, _ := newProviderEnv(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/user":
			_ = json.NewEncoder(w).Encode(map[string]any{"login": "octocat", "name": "Octocat"})
		case "/user/repos":
			http.Error(w, `{"message":"Server Error"}`, http.StatusInternalServerError)
		default:
			http.NotFound(w, r)
		}
	}))

	ctx := context.Background()
	_, err := p.Connect(ctx, "ghp_test")
	if err == nil {
		t.Fatal("expected sync failure")
	}
	if _, err := p.Store.Get(service.ProviderGitHub, "octocat"); !errors.Is(err, secrets.ErrNotFound) {
		t.Fatalf("expected token rolled back, got %v", err)
	}
	if _, err := reg.Get(ctx, service.ProviderGitHub, "octocat"); !errors.Is(err, connection.ErrNotFound) {
		t.Fatalf("expected connection rolled back, got %v", err)
	}
}

func TestConnect_StoresTokenSyncsRepos(t *testing.T) {
	p, reg, q := newProviderEnv(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		auth := r.Header.Get("Authorization")
		if !strings.HasPrefix(auth, "Bearer ") {
			http.Error(w, "missing bearer", http.StatusUnauthorized)
			return
		}
		switch r.URL.Path {
		case "/user":
			_ = json.NewEncoder(w).Encode(map[string]any{
				"login": "octocat",
				"name":  "The Octocat",
			})
		case "/user/repos":
			_ = json.NewEncoder(w).Encode([]map[string]any{
				{"name": "Hello-World", "full_name": "octocat/Hello-World", "private": false},
				{"name": "secret", "full_name": "octocat/secret", "private": true},
			})
		default:
			http.NotFound(w, r)
		}
	}))

	ctx := context.Background()
	got, err := p.Connect(ctx, "ghp_test_token")
	if err != nil {
		t.Fatal(err)
	}
	if got.Provider != service.ProviderGitHub {
		t.Fatalf("provider: %q", got.Provider)
	}
	if got.AccountID != "octocat" {
		t.Fatalf("account id: %q", got.AccountID)
	}
	if got.AccountLabel != "The Octocat" {
		t.Fatalf("label: %q", got.AccountLabel)
	}
	if got.Status != connection.StatusConnected {
		t.Fatalf("status: %q", got.Status)
	}

	token, err := p.Store.Get(service.ProviderGitHub, "octocat")
	if err != nil {
		t.Fatal(err)
	}
	if token.AccessToken != "ghp_test_token" {
		t.Fatalf("token: %+v", token)
	}

	stored, err := reg.Get(ctx, service.ProviderGitHub, "octocat")
	if err != nil {
		t.Fatal(err)
	}
	if stored.AccountID != "octocat" {
		t.Fatalf("stored: %+v", stored)
	}

	repos, err := q.ListGitHubRepos(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(repos) != 2 {
		t.Fatalf("repos: %#v", repos)
	}
	if repos[0].FullName != "octocat/Hello-World" || repos[0].Selected != 0 {
		t.Fatalf("first repo: %+v", repos[0])
	}
	if repos[1].FullName != "octocat/secret" || repos[1].Private != 1 {
		t.Fatalf("second repo: %+v", repos[1])
	}
}

func TestSyncRepos_PreservesSelected(t *testing.T) {
	p, _, q := newProviderEnv(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/user":
			_ = json.NewEncoder(w).Encode(map[string]any{"login": "octocat", "name": "Octocat"})
		case "/user/repos":
			_ = json.NewEncoder(w).Encode([]map[string]any{
				{"name": "Hello-World", "full_name": "octocat/Hello-World", "private": false},
			})
		default:
			http.NotFound(w, r)
		}
	}))

	ctx := context.Background()
	if _, err := p.Connect(ctx, "ghp_test"); err != nil {
		t.Fatal(err)
	}
	repos, err := q.ListGitHubRepos(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(repos) != 1 {
		t.Fatalf("repos: %#v", repos)
	}
	if err := p.SetRepoSelected(ctx, repos[0].ID, true); err != nil {
		t.Fatal(err)
	}

	synced, err := p.SyncRepos(ctx, "octocat")
	if err != nil {
		t.Fatal(err)
	}
	if len(synced) != 1 {
		t.Fatalf("synced: %#v", synced)
	}
	if synced[0].Selected != 1 {
		t.Fatalf("selected not preserved: %+v", synced[0])
	}
}

func TestDisconnect_ClearsTokenAndRepos(t *testing.T) {
	p, reg, q := newProviderEnv(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/user":
			_ = json.NewEncoder(w).Encode(map[string]any{"login": "octocat"})
		case "/user/repos":
			_ = json.NewEncoder(w).Encode([]map[string]any{
				{"name": "Hello-World", "full_name": "octocat/Hello-World", "private": false},
			})
		default:
			http.NotFound(w, r)
		}
	}))

	ctx := context.Background()
	if _, err := p.Connect(ctx, "ghp_test"); err != nil {
		t.Fatal(err)
	}
	if err := p.Disconnect(ctx, "octocat"); err != nil {
		t.Fatal(err)
	}

	if _, err := p.Store.Get(service.ProviderGitHub, "octocat"); !errors.Is(err, secrets.ErrNotFound) {
		t.Fatalf("expected token gone, got %v", err)
	}
	if _, err := reg.Get(ctx, service.ProviderGitHub, "octocat"); !errors.Is(err, connection.ErrNotFound) {
		t.Fatalf("expected connection gone, got %v", err)
	}
	repos, err := q.ListGitHubRepos(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(repos) != 0 {
		t.Fatalf("repos should be cleared: %#v", repos)
	}
}

func TestDisconnect_BrokerModeRevokesOAuthToken(t *testing.T) {
	p, reg := newProviderStorageEnv(t)
	ctx := context.Background()
	if err := p.Store.Set(service.ProviderGitHub, "octocat", secrets.Token{AccessToken: "gho_access", CredentialSource: secrets.CredentialSourceBroker}); err != nil {
		t.Fatal(err)
	}
	if _, err := reg.Upsert(ctx, connection.UpsertInput{
		Provider: service.ProviderGitHub, AccountID: "octocat", AccountLabel: "Octocat", Status: connection.StatusConnected,
	}); err != nil {
		t.Fatal(err)
	}
	revoker := &stubRevoker{}
	p.AuthMode = "broker"
	p.Revoker = revoker

	if err := p.Disconnect(ctx, "octocat"); err != nil {
		t.Fatal(err)
	}
	if revoker.token != "gho_access" {
		t.Fatalf("revoked token: got %q", revoker.token)
	}
}

func TestDisconnect_BrokerModeDoesNotSendPATToBrokerRevoke(t *testing.T) {
	p, reg := newProviderStorageEnv(t)
	ctx := context.Background()
	if err := p.Store.Set(service.ProviderGitHub, "octocat", secrets.Token{AccessToken: "github_pat_secret", CredentialSource: secrets.CredentialSourcePAT}); err != nil {
		t.Fatal(err)
	}
	if _, err := reg.Upsert(ctx, connection.UpsertInput{
		Provider: service.ProviderGitHub, AccountID: "octocat", AccountLabel: "Octocat", Status: connection.StatusConnected,
	}); err != nil {
		t.Fatal(err)
	}
	revoker := &stubRevoker{}
	p.AuthMode = "broker"
	p.Revoker = revoker

	if err := p.Disconnect(ctx, "octocat"); err != nil {
		t.Fatal(err)
	}
	if revoker.token != "" {
		t.Fatalf("PAT must not be sent to broker revoke, got %q", revoker.token)
	}
}
