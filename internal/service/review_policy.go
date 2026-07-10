package service

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/dylanbr0wn/shiet/internal/db/sqlc"
)

// Review resolution actions (must match frontend).
const (
	ReviewActionKeepEntry = "keep_entry"
	ReviewActionDropEntry = "drop_entry"
	ReviewActionAccept    = "accept"
	ReviewActionDismiss   = "dismiss"
	ReviewActionKeepGap   = "keep_gap"
	ReviewActionUseEvent  = "use_event"
	ReviewActionInclude   = "include"
	ReviewActionExclude   = "exclude"
)

// review_item.kind values (must match the schema CHECK constraint).
const (
	reviewNewInGap         = "new_in_gap"
	reviewTitleChanged     = "title_changed"
	reviewDeletedCategoriz = "deleted_categorized"
	reviewTentative        = "tentative"
	reviewAllDay           = "all_day"
)

// overlay status note used to hide an event from the schedule.
const overlayStatusExclude = "excluded"

// reviewPolicy owns re-sync review vocabulary, enqueue/replay, and side effects.
// Service methods keep transaction begin/commit and call into this type.
type reviewPolicy struct{}

func (s *Service) review() reviewPolicy { return reviewPolicy{} }

// ConflictKey builds a stable conflict key from an event identity and parts.
func (reviewPolicy) ConflictKey(identity string, parts ...string) string {
	fields := append([]string{identity}, parts...)
	return strings.Join(fields, "|")
}

// EnqueueOrReplay creates an open review item, or returns a prior decision action
// when the same conflict key was already resolved/dismissed.
func (reviewPolicy) EnqueueOrReplay(ctx context.Context, q *sqlc.Queries, periodID int64, kind string, eventID int64, conflictKey string, payload map[string]any) (action string, created bool, err error) {
	b, err := json.Marshal(payload)
	if err != nil {
		return "", false, fmt.Errorf("marshal review payload: %w", err)
	}
	existing, err := q.GetReviewItemByConflictKey(ctx, sqlc.GetReviewItemByConflictKeyParams{
		PeriodID:    periodID,
		Kind:        kind,
		ConflictKey: conflictKey,
	})
	if err == nil {
		if existing.Status == "open" {
			return "", false, nil
		}
		return existing.DecisionAction, false, nil
	}
	if !errors.Is(err, sql.ErrNoRows) {
		return "", false, mapErr("get review item by conflict key", err)
	}

	if _, err := q.CreateReviewItem(ctx, sqlc.CreateReviewItemParams{
		PeriodID:    periodID,
		Kind:        kind,
		EventID:     sql.NullInt64{Int64: eventID, Valid: true},
		Payload:     string(b),
		ConflictKey: conflictKey,
	}); err != nil {
		return "", false, fmt.Errorf("enqueue review item: %w", err)
	}
	return "", true, nil
}

// Apply validates kind/action and runs review side effects. status is
// "resolved" or "dismissed" for the review_item row (caller persists it).
func (p reviewPolicy) Apply(ctx context.Context, q *sqlc.Queries, item sqlc.ReviewItem, action string) (status string, err error) {
	status = "resolved"
	switch item.Kind {
	case reviewDeletedCategoriz:
		err = p.applyDeletedCategorized(ctx, q, item, action)
	case reviewTitleChanged:
		switch action {
		case ReviewActionAccept:
		case ReviewActionDismiss:
			status = "dismissed"
		default:
			return "", invalidInputf("unsupported action %q for %s", action, item.Kind)
		}
	case reviewNewInGap:
		err = p.applyNewInGap(ctx, q, item, action)
	case reviewTentative, reviewAllDay:
		err = p.applyIncludeExclude(ctx, q, item, action)
	case "overlap", "dedup_ambiguous":
		return "", failedPreconditionf("review kind %q is not supported yet", item.Kind)
	default:
		return "", fmt.Errorf("unknown review kind %q", item.Kind)
	}
	return status, err
}

