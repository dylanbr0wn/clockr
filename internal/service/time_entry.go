package service

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"time"

	"github.com/dylanbr0wn/shiet/internal/db/sqlc"
)

// TimeEntryInput describes a user-authored interval on the schedule.
type TimeEntryInput struct {
	PeriodID       int64  `json:"periodId"`
	Day            string `json:"day"` // YYYY-MM-DD in the period's active local timezone
	StartMinutes   int    `json:"startMinutes"`
	EndMinutes     int    `json:"endMinutes"`
	CategoryID     *int64 `json:"categoryId,omitempty"`
	Description    string `json:"description,omitempty"`
	WorkType       string `json:"workType,omitempty"`       // empty → worked
	ProjectID      *int64 `json:"projectId,omitempty"`
	BillableStatus string `json:"billableStatus,omitempty"` // empty → unset
}

// TimeEntryUpdateInput describes an edit to an existing time entry.
type TimeEntryUpdateInput struct {
	ID int64 `json:"id"`
	TimeEntryInput
}

// TimeEntryDeleteInput identifies a time entry to remove.
type TimeEntryDeleteInput struct {
	ID       int64 `json:"id"`
	PeriodID int64 `json:"periodId"`
}

// CreateTimeEntry persists a user-authored schedule block as a confirmed TimeEntry.
func (s *Service) CreateTimeEntry(ctx context.Context, input TimeEntryInput) (TimeEntry, error) {
	return s.createTimeEntry(ctx, "create time entry", input, false)
}

// CreateGapTimeEntry persists a gap-origin confirmation as a confirmed TimeEntry
// with method=gap_fill provenance.
func (s *Service) CreateGapTimeEntry(ctx context.Context, input TimeEntryInput) (TimeEntry, error) {
	return s.createTimeEntry(ctx, "create gap time entry", input, true)
}

func (s *Service) createTimeEntry(ctx context.Context, action string, input TimeEntryInput, gapOrigin bool) (TimeEntry, error) {
	span, err := s.timeEntrySpan(ctx, action, input)
	if err != nil {
		return TimeEntry{}, err
	}
	workType, billableStatus, err := normalizeTimeEntryAllocation(action, input.WorkType, input.BillableStatus)
	if err != nil {
		return TimeEntry{}, err
	}

	categoryID := sql.NullInt64{}
	if input.CategoryID != nil {
		categoryID = sql.NullInt64{Int64: *input.CategoryID, Valid: true}
	}
	projectID := sql.NullInt64{}
	if input.ProjectID != nil {
		projectID = sql.NullInt64{Int64: *input.ProjectID, Valid: true}
	}

	params := sqlc.CreateTimeEntryParams{
		PeriodID:        input.PeriodID,
		StartInstant:    span.start.Format(time.RFC3339),
		EndInstant:      span.end.Format(time.RFC3339),
		DurationMinutes: durationMinutes(span.start, span.end),
		LocalWorkDate:   input.Day,
		CategoryID:      categoryID,
		Description:     strings.TrimSpace(input.Description),
		Attestation:     "confirmed",
		WorkType:        workType,
		ProjectID:       projectID,
		BillableStatus:  billableStatus,
	}
	if gapOrigin {
		params.Method = sql.NullString{String: "gap_fill", Valid: true}
	}

	row, err := s.q.CreateTimeEntry(ctx, params)
	if err != nil {
		return TimeEntry{}, mapErr(action, err)
	}
	return toTimeEntry(row), nil
}

// UpdateTimeEntry persists edits to an existing time entry.
func (s *Service) UpdateTimeEntry(ctx context.Context, input TimeEntryUpdateInput) (TimeEntry, error) {
	if input.ID <= 0 {
		return TimeEntry{}, fmt.Errorf("update time entry: id is required")
	}

	span, err := s.timeEntrySpan(ctx, "update time entry", input.TimeEntryInput)
	if err != nil {
		return TimeEntry{}, err
	}
	workType, billableStatus, err := normalizeTimeEntryAllocation("update time entry", input.WorkType, input.BillableStatus)
	if err != nil {
		return TimeEntry{}, err
	}

	categoryID := sql.NullInt64{}
	if input.CategoryID != nil {
		categoryID = sql.NullInt64{Int64: *input.CategoryID, Valid: true}
	}
	projectID := sql.NullInt64{}
	if input.ProjectID != nil {
		projectID = sql.NullInt64{Int64: *input.ProjectID, Valid: true}
	}

	row, err := s.q.UpdateTimeEntry(ctx, sqlc.UpdateTimeEntryParams{
		StartInstant:    span.start.Format(time.RFC3339),
		EndInstant:      span.end.Format(time.RFC3339),
		DurationMinutes: durationMinutes(span.start, span.end),
		LocalWorkDate:   input.Day,
		CategoryID:      categoryID,
		Description:     strings.TrimSpace(input.Description),
		WorkType:        workType,
		ProjectID:       projectID,
		BillableStatus:  billableStatus,
		ID:              input.ID,
		PeriodID:        input.PeriodID,
	})
	if err != nil {
		return TimeEntry{}, mapErr("update time entry", err)
	}
	return toTimeEntry(row), nil
}

