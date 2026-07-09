package service

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/dylanbr0wn/shiet/internal/db/sqlc"
)

// IncomingEvent is one event from a fresh calendar pull, decoupled from the
// Google API types. The sync layer merges a batch of these into the stored
// facts for a period without ever destroying a user decision (overlays / gap
// fills). For timed events Start/End are set; for all-day events StartDate/
// EndDate (date-only YYYY-MM-DD) are set instead.
type IncomingEvent struct {
	CalendarID       int64
	Provider         string
	ExternalID       string
	InstanceID       string
	RecurringEventID string
	ICalUID          string
	Title            string
	Description      string
	Location         string
	Organizer        string
	Attendees        []Attendee
	Status           string // accepted | declined | tentative | needsAction
	AllDay           bool
	Start            *time.Time
	End              *time.Time
	StartDate        string
	EndDate          string
	OriginalTz       string
}

// SyncResult summarizes what a sync did. Flagged counts review items created.
type SyncResult struct {
	Added     int `json:"added"`
	Updated   int `json:"updated"`
	Unchanged int `json:"unchanged"`
	Removed   int `json:"removed"`
	Flagged   int `json:"flagged"`
}

// review_item.kind values (must match the schema CHECK constraint).
const (
	reviewNewInGap         = "new_in_gap"
	reviewTitleChanged     = "title_changed"
	reviewDeletedCategoriz = "deleted_categorized"
	reviewTentative        = "tentative"
	reviewAllDay           = "all_day"
)

// overlay.kind for a category decision.
const (
	overlayKindCategory  = "category"
	overlayKindStatus    = "status"
	overlayStatusExclude = "excluded"
)

// SyncEvents runs the 3-way merge for a period (DESIGN.md "Re-sync"):
//   - base  = events currently stored for the period (last import)
//   - theirs = the incoming pull
//   - mine  = user overlays + gap fills (never destroyed)
//
// Safe changes auto-apply; genuine conflicts become review items. The whole
// merge runs in one transaction.
func (s *Service) SyncEvents(ctx context.Context, periodID int64, incoming []IncomingEvent) (SyncResult, error) {
	var res SyncResult

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return res, fmt.Errorf("begin sync tx: %w", err)
	}
	defer func() { _ = tx.Rollback() }() // no-op after Commit

	q := s.q.WithTx(tx)

	base, err := q.ListAllEventsForPeriod(ctx, periodID)
	if err != nil {
		return res, fmt.Errorf("load base events: %w", err)
	}
	baseByKey := make(map[string]sqlc.Event, len(base))
	for _, e := range base {
		baseByKey[eventKey(e.CalendarID, e.ExternalID, e.InstanceID)] = e
	}

	gaps, err := q.ListGapFillsForPeriod(ctx, periodID)
	if err != nil {
		return res, fmt.Errorf("load gap fills: %w", err)
	}

	seen := make(map[string]struct{}, len(incoming))
	for _, inc := range incoming {
		key := eventKey(inc.CalendarID, inc.ExternalID, inc.InstanceID)
		seen[key] = struct{}{}

		params, hash := upsertParams(periodID, inc)
		ex, exists := baseByKey[key]

		if !exists {
			ev, err := q.UpsertEvent(ctx, params)
			if err != nil {
				return res, fmt.Errorf("insert event %s: %w", inc.ExternalID, err)
			}
			res.Added++
			if err := s.handleNewEvent(ctx, q, periodID, inc, ev.ID, gaps, &res); err != nil {
				return res, err
			}
			continue
		}

		if ex.SourceHash == hash {
			res.Unchanged++
			continue
		}

		// Changed: update the fact (overlay untouched), then classify.
		ev, err := q.UpsertEvent(ctx, params)
		if err != nil {
			return res, fmt.Errorf("update event %s: %w", inc.ExternalID, err)
		}
		res.Updated++
		if err := s.handleChangedEvent(ctx, q, periodID, ev.ID, ex, inc, &res); err != nil {
			return res, err
		}
	}

	// Events that vanished from the pull.
	for _, e := range base {
		if _, ok := seen[eventKey(e.CalendarID, e.ExternalID, e.InstanceID)]; ok {
			continue
		}
		excluded, err := hasStatusExcluded(ctx, q, e)
		if err != nil {
			return res, err
		}
		if excluded {
			continue
		}
		categorized, err := hasCategory(ctx, q, periodID, e.Provider, e.ExternalID, e.InstanceID)
		if err != nil {
			return res, err
		}
		if categorized {
			// Never silently drop a categorized event — keep the fact, queue it
			// unless the same conflict was already resolved.
			action, created, err := s.enqueueIfUnresolved(ctx, q, periodID, reviewDeletedCategoriz, e.ID,
				reviewConflictKey(eventIdentityFromRow(e), "deleted"),
				map[string]any{"reason": "deleted", "title": e.Title})
			if err != nil {
				return res, err
			}
			if created {
				res.Flagged++
			}
			if action == ReviewActionDropEntry || action == ReviewActionKeepEntry {
				if err := s.resolveDeletedCategorized(ctx, q, sqlc.ReviewItem{
					PeriodID: periodID,
					Kind:     reviewDeletedCategoriz,
					EventID:  sql.NullInt64{Int64: e.ID, Valid: true},
				}, action); err != nil {
					return res, err
				}
				if action == ReviewActionDropEntry {
					res.Removed++
				}
			}
		} else {
			if err := q.DeleteEvent(ctx, e.ID); err != nil {
				return res, fmt.Errorf("delete event %d: %w", e.ID, err)
			}
			res.Removed++
		}
	}

	if err := tx.Commit(); err != nil {
		return res, fmt.Errorf("commit sync tx: %w", err)
	}
	return res, nil
}

