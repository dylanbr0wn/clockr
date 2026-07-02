package service

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/dylanbr0wn/clockr/internal/db/sqlc"
)

// Review resolution actions (must match frontend).
const (
	ReviewActionKeepEntry  = "keep_entry"
	ReviewActionDropEntry  = "drop_entry"
	ReviewActionAccept     = "accept"
	ReviewActionDismiss    = "dismiss"
	ReviewActionKeepGap    = "keep_gap"
	ReviewActionUseEvent   = "use_event"
	ReviewActionInclude    = "include"
	ReviewActionExclude    = "exclude"
)

// ResolveReviewItemInput is the user decision for one review-queue item.
type ResolveReviewItemInput struct {
	ReviewItemID int64  `json:"reviewItemId"`
	Action       string `json:"action"`
}

// ResolveReviewItemResult identifies the period whose schedule data changed.
type ResolveReviewItemResult struct {
	PeriodID int64 `json:"periodId"`
}

// ResolveReviewItem applies a user decision to an open review item and marks it
// resolved or dismissed. Side effects depend on kind + action.
func (s *Service) ResolveReviewItem(ctx context.Context, input ResolveReviewItemInput) (ResolveReviewItemResult, error) {
	var res ResolveReviewItemResult

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return res, fmt.Errorf("begin resolve tx: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	q := s.q.WithTx(tx)
	item, err := q.GetReviewItem(ctx, input.ReviewItemID)
	if err != nil {
		return res, mapErr("get review item", err)
	}
	if item.Status != "open" {
		return res, fmt.Errorf("review item %d is not open", item.ID)
	}

	res.PeriodID = item.PeriodID

	status := "resolved"
	switch item.Kind {
	case reviewDeletedCategoriz:
		if err := s.resolveDeletedCategorized(ctx, q, item, input.Action); err != nil {
			return res, err
		}
	case reviewTitleChanged:
		switch input.Action {
		case ReviewActionAccept:
		case ReviewActionDismiss:
			status = "dismissed"
		default:
			return res, fmt.Errorf("unsupported action %q for %s", input.Action, item.Kind)
		}
	case reviewNewInGap:
		if err := s.resolveNewInGap(ctx, q, item, input.Action); err != nil {
			return res, err
		}
	case reviewTentative, reviewAllDay:
		if err := s.resolveIncludeExclude(ctx, q, item, input.Action); err != nil {
			return res, err
		}
	case "overlap", "dedup_ambiguous":
		return res, fmt.Errorf("review kind %q is not supported yet", item.Kind)
	default:
		return res, fmt.Errorf("unknown review kind %q", item.Kind)
	}

	if err := q.ResolveReviewItem(ctx, sqlc.ResolveReviewItemParams{
		Status: status,
		ID:     item.ID,
	}); err != nil {
		return res, mapErr("resolve review item", err)
	}

	if err := tx.Commit(); err != nil {
		return res, fmt.Errorf("commit resolve tx: %w", err)
	}
	return res, nil
}

func (s *Service) resolveDeletedCategorized(ctx context.Context, q *sqlc.Queries, item sqlc.ReviewItem, action string) error {
	if !item.EventID.Valid {
		return fmt.Errorf("deleted_categorized item %d has no event", item.ID)
	}
	ev, err := q.GetEvent(ctx, item.EventID.Int64)
	if err != nil {
		return mapErr("get event", err)
	}

	switch action {
	case ReviewActionKeepEntry:
		return nil
	case ReviewActionDropEntry:
		if err := s.deleteCategoryOverlay(ctx, q, ev); err != nil {
			return err
		}
		if err := q.DeleteEvent(ctx, ev.ID); err != nil {
			return mapErr("delete event", err)
		}
		return nil
	default:
		return fmt.Errorf("unsupported action %q for deleted_categorized", action)
	}
}

func (s *Service) resolveNewInGap(ctx context.Context, q *sqlc.Queries, item sqlc.ReviewItem, action string) error {
	if !item.EventID.Valid {
		return fmt.Errorf("new_in_gap item %d has no event", item.ID)
	}
	ev, err := q.GetEvent(ctx, item.EventID.Int64)
	if err != nil {
		return mapErr("get event", err)
	}

	switch action {
	case ReviewActionKeepGap:
		if err := q.DeleteEvent(ctx, ev.ID); err != nil {
			return mapErr("delete event", err)
		}
		return nil
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
		return fmt.Errorf("unsupported action %q for new_in_gap", action)
	}
}

func (s *Service) resolveIncludeExclude(ctx context.Context, q *sqlc.Queries, item sqlc.ReviewItem, action string) error {
	if !item.EventID.Valid {
		return fmt.Errorf("%s item %d has no event", item.Kind, item.ID)
	}

	switch action {
	case ReviewActionInclude:
		return nil
	case ReviewActionExclude:
		ev, err := q.GetEvent(ctx, item.EventID.Int64)
		if err != nil {
			return mapErr("get event", err)
		}
		if err := s.deleteCategoryOverlay(ctx, q, ev); err != nil {
			return err
		}
		if err := q.DeleteEvent(ctx, ev.ID); err != nil {
			return mapErr("delete event", err)
		}
		return nil
	default:
		return fmt.Errorf("unsupported action %q for %s", action, item.Kind)
	}
}

func (s *Service) deleteCategoryOverlay(ctx context.Context, q *sqlc.Queries, ev sqlc.Event) error {
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
