package google_test

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/dylanbr0wn/shiet/internal/config"
	"github.com/dylanbr0wn/shiet/internal/db"
	"github.com/dylanbr0wn/shiet/internal/db/sqlc"
	"github.com/dylanbr0wn/shiet/internal/integration/connection"
	"github.com/dylanbr0wn/shiet/internal/integration/google"
	"github.com/dylanbr0wn/shiet/internal/integration/oauth"
	"github.com/dylanbr0wn/shiet/internal/integration/secrets"
	"github.com/dylanbr0wn/shiet/internal/service"
	"golang.org/x/oauth2"
)

func TestOAuthConfig(t *testing.T) {
	cfg := google.OAuthConfig("client-id", "client-secret")
	if cfg.Provider != service.ProviderGoogle {
		t.Fatalf("provider: %q", cfg.Provider)
	}
	if cfg.ClientID != "client-id" {
		t.Fatalf("client id: %q", cfg.ClientID)
	}
	if cfg.ClientSecret != "client-secret" {
		t.Fatalf("client secret: %q", cfg.ClientSecret)
	}
	if cfg.AuthURL != "https://accounts.google.com/o/oauth2/v2/auth" {
		t.Fatalf("auth url: %q", cfg.AuthURL)
	}
	if cfg.TokenURL != "https://oauth2.googleapis.com/token" {
		t.Fatalf("token url: %q", cfg.TokenURL)
	}
	if cfg.AuthStyle != oauth2.AuthStyleInParams {
		t.Fatalf("auth style: %v", cfg.AuthStyle)
	}
	if len(cfg.Scopes) != 1 || cfg.Scopes[0] != "https://www.googleapis.com/auth/calendar.readonly" {
		t.Fatalf("scopes: %#v", cfg.Scopes)
	}
}

func TestAuthSettingsFromConfig_brokerOmitsClientSecret(t *testing.T) {
	var cfg config.Config
	cfg.Google.AuthMode = config.AuthModeBroker
	cfg.Google.BrokerBaseURL = "https://auth.shiet.app"
	cfg.Google.ClientID = "should-not-matter"
	cfg.Google.ClientSecret = "must-not-copy"

	got := google.AuthSettingsFromConfig(cfg)
	if got.Mode != config.AuthModeBroker {
		t.Fatalf("mode: %q", got.Mode)
	}
	if got.BrokerBaseURL != "https://auth.shiet.app" {
		t.Fatalf("broker url: %q", got.BrokerBaseURL)
	}
	if got.ClientSecret != "" {
		t.Fatalf("broker mode must not copy client_secret, got %q", got.ClientSecret)
	}
}

func TestAuthSettingsFromConfig_localKeepsCredentials(t *testing.T) {
	var cfg config.Config
	cfg.Google.AuthMode = config.AuthModeLocal
	cfg.Google.ClientID = "desktop-id"
	cfg.Google.ClientSecret = "desktop-secret"

	got := google.AuthSettingsFromConfig(cfg)
	if got.Mode != config.AuthModeLocal {
		t.Fatalf("mode: %q", got.Mode)
	}
	if got.ClientID != "desktop-id" || got.ClientSecret != "desktop-secret" {
		t.Fatalf("local credentials: %+v", got)
	}
}

func TestConnect_brokerModeReportsUnavailable(t *testing.T) {
	p, _, _ := newProviderEnv(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.NotFound(w, r)
	}))
	p.AuthMode = config.AuthModeBroker
	p.BrokerBaseURL = "https://127.0.0.1:1" // nothing listening
	p.Config.ClientSecret = "must-not-be-required"
	p.Authorizer = nil

	_, err := p.Connect(context.Background(), "user@example.com", "Work")
	if err == nil {
		t.Fatal("expected broker unavailable error")
	}
	if !errors.Is(err, google.ErrBrokerUnavailable) {
		t.Fatalf("want ErrBrokerUnavailable, got %v", err)
	}
	if strings.Contains(err.Error(), "client_secret") || strings.Contains(err.Error(), "client_id") {
		t.Fatalf("broker error must not look like local credential gap: %v", err)
	}
}

