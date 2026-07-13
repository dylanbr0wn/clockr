package service_test

import (
	"context"
	"database/sql"
	"strings"
	"testing"

	"github.com/dylanbr0wn/shiet/internal/db/sqlc"
	"github.com/dylanbr0wn/shiet/internal/service"
)

func TestListReviewDecisions_SupportedKinds(t *testing.T) {
	tests := []struct {
		name       string
		setup      func(t *testing.T, e *syncEnv)
		wantKind   string
		wantTag    string
		wantKeys   []string
		wantHidden bool
	}{
		{
			name: "deleted_categorized",
			setup: func(t *testing.T, e *syncEnv) {
				ctx := context.Background()
				if _, err := e.svc.SyncEvents(ctx, e.periodID, []service.IncomingEvent{e.baseEvent()}); err != nil {
					t.Fatal(err)
				}
				mustOverlay(t, e, "evt-1")
				if _, err := e.svc.SyncEvents(ctx, e.periodID, []service.IncomingEvent{}); err != nil {
					t.Fatal(err)
				}
			},
			wantKind: "deleted_categorized",
			wantTag:  "Removed",
			wantKeys: []string{service.ReviewActionDropEntry, service.ReviewActionKeepEntry},
		},
		{
			name: "title_changed",
			setup: func(t *testing.T, e *syncEnv) {
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
			},
			wantKind: "title_changed",
			wantTag:  "Title changed",
			wantKeys: []string{service.ReviewActionAccept, service.ReviewActionDismiss},
		},
		{
			name: "new_in_gap",
			setup: func(t *testing.T, e *syncEnv) {
				ctx := context.Background()
				insertTimeEntry(t, e.q, e.periodID, "2026-06-02", "2026-06-02T13:00:00Z", "2026-06-02T14:00:00Z", sql.NullInt64{Int64: e.catID, Valid: true}, "", true)
				inc := e.baseEvent()
				inc.ExternalID = "evt-overlap"
				inc.Start = tm("2026-06-02T13:30:00Z")
				inc.End = tm("2026-06-02T14:30:00Z")
				if _, err := e.svc.SyncEvents(ctx, e.periodID, []service.IncomingEvent{inc}); err != nil {
					t.Fatal(err)
				}
			},
			wantKind: "new_in_gap",
			wantTag:  "Gap conflict",
			wantKeys: []string{service.ReviewActionUseEvent, service.ReviewActionKeepGap},
		},
		{
			name: "tentative",
			setup: func(t *testing.T, e *syncEnv) {
				ctx := context.Background()
				tentative := e.baseEvent()
				tentative.ExternalID = "evt-tent"
				tentative.Status = "tentative"
				if _, err := e.svc.SyncEvents(ctx, e.periodID, []service.IncomingEvent{tentative}); err != nil {
					t.Fatal(err)
				}
			},
			wantKind: "tentative",
			wantTag:  "Tentative",
			wantKeys: []string{service.ReviewActionInclude, service.ReviewActionExclude},
		},
		{
			name: "all_day",
			setup: func(t *testing.T, e *syncEnv) {
				ctx := context.Background()
				allDay := service.IncomingEvent{
					CalendarID: e.calID, Provider: service.ProviderGoogle, ExternalID: "evt-allday", Title: "Holiday",
					Status: "accepted", AllDay: true, StartDate: "2026-06-03", EndDate: "2026-06-04",
				}
				if _, err := e.svc.SyncEvents(ctx, e.periodID, []service.IncomingEvent{allDay}); err != nil {
					t.Fatal(err)
				}
			},
			wantKind: "all_day",
			wantTag:  "All day",
			wantKeys: []string{service.ReviewActionInclude, service.ReviewActionExclude},
		},
		{
			name: "unsupported overlap hidden",
			setup: func(t *testing.T, e *syncEnv) {
				ctx := context.Background()
				if _, err := e.q.CreateReviewItem(ctx, sqlc.CreateReviewItemParams{
					PeriodID:    e.periodID,
					Kind:        "overlap",
					EventID:     sql.NullInt64{Int64: 1, Valid: true},
					Payload:     `{"title":"Overlap"}`,
					ConflictKey: "overlap|test",
				}); err != nil {
					t.Fatal(err)
				}
			},
			wantHidden: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			e := newSyncEnv(t)
			tt.setup(t, e)
			decisions, err := e.svc.ListReviewDecisions(context.Background(), e.periodID)
			if err != nil {
				t.Fatal(err)
			}
			if tt.wantHidden {
				if len(decisions) != 0 {
					t.Fatalf("unsupported kind should be hidden, got %+v", decisions)
				}
				return
			}

			var found *service.ReviewDecision
			for i := range decisions {
				if decisions[i].Kind == tt.wantKind {
					found = &decisions[i]
					break
				}
			}
			if found == nil {
				t.Fatalf("want decision kind %q, got %+v", tt.wantKind, decisions)
			}
			if found.Tag != tt.wantTag {
				t.Fatalf("tag = %q, want %q", found.Tag, tt.wantTag)
			}
			if found.Title == "" || found.Description == "" {
				t.Fatalf("title/description should be populated: %+v", found)
			}
			if len(found.Actions) != len(tt.wantKeys) {
				t.Fatalf("actions = %+v, want keys %v", found.Actions, tt.wantKeys)
			}
			for i, key := range tt.wantKeys {
				if found.Actions[i].Key != key {
					t.Fatalf("action[%d].Key = %q, want %q", i, found.Actions[i].Key, key)
				}
				if found.Actions[i].Label == "" {
					t.Fatalf("action[%d] missing label", i)
				}
			}
		})
	}
}

