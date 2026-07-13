package main

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"path/filepath"

	"github.com/dylanbr0wn/shiet/internal/ai"
	"github.com/dylanbr0wn/shiet/internal/config"
	"github.com/dylanbr0wn/shiet/internal/integration/connection"
	"github.com/dylanbr0wn/shiet/internal/integration/bitbucket"
	"github.com/dylanbr0wn/shiet/internal/integration/github"
	"github.com/dylanbr0wn/shiet/internal/integration/google"
	"github.com/dylanbr0wn/shiet/internal/integration/slack"
	"github.com/dylanbr0wn/shiet/internal/service"
	"github.com/rs/zerolog"
	"github.com/wailsapp/wails/v2/pkg/runtime"
)

// App struct
type App struct {
	ctx      context.Context
	conn     *sql.DB
	log      zerolog.Logger
	logPath  string
	Svc      *service.Service
	google   *google.Provider
	github   *github.Provider
	slack    *slack.Provider
	bitbucket *bitbucket.Provider
	registry *connection.Registry
}

// GoogleAuthStatus is the read-only Google OAuth mode shown in Settings.
// It never includes client secrets or token material.
type GoogleAuthStatus struct {
	Mode          string `json:"mode"`          // "broker" | "local"
	BrokerBaseURL string `json:"brokerBaseUrl"` // set in broker mode
}

// GetGoogleAuthStatus returns the active Google Calendar auth mode for Settings.
func (a *App) GetGoogleAuthStatus() GoogleAuthStatus {
	status := (*google.Provider)(nil).Status()
	if a != nil && a.google != nil {
		status = a.google.Status()
	}
	return GoogleAuthStatus{
		Mode:          status.Mode,
		BrokerBaseURL: status.BrokerBaseURL,
	}
}

// NewApp creates a new App over an already-open database connection. The
// connection is opened, migrated, and seeded in main before binding, so Svc is
// live at bind time (Wails reflects bound instances up front).
func NewApp(conn *sql.DB, cfg config.Config, logger zerolog.Logger) *App {
	svc := service.New(conn)
	googleProvider, githubProvider, slackProvider, bitbucketProvider, registry := wireIntegrations(conn, svc, cfg)
	return &App{
		conn:     conn,
		log:      logger,
		logPath:  cfg.Log.Path,
		Svc:      svc,
		google:   googleProvider,
		github:   githubProvider,
		slack:    slackProvider,
		bitbucket: bitbucketProvider,
		registry: registry,
	}
}

// LogPath returns the configured log file path (log.path / default).
func (a *App) LogPath() string {
	if a == nil {
		return ""
	}
	return a.logPath
}

// RevealLogFolder opens the directory containing the log file in the OS file
// manager. Creates the directory if needed so reveal works before the first write.
func (a *App) RevealLogFolder() error {
	if a == nil {
		return fmt.Errorf("log path is not configured")
	}
	if a.logPath == "" {
		return a.logErr("log.reveal_folder", fmt.Errorf("log path is not configured"))
	}
	dir := filepath.Dir(a.logPath)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return a.logErr("log.reveal_folder", fmt.Errorf("create log directory: %w", err))
	}
	if err := openFolderFn(dir); err != nil {
		return a.logErr("log.reveal_folder", err)
	}
	a.log.Info().Str("op", "log.reveal_folder").Str("dir", dir).Msg("opened log folder")
	return nil
}

// startup is called when the app starts; saves the context for runtime calls.
func (a *App) startup(ctx context.Context) {
	a.ctx = ctx
	a.log.Info().Str("op", "app.startup").Msg("startup success")
}

// shutdown is called on app exit; close the database cleanly.
func (a *App) shutdown(ctx context.Context) {
	a.log.Info().Str("op", "app.shutdown").Msg("shutdown")
	if a.conn != nil {
		_ = a.conn.Close()
	}
}

// ConnectGoogle runs desktop OAuth for a Google Calendar account.
func (a *App) ConnectGoogle(accountID, accountLabel string) (connection.Connection, error) {
	conn, err := a.google.Connect(a.callContext(), accountID, accountLabel)
	if err != nil {
		return conn, a.logErr("google.connect", err)
	}
	a.log.Info().Str("op", "google.connect").Str("account_id", accountID).Msg("connected")
	return conn, nil
}

// DisconnectGoogle removes a Google Calendar connection and its tokens.
func (a *App) DisconnectGoogle(accountID string) error {
	return a.logErr("google.disconnect", a.google.Disconnect(a.callContext(), accountID))
}

