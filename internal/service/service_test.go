package service_test

import (
	"context"
	"errors"
	"path/filepath"
	"testing"
	"time"

	"github.com/dylanbr0wn/shiet/internal/db"
	"github.com/dylanbr0wn/shiet/internal/seed"
	"github.com/dylanbr0wn/shiet/internal/service"
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
		if c.Color == "" {
			t.Fatalf("category %q missing color", c.Name)
		}
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

func TestEnsureCurrentPeriodCreatesNextBiWeeklyPeriod(t *testing.T) {
	s := newSvc(t)
	ctx := context.Background()

	p, err := s.EnsureCurrentPeriod(ctx, "2026-06-16", "America/Vancouver")
	if err != nil {
		t.Fatal(err)
	}
	if p.StartDate != "2026-06-15" || p.EndDate != "2026-06-28" {
		t.Fatalf("unexpected current period: %+v", p)
	}

	segs, err := s.ListTzSegments(ctx, p.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(segs) != 1 || segs[0].IanaTz != "America/Vancouver" {
		t.Fatalf("current period should have a timezone segment, got %+v", segs)
	}

	fill, err := s.CreateManualEvent(ctx, service.ManualEventInput{
		PeriodID:     p.ID,
		Day:          "2026-06-16",
		StartMinutes: 9 * 60,
		EndMinutes:   10 * 60,
	})
	if err != nil {
		t.Fatal(err)
	}
	if fill.Day != "2026-06-16" {
		t.Fatalf("manual event should be added to current period day, got %+v", fill)
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

func TestSeededEventsAndEmptyGapsAreNonNil(t *testing.T) {
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
	if len(events) != 3 {
		t.Fatalf("want 3 seeded events, got %d", len(events))
	}
	if events[0].Title == "" {
		t.Fatalf("seeded event should have a title: %+v", events[0])
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

func TestSetSetting(t *testing.T) {
	s := newSvc(t)
	ctx := context.Background()

	if err := s.SetSetting(ctx, "period.target_hours", `7.5`); err != nil {
		t.Fatal(err)
	}

	v, err := s.GetSetting(ctx, "period.target_hours")
	if err != nil {
		t.Fatal(err)
	}
	if v != `7.5` {
		t.Fatalf("unexpected setting value: %q", v)
	}
}

func TestCreateManualEvent(t *testing.T) {
	s := newSvc(t)
	ctx := context.Background()

	periods, err := s.ListPeriods(ctx)
	if err != nil {
		t.Fatal(err)
	}
	pid := periods[0].ID

	fill, err := s.CreateManualEvent(ctx, service.ManualEventInput{
		PeriodID:     pid,
		Day:          "2026-06-01",
		StartMinutes: 9 * 60,
		EndMinutes:   10*60 + 30,
		Note:         "New block",
	})
	if err != nil {
		t.Fatal(err)
	}
	if fill.PeriodID != pid || fill.Day != "2026-06-01" || fill.Source != "manual" || fill.Note != "New block" {
		t.Fatalf("unexpected fill: %+v", fill)
	}

	wantStart := time.Date(2026, 6, 1, 13, 0, 0, 0, time.UTC)
	wantEnd := time.Date(2026, 6, 1, 14, 30, 0, 0, time.UTC)
	gotStart, err := time.Parse(time.RFC3339, fill.Start)
	if err != nil {
		t.Fatal(err)
	}
	gotEnd, err := time.Parse(time.RFC3339, fill.End)
	if err != nil {
		t.Fatal(err)
	}
	if !gotStart.Equal(wantStart) || !gotEnd.Equal(wantEnd) {
		t.Fatalf("unexpected UTC span: %s to %s", fill.Start, fill.End)
	}

	fills, err := s.ListGapFills(ctx, pid)
	if err != nil {
		t.Fatal(err)
	}
	if len(fills) != 1 || fills[0].ID != fill.ID {
		t.Fatalf("manual event was not listed: %+v", fills)
	}
}

func TestUpdateManualEvent(t *testing.T) {
	s := newSvc(t)
	ctx := context.Background()

	periods, err := s.ListPeriods(ctx)
	if err != nil {
		t.Fatal(err)
	}
	pid := periods[0].ID

	fill, err := s.CreateManualEvent(ctx, service.ManualEventInput{
		PeriodID:     pid,
		Day:          "2026-06-01",
		StartMinutes: 9 * 60,
		EndMinutes:   10 * 60,
		Note:         "New block",
	})
	if err != nil {
		t.Fatal(err)
	}

	updated, err := s.UpdateManualEvent(ctx, service.ManualEventUpdateInput{
		ID: fill.ID,
		ManualEventInput: service.ManualEventInput{
			PeriodID:     pid,
			Day:          "2026-06-02",
			StartMinutes: 11 * 60,
			EndMinutes:   12 * 60,
			Note:         "Moved block",
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if updated.ID != fill.ID || updated.Day != "2026-06-02" || updated.Note != "Moved block" || updated.Source != "manual" {
		t.Fatalf("unexpected updated fill: %+v", updated)
	}

	wantStart := time.Date(2026, 6, 2, 15, 0, 0, 0, time.UTC)
	wantEnd := time.Date(2026, 6, 2, 16, 0, 0, 0, time.UTC)
	gotStart, err := time.Parse(time.RFC3339, updated.Start)
	if err != nil {
		t.Fatal(err)
	}
	gotEnd, err := time.Parse(time.RFC3339, updated.End)
	if err != nil {
		t.Fatal(err)
	}
	if !gotStart.Equal(wantStart) || !gotEnd.Equal(wantEnd) {
		t.Fatalf("unexpected UTC span: %s to %s", updated.Start, updated.End)
	}

	fills, err := s.ListGapFills(ctx, pid)
	if err != nil {
		t.Fatal(err)
	}
	if len(fills) != 1 || fills[0].ID != fill.ID || fills[0].Day != "2026-06-02" {
		t.Fatalf("manual event update was not listed: %+v", fills)
	}
}

func TestDeleteManualEvent(t *testing.T) {
	s := newSvc(t)
	ctx := context.Background()

	periods, err := s.ListPeriods(ctx)
	if err != nil {
		t.Fatal(err)
	}
	pid := periods[0].ID

	fill, err := s.CreateManualEvent(ctx, service.ManualEventInput{
		PeriodID:     pid,
		Day:          "2026-06-01",
		StartMinutes: 9 * 60,
		EndMinutes:   10 * 60,
		Note:         "Temporary block",
	})
	if err != nil {
		t.Fatal(err)
	}

	if err := s.DeleteManualEvent(ctx, service.ManualEventDeleteInput{
		ID:       fill.ID,
		PeriodID: pid,
	}); err != nil {
		t.Fatal(err)
	}

	fills, err := s.ListGapFills(ctx, pid)
	if err != nil {
		t.Fatal(err)
	}
	if len(fills) != 0 {
		t.Fatalf("manual event was not deleted: %+v", fills)
	}

	err = s.DeleteManualEvent(ctx, service.ManualEventDeleteInput{
		ID:       fill.ID,
		PeriodID: pid,
	})
	if !errors.Is(err, service.ErrNotFound) {
		t.Fatalf("want ErrNotFound after delete, got %v", err)
	}
}

func TestDeleteGapFillViaDeleteManualEvent(t *testing.T) {
	s := newSvc(t)
	ctx := context.Background()

	periods, err := s.ListPeriods(ctx)
	if err != nil {
		t.Fatal(err)
	}
	pid := periods[0].ID

	fill, err := s.CreateGapFill(ctx, service.ManualEventInput{
		PeriodID:     pid,
		Day:          "2026-06-01",
		StartMinutes: 9 * 60,
		EndMinutes:   10 * 60,
		Note:         "Confirmed gap",
	})
	if err != nil {
		t.Fatal(err)
	}
	if fill.Source != "gap" {
		t.Fatalf("want source gap, got %q", fill.Source)
	}

	if err := s.DeleteManualEvent(ctx, service.ManualEventDeleteInput{
		ID:       fill.ID,
		PeriodID: pid,
	}); err != nil {
		t.Fatal(err)
	}

	fills, err := s.ListGapFills(ctx, pid)
	if err != nil {
		t.Fatal(err)
	}
	if len(fills) != 0 {
		t.Fatalf("gap fill was not deleted: %+v", fills)
	}
}

func TestUpdateGapFillViaUpdateManualEvent(t *testing.T) {
	s := newSvc(t)
	ctx := context.Background()

	periods, err := s.ListPeriods(ctx)
	if err != nil {
		t.Fatal(err)
	}
	pid := periods[0].ID

	fill, err := s.CreateGapFill(ctx, service.ManualEventInput{
		PeriodID:     pid,
		Day:          "2026-06-01",
		StartMinutes: 9 * 60,
		EndMinutes:   10 * 60,
		Note:         "Confirmed gap",
	})
	if err != nil {
		t.Fatal(err)
	}

	updated, err := s.UpdateManualEvent(ctx, service.ManualEventUpdateInput{
		ID: fill.ID,
		ManualEventInput: service.ManualEventInput{
			PeriodID:     pid,
			Day:          "2026-06-01",
			StartMinutes: 10 * 60,
			EndMinutes:   11 * 60,
			Note:         "Moved gap",
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if updated.ID != fill.ID || updated.Source != "gap" || updated.Note != "Moved gap" {
		t.Fatalf("unexpected updated gap fill: %+v", updated)
	}
}

func TestManualEventDescriptionRoundTrip(t *testing.T) {
	s := newSvc(t)
	ctx := context.Background()

	periods, err := s.ListPeriods(ctx)
	if err != nil {
		t.Fatal(err)
	}
	pid := periods[0].ID

	fill, err := s.CreateManualEvent(ctx, service.ManualEventInput{
		PeriodID:     pid,
		Day:          "2026-06-01",
		StartMinutes: 9 * 60,
		EndMinutes:   10 * 60,
		Note:         "Title",
		Description:  "  Worked on feature X  ",
	})
	if err != nil {
		t.Fatal(err)
	}
	if fill.Note != "Title" || fill.Description != "Worked on feature X" {
		t.Fatalf("unexpected create fill: %+v", fill)
	}

	fills, err := s.ListGapFills(ctx, pid)
	if err != nil {
		t.Fatal(err)
	}
	if len(fills) != 1 || fills[0].Description != "Worked on feature X" {
		t.Fatalf("description not listed: %+v", fills)
	}

	updated, err := s.UpdateManualEvent(ctx, service.ManualEventUpdateInput{
		ID: fill.ID,
		ManualEventInput: service.ManualEventInput{
			PeriodID:     pid,
			Day:          "2026-06-01",
			StartMinutes: 9 * 60,
			EndMinutes:   10 * 60,
			Note:         "Title",
			Description:  "Updated description",
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if updated.Description != "Updated description" {
		t.Fatalf("unexpected updated description: %q", updated.Description)
	}

	gapFill, err := s.CreateGapFill(ctx, service.ManualEventInput{
		PeriodID:     pid,
		Day:          "2026-06-02",
		StartMinutes: 14 * 60,
		EndMinutes:   15 * 60,
		Description:  "AI suggested work",
	})
	if err != nil {
		t.Fatal(err)
	}
	if gapFill.Source != "gap" || gapFill.Note != "" || gapFill.Description != "AI suggested work" {
		t.Fatalf("unexpected gap fill: %+v", gapFill)
	}
}

func TestCreateManualEventValidation(t *testing.T) {
	s := newSvc(t)
	ctx := context.Background()

	periods, err := s.ListPeriods(ctx)
	if err != nil {
		t.Fatal(err)
	}
	pid := periods[0].ID

	_, err = s.CreateManualEvent(ctx, service.ManualEventInput{
		PeriodID:     pid,
		Day:          "2026-06-01",
		StartMinutes: 10 * 60,
		EndMinutes:   10 * 60,
	})
	if err == nil {
		t.Fatal("expected invalid range error")
	}

	_, err = s.CreateManualEvent(ctx, service.ManualEventInput{
		PeriodID:     pid,
		Day:          "2026-06-15",
		StartMinutes: 9 * 60,
		EndMinutes:   10 * 60,
	})
	if err == nil {
		t.Fatal("expected out-of-period error")
	}
}