// OnVanishedCategorized enqueues deleted_categorized (or replays a prior
// decision). flagged/removed report sync counters.
func (p reviewPolicy) OnVanishedCategorized(ctx context.Context, q *sqlc.Queries, periodID int64, e sqlc.Event) (flagged, removed bool, err error) {
	action, created, err := p.EnqueueOrReplay(ctx, q, periodID, reviewDeletedCategoriz, e.ID,
		p.ConflictKey(eventIdentityFromRow(e), "deleted"),
		map[string]any{"reason": "deleted", "title": e.Title})
	if err != nil {
		return false, false, err
	}
	if created {
		flagged = true
	}
	if action == ReviewActionDropEntry || action == ReviewActionKeepEntry {
		if _, err := p.Apply(ctx, q, sqlc.ReviewItem{
			PeriodID: periodID,
			Kind:     reviewDeletedCategoriz,
			EventID:  sql.NullInt64{Int64: e.ID, Valid: true},
		}, action); err != nil {
			return flagged, false, err
		}
		if action == ReviewActionDropEntry {
			removed = true
		}
	}
	return flagged, removed, nil
}

// OnNewEvent flags gap / all-day / tentative conflicts for a newly inserted
// event and replays prior decisions. skipAuto is true when the event should
// not receive memory / calendar-default / AI overlays (excluded by replay).
func (p reviewPolicy) OnNewEvent(ctx context.Context, q *sqlc.Queries, periodID int64, inc IncomingEvent, eventID int64, gaps []sqlc.GapFill) (flagged int, skipAuto bool, err error) {
	identity := eventIdentityFromIncoming(inc)
	gapFingerprint, overlapsGap := overlappingGapFingerprint(inc, gaps)
	switch {
	case overlapsGap:
		action, created, err := p.EnqueueOrReplay(ctx, q, periodID, reviewNewInGap, eventID,
			p.ConflictKey(identity, gapFingerprint),
			map[string]any{"title": inc.Title})
		if err != nil {
			return 0, false, err
		}
		if created {
			flagged = 1
		}
		switch action {
		case ReviewActionKeepGap, ReviewActionUseEvent:
			if _, err := p.Apply(ctx, q, sqlc.ReviewItem{
				PeriodID: periodID,
				Kind:     reviewNewInGap,
				EventID:  sql.NullInt64{Int64: eventID, Valid: true},
			}, action); err != nil {
				return flagged, false, err
			}
			if action == ReviewActionKeepGap {
				return flagged, true, nil
			}
		}
	case inc.AllDay:
		_, created, err := p.EnqueueOrReplay(ctx, q, periodID, reviewAllDay, eventID,
			p.ConflictKey(identity, inc.StartDate, inc.EndDate),
			map[string]any{"title": inc.Title})
		if err != nil {
			return 0, false, err
		}
		if created {
			flagged = 1
		}
	case inc.Status == "tentative" || inc.Status == "needsAction":
		action, created, err := p.EnqueueOrReplay(ctx, q, periodID, reviewTentative, eventID,
			p.ConflictKey(identity, inc.Status),
			map[string]any{"title": inc.Title, "status": inc.Status})
		if err != nil {
			return 0, false, err
		}
		if created {
			flagged = 1
		}
		switch action {
		case ReviewActionExclude:
			if _, err := p.Apply(ctx, q, sqlc.ReviewItem{
				PeriodID: periodID,
				Kind:     reviewTentative,
				EventID:  sql.NullInt64{Int64: eventID, Valid: true},
			}, action); err != nil {
				return flagged, false, err
			}
			return flagged, true, nil
		case ReviewActionInclude:
		}
	}
	return flagged, false, nil
}

