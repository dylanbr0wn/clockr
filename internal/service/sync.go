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

// overlay.kind for a category / status decision.
const (
	overlayKindCategory = "category"
	overlayKindStatus   = "status"
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

	entries, err := q.ListTimeEntriesForPeriod(ctx, periodID)
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
			if err := s.handleNewEvent(ctx, q, periodID, inc, ev.ID, entries, &res); err != nil {
				return res, err
			}
			if err := s.materializeDraftForEvent(ctx, q, periodID, inc, ev, &entries, false); err != nil {
				return res, err
			}
			continue
		}

		if ex.SourceHash == hash {
			res.Unchanged++
			if err := s.materializeDraftForEvent(ctx, q, periodID, inc, ex, &entries, false); err != nil {
				return res, err
			}
			continue
		}

		// Changed: update the fact (overlay untouched), then classify.
		ev, err := q.UpsertEvent(ctx, params)
		if err != nil {
			return res, fmt.Errorf("update event %s: %w", inc.ExternalID, err)
		}
		res.Updated++
		if err := s.handleChangedEvent(ctx, q, periodID, ev.ID, ex, inc, entries, hash, &res); err != nil {
			return res, err
		}
		material := titleMaterialChange(ex, inc) || scheduleIncludeDropped(ex, inc)
		if err := s.materializeDraftForEvent(ctx, q, periodID, inc, ev, &entries, material); err != nil {
			return res, err
		}
	}

	// Events that vanished from the pull.
	policy := s.review()
	for _, e := range base {
		if _, ok := seen[eventKey(e.CalendarID, e.ExternalID, e.InstanceID)]; ok {
			continue
		}
		excluded, err := policy.HasStatusExcluded(ctx, q, e)
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
			flagged, removed, err := policy.OnVanishedCategorized(ctx, q, periodID, e)
			if err != nil {
				return res, err
			}
			if flagged {
				res.Flagged++
			}
			if removed {
				res.Removed++
			}
			continue
		}
		flagged, kept, err := policy.OnVanishedConfirmedSource(ctx, q, periodID, e, entries)
		if err != nil {
			return res, err
		}
		if kept {
			if flagged > 0 {
				res.Flagged += flagged
			}
			continue
		}
		if err := q.DeleteEvent(ctx, e.ID); err != nil {
			return res, fmt.Errorf("delete event %d: %w", e.ID, err)
		}
		res.Removed++
	}

	if err := tx.Commit(); err != nil {
		return res, fmt.Errorf("commit sync tx: %w", err)
	}
	return res, nil
}

// handleNewEvent flags a new event when it conflicts / needs resolution, then
// auto-categorizes from memory. Flags are mutually exclusive (gap takes
// precedence) to keep the review queue quiet.
func (s *Service) handleNewEvent(ctx context.Context, q *sqlc.Queries, periodID int64, inc IncomingEvent, eventID int64, gaps []sqlc.TimeEntry, res *SyncResult) error {
	flagged, skipAuto, err := s.review().OnNewEvent(ctx, q, periodID, inc, eventID, gaps)
	if err != nil {
		return err
	}
	res.Flagged += flagged
	if skipAuto {
		return nil
	}

	if err := s.applyMemory(ctx, q, periodID, inc); err != nil {
		return err
	}
	if err := s.applyCalendarDefault(ctx, q, periodID, inc); err != nil {
		return err
	}
	return s.applyAISuggestion(ctx, q, periodID, inc)
}

// handleChangedEvent flags conflicts arising from a material change, and opens
// source_drift when a confirmed calendar-sourced TimeEntry's revision lags.
func (s *Service) handleChangedEvent(
	ctx context.Context,
	q *sqlc.Queries,
	periodID, eventID int64,
	ex sqlc.Event,
	inc IncomingEvent,
	entries []sqlc.TimeEntry,
	newHash string,
	res *SyncResult,
) error {
	flagged, err := s.review().OnConfirmedSourceDrift(ctx, q, periodID, eventID, inc, entries, newHash)
	if err != nil {
		return err
	}
	res.Flagged += flagged

	categorized, err := hasCategory(ctx, q, periodID, inc.Provider, inc.ExternalID, inc.InstanceID)
	if err != nil {
		return err
	}
	if !categorized {
		return nil // nothing the user decided yet → no overlay conflict
	}

	// Draft-only proposals absorb title refresh; skip duplicate title_changed review.
	identity := eventIdentityFromIncoming(inc)
	linked := linkedCalendarEntries(entries, identity)
	liveDrafts, hasConfirmed, _ := partitionLinked(linked)
	if len(liveDrafts) > 0 && !hasConfirmed && titleMaterialChange(ex, inc) {
		// Still flag declined and other overlay conflicts; only suppress title_changed.
		if inc.Status == "declined" {
			overlayFlagged, err := s.review().OnChangedCategorized(ctx, q, periodID, eventID, ex, inc)
			if err != nil {
				return err
			}
			res.Flagged += overlayFlagged
		}
		return nil
	}

	overlayFlagged, err := s.review().OnChangedCategorized(ctx, q, periodID, eventID, ex, inc)
	if err != nil {
		return err
	}
	res.Flagged += overlayFlagged
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
