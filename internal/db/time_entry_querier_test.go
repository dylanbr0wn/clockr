package db_test

import (
	"context"
	"database/sql"
	"path/filepath"
	"testing"

	"github.com/dylanbr0wn/shiet/internal/db"
	"github.com/dylanbr0wn/shiet/internal/db/sqlc"
)

func TestTimeEntryQuerier_CRUDRoundTrip(t *testing.T) {
	t.Parallel()

	path := filepath.Join(t.TempDir(), "time-entry-querier.db")
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

	period, err := q.CreatePeriod(ctx, sqlc.CreatePeriodParams{
		StartDate:  "2026-06-01",
		EndDate:    "2026-06-14",
		Cadence:    "bi-weekly",
		AnchorDate: "2026-06-01",
	})
	if err != nil {
		t.Fatalf("create period: %v", err)
	}

	// Overnight span in America/Toronto (EDT, UTC-4):
	// start 03:00Z = 23:00 local Jun 1; end 05:00Z = 01:00 local Jun 2.
	const (
		startInstant    = "2026-06-02T03:00:00Z"
		endInstant      = "2026-06-02T05:00:00Z"
		durationMinutes = int64(120)
		localWorkDate   = "2026-06-01"
	)

	created, err := q.CreateTimeEntry(ctx, sqlc.CreateTimeEntryParams{
		PeriodID:        period.ID,
		StartInstant:    startInstant,
		EndInstant:      endInstant,
		DurationMinutes: durationMinutes,
		LocalWorkDate:   localWorkDate,
		CategoryID:      sql.NullInt64{},
		Description:     "overnight on-call",
		Attestation:     "confirmed",
		Method:          sql.NullString{String: "gap_fill", Valid: true},
		WorkType:        "worked",
		BillableStatus:  "unset",
	})
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	if created.DurationMinutes != durationMinutes {
		t.Fatalf("duration_minutes = %d, want %d", created.DurationMinutes, durationMinutes)
	}
	if created.LocalWorkDate != localWorkDate {
		t.Fatalf("local_work_date = %q, want %q", created.LocalWorkDate, localWorkDate)
	}
	if created.Attestation != "confirmed" {
		t.Fatalf("attestation = %q, want confirmed", created.Attestation)
	}
	if !created.Method.Valid || created.Method.String != "gap_fill" {
		t.Fatalf("method = %+v, want gap_fill", created.Method)
	}

	got, err := q.GetTimeEntry(ctx, sqlc.GetTimeEntryParams{ID: created.ID, PeriodID: period.ID})
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if got.Description != "overnight on-call" {
		t.Fatalf("get description = %q", got.Description)
	}

	listed, err := q.ListTimeEntriesForPeriod(ctx, period.ID)
	if err != nil {
		t.Fatalf("list period: %v", err)
	}
	if len(listed) != 1 {
		t.Fatalf("list period len = %d, want 1", len(listed))
	}

	dayListed, err := q.ListTimeEntriesForDay(ctx, sqlc.ListTimeEntriesForDayParams{
		PeriodID:      period.ID,
		LocalWorkDate: localWorkDate,
	})
	if err != nil {
		t.Fatalf("list day: %v", err)
	}
	if len(dayListed) != 1 {
		t.Fatalf("list day len = %d, want 1", len(dayListed))
	}

	updated, err := q.UpdateTimeEntry(ctx, sqlc.UpdateTimeEntryParams{
		StartInstant:    startInstant,
		EndInstant:      "2026-06-02T06:00:00Z",
		DurationMinutes: 180,
		LocalWorkDate:   localWorkDate,
		CategoryID:      sql.NullInt64{},
		Description:     "extended",
		WorkType:        "worked",
		BillableStatus:  "unset",
		ID:              created.ID,
		PeriodID:        period.ID,
	})
	if err != nil {
		t.Fatalf("update: %v", err)
	}
	if updated.DurationMinutes != 180 || updated.Description != "extended" {
		t.Fatalf("unexpected update: %+v", updated)
	}

	rows, err := q.DeleteTimeEntry(ctx, sqlc.DeleteTimeEntryParams{ID: created.ID, PeriodID: period.ID})
	if err != nil {
		t.Fatalf("delete: %v", err)
	}
	if rows != 1 {
		t.Fatalf("delete rows = %d, want 1", rows)
	}
	if _, err := q.GetTimeEntry(ctx, sqlc.GetTimeEntryParams{ID: created.ID, PeriodID: period.ID}); err == nil {
		t.Fatal("get after delete: want error")
	}
}