// OnChangedCategorized flags title_changed / declined conflicts for an already
// categorized event and replays prior drop decisions for declined.
func (p reviewPolicy) OnChangedCategorized(ctx context.Context, q *sqlc.Queries, periodID, eventID int64, ex sqlc.Event, inc IncomingEvent) (flagged int, err error) {
	if normalizeTitle(ex.Title) != normalizeTitle(inc.Title) {
		_, created, err := p.EnqueueOrReplay(ctx, q, periodID, reviewTitleChanged, eventID,
			p.ConflictKey(eventIdentityFromIncoming(inc), normalizeTitle(ex.Title), normalizeTitle(inc.Title)),
			map[string]any{"from": ex.Title, "to": inc.Title})
		if err != nil {
			return 0, err
		}
		if created {
			flagged++
		}
	}
	if inc.Status == "declined" {
		action, created, err := p.EnqueueOrReplay(ctx, q, periodID, reviewDeletedCategoriz, eventID,
			p.ConflictKey(eventIdentityFromIncoming(inc), "declined"),
			map[string]any{"reason": "declined", "title": inc.Title})
		if err != nil {
			return flagged, err
		}
		if created {
			flagged++
		}
		if action == ReviewActionDropEntry {
			if _, err := p.Apply(ctx, q, sqlc.ReviewItem{
				PeriodID: periodID,
				Kind:     reviewDeletedCategoriz,
				EventID:  sql.NullInt64{Int64: eventID, Valid: true},
			}, action); err != nil {
				return flagged, err
			}
		}
	}
	return flagged, nil
}

// MarkExcluded upserts a status=excluded overlay for the event.
func (reviewPolicy) MarkExcluded(ctx context.Context, q *sqlc.Queries, ev sqlc.Event) error {
	if _, err := q.UpsertOverlay(ctx, sqlc.UpsertOverlayParams{
		PeriodID:   ev.PeriodID,
		Provider:   ev.Provider,
		ExternalID: ev.ExternalID,
		InstanceID: ev.InstanceID,
		Note:       overlayStatusExclude,
		Kind:       overlayKindStatus,
	}); err != nil {
		return mapErr("mark event excluded", err)
	}
	return nil
}

// HasStatusExcluded reports whether the event has a status=excluded overlay.
func (reviewPolicy) HasStatusExcluded(ctx context.Context, q *sqlc.Queries, ev sqlc.Event) (bool, error) {
	o, err := q.GetOverlay(ctx, sqlc.GetOverlayParams{
		PeriodID:   ev.PeriodID,
		Provider:   ev.Provider,
		ExternalID: ev.ExternalID,
		InstanceID: ev.InstanceID,
		Kind:       overlayKindStatus,
	})
	if errors.Is(err, sql.ErrNoRows) {
		return false, nil
	}
	if err != nil {
		return false, mapErr("get status overlay", err)
	}
	return o.Note == overlayStatusExclude, nil
}

// DeleteCategoryOverlay removes the category overlay for the event if present.
func (reviewPolicy) DeleteCategoryOverlay(ctx context.Context, q *sqlc.Queries, ev sqlc.Event) error {
	o, err := q.GetOverlay(ctx, sqlc.GetOverlayParams{
		PeriodID:   ev.PeriodID,
		Provider:   ev.Provider,
		ExternalID: ev.ExternalID,
		InstanceID: ev.InstanceID,
		Kind:       overlayKindCategory,
	})
	if errors.Is(err, sql.ErrNoRows) {
		return nil
	}
	if err != nil {
		return mapErr("get overlay", err)
	}
	if err := q.DeleteOverlay(ctx, o.ID); err != nil {
		return mapErr("delete overlay", err)
	}
	return nil
}

func (p reviewPolicy) applyDeletedCategorized(ctx context.Context, q *sqlc.Queries, item sqlc.ReviewItem, action string) error {
	if !item.EventID.Valid {
		return fmt.Errorf("deleted_categorized item %d has no event", item.ID)
	}
	ev, err := q.GetEvent(ctx, item.EventID.Int64)
	if err != nil {
		return mapErr("get event", err)
	}

	switch action {
	case ReviewActionKeepEntry:
		if err := p.convertEventToManualFill(ctx, q, ev); err != nil {
			return err
		}
		return p.MarkExcluded(ctx, q, ev)
	case ReviewActionDropEntry:
		if err := p.DeleteCategoryOverlay(ctx, q, ev); err != nil {
			return err
		}
		return p.MarkExcluded(ctx, q, ev)
	default:
		return invalidInputf("unsupported action %q for deleted_categorized", action)
	}
}