func TestConnect_brokerModeSuccessStoresTokenAndRefreshesCalendars(t *testing.T) {
	p, reg, q := newProviderEnv(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/users/me/calendarList" {
			_ = json.NewEncoder(w).Encode(map[string]any{
				"items": []map[string]any{
					{"id": "primary", "summary": "Primary", "primary": true},
				},
			})
			return
		}
		http.NotFound(w, r)
	}))
	p.AuthMode = config.AuthModeBroker
	p.BrokerBaseURL = "https://auth.example"
	p.Config.ClientID = ""
	p.Config.ClientSecret = ""
	p.Authorizer = stubAuthorizer{
		result: oauth.Result{
			Provider:  service.ProviderGoogle,
			AccountID: "user@example.com",
			Token: secrets.Token{
				AccessToken:  "broker-access",
				RefreshToken: "broker-refresh",
				TokenType:    "Bearer",
			},
			Scopes: []string{"https://www.googleapis.com/auth/calendar.readonly"},
		},
	}

	got, err := p.Connect(context.Background(), "user@example.com", "Work")
	if err != nil {
		t.Fatal(err)
	}
	if got.Status != connection.StatusConnected {
		t.Fatalf("status: %q", got.Status)
	}
	token, err := p.Store.Get(service.ProviderGoogle, "user@example.com")
	if err != nil {
		t.Fatal(err)
	}
	if token.AccessToken != "broker-access" || token.RefreshToken != "broker-refresh" {
		t.Fatalf("token: %+v", token)
	}
	stored, err := reg.Get(context.Background(), service.ProviderGoogle, "user@example.com")
	if err != nil {
		t.Fatal(err)
	}
	if stored.AccountLabel != "Work" {
		t.Fatalf("label: %q", stored.AccountLabel)
	}
	cal, err := q.GetCalendarByProviderExternalID(context.Background(), sqlc.GetCalendarByProviderExternalIDParams{
		Provider: service.ProviderGoogle, ExternalID: "primary",
	})
	if err != nil {
		t.Fatal(err)
	}
	if cal.Name != "Primary" {
		t.Fatalf("calendar: %+v", cal)
	}
}

func TestConnect_localModeMissingClientID(t *testing.T) {
	p, _, _ := newProviderEnv(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.NotFound(w, r)
	}))
	p.AuthMode = config.AuthModeLocal
	p.Config.ClientID = ""

	_, err := p.Connect(context.Background(), "user@example.com", "Work")
	if err == nil {
		t.Fatal("expected local credentials error")
	}
	if !errors.Is(err, config.ErrLocalCredentials) {
		t.Fatalf("want ErrLocalCredentials, got %v", err)
	}
}

type stubAuthorizer struct {
	result oauth.Result
	err    error
}

func (s stubAuthorizer) Authorize(ctx context.Context, accountID string) (oauth.Result, error) {
	if s.err != nil {
		return oauth.Result{}, s.err
	}
	return s.result, nil
}

func newProviderEnv(t *testing.T, handler http.Handler) (*google.Provider, *connection.Registry, *sqlc.Queries) {
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
	cfg := google.OAuthConfig("client-id", "client-secret")
	reg := connection.NewRegistry(conn)
	q := sqlc.New(conn)

	return &google.Provider{
		Config:   cfg,
		AuthMode: config.AuthModeLocal,
		Store:    store,
		Registry: reg,
		Queries:  q,
		BaseURL:  server.URL,
	}, reg, q
}

func TestConnectUpsertsConnection(t *testing.T) {
	p, reg, _ := newProviderEnv(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/users/me/calendarList" {
			_ = json.NewEncoder(w).Encode(map[string]any{"items": []any{}})
			return
		}
		http.NotFound(w, r)
	}))
	p.Authorizer = stubAuthorizer{
		result: oauth.Result{
			Provider:  service.ProviderGoogle,
			AccountID: "user@example.com",
			Token:     secrets.Token{AccessToken: "access"},
			Scopes:    []string{"https://www.googleapis.com/auth/calendar.readonly"},
		},
	}

	ctx := context.Background()
	got, err := p.Connect(ctx, "user@example.com", "Work Google")
	if err != nil {
		t.Fatal(err)
	}
	if got.Status != connection.StatusConnected {
		t.Fatalf("status: %q", got.Status)
	}
	if got.AccountLabel != "Work Google" {
		t.Fatalf("label: %q", got.AccountLabel)
	}

	stored, err := reg.Get(ctx, service.ProviderGoogle, "user@example.com")
	if err != nil {
		t.Fatal(err)
	}
	if stored.AccountLabel != "Work Google" {
		t.Fatalf("stored label: %q", stored.AccountLabel)
	}

	token, err := p.Store.Get(service.ProviderGoogle, "user@example.com")
	if err != nil {
		t.Fatal(err)
	}
	if token.AccessToken != "access" {
		t.Fatalf("token: %+v", token)
	}
}