// DeleteTimeEntry removes a time entry from the ledger.
func (s *Service) DeleteTimeEntry(ctx context.Context, input TimeEntryDeleteInput) error {
	if input.ID <= 0 {
		return fmt.Errorf("delete time entry: id is required")
	}
	if input.PeriodID <= 0 {
		return fmt.Errorf("delete time entry: periodId is required")
	}

	rows, err := s.q.DeleteTimeEntry(ctx, sqlc.DeleteTimeEntryParams{
		ID:       input.ID,
		PeriodID: input.PeriodID,
	})
	if err != nil {
		return mapErr("delete time entry", err)
	}
	if rows == 0 {
		return fmt.Errorf("delete time entry: %w", ErrNotFound)
	}
	return nil
}

type timeEntrySpan struct {
	start time.Time
	end   time.Time
}

func (s *Service) timeEntrySpan(ctx context.Context, action string, input TimeEntryInput) (timeEntrySpan, error) {
	if input.PeriodID <= 0 {
		return timeEntrySpan{}, invalidInputf("%s: periodId is required", action)
	}
	if input.StartMinutes < 0 || input.StartMinutes >= 24*60 {
		return timeEntrySpan{}, invalidInputf("%s: startMinutes must be within the day", action)
	}
	if input.EndMinutes <= input.StartMinutes || input.EndMinutes > 24*60 {
		return timeEntrySpan{}, invalidInputf("%s: endMinutes must be after startMinutes and within the day", action)
	}

	period, err := s.GetPeriod(ctx, input.PeriodID)
	if err != nil {
		return timeEntrySpan{}, err
	}
	day, err := parseDate(input.Day)
	if err != nil {
		return timeEntrySpan{}, invalidInputf("%s: day: %v", action, err)
	}
	startDay, err := parseDate(period.StartDate)
	if err != nil {
		return timeEntrySpan{}, fmt.Errorf("%s: period start_date: %w", action, err)
	}
	endDay, err := parseDate(period.EndDate)
	if err != nil {
		return timeEntrySpan{}, fmt.Errorf("%s: period end_date: %w", action, err)
	}
	if day.Before(startDay) || day.After(endDay) {
		return timeEntrySpan{}, invalidInputf("%s: day %s is outside period %s to %s", action, input.Day, period.StartDate, period.EndDate)
	}

	segs, err := s.ListTzSegments(ctx, input.PeriodID)
	if err != nil {
		return timeEntrySpan{}, err
	}
	if len(segs) == 0 {
		return timeEntrySpan{}, failedPreconditionf("%s: period %d has no timezone segment", action, input.PeriodID)
	}
	seg := activeSegment(segs, input.Day)
	loc, err := loadLoc(map[string]*time.Location{}, seg.IanaTz)
	if err != nil {
		return timeEntrySpan{}, err
	}

	y, m, d := day.Date()
	return timeEntrySpan{
		start: time.Date(y, m, d, 0, input.StartMinutes, 0, 0, loc).UTC(),
		end:   time.Date(y, m, d, 0, input.EndMinutes, 0, 0, loc).UTC(),
	}, nil
}

func durationMinutes(start, end time.Time) int64 {
	return int64(end.Sub(start) / time.Minute)
}

var allowedWorkTypes = map[string]struct{}{
	"worked": {}, "paid_leave": {}, "unpaid_leave": {}, "holiday": {}, "break": {}, "adjustment": {},
}

var allowedBillableStatuses = map[string]struct{}{
	"unset": {}, "billable": {}, "non_billable": {},
}

func normalizeTimeEntryAllocation(action, workType, billableStatus string) (string, string, error) {
	workType = strings.TrimSpace(workType)
	if workType == "" {
		workType = "worked"
	}
	if _, ok := allowedWorkTypes[workType]; !ok {
		return "", "", invalidInputf("%s: work_type %q is invalid", action, workType)
	}

	billableStatus = strings.TrimSpace(billableStatus)
	if billableStatus == "" {
		billableStatus = "unset"
	}
	if _, ok := allowedBillableStatuses[billableStatus]; !ok {
		return "", "", invalidInputf("%s: billable_status %q is invalid", action, billableStatus)
	}
	return workType, billableStatus, nil
}
