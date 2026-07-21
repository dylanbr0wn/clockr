package service

import (
	"context"
	"time"

	"github.com/dylanbr0wn/shiet/internal/db/sqlc"
)

const (
	// OvernightAttributeToStart keeps one confirmed entry bucketed on the start date.
	OvernightAttributeToStart = "attribute_to_start"
	// OvernightSplitAtMidnight confirms midnight-cut segments and dismisses the original draft.
	OvernightSplitAtMidnight = "split_at_midnight"

	// OverlapAllowParallel confirms despite overlapping confirmed entries (one-shot).
	OverlapAllowParallel = "allow_parallel"
	// OverlapKeepTheirs dismisses the draft and leaves overlapping confirmed entries.
	OverlapKeepTheirs = "keep_theirs"
	// OverlapKeepMine deletes overlapping confirmed entries, then confirms the draft.
	OverlapKeepMine = "keep_mine"
	// OverlapSplit confirms only the non-overlapping remainder(s) of the draft.
	OverlapSplit = "split"
)

// ConfirmTimeEntryInput identifies a draft TimeEntry to promote to confirmed.
type ConfirmTimeEntryInput struct {
	ID                int64  `json:"id"`
	PeriodID          int64  `json:"periodId"`
	OvernightPolicy   string `json:"overnightPolicy,omitempty"`   // required when span crosses local midnight
	OverlapResolution string `json:"overlapResolution,omitempty"` // required when overlapping confirmed entries
}

// ConfirmTimeEntry promotes a draft TimeEntry to confirmed.
// Returns the resulting confirmed entries (one for same-day / attribute-to-start;
// many for split-at-midnight).
func (s *Service) ConfirmTimeEntry(ctx context.Context, input ConfirmTimeEntryInput) ([]TimeEntry, error) {
	const action = "confirm time entry"
	if input.ID <= 0 {
		return nil, invalidInputf("%s: id is required", action)
	}
	if input.PeriodID <= 0 {
		return nil, invalidInputf("%s: periodId is required", action)
	}

	row, err := s.q.GetTimeEntry(ctx, sqlc.GetTimeEntryParams{
		ID:       input.ID,
		PeriodID: input.PeriodID,
	})
	if err != nil {
		return nil, mapErr(action, err)
	}
	if row.Attestation != "draft" {
		return nil, failedPreconditionf("%s: entry %d is %s, not draft", action, input.ID, row.Attestation)
	}

	start, end, loc, err := s.timeEntryLocalSpan(ctx, action, row)
	if err != nil {
		return nil, err
	}

	overlaps, err := s.overlappingConfirmed(ctx, row.PeriodID, row.ID, start, end)
	if err != nil {
		return nil, err
	}
	if len(overlaps) > 0 {
		switch input.OverlapResolution {
		case OverlapAllowParallel:
			// proceed
		case OverlapKeepTheirs:
			if _, err := s.q.UpdateTimeEntryAttestation(ctx, sqlc.UpdateTimeEntryAttestationParams{
				Attestation: "dismissed",
				ID:          row.ID,
				PeriodID:    row.PeriodID,
			}); err != nil {
				return nil, mapErr(action, err)
			}
			return nil, nil
		case OverlapKeepMine:
			for _, other := range overlaps {
				if _, err := s.q.DeleteTimeEntry(ctx, sqlc.DeleteTimeEntryParams{
					ID:       other.ID,
					PeriodID: other.PeriodID,
				}); err != nil {
					return nil, mapErr(action, err)
				}
			}
		case OverlapSplit:
			return s.confirmDraftAroundOverlaps(ctx, action, row, start, end, overlaps, loc, input.OvernightPolicy)
		case "":
			return nil, failedPreconditionf("%s: overlaps %d confirmed entr(y/ies); set overlap_resolution (%s, %s, %s, or %s)",
				action, len(overlaps), OverlapKeepMine, OverlapKeepTheirs, OverlapSplit, OverlapAllowParallel)
		default:
			return nil, invalidInputf("%s: unknown overlap_resolution %q", action, input.OverlapResolution)
		}
	} else if input.OverlapResolution != "" {
		return nil, invalidInputf("%s: overlap_resolution is only valid when overlaps exist", action)
	}

	overnight := crossesLocalMidnight(start.In(loc), end.In(loc))

	if overnight {
		switch input.OvernightPolicy {
		case OvernightAttributeToStart:
			return s.confirmInPlace(ctx, action, input)
		case OvernightSplitAtMidnight:
			return s.confirmSplitAtMidnight(ctx, action, row, start, end, loc)
		case "":
			return nil, invalidInputf("%s: overnight/multi-day entry requires overnight_policy (%s or %s)",
				action, OvernightAttributeToStart, OvernightSplitAtMidnight)
		default:
			return nil, invalidInputf("%s: unknown overnight_policy %q", action, input.OvernightPolicy)
		}
	}
	if input.OvernightPolicy != "" {
		return nil, invalidInputf("%s: overnight_policy is only valid for overnight/multi-day entries", action)
	}
	return s.confirmInPlace(ctx, action, input)
}

