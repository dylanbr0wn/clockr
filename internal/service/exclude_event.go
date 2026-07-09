package service

import (
	"context"
	"fmt"

	"github.com/dylanbr0wn/shiet/internal/db/sqlc"
)

// ExcludeEventInput identifies a synced calendar event to hide from the schedule.
type ExcludeEventInput struct {
	EventID  int64 `json:"eventId"`
	PeriodID int64 `json:"periodId"`
}

// ExcludeEventResult identifies the period whose schedule data changed.
type ExcludeEventResult struct {
	PeriodID int64 `json:"periodId"`
	EventID  int64 `json:"eventId"`
}

// ExcludeEvent hides a synced calendar event from the schedule via a status
// overlay. The event row stays so future syncs can reapply the decision.
func (s *Service) ExcludeEvent(ctx context.Context, input ExcludeEventInput) (ExcludeEventResult, error) {
	var res ExcludeEventResult
	if input.EventID <= 0 {
		return res, fmt.Errorf("exclude event: eventId is required")
	}
	if input.PeriodID <= 0 {
		return res, fmt.Errorf("exclude event: periodId is required")
	}

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return res, fmt.Errorf("begin exclude event tx: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	q := s.q.WithTx(tx)
	ev, err := q.GetEvent(ctx, input.EventID)
	if err != nil {
		return res, mapErr("get event", err)
	}
	if ev.PeriodID != input.PeriodID {
		return res, fmt.Errorf("exclude event: %w", ErrNotFound)
	}

	if err := s.deleteCategoryOverlay(ctx, q, ev); err != nil {
		return res, err
	}
	if err := markEventExcluded(ctx, q, ev); err != nil {
		return res, err
	}

	openItems, err := q.ListOpenReviewItems(ctx, input.PeriodID)
	if err != nil {
		return res, mapErr("list open review items", err)
	}
	for _, item := range openItems {
		if !item.EventID.Valid || item.EventID.Int64 != ev.ID {
			continue
		}
		if err := q.ResolveReviewItem(ctx, sqlc.ResolveReviewItemParams{
			Status:          "dismissed",
			DecisionAction:  ReviewActionExclude,
			DecisionPayload: "{}",
			ID:              item.ID,
		}); err != nil {
			return res, mapErr("dismiss review item", err)
		}
	}

	if err := tx.Commit(); err != nil {
		return res, fmt.Errorf("commit exclude event tx: %w", err)
	}
	return ExcludeEventResult{PeriodID: ev.PeriodID, EventID: ev.ID}, nil
}
