package service

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/dylanbr0wn/shiet/internal/db/sqlc"
)

// materializeDraftForEvent creates or refreshes a calendar_import draft for a
// schedule-included timed event. Confirmed rows are never reshaped here.
func (s *Service) materializeDraftForEvent(
	ctx context.Context,
	q *sqlc.Queries,
	periodID int64,
	inc IncomingEvent,
	ev sqlc.Event,
	entries *[]sqlc.TimeEntry,
	materialChange bool,
) error {
	if !draftEligible(inc) {
		return nil
	}

	excluded, err := s.review().HasStatusExcluded(ctx, q, ev)
	if err != nil {
		return err
	}
	if excluded {
		return dismissLinkedDrafts(ctx, q, linkedCalendarEntries(*entries, eventIdentityFromIncoming(inc)))
	}

	if inc.Status == "tentative" || inc.Status == "needsAction" {
		open, err := hasOpenTentativeReview(ctx, q, periodID, ev.ID)
		if err != nil {
			return err
		}
		if open {
			return nil
		}
	}

	identity := eventIdentityFromIncoming(inc)
	linked := linkedCalendarEntries(*entries, identity)
	liveDrafts, hasConfirmed, allDismissed := partitionLinked(linked)

	if hasConfirmed || len(liveDrafts) > 0 {
		for i := range liveDrafts {
			if err := refreshDraftFromEvent(ctx, q, &liveDrafts[i], ev, inc, periodID); err != nil {
				return err
			}
		}
		return nil
	}

	if allDismissed {
		if !materialChange {
			for _, te := range linked {
				if err := q.UpdateTimeEntrySourceRevision(ctx, sqlc.UpdateTimeEntrySourceRevisionParams{
					SourceRevision: sql.NullString{String: ev.SourceHash, Valid: true},
					ID:             te.ID,
					PeriodID:       te.PeriodID,
				}); err != nil {
					return fmt.Errorf("bump dismissed revision %d: %w", te.ID, err)
				}
			}
			return nil
		}
		// Material change: re-propose one full-span draft.
	}

	return createDraftFromEvent(ctx, q, periodID, inc, ev, identity, entries)
}

func draftEligible(inc IncomingEvent) bool {
	return !inc.AllDay && inc.Start != nil && inc.End != nil
}

func linkedCalendarEntries(entries []sqlc.TimeEntry, identity string) []sqlc.TimeEntry {
	out := make([]sqlc.TimeEntry, 0)
	for _, te := range entries {
		if !te.SourceKind.Valid || te.SourceKind.String != SourceKindCalendarEvent {
			continue
		}
		if !te.SourceID.Valid || te.SourceID.String != identity {
			continue
		}
		out = append(out, te)
	}
	return out
}

func partitionLinked(linked []sqlc.TimeEntry) (drafts []sqlc.TimeEntry, hasConfirmed, allDismissed bool) {
	if len(linked) == 0 {
		return nil, false, false
	}
	allDismissed = true
	for _, te := range linked {
		switch te.Attestation {
		case "draft":
			drafts = append(drafts, te)
			allDismissed = false
		case "confirmed":
			hasConfirmed = true
			allDismissed = false
		case "dismissed":
			// keep scanning
		default:
			allDismissed = false
		}
	}
	return drafts, hasConfirmed, allDismissed
}

func createDraftFromEvent(
	ctx context.Context,
	q *sqlc.Queries,
	periodID int64,
	inc IncomingEvent,
	ev sqlc.Event,
	identity string,
	entries *[]sqlc.TimeEntry,
) error {
	day, err := eventLocalDay(ctx, q, ev)
	if err != nil {
		return err
	}

	categoryID, err := effectiveCategory(ctx, q, periodID, inc)
	if err != nil {
		return err
	}

	start := inc.Start.UTC()
	end := inc.End.UTC()
	row, err := q.CreateTimeEntry(ctx, sqlc.CreateTimeEntryParams{
		PeriodID:        periodID,
		StartInstant:    start.Format(time.RFC3339),
		EndInstant:      end.Format(time.RFC3339),
		DurationMinutes: durationMinutes(start, end),
		LocalWorkDate:   day,
		CategoryID:      categoryID,
		Description:     inc.Title,
		Attestation:     "draft",
		SourceKind:      sql.NullString{String: SourceKindCalendarEvent, Valid: true},
		SourceID:        sql.NullString{String: identity, Valid: true},
		SourceRevision:  sql.NullString{String: ev.SourceHash, Valid: true},
		Method:          sql.NullString{String: MethodCalendarImport, Valid: true},
		WorkType:        "worked",
		BillableStatus:  "unset",
	})
	if err != nil {
		return fmt.Errorf("materialize draft for %s: %w", identity, err)
	}
	*entries = append(*entries, row)
	return nil
}