func TestResolveReviewDecision_AcceptsReadModelActions(t *testing.T) {
	e := newSyncEnv(t)
	ctx := context.Background()

	if _, err := e.svc.SyncEvents(ctx, e.periodID, []service.IncomingEvent{e.baseEvent()}); err != nil {
		t.Fatal(err)
	}
	mustOverlay(t, e, "evt-1")
	if _, err := e.svc.SyncEvents(ctx, e.periodID, []service.IncomingEvent{}); err != nil {
		t.Fatal(err)
	}

	decisions, err := e.svc.ListReviewDecisions(ctx, e.periodID)
	if err != nil {
		t.Fatal(err)
	}
	if len(decisions) != 1 || len(decisions[0].Actions) == 0 {
		t.Fatalf("want decision with actions, got %+v", decisions)
	}

	primary := decisions[0].Actions[0]
	if _, err := e.svc.ResolveReviewDecision(ctx, service.ResolveReviewDecisionInput{
		DecisionID: decisions[0].ID,
		Action:     primary.Key,
	}); err != nil {
		t.Fatalf("read-model action %q should be accepted: %v", primary.Key, err)
	}
}

func TestListReviewDecisions_DeletedUsesEventTitle(t *testing.T) {
	e := newSyncEnv(t)
	ctx := context.Background()

	if _, err := e.svc.SyncEvents(ctx, e.periodID, []service.IncomingEvent{e.baseEvent()}); err != nil {
		t.Fatal(err)
	}
	mustOverlay(t, e, "evt-1")
	if _, err := e.svc.SyncEvents(ctx, e.periodID, []service.IncomingEvent{}); err != nil {
		t.Fatal(err)
	}

	decisions := openDecisions(t, e)
	if len(decisions) != 1 {
		t.Fatalf("want 1 decision, got %d", len(decisions))
	}
	if decisions[0].Title != "Standup" {
		t.Fatalf("title = %q, want event title", decisions[0].Title)
	}
	if !strings.Contains(decisions[0].Description, "30m") {
		t.Fatalf("description should include duration from event: %q", decisions[0].Description)
	}
}