func (s *Service) confirmInPlace(ctx context.Context, action string, input ConfirmTimeEntryInput) ([]TimeEntry, error) {
	updated, err := s.q.UpdateTimeEntryAttestation(ctx, sqlc.UpdateTimeEntryAttestationParams{
		Attestation: "confirmed",
		ID:          input.ID,
		PeriodID:    input.PeriodID,
	})
	if err != nil {
		return nil, mapErr(action, err)
	}
	return []TimeEntry{toTimeEntry(updated)}, nil
}

func (s *Service) confirmSplitAtMidnight(
	ctx context.Context,
	action string,
	row sqlc.TimeEntry,
	start, end time.Time,
	loc *time.Location,
) ([]TimeEntry, error) {
	segments := splitAtLocalMidnights(start.In(loc), end.In(loc))
	if len(segments) < 2 {
		return nil, failedPreconditionf("%s: split_at_midnight produced fewer than 2 segments", action)
	}

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, mapErr(action, err)
	}
	defer func() { _ = tx.Rollback() }()
	q := s.q.WithTx(tx)

	out := make([]TimeEntry, 0, len(segments))
	for _, seg := range segments {
		created, err := q.CreateTimeEntry(ctx, sqlc.CreateTimeEntryParams{
			PeriodID:        row.PeriodID,
			StartInstant:    seg.start.UTC().Format(time.RFC3339),
			EndInstant:      seg.end.UTC().Format(time.RFC3339),
			DurationMinutes: durationMinutes(seg.start, seg.end),
			LocalWorkDate:   seg.start.Format("2006-01-02"),
			CategoryID:      row.CategoryID,
			Description:     row.Description,
			Attestation:     "confirmed",
			SourceKind:      row.SourceKind,
			SourceID:        row.SourceID,
			SourceRevision:  row.SourceRevision,
			Method:          row.Method,
			WorkType:        row.WorkType,
			ProjectID:       row.ProjectID,
			BillableStatus:  row.BillableStatus,
		})
		if err != nil {
			return nil, mapErr(action, err)
		}
		out = append(out, toTimeEntry(created))
	}

	if _, err := q.UpdateTimeEntryAttestation(ctx, sqlc.UpdateTimeEntryAttestationParams{
		Attestation: "dismissed",
		ID:          row.ID,
		PeriodID:    row.PeriodID,
	}); err != nil {
		return nil, mapErr(action, err)
	}

	if err := tx.Commit(); err != nil {
		return nil, mapErr(action, err)
	}
	return out, nil
}

func (s *Service) overlappingConfirmed(ctx context.Context, periodID, excludeID int64, start, end time.Time) ([]sqlc.TimeEntry, error) {
	rows, err := s.q.ListTimeEntriesForPeriod(ctx, periodID)
	if err != nil {
		return nil, mapErr("confirm time entry", err)
	}
	var out []sqlc.TimeEntry
	for _, r := range rows {
		if r.ID == excludeID || r.Attestation != "confirmed" {
			continue
		}
		otherStart, err := time.Parse(time.RFC3339, r.StartInstant)
		if err != nil {
			continue
		}
		otherEnd, err := time.Parse(time.RFC3339, r.EndInstant)
		if err != nil {
			continue
		}
		if otherStart.Before(end) && otherEnd.After(start) {
			out = append(out, r)
		}
	}
	return out, nil
}

