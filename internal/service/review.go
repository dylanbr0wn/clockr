package service

import (
	"context"
	"fmt"

	"github.com/dylanbr0wn/shiet/internal/db/sqlc"
)

// ResolveReviewDecisionInput is the user decision for one review decision.
type ResolveReviewDecisionInput struct {
	DecisionID int64  `json:"decisionId"`
	Action     string `json:"action"`
}

// ResolveReviewDecisionResult identifies the period whose schedule data changed.
type ResolveReviewDecisionResult struct {
	PeriodID int64 `json:"periodId"`
}

// ResolveReviewDecision applies a user decision to an open review item and marks it
// resolved or dismissed. Side effects depend on kind + action.
func (s *Service) ResolveReviewDecision(ctx context.Context, input ResolveReviewDecisionInput) (ResolveReviewDecisionResult, error) {
	var res ResolveReviewDecisionResult

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return res, fmt.Errorf("begin resolve tx: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	q := s.q.WithTx(tx)
	item, err := q.GetReviewItem(ctx, input.DecisionID)
	if err != nil {
		return res, mapErr("get review item", err)
	}
	if item.Status != "open" {
		return res, fmt.Errorf("review item %d is not open", item.ID)
	}

	res.PeriodID = item.PeriodID

	status, err := s.review().Apply(ctx, q, item, input.Action)
	if err != nil {
		return res, err
	}

	if err := q.ResolveReviewItem(ctx, sqlc.ResolveReviewItemParams{
		Status:          status,
		DecisionAction:  input.Action,
		DecisionPayload: "{}",
		ID:              item.ID,
	}); err != nil {
		return res, mapErr("resolve review item", err)
	}

	if err := tx.Commit(); err != nil {
		return res, fmt.Errorf("commit resolve tx: %w", err)
	}
	return res, nil
}
