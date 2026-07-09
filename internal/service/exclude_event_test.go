package service_test

import (
	"context"
	"errors"
	"testing"

	"github.com/dylanbr0wn/shiet/internal/service"
)

func TestExcludeEvent_HidesTimedEventAcrossSync(t *testing.T) {
	e := newSyncEnv(t)
	ctx := context.Background()

	inc := e.baseEvent()
	if _, err := e.svc.SyncEvents(ctx, e.periodID, []service.IncomingEvent{inc}); err != nil {
		t.Fatal(err)
	}
	events, err := e.svc.ListEvents(ctx, e.periodID)
	if err != nil {
		t.Fatal(err)
	}
	if len(events) != 1 {
		t.Fatalf("want 1 event before exclude, got %d", len(events))
	}

	if _, err := e.svc.ExcludeEvent(ctx, service.ExcludeEventInput{
		EventID:  events[0].ID,
		PeriodID: e.periodID,
	}); err != nil {
		t.Fatal(err)
	}

	events, err = e.svc.ListEvents(ctx, e.periodID)
	if err != nil {
		t.Fatal(err)
	}
	if len(events) != 0 {
		t.Fatalf("excluded timed event should be hidden, got %+v", events)
	}

	if _, err := e.svc.SyncEvents(ctx, e.periodID, []service.IncomingEvent{inc}); err != nil {
		t.Fatal(err)
	}
	events, err = e.svc.ListEvents(ctx, e.periodID)
	if err != nil {
		t.Fatal(err)
	}
	if len(events) != 0 {
		t.Fatalf("excluded timed event should stay hidden across sync, got %+v", events)
	}
}

func TestExcludeEvent_HidesAllDayEventAcrossSync(t *testing.T) {
	e := newSyncEnv(t)
	ctx := context.Background()

	allDay := service.IncomingEvent{
		CalendarID: e.calID, Provider: service.ProviderGoogle, ExternalID: "evt-allday-hide",
		Title: "Holiday", Status: "accepted", AllDay: true,
		StartDate: "2026-06-03", EndDate: "2026-06-04",
	}
	if _, err := e.svc.SyncEvents(ctx, e.periodID, []service.IncomingEvent{allDay}); err != nil {
		t.Fatal(err)
	}
	events, err := e.svc.ListEvents(ctx, e.periodID)
	if err != nil {
		t.Fatal(err)
	}
	if len(events) != 1 || !events[0].AllDay {
		t.Fatalf("want 1 all-day event before exclude, got %+v", events)
	}

	if _, err := e.svc.ExcludeEvent(ctx, service.ExcludeEventInput{
		EventID:  events[0].ID,
		PeriodID: e.periodID,
	}); err != nil {
		t.Fatal(err)
	}

	events, err = e.svc.ListEvents(ctx, e.periodID)
	if err != nil {
		t.Fatal(err)
	}
	if len(events) != 0 {
		t.Fatalf("excluded all-day event should be hidden, got %+v", events)
	}

	if _, err := e.svc.SyncEvents(ctx, e.periodID, []service.IncomingEvent{allDay}); err != nil {
		t.Fatal(err)
	}
	events, err = e.svc.ListEvents(ctx, e.periodID)
	if err != nil {
		t.Fatal(err)
	}
	if len(events) != 0 {
		t.Fatalf("excluded all-day event should stay hidden across sync, got %+v", events)
	}
}

func TestExcludeEvent_WrongPeriod(t *testing.T) {
	e := newSyncEnv(t)
	ctx := context.Background()

	if _, err := e.svc.SyncEvents(ctx, e.periodID, []service.IncomingEvent{e.baseEvent()}); err != nil {
		t.Fatal(err)
	}
	events, err := e.svc.ListEvents(ctx, e.periodID)
	if err != nil {
		t.Fatal(err)
	}

	_, err = e.svc.ExcludeEvent(ctx, service.ExcludeEventInput{
		EventID:  events[0].ID,
		PeriodID: e.periodID + 999,
	})
	if !errors.Is(err, service.ErrNotFound) {
		t.Fatalf("want ErrNotFound for wrong period, got %v", err)
	}
}