// ConnectGitHub connects a GitHub account using a personal access token.
func (a *App) ConnectGitHub(pat string) (connection.Connection, error) {
	conn, err := a.github.Connect(a.callContext(), pat)
	if err != nil {
		return conn, a.logErr("github.connect", err)
	}
	a.log.Info().Str("op", "github.connect").Str("account_id", conn.AccountID).Msg("connected")
	return conn, nil
}

// GitHubAuthMode returns the configured connect mode so the UI can present
// broker OAuth as the public default while retaining the local PAT escape hatch.
func (a *App) GitHubAuthMode() string {
	return a.github.AuthMode
}

// GitHubOAuthAvailable reports whether GitHub browser OAuth can be started.
// PAT connect is exposed separately and remains available in either mode.
func (a *App) GitHubOAuthAvailable() bool {
	return a.github.OAuthAvailable()
}

// DisconnectGitHub removes a GitHub connection, tokens, and synced repos.
func (a *App) DisconnectGitHub(accountID string) error {
	return a.logErr("github.disconnect", a.github.Disconnect(a.callContext(), accountID))
}

// ConnectSlack runs desktop OAuth for a Slack workspace.
func (a *App) ConnectSlack() (connection.Connection, error) {
	conn, err := a.slack.Connect(a.callContext())
	if err != nil {
		return conn, a.logErr("slack.connect", err)
	}
	a.log.Info().Str("op", "slack.connect").Str("account_id", conn.AccountID).Msg("connected")
	return conn, nil
}

// SlackAuthMode returns the configured connect mode for Slack OAuth.
func (a *App) SlackAuthMode() string {
	return a.slack.AuthMode
}

// SlackOAuthAvailable reports whether Slack browser OAuth can be started.
func (a *App) SlackOAuthAvailable() bool {
	return a.slack.OAuthAvailable()
}

// DisconnectSlack removes a Slack connection, tokens, and synced channels.
func (a *App) DisconnectSlack(accountID string) error {
	return a.logErr("slack.disconnect", a.slack.Disconnect(a.callContext(), accountID))
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
	models, err := a.Svc.ListAIModels(a.callContext(), baseURL, apiKey)
	return models, a.logErr("ai.list_models", err)
}

// ValidateAIConfig checks endpoint connectivity and returns the privacy verdict.
func (a *App) ValidateAIConfig(baseURL string, apiKey string, model string) (ai.ValidationResult, error) {
	result, err := a.Svc.ValidateAIConfig(a.callContext(), baseURL, apiKey, model)
	return result, a.logErr("ai.validate_config", err)
}

// SaveAIEndpoint persists the selected OpenAI-compatible base URL.
func (a *App) SaveAIEndpoint(baseURL string) error {
	return a.logErr("ai.save_endpoint", a.Svc.SaveAIEndpoint(a.callContext(), baseURL))
}

// SaveAIModel persists the selected model name.
func (a *App) SaveAIModel(model string) error {
	return a.logErr("ai.save_model", a.Svc.SaveAIModel(a.callContext(), model))
}

// SaveAIConfig persists the selected endpoint and model.
func (a *App) SaveAIConfig(baseURL string, model string) error {
	return a.logErr("ai.save_config", a.Svc.SaveAIConfig(a.callContext(), baseURL, model))
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
		return "", a.logErr("export.save_file", err)
	}
	if path == "" {
		return "", nil
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		return "", a.logErr("export.save_file", fmt.Errorf("write export file: %w", err))
	}
	return path, nil
}

// logErr logs a non-nil error at the Go→frontend boundary, then returns it unchanged.
func (a *App) logErr(op string, err error) error {
	if err != nil {
		a.log.Error().Err(err).Str("op", op).Msg("operation failed")
	}
	return err
}

// wrapSyncPeriod logs calendar sync start/fail around the service SyncPeriod call.
func wrapSyncPeriod(logger zerolog.Logger, sync func(context.Context, int64) (service.SyncResult, error)) func(context.Context, int64) (service.SyncResult, error) {
	return func(ctx context.Context, periodID int64) (service.SyncResult, error) {
		logger.Info().Str("op", "calendar.sync_period").Int64("period_id", periodID).Msg("sync started")
		result, err := sync(ctx, periodID)
		if err != nil {
			logger.Error().Err(err).Str("op", "calendar.sync_period").Int64("period_id", periodID).Msg("operation failed")
			return result, err
		}
		return result, nil
	}
}

func (a *App) callContext() context.Context {
	if a.ctx != nil {
		return a.ctx
	}
	return context.Background()
}
