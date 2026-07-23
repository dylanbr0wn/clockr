package service_test

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"testing"

	"github.com/dylanbr0wn/shiet/internal/service"
)

func TestResolveReviewDecision_SourceDriftSpawnDraft(t *testing.T) {
	e := newSyncEnv(t)
	ctx := context.Background()

	entryID, _ := seedConfirmedCalendarDrift(t, e)

	items := openDecisions(t, e)
	if len(items) != 1 || items[0].Kind != "source_drift" {
		t.Fatalf("want source_drift, got %+v", items)
	}

	if _, err := e.svc.ResolveReviewDecision(ctx, service.ResolveReviewDecisionInput{
		DecisionID: items[0].ID,
		Action:     service.ReviewActionSpawnDraft,
	}); err != nil {
		t.Fatal(err)
	}

	if open := openDecisions(t, e); len(open) != 0 {
		t.Fatalf("review should resolve, got %d open", len(open))
	}

	confirmed, err := e.svc.GetTimeEntry(ctx, entryID, e.periodID)
	if err != nil {
		t.Fatal(err)
	}
	if confirmed.Attestation != "confirmed" || confirmed.Start != "2026-06-02T09:00:00Z" {
		t.Fatalf("confirmed mutated: %+v", confirmed)
	}

	entries, err := e.svc.ListTimeEntries(ctx, e.periodID)
	if err != nil {
		t.Fatal(err)
	}
	var draft *service.TimeEntry
	for i := range entries {
		if entries[i].Attestation == "draft" {
			draft = &entries[i]
			break
		}
	}
	if draft == nil {
		t.Fatalf("expected draft proposal, got %+v", entries)
	}
	if draft.Start != "2026-06-02T10:00:00Z" || draft.End != "2026-06-02T10:30:00Z" {
		t.Fatalf("draft should mirror drifted event span, got start=%s end=%s", draft.Start, draft.End)
	}
	if draft.Method != "calendar_import" {
		t.Fatalf("draft method = %q, want calendar_import", draft.Method)
	}
}

func TestResolveReviewDecision_SourceDriftDismiss(t *testing.T) {
	e := newSyncEnv(t)
	ctx := context.Background()

	entryID, _ := seedConfirmedCalendarDrift(t, e)
	items := openDecisions(t, e)
	if len(items) != 1 {
		t.Fatalf("want 1 review, got %d", len(items))
	}

	if _, err := e.svc.ResolveReviewDecision(ctx, service.ResolveReviewDecisionInput{
		DecisionID: items[0].ID,
		Action:     service.ReviewActionDismiss,
	}); err != nil {
		t.Fatal(err)
	}
	if open := openDecisions(t, e); len(open) != 0 {
		t.Fatalf("want dismissed, got %d open", len(open))
	}
	entries, err := e.svc.ListTimeEntries(ctx, e.periodID)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 1 || entries[0].ID != entryID || entries[0].Attestation != "confirmed" {
		t.Fatalf("dismiss should leave only confirmed, got %+v", entries)
	}
}

func TestResolveReviewDecision_SourceDriftReplaceUsesEventTitle(t *testing.T) {
	e := newSyncEnv(t)
	ctx := context.Background()

	_, _ = seedConfirmedCalendarDrift(t, e)
	items := openDecisions(t, e)
	if _, err := e.svc.ResolveReviewDecision(ctx, service.ResolveReviewDecisionInput{
		DecisionID: items[0].ID,
		Action:     service.ReviewActionReplace,
	}); err != nil {
		t.Fatal(err)
	}
	entries, err := e.svc.ListTimeEntries(ctx, e.periodID)
	if err != nil {
		t.Fatal(err)
	}
	var draft *service.TimeEntry
	for i := range entries {
		if entries[i].Attestation == "draft" {
			draft = &entries[i]
			break
		}
	}
	if draft == nil {
		t.Fatal("expected draft")
	}
	if draft.Description != "Standup" {
		t.Fatalf("replace draft description = %q, want event title Standup", draft.Description)
	}
}

