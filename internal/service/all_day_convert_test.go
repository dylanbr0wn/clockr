package service_test

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"testing"

	"github.com/dylanbr0wn/shiet/internal/db/sqlc"
	"github.com/dylanbr0wn/shiet/internal/service"
)

func TestConvertAllDayEvent_CreatesCalendarConvertDraft(t *testing.T) {
	e := newSyncEnv(t)
	ctx := context.Background()

	inc := service.IncomingEvent{
		CalendarID: e.calID,
		Provider:   service.ProviderGoogle,
		ExternalID: "allday-1",
		Title:      "Company Holiday",
		Status:     "accepted",
		AllDay:     true,
		StartDate:  "2026-06-03",
		EndDate:    "2026-06-04",
	}
	if _, err := e.svc.SyncEvents(ctx, e.periodID, []service.IncomingEvent{inc}); err != nil {
		t.Fatal(err)
	}
	events, err := e.svc.ListEvents(ctx, e.periodID)
	if err != nil || len(events) != 1 {
		t.Fatalf("list events: %#v err=%v", events, err)
	}
	ev := events[0]

	got, err := e.svc.ConvertAllDayEvent(ctx, service.ConvertAllDayEventInput{
		EventID: ev.ID,
		TimeEntryInput: service.TimeEntryInput{
			PeriodID:     e.periodID,
			Day:          "2026-06-03",
			StartMinutes: 9 * 60,
			EndMinutes:   17 * 60,
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if got.Attestation != "draft" {
		t.Fatalf("attestation = %q, want draft", got.Attestation)
	}
	if got.Method != service.MethodCalendarConvert {
		t.Fatalf("method = %q, want %q", got.Method, service.MethodCalendarConvert)
	}
	if got.Description != "Company Holiday" {
		t.Fatalf("description = %q, want event title", got.Description)
	}
	if got.DurationMinutes != 8*60 || got.LocalWorkDate != "2026-06-03" {
		t.Fatalf("span: %+v", got)
	}

	row, err := e.q.GetTimeEntry(ctx, sqlc.GetTimeEntryParams{ID: got.ID, PeriodID: e.periodID})
	if err != nil {
		t.Fatal(err)
	}
	wantID := fmt.Sprintf("google|%s|allday-1|", strconv.FormatInt(e.calID, 10))
	if !row.SourceKind.Valid || row.SourceKind.String != service.SourceKindCalendarEvent {
		t.Fatalf("source_kind: %+v", row.SourceKind)
	}
	if !row.SourceID.Valid || row.SourceID.String != wantID {
		t.Fatalf("source_id = %q, want %q", row.SourceID.String, wantID)
	}
	if !row.SourceRevision.Valid || row.SourceRevision.String == "" {
		t.Fatalf("source_revision missing: %+v", row.SourceRevision)
	}
	if !row.Method.Valid || row.Method.String != service.MethodCalendarConvert {
		t.Fatalf("row method: %+v", row.Method)
	}
}

func TestConvertAllDayEvent_RejectsTimedEvent(t *testing.T) {
	e := newSyncEnv(t)
	ctx := context.Background()
	if _, err := e.svc.SyncEvents(ctx, e.periodID, []service.IncomingEvent{e.baseEvent()}); err != nil {
		t.Fatal(err)
	}
	events, err := e.svc.ListEvents(ctx, e.periodID)
	if err != nil || len(events) != 1 {
		t.Fatalf("list events: %#v err=%v", events, err)
	}

	_, err = e.svc.ConvertAllDayEvent(ctx, service.ConvertAllDayEventInput{
		EventID: events[0].ID,
		TimeEntryInput: service.TimeEntryInput{
			PeriodID:     e.periodID,
			Day:          "2026-06-02",
			StartMinutes: 9 * 60,
			EndMinutes:   10 * 60,
		},
	})
	if err == nil {
		t.Fatal("expected error for timed event")
	}
	if !errors.Is(err, service.ErrFailedPrecondition) {
		t.Fatalf("want ErrFailedPrecondition, got %v", err)
	}
}

func TestConvertAllDayEvent_RejectsWhenLiveDraftExists(t *testing.T) {
	e := newSyncEnv(t)
	ctx := context.Background()
	inc := service.IncomingEvent{
		CalendarID: e.calID,
		Provider:   service.ProviderGoogle,
		ExternalID: "allday-dup",
		Title:      "Offsite",
		Status:     "accepted",
		AllDay:     true,
		StartDate:  "2026-06-05",
		EndDate:    "2026-06-06",
	}
	if _, err := e.svc.SyncEvents(ctx, e.periodID, []service.IncomingEvent{inc}); err != nil {
		t.Fatal(err)
	}
	events, err := e.svc.ListEvents(ctx, e.periodID)
	if err != nil || len(events) != 1 {
		t.Fatalf("list events: %#v err=%v", events, err)
	}
	input := service.ConvertAllDayEventInput{
		EventID: events[0].ID,
		TimeEntryInput: service.TimeEntryInput{
			PeriodID:     e.periodID,
			Day:          "2026-06-05",
			StartMinutes: 10 * 60,
			EndMinutes:   12 * 60,
		},
	}
	if _, err := e.svc.ConvertAllDayEvent(ctx, input); err != nil {
		t.Fatal(err)
	}
	_, err = e.svc.ConvertAllDayEvent(ctx, input)
	if err == nil {
		t.Fatal("expected error on second convert")
	}
	if !errors.Is(err, service.ErrFailedPrecondition) {
		t.Fatalf("want ErrFailedPrecondition, got %v", err)
	}
}

func TestConvertAllDayEvent_RejectsExcludedEvent(t *testing.T) {
	e := newSyncEnv(t)
	ctx := context.Background()
	inc := service.IncomingEvent{
		CalendarID: e.calID,
		Provider:   service.ProviderGoogle,
		ExternalID: "allday-ex",
		Title:      "Hidden day",
		Status:     "accepted",
		AllDay:     true,
		StartDate:  "2026-06-07",
		EndDate:    "2026-06-08",
	}
	if _, err := e.svc.SyncEvents(ctx, e.periodID, []service.IncomingEvent{inc}); err != nil {
		t.Fatal(err)
	}
	events, err := e.svc.ListEvents(ctx, e.periodID)
	if err != nil || len(events) != 1 {
		t.Fatalf("list events: %#v err=%v", events, err)
	}
	if _, err := e.svc.ExcludeEvent(ctx, service.ExcludeEventInput{
		EventID:  events[0].ID,
		PeriodID: e.periodID,
	}); err != nil {
		t.Fatal(err)
	}

	_, err = e.svc.ConvertAllDayEvent(ctx, service.ConvertAllDayEventInput{
		EventID: events[0].ID,
		TimeEntryInput: service.TimeEntryInput{
			PeriodID:     e.periodID,
			Day:          "2026-06-07",
			StartMinutes: 9 * 60,
			EndMinutes:   10 * 60,
		},
	})
	if err == nil {
		t.Fatal("expected error for excluded event")
	}
	if !errors.Is(err, service.ErrFailedPrecondition) {
		t.Fatalf("want ErrFailedPrecondition, got %v", err)
	}
}