// handleNewEvent flags a new event when it conflicts / needs resolution, then
// auto-categorizes from memory. Flags are mutually exclusive (gap takes
// precedence) to keep the review queue quiet.
func (s *Service) handleNewEvent(ctx context.Context, q *sqlc.Queries, periodID int64, inc IncomingEvent, eventID int64, gaps []sqlc.GapFill, res *SyncResult) error {
	identity := eventIdentityFromIncoming(inc)
	gapFingerprint, overlapsGap := overlappingGapFingerprint(inc, gaps)
	switch {
	case overlapsGap:
		action, created, err := s.enqueueIfUnresolved(ctx, q, periodID, reviewNewInGap, eventID,
			reviewConflictKey(identity, gapFingerprint),
			map[string]any{"title": inc.Title})
		if err != nil {
			return err
		}
		if created {
			res.Flagged++
		}
		switch action {
		case ReviewActionKeepGap:
			ev, err := q.GetEvent(ctx, eventID)
			if err != nil {
				return mapErr("get event", err)
			}
			if err := markEventExcluded(ctx, q, ev); err != nil {
				return err
			}
			return nil
		case ReviewActionUseEvent:
			if err := s.resolveNewInGap(ctx, q, sqlc.ReviewItem{
				PeriodID: periodID,
				Kind:     reviewNewInGap,
				EventID:  sql.NullInt64{Int64: eventID, Valid: true},
			}, action); err != nil {
				return err
			}
		}
	case inc.AllDay:
		_, created, err := s.enqueueIfUnresolved(ctx, q, periodID, reviewAllDay, eventID,
			reviewConflictKey(identity, inc.StartDate, inc.EndDate),
			map[string]any{"title": inc.Title})
		if err != nil {
			return err
		}
		if created {
			res.Flagged++
		}
	case inc.Status == "tentative" || inc.Status == "needsAction":
		action, created, err := s.enqueueIfUnresolved(ctx, q, periodID, reviewTentative, eventID,
			reviewConflictKey(identity, inc.Status),
			map[string]any{"title": inc.Title, "status": inc.Status})
		if err != nil {
			return err
		}
		if created {
			res.Flagged++
		}
		switch action {
		case ReviewActionExclude:
			ev, err := q.GetEvent(ctx, eventID)
			if err != nil {
				return mapErr("get event", err)
			}
			if err := markEventExcluded(ctx, q, ev); err != nil {
				return err
			}
			return nil
		case ReviewActionInclude:
		}
	}

	if err := s.applyMemory(ctx, q, periodID, inc); err != nil {
		return err
	}
	if err := s.applyCalendarDefault(ctx, q, periodID, inc); err != nil {
		return err
	}
	return s.applyAISuggestion(ctx, q, periodID, inc)
}

