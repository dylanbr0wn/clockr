package service

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/dylanbr0wn/shiet/internal/db/sqlc"
)

// CalendarPuller syncs calendar metadata and fetches events for a pay period.
// Implemented by integration providers (for example google.Provider).
type CalendarPuller interface {
	SyncCalendars(ctx context.Context, accountID string) ([]sqlc.Calendar, error)
	FetchEvents(ctx context.Context, accountID, periodStart, periodEnd string, calendars []sqlc.Calendar) ([]IncomingEvent, error)
}

// IntegrationAccount is a connected provider account eligible for sync.
type IntegrationAccount struct {
	Provider  string `json:"provider"`
	AccountID string `json:"accountId"`
	Status    string `json:"status"`
}

// ConnectionLister returns connected integration accounts.
type ConnectionLister interface {
	ListByProvider(ctx context.Context, provider string) ([]IntegrationAccount, error)
}

// CalendarSyncConfig wires calendar providers into the service layer.
type CalendarSyncConfig struct {
	Puller      CalendarPuller
	Connections ConnectionLister
}

// ErrCalendarSyncNotConfigured is returned when SyncPeriod is called without
// integration dependencies wired in.
var ErrCalendarSyncNotConfigured = errors.New("calendar sync not configured")

// ErrNoConnectedAccounts is returned when sync runs but no usable accounts exist.
var ErrNoConnectedAccounts = errors.New("no connected calendar accounts")

// ErrNeedsReauth is returned when a connected account requires re-authentication.
var ErrNeedsReauth = errors.New("calendar account needs re-authentication")

// SetCalendarSync wires calendar pull + connection listing into the service.
func (s *Service) SetCalendarSync(cfg CalendarSyncConfig) {
	s.calendarSync = &cfg
}

// SyncPeriod refreshes calendar metadata, pulls events for selected calendars
// across the period range, merges them into stored facts, and stamps
// last_synced_at on success.
func (s *Service) SyncPeriod(ctx context.Context, periodID int64) (SyncResult, error) {
	var res SyncResult
	if s.calendarSync == nil || s.calendarSync.Puller == nil || s.calendarSync.Connections == nil {
		return res, ErrCalendarSyncNotConfigured
	}

	period, err := s.GetPeriod(ctx, periodID)
	if err != nil {
		return res, err
	}

	accounts, err := s.calendarSync.Connections.ListByProvider(ctx, ProviderGoogle)
	if err != nil {
		return res, fmt.Errorf("list google connections: %w", err)
	}

	usable := filterUsableAccounts(accounts)
	if len(usable) == 0 {
		if hasNeedsReauth(accounts) {
			return res, ErrNeedsReauth
		}
		return res, ErrNoConnectedAccounts
	}

	selected, err := s.q.ListSelectedCalendars(ctx)
	if err != nil {
		return res, mapErr("list selected calendars", err)
	}
	googleSelected := filterGoogleCalendars(selected)
	if len(googleSelected) == 0 {
		if err := s.touchPeriodSynced(ctx, periodID); err != nil {
			return res, err
		}
		return res, nil
	}

	var incoming []IncomingEvent
	for _, acct := range usable {
		if _, err := s.calendarSync.Puller.SyncCalendars(ctx, acct.AccountID); err != nil {
			return res, fmt.Errorf("sync calendars for %q: %w", acct.AccountID, err)
		}

		// Re-read selection after calendar list refresh so new calendars respect
		// existing selected flags from the registry pull.
		selected, err := s.q.ListSelectedCalendars(ctx)
		if err != nil {
			return res, mapErr("list selected calendars", err)
		}
		googleSelected = filterGoogleCalendars(selected)
		if len(googleSelected) == 0 {
			continue
		}

		events, err := s.calendarSync.Puller.FetchEvents(ctx, acct.AccountID, period.StartDate, period.EndDate, googleSelected)
		if err != nil {
			return res, fmt.Errorf("fetch events for %q: %w", acct.AccountID, err)
		}
		incoming = append(incoming, events...)
	}

	res, err = s.SyncEvents(ctx, periodID, incoming)
	if err != nil {
		return res, err
	}
	if err := s.touchPeriodSynced(ctx, periodID); err != nil {
		return res, err
	}
	return res, nil
}

// SetCalendarSelected toggles whether a calendar is included in schedule imports.
// Deselecting soft-hides existing events from that calendar; reselecting restores them.
func (s *Service) SetCalendarSelected(ctx context.Context, calendarID int64, selected bool) error {
	sel := int64(0)
	if selected {
		sel = 1
	}
	if err := s.q.SetCalendarSelected(ctx, sqlc.SetCalendarSelectedParams{
		Selected: sel,
		ID:       calendarID,
	}); err != nil {
		return mapErr("set calendar selected", err)
	}

	active := int64(0)
	if selected {
		active = 1
	}
	if err := s.q.SetEventActiveByCalendar(ctx, sqlc.SetEventActiveByCalendarParams{
		Active:     active,
		CalendarID: calendarID,
	}); err != nil {
		return mapErr("set event active by calendar", err)
	}
	return nil
}

// SetCalendarDefaultCategory assigns a default category to a calendar source.
func (s *Service) SetCalendarDefaultCategory(ctx context.Context, calendarID int64, categoryID *int64) error {
	var cat sql.NullInt64
	if categoryID != nil {
		cat = sql.NullInt64{Int64: *categoryID, Valid: true}
	}
	if err := s.q.SetCalendarDefaultCategory(ctx, sqlc.SetCalendarDefaultCategoryParams{
		DefaultCategoryID: cat,
		ID:                calendarID,
	}); err != nil {
		return mapErr("set calendar default category", err)
	}
	return nil
}

func (s *Service) touchPeriodSynced(ctx context.Context, periodID int64) error {
	now := time.Now().UTC().Format(time.RFC3339)
	if err := s.q.TouchPeriodSynced(ctx, sqlc.TouchPeriodSyncedParams{
		LastSyncedAt: sql.NullString{String: now, Valid: true},
		ID:           periodID,
	}); err != nil {
		return mapErr("touch period synced", err)
	}
	return nil
}

func filterGoogleCalendars(cals []sqlc.Calendar) []sqlc.Calendar {
	out := make([]sqlc.Calendar, 0, len(cals))
	for _, cal := range cals {
		if cal.Provider == ProviderGoogle {
			out = append(out, cal)
		}
	}
	return out
}

func filterUsableAccounts(accounts []IntegrationAccount) []IntegrationAccount {
	out := make([]IntegrationAccount, 0, len(accounts))
	for _, acct := range accounts {
		if acct.Status == "connected" {
			out = append(out, acct)
		}
	}
	return out
}

func hasNeedsReauth(accounts []IntegrationAccount) bool {
	for _, acct := range accounts {
		if acct.Status == "needs_reauth" {
			return true
		}
	}
	return false
}