func TestSyncCalendarsPaginatesAndUpserts(t *testing.T) {
	var calls int
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/users/me/calendarList" {
			http.NotFound(w, r)
			return
		}
		calls++
		if r.URL.Query().Get("pageToken") == "" {
			_ = json.NewEncoder(w).Encode(map[string]any{
				"items": []map[string]any{
					{"id": "primary", "summary": "Primary", "primary": true},
				},
				"nextPageToken": "page-2",
			})
			return
		}
		if r.URL.Query().Get("pageToken") != "page-2" {
			t.Fatalf("unexpected page token: %q", r.URL.Query().Get("pageToken"))
		}
		_ = json.NewEncoder(w).Encode(map[string]any{
			"items": []map[string]any{
				{"id": "team@example.com", "summary": "Team"},
			},
		})
	})

	p, _, q := newProviderEnv(t, handler)
	_ = p.Store.Set(service.ProviderGoogle, "user@example.com", secrets.Token{AccessToken: "token"})

	cals, err := p.SyncCalendars(context.Background(), "user@example.com")
	if err != nil {
		t.Fatal(err)
	}
	if calls != 2 {
		t.Fatalf("calendar list calls: %d", calls)
	}
	if len(cals) != 2 {
		t.Fatalf("calendars: %d", len(cals))
	}

	primary, err := q.GetCalendarByProviderExternalID(context.Background(), sqlc.GetCalendarByProviderExternalIDParams{
		Provider: service.ProviderGoogle, ExternalID: "primary",
	})
	if err != nil {
		t.Fatal(err)
	}
	if primary.Name != "Primary" || primary.IsPrimary != 1 || primary.Selected != 1 {
		t.Fatalf("primary calendar: %+v", primary)
	}
}

func TestFetchEventsMapsTimedRecurringAllDayAndPagination(t *testing.T) {
	var eventCalls int
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.URL.Path == "/calendars/primary/events":
			eventCalls++
			if r.URL.Query().Get("pageToken") == "" {
				assertQuery(t, r.URL.Query(), map[string]string{
					"singleEvents": "true",
					"orderBy":      "startTime",
					"timeMin":      "2026-06-01T00:00:00Z",
					"timeMax":      "2026-06-15T00:00:00Z",
				})
				_ = json.NewEncoder(w).Encode(map[string]any{
					"items": []map[string]any{
						{
							"id":                "evt-recurring",
							"recurringEventId":  "series-1",
							"iCalUID":           "uid-recurring@google.com",
							"summary":           "Weekly sync",
							"start":             map[string]string{"dateTime": "2026-06-02T13:00:00-04:00", "timeZone": "America/Toronto"},
							"end":               map[string]string{"dateTime": "2026-06-02T13:30:00-04:00", "timeZone": "America/Toronto"},
							"originalStartTime": map[string]string{"dateTime": "2026-06-02T14:00:00-04:00", "timeZone": "America/Toronto"},
							"organizer":         map[string]string{"email": "boss@example.com", "displayName": "Boss"},
							"attendees": []map[string]any{
								{"email": "user@example.com", "self": true, "responseStatus": "accepted"},
								{"email": "peer@example.com", "responseStatus": "tentative"},
							},
						},
						{
							"id":      "evt-allday",
							"iCalUID": "uid-allday@google.com",
							"summary": "Holiday",
							"start":   map[string]string{"date": "2026-06-03", "timeZone": "America/Toronto"},
							"end":     map[string]string{"date": "2026-06-04", "timeZone": "America/Toronto"},
						},
					},
					"nextPageToken": "events-2",
				})
				return
			}
			if r.URL.Query().Get("pageToken") != "events-2" {
				t.Fatalf("unexpected events page token: %q", r.URL.Query().Get("pageToken"))
			}
			_ = json.NewEncoder(w).Encode(map[string]any{
				"items": []map[string]any{
					{
						"id":      "evt-cancelled",
						"summary": "Cancelled",
						"status":  "cancelled",
						"start":   map[string]string{"dateTime": "2026-06-05T10:00:00Z"},
						"end":     map[string]string{"dateTime": "2026-06-05T11:00:00Z"},
					},
				},
			})
		default:
			http.NotFound(w, r)
		}
	})

	p, _, _ := newProviderEnv(t, handler)
	_ = p.Store.Set(service.ProviderGoogle, "user@example.com", secrets.Token{AccessToken: "token"})

	cal := sqlc.Calendar{ID: 42, Provider: service.ProviderGoogle, ExternalID: "primary", Name: "Primary"}
	events, err := p.FetchEvents(context.Background(), "user@example.com", "2026-06-01", "2026-06-14", []sqlc.Calendar{cal})
	if err != nil {
		t.Fatal(err)
	}
	if eventCalls != 2 {
		t.Fatalf("event list calls: %d", eventCalls)
	}
	if len(events) != 2 {
		t.Fatalf("events: %d", len(events))
	}

	recurring := events[0]
	if recurring.ExternalID != "evt-recurring" || recurring.RecurringEventID != "series-1" {
		t.Fatalf("recurring ids: %+v", recurring)
	}
	if recurring.InstanceID != "2026-06-02T18:00:00Z" {
		t.Fatalf("instance id: %q", recurring.InstanceID)
	}
	if recurring.ICalUID != "uid-recurring@google.com" {
		t.Fatalf("ical uid: %q", recurring.ICalUID)
	}
	if recurring.Status != "accepted" {
		t.Fatalf("self status: %q", recurring.Status)
	}
	if recurring.Organizer != "Boss" {
		t.Fatalf("organizer: %q", recurring.Organizer)
	}
	if recurring.OriginalTz != "America/Toronto" {
		t.Fatalf("tz: %q", recurring.OriginalTz)
	}
	if recurring.Start == nil || !recurring.Start.Equal(time.Date(2026, 6, 2, 17, 0, 0, 0, time.UTC)) {
		t.Fatalf("start: %+v", recurring.Start)
	}
	if len(recurring.Attendees) != 2 || recurring.Attendees[1].ResponseStatus != "tentative" {
		t.Fatalf("attendees: %+v", recurring.Attendees)
	}

	allDay := events[1]
	if !allDay.AllDay || allDay.StartDate != "2026-06-03" || allDay.EndDate != "2026-06-04" {
		t.Fatalf("all-day mapping: %+v", allDay)
	}
	if allDay.ICalUID != "uid-allday@google.com" {
		t.Fatalf("all-day ical uid: %q", allDay.ICalUID)
	}
}

