// Package service is Clockr's application layer over the SQLite store. It wraps
// the generated sqlc queries, converting raw rows into clean domain types and
// adding the cross-table logic (re-sync merge, gap computation) the UI needs.
//
// This file covers construction and simple read queries; mutating flows
// (sync 3-way merge, gap computation) live in their own files.
package service

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/dylanbr0wn/clockr/internal/db/sqlc"
)

// ErrNotFound is returned when a requested row does not exist. Callers can test
// with errors.Is so they need not know about sql.ErrNoRows.
var ErrNotFound = errors.New("not found")

// Service is the application layer. Bind it (or App methods that delegate to
// it) into Wails. Methods take a context.Context first arg, which Wails injects
// for bound calls.
type Service struct {
	db           *sql.DB
	q            *sqlc.Queries
	calendarSync *CalendarSyncConfig
	evidence     *EvidenceConfig
}

// New builds a Service over an open database connection.
func New(db *sql.DB) *Service {
	return &Service{db: db, q: sqlc.New(db)}
}

// mapErr normalizes a no-rows error to ErrNotFound, otherwise wraps with ctx.
func mapErr(what string, err error) error {
	if errors.Is(err, sql.ErrNoRows) {
		return fmt.Errorf("%s: %w", what, ErrNotFound)
	}
	return fmt.Errorf("%s: %w", what, err)
}

// ── categories ────────────────────────────────────────────────────────

func (s *Service) ListCategories(ctx context.Context) ([]Category, error) {
	rows, err := s.q.ListCategories(ctx)
	if err != nil {
		return nil, mapErr("list categories", err)
	}
	out := make([]Category, len(rows))
	for i, r := range rows {
		out[i] = toCategory(r)
	}
	return out, nil
}

func (s *Service) GetCategory(ctx context.Context, id int64) (Category, error) {
	r, err := s.q.GetCategory(ctx, id)
	if err != nil {
		return Category{}, mapErr("get category", err)
	}
	return toCategory(r), nil
}

// ListEventCategoryOverlays returns category decisions for imported events in a period.
func (s *Service) ListEventCategoryOverlays(ctx context.Context, periodID int64) ([]EventCategoryOverlay, error) {
	rows, err := s.q.ListOverlaysForPeriod(ctx, periodID)
	if err != nil {
		return nil, mapErr("list event category overlays", err)
	}
	out := make([]EventCategoryOverlay, 0, len(rows))
	for _, row := range rows {
		if row.Kind != overlayKindCategory || !row.CategoryID.Valid {
			continue
		}
		out = append(out, EventCategoryOverlay{
			Provider:   row.Provider,
			ExternalID: row.ExternalID,
			InstanceID: row.InstanceID,
			CategoryID: row.CategoryID.Int64,
		})
	}
	return out, nil
}

// ── periods ───────────────────────────────────────────────────────────

func (s *Service) ListPeriods(ctx context.Context) ([]Period, error) {
	rows, err := s.q.ListPeriods(ctx)
	if err != nil {
		return nil, mapErr("list periods", err)
	}
	out := make([]Period, len(rows))
	for i, r := range rows {
		out[i] = toPeriod(r)
	}
	return out, nil
}

func (s *Service) GetPeriod(ctx context.Context, id int64) (Period, error) {
	r, err := s.q.GetPeriod(ctx, id)
	if err != nil {
		return Period{}, mapErr("get period", err)
	}
	return toPeriod(r), nil
}

// GetPeriodByRange looks up a period by its (start, end) date range.
func (s *Service) GetPeriodByRange(ctx context.Context, start, end string) (Period, error) {
	r, err := s.q.GetPeriodByRange(ctx, sqlc.GetPeriodByRangeParams{StartDate: start, EndDate: end})
	if err != nil {
		return Period{}, mapErr("get period by range", err)
	}
	return toPeriod(r), nil
}

// ListTzSegments returns a period's timezone segments, ordered by date.
func (s *Service) ListTzSegments(ctx context.Context, periodID int64) ([]TzSegment, error) {
	rows, err := s.q.ListTzSegments(ctx, periodID)
	if err != nil {
		return nil, mapErr("list tz segments", err)
	}
	out := make([]TzSegment, len(rows))
	for i, r := range rows {
		out[i] = toTzSegment(r)
	}
	return out, nil
}

// ── calendars ─────────────────────────────────────────────────────────

func (s *Service) ListCalendars(ctx context.Context) ([]Calendar, error) {
	rows, err := s.q.ListCalendars(ctx)
	if err != nil {
		return nil, mapErr("list calendars", err)
	}
	out := make([]Calendar, len(rows))
	for i, r := range rows {
		out[i] = toCalendar(r)
	}
	return out, nil
}

func (s *Service) ListSelectedCalendars(ctx context.Context) ([]Calendar, error) {
	rows, err := s.q.ListSelectedCalendars(ctx)
	if err != nil {
		return nil, mapErr("list selected calendars", err)
	}
	out := make([]Calendar, len(rows))
	for i, r := range rows {
		out[i] = toCalendar(r)
	}
	return out, nil
}

// ── events ────────────────────────────────────────────────────────────

// ListEvents returns the active (non-soft-hidden) events for a period.
func (s *Service) ListEvents(ctx context.Context, periodID int64) ([]Event, error) {
	rows, err := s.q.ListEventsForPeriod(ctx, periodID)
	if err != nil {
		return nil, mapErr("list events", err)
	}
	out := make([]Event, len(rows))
	for i, r := range rows {
		out[i] = toEvent(r)
	}
	return out, nil
}

// GetEvent returns a single event by id.
func (s *Service) GetEvent(ctx context.Context, id int64) (Event, error) {
	r, err := s.q.GetEvent(ctx, id)
	if err != nil {
		return Event{}, mapErr("get event", err)
	}
	return toEvent(r), nil
}

// ── gap fills ─────────────────────────────────────────────────────────

func (s *Service) ListGapFills(ctx context.Context, periodID int64) ([]GapFill, error) {
	rows, err := s.q.ListGapFillsForPeriod(ctx, periodID)
	if err != nil {
		return nil, mapErr("list gap fills", err)
	}
	out := make([]GapFill, len(rows))
	for i, r := range rows {
		out[i] = toGapFill(r)
	}
	return out, nil
}

// ── review queue ──────────────────────────────────────────────────────

// ListOpenReviewItems returns the unresolved review-queue items for a period.
func (s *Service) ListOpenReviewItems(ctx context.Context, periodID int64) ([]ReviewItem, error) {
	rows, err := s.q.ListOpenReviewItems(ctx, periodID)
	if err != nil {
		return nil, mapErr("list review items", err)
	}
	out := make([]ReviewItem, len(rows))
	for i, r := range rows {
		out[i] = toReviewItem(r)
	}
	return out, nil
}

// ── settings ──────────────────────────────────────────────────────────

// GetSetting returns the raw JSON-encoded value for a setting key.
func (s *Service) GetSetting(ctx context.Context, key string) (string, error) {
	v, err := s.q.GetSetting(ctx, key)
	if err != nil {
		return "", mapErr("get setting", err)
	}
	return v, nil
}

// SetSetting stores a raw JSON-encoded setting value.
func (s *Service) SetSetting(ctx context.Context, key, value string) error {
	if err := s.q.SetSetting(ctx, sqlc.SetSettingParams{Key: key, Value: value}); err != nil {
		return mapErr("set setting", err)
	}
	return nil
}
