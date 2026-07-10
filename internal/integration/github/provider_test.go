package github_test

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
	"github.com/dylanbr0wn/shiet/internal/integration/github"
	"github.com/dylanbr0wn/shiet/internal/integration/secrets"
	"github.com/dylanbr0wn/shiet/internal/service"
)

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

func seedSelectedRepo(t *testing.T, q *sqlc.Queries, accountID, fullName string) {
	t.Helper()
	ctx := context.Background()
	repo, err := q.UpsertGitHubRepo(ctx, sqlc.UpsertGitHubRepoParams{
		AccountID:  accountID,
		ExternalID: fullName,
		Name:       strings.TrimPrefix(fullName, accountID+"/"),
		FullName:   fullName,
		Private:    0,
		Column6:    1,
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := q.SetGitHubRepoSelected(ctx, sqlc.SetGitHubRepoSelectedParams{
		Selected: 1,
		ID:       repo.ID,
	}); err != nil {
		t.Fatal(err)
	}
}

func TestFetchEvidence_NoSelectedRepos(t *testing.T) {
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

func TestFetchEvidence_CommitsAndPRs(t *testing.T) {
	start := time.Date(2026, 7, 1, 10, 0, 0, 0, time.UTC)
	end := time.Date(2026, 7, 1, 12, 0, 0, 0, time.UTC)

	p, reg, q := newProviderEnv(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.URL.Path == "/repos/octocat/Hello-World/commits":
			_ = json.NewEncoder(w).Encode([]map[string]any{
				{
					"sha":      "abcdef1234567890",
					"html_url": "https://github.com/octocat/Hello-World/commit/abcdef1234567890",
					"commit": map[string]any{
						"message": "fix auth middleware\n\nFull body stays in detail.",
						"author":  map[string]any{"date": "2026-07-01T10:30:00Z"},
					},
				},
				{
					"sha":      "outside999",
					"html_url": "https://github.com/octocat/Hello-World/commit/outside999",
					"commit": map[string]any{
						"message": "outside window",
						"author":  map[string]any{"date": "2026-07-01T13:00:00Z"},
					},
				},
			})
		case r.URL.Path == "/search/issues":
			_ = json.NewEncoder(w).Encode(map[string]any{
				"items": []map[string]any{
					{
						"number":    42,
						"title":     "Ship gap fill",
						"body":      "PR body text",
						"html_url":  "https://github.com/octocat/Hello-World/pull/42",
						"closed_at": "2026-07-01T11:00:00Z",
						"pull_request": map[string]any{
							"merged_at": "2026-07-01T11:00:00Z",
						},
					},
				},
			})
		default:
			http.NotFound(w, r)
		}
	}))

	ctx := context.Background()
	if err := p.Store.Set(service.ProviderGitHub, "octocat", secrets.Token{
		AccessToken: "ghp_test",
		TokenType:   "Bearer",
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := reg.Upsert(ctx, connection.UpsertInput{
		Provider:  service.ProviderGitHub,
		AccountID: "octocat",
		Status:    connection.StatusConnected,
	}); err != nil {
		t.Fatal(err)
	}
	seedSelectedRepo(t, q, "octocat", "octocat/Hello-World")

	got, err := p.FetchEvidence(ctx, service.TimeWindow{Start: start, End: end})
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 {
		t.Fatalf("expected 2 evidence items, got %#v", got)
	}

	var commit, pr *service.ActivityEvidence
	for i := range got {
		switch got[i].Kind {
		case "commit":
			commit = &got[i]
		case "pr":
			pr = &got[i]
		}
	}
	if commit == nil || pr == nil {
		t.Fatalf("missing kinds: %#v", got)
	}

	if commit.Provider != service.ProviderGitHub {
		t.Fatalf("commit provider: %q", commit.Provider)
	}
	if commit.Summary != "abcdef1: fix auth middleware" {
		t.Fatalf("commit summary: %q", commit.Summary)
	}
	if commit.Detail != "fix auth middleware\n\nFull body stays in detail." {
		t.Fatalf("commit detail: %q", commit.Detail)
	}
	if commit.URL != "https://github.com/octocat/Hello-World/commit/abcdef1234567890" {
		t.Fatalf("commit url: %q", commit.URL)
	}
	if !commit.Start.Equal(time.Date(2026, 7, 1, 10, 30, 0, 0, time.UTC)) {
		t.Fatalf("commit start: %v", commit.Start)
	}

	if pr.Summary != "Merged PR #42: Ship gap fill" {
		t.Fatalf("pr summary: %q", pr.Summary)
	}
	if !strings.Contains(pr.Detail, "PR body text") {
		t.Fatalf("pr detail: %q", pr.Detail)
	}
	if pr.URL != "https://github.com/octocat/Hello-World/pull/42" {
		t.Fatalf("pr url: %q", pr.URL)
	}
}

func TestFetchEvidence_BestEffortSkipsFailingRepo(t *testing.T) {
	start := time.Date(2026, 7, 1, 10, 0, 0, 0, time.UTC)
	end := time.Date(2026, 7, 1, 12, 0, 0, 0, time.UTC)

	p, reg, q := newProviderEnv(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/repos/octocat/bad/commits":
			http.Error(w, `{"message":"Not Found"}`, http.StatusNotFound)
		case "/repos/octocat/good/commits":
			_ = json.NewEncoder(w).Encode([]map[string]any{
				{
					"sha":      "deadbeef",
					"html_url": "https://github.com/octocat/good/commit/deadbeef",
					"commit": map[string]any{
						"message": "good commit",
						"author":  map[string]any{"date": "2026-07-01T10:15:00Z"},
					},
				},
			})
		case "/search/issues":
			_ = json.NewEncoder(w).Encode(map[string]any{"items": []any{}})
		default:
			http.NotFound(w, r)
		}
	}))

	ctx := context.Background()
	if err := p.Store.Set(service.ProviderGitHub, "octocat", secrets.Token{
		AccessToken: "ghp_test",
		TokenType:   "Bearer",
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := reg.Upsert(ctx, connection.UpsertInput{
		Provider:  service.ProviderGitHub,
		AccountID: "octocat",
		Status:    connection.StatusConnected,
	}); err != nil {
		t.Fatal(err)
	}
	seedSelectedRepo(t, q, "octocat", "octocat/bad")
	seedSelectedRepo(t, q, "octocat", "octocat/good")

	got, err := p.FetchEvidence(ctx, service.TimeWindow{Start: start, End: end})
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 {
		t.Fatalf("expected 1 item from good repo, got %#v", got)
	}
	if got[0].Kind != "commit" || got[0].Summary != "deadbee: good commit" {
		t.Fatalf("unexpected item: %+v", got[0])
	}
}

func TestFetchEvidence_WindowFilterExcludesBoundaryEnd(t *testing.T) {
	start := time.Date(2026, 7, 1, 10, 0, 0, 0, time.UTC)
	end := time.Date(2026, 7, 1, 12, 0, 0, 0, time.UTC)

	p, reg, q := newProviderEnv(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/repos/octocat/Hello-World/commits":
			_ = json.NewEncoder(w).Encode([]map[string]any{
				{
					"sha":      "atstart",
					"html_url": "https://github.com/octocat/Hello-World/commit/atstart",
					"commit": map[string]any{
						"message": "at start",
						"author":  map[string]any{"date": "2026-07-01T10:00:00Z"},
					},
				},
				{
					"sha":      "atend",
					"html_url": "https://github.com/octocat/Hello-World/commit/atend",
					"commit": map[string]any{
						"message": "at end exclusive",
						"author":  map[string]any{"date": "2026-07-01T12:00:00Z"},
					},
				},
			})
		case "/search/issues":
			_ = json.NewEncoder(w).Encode(map[string]any{"items": []any{}})
		default:
			http.NotFound(w, r)
		}
	}))

	ctx := context.Background()
	if err := p.Store.Set(service.ProviderGitHub, "octocat", secrets.Token{
		AccessToken: "ghp_test",
		TokenType:   "Bearer",
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := reg.Upsert(ctx, connection.UpsertInput{
		Provider:  service.ProviderGitHub,
		AccountID: "octocat",
		Status:    connection.StatusConnected,
	}); err != nil {
		t.Fatal(err)
	}
	seedSelectedRepo(t, q, "octocat", "octocat/Hello-World")

	got, err := p.FetchEvidence(ctx, service.TimeWindow{Start: start, End: end})
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 {
		t.Fatalf("expected only start-inclusive commit, got %#v", got)
	}
	if got[0].Summary != "atstart: at start" {
		t.Fatalf("unexpected: %+v", got[0])
	}
}
