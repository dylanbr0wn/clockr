package service_test

import (
	"context"
	"errors"
	"path/filepath"
	"testing"

	"github.com/dylanbr0wn/clockr/internal/db"
	"github.com/dylanbr0wn/clockr/internal/seed"
	"github.com/dylanbr0wn/clockr/internal/service"
)

// newSvc opens a fresh temp database, migrates it, seeds dev data, and returns
// a Service over it.
func newSvc(t *testing.T) *service.Service {
	t.Helper()
	path := filepath.Join(t.TempDir(), "test.db")
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
	return service.New(conn)
}

func TestListCategories(t *testing.T) {
	s := newSvc(t)
	cats, err := s.ListCategories(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(cats) != 5 {
		t.Fatalf("want 5 seeded categories, got %d", len(cats))
	}
	var gaps int
	for _, c := range cats {
		if c.IsDefaultGap {
			gaps++
		}
	}
	if gaps != 1 {
		t.Fatalf("want exactly 1 default-gap category, got %d", gaps)
	}
}

func TestPeriodsAndTzSegments(t *testing.T) {
	s := newSvc(t)
	ctx := context.Background()

	periods, err := s.ListPeriods(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(periods) != 1 {
		t.Fatalf("want 1 seeded period, got %d", len(periods))
	}
	p := periods[0]
	if p.Cadence != "bi-weekly" || p.TargetHoursPerDay != 8 {
		t.Fatalf("unexpected period: %+v", p)
	}

	got, err := s.GetPeriodByRange(ctx, p.StartDate, p.EndDate)
	if err != nil {
		t.Fatal(err)
	}
	if got.ID != p.ID {
		t.Fatalf("GetPeriodByRange id mismatch: %d vs %d", got.ID, p.ID)
	}

	segs, err := s.ListTzSegments(ctx, p.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(segs) != 1 || segs[0].IanaTz != "America/Toronto" {
		t.Fatalf("unexpected tz segments: %+v", segs)
	}
}

func TestCalendars(t *testing.T) {
	s := newSvc(t)
	ctx := context.Background()

	all, err := s.ListCalendars(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(all) != 1 {
		t.Fatalf("want 1 seeded calendar, got %d", len(all))
	}
	if !all[0].IsPrimary {
		t.Fatalf("seeded calendar should be primary")
	}

	selected, err := s.ListSelectedCalendars(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(selected) != 1 {
		t.Fatalf("want 1 selected calendar, got %d", len(selected))
	}
}

func TestEmptyEventsAndGapsAreNonNil(t *testing.T) {
	s := newSvc(t)
	ctx := context.Background()

	periods, _ := s.ListPeriods(ctx)
	pid := periods[0].ID

	events, err := s.ListEvents(ctx, pid)
	if err != nil {
		t.Fatal(err)
	}
	if events == nil {
		t.Fatal("events slice should be non-nil")
	}
	if len(events) != 0 {
		t.Fatalf("want 0 seeded events, got %d", len(events))
	}

	gaps, err := s.ListGapFills(ctx, pid)
	if err != nil {
		t.Fatal(err)
	}
	if gaps == nil {
		t.Fatal("gaps slice should be non-nil")
	}
}

func TestGetCategoryNotFound(t *testing.T) {
	s := newSvc(t)
	_, err := s.GetCategory(context.Background(), 99999)
	if err == nil {
		t.Fatal("expected error for missing category")
	}
	if !errors.Is(err, service.ErrNotFound) {
		t.Fatalf("want ErrNotFound, got %v", err)
	}
}

func TestGetSetting(t *testing.T) {
	s := newSvc(t)
	v, err := s.GetSetting(context.Background(), "period.cadence")
	if err != nil {
		t.Fatal(err)
	}
	if v != `"bi-weekly"` {
		t.Fatalf("unexpected setting value: %q", v)
	}
}