// confirmDraftAroundOverlaps confirms the draft's non-overlapping remainders and dismisses the original.
func (s *Service) confirmDraftAroundOverlaps(
	ctx context.Context,
	action string,
	row sqlc.TimeEntry,
	start, end time.Time,
	overlaps []sqlc.TimeEntry,
	loc *time.Location,
	overnightPolicy string,
) ([]TimeEntry, error) {
	remainders := []localSpan{{start: start, end: end}}
	for _, other := range overlaps {
		otherStart, err := time.Parse(time.RFC3339, other.StartInstant)
		if err != nil {
			return nil, failedPreconditionf("%s: overlap start: %v", action, err)
		}
		otherEnd, err := time.Parse(time.RFC3339, other.EndInstant)
		if err != nil {
			return nil, failedPreconditionf("%s: overlap end: %v", action, err)
		}
		remainders = subtractInterval(remainders, otherStart, otherEnd)
	}
	if len(remainders) == 0 {
		if _, err := s.q.UpdateTimeEntryAttestation(ctx, sqlc.UpdateTimeEntryAttestationParams{
			Attestation: "dismissed",
			ID:          row.ID,
			PeriodID:    row.PeriodID,
		}); err != nil {
			return nil, mapErr(action, err)
		}
		return nil, nil
	}

	// Overnight gate still applies to each remainder that crosses midnight.
	for _, rem := range remainders {
		if crossesLocalMidnight(rem.start.In(loc), rem.end.In(loc)) && overnightPolicy == "" {
			return nil, invalidInputf("%s: overnight remainder requires overnight_policy", action)
		}
	}

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, mapErr(action, err)
	}
	defer func() { _ = tx.Rollback() }()
	q := s.q.WithTx(tx)

	out := make([]TimeEntry, 0, len(remainders))
	for _, rem := range remainders {
		pieces := []localSpan{rem}
		if overnightPolicy == OvernightSplitAtMidnight && crossesLocalMidnight(rem.start.In(loc), rem.end.In(loc)) {
			pieces = splitAtLocalMidnights(rem.start.In(loc), rem.end.In(loc))
		}
		for _, piece := range pieces {
			localStart := piece.start.In(loc)
			created, err := q.CreateTimeEntry(ctx, sqlc.CreateTimeEntryParams{
				PeriodID:        row.PeriodID,
				StartInstant:    piece.start.UTC().Format(time.RFC3339),
				EndInstant:      piece.end.UTC().Format(time.RFC3339),
				DurationMinutes: durationMinutes(piece.start, piece.end),
				LocalWorkDate:   localStart.Format("2006-01-02"),
				CategoryID:      row.CategoryID,
				Description:     row.Description,
				Attestation:     "confirmed",
				SourceKind:      row.SourceKind,
				SourceID:        row.SourceID,
				SourceRevision:  row.SourceRevision,
				Method:          row.Method,
				WorkType:        row.WorkType,
				ProjectID:       row.ProjectID,
				BillableStatus:  row.BillableStatus,
			})
			if err != nil {
				return nil, mapErr(action, err)
			}
			out = append(out, toTimeEntry(created))
		}
	}

	if _, err := q.UpdateTimeEntryAttestation(ctx, sqlc.UpdateTimeEntryAttestationParams{
		Attestation: "dismissed",
		ID:          row.ID,
		PeriodID:    row.PeriodID,
	}); err != nil {
		return nil, mapErr(action, err)
	}
	if err := tx.Commit(); err != nil {
		return nil, mapErr(action, err)
	}
	return out, nil
}

func subtractInterval(spans []localSpan, cutStart, cutEnd time.Time) []localSpan {
	var out []localSpan
	for _, sp := range spans {
		if !cutStart.Before(sp.end) || !cutEnd.After(sp.start) {
			out = append(out, sp)
			continue
		}
		if sp.start.Before(cutStart) {
			out = append(out, localSpan{start: sp.start, end: cutStart})
		}
		if cutEnd.Before(sp.end) {
			out = append(out, localSpan{start: cutEnd, end: sp.end})
		}
	}
	return out
}

