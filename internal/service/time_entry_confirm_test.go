package service_test

import (
	"context"
	"database/sql"
	"errors"
	"path/filepath"
	"testing"

	"github.com/dylanbr0wn/shiet/internal/db"
	"github.com/dylanbr0wn/shiet/internal/db/sqlc"
	"github.com/dylanbr0wn/shiet/internal/seed"
	"github.com/dylanbr0wn/shiet/internal/service"
)

type timeEntryEnv struct {
	svc      *service.Service
	q        *sqlc.Queries
	periodID int64
}

func newTimeEntryEnv(t *testing.T) timeEntryEnv {
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
	svc := service.New(conn)
	periods, err := svc.ListPeriods(context.Background())
	if err != nil || len(periods) == 0 {
		t.Fatalf("periods: %v", err)
	}
	return timeEntryEnv{svc: svc, q: sqlc.New(conn), periodID: periods[0].ID}
}

func TestConfirmTimeEntry_PromotesDraftToConfirmed(t *testing.T) {
	e := newTimeEntryEnv(t)
	ctx := context.Background()

	id := insertTimeEntryFull(t, e.q, e.periodID, "2026-06-01",
		"2026-06-01T13:00:00Z", "2026-06-01T14:30:00Z",
		sql.NullInt64{}, "Proposal", "draft", false)

	confirmed, err := e.svc.ConfirmTimeEntry(ctx, service.ConfirmTimeEntryInput{
		ID:       id,
		PeriodID: e.periodID,
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(confirmed) != 1 {
		t.Fatalf("want 1 confirmed entry, got %d", len(confirmed))
	}
	got := confirmed[0]
	if got.ID != id || got.Attestation != "confirmed" {
		t.Fatalf("unexpected confirm result: %+v", got)
	}
	if got.Description != "Proposal" || got.DurationMinutes != 90 {
		t.Fatalf("confirm mutated payload: %+v", got)
	}

	listed, err := e.svc.GetTimeEntry(ctx, id, e.periodID)
	if err != nil {
		t.Fatal(err)
	}
	if listed.Attestation != "confirmed" {
		t.Fatalf("want confirmed via get, got %q", listed.Attestation)
	}
}

func TestConfirmTimeEntry_RejectsNonDraft(t *testing.T) {
	e := newTimeEntryEnv(t)
	ctx := context.Background()

	id := insertTimeEntryFull(t, e.q, e.periodID, "2026-06-01",
		"2026-06-01T13:00:00Z", "2026-06-01T14:00:00Z",
		sql.NullInt64{}, "", "confirmed", false)

	_, err := e.svc.ConfirmTimeEntry(ctx, service.ConfirmTimeEntryInput{
		ID:       id,
		PeriodID: e.periodID,
	})
	if !errors.Is(err, service.ErrFailedPrecondition) {
		t.Fatalf("want ErrFailedPrecondition, got %v", err)
	}
}

func TestRejectTimeEntry_DismissesDraft(t *testing.T) {
	e := newTimeEntryEnv(t)
	ctx := context.Background()

	id := insertTimeEntryFull(t, e.q, e.periodID, "2026-06-01",
		"2026-06-01T13:00:00Z", "2026-06-01T14:00:00Z",
		sql.NullInt64{}, "Nope", "draft", false)

	got, err := e.svc.RejectTimeEntry(ctx, service.RejectTimeEntryInput{
		ID:       id,
		PeriodID: e.periodID,
	})
	if err != nil {
		t.Fatal(err)
	}
	if got.Attestation != "dismissed" || got.ID != id {
		t.Fatalf("unexpected reject: %+v", got)
	}
}

func TestAdjustDraftTimeEntry_UpdatesDraftInPlace(t *testing.T) {
	e := newTimeEntryEnv(t)
	ctx := context.Background()

	id := insertTimeEntryFull(t, e.q, e.periodID, "2026-06-01",
		"2026-06-01T13:00:00Z", "2026-06-01T14:00:00Z",
		sql.NullInt64{}, "Draft", "draft", false)

	got, err := e.svc.AdjustDraftTimeEntry(ctx, service.TimeEntryUpdateInput{
		ID: id,
		TimeEntryInput: service.TimeEntryInput{
			PeriodID:     e.periodID,
			Day:          "2026-06-01",
			StartMinutes: 10 * 60,
			EndMinutes:   12 * 60,
			Description:  "Tweaked",
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if got.Attestation != "draft" || got.Description != "Tweaked" || got.DurationMinutes != 120 {
		t.Fatalf("unexpected adjust: %+v", got)
	}
}

func TestAdjustDraftTimeEntry_RejectsConfirmed(t *testing.T) {
	e := newTimeEntryEnv(t)
	ctx := context.Background()

	entry, err := e.svc.CreateTimeEntry(ctx, service.TimeEntryInput{
		PeriodID:     e.periodID,
		Day:          "2026-06-01",
		StartMinutes: 9 * 60,
		EndMinutes:   10 * 60,
	})
	if err != nil {
		t.Fatal(err)
	}

	_, err = e.svc.AdjustDraftTimeEntry(ctx, service.TimeEntryUpdateInput{
		ID: entry.ID,
		TimeEntryInput: service.TimeEntryInput{
			PeriodID:     e.periodID,
			Day:          "2026-06-01",
			StartMinutes: 11 * 60,
			EndMinutes:   12 * 60,
		},
	})
	if !errors.Is(err, service.ErrFailedPrecondition) {
		t.Fatalf("want ErrFailedPrecondition, got %v", err)
	}
}

// overnightDraft seeds a 22:00→02:00 America/Toronto draft (4h crossing midnight).
func overnightDraft(t *testing.T, e timeEntryEnv) int64 {
	t.Helper()
	return insertTimeEntryFull(t, e.q, e.periodID, "2026-06-01",
		"2026-06-02T02:00:00Z", "2026-06-02T06:00:00Z",
		sql.NullInt64{}, "Overnight", "draft", false)
}

func TestConfirmTimeEntry_OvernightRequiresPolicy(t *testing.T) {
	e := newTimeEntryEnv(t)
	id := overnightDraft(t, e)

	_, err := e.svc.ConfirmTimeEntry(context.Background(), service.ConfirmTimeEntryInput{
		ID:       id,
		PeriodID: e.periodID,
	})
	if !errors.Is(err, service.ErrInvalidInput) {
		t.Fatalf("want ErrInvalidInput for missing overnight policy, got %v", err)
	}
}

func TestConfirmTimeEntry_OvernightAttributeToStart(t *testing.T) {
	e := newTimeEntryEnv(t)
	ctx := context.Background()
	id := overnightDraft(t, e)

	got, err := e.svc.ConfirmTimeEntry(ctx, service.ConfirmTimeEntryInput{
		ID:              id,
		PeriodID:        e.periodID,
		OvernightPolicy: service.OvernightAttributeToStart,
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 {
		t.Fatalf("want 1 entry, got %d", len(got))
	}
	if got[0].Attestation != "confirmed" || got[0].LocalWorkDate != "2026-06-01" || got[0].DurationMinutes != 240 {
		t.Fatalf("unexpected attribute-to-start: %+v", got[0])
	}
}

func TestConfirmTimeEntry_OvernightSplitAtMidnight(t *testing.T) {
	e := newTimeEntryEnv(t)
	ctx := context.Background()
	id := overnightDraft(t, e)

	got, err := e.svc.ConfirmTimeEntry(ctx, service.ConfirmTimeEntryInput{
		ID:              id,
		PeriodID:        e.periodID,
		OvernightPolicy: service.OvernightSplitAtMidnight,
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 {
		t.Fatalf("want 2 segments, got %d (%+v)", len(got), got)
	}
	if got[0].LocalWorkDate != "2026-06-01" || got[0].DurationMinutes != 120 || got[0].Attestation != "confirmed" {
		t.Fatalf("seg0: %+v", got[0])
	}
	if got[1].LocalWorkDate != "2026-06-02" || got[1].DurationMinutes != 120 || got[1].Attestation != "confirmed" {
		t.Fatalf("seg1: %+v", got[1])
	}

	orig, err := e.svc.GetTimeEntry(ctx, id, e.periodID)
	if err != nil {
		t.Fatal(err)
	}
	if orig.Attestation != "dismissed" {
		t.Fatalf("original should be dismissed, got %q", orig.Attestation)
	}
}

func TestConfirmTimeEntry_OverlapBlocksWithoutResolution(t *testing.T) {
	e := newTimeEntryEnv(t)
	ctx := context.Background()

	_, err := e.svc.CreateTimeEntry(ctx, service.TimeEntryInput{
		PeriodID:     e.periodID,
		Day:          "2026-06-01",
		StartMinutes: 9 * 60,
		EndMinutes:   11 * 60,
		Description:  "Theirs",
	})
	if err != nil {
		t.Fatal(err)
	}
	draftID := insertTimeEntryFull(t, e.q, e.periodID, "2026-06-01",
		"2026-06-01T14:00:00Z", "2026-06-01T16:00:00Z", // 10:00–12:00 Toronto
		sql.NullInt64{}, "Mine", "draft", false)

	_, err = e.svc.ConfirmTimeEntry(ctx, service.ConfirmTimeEntryInput{
		ID:       draftID,
		PeriodID: e.periodID,
	})
	if !errors.Is(err, service.ErrFailedPrecondition) {
		t.Fatalf("want ErrFailedPrecondition for overlap, got %v", err)
	}
}

func TestConfirmTimeEntry_OverlapAllowParallel(t *testing.T) {
	e := newTimeEntryEnv(t)
	ctx := context.Background()

	_, err := e.svc.CreateTimeEntry(ctx, service.TimeEntryInput{
		PeriodID:     e.periodID,
		Day:          "2026-06-01",
		StartMinutes: 9 * 60,
		EndMinutes:   11 * 60,
	})
	if err != nil {
		t.Fatal(err)
	}
	draftID := insertTimeEntryFull(t, e.q, e.periodID, "2026-06-01",
		"2026-06-01T14:00:00Z", "2026-06-01T16:00:00Z",
		sql.NullInt64{}, "Mine", "draft", false)

	got, err := e.svc.ConfirmTimeEntry(ctx, service.ConfirmTimeEntryInput{
		ID:                draftID,
		PeriodID:          e.periodID,
		OverlapResolution: service.OverlapAllowParallel,
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 || got[0].Attestation != "confirmed" {
		t.Fatalf("unexpected: %+v", got)
	}
}

func TestConfirmTimeEntry_OverlapKeepTheirsDismissesDraft(t *testing.T) {
	e := newTimeEntryEnv(t)
	ctx := context.Background()

	_, err := e.svc.CreateTimeEntry(ctx, service.TimeEntryInput{
		PeriodID:     e.periodID,
		Day:          "2026-06-01",
		StartMinutes: 9 * 60,
		EndMinutes:   11 * 60,
	})
	if err != nil {
		t.Fatal(err)
	}
	draftID := insertTimeEntryFull(t, e.q, e.periodID, "2026-06-01",
		"2026-06-01T14:00:00Z", "2026-06-01T16:00:00Z",
		sql.NullInt64{}, "Mine", "draft", false)

	got, err := e.svc.ConfirmTimeEntry(ctx, service.ConfirmTimeEntryInput{
		ID:                draftID,
		PeriodID:          e.periodID,
		OverlapResolution: service.OverlapKeepTheirs,
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 0 {
		t.Fatalf("keep_theirs should confirm nothing, got %+v", got)
	}
	orig, err := e.svc.GetTimeEntry(ctx, draftID, e.periodID)
	if err != nil {
		t.Fatal(err)
	}
	if orig.Attestation != "dismissed" {
		t.Fatalf("want dismissed, got %q", orig.Attestation)
	}
}

func TestConfirmTimeEntry_OverlapKeepMineDeletesTheirs(t *testing.T) {
	e := newTimeEntryEnv(t)
	ctx := context.Background()

	theirs, err := e.svc.CreateTimeEntry(ctx, service.TimeEntryInput{
		PeriodID:     e.periodID,
		Day:          "2026-06-01",
		StartMinutes: 9 * 60,
		EndMinutes:   11 * 60,
	})
	if err != nil {
		t.Fatal(err)
	}
	draftID := insertTimeEntryFull(t, e.q, e.periodID, "2026-06-01",
		"2026-06-01T14:00:00Z", "2026-06-01T16:00:00Z",
		sql.NullInt64{}, "Mine", "draft", false)

	got, err := e.svc.ConfirmTimeEntry(ctx, service.ConfirmTimeEntryInput{
		ID:                draftID,
		PeriodID:          e.periodID,
		OverlapResolution: service.OverlapKeepMine,
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 || got[0].Attestation != "confirmed" || got[0].ID != draftID {
		t.Fatalf("unexpected: %+v", got)
	}
	_, err = e.svc.GetTimeEntry(ctx, theirs.ID, e.periodID)
	if !errors.Is(err, service.ErrNotFound) {
		t.Fatalf("want theirs deleted, got %v", err)
	}
}

func TestConfirmTimeEntry_OverlapSplitKeepsNonOverlappingRemainder(t *testing.T) {
	e := newTimeEntryEnv(t)
	ctx := context.Background()

	// Confirmed 09:00–11:00 Toronto; draft 10:00–12:00 → remainder 11:00–12:00 (60m).
	_, err := e.svc.CreateTimeEntry(ctx, service.TimeEntryInput{
		PeriodID:     e.periodID,
		Day:          "2026-06-01",
		StartMinutes: 9 * 60,
		EndMinutes:   11 * 60,
	})
	if err != nil {
		t.Fatal(err)
	}
	draftID := insertTimeEntryFull(t, e.q, e.periodID, "2026-06-01",
		"2026-06-01T14:00:00Z", "2026-06-01T16:00:00Z",
		sql.NullInt64{}, "Mine", "draft", false)

	got, err := e.svc.ConfirmTimeEntry(ctx, service.ConfirmTimeEntryInput{
		ID:                draftID,
		PeriodID:          e.periodID,
		OverlapResolution: service.OverlapSplit,
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 || got[0].DurationMinutes != 60 || got[0].Attestation != "confirmed" {
		t.Fatalf("unexpected remainder: %+v", got)
	}
	orig, err := e.svc.GetTimeEntry(ctx, draftID, e.periodID)
	if err != nil {
		t.Fatal(err)
	}
	if orig.Attestation != "dismissed" {
		t.Fatalf("original should be dismissed, got %q", orig.Attestation)
	}
}

func TestSplitTimeEntry_CutPointProducesDraftSegments(t *testing.T) {
	e := newTimeEntryEnv(t)
	ctx := context.Background()

	draftID := insertTimeEntryFull(t, e.q, e.periodID, "2026-06-01",
		"2026-06-01T13:00:00Z", "2026-06-01T17:00:00Z", // 09:00–13:00 Toronto, 4h
		sql.NullInt64{}, "Long", "draft", false)

	got, err := e.svc.SplitTimeEntry(ctx, service.SplitTimeEntryInput{
		ID:        draftID,
		PeriodID:  e.periodID,
		CutPoints: []string{"2026-06-01T15:00:00Z"}, // 11:00 local
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 {
		t.Fatalf("want 2 drafts, got %d", len(got))
	}
	if got[0].DurationMinutes != 120 || got[0].Attestation != "draft" {
		t.Fatalf("seg0: %+v", got[0])
	}
	if got[1].DurationMinutes != 120 || got[1].Attestation != "draft" {
		t.Fatalf("seg1: %+v", got[1])
	}
	orig, err := e.svc.GetTimeEntry(ctx, draftID, e.periodID)
	if err != nil {
		t.Fatal(err)
	}
	if orig.Attestation != "dismissed" {
		t.Fatalf("original should be dismissed, got %q", orig.Attestation)
	}
}
