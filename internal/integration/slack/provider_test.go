package slack_test

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
	"github.com/dylanbr0wn/shiet/internal/integration/oauth"
	"github.com/dylanbr0wn/shiet/internal/integration/secrets"
	"github.com/dylanbr0wn/shiet/internal/integration/slack"
	"github.com/dylanbr0wn/shiet/internal/service"
)

type stubAuthorizer struct {
	result oauth.Result
	err    error
}

func (s stubAuthorizer) Authorize(context.Context, string) (oauth.Result, error) {
	return s.result, s.err
}

func newProviderEnv(t *testing.T, handler http.Handler) (*slack.Provider, *connection.Registry, *sqlc.Queries) {
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

	return &slack.Provider{
		Store:    store,
		Registry: reg,
		Queries:  q,
		BaseURL:  server.URL,
		HTTP:     server.Client(),
	}, reg, q
}

func TestOAuthConfigUsesSlackUserScopes(t *testing.T) {
	cfg := slack.OAuthConfig("client-id", "client-secret")
	if cfg.Provider != service.ProviderSlack {
		t.Fatalf("provider: %q", cfg.Provider)
	}
	got := strings.Join(cfg.Scopes, ",")
	want := "channels:history,groups:history,channels:read,groups:read"
	if got != want {
		t.Fatalf("scopes: %q", got)
	}
}

func TestConnect_BrokerModeStoresOAuthTokenUnderTeamID(t *testing.T) {
	p, _, _ := newProviderEnv(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/auth.test":
			_ = json.NewEncoder(w).Encode(map[string]any{
				"ok":      true,
				"team":    "Acme",
				"team_id": "T123",
			})
		case "/users.conversations":
			_ = json.NewEncoder(w).Encode(map[string]any{
				"ok":       true,
				"channels": []map[string]any{},
			})
		default:
			http.NotFound(w, r)
		}
	}))
	p.AuthMode = "broker"
	p.Authorizer = stubAuthorizer{result: oauth.Result{
		Provider: service.ProviderSlack,
		Token:    secrets.Token{AccessToken: "xoxp-test", TokenType: "Bearer"},
		Scopes:   []string{"channels:history", "channels:read"},
	}}

	conn, err := p.Connect(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if conn.AccountID != "T123" || conn.AccountLabel != "Acme" {
		t.Fatalf("connection: %+v", conn)
	}
	token, err := p.Store.Get(service.ProviderSlack, "T123")
	if err != nil {
		t.Fatal(err)
	}
	if token.AccessToken != "xoxp-test" || token.CredentialSource != secrets.CredentialSourceBroker {
		t.Fatalf("token: %+v", token)
	}
}

func TestConnect_SyncsChannels(t *testing.T) {
	p, _, q := newProviderEnv(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/auth.test":
			_ = json.NewEncoder(w).Encode(map[string]any{
				"ok":      true,
				"team":    "Acme",
				"team_id": "T123",
			})
		case "/users.conversations":
			_ = json.NewEncoder(w).Encode(map[string]any{
				"ok": true,
				"channels": []map[string]any{
					{"id": "C1", "name": "general", "is_private": false},
					{"id": "C2", "name": "secret", "is_private": true},
				},
			})
		default:
			http.NotFound(w, r)
		}
	}))
	p.AuthMode = "broker"
	p.Authorizer = stubAuthorizer{result: oauth.Result{
		Provider: service.ProviderSlack,
		Token:    secrets.Token{AccessToken: "xoxp-test", TokenType: "Bearer"},
	}}

	ctx := context.Background()
	if _, err := p.Connect(ctx); err != nil {
		t.Fatal(err)
	}
	channels, err := q.ListSlackChannels(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(channels) != 2 {
		t.Fatalf("channels: %#v", channels)
	}
	if channels[0].Name != "general" || channels[0].Selected != 0 {
		t.Fatalf("first channel: %+v", channels[0])
	}
	if channels[1].Name != "secret" || channels[1].IsPrivate != 1 {
		t.Fatalf("second channel: %+v", channels[1])
	}
}

func TestSyncChannels_PreservesSelected(t *testing.T) {
	p, _, q := newProviderEnv(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/auth.test":
			_ = json.NewEncoder(w).Encode(map[string]any{
				"ok": true, "team": "Acme", "team_id": "T123",
			})
		case "/users.conversations":
			_ = json.NewEncoder(w).Encode(map[string]any{
				"ok": true,
				"channels": []map[string]any{
					{"id": "C1", "name": "general", "is_private": false},
				},
			})
		default:
			http.NotFound(w, r)
		}
	}))
	p.AuthMode = "broker"
	p.Authorizer = stubAuthorizer{result: oauth.Result{
		Provider: service.ProviderSlack,
		Token:    secrets.Token{AccessToken: "xoxp-test", TokenType: "Bearer"},
	}}

	ctx := context.Background()
	if _, err := p.Connect(ctx); err != nil {
		t.Fatal(err)
	}
	channels, err := q.ListSlackChannels(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if err := p.SetChannelSelected(ctx, channels[0].ID, true); err != nil {
		t.Fatal(err)
	}
	synced, err := p.SyncChannels(ctx, "T123")
	if err != nil {
		t.Fatal(err)
	}
	if len(synced) != 1 || synced[0].Selected != 1 {
		t.Fatalf("selected not preserved: %+v", synced)
	}
}

func TestConnect_RejectsWhenOAuthUnavailable(t *testing.T) {
	p := &slack.Provider{AuthMode: "local"}
	_, err := p.Connect(context.Background())
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "not configured") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestDisconnect_ClearsTokenAndChannels(t *testing.T) {
	p, reg, q := newProviderEnv(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/auth.test":
			_ = json.NewEncoder(w).Encode(map[string]any{
				"ok": true, "team": "Acme", "team_id": "T123",
			})
		case "/users.conversations":
			_ = json.NewEncoder(w).Encode(map[string]any{
				"ok": true,
				"channels": []map[string]any{
					{"id": "C1", "name": "general", "is_private": false},
				},
			})
		default:
			http.NotFound(w, r)
		}
	}))
	p.AuthMode = "broker"
	p.Authorizer = stubAuthorizer{result: oauth.Result{
		Provider: service.ProviderSlack,
		Token:    secrets.Token{AccessToken: "xoxp-test", TokenType: "Bearer"},
	}}

	ctx := context.Background()
	if _, err := p.Connect(ctx); err != nil {
		t.Fatal(err)
	}
	if err := p.Disconnect(ctx, "T123"); err != nil {
		t.Fatal(err)
	}
	if _, err := p.Store.Get(service.ProviderSlack, "T123"); !errors.Is(err, secrets.ErrNotFound) {
		t.Fatalf("expected token deleted, got %v", err)
	}
	if _, err := reg.Get(ctx, service.ProviderSlack, "T123"); !errors.Is(err, connection.ErrNotFound) {
		t.Fatalf("expected connection removed, got %v", err)
	}
	channels, err := q.ListSlackChannels(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(channels) != 0 {
		t.Fatalf("expected channels cleared, got %#v", channels)
	}
}
