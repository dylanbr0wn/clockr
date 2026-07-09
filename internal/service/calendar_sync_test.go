package service_test

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/dylanbr0wn/shiet/internal/db"
	"github.com/dylanbr0wn/shiet/internal/db/sqlc"
	"github.com/dylanbr0wn/shiet/internal/seed"
	"github.com/dylanbr0wn/shiet/internal/service"
)

type stubPuller struct {
	syncCalls  int
	fetchCalls int
	events     []service.IncomingEvent
}

func (s *stubPuller) SyncCalendars(ctx context.Context, accountID string) ([]sqlc.Calendar, error) {
	s.syncCalls++
	return nil, nil
}

func (s *stubPuller) FetchEvents(ctx context.Context, accountID, periodStart, periodEnd string, calendars []sqlc.Calendar) ([]service.IncomingEvent, error) {
	s.fetchCalls++
	return s.events, nil
}

type stubConnections struct {
	accounts []service.IntegrationAccount
}

func (s stubConnections) ListByProvider(ctx context.Context, provider string) ([]service.IntegrationAccount, error) {
	return s.accounts, nil
}

func newCalendarSyncSvc(t *testing.T, puller *stubPuller, accounts []service.IntegrationAccount) (*service.Service, int64, int64) {
	t.Helper()
	path := filepath.Join(t.TempDir(), "calendar-sync.db")
	conn, err := db.Open(path)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	t.Cleanup(func() { _ = conn.Close() })
	if err := db.Migrate(conn); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	if err := seed.Dev(context.Background(), conn); err != nil {
		t.Fatalf("seed: %v", err)
	}

	svc := service.New(conn)
	svc.SetCalendarSync(service.CalendarSyncConfig{
		Puller:      puller,
		Connections: stubConnections{accounts: accounts},
	})

	periods, err := svc.ListPeriods(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	cals, err := svc.ListCalendars(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	return svc, periods[0].ID, cals[0].ID
}

func TestSyncPeriod_PullsAndTouchesPeriod(t *testing.T) {
	ctx := context.Background()
	start := tm("2026-06-02T10:00:00Z")
	end := tm("2026-06-02T11:00:00Z")

	puller := &stubPuller{
		events: []service.IncomingEvent{{
			Provider:   service.ProviderGoogle,
			ExternalID: "sync-evt-1",
			Title:      "Imported",
			Status:     "accepted",
			Start:      start,
			End:        end,
			OriginalTz: "America/Toronto",
		}},
	}
	svc, periodID, calID := newCalendarSyncSvc(t, puller, []service.IntegrationAccount{{
		Provider: service.ProviderGoogle, AccountID: "user@example.com", Status: "connected",
	}})
	puller.events[0].CalendarID = calID

	res, err := svc.SyncPeriod(ctx, periodID)
	if err != nil {
		t.Fatal(err)
	}
	if res.Added != 1 {
		t.Fatalf("sync result: %+v", res)
	}
	if puller.syncCalls != 1 || puller.fetchCalls != 1 {
		t.Fatalf("puller calls: sync=%d fetch=%d", puller.syncCalls, puller.fetchCalls)
	}

	period, err := svc.GetPeriod(ctx, periodID)
	if err != nil {
		t.Fatal(err)
	}
	if period.LastSyncedAt == nil {
		t.Fatal("expected lastSyncedAt to be set")
	}
}

func TestSyncPeriod_NoConnectedAccounts(t *testing.T) {
	svc, periodID, _ := newCalendarSyncSvc(t, &stubPuller{}, nil)
	_, err := svc.SyncPeriod(context.Background(), periodID)
	if err == nil || err != service.ErrNoConnectedAccounts {
		t.Fatalf("want ErrNoConnectedAccounts, got %v", err)
	}
}

func TestSyncPeriod_NeedsReauth(t *testing.T) {
	svc, periodID, _ := newCalendarSyncSvc(t, &stubPuller{}, []service.IntegrationAccount{{
		Provider: service.ProviderGoogle, AccountID: "user@example.com", Status: "needs_reauth",
	}})
	_, err := svc.SyncPeriod(context.Background(), periodID)
	if err == nil || err != service.ErrNeedsReauth {
		t.Fatalf("want ErrNeedsReauth, got %v", err)
	}
}

func TestSyncPeriod_ReSyncPreservesFacts(t *testing.T) {
	ctx := context.Background()
	puller := &stubPuller{
		events: []service.IncomingEvent{{
			Provider:   service.ProviderGoogle,
			ExternalID: "stable-evt",
			Title:      "Stable",
			Status:     "accepted",
			Start:      tm("2026-06-03T10:00:00Z"),
			End:        tm("2026-06-03T11:00:00Z"),
		}},
	}
	svc, periodID, calID := newCalendarSyncSvc(t, puller, []service.IntegrationAccount{{
		Provider: service.ProviderGoogle, AccountID: "user@example.com", Status: "connected",
	}})
	puller.events[0].CalendarID = calID

	r1, err := svc.SyncPeriod(ctx, periodID)
	if err != nil {
		t.Fatal(err)
	}
	if r1.Added != 1 {
		t.Fatalf("first sync: %+v", r1)
	}

	r2, err := svc.SyncPeriod(ctx, periodID)
	if err != nil {
		t.Fatal(err)
	}
	if r2.Unchanged != 1 || r2.Added != 0 {
		t.Fatalf("second sync: %+v", r2)
	}
}

func TestSetCalendarSelected_SoftHidesEvents(t *testing.T) {
	ctx := context.Background()
	puller := &stubPuller{
		events: []service.IncomingEvent{{
			Provider:   service.ProviderGoogle,
			ExternalID: "hide-me",
			Title:      "Hide me",
			Status:     "accepted",
			Start:      tm("2026-06-05T10:00:00Z"),
			End:        tm("2026-06-05T11:00:00Z"),
		}},
	}
	svc, periodID, calID := newCalendarSyncSvc(t, puller, []service.IntegrationAccount{{
		Provider: service.ProviderGoogle, AccountID: "user@example.com", Status: "connected",
	}})
	puller.events[0].CalendarID = calID
	if _, err := svc.SyncPeriod(ctx, periodID); err != nil {
		t.Fatal(err)
	}

	events, err := svc.ListEvents(ctx, periodID)
	if err != nil {
		t.Fatal(err)
	}
	if len(events) == 0 {
		t.Fatal("expected synced event before deselect")
	}

	if err := svc.SetCalendarSelected(ctx, calID, false); err != nil {
		t.Fatal(err)
	}
	events, err = svc.ListEvents(ctx, periodID)
	if err != nil {
		t.Fatal(err)
	}
	if len(events) != 0 {
		t.Fatalf("expected hidden events, got %d", len(events))
	}

	if err := svc.SetCalendarSelected(ctx, calID, true); err != nil {
		t.Fatal(err)
	}
	events, err = svc.ListEvents(ctx, periodID)
	if err != nil {
		t.Fatal(err)
	}
	if len(events) == 0 {
		t.Fatal("expected events restored after reselect")
	}
}

func TestSetCalendarDefaultCategory(t *testing.T) {
	ctx := context.Background()
	svc := newSvc(t)
	cals, err := svc.ListCalendars(ctx)
	if err != nil {
		t.Fatal(err)
	}
	cats, err := svc.ListCategories(ctx)
	if err != nil {
		t.Fatal(err)
	}

	catID := cats[0].ID
	if err := svc.SetCalendarDefaultCategory(ctx, cals[0].ID, &catID); err != nil {
		t.Fatal(err)
	}

	got, err := svc.ListCalendars(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if got[0].DefaultCategoryID == nil || *got[0].DefaultCategoryID != catID {
		t.Fatalf("default category not set: %+v", got[0])
	}

	if err := svc.SetCalendarDefaultCategory(ctx, cals[0].ID, nil); err != nil {
		t.Fatal(err)
	}
	got, err = svc.ListCalendars(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if got[0].DefaultCategoryID != nil {
		t.Fatalf("expected cleared default category, got %+v", got[0].DefaultCategoryID)
	}
}