// handleChangedEvent flags conflicts arising from a material change. Time-only
// and other non-title changes apply silently (category describes what, not when).
func (s *Service) handleChangedEvent(ctx context.Context, q *sqlc.Queries, periodID, eventID int64, ex sqlc.Event, inc IncomingEvent, res *SyncResult) error {
	categorized, err := hasCategory(ctx, q, periodID, inc.Provider, inc.ExternalID, inc.InstanceID)
	if err != nil {
		return err
	}
	if !categorized {
		return nil // nothing the user decided yet → no conflict
	}

	if normalizeTitle(ex.Title) != normalizeTitle(inc.Title) {
		_, created, err := s.enqueueIfUnresolved(ctx, q, periodID, reviewTitleChanged, eventID,
			reviewConflictKey(eventIdentityFromIncoming(inc), normalizeTitle(ex.Title), normalizeTitle(inc.Title)),
			map[string]any{"from": ex.Title, "to": inc.Title})
		if err != nil {
			return err
		}
		if created {
			res.Flagged++
		}
	}
	if inc.Status == "declined" {
		action, created, err := s.enqueueIfUnresolved(ctx, q, periodID, reviewDeletedCategoriz, eventID,
			reviewConflictKey(eventIdentityFromIncoming(inc), "declined"),
			map[string]any{"reason": "declined", "title": inc.Title})
		if err != nil {
			return err
		}
		if created {
			res.Flagged++
		}
		if action == ReviewActionDropEntry {
			if err := s.resolveDeletedCategorized(ctx, q, sqlc.ReviewItem{
				PeriodID: periodID,
				Kind:     reviewDeletedCategoriz,
				EventID:  sql.NullInt64{Int64: eventID, Valid: true},
			}, action); err != nil {
				return err
			}
		}
	}
	return nil
}

// applyCalendarDefault auto-applies a calendar's default category when memory
// did not match. AI suggestion is a later layer.
func (s *Service) applyCalendarDefault(ctx context.Context, q *sqlc.Queries, periodID int64, inc IncomingEvent) error {
	has, err := hasCategory(ctx, q, periodID, inc.Provider, inc.ExternalID, inc.InstanceID)
	if err != nil || has {
		return err
	}

	cal, err := q.GetCalendar(ctx, inc.CalendarID)
	if errors.Is(err, sql.ErrNoRows) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("calendar lookup: %w", err)
	}
	if !cal.DefaultCategoryID.Valid {
		return nil
	}

	if _, err := q.UpsertOverlay(ctx, sqlc.UpsertOverlayParams{
		PeriodID:   periodID,
		Provider:   inc.Provider,
		ExternalID: inc.ExternalID,
		InstanceID: inc.InstanceID,
		CategoryID: cal.DefaultCategoryID,
		Kind:       overlayKindCategory,
	}); err != nil {
		return fmt.Errorf("apply calendar default overlay: %w", err)
	}
	return nil
}

// applyMemory auto-applies a remembered category to a (new) event when its match
// key is known. Creates a category overlay; AI suggestion is a later layer.
func (s *Service) applyMemory(ctx context.Context, q *sqlc.Queries, periodID int64, inc IncomingEvent) error {
	m, err := q.GetMemory(ctx, matchKey(inc))
	if errors.Is(err, sql.ErrNoRows) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("memory lookup: %w", err)
	}
	if _, err := q.UpsertOverlay(ctx, sqlc.UpsertOverlayParams{
		PeriodID:   periodID,
		Provider:   inc.Provider,
		ExternalID: inc.ExternalID,
		InstanceID: inc.InstanceID,
		CategoryID: sql.NullInt64{Int64: m.CategoryID, Valid: true},
		Kind:       overlayKindCategory,
	}); err != nil {
		return fmt.Errorf("apply memory overlay: %w", err)
	}
	return nil
}

// ── helpers ───────────────────────────────────────────────────────────

func eventKey(calendarID int64, externalID, instanceID string) string {
	return fmt.Sprintf("%d|%s|%s", calendarID, externalID, instanceID)
}

// hasCategory reports whether a category overlay (with a non-null category)
// exists for an event occurrence.
func hasCategory(ctx context.Context, q *sqlc.Queries, periodID int64, provider, externalID, instanceID string) (bool, error) {
	o, err := q.GetOverlay(ctx, sqlc.GetOverlayParams{
		PeriodID:   periodID,
		Provider:   provider,
		ExternalID: externalID,
		InstanceID: instanceID,
		Kind:       overlayKindCategory,
	})
	if errors.Is(err, sql.ErrNoRows) {
		return false, nil
	}
	if err != nil {
		return false, fmt.Errorf("overlay lookup: %w", err)
	}
	return o.CategoryID.Valid, nil
}

