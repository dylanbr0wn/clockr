package main

import (
	"context"
	"database/sql"
	"fmt"
	"os"

	"github.com/dylanbr0wn/shiet/internal/ai"
	"github.com/dylanbr0wn/shiet/internal/config"
	"github.com/dylanbr0wn/shiet/internal/integration/connection"
	"github.com/dylanbr0wn/shiet/internal/integration/google"
	"github.com/dylanbr0wn/shiet/internal/service"
	"github.com/wailsapp/wails/v2/pkg/runtime"
)

// App struct
type App struct {
	ctx      context.Context
	conn     *sql.DB
	Svc      *service.Service
	google   *google.Provider
	registry *connection.Registry
}

type ManualEventResult struct {
	PeriodID int64 `json:"periodId"`
	ID       int64 `json:"id"`
}

// NewApp creates a new App over an already-open database connection. The
// connection is opened, migrated, and seeded in main before binding, so Svc is
// live at bind time (Wails reflects bound instances up front).
func NewApp(conn *sql.DB, cfg config.Config) *App {
	svc := service.New(conn)
	googleProvider, registry := wireIntegrations(conn, svc, cfg)
	return &App{
		conn:     conn,
		Svc:      svc,
		google:   googleProvider,
		registry: registry,
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

// CreateCategory adds a user-defined category.
func (a *App) CreateCategory(input service.CreateCategoryInput) (service.Category, error) {
	return a.Svc.CreateCategory(a.callContext(), input)
}

// UpdateCategory updates a user-defined category.
func (a *App) UpdateCategory(input service.UpdateCategoryInput) (service.Category, error) {
	return a.Svc.UpdateCategory(a.callContext(), input)
}

// DeleteCategory removes a category when it is not referenced.
func (a *App) DeleteCategory(id int64) error {
	return a.Svc.DeleteCategory(a.callContext(), id)
}

// ListEventCategoryOverlays returns category decisions for imported events.
func (a *App) ListEventCategoryOverlays(periodID int64) ([]service.EventCategoryOverlay, error) {
	return a.Svc.ListEventCategoryOverlays(a.callContext(), periodID)
}

// ListCalendars returns all known calendars.
func (a *App) ListCalendars() ([]service.Calendar, error) {
	return a.Svc.ListCalendars(a.callContext())
}

// ListSelectedCalendars returns calendars included in schedule imports.
func (a *App) ListSelectedCalendars() ([]service.Calendar, error) {
	return a.Svc.ListSelectedCalendars(a.callContext())
}

// SyncPeriod pulls calendar events for a pay period and merges them locally.
func (a *App) SyncPeriod(periodID int64) (service.SyncResult, error) {
	return a.Svc.SyncPeriod(a.callContext(), periodID)
}

// SetCalendarSelected toggles whether a calendar is included in imports.
func (a *App) SetCalendarSelected(calendarID int64, selected bool) error {
	return a.Svc.SetCalendarSelected(a.callContext(), calendarID, selected)
}

// SetCalendarDefaultCategory assigns a default category to a calendar source.
func (a *App) SetCalendarDefaultCategory(calendarID int64, categoryID *int64) error {
	return a.Svc.SetCalendarDefaultCategory(a.callContext(), calendarID, categoryID)
}

// ListIntegrationConnections returns all connected integration accounts.
func (a *App) ListIntegrationConnections() ([]connection.Connection, error) {
	return a.registry.List(a.callContext())
}

// ConnectGoogle runs desktop OAuth for a Google Calendar account.
func (a *App) ConnectGoogle(accountID, accountLabel string) (connection.Connection, error) {
	return a.google.Connect(a.callContext(), accountID, accountLabel)
}

// DisconnectGoogle removes a Google Calendar connection and its tokens.
func (a *App) DisconnectGoogle(accountID string) error {
	return a.google.Disconnect(a.callContext(), accountID)
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

// ResolveReviewItem applies a user decision to a review-queue item.
func (a *App) ResolveReviewItem(input service.ResolveReviewItemInput) (service.ResolveReviewItemResult, error) {
	return a.Svc.ResolveReviewItem(a.callContext(), input)
}

// ExcludeEvent hides a synced calendar event from the schedule for a period.
func (a *App) ExcludeEvent(input service.ExcludeEventInput) (service.ExcludeEventResult, error) {
	return a.Svc.ExcludeEvent(a.callContext(), input)
}

// ListTzSegments returns timezone segments for a period.
func (a *App) ListTzSegments(periodID int64) ([]service.TzSegment, error) {
	return a.Svc.ListTzSegments(a.callContext(), periodID)
}

// ComputeGaps returns the period's daily gap timeline.
func (a *App) ComputeGaps(periodID int64) ([]service.DayTimeline, error) {
	return a.Svc.ComputeGaps(a.callContext(), periodID)
}

// SuggestGapFill proposes a category and description for an uncovered interval
// using aggregated activity evidence and the configured AI model.
func (a *App) SuggestGapFill(window service.TimeWindow) (service.GapSuggestion, error) {
	return a.Svc.SuggestGapFill(a.callContext(), window)
}

// GetSetting returns a raw JSON setting value.
func (a *App) GetSetting(key string) (string, error) {
	return a.Svc.GetSetting(a.callContext(), key)
}

// SetSetting persists a raw JSON setting value.
func (a *App) SetSetting(key string, value string) error {
	return a.Svc.SetSetting(a.callContext(), key, value)
}

// AIClassification is the privacy verdict for an endpoint URL.
type AIClassification struct {
	Local   bool   `json:"local"`
	Verdict string `json:"verdict"`
}

// DiscoverLocalAIEndpoints probes known local OpenAI-compatible runtimes.
func (a *App) DiscoverLocalAIEndpoints() ([]ai.Endpoint, error) {
	return a.Svc.DiscoverLocalAIEndpoints(a.callContext()), nil
}

// ClassifyAIEndpoint reports whether a base URL is local and the privacy verdict.
func (a *App) ClassifyAIEndpoint(baseURL string) AIClassification {
	local, verdict := a.Svc.ClassifyAIEndpoint(baseURL)
	return AIClassification{Local: local, Verdict: verdict}
}

// ListAIModels returns model ids from an OpenAI-compatible endpoint.
func (a *App) ListAIModels(baseURL string, apiKey string) ([]string, error) {
	return a.Svc.ListAIModels(a.callContext(), baseURL, apiKey)
}

// ValidateAIConfig checks endpoint connectivity and returns the privacy verdict.
func (a *App) ValidateAIConfig(baseURL string, apiKey string, model string) (ai.ValidationResult, error) {
	return a.Svc.ValidateAIConfig(a.callContext(), baseURL, apiKey, model)
}

// SaveAIEndpoint persists the selected OpenAI-compatible base URL.
func (a *App) SaveAIEndpoint(baseURL string) error {
	return a.Svc.SaveAIEndpoint(a.callContext(), baseURL)
}

// SaveAIModel persists the selected model name.
func (a *App) SaveAIModel(model string) error {
	return a.Svc.SaveAIModel(a.callContext(), model)
}

// SaveAIConfig persists the selected endpoint and model.
func (a *App) SaveAIConfig(baseURL string, model string) error {
	return a.Svc.SaveAIConfig(a.callContext(), baseURL, model)
}

// CreateManualEvent persists a scheduler-created manual block.
func (a *App) CreateManualEvent(input service.ManualEventInput) (ManualEventResult, error) {
	fill, err := a.Svc.CreateManualEvent(a.callContext(), input)
	if err != nil {
		return ManualEventResult{}, err
	}
	return ManualEventResult{PeriodID: fill.PeriodID, ID: fill.ID}, nil
}

// CreateGapFill persists a user-confirmed gap assignment (e.g. from AI suggest).
func (a *App) CreateGapFill(input service.ManualEventInput) (ManualEventResult, error) {
	fill, err := a.Svc.CreateGapFill(a.callContext(), input)
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

// SaveExportFile writes content to a user-selected path via the native save dialog.
// Returns the saved path, or an empty string when the dialog is cancelled.
func (a *App) SaveExportFile(defaultFilename, content string) (string, error) {
	path, err := runtime.SaveFileDialog(a.ctx, runtime.SaveDialogOptions{
		DefaultFilename: defaultFilename,
		Title:           "Save export",
		Filters: []runtime.FileFilter{
			{DisplayName: "CSV Files (*.csv)", Pattern: "*.csv"},
			{DisplayName: "All Files (*.*)", Pattern: "*.*"},
		},
	})
	if err != nil {
		return "", err
	}
	if path == "" {
		return "", nil
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		return "", fmt.Errorf("write export file: %w", err)
	}
	return path, nil
}

func (a *App) callContext() context.Context {
	if a.ctx != nil {
		return a.ctx
	}
	return context.Background()
}