func TestResolveReviewDecision_SourceDriftSpawnKeepsConfirmedDescription(t *testing.T) {
	e := newSyncEnv(t)
	ctx := context.Background()

	_, _ = seedConfirmedCalendarDrift(t, e)
	items := openDecisions(t, e)
	if _, err := e.svc.ResolveReviewDecision(ctx, service.ResolveReviewDecisionInput{
		DecisionID: items[0].ID,
		Action:     service.ReviewActionSpawnDraft,
	}); err != nil {
		t.Fatal(err)
	}
	entries, err := e.svc.ListTimeEntries(ctx, e.periodID)
	if err != nil {
		t.Fatal(err)
	}
	var draft *service.TimeEntry
	for i := range entries {
		if entries[i].Attestation == "draft" {
			draft = &entries[i]
			break
		}
	}
	if draft == nil {
		t.Fatal("expected draft")
	}
	if draft.Description != "My notes" {
		t.Fatalf("spawn draft description = %q, want confirmed description", draft.Description)
	}
}

func TestSync_SourceDriftOpenUpdatesPayloadOnFurtherChange(t *testing.T) {
	e := newSyncEnv(t)
	ctx := context.Background()

	_, _ = seedConfirmedCalendarDrift(t, e)
	items := openDecisions(t, e)
	if len(items) != 1 {
		t.Fatalf("want open source_drift, got %d", len(items))
	}

	movedAgain := e.baseEvent()
	movedAgain.Start = tm("2026-06-02T11:00:00Z")
	movedAgain.End = tm("2026-06-02T11:30:00Z")
	if _, err := e.svc.SyncEvents(ctx, e.periodID, []service.IncomingEvent{movedAgain}); err != nil {
		t.Fatal(err)
	}
	after := openDecisions(t, e)
	if len(after) != 1 {
		t.Fatalf("want still one open source_drift, got %d", len(after))
	}
	row, err := e.q.GetReviewItem(ctx, after[0].ID)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(row.Payload, `"source_revision"`) {
		t.Fatalf("payload missing revision: %s", row.Payload)
	}
	// Event fact should reflect latest move; confirmed TE unchanged.
	events, err := e.q.ListAllEventsForPeriod(ctx, e.periodID)
	if err != nil || len(events) != 1 {
		t.Fatalf("events: err=%v n=%d", err, len(events))
	}
	if events[0].StartUtc.String != "2026-06-02T11:00:00Z" {
		t.Fatalf("event start = %s", events[0].StartUtc.String)
	}
}

func TestSync_VanishedConfirmedCalendarSourceOpensSourceDrift(t *testing.T) {
	e := newSyncEnv(t)
	ctx := context.Background()

	if _, err := e.svc.SyncEvents(ctx, e.periodID, []service.IncomingEvent{e.baseEvent()}); err != nil {
		t.Fatal(err)
	}
	events, err := e.q.ListAllEventsForPeriod(ctx, e.periodID)
	if err != nil || len(events) != 1 {
		t.Fatalf("seed event: err=%v n=%d", err, len(events))
	}
	sourceID := fmt.Sprintf("%s|%d|evt-1|", service.ProviderGoogle, e.calID)
	entryID := insertTimeEntryProvenance(t, e.q, e.periodID,
		"2026-06-02", "2026-06-02T09:00:00Z", "2026-06-02T09:30:00Z",
		sql.NullInt64{Int64: e.catID, Valid: true}, "Standup", "confirmed", false,
		"calendar_event", sourceID, events[0].SourceHash,
	)

	r, err := e.svc.SyncEvents(ctx, e.periodID, []service.IncomingEvent{})
	if err != nil {
		t.Fatal(err)
	}
	if r.Flagged != 1 {
		t.Fatalf("want flagged source_drift, got %+v", r)
	}
	items := openDecisions(t, e)
	if len(items) != 1 || items[0].Kind != "source_drift" {
		t.Fatalf("want source_drift, got %+v", items)
	}
	got, err := e.svc.GetTimeEntry(ctx, entryID, e.periodID)
	if err != nil {
		t.Fatal(err)
	}
	if got.Attestation != "confirmed" {
		t.Fatalf("confirmed mutated: %+v", got)
	}
	listed, err := e.svc.ListEvents(ctx, e.periodID)
	if err != nil {
		t.Fatal(err)
	}
	if len(listed) != 1 {
		t.Fatalf("event fact should be retained for drift review, got %+v", listed)
	}
}