func (p reviewPolicy) convertEventToManualFill(ctx context.Context, q *sqlc.Queries, ev sqlc.Event) error {
	if ev.AllDay != 0 || !ev.StartUtc.Valid || !ev.EndUtc.Valid {
		return fmt.Errorf("convert deleted event to manual: event %d is not a timed event", ev.ID)
	}

	categoryID := sql.NullInt64{}
	o, err := q.GetOverlay(ctx, sqlc.GetOverlayParams{
		PeriodID:   ev.PeriodID,
		Provider:   ev.Provider,
		ExternalID: ev.ExternalID,
		InstanceID: ev.InstanceID,
		Kind:       overlayKindCategory,
	})
	if errors.Is(err, sql.ErrNoRows) {
		// A deleted_categorized review item should have a category overlay, but
		// preserve the user's entry even if the overlay was removed meanwhile.
	} else if err != nil {
		return mapErr("get category overlay", err)
	} else {
		categoryID = o.CategoryID
	}

	day, err := eventLocalDay(ctx, q, ev)
	if err != nil {
		return err
	}
	if _, err := q.CreateGapFill(ctx, sqlc.CreateGapFillParams{
		PeriodID:   ev.PeriodID,
		Day:        day,
		StartUtc:   parseTime(ev.StartUtc.String).Format(time.RFC3339),
		EndUtc:     parseTime(ev.EndUtc.String).Format(time.RFC3339),
		CategoryID: categoryID,
		Note:       ev.Title,
		Source:     "manual",
	}); err != nil {
		return mapErr("create manual copy of deleted event", err)
	}
	return p.DeleteCategoryOverlay(ctx, q, ev)
}

func (p reviewPolicy) applyNewInGap(ctx context.Context, q *sqlc.Queries, item sqlc.ReviewItem, action string) error {
	if !item.EventID.Valid {
		return fmt.Errorf("new_in_gap item %d has no event", item.ID)
	}
	ev, err := q.GetEvent(ctx, item.EventID.Int64)
	if err != nil {
		return mapErr("get event", err)
	}

	switch action {
	case ReviewActionKeepGap:
		return p.MarkExcluded(ctx, q, ev)
	case ReviewActionUseEvent:
		if ev.AllDay != 0 || !ev.StartUtc.Valid || !ev.EndUtc.Valid {
			return fmt.Errorf("event %d is not a timed event", ev.ID)
		}
		eventSpan := Interval{
			Start: parseTime(ev.StartUtc.String),
			End:   parseTime(ev.EndUtc.String),
		}
		fills, err := q.ListGapFillsForPeriod(ctx, item.PeriodID)
		if err != nil {
			return mapErr("list gap fills", err)
		}
		return shrinkGapFillsForEvent(ctx, q, item.PeriodID, eventSpan, fills)
	default:
		return invalidInputf("unsupported action %q for new_in_gap", action)
	}
}

func (p reviewPolicy) applyIncludeExclude(ctx context.Context, q *sqlc.Queries, item sqlc.ReviewItem, action string) error {
	if !item.EventID.Valid {
		return fmt.Errorf("%s item %d has no event", item.Kind, item.ID)
	}

	switch action {
	case ReviewActionInclude:
		if item.Kind == reviewAllDay {
			return p.clearAllDayReviewExclusion(ctx, q, item.EventID.Int64)
		}
		return nil
	case ReviewActionExclude:
		ev, err := q.GetEvent(ctx, item.EventID.Int64)
		if err != nil {
			return mapErr("get event", err)
		}
		if err := p.DeleteCategoryOverlay(ctx, q, ev); err != nil {
			return err
		}
		if item.Kind == reviewAllDay {
			// All-day markers never occupy gap time; exclude dismisses review only.
			return p.clearAllDayReviewExclusion(ctx, q, ev.ID)
		}
		return p.MarkExcluded(ctx, q, ev)
	default:
		return invalidInputf("unsupported action %q for %s", action, item.Kind)
	}
}

