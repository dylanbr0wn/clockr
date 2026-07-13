package service_test

import (
	"context"
	"database/sql"
	"testing"
	"time"

	"github.com/dylanbr0wn/shiet/internal/db/sqlc"
)

// insertTimeEntry seeds a confirmed time_entry for service tests (replaces CreateGapFill fixtures).
func insertTimeEntry(t *testing.T, q *sqlc.Queries, periodID int64, day, startUTC, endUTC string, categoryID sql.NullInt64, description string, gapOrigin bool) {
	t.Helper()
	start, err := time.Parse(time.RFC3339, startUTC)
	if err != nil {
		t.Fatalf("parse start: %v", err)
	}
	end, err := time.Parse(time.RFC3339, endUTC)
	if err != nil {
		t.Fatalf("parse end: %v", err)
	}
	method := sql.NullString{}
	if gapOrigin {
		method = sql.NullString{String: "gap_fill", Valid: true}
	}
	if _, err := q.CreateTimeEntry(context.Background(), sqlc.CreateTimeEntryParams{
		PeriodID:        periodID,
		StartInstant:    startUTC,
		EndInstant:      endUTC,
		DurationMinutes: int64(end.Sub(start) / time.Minute),
		LocalWorkDate:   day,
		CategoryID:      categoryID,
		Description:     description,
		Attestation:     "confirmed",
		Method:          method,
		WorkType:        "worked",
		BillableStatus:  "unset",
	}); err != nil {
		t.Fatalf("insert time entry: %v", err)
	}
}