func refreshDraftFromEvent(
	ctx context.Context,
	q *sqlc.Queries,
	draft *sqlc.TimeEntry,
	ev sqlc.Event,
	inc IncomingEvent,
	periodID int64,
) error {
	if draft.SourceRevision.Valid && draft.SourceRevision.String == ev.SourceHash {
		return nil
	}
	day, err := eventLocalDay(ctx, q, ev)
	if err != nil {
		return err
	}
	categoryID, err := effectiveCategory(ctx, q, periodID, inc)
	if err != nil {
		return err
	}
	// Keep a user-edited draft category if set and overlay empty; otherwise copy overlay.
	if !categoryID.Valid {
		categoryID = draft.CategoryID
	}
	start := inc.Start.UTC()
	end := inc.End.UTC()
	updated, err := q.UpdateTimeEntryCalendarDraft(ctx, sqlc.UpdateTimeEntryCalendarDraftParams{
		StartInstant:    start.Format(time.RFC3339),
		EndInstant:      end.Format(time.RFC3339),
		DurationMinutes: durationMinutes(start, end),
		LocalWorkDate:   day,
		Description:     inc.Title,
		SourceRevision:  sql.NullString{String: ev.SourceHash, Valid: true},
		CategoryID:      categoryID,
		ID:              draft.ID,
		PeriodID:        draft.PeriodID,
	})
	if err != nil {
		return fmt.Errorf("refresh draft %d: %w", draft.ID, err)
	}
	*draft = updated
	return nil
}

func dismissLinkedDrafts(ctx context.Context, q *sqlc.Queries, linked []sqlc.TimeEntry) error {
	for _, te := range linked {
		if te.Attestation != "draft" {
			continue
		}
		if _, err := q.UpdateTimeEntryAttestation(ctx, sqlc.UpdateTimeEntryAttestationParams{
			Attestation: "dismissed",
			ID:          te.ID,
			PeriodID:    te.PeriodID,
		}); err != nil {
			return fmt.Errorf("dismiss draft %d on exclude: %w", te.ID, err)
		}
	}
	return nil
}

func effectiveCategory(ctx context.Context, q *sqlc.Queries, periodID int64, inc IncomingEvent) (sql.NullInt64, error) {
	o, err := q.GetOverlay(ctx, sqlc.GetOverlayParams{
		PeriodID:   periodID,
		Provider:   inc.Provider,
		ExternalID: inc.ExternalID,
		InstanceID: inc.InstanceID,
		Kind:       overlayKindCategory,
	})
	if errors.Is(err, sql.ErrNoRows) {
		return sql.NullInt64{}, nil
	}
	if err != nil {
		return sql.NullInt64{}, fmt.Errorf("effective category: %w", err)
	}
	return o.CategoryID, nil
}

func hasOpenTentativeReview(ctx context.Context, q *sqlc.Queries, periodID, eventID int64) (bool, error) {
	items, err := q.ListOpenReviewItems(ctx, periodID)
	if err != nil {
		return false, fmt.Errorf("list open reviews: %w", err)
	}
	for _, it := range items {
		if it.Kind != reviewTentative {
			continue
		}
		if it.EventID.Valid && it.EventID.Int64 == eventID {
			return true, nil
		}
	}
	return false, nil
}

// titleMaterialChange reports whether the title changed (material per lifecycle).
func titleMaterialChange(ex sqlc.Event, inc IncomingEvent) bool {
	return normalizeTitle(ex.Title) != normalizeTitle(inc.Title)
}

// scheduleIncludeDropped is a material change: status flipped away from a
// schedule-included attendance state (e.g. accepted → declined).
func scheduleIncludeDropped(ex sqlc.Event, inc IncomingEvent) bool {
	wasIncluded := ex.Status == "accepted" || ex.Status == ""
	nowExcluded := inc.Status == "declined" || inc.Status == "tentative" || inc.Status == "needsAction"
	return wasIncluded && nowExcluded
}