func (p reviewPolicy) clearAllDayReviewExclusion(ctx context.Context, q *sqlc.Queries, eventID int64) error {
	ev, err := q.GetEvent(ctx, eventID)
	if err != nil {
		return mapErr("get event", err)
	}
	if ev.AllDay == 0 {
		return nil
	}
	o, err := q.GetOverlay(ctx, sqlc.GetOverlayParams{
		PeriodID:   ev.PeriodID,
		Provider:   ev.Provider,
		ExternalID: ev.ExternalID,
		InstanceID: ev.InstanceID,
		Kind:       overlayKindStatus,
	})
	if errors.Is(err, sql.ErrNoRows) {
		return nil
	}
	if err != nil {
		return mapErr("get status overlay", err)
	}
	if o.Note != overlayStatusExclude {
		return nil
	}
	if err := q.DeleteOverlay(ctx, o.ID); err != nil {
		return mapErr("delete all-day status overlay", err)
	}
	return nil
}

func eventLocalDay(ctx context.Context, q *sqlc.Queries, ev sqlc.Event) (string, error) {
	start := parseTime(ev.StartUtc.String)
	if start.IsZero() {
		return "", fmt.Errorf("event %d has invalid start time", ev.ID)
	}
	segs, err := q.ListTzSegments(ctx, ev.PeriodID)
	if err != nil {
		return "", mapErr("list timezone segments", err)
	}
	if len(segs) == 0 {
		return "", fmt.Errorf("period %d has no timezone segment", ev.PeriodID)
	}

	// Bootstrap with the UTC date, then re-check after converting through the
	// active timezone in case the local date differs from the UTC date.
	date := start.UTC().Format("2006-01-02")
	for i := 0; i < 2; i++ {
		seg := activeSQLCTzSegment(segs, date)
		loc, err := time.LoadLocation(seg.IanaTz)
		if err != nil {
			return "", fmt.Errorf("load timezone %q: %w", seg.IanaTz, err)
		}
		next := start.In(loc).Format("2006-01-02")
		if next == date {
			return date, nil
		}
		date = next
	}
	return date, nil
}

func activeSQLCTzSegment(segs []sqlc.TzSegment, dateStr string) sqlc.TzSegment {
	active := segs[0]
	for _, seg := range segs {
		if seg.EffectiveFromDate <= dateStr {
			active = seg
		} else {
			break
		}
	}
	return active
}