func (s *Service) timeEntryLocalSpan(ctx context.Context, action string, row sqlc.TimeEntry) (time.Time, time.Time, *time.Location, error) {
	start, err := time.Parse(time.RFC3339, row.StartInstant)
	if err != nil {
		return time.Time{}, time.Time{}, nil, failedPreconditionf("%s: start_instant: %v", action, err)
	}
	end, err := time.Parse(time.RFC3339, row.EndInstant)
	if err != nil {
		return time.Time{}, time.Time{}, nil, failedPreconditionf("%s: end_instant: %v", action, err)
	}
	segs, err := s.ListTzSegments(ctx, row.PeriodID)
	if err != nil {
		return time.Time{}, time.Time{}, nil, err
	}
	if len(segs) == 0 {
		return time.Time{}, time.Time{}, nil, failedPreconditionf("%s: period %d has no timezone segment", action, row.PeriodID)
	}
	seg := activeSegment(segs, row.LocalWorkDate)
	loc, err := loadLoc(map[string]*time.Location{}, seg.IanaTz)
	if err != nil {
		return time.Time{}, time.Time{}, nil, err
	}
	return start, end, loc, nil
}

func crossesLocalMidnight(localStart, localEnd time.Time) bool {
	sy, sm, sd := localStart.Date()
	ey, em, ed := localEnd.Date()
	return ey != sy || em != sm || ed != sd
}

type localSpan struct {
	start time.Time
	end   time.Time
}

func splitAtLocalMidnights(localStart, localEnd time.Time) []localSpan {
	var out []localSpan
	cur := localStart
	for cur.Before(localEnd) {
		y, m, d := cur.Date()
		nextMidnight := time.Date(y, m, d+1, 0, 0, 0, 0, cur.Location())
		segEnd := nextMidnight
		if segEnd.After(localEnd) {
			segEnd = localEnd
		}
		if segEnd.After(cur) {
			out = append(out, localSpan{start: cur, end: segEnd})
		}
		cur = segEnd
	}
	return out
}

// RejectTimeEntryInput identifies a draft TimeEntry to soft-reject.
type RejectTimeEntryInput struct {
	ID       int64 `json:"id"`
	PeriodID int64 `json:"periodId"`
}

// RejectTimeEntry soft-rejects a draft TimeEntry (attestation → dismissed).
func (s *Service) RejectTimeEntry(ctx context.Context, input RejectTimeEntryInput) (TimeEntry, error) {
	const action = "reject time entry"
	if input.ID <= 0 {
		return TimeEntry{}, invalidInputf("%s: id is required", action)
	}
	if input.PeriodID <= 0 {
		return TimeEntry{}, invalidInputf("%s: periodId is required", action)
	}

	row, err := s.q.GetTimeEntry(ctx, sqlc.GetTimeEntryParams{
		ID:       input.ID,
		PeriodID: input.PeriodID,
	})
	if err != nil {
		return TimeEntry{}, mapErr(action, err)
	}
	if row.Attestation != "draft" {
		return TimeEntry{}, failedPreconditionf("%s: entry %d is %s, not draft", action, input.ID, row.Attestation)
	}

	updated, err := s.q.UpdateTimeEntryAttestation(ctx, sqlc.UpdateTimeEntryAttestationParams{
		Attestation: "dismissed",
		ID:          input.ID,
		PeriodID:    input.PeriodID,
	})
	if err != nil {
		return TimeEntry{}, mapErr(action, err)
	}
	return toTimeEntry(updated), nil
}

// AdjustDraftTimeEntry edits span/allocation on a draft TimeEntry only.
func (s *Service) AdjustDraftTimeEntry(ctx context.Context, input TimeEntryUpdateInput) (TimeEntry, error) {
	const action = "adjust draft time entry"
	if input.ID <= 0 {
		return TimeEntry{}, invalidInputf("%s: id is required", action)
	}

	row, err := s.q.GetTimeEntry(ctx, sqlc.GetTimeEntryParams{
		ID:       input.ID,
		PeriodID: input.PeriodID,
	})
	if err != nil {
		return TimeEntry{}, mapErr(action, err)
	}
	if row.Attestation != "draft" {
		return TimeEntry{}, failedPreconditionf("%s: entry %d is %s, not draft", action, input.ID, row.Attestation)
	}

	return s.UpdateTimeEntry(ctx, input)
}

