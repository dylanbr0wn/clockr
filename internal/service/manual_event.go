package service

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"time"

	"github.com/dylanbr0wn/shiet/internal/db/sqlc"
)

// ManualEventInput describes a user-created time block on the scheduler.
type ManualEventInput struct {
	PeriodID     int64  `json:"periodId"`
	Day          string `json:"day"` // YYYY-MM-DD in the period's active local timezone
	StartMinutes int    `json:"startMinutes"`
	EndMinutes   int    `json:"endMinutes"`
	CategoryID   *int64 `json:"categoryId,omitempty"`
	Note         string `json:"note,omitempty"`
}

// ManualEventUpdateInput describes a scheduler edit to an existing manual block.
type ManualEventUpdateInput struct {
	ID int64 `json:"id"`
	ManualEventInput
}

// ManualEventDeleteInput identifies a user-created scheduler block to remove.
type ManualEventDeleteInput struct {
	ID       int64 `json:"id"`
	PeriodID int64 `json:"periodId"`
}

// CreateManualEvent persists a manually-created scheduler block as a gap fill.
func (s *Service) CreateManualEvent(ctx context.Context, input ManualEventInput) (GapFill, error) {
	return s.createGapFillEntry(ctx, "create manual event", input, "manual")
}

// CreateGapFill persists a user-confirmed gap assignment (e.g. from AI suggest).
func (s *Service) CreateGapFill(ctx context.Context, input ManualEventInput) (GapFill, error) {
	return s.createGapFillEntry(ctx, "create gap fill", input, "gap")
}

func (s *Service) createGapFillEntry(ctx context.Context, action string, input ManualEventInput, source string) (GapFill, error) {
	span, err := s.manualEventSpan(ctx, action, input)
	if err != nil {
		return GapFill{}, err
	}

	categoryID := sql.NullInt64{}
	if input.CategoryID != nil {
		categoryID = sql.NullInt64{Int64: *input.CategoryID, Valid: true}
	}

	row, err := s.q.CreateGapFill(ctx, sqlc.CreateGapFillParams{
		PeriodID:   input.PeriodID,
		Day:        input.Day,
		StartUtc:   span.start.Format(time.RFC3339),
		EndUtc:     span.end.Format(time.RFC3339),
		CategoryID: categoryID,
		Note:       strings.TrimSpace(input.Note),
		Source:     source,
	})
	if err != nil {
		return GapFill{}, mapErr(action, err)
	}
	return toGapFill(row), nil
}

// UpdateManualEvent persists a scheduler edit to an existing manual block.
func (s *Service) UpdateManualEvent(ctx context.Context, input ManualEventUpdateInput) (GapFill, error) {
	if input.ID <= 0 {
		return GapFill{}, fmt.Errorf("update manual event: id is required")
	}

	span, err := s.manualEventSpan(ctx, "update manual event", input.ManualEventInput)
	if err != nil {
		return GapFill{}, err
	}

	categoryID := sql.NullInt64{}
	if input.CategoryID != nil {
		categoryID = sql.NullInt64{Int64: *input.CategoryID, Valid: true}
	}

	row, err := s.q.UpdateGapFill(ctx, sqlc.UpdateGapFillParams{
		Day:        input.Day,
		StartUtc:   span.start.Format(time.RFC3339),
		EndUtc:     span.end.Format(time.RFC3339),
		CategoryID: categoryID,
		Note:       strings.TrimSpace(input.Note),
		ID:         input.ID,
		PeriodID:   input.PeriodID,
	})
	if err != nil {
		return GapFill{}, mapErr("update manual event", err)
	}
	return toGapFill(row), nil
}

// DeleteManualEvent removes a user-created scheduler block.
func (s *Service) DeleteManualEvent(ctx context.Context, input ManualEventDeleteInput) error {
	if input.ID <= 0 {
		return fmt.Errorf("delete manual event: id is required")
	}
	if input.PeriodID <= 0 {
		return fmt.Errorf("delete manual event: periodId is required")
	}

	rows, err := s.q.DeleteManualGapFill(ctx, sqlc.DeleteManualGapFillParams{
		ID:       input.ID,
		PeriodID: input.PeriodID,
	})
	if err != nil {
		return mapErr("delete manual event", err)
	}
	if rows == 0 {
		return fmt.Errorf("delete manual event: %w", ErrNotFound)
	}
	return nil
}

type manualEventSpan struct {
	start time.Time
	end   time.Time
}

func (s *Service) manualEventSpan(ctx context.Context, action string, input ManualEventInput) (manualEventSpan, error) {
	if input.PeriodID <= 0 {
		return manualEventSpan{}, fmt.Errorf("%s: periodId is required", action)
	}
	if input.StartMinutes < 0 || input.StartMinutes >= 24*60 {
		return manualEventSpan{}, fmt.Errorf("%s: startMinutes must be within the day", action)
	}
	if input.EndMinutes <= input.StartMinutes || input.EndMinutes > 24*60 {
		return manualEventSpan{}, fmt.Errorf("%s: endMinutes must be after startMinutes and within the day", action)
	}

	period, err := s.GetPeriod(ctx, input.PeriodID)
	if err != nil {
		return manualEventSpan{}, err
	}
	day, err := parseDate(input.Day)
	if err != nil {
		return manualEventSpan{}, fmt.Errorf("%s: day: %w", action, err)
	}
	startDay, err := parseDate(period.StartDate)
	if err != nil {
		return manualEventSpan{}, fmt.Errorf("%s: period start_date: %w", action, err)
	}
	endDay, err := parseDate(period.EndDate)
	if err != nil {
		return manualEventSpan{}, fmt.Errorf("%s: period end_date: %w", action, err)
	}
	if day.Before(startDay) || day.After(endDay) {
		return manualEventSpan{}, fmt.Errorf("%s: day %s is outside period %s to %s", action, input.Day, period.StartDate, period.EndDate)
	}

	segs, err := s.ListTzSegments(ctx, input.PeriodID)
	if err != nil {
		return manualEventSpan{}, err
	}
	if len(segs) == 0 {
		return manualEventSpan{}, fmt.Errorf("%s: period %d has no timezone segment", action, input.PeriodID)
	}
	seg := activeSegment(segs, input.Day)
	loc, err := loadLoc(map[string]*time.Location{}, seg.IanaTz)
	if err != nil {
		return manualEventSpan{}, err
	}

	y, m, d := day.Date()
	return manualEventSpan{
		start: time.Date(y, m, d, 0, input.StartMinutes, 0, 0, loc).UTC(),
		end:   time.Date(y, m, d, 0, input.EndMinutes, 0, 0, loc).UTC(),
	}, nil
}
