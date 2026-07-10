package slack_test

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

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

func seedSelectedChannel(t *testing.T, q *sqlc.Queries, accountID, externalID, name string) {
	t.Helper()
	ctx := context.Background()
	ch, err := q.UpsertSlackChannel(ctx, sqlc.UpsertSlackChannelParams{
		AccountID:  accountID,
		ExternalID: externalID,
		Name:       name,
		IsPrivate:  0,
		Column5:    0,
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := q.SetSlackChannelSelected(ctx, sqlc.SetSlackChannelSelectedParams{
		Selected: 1,
		ID:       ch.ID,
	}); err != nil {
		t.Fatal(err)
	}
}

func TestFetchEvidence_NoSelectedChannels(t *testing.T) {
	p, _, _ := newProviderEnv(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Fatalf("unexpected request: %s", r.URL.Path)
	}))

	got, err := p.FetchEvidence(context.Background(), service.TimeWindow{
		Start: time.Date(2026, 7, 1, 10, 0, 0, 0, time.UTC),
		End:   time.Date(2026, 7, 1, 12, 0, 0, 0, time.UTC),
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 0 {
		t.Fatalf("expected empty, got %#v", got)
	}
}

func TestFetchEvidence_MessagesInWindow(t *testing.T) {
	start := time.Date(2026, 7, 1, 10, 0, 0, 0, time.UTC)
	end := time.Date(2026, 7, 1, 12, 0, 0, 0, time.UTC)

	p, reg, q := newProviderEnv(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/conversations.history" {
			http.NotFound(w, r)
			return
		}
		if r.URL.Query().Get("channel") != "C1" {
			t.Fatalf("unexpected channel: %q", r.URL.Query().Get("channel"))
		}
		_ = json.NewEncoder(w).Encode(map[string]any{
			"ok": true,
			"messages": []map[string]any{
				{
					"type": "message",
					"user": "U1",
					"text": "standup notes\n\nFull thread body stays in detail.",
					"ts":   "1782901800.000100", // 2026-07-01T10:30:00Z
				},
				{
					"type":    "message",
					"subtype": "channel_join",
					"user":    "U1",
					"text":    "joined",
					"ts":      "1782901801.000100",
				},
				{
					"type":   "message",
					"bot_id": "B1",
					"text":   "bot noise",
					"ts":     "1782901802.000100",
				},
				{
					"type": "message",
					"user": "U2",
					"text": "outside window",
					"ts":   "1782910800.000100", // 2026-07-01T13:00:00Z
				},
			},
		})
	}))

	ctx := context.Background()
	if err := p.Store.Set(service.ProviderSlack, "T123", secrets.Token{
		AccessToken: "xoxp-test",
		TokenType:   "Bearer",
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := reg.Upsert(ctx, connection.UpsertInput{
		Provider:  service.ProviderSlack,
		AccountID: "T123",
		Status:    connection.StatusConnected,
	}); err != nil {
		t.Fatal(err)
	}
	seedSelectedChannel(t, q, "T123", "C1", "general")

	got, err := p.FetchEvidence(ctx, service.TimeWindow{Start: start, End: end})
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 {
		t.Fatalf("expected 1 message, got %#v", got)
	}
	ev := got[0]
	if ev.Provider != service.ProviderSlack {
		t.Fatalf("provider: %q", ev.Provider)
	}
	if ev.Kind != "message" {
		t.Fatalf("kind: %q", ev.Kind)
	}
	if ev.Summary != "general · standup notes" {
		t.Fatalf("summary: %q", ev.Summary)
	}
	if ev.Source != "general" {
		t.Fatalf("source: %q", ev.Source)
	}
	if ev.Detail != "standup notes\n\nFull thread body stays in detail." {
		t.Fatalf("detail: %q", ev.Detail)
	}
	if ev.URL != "https://slack.com/archives/C1/p1782901800000100" {
		t.Fatalf("url: %q", ev.URL)
	}
	wantStart := time.Unix(1782901800, 100000).UTC()
	if !ev.Start.Equal(wantStart) {
		t.Fatalf("start: got %v want %v", ev.Start, wantStart)
	}
}

func TestFetchEvidence_BestEffortSkipsFailingChannel(t *testing.T) {
	start := time.Date(2026, 7, 1, 10, 0, 0, 0, time.UTC)
	end := time.Date(2026, 7, 1, 12, 0, 0, 0, time.UTC)

	p, reg, q := newProviderEnv(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/conversations.history" {
			http.NotFound(w, r)
			return
		}
		switch r.URL.Query().Get("channel") {
		case "CBAD":
			_ = json.NewEncoder(w).Encode(map[string]any{
				"ok":    false,
				"error": "channel_not_found",
			})
		case "CGOOD":
			_ = json.NewEncoder(w).Encode(map[string]any{
				"ok": true,
				"messages": []map[string]any{
					{
						"type": "message",
						"user": "U1",
						"text": "good message",
						"ts":   "1782900900.000000", // 2026-07-01T10:15:00Z
					},
				},
			})
		default:
			http.NotFound(w, r)
		}
	}))

	ctx := context.Background()
	if err := p.Store.Set(service.ProviderSlack, "T123", secrets.Token{
		AccessToken: "xoxp-test",
		TokenType:   "Bearer",
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := reg.Upsert(ctx, connection.UpsertInput{
		Provider:  service.ProviderSlack,
		AccountID: "T123",
		Status:    connection.StatusConnected,
	}); err != nil {
		t.Fatal(err)
	}
	seedSelectedChannel(t, q, "T123", "CBAD", "bad")
	seedSelectedChannel(t, q, "T123", "CGOOD", "good")

	got, err := p.FetchEvidence(ctx, service.TimeWindow{Start: start, End: end})
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 {
		t.Fatalf("expected 1 item from good channel, got %#v", got)
	}
	if got[0].Kind != "message" || got[0].Summary != "good · good message" {
		t.Fatalf("unexpected item: %+v", got[0])
	}
	if got[0].Source != "good" {
		t.Fatalf("source: %q", got[0].Source)
	}
}

func TestFetchEvidence_WindowFilterExcludesBoundaryEnd(t *testing.T) {
	start := time.Date(2026, 7, 1, 10, 0, 0, 0, time.UTC)
	end := time.Date(2026, 7, 1, 12, 0, 0, 0, time.UTC)

	p, reg, q := newProviderEnv(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/conversations.history" {
			http.NotFound(w, r)
			return
		}
		_ = json.NewEncoder(w).Encode(map[string]any{
			"ok": true,
			"messages": []map[string]any{
				{
					"type": "message",
					"user": "U1",
					"text": "at start",
					"ts":   "1782900000.000000", // 2026-07-01T10:00:00Z
				},
				{
					"type": "message",
					"user": "U1",
					"text": "at end exclusive",
					"ts":   "1782907200.000000", // 2026-07-01T12:00:00Z
				},
			},
		})
	}))

	ctx := context.Background()
	if err := p.Store.Set(service.ProviderSlack, "T123", secrets.Token{
		AccessToken: "xoxp-test",
		TokenType:   "Bearer",
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := reg.Upsert(ctx, connection.UpsertInput{
		Provider:  service.ProviderSlack,
		AccountID: "T123",
		Status:    connection.StatusConnected,
	}); err != nil {
		t.Fatal(err)
	}
	seedSelectedChannel(t, q, "T123", "C1", "general")

	got, err := p.FetchEvidence(ctx, service.TimeWindow{Start: start, End: end})
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 {
		t.Fatalf("expected only start-inclusive message, got %#v", got)
	}
	if got[0].Summary != "general · at start" {
		t.Fatalf("unexpected: %+v", got[0])
	}
}