// SplitTimeEntryInput cuts a draft into multiple draft segments at UTC cut points.
type SplitTimeEntryInput struct {
	ID        int64    `json:"id"`
	PeriodID  int64    `json:"periodId"`
	CutPoints []string `json:"cutPoints"` // RFC3339 UTC instants strictly inside (start, end)
}

// SplitTimeEntry replaces a draft with N draft segments at the given cut points.
// The original draft is dismissed. Cut points must be sorted ascending and
// strictly inside the entry span.
func (s *Service) SplitTimeEntry(ctx context.Context, input SplitTimeEntryInput) ([]TimeEntry, error) {
	const action = "split time entry"
	if input.ID <= 0 {
		return nil, invalidInputf("%s: id is required", action)
	}
	if input.PeriodID <= 0 {
		return nil, invalidInputf("%s: periodId is required", action)
	}
	if len(input.CutPoints) == 0 {
		return nil, invalidInputf("%s: at least one cut point is required", action)
	}

	row, err := s.q.GetTimeEntry(ctx, sqlc.GetTimeEntryParams{
		ID:       input.ID,
		PeriodID: input.PeriodID,
	})
	if err != nil {
		return nil, mapErr(action, err)
	}
	if row.Attestation != "draft" {
		return nil, failedPreconditionf("%s: entry %d is %s, not draft", action, input.ID, row.Attestation)
	}

	start, end, loc, err := s.timeEntryLocalSpan(ctx, action, row)
	if err != nil {
		return nil, err
	}

	cuts := make([]time.Time, 0, len(input.CutPoints))
	prev := start
	for i, raw := range input.CutPoints {
		cut, err := time.Parse(time.RFC3339, raw)
		if err != nil {
			return nil, invalidInputf("%s: cut_points[%d]: %v", action, i, err)
		}
		if !cut.After(prev) || !cut.Before(end) {
			return nil, invalidInputf("%s: cut_points must be strictly inside the span and ascending", action)
		}
		cuts = append(cuts, cut)
		prev = cut
	}

	bounds := make([]time.Time, 0, len(cuts)+2)
	bounds = append(bounds, start)
	bounds = append(bounds, cuts...)
	bounds = append(bounds, end)

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, mapErr(action, err)
	}
	defer func() { _ = tx.Rollback() }()
	q := s.q.WithTx(tx)

	out := make([]TimeEntry, 0, len(bounds)-1)
	for i := 0; i < len(bounds)-1; i++ {
		segStart, segEnd := bounds[i], bounds[i+1]
		created, err := q.CreateTimeEntry(ctx, sqlc.CreateTimeEntryParams{
			PeriodID:        row.PeriodID,
			StartInstant:    segStart.UTC().Format(time.RFC3339),
			EndInstant:      segEnd.UTC().Format(time.RFC3339),
			DurationMinutes: durationMinutes(segStart, segEnd),
			LocalWorkDate:   segStart.In(loc).Format("2006-01-02"),
			CategoryID:      row.CategoryID,
			Description:     row.Description,
			Attestation:     "draft",
			SourceKind:      row.SourceKind,
			SourceID:        row.SourceID,
			SourceRevision:  row.SourceRevision,
			Method:          row.Method,
			WorkType:        row.WorkType,
			ProjectID:       row.ProjectID,
			BillableStatus:  row.BillableStatus,
		})
		if err != nil {
			return nil, mapErr(action, err)
		}
		out = append(out, toTimeEntry(created))
	}

	if _, err := q.UpdateTimeEntryAttestation(ctx, sqlc.UpdateTimeEntryAttestationParams{
		Attestation: "dismissed",
		ID:          row.ID,
		PeriodID:    row.PeriodID,
	}); err != nil {
		return nil, mapErr(action, err)
	}
	if err := tx.Commit(); err != nil {
		return nil, mapErr(action, err)
	}
	return out, nil
}