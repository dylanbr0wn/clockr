package service_test

import (
	"context"
	"database/sql"
	"testing"

	"github.com/dylanbr0wn/clockr/internal/db/sqlc"
	"github.com/dylanbr0wn/clockr/internal/service"
)

func TestResolveReviewItem_DeletedCategorizedDrop(t *testing.T) {
	e := newSyncEnv(t)
	ctx := context.Background()

	if _, err := e.svc.SyncEvents(ctx, e.periodID, []service.IncomingEvent{e.baseEvent()}); err != nil {
		t.Fatal(err)
	}
	mustOverlay(t, e, "evt-1")

	if _, err := e.svc.SyncEvents(ctx, e.periodID, []service.IncomingEvent{}); err != nil {
		t.Fatal(err)
	}
	items := openItems(t, e)
	if len(items) != 1 {
		t.Fatalf("want 1 review item, got %d", len(items))
	}

	if _, err := e.svc.ResolveReviewItem(ctx, service.ResolveReviewItemInput{
		ReviewItemID: items[0].ID,
		Action:       service.ReviewActionDropEntry,
	}); err != nil {
		t.Fatal(err)
	}

	if open := openItems(t, e); len(open) != 0 {
		t.Fatalf("item should be resolved, got %d open", len(open))
	}
	events, err := e.svc.ListEvents(ctx, e.periodID)
	if err != nil {
		t.Fatal(err)
	}
	if len(events) != 0 {
		t.Fatalf("event should be deleted, got %+v", events)
	}
}

func TestResolveReviewItem_DeletedCategorizedKeep(t *testing.T) {
	e := newSyncEnv(t)
	ctx := context.Background()

	if _, err := e.svc.SyncEvents(ctx, e.periodID, []service.IncomingEvent{e.baseEvent()}); err != nil {
		t.Fatal(err)
	}
	mustOverlay(t, e, "evt-1")
	if _, err := e.svc.SyncEvents(ctx, e.periodID, []service.IncomingEvent{}); err != nil {
		t.Fatal(err)
	}
	items := openItems(t, e)

	if _, err := e.svc.ResolveReviewItem(ctx, service.ResolveReviewItemInput{
		ReviewItemID: items[0].ID,
		Action:       service.ReviewActionKeepEntry,
	}); err != nil {
		t.Fatal(err)
	}

	if open := openItems(t, e); len(open) != 0 {
		t.Fatalf("item should be resolved, got %d open", len(open))
	}
	events, err := e.svc.ListEvents(ctx, e.periodID)
	if err != nil {
		t.Fatal(err)
	}
	if len(events) != 1 {
		t.Fatalf("event should remain, got %+v", events)
	}
}

func TestResolveReviewItem_TitleChangedAccept(t *testing.T) {
	e := newSyncEnv(t)
	ctx := context.Background()

	if _, err := e.svc.SyncEvents(ctx, e.periodID, []service.IncomingEvent{e.baseEvent()}); err != nil {
		t.Fatal(err)
	}
	mustOverlay(t, e, "evt-1")

	renamed := e.baseEvent()
	renamed.Title = "Sprint Planning"
	if _, err := e.svc.SyncEvents(ctx, e.periodID, []service.IncomingEvent{renamed}); err != nil {
		t.Fatal(err)
	}
	items := openItems(t, e)

	if _, err := e.svc.ResolveReviewItem(ctx, service.ResolveReviewItemInput{
		ReviewItemID: items[0].ID,
		Action:       service.ReviewActionAccept,
	}); err != nil {
		t.Fatal(err)
	}

	if open := openItems(t, e); len(open) != 0 {
		t.Fatalf("item should be resolved, got %d open", len(open))
	}
}

func TestResolveReviewItem_NewInGapUseEvent(t *testing.T) {
	e := newSyncEnv(t)
	ctx := context.Background()

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
	inc.End = tm("2026-06-02T14:30:00Z")
	if _, err := e.svc.SyncEvents(ctx, e.periodID, []service.IncomingEvent{inc}); err != nil {
		t.Fatal(err)
	}
	items := openItems(t, e)

	if _, err := e.svc.ResolveReviewItem(ctx, service.ResolveReviewItemInput{
		ReviewItemID: items[0].ID,
		Action:       service.ReviewActionUseEvent,
	}); err != nil {
		t.Fatal(err)
	}

	fills, err := e.svc.ListGapFills(ctx, e.periodID)
	if err != nil {
		t.Fatal(err)
	}
	if len(fills) != 1 {
		t.Fatalf("want 1 shrunk gap fill, got %+v", fills)
	}
	if fills[0].Start != "2026-06-02T13:00:00Z" || fills[0].End != "2026-06-02T13:30:00Z" {
		t.Fatalf("unexpected gap fill span: %+v", fills[0])
	}
	events, err := e.svc.ListEvents(ctx, e.periodID)
	if err != nil {
		t.Fatal(err)
	}
	if len(events) != 1 {
		t.Fatalf("event should remain, got %+v", events)
	}
}

func TestResolveReviewItem_TentativeExclude(t *testing.T) {
	e := newSyncEnv(t)
	ctx := context.Background()

	tentative := e.baseEvent()
	tentative.ExternalID = "evt-tent"
	tentative.Status = "tentative"
	if _, err := e.svc.SyncEvents(ctx, e.periodID, []service.IncomingEvent{tentative}); err != nil {
		t.Fatal(err)
	}
	items := openItems(t, e)

	if _, err := e.svc.ResolveReviewItem(ctx, service.ResolveReviewItemInput{
		ReviewItemID: items[0].ID,
		Action:       service.ReviewActionExclude,
	}); err != nil {
		t.Fatal(err)
	}

	events, err := e.svc.ListEvents(ctx, e.periodID)
	if err != nil {
		t.Fatal(err)
	}
	if len(events) != 0 {
		t.Fatalf("event should be removed, got %+v", events)
	}
}