// seedConfirmedCalendarDrift syncs an event (materializing a draft), confirms it,
// then re-syncs a moved event to open source_drift.
func seedConfirmedCalendarDrift(t *testing.T, e *syncEnv) (entryID int64, sourceID string) {
	t.Helper()
	ctx := context.Background()
	if _, err := e.svc.SyncEvents(ctx, e.periodID, []service.IncomingEvent{e.baseEvent()}); err != nil {
		t.Fatal(err)
	}
	entries, err := e.svc.ListTimeEntries(ctx, e.periodID)
	if err != nil {
		t.Fatal(err)
	}
	var draft *service.TimeEntry
	for i := range entries {
		if entries[i].Attestation == "draft" && entries[i].Method == service.MethodCalendarImport {
			draft = &entries[i]
			break
		}
	}
	if draft == nil {
		t.Fatal("expected materialize draft after sync")
	}
	cat := e.catID
	if _, err := e.svc.AdjustDraftTimeEntry(ctx, service.TimeEntryUpdateInput{
		ID: draft.ID,
		TimeEntryInput: service.TimeEntryInput{
			PeriodID:     e.periodID,
			Day:          draft.LocalWorkDate,
			StartMinutes: 5 * 60, // 09:00Z = 05:00 Toronto
			EndMinutes:   5*60 + 30,
			CategoryID:   &cat,
			Description:  "My notes",
		},
	}); err != nil {
		t.Fatal(err)
	}
	confirmed, err := e.svc.ConfirmTimeEntry(ctx, service.ConfirmTimeEntryInput{
		ID:       draft.ID,
		PeriodID: e.periodID,
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(confirmed) != 1 {
		t.Fatalf("want 1 confirmed, got %d", len(confirmed))
	}
	entryID = confirmed[0].ID
	sourceID = fmt.Sprintf("%s|%d|evt-1|", service.ProviderGoogle, e.calID)

	moved := e.baseEvent()
	moved.Start = tm("2026-06-02T10:00:00Z")
	moved.End = tm("2026-06-02T10:30:00Z")
	if _, err := e.svc.SyncEvents(ctx, e.periodID, []service.IncomingEvent{moved}); err != nil {
		t.Fatal(err)
	}
	return entryID, sourceID
}

func TestResolveReviewDecision_DeletedCategorizedDrop(t *testing.T) {
	e := newSyncEnv(t)
	ctx := context.Background()

	if _, err := e.svc.SyncEvents(ctx, e.periodID, []service.IncomingEvent{e.baseEvent()}); err != nil {
		t.Fatal(err)
	}
	mustOverlay(t, e, "evt-1")

	if _, err := e.svc.SyncEvents(ctx, e.periodID, []service.IncomingEvent{}); err != nil {
		t.Fatal(err)
	}
	items := openDecisions(t, e)
	if len(items) != 1 {
		t.Fatalf("want 1 review item, got %d", len(items))
	}

	if _, err := e.svc.ResolveReviewDecision(ctx, service.ResolveReviewDecisionInput{
		DecisionID: items[0].ID,
		Action:       service.ReviewActionDropEntry,
	}); err != nil {
		t.Fatal(err)
	}

	if open := openDecisions(t, e); len(open) != 0 {
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

func TestResolveReviewDecision_DeletedCategorizedKeep(t *testing.T) {
	e := newSyncEnv(t)
	ctx := context.Background()

	if _, err := e.svc.SyncEvents(ctx, e.periodID, []service.IncomingEvent{e.baseEvent()}); err != nil {
		t.Fatal(err)
	}
	mustOverlay(t, e, "evt-1")
	if _, err := e.svc.SyncEvents(ctx, e.periodID, []service.IncomingEvent{}); err != nil {
		t.Fatal(err)
	}
	items := openDecisions(t, e)

	if _, err := e.svc.ResolveReviewDecision(ctx, service.ResolveReviewDecisionInput{
		DecisionID: items[0].ID,
		Action:       service.ReviewActionKeepEntry,
	}); err != nil {
		t.Fatal(err)
	}

	if open := openDecisions(t, e); len(open) != 0 {
		t.Fatalf("item should be resolved, got %d open", len(open))
	}
	events, err := e.svc.ListEvents(ctx, e.periodID)
	if err != nil {
		t.Fatal(err)
	}
	if len(events) != 0 {
		t.Fatalf("calendar event should be hidden after manual conversion, got %+v", events)
	}
	fills, err := e.svc.ListTimeEntries(ctx, e.periodID)
	if err != nil {
		t.Fatal(err)
	}
	var live []service.TimeEntry
	for _, te := range fills {
		if te.Attestation != "dismissed" {
			live = append(live, te)
		}
	}
	if len(live) != 1 {
		t.Fatalf("want one manual copy, got %+v", fills)
	}
	if live[0].Method != "" || live[0].Description != "Standup" || live[0].CategoryID == nil || *live[0].CategoryID != e.catID {
		t.Fatalf("manual copy did not preserve event title/category: %+v", live[0])
	}
	if err := e.svc.DeleteTimeEntry(ctx, service.TimeEntryDeleteInput{ID: live[0].ID, PeriodID: e.periodID}); err != nil {
		t.Fatalf("manual copy should be deletable: %v", err)
	}
	fills, err = e.svc.ListTimeEntries(ctx, e.periodID)
	if err != nil {
		t.Fatal(err)
	}
	live = live[:0]
	for _, te := range fills {
		if te.Attestation != "dismissed" {
			live = append(live, te)
		}
	}
	if len(live) != 0 {
		t.Fatalf("manual copy should be deleted, got %+v", fills)
	}

	if _, err := e.svc.SyncEvents(ctx, e.periodID, []service.IncomingEvent{}); err != nil {
		t.Fatal(err)
	}
	if open := openDecisions(t, e); len(open) != 0 {
		t.Fatalf("resolved deleted event should not requeue, got %d open", len(open))
	}
}

func TestResolveReviewDecision_TitleChangedAccept(t *testing.T) {
	e := newSyncEnv(t)
	ctx := context.Background()

	if _, err := e.svc.SyncEvents(ctx, e.periodID, []service.IncomingEvent{e.baseEvent()}); err != nil {
		t.Fatal(err)
	}
	mustOverlay(t, e, "evt-1")
	dismissCalendarDrafts(t, e)

	renamed := e.baseEvent()
	renamed.Title = "Sprint Planning"
	if _, err := e.svc.SyncEvents(ctx, e.periodID, []service.IncomingEvent{renamed}); err != nil {
		t.Fatal(err)
	}
	items := openDecisions(t, e)
	if len(items) != 1 || items[0].Kind != "title_changed" {
		t.Fatalf("want title_changed, got %+v", items)
	}

	if _, err := e.svc.ResolveReviewDecision(ctx, service.ResolveReviewDecisionInput{
		DecisionID: items[0].ID,
		Action:       service.ReviewActionAccept,
	}); err != nil {
		t.Fatal(err)
	}

	if open := openDecisions(t, e); len(open) != 0 {
		t.Fatalf("item should be resolved, got %d open", len(open))
	}
}

func TestResolveReviewDecision_NewInGapUseEvent(t *testing.T) {
	e := newSyncEnv(t)
	ctx := context.Background()

	insertTimeEntry(t, e.q, e.periodID, "2026-06-02", "2026-06-02T13:00:00Z", "2026-06-02T14:00:00Z", sql.NullInt64{Int64: e.catID, Valid: true}, "", true)

	inc := e.baseEvent()
	inc.ExternalID = "evt-overlap"
	inc.Start = tm("2026-06-02T13:30:00Z")
	inc.End = tm("2026-06-02T14:30:00Z")
	if _, err := e.svc.SyncEvents(ctx, e.periodID, []service.IncomingEvent{inc}); err != nil {
		t.Fatal(err)
	}
	items := openDecisions(t, e)

	if _, err := e.svc.ResolveReviewDecision(ctx, service.ResolveReviewDecisionInput{
		DecisionID: items[0].ID,
		Action:       service.ReviewActionUseEvent,
	}); err != nil {
		t.Fatal(err)
	}

	fills, err := e.svc.ListTimeEntries(ctx, e.periodID)
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

func TestResolveReviewDecision_NewInGapKeepGapPersistsAcrossSync(t *testing.T) {
	e := newSyncEnv(t)
	ctx := context.Background()

	insertTimeEntry(t, e.q, e.periodID, "2026-06-02", "2026-06-02T13:00:00Z", "2026-06-02T14:00:00Z", sql.NullInt64{Int64: e.catID, Valid: true}, "", true)

	inc := e.baseEvent()
	inc.ExternalID = "evt-overlap"
	inc.Start = tm("2026-06-02T13:30:00Z")
	inc.End = tm("2026-06-02T14:30:00Z")
	if _, err := e.svc.SyncEvents(ctx, e.periodID, []service.IncomingEvent{inc}); err != nil {
		t.Fatal(err)
	}
	items := openDecisions(t, e)
	if len(items) != 1 {
		t.Fatalf("want 1 review item, got %d", len(items))
	}

	if _, err := e.svc.ResolveReviewDecision(ctx, service.ResolveReviewDecisionInput{
		DecisionID: items[0].ID,
		Action:       service.ReviewActionKeepGap,
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := e.svc.SyncEvents(ctx, e.periodID, []service.IncomingEvent{inc}); err != nil {
		t.Fatal(err)
	}

	if open := openDecisions(t, e); len(open) != 0 {
		t.Fatalf("resolved gap conflict should not requeue, got %d open", len(open))
	}
	events, err := e.svc.ListEvents(ctx, e.periodID)
	if err != nil {
		t.Fatal(err)
	}
	if len(events) != 0 {
		t.Fatalf("keep-gap decision should continue removing event, got %+v", events)
	}
}

func TestResolveReviewDecision_AllDayExcludeKeepsEventVisible(t *testing.T) {
	e := newSyncEnv(t)
	ctx := context.Background()

	allDay := service.IncomingEvent{
		CalendarID: e.calID, Provider: service.ProviderGoogle, ExternalID: "evt-allday", Title: "Holiday",
		Status: "accepted", AllDay: true, StartDate: "2026-06-03", EndDate: "2026-06-04",
	}
	if _, err := e.svc.SyncEvents(ctx, e.periodID, []service.IncomingEvent{allDay}); err != nil {
		t.Fatal(err)
	}
	items := openDecisions(t, e)
	if len(items) != 1 || items[0].Kind != "all_day" {
		t.Fatalf("want one all_day review item, got %+v", items)
	}

	if _, err := e.svc.ResolveReviewDecision(ctx, service.ResolveReviewDecisionInput{
		DecisionID: items[0].ID,
		Action:       service.ReviewActionExclude,
	}); err != nil {
		t.Fatal(err)
	}

	events, err := e.svc.ListEvents(ctx, e.periodID)
	if err != nil {
		t.Fatal(err)
	}
	if len(events) != 1 || !events[0].AllDay || events[0].StartDate != "2026-06-03" {
		t.Fatalf("all-day event should stay visible after exclude, got %+v", events)
	}

	if _, err := e.svc.SyncEvents(ctx, e.periodID, []service.IncomingEvent{allDay}); err != nil {
		t.Fatal(err)
	}
	events, err = e.svc.ListEvents(ctx, e.periodID)
	if err != nil {
		t.Fatal(err)
	}
	if len(events) != 1 {
		t.Fatalf("all-day event should remain visible across re-sync, got %+v", events)
	}
}

func TestResolveReviewDecision_TentativeExclude(t *testing.T) {
	e := newSyncEnv(t)
	ctx := context.Background()

	tentative := e.baseEvent()
	tentative.ExternalID = "evt-tent"
	tentative.Status = "tentative"
	if _, err := e.svc.SyncEvents(ctx, e.periodID, []service.IncomingEvent{tentative}); err != nil {
		t.Fatal(err)
	}
	items := openDecisions(t, e)

	if _, err := e.svc.ResolveReviewDecision(ctx, service.ResolveReviewDecisionInput{
		DecisionID: items[0].ID,
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

	if _, err := e.svc.SyncEvents(ctx, e.periodID, []service.IncomingEvent{tentative}); err != nil {
		t.Fatal(err)
	}
	if open := openDecisions(t, e); len(open) != 0 {
		t.Fatalf("resolved tentative event should not requeue, got %d open", len(open))
	}
	events, err = e.svc.ListEvents(ctx, e.periodID)
	if err != nil {
		t.Fatal(err)
	}
	if len(events) != 0 {
		t.Fatalf("excluded event should remain hidden, got %+v", events)
	}
}
