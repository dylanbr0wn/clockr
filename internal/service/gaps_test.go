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

// gapEnv builds a db with a custom period + tz segment for precise gap tests.
type gapEnv struct {
	svc      *service.Service
	q        *sqlc.Queries
	periodID int64
	calID    int64
	catID    int64
}

func newGapEnv(t *testing.T, start, end, iana string, target float64) *gapEnv {
	t.Helper()
	ctx := context.Background()
	path := filepath.Join(t.TempDir(), "gap.db")
	conn, err := db.Open(path)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	t.Cleanup(func() { _ = conn.Close() })
	if err := db.Migrate(conn); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	// Core seed (categories + settings incl. window.start) but a custom period.
	if err := seed.Core(ctx, conn); err != nil {
		t.Fatalf("seed: %v", err)
	}
	q := sqlc.New(conn)
	cal, err := q.UpsertCalendar(ctx, sqlc.UpsertCalendarParams{Provider: service.ProviderGoogle, ExternalID: "primary", Name: "Primary", IsPrimary: 1, Column5: int64(1)})
	if err != nil {
		t.Fatal(err)
	}
	p, err := q.CreatePeriod(ctx, sqlc.CreatePeriodParams{
		StartDate: start, EndDate: end, Cadence: "weekly", AnchorDate: start, TargetHoursPerDay: target,
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := q.UpsertTzSegment(ctx, sqlc.UpsertTzSegmentParams{PeriodID: p.ID, EffectiveFromDate: start, IanaTz: iana}); err != nil {
		t.Fatal(err)
	}
	cats, _ := service.New(conn).ListCategories(ctx)
	return &gapEnv{svc: service.New(conn), q: q, periodID: p.ID, calID: cal.ID, catID: cats[0].ID}
}

func (e *gapEnv) addEvent(t *testing.T, gid string, startUTC, endUTC string) {
	t.Helper()
	if _, err := e.q.UpsertEvent(context.Background(), sqlc.UpsertEventParams{
		PeriodID:   e.periodID,
		CalendarID: e.calID,
		Provider:   service.ProviderGoogle,
		ExternalID: gid,
		Title:         gid,
		Status:        "accepted",
		Attendees:     "[]",
		StartUtc:      sql.NullString{String: startUTC, Valid: true},
		EndUtc:        sql.NullString{String: endUTC, Valid: true},
		SourceHash:    gid,
	}); err != nil {
		t.Fatal(err)
	}
}

func TestGaps_EmptyDayIsOneFullGap(t *testing.T) {
	e := newGapEnv(t, "2026-06-01", "2026-06-01", "America/Toronto", 8)
	days, err := e.svc.ComputeGaps(context.Background(), e.periodID)
	if err != nil {
		t.Fatal(err)
	}
	if len(days) != 1 {
		t.Fatalf("want 1 day, got %d", len(days))
	}
	d := days[0]
	if len(d.Gaps) != 1 {
		t.Fatalf("want 1 gap, got %d (%+v)", len(d.Gaps), d.Gaps)
	}
	if d.GapHours != 8 || d.CoveredHours != 0 {
		t.Fatalf("want 8 gap / 0 covered, got %v / %v", d.GapHours, d.CoveredHours)
	}
}

func TestGaps_WindowStartUsesSegmentTZ(t *testing.T) {
	// June → EDT (UTC-4). Local 09:00 → 13:00 UTC.
	e := newGapEnv(t, "2026-06-01", "2026-06-01", "America/Toronto", 8)
	days, _ := e.svc.ComputeGaps(context.Background(), e.periodID)
	ws := days[0].WindowStart
	if ws.Hour() != 13 || ws.Minute() != 0 {
		t.Fatalf("window start want 13:00Z (09:00 EDT), got %s", ws.Format(time.RFC3339))
	}
	we := days[0].WindowEnd
	if we.Sub(ws) != 8*time.Hour {
		t.Fatalf("window span want 8h, got %s", we.Sub(ws))
	}
}

func TestGaps_EventSplitsWindow(t *testing.T) {
	e := newGapEnv(t, "2026-06-01", "2026-06-01", "America/Toronto", 8)
	// Window 13:00–21:00Z. Event 15:00–16:00Z (1h) splits into two gaps.
	e.addEvent(t, "mid", "2026-06-01T15:00:00Z", "2026-06-01T16:00:00Z")
	days, _ := e.svc.ComputeGaps(context.Background(), e.periodID)
	d := days[0]
	if len(d.Gaps) != 2 {
		t.Fatalf("want 2 gaps around event, got %d (%+v)", len(d.Gaps), d.Gaps)
	}
	if d.CoveredHours != 1 || d.GapHours != 7 {
		t.Fatalf("want 1 covered / 7 gap, got %v / %v", d.CoveredHours, d.GapHours)
	}
}

func TestGaps_OverlappingEventsUnioned(t *testing.T) {
	e := newGapEnv(t, "2026-06-01", "2026-06-01", "America/Toronto", 8)
	e.addEvent(t, "a", "2026-06-01T15:00:00Z", "2026-06-01T16:30:00Z")
	e.addEvent(t, "b", "2026-06-01T16:00:00Z", "2026-06-01T17:00:00Z") // overlaps a
	days, _ := e.svc.ComputeGaps(context.Background(), e.periodID)
	d := days[0]
	// Union 15:00–17:00 = 2h covered, not 2.5h.
	if d.CoveredHours != 2 || d.GapHours != 6 {
		t.Fatalf("want 2 covered / 6 gap, got %v / %v", d.CoveredHours, d.GapHours)
	}
}

func TestGaps_EventOutsideWindowIgnored(t *testing.T) {
	e := newGapEnv(t, "2026-06-01", "2026-06-01", "America/Toronto", 8)
	// 06:00–07:00Z is before the 13:00Z window start.
	e.addEvent(t, "early", "2026-06-01T06:00:00Z", "2026-06-01T07:00:00Z")
	days, _ := e.svc.ComputeGaps(context.Background(), e.periodID)
	if days[0].CoveredHours != 0 || days[0].GapHours != 8 {
		t.Fatalf("out-of-window event should not count: %+v", days[0])
	}
}

func TestGaps_GapFillCounts(t *testing.T) {
	e := newGapEnv(t, "2026-06-01", "2026-06-01", "America/Toronto", 8)
	if _, err := e.q.CreateGapFill(context.Background(), sqlc.CreateGapFillParams{
		PeriodID: e.periodID, Day: "2026-06-01",
		StartUtc: "2026-06-01T13:00:00Z", EndUtc: "2026-06-01T15:00:00Z",
		CategoryID: sql.NullInt64{Int64: e.catID, Valid: true}, Source: "gap",
	}); err != nil {
		t.Fatal(err)
	}
	days, _ := e.svc.ComputeGaps(context.Background(), e.periodID)
	d := days[0]
	if d.CoveredHours != 2 || d.GapHours != 6 {
		t.Fatalf("want 2 covered / 6 gap from fill, got %v / %v", d.CoveredHours, d.GapHours)
	}
	if len(d.Filled) != 1 {
		t.Fatalf("want 1 fill span, got %d", len(d.Filled))
	}
}

func TestGaps_AllDayEventDoesNotOccupy(t *testing.T) {
	e := newGapEnv(t, "2026-06-01", "2026-06-01", "America/Toronto", 8)
	if _, err := e.q.UpsertEvent(context.Background(), sqlc.UpsertEventParams{
		PeriodID: e.periodID, CalendarID: e.calID, Provider: service.ProviderGoogle, ExternalID: "holiday", Title: "Holiday",
		Status: "accepted", Attendees: "[]", AllDay: 1,
		StartDate:  sql.NullString{String: "2026-06-01", Valid: true},
		EndDate:    sql.NullString{String: "2026-06-02", Valid: true},
		SourceHash: "holiday",
	}); err != nil {
		t.Fatal(err)
	}
	days, _ := e.svc.ComputeGaps(context.Background(), e.periodID)
	if days[0].GapHours != 8 {
		t.Fatalf("all-day event must not occupy the window, got %v gap", days[0].GapHours)
	}
}

func TestGaps_DeclinedEventExcluded(t *testing.T) {
	e := newGapEnv(t, "2026-06-01", "2026-06-01", "America/Toronto", 8)
	if _, err := e.q.UpsertEvent(context.Background(), sqlc.UpsertEventParams{
		PeriodID: e.periodID, CalendarID: e.calID, Provider: service.ProviderGoogle, ExternalID: "decl", Title: "Declined",
		Status: "declined", Attendees: "[]",
		StartUtc:   sql.NullString{String: "2026-06-01T15:00:00Z", Valid: true},
		EndUtc:     sql.NullString{String: "2026-06-01T16:00:00Z", Valid: true},
		SourceHash: "decl",
	}); err != nil {
		t.Fatal(err)
	}
	days, _ := e.svc.ComputeGaps(context.Background(), e.periodID)
	if days[0].GapHours != 8 {
		t.Fatalf("declined event must be excluded, got %v gap", days[0].GapHours)
	}
}

func TestGaps_DSTSpringForward(t *testing.T) {
	// US DST starts 2026-03-08. Window 09:00 local both days.
	//   Mar 7 EST (UTC-5) → 14:00Z; Mar 8 EDT (UTC-4) → 13:00Z.
	e := newGapEnv(t, "2026-03-07", "2026-03-08", "America/Toronto", 8)
	days, err := e.svc.ComputeGaps(context.Background(), e.periodID)
	if err != nil {
		t.Fatal(err)
	}
	if len(days) != 2 {
		t.Fatalf("want 2 days, got %d", len(days))
	}
	if h := days[0].WindowStart.UTC().Hour(); h != 14 {
		t.Fatalf("Mar 7 (EST) window start want 14:00Z, got %02d:00Z", h)
	}
	if h := days[1].WindowStart.UTC().Hour(); h != 13 {
		t.Fatalf("Mar 8 (EDT) window start want 13:00Z, got %02d:00Z", h)
	}
	// Both days still 8 working hours despite the 23h calendar day.
	for i, d := range days {
		if d.GapHours != 8 {
			t.Fatalf("day %d want 8 gap hours, got %v", i, d.GapHours)
		}
	}
}

func TestGaps_MultiDayPeriodAndSegmentSwitch(t *testing.T) {
	// Two segments: first 2 days Toronto, then switch to Vancouver (UTC-7 in June).
	e := newGapEnv(t, "2026-06-01", "2026-06-03", "America/Toronto", 8)
	if _, err := e.q.UpsertTzSegment(context.Background(), sqlc.UpsertTzSegmentParams{
		PeriodID: e.periodID, EffectiveFromDate: "2026-06-03", IanaTz: "America/Vancouver",
	}); err != nil {
		t.Fatal(err)
	}
	days, _ := e.svc.ComputeGaps(context.Background(), e.periodID)
	if len(days) != 3 {
		t.Fatalf("want 3 days, got %d", len(days))
	}
	if days[0].Tz != "America/Toronto" || days[2].Tz != "America/Vancouver" {
		t.Fatalf("segment switch wrong: %s ... %s", days[0].Tz, days[2].Tz)
	}
	// Vancouver 09:00 PDT (UTC-7) → 16:00Z.
	if h := days[2].WindowStart.UTC().Hour(); h != 16 {
		t.Fatalf("Vancouver window start want 16:00Z, got %02d:00Z", h)
	}
}