// shrinkGapFillsForEvent removes or trims gap fills that overlap a timed event
// so the event can be counted without double-counting the filled interval.
func shrinkGapFillsForEvent(ctx context.Context, q *sqlc.Queries, periodID int64, event Interval, fills []sqlc.GapFill) error {
	for _, fill := range fills {
		gap := Interval{Start: parseTime(fill.StartUtc), End: parseTime(fill.EndUtc)}
		if gap.Start.IsZero() || gap.End.IsZero() {
			continue
		}
		before, overlap, after := splitAround(gap, event)
		if overlap == nil {
			continue
		}

		if before == nil && after == nil {
			if _, err := q.DeleteGapFill(ctx, sqlc.DeleteGapFillParams{ID: fill.ID, PeriodID: periodID}); err != nil {
				return mapErr("delete gap fill", err)
			}
			continue
		}

		if before == nil && after != nil {
			if _, err := q.UpdateGapFillSpan(ctx, sqlc.UpdateGapFillSpanParams{
				StartUtc: after.Start.UTC().Format(time.RFC3339),
				EndUtc:   after.End.UTC().Format(time.RFC3339),
				ID:       fill.ID,
				PeriodID: periodID,
			}); err != nil {
				return mapErr("update gap fill", err)
			}
			continue
		}

		if before != nil && after == nil {
			if _, err := q.UpdateGapFillSpan(ctx, sqlc.UpdateGapFillSpanParams{
				StartUtc: before.Start.UTC().Format(time.RFC3339),
				EndUtc:   before.End.UTC().Format(time.RFC3339),
				ID:       fill.ID,
				PeriodID: periodID,
			}); err != nil {
				return mapErr("update gap fill", err)
			}
			continue
		}

		// Event sits in the middle: keep the leading segment, create a trailing fill.
		if _, err := q.UpdateGapFillSpan(ctx, sqlc.UpdateGapFillSpanParams{
			StartUtc: before.Start.UTC().Format(time.RFC3339),
			EndUtc:   before.End.UTC().Format(time.RFC3339),
			ID:       fill.ID,
			PeriodID: periodID,
		}); err != nil {
			return mapErr("update gap fill", err)
		}
		if _, err := q.CreateGapFill(ctx, sqlc.CreateGapFillParams{
			PeriodID:   periodID,
			Day:        fill.Day,
			StartUtc:   after.Start.UTC().Format(time.RFC3339),
			EndUtc:     after.End.UTC().Format(time.RFC3339),
			CategoryID: fill.CategoryID,
			Note:       fill.Note,
			Source:     fill.Source,
		}); err != nil {
			return mapErr("create gap fill", err)
		}
	}
	return nil
}

// splitAround decomposes gap into the portions before, overlapping, and after
// event. Each non-nil portion is a non-empty half-open interval.
func splitAround(gap, event Interval) (before, overlap, after *Interval) {
	if !gap.Start.Before(gap.End) {
		return nil, nil, nil
	}
	if gap.End.Before(event.Start) || !gap.Start.Before(event.End) {
		g := gap
		return &g, nil, nil
	}
	if !event.Start.After(gap.Start) {
		if !gap.End.After(event.End) {
			o := gap
			return nil, &o, nil
		}
		a := Interval{Start: event.End, End: gap.End}
		o := Interval{Start: gap.Start, End: event.End}
		return nil, &o, &a
	}
	if !gap.End.After(event.End) {
		b := Interval{Start: gap.Start, End: event.Start}
		o := Interval{Start: event.Start, End: gap.End}
		return &b, &o, nil
	}
	b := Interval{Start: gap.Start, End: event.Start}
	o := Interval{Start: event.Start, End: event.End}
	a := Interval{Start: event.End, End: gap.End}
	return &b, &o, &a
}

func eventIdentityFromIncoming(inc IncomingEvent) string {
	return strings.Join([]string{
		inc.Provider,
		fmt.Sprint(inc.CalendarID),
		inc.ExternalID,
		inc.InstanceID,
	}, "|")
}

func eventIdentityFromRow(ev sqlc.Event) string {
	return strings.Join([]string{
		ev.Provider,
		fmt.Sprint(ev.CalendarID),
		ev.ExternalID,
		ev.InstanceID,
	}, "|")
}

func overlappingGapFingerprint(inc IncomingEvent, gaps []sqlc.GapFill) (string, bool) {
	if inc.Start == nil || inc.End == nil {
		return "", false
	}
	s, e := inc.Start.UTC(), inc.End.UTC()
	parts := []string{}
	for _, g := range gaps {
		gs := parseTime(g.StartUtc)
		ge := parseTime(g.EndUtc)
		if gs.IsZero() || ge.IsZero() {
			continue
		}
		if s.Before(ge) && gs.Before(e) {
			category := "none"
			if g.CategoryID.Valid {
				category = fmt.Sprint(g.CategoryID.Int64)
			}
			parts = append(parts, strings.Join([]string{
				g.StartUtc,
				g.EndUtc,
				category,
				g.Source,
			}, "~"))
		}
	}
	if len(parts) == 0 {
		return "", false
	}
	return strings.Join(parts, "^"), true
}
