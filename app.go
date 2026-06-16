package main

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/dylanbr0wn/clockr/internal/service"
)

// App struct
type App struct {
	ctx  context.Context
	conn *sql.DB
	Svc  *service.Service
}

type ManualEventResult struct {
	PeriodID int64 `json:"periodId"`
	ID       int64 `json:"id"`
}

// NewApp creates a new App over an already-open database connection. The
// connection is opened, migrated, and seeded in main before binding, so Svc is
// live at bind time (Wails reflects bound instances up front).
func NewApp(conn *sql.DB) *App {
	return &App{
		conn: conn,
		Svc:  service.New(conn),
	}
}

// startup is called when the app starts; saves the context for runtime calls.
func (a *App) startup(ctx context.Context) {
	a.ctx = ctx
}

// shutdown is called on app exit; close the database cleanly.
func (a *App) shutdown(ctx context.Context) {
	if a.conn != nil {
		_ = a.conn.Close()
	}
}

// Greet returns a greeting for the given name
func (a *App) Greet(name string) string {
	return fmt.Sprintf("Hello %s, It's show time!", name)
}

// ListPeriods returns available editable periods.
func (a *App) ListPeriods() ([]service.Period, error) {
	return a.Svc.ListPeriods(a.callContext())
}

// EnsureCurrentPeriod returns the period containing today, creating it when
// needed. This frontend-facing wrapper keeps Wails' context parameter out of
// the generated JavaScript call shape.
func (a *App) EnsureCurrentPeriod(today string, ianaTz string) (service.Period, error) {
	return a.Svc.EnsureCurrentPeriod(a.callContext(), today, ianaTz)
}

// ListCategories returns all user categories.
func (a *App) ListCategories() ([]service.Category, error) {
	return a.Svc.ListCategories(a.callContext())
}

// ListCalendars returns all known calendars.
func (a *App) ListCalendars() ([]service.Calendar, error) {
	return a.Svc.ListCalendars(a.callContext())
}

// ListSelectedCalendars returns calendars included in schedule imports.
func (a *App) ListSelectedCalendars() ([]service.Calendar, error) {
	return a.Svc.ListSelectedCalendars(a.callContext())
}

// ListEvents returns active events for a period.
func (a *App) ListEvents(periodID int64) ([]service.Event, error) {
	return a.Svc.ListEvents(a.callContext(), periodID)
}

// ListGapFills returns manual/gap-fill entries for a period.
func (a *App) ListGapFills(periodID int64) ([]service.GapFill, error) {
	return a.Svc.ListGapFills(a.callContext(), periodID)
}

// ListOpenReviewItems returns unresolved review items for a period.
func (a *App) ListOpenReviewItems(periodID int64) ([]service.ReviewItem, error) {
	return a.Svc.ListOpenReviewItems(a.callContext(), periodID)
}

// ListTzSegments returns timezone segments for a period.
func (a *App) ListTzSegments(periodID int64) ([]service.TzSegment, error) {
	return a.Svc.ListTzSegments(a.callContext(), periodID)
}

// ComputeGaps returns the period's daily gap timeline.
func (a *App) ComputeGaps(periodID int64) ([]service.DayTimeline, error) {
	return a.Svc.ComputeGaps(a.callContext(), periodID)
}

// GetSetting returns a raw JSON setting value.
func (a *App) GetSetting(key string) (string, error) {
	return a.Svc.GetSetting(a.callContext(), key)
}

// CreateManualEvent persists a scheduler-created manual block.
func (a *App) CreateManualEvent(input service.ManualEventInput) (ManualEventResult, error) {
	fill, err := a.Svc.CreateManualEvent(a.callContext(), input)
	if err != nil {
		return ManualEventResult{}, err
	}
	return ManualEventResult{PeriodID: fill.PeriodID, ID: fill.ID}, nil
}

// UpdateManualEvent persists a scheduler edit to an existing manual block.
func (a *App) UpdateManualEvent(input service.ManualEventUpdateInput) (ManualEventResult, error) {
	fill, err := a.Svc.UpdateManualEvent(a.callContext(), input)
	if err != nil {
		return ManualEventResult{}, err
	}
	return ManualEventResult{PeriodID: fill.PeriodID, ID: fill.ID}, nil
}

// DeleteManualEvent removes a scheduler-created manual block.
func (a *App) DeleteManualEvent(input service.ManualEventDeleteInput) (ManualEventResult, error) {
	if err := a.Svc.DeleteManualEvent(a.callContext(), input); err != nil {
		return ManualEventResult{}, err
	}
	return ManualEventResult{PeriodID: input.PeriodID, ID: input.ID}, nil
}

func (a *App) callContext() context.Context {
	if a.ctx != nil {
		return a.ctx
	}
	return context.Background()
}
