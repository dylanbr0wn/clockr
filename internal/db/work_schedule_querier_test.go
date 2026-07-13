package db_test

import (
	"context"
	"database/sql"
	"path/filepath"
	"testing"

	"github.com/dylanbr0wn/shiet/internal/db"
	"github.com/dylanbr0wn/shiet/internal/db/sqlc"
)

func TestWorkScheduleQuerier_RoundTripAndException(t *testing.T) {
	t.Parallel()

	path := filepath.Join(t.TempDir(), "work-schedule-querier.db")
	conn, err := db.Open(path)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	t.Cleanup(func() { _ = conn.Close() })
	if err := db.Migrate(conn); err != nil {
		t.Fatalf("migrate: %v", err)
	}

	ctx := context.Background()
	q := sqlc.New(conn)

	sched, err := q.CreateWorkSchedule(ctx, sqlc.CreateWorkScheduleParams{
		Timezone:      "America/Vancouver",
		WorkweekStart: "sunday",
		EffectiveFrom: "2026-01-01",
		EffectiveTo:   sql.NullString{String: "2026-07-01", Valid: true},
	})
	if err != nil {
		t.Fatalf("create schedule: %v", err)
	}

	day, err := q.CreateWorkScheduleDay(ctx, sqlc.CreateWorkScheduleDayParams{
		WorkScheduleID:  sched.ID,
		Weekday:         "monday",
		ExpectedMinutes: 240,
	})
	if err != nil {
		t.Fatalf("create day: %v", err)
	}
	if _, err := q.CreateWorkScheduleWindow(ctx, sqlc.CreateWorkScheduleWindowParams{
		WorkScheduleDayID: day.ID,
		StartMinutes:      10 * 60,
		EndMinutes:        14 * 60,
	}); err != nil {
		t.Fatalf("create window: %v", err)
	}

	got, err := q.GetWorkSchedule(ctx, sched.ID)
	if err != nil {
		t.Fatalf("get schedule: %v", err)
	}
	if got.Timezone != "America/Vancouver" || got.WorkweekStart != "sunday" {
		t.Fatalf("schedule = %+v", got)
	}
	if !got.EffectiveTo.Valid || got.EffectiveTo.String != "2026-07-01" {
		t.Fatalf("effective_to = %+v, want 2026-07-01", got.EffectiveTo)
	}

	days, err := q.ListWorkScheduleDays(ctx, sched.ID)
	if err != nil {
		t.Fatalf("list days: %v", err)
	}
	if len(days) != 1 || days[0].ExpectedMinutes != 240 {
		t.Fatalf("days = %+v", days)
	}
	wins, err := q.ListWorkScheduleWindows(ctx, day.ID)
	if err != nil {
		t.Fatalf("list windows: %v", err)
	}
	if len(wins) != 1 || wins[0].StartMinutes != 600 || wins[0].EndMinutes != 840 {
		t.Fatalf("windows = %+v", wins)
	}

	ex, err := q.CreateScheduleException(ctx, sqlc.CreateScheduleExceptionParams{
		Date:            "2026-06-01",
		Kind:            "changed_hours",
		ExpectedMinutes: 120,
	})
	if err != nil {
		t.Fatalf("create exception: %v", err)
	}
	if _, err := q.CreateScheduleExceptionWindow(ctx, sqlc.CreateScheduleExceptionWindowParams{
		ScheduleExceptionID: ex.ID,
		StartMinutes:        13 * 60,
		EndMinutes:          15 * 60,
	}); err != nil {
		t.Fatalf("create exception window: %v", err)
	}

	byDate, err := q.GetScheduleExceptionByDate(ctx, "2026-06-01")
	if err != nil {
		t.Fatalf("get exception: %v", err)
	}
	if byDate.Kind != "changed_hours" || byDate.ExpectedMinutes != 120 {
		t.Fatalf("exception = %+v", byDate)
	}

	// Duplicate weekday rejected.
	if _, err := q.CreateWorkScheduleDay(ctx, sqlc.CreateWorkScheduleDayParams{
		WorkScheduleID:  sched.ID,
		Weekday:         "monday",
		ExpectedMinutes: 0,
	}); err == nil {
		t.Fatal("duplicate weekday should fail")
	}

	// Duplicate exception date rejected.
	if _, err := q.CreateScheduleException(ctx, sqlc.CreateScheduleExceptionParams{
		Date:            "2026-06-01",
		Kind:            "holiday",
		ExpectedMinutes: 0,
	}); err == nil {
		t.Fatal("duplicate exception date should fail")
	}

	// Invalid kind rejected.
	if _, err := q.CreateScheduleException(ctx, sqlc.CreateScheduleExceptionParams{
		Date:            "2026-06-02",
		Kind:            "pto",
		ExpectedMinutes: 0,
	}); err == nil {
		t.Fatal("invalid exception kind should fail")
	}

	// Invalid workweek_start rejected.
	if _, err := q.CreateWorkSchedule(ctx, sqlc.CreateWorkScheduleParams{
		Timezone:      "UTC",
		WorkweekStart: "mon",
		EffectiveFrom: "2027-01-01",
		EffectiveTo:   sql.NullString{String: "2027-06-01", Valid: true},
	}); err == nil {
		t.Fatal("invalid workweek_start should fail")
	}

	// Invalid weekday rejected.
	if _, err := q.CreateWorkScheduleDay(ctx, sqlc.CreateWorkScheduleDayParams{
		WorkScheduleID:  sched.ID,
		Weekday:         "mon",
		ExpectedMinutes: 0,
	}); err == nil {
		t.Fatal("invalid weekday should fail")
	}

	// Window end must be after start; bounds 0–1440.
	if _, err := q.CreateWorkScheduleWindow(ctx, sqlc.CreateWorkScheduleWindowParams{
		WorkScheduleDayID: day.ID,
		StartMinutes:      600,
		EndMinutes:        600,
	}); err == nil {
		t.Fatal("equal window start/end should fail")
	}
	if _, err := q.CreateWorkScheduleWindow(ctx, sqlc.CreateWorkScheduleWindowParams{
		WorkScheduleDayID: day.ID,
		StartMinutes:      -1,
		EndMinutes:        60,
	}); err == nil {
		t.Fatal("negative start_minutes should fail")
	}

	// effective_to must be after effective_from.
	if _, err := q.CreateWorkSchedule(ctx, sqlc.CreateWorkScheduleParams{
		Timezone:      "UTC",
		WorkweekStart: "monday",
		EffectiveFrom: "2027-06-01",
		EffectiveTo:   sql.NullString{String: "2027-01-01", Valid: true},
	}); err == nil {
		t.Fatal("effective_to <= effective_from should fail")
	}

	// Adjacent half-open ranges OK; overlapping rejected (including vs open-ended).
	if _, err := q.CreateWorkSchedule(ctx, sqlc.CreateWorkScheduleParams{
		Timezone:      "UTC",
		WorkweekStart: "monday",
		EffectiveFrom: "2026-07-01",
		EffectiveTo:   sql.NullString{String: "2027-01-01", Valid: true},
	}); err != nil {
		t.Fatalf("adjacent schedule should succeed: %v", err)
	}
	if _, err := q.CreateWorkSchedule(ctx, sqlc.CreateWorkScheduleParams{
		Timezone:      "UTC",
		WorkweekStart: "monday",
		EffectiveFrom: "2026-06-01",
		EffectiveTo:   sql.NullString{String: "2026-08-01", Valid: true},
	}); err == nil {
		t.Fatal("overlapping schedule should fail")
	}
	if _, err := q.CreateWorkSchedule(ctx, sqlc.CreateWorkScheduleParams{
		Timezone:      "UTC",
		WorkweekStart: "monday",
		EffectiveFrom: "2027-06-01",
		EffectiveTo:   sql.NullString{},
	}); err != nil {
		t.Fatalf("later open-ended schedule should succeed: %v", err)
	}
	if _, err := q.CreateWorkSchedule(ctx, sqlc.CreateWorkScheduleParams{
		Timezone:      "UTC",
		WorkweekStart: "monday",
		EffectiveFrom: "2028-01-01",
		EffectiveTo:   sql.NullString{String: "2028-06-01", Valid: true},
	}); err == nil {
		t.Fatal("overlap with open-ended schedule should fail")
	}
}
