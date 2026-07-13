package bitbucket_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/dylanbr0wn/shiet/internal/db"
	"github.com/dylanbr0wn/shiet/internal/db/sqlc"
	"github.com/dylanbr0wn/shiet/internal/integration/bitbucket"
	"github.com/dylanbr0wn/shiet/internal/integration/connection"
	"github.com/dylanbr0wn/shiet/internal/integration/secrets"
	"github.com/dylanbr0wn/shiet/internal/service"
)

func newProviderEnv(t *testing.T, handler http.Handler) (*bitbucket.Provider, *connection.Registry, *sqlc.Queries) {
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

	return &bitbucket.Provider{
		Config:   bitbucket.OAuthConfig("client", "secret"),
		Store:    store,
		Registry: reg,
		Queries:  q,
		BaseURL:  server.URL,
		HTTP:     server.Client(),
		AuthMode: "local",
	}, reg, q
}

func seedSelectedRepo(t *testing.T, q *sqlc.Queries, accountID, fullName, workspaceUUID string) {
	t.Helper()
	ctx := context.Background()
	parts := strings.Split(fullName, "/")
	name := parts[len(parts)-1]
	repo, err := q.UpsertBitbucketRepo(ctx, sqlc.UpsertBitbucketRepoParams{
		AccountID:     accountID,
		WorkspaceUuid: workspaceUUID,
		ExternalID:    fullName,
		Name:          name,
		FullName:      fullName,
		Private:       0,
		Column7:       1,
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := q.SetBitbucketRepoSelected(ctx, sqlc.SetBitbucketRepoSelectedParams{
		Selected: 1,
		ID:       repo.ID,
	}); err != nil {
		t.Fatal(err)
	}
}

func TestSyncWorkspacesReposPreservesSelection(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch {
		case strings.HasSuffix(r.URL.Path, "/user"):
			_ = json.NewEncoder(w).Encode(map[string]any{
				"uuid":         "{user-1}",
				"display_name": "Dylan",
				"username":     "dylan",
			})
		case strings.Contains(r.URL.Path, "/workspaces"):
			_ = json.NewEncoder(w).Encode(map[string]any{
				"values": []map[string]any{{
					"uuid": "{ws-1}",
					"slug": "acme",
					"name": "Acme",
				}},
			})
		case strings.Contains(r.URL.Path, "/repositories"):
			_ = json.NewEncoder(w).Encode(map[string]any{
				"values": []map[string]any{{
					"uuid":       "{repo-1}",
					"name":       "app",
					"full_name":  "acme/app",
					"is_private": true,
					"workspace": map[string]any{
						"uuid": "{ws-1}",
						"slug": "acme",
					},
				}},
			})
		default:
			http.NotFound(w, r)
		}
	}))
	t.Cleanup(srv.Close)

	conn, err := db.Open(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = conn.Close() })
	if err := db.Migrate(conn); err != nil {
		t.Fatal(err)
	}

	registry := connection.NewRegistry(conn)
	store := secrets.NewMemoryStore()
	queries := sqlc.New(conn)
	provider := &bitbucket.Provider{
		Config:   bitbucket.OAuthConfig("client", "secret"),
		Store:    store,
		Registry: registry,
		Queries:  queries,
		HTTP:     srv.Client(),
		BaseURL:  srv.URL,
		AuthMode: "local",
	}

	if err := store.Set("bitbucket", "{user-1}", secrets.Token{AccessToken: "token", TokenType: "Bearer"}); err != nil {
		t.Fatal(err)
	}
	if _, err := registry.Upsert(context.Background(), connection.UpsertInput{
		Provider:     "bitbucket",
		AccountLabel: "Dylan",
		AccountID:    "{user-1}",
		Status:       connection.StatusConnected,
	}); err != nil {
		t.Fatal(err)
	}

	workspaces, repos, err := provider.SyncWorkspacesRepos(context.Background(), "{user-1}")
	if err != nil {
		t.Fatal(err)
	}
	if len(workspaces) != 1 || len(repos) != 1 {
		t.Fatalf("sync = %d workspaces, %d repos", len(workspaces), len(repos))
	}

	if err := provider.SetWorkspaceSelected(context.Background(), workspaces[0].ID, true); err != nil {
		t.Fatal(err)
	}
	if err := provider.SetRepoSelected(context.Background(), repos[0].ID, true); err != nil {
		t.Fatal(err)
	}

	workspaces, repos, err = provider.SyncWorkspacesRepos(context.Background(), "{user-1}")
	if err != nil {
		t.Fatal(err)
	}
	if workspaces[0].Selected != 1 || repos[0].Selected != 1 {
		t.Fatalf("selection not preserved: workspace=%d repo=%d", workspaces[0].Selected, repos[0].Selected)
	}
}

