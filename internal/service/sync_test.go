package service_test

import (
	"context"
	"database/sql"
	"path/filepath"
	"testing"
	"time"

	"github.com/dylanbr0wn/clockr/internal/db"
	"github.com/dylanbr0wn/clockr/internal/db/sqlc"
	"github.com/dylanbr0wn/clockr/internal/seed"
	"github.com/dylanbr0wn/clockr/internal/service"
)

// syncEnv is a migrated+seeded db with handles for building sync fixtures.
type syncEnv struct {
	svc      *service.Service
	q        *sqlc.Queries // direct access for fixtures (overlays, memory, gaps)
	periodID int64
	calID    int64
	catID    int64
}

func newSyncEnv(t *testing.T) *syncEnv {
	t.Helper()
	ctx := context.Background()
	path := filepath.Join(t.TempDir(), "sync.db")
	conn, err := db.Open(path)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	t.Cleanup(func() { _ = conn.Close() })
	if err := db.Migrate(conn); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	if err := seed.Dev(ctx, conn); err != nil {
		t.Fatalf("seed: %v", err)
	}

	svc := service.New(conn)
	periods, _ := svc.ListPeriods(ctx)
	cals, _ := svc.ListCalendars(ctx)
	cats, _ := svc.ListCategories(ctx)
	return &syncEnv{
		svc:      svc,
		q:        sqlc.New(conn),
		periodID: periods[0].ID,
		calID:    cals[0].ID,
		catID:    cats[0].ID,
	}
}

func tm(s string) *time.Time {
	v, err := time.Parse(time.RFC3339, s)
	if err != nil {
		panic(err)
	}
	v = v.UTC()
	return &v
}

// baseEvent is a timed, accepted event used as a building block.
func (e *syncEnv) baseEvent() service.IncomingEvent {
	return service.IncomingEvent{
		CalendarID: e.calID,
		Provider:   service.ProviderGoogle,
		ExternalID: "evt-1",
		Title:         "Standup",
		Status:        "accepted",
		Start:         tm("2026-06-02T09:00:00Z"),
		End:           tm("2026-06-02T09:30:00Z"),
		OriginalTz:    "America/Toronto",
	}
}

func TestSync_AddAndUnchanged(t *testing.T) {
	e := newSyncEnv(t)
	ctx := context.Background()
	batch := []service.IncomingEvent{e.baseEvent()}

	r1, err := e.svc.SyncEvents(ctx, e.periodID, batch)
	if err != nil {
		t.Fatal(err)
	}
	if r1.Added != 1 || r1.Updated != 0 || r1.Unchanged != 0 {
		t.Fatalf("first sync: %+v", r1)
	}

	r2, err := e.svc.SyncEvents(ctx, e.periodID, batch)
	if err != nil {
		t.Fatal(err)
	}
	if r2.Unchanged != 1 || r2.Added != 0 {
		t.Fatalf("second sync should be all-unchanged: %+v", r2)
	}
}

func TestSync_MemoryAutoCategorizes(t *testing.T) {
	e := newSyncEnv(t)
	ctx := context.Background()

	// Train memory on a recurring series key.
	if _, err := e.q.RememberCategory(ctx, sqlc.RememberCategoryParams{
		MatchKey: "rid:series-1", CategoryID: e.catID,
	}); err != nil {
		t.Fatal(err)
	}

	inc := e.baseEvent()
	inc.RecurringEventID = "series-1"
	if _, err := e.svc.SyncEvents(ctx, e.periodID, []service.IncomingEvent{inc}); err != nil {
		t.Fatal(err)
	}

	o, err := e.q.GetOverlay(ctx, sqlc.GetOverlayParams{
		PeriodID: e.periodID, Provider: service.ProviderGoogle, ExternalID: "evt-1", InstanceID: "", Kind: "category",
	})
	if err != nil {
		t.Fatalf("expected memory overlay: %v", err)
	}
	if !o.CategoryID.Valid || o.CategoryID.Int64 != e.catID {
		t.Fatalf("overlay category mismatch: %+v", o)
	}
}

func TestSync_TimeOnlyChangeKeepsCategorySilently(t *testing.T) {
	e := newSyncEnv(t)
	ctx := context.Background()

	if _, err := e.svc.SyncEvents(ctx, e.periodID, []service.IncomingEvent{e.baseEvent()}); err != nil {
		t.Fatal(err)
	}
	// User categorizes it.
	mustOverlay(t, e, "evt-1")

	// Same title, shifted time.
	moved := e.baseEvent()
	moved.Start = tm("2026-06-02T10:00:00Z")
	moved.End = tm("2026-06-02T10:30:00Z")

	r, err := e.svc.SyncEvents(ctx, e.periodID, []service.IncomingEvent{moved})
	if err != nil {
		t.Fatal(err)
	}
	if r.Updated != 1 || r.Flagged != 0 {
		t.Fatalf("time-only change should update silently: %+v", r)
	}
	if items := openItems(t, e); len(items) != 0 {
		t.Fatalf("no review items expected, got %d", len(items))
	}
}

func TestSync_MaterialTitleChangeFlags(t *testing.T) {
	e := newSyncEnv(t)
	ctx := context.Background()

	if _, err := e.svc.SyncEvents(ctx, e.periodID, []service.IncomingEvent{e.baseEvent()}); err != nil {
		t.Fatal(err)
	}
	mustOverlay(t, e, "evt-1")

	renamed := e.baseEvent()
	renamed.Title = "Sprint Planning" // material change

	r, err := e.svc.SyncEvents(ctx, e.periodID, []service.IncomingEvent{renamed})
	if err != nil {
		t.Fatal(err)
	}
	if r.Updated != 1 || r.Flagged != 1 {
		t.Fatalf("title change should flag: %+v", r)
	}
	items := openItems(t, e)
	if len(items) != 1 || items[0].Kind != "title_changed" {
		t.Fatalf("want one title_changed item, got %+v", items)
	}
}