func TestFetchEventsPropagatesAPIError(t *testing.T) {
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusForbidden)
		_, _ = w.Write([]byte(`{"error":"forbidden"}`))
	})
	p, _, _ := newProviderEnv(t, handler)
	_ = p.Store.Set(service.ProviderGoogle, "user@example.com", secrets.Token{AccessToken: "token"})

	_, err := p.FetchEvents(
		context.Background(),
		"user@example.com",
		"2026-06-01",
		"2026-06-14",
		[]sqlc.Calendar{{ID: 1, ExternalID: "primary"}},
	)
	if err == nil || !strings.Contains(err.Error(), "forbidden") {
		t.Fatalf("expected api error, got %v", err)
	}
}

func TestDisconnectRemovesTokenAndConnection(t *testing.T) {
	p, reg, _ := newProviderEnv(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/users/me/calendarList" {
			_ = json.NewEncoder(w).Encode(map[string]any{"items": []any{}})
			return
		}
		http.NotFound(w, r)
	}))
	ctx := context.Background()

	p.Authorizer = stubAuthorizer{
		result: oauth.Result{
			Provider:  service.ProviderGoogle,
			AccountID: "user@example.com",
			Token:     secrets.Token{AccessToken: "access"},
			Scopes:    []string{"calendar.readonly"},
		},
	}
	if _, err := p.Connect(ctx, "user@example.com", "Work"); err != nil {
		t.Fatal(err)
	}
	if err := p.Disconnect(ctx, "user@example.com"); err != nil {
		t.Fatal(err)
	}
	if _, err := reg.Get(ctx, service.ProviderGoogle, "user@example.com"); err == nil {
		t.Fatal("expected connection removed")
	}
	if _, err := p.Store.Get(service.ProviderGoogle, "user@example.com"); err == nil {
		t.Fatal("expected token removed")
	}
}

func assertQuery(t *testing.T, got url.Values, want map[string]string) {
	t.Helper()
	for k, v := range want {
		if got.Get(k) != v {
			t.Fatalf("query %s: got %q want %q", k, got.Get(k), v)
		}
	}
}