func TestFetchEvidence_NoSelectedRepos(t *testing.T) {
	t.Parallel()

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

func TestFetchEvidence_Commits(t *testing.T) {
	t.Parallel()

	start := time.Date(2026, 7, 1, 10, 0, 0, 0, time.UTC)
	end := time.Date(2026, 7, 1, 12, 0, 0, 0, time.UTC)

	p, reg, q := newProviderEnv(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if r.URL.Path != "/repositories/acme/app/commits" {
			http.NotFound(w, r)
			return
		}
		_ = json.NewEncoder(w).Encode(map[string]any{
			"values": []map[string]any{
				{
					"hash":    "abcdef1234567890",
					"date":    "2026-07-01T10:30:00Z",
					"message": "fix auth middleware\n\nFull body stays in detail.",
					"author": map[string]any{
						"user": map[string]any{"uuid": "{user-1}"},
					},
					"links": map[string]any{
						"html": map[string]any{
							"href": "https://bitbucket.org/acme/app/commits/abcdef1234567890",
						},
					},
				},
				{
					"hash":    "outside999",
					"date":    "2026-07-01T13:00:00Z",
					"message": "outside window",
					"author": map[string]any{
						"user": map[string]any{"uuid": "{user-1}"},
					},
				},
				{
					"hash":    "otheruser",
					"date":    "2026-07-01T10:45:00Z",
					"message": "wrong author",
					"author": map[string]any{
						"user": map[string]any{"uuid": "{other-user}"},
					},
				},
			},
		})
	}))

	ctx := context.Background()
	if err := p.Store.Set(service.ProviderBitbucket, "{user-1}", secrets.Token{
		AccessToken: "bb_test",
		TokenType:   "Bearer",
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := reg.Upsert(ctx, connection.UpsertInput{
		Provider:  service.ProviderBitbucket,
		AccountID: "{user-1}",
		Status:    connection.StatusConnected,
	}); err != nil {
		t.Fatal(err)
	}
	seedSelectedRepo(t, q, "{user-1}", "acme/app", "{ws-1}")

	got, err := p.FetchEvidence(ctx, service.TimeWindow{Start: start, End: end})
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 {
		t.Fatalf("expected 1 evidence item, got %#v", got)
	}
	commit := got[0]
	if commit.Provider != service.ProviderBitbucket {
		t.Fatalf("provider: %q", commit.Provider)
	}
	if commit.Kind != "commit" {
		t.Fatalf("kind: %q", commit.Kind)
	}
	if commit.Summary != "acme/app · abcdef1: fix auth middleware" {
		t.Fatalf("summary: %q", commit.Summary)
	}
	if commit.Source != "acme/app" {
		t.Fatalf("source: %q", commit.Source)
	}
	if commit.Detail != "fix auth middleware\n\nFull body stays in detail." {
		t.Fatalf("detail: %q", commit.Detail)
	}
	if commit.URL != "https://bitbucket.org/acme/app/commits/abcdef1234567890" {
		t.Fatalf("url: %q", commit.URL)
	}
	if !commit.Start.Equal(time.Date(2026, 7, 1, 10, 30, 0, 0, time.UTC)) {
		t.Fatalf("start: %v", commit.Start)
	}
}

func TestFetchEvidence_BestEffortSkipsFailingRepo(t *testing.T) {
	t.Parallel()

	start := time.Date(2026, 7, 1, 10, 0, 0, 0, time.UTC)
	end := time.Date(2026, 7, 1, 12, 0, 0, 0, time.UTC)

	p, reg, q := newProviderEnv(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/repositories/acme/bad/commits":
			http.Error(w, `{"error":{"message":"Not Found"}}`, http.StatusNotFound)
		case "/repositories/acme/good/commits":
			_ = json.NewEncoder(w).Encode(map[string]any{
				"values": []map[string]any{
					{
						"hash":    "deadbeef",
						"date":    "2026-07-01T10:15:00Z",
						"message": "good commit",
						"author": map[string]any{
							"user": map[string]any{"uuid": "{user-1}"},
						},
						"links": map[string]any{
							"html": map[string]any{
								"href": "https://bitbucket.org/acme/good/commits/deadbeef",
							},
						},
					},
				},
			})
		default:
			http.NotFound(w, r)
		}
	}))

	ctx := context.Background()
	if err := p.Store.Set(service.ProviderBitbucket, "{user-1}", secrets.Token{
		AccessToken: "bb_test",
		TokenType:   "Bearer",
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := reg.Upsert(ctx, connection.UpsertInput{
		Provider:  service.ProviderBitbucket,
		AccountID: "{user-1}",
		Status:    connection.StatusConnected,
	}); err != nil {
		t.Fatal(err)
	}
	seedSelectedRepo(t, q, "{user-1}", "acme/bad", "{ws-1}")
	seedSelectedRepo(t, q, "{user-1}", "acme/good", "{ws-1}")

	got, err := p.FetchEvidence(ctx, service.TimeWindow{Start: start, End: end})
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 {
		t.Fatalf("expected 1 item from good repo, got %#v", got)
	}
	if got[0].Kind != "commit" || got[0].Summary != "acme/good · deadbee: good commit" {
		t.Fatalf("unexpected item: %+v", got[0])
	}
	if got[0].Source != "acme/good" {
		t.Fatalf("source: %q", got[0].Source)
	}
}

func TestFetchEvidence_WindowFilterExcludesBoundaryEnd(t *testing.T) {
	t.Parallel()

	start := time.Date(2026, 7, 1, 10, 0, 0, 0, time.UTC)
	end := time.Date(2026, 7, 1, 12, 0, 0, 0, time.UTC)

	p, reg, q := newProviderEnv(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if r.URL.Path != "/repositories/acme/app/commits" {
			http.NotFound(w, r)
			return
		}
		_ = json.NewEncoder(w).Encode(map[string]any{
			"values": []map[string]any{
				{
					"hash":    "atstart",
					"date":    "2026-07-01T10:00:00Z",
					"message": "at start",
					"author": map[string]any{
						"user": map[string]any{"uuid": "{user-1}"},
					},
					"links": map[string]any{
						"html": map[string]any{"href": "https://bitbucket.org/acme/app/commits/atstart"},
					},
				},
				{
					"hash":    "atend",
					"date":    "2026-07-01T12:00:00Z",
					"message": "at end exclusive",
					"author": map[string]any{
						"user": map[string]any{"uuid": "{user-1}"},
					},
				},
				{
					"hash":    "beforestart",
					"date":    "2026-07-01T09:00:00Z",
					"message": "before window",
					"author": map[string]any{
						"user": map[string]any{"uuid": "{user-1}"},
					},
				},
			},
		})
	}))

	ctx := context.Background()
	if err := p.Store.Set(service.ProviderBitbucket, "{user-1}", secrets.Token{
		AccessToken: "bb_test",
		TokenType:   "Bearer",
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := reg.Upsert(ctx, connection.UpsertInput{
		Provider:  service.ProviderBitbucket,
		AccountID: "{user-1}",
		Status:    connection.StatusConnected,
	}); err != nil {
		t.Fatal(err)
	}
	seedSelectedRepo(t, q, "{user-1}", "acme/app", "{ws-1}")

	got, err := p.FetchEvidence(ctx, service.TimeWindow{Start: start, End: end})
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 {
		t.Fatalf("expected only start-inclusive commit, got %#v", got)
	}
	if got[0].Summary != "acme/app · atstart: at start" {
		t.Fatalf("unexpected: %+v", got[0])
	}
}