func TestSync_NewEventInFilledGapFlags(t *testing.T) {
	e := newSyncEnv(t)
	ctx := context.Background()

	// A gap fill covering 13:00–14:00.
	if _, err := e.q.CreateGapFill(ctx, sqlc.CreateGapFillParams{
		PeriodID:   e.periodID,
		Day:        "2026-06-02",
		StartUtc:   "2026-06-02T13:00:00Z",
		EndUtc:     "2026-06-02T14:00:00Z",
		CategoryID: sql.NullInt64{Int64: e.catID, Valid: true},
		Source:     "gap",
	}); err != nil {
		t.Fatal(err)
	}

	inc := e.baseEvent()
	inc.ExternalID = "evt-overlap"
	inc.Start = tm("2026-06-02T13:30:00Z")
	inc.End = tm("2026-06-02T14:30:00Z") // overlaps the gap fill

	r, err := e.svc.SyncEvents(ctx, e.periodID, []service.IncomingEvent{inc})
	if err != nil {
		t.Fatal(err)
	}
	if r.Added != 1 || r.Flagged != 1 {
		t.Fatalf("new-in-gap should add+flag: %+v", r)
	}
	if items := openItems(t, e); len(items) != 1 || items[0].Kind != "new_in_gap" {
		t.Fatalf("want one new_in_gap item, got %+v", items)
	}
}

func TestSync_DisappearedEvent(t *testing.T) {
	e := newSyncEnv(t)
	ctx := context.Background()

	// Two events; one will be categorized, one not.
	a := e.baseEvent() // evt-1, will be categorized → kept + flagged
	b := e.baseEvent()
	b.ExternalID = "evt-2" // uncategorized → removed
	if _, err := e.svc.SyncEvents(ctx, e.periodID, []service.IncomingEvent{a, b}); err != nil {
		t.Fatal(err)
	}
	mustOverlay(t, e, "evt-1")

	// Re-sync with an empty pull: both vanished.
	r, err := e.svc.SyncEvents(ctx, e.periodID, []service.IncomingEvent{})
	if err != nil {
		t.Fatal(err)
	}
	if r.Removed != 1 || r.Flagged != 1 {
		t.Fatalf("want 1 removed + 1 flagged, got %+v", r)
	}
	items := openItems(t, e)
	if len(items) != 1 || items[0].Kind != "deleted_categorized" {
		t.Fatalf("want one deleted_categorized item, got %+v", items)
	}
	// The categorized event's fact is retained.
	events, _ := e.svc.ListEvents(ctx, e.periodID)
	if len(events) != 1 || events[0].ExternalID != "evt-1" {
		t.Fatalf("categorized event should be retained, got %+v", events)
	}
}

func TestSync_AllDayAndTentativeFlags(t *testing.T) {
	e := newSyncEnv(t)
	ctx := context.Background()

	allDay := service.IncomingEvent{
		CalendarID: e.calID, Provider: service.ProviderGoogle, ExternalID: "evt-allday", Title: "Holiday",
		Status: "accepted", AllDay: true, StartDate: "2026-06-03", EndDate: "2026-06-04",
	}
	tentative := e.baseEvent()
	tentative.ExternalID = "evt-tent"
	tentative.Status = "tentative"

	r, err := e.svc.SyncEvents(ctx, e.periodID, []service.IncomingEvent{allDay, tentative})
	if err != nil {
		t.Fatal(err)
	}
	if r.Added != 2 || r.Flagged != 2 {
		t.Fatalf("want 2 added + 2 flagged, got %+v", r)
	}
	kinds := map[string]int{}
	for _, it := range openItems(t, e) {
		kinds[it.Kind]++
	}
	if kinds["all_day"] != 1 || kinds["tentative"] != 1 {
		t.Fatalf("want all_day + tentative, got %+v", kinds)
	}

	events, err := e.svc.ListEvents(ctx, e.periodID)
	if err != nil {
		t.Fatal(err)
	}
	if len(events) != 2 {
		t.Fatalf("want 2 listed events, got %+v", events)
	}
	var listedAllDay *service.Event
	for i := range events {
		if events[i].AllDay {
			listedAllDay = &events[i]
			break
		}
	}
	if listedAllDay == nil {
		t.Fatalf("all-day event missing from ListEvents: %+v", events)
	}
	if listedAllDay.StartDate != "2026-06-03" || listedAllDay.EndDate != "2026-06-04" {
		t.Fatalf("all-day dates: %+v", listedAllDay)
	}
}

// mustOverlay assigns the env's category to an event occurrence (user override).
func mustOverlay(t *testing.T, e *syncEnv, gid string) {
	t.Helper()
	if _, err := e.q.UpsertOverlay(context.Background(), sqlc.UpsertOverlayParams{
		PeriodID:   e.periodID,
		Provider:   service.ProviderGoogle,
		ExternalID: gid,
		InstanceID:    "",
		CategoryID:    sql.NullInt64{Int64: e.catID, Valid: true},
		Kind:          "category",
	}); err != nil {
		t.Fatalf("seed overlay: %v", err)
	}
}

func openItems(t *testing.T, e *syncEnv) []service.ReviewItem {
	t.Helper()
	items, err := e.svc.ListOpenReviewItems(context.Background(), e.periodID)
	if err != nil {
		t.Fatal(err)
	}
	return items
}