func (s *Service) enqueueIfUnresolved(ctx context.Context, q *sqlc.Queries, periodID int64, kind string, eventID int64, conflictKey string, payload map[string]any) (action string, created bool, err error) {
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

func reviewConflictKey(identity string, parts ...string) string {
	fields := append([]string{identity}, parts...)
	return strings.Join(fields, "|")
}

// overlapsAnyGap reports whether a timed event's interval intersects any gap
// fill. Half-open intervals: [s,e) overlaps [gs,ge) iff s < ge && gs < e.
func overlapsAnyGap(inc IncomingEvent, gaps []sqlc.GapFill) bool {
	if inc.Start == nil || inc.End == nil {
		return false
	}
	s, e := inc.Start.UTC(), inc.End.UTC()
	for _, g := range gaps {
		gs := parseTime(g.StartUtc)
		ge := parseTime(g.EndUtc)
		if gs.IsZero() || ge.IsZero() {
			continue
		}
		if s.Before(ge) && gs.Before(e) {
			return true
		}
	}
	return false
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

// matchKey is the categorization-memory key: the recurring series id when
// present, else the normalized title plus organizer.
func matchKey(inc IncomingEvent) string {
	if inc.RecurringEventID != "" {
		return "rid:" + inc.RecurringEventID
	}
	return "title:" + normalizeTitle(inc.Title) + "|" + strings.ToLower(strings.TrimSpace(inc.Organizer))
}

// normalizeTitle lowercases, trims, and collapses internal whitespace so
// cosmetic title differences don't read as material changes.
func normalizeTitle(s string) string {
	return strings.ToLower(strings.Join(strings.Fields(s), " "))
}

// upsertParams builds the sqlc upsert params for an incoming event and the
// source hash used for change detection.
func upsertParams(periodID int64, inc IncomingEvent) (sqlc.UpsertEventParams, string) {
	attJSON := marshalAttendees(inc.Attendees)
	p := sqlc.UpsertEventParams{
		PeriodID:         periodID,
		CalendarID:       inc.CalendarID,
		Provider:         inc.Provider,
		ExternalID:       inc.ExternalID,
		InstanceID:       inc.InstanceID,
		RecurringEventID: inc.RecurringEventID,
		IcalUid:          inc.ICalUID,
		Title:            inc.Title,
		Description:      inc.Description,
		Location:         inc.Location,
		Organizer:        inc.Organizer,
		Attendees:        attJSON,
		Status:           inc.Status,
		AllDay:           boolToInt(inc.AllDay),
		StartUtc:         nullTime(inc.Start),
		EndUtc:           nullTime(inc.End),
		StartDate:        nullStr(inc.StartDate),
		EndDate:          nullStr(inc.EndDate),
		OriginalTz:       inc.OriginalTz,
	}
	p.SourceHash = sourceHash(p)
	return p, p.SourceHash
}

// sourceHash hashes the synced fields so an unchanged re-pull is detected
// cheaply. Excludes period/calendar (identity) and the hash itself.
func sourceHash(p sqlc.UpsertEventParams) string {
	h := sha256.New()
	fmt.Fprintf(h, "%s\x00%s\x00%s\x00%s\x00%s\x00%s\x00%s\x00%s\x00%s\x00%s\x00%d\x00%s\x00%s\x00%s\x00%s\x00%s",
		p.Provider, p.ExternalID, p.InstanceID, p.RecurringEventID, p.IcalUid,
		p.Title, p.Description, p.Location, p.Organizer, p.Attendees,
		p.AllDay, p.StartUtc.String, p.EndUtc.String, p.StartDate.String, p.EndDate.String, p.OriginalTz)
	return hex.EncodeToString(h.Sum(nil))
}

func marshalAttendees(a []Attendee) string {
	if len(a) == 0 {
		return "[]"
	}
	b, err := json.Marshal(a)
	if err != nil {
		return "[]"
	}
	return string(b)
}

func boolToInt(b bool) int64 {
	if b {
		return 1
	}
	return 0
}

func nullStr(s string) sql.NullString {
	if s == "" {
		return sql.NullString{}
	}
	return sql.NullString{String: s, Valid: true}
}

func nullTime(t *time.Time) sql.NullString {
	if t == nil {
		return sql.NullString{}
	}
	return sql.NullString{String: t.UTC().Format(time.RFC3339), Valid: true}
}
