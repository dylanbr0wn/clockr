package service

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"time"

	"github.com/dylanbr0wn/shiet/internal/db/sqlc"
)

// ConvertAllDayEventInput turns an all-day calendar event into a draft TimeEntry
// with a user-picked interval. Confirm stays on ConfirmTimeEntry.
type ConvertAllDayEventInput struct {
	EventID int64
	TimeEntryInput
}

// ConvertAllDayEvent creates a draft TimeEntry linked to an all-day event.
// Sync never auto-materializes all-day intervals; this is the explicit path.
func (s *Service) ConvertAllDayEvent(ctx context.Context, input ConvertAllDayEventInput) (TimeEntry, error) {
	const action = "convert all-day event"
	if input.EventID <= 0 {
		return TimeEntry{}, invalidInputf("%s: eventId is required", action)
	}

	span, err := s.timeEntrySpan(ctx, action, input.TimeEntryInput)
	if err != nil {
		return TimeEntry{}, err
	}
	workType, billableStatus, err := normalizeTimeEntryAllocation(action, input.WorkType, input.BillableStatus)
	if err != nil {
		return TimeEntry{}, err
	}

	ev, err := s.q.GetEvent(ctx, input.EventID)
	if err != nil {
		return TimeEntry{}, mapErr("get event", err)
	}
	if ev.PeriodID != input.PeriodID {
		return TimeEntry{}, fmt.Errorf("%s: %w", action, ErrNotFound)
	}
	if ev.AllDay == 0 {
		return TimeEntry{}, failedPreconditionf("%s: event %d is not all-day", action, ev.ID)
	}
	if ev.Active == 0 {
		return TimeEntry{}, failedPreconditionf("%s: event %d is inactive", action, ev.ID)
	}

	excluded, err := s.review().HasStatusExcluded(ctx, s.q, ev)
	if err != nil {
		return TimeEntry{}, err
	}
	if excluded {
		return TimeEntry{}, failedPreconditionf("%s: event %d is excluded from the schedule", action, ev.ID)
	}

	identity := eventIdentityFromRow(ev)
	entries, err := s.q.ListTimeEntriesForPeriod(ctx, input.PeriodID)
	if err != nil {
		return TimeEntry{}, mapErr("list time entries", err)
	}
	linked := linkedCalendarEntries(entries, identity)
	liveDrafts, hasConfirmed, _ := partitionLinked(linked)
	if hasConfirmed || len(liveDrafts) > 0 {
		return TimeEntry{}, failedPreconditionf("%s: event already has a live TimeEntry; adjust or confirm that draft instead", action)
	}

	categoryID := sql.NullInt64{}
	if input.CategoryID != nil {
		categoryID = sql.NullInt64{Int64: *input.CategoryID, Valid: true}
	} else {
		categoryID, err = effectiveCategory(ctx, s.q, input.PeriodID, IncomingEvent{
			Provider:   ev.Provider,
			ExternalID: ev.ExternalID,
			InstanceID: ev.InstanceID,
		})
		if err != nil {
			return TimeEntry{}, err
		}
	}

	description := strings.TrimSpace(input.Description)
	if description == "" {
		description = ev.Title
	}

	projectID := sql.NullInt64{}
	if input.ProjectID != nil {
		projectID = sql.NullInt64{Int64: *input.ProjectID, Valid: true}
	}

	row, err := s.q.CreateTimeEntry(ctx, sqlc.CreateTimeEntryParams{
		PeriodID:        input.PeriodID,
		StartInstant:    span.start.Format(time.RFC3339),
		EndInstant:      span.end.Format(time.RFC3339),
		DurationMinutes: durationMinutes(span.start, span.end),
		LocalWorkDate:   input.Day,
		CategoryID:      categoryID,
		Description:     description,
		Attestation:     "draft",
		SourceKind:      sql.NullString{String: SourceKindCalendarEvent, Valid: true},
		SourceID:        sql.NullString{String: identity, Valid: true},
		SourceRevision:  sql.NullString{String: ev.SourceHash, Valid: true},
		Method:          sql.NullString{String: MethodCalendarConvert, Valid: true},
		WorkType:        workType,
		ProjectID:       projectID,
		BillableStatus:  billableStatus,
	})
	if err != nil {
		return TimeEntry{}, mapErr(action, err)
	}
	return toTimeEntry(row), nil
}
