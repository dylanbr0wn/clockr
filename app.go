package main

import (
	"context"
	"database/sql"
	"fmt"
	"os"

	"github.com/dylanbr0wn/shiet/internal/ai"
	"github.com/dylanbr0wn/shiet/internal/config"
	"github.com/dylanbr0wn/shiet/internal/integration/connection"
	"github.com/dylanbr0wn/shiet/internal/integration/github"
	"github.com/dylanbr0wn/shiet/internal/integration/google"
	"github.com/dylanbr0wn/shiet/internal/integration/slack"
	"github.com/dylanbr0wn/shiet/internal/service"
	"github.com/wailsapp/wails/v2/pkg/runtime"
)

// App struct
type App struct {
	ctx      context.Context
	conn     *sql.DB
	Svc      *service.Service
	google   *google.Provider
	github   *github.Provider
	slack    *slack.Provider
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
func NewApp(conn *sql.DB, cfg config.Config) *App {
	svc := service.New(conn)
	googleProvider, githubProvider, slackProvider, registry := wireIntegrations(conn, svc, cfg)
	return &App{
		conn:     conn,
		Svc:      svc,
		google:   googleProvider,
		github:   githubProvider,
		slack:    slackProvider,
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

// ConnectGoogle runs desktop OAuth for a Google Calendar account.
func (a *App) ConnectGoogle(accountID, accountLabel string) (connection.Connection, error) {
	return a.google.Connect(a.callContext(), accountID, accountLabel)
}

// DisconnectGoogle removes a Google Calendar connection and its tokens.
func (a *App) DisconnectGoogle(accountID string) error {
	return a.google.Disconnect(a.callContext(), accountID)
}

// ConnectGitHub connects a GitHub account using a personal access token.
func (a *App) ConnectGitHub(pat string) (connection.Connection, error) {
	return a.github.Connect(a.callContext(), pat)
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
	return a.github.Disconnect(a.callContext(), accountID)
}

// ConnectSlack runs desktop OAuth for a Slack workspace.
func (a *App) ConnectSlack() (connection.Connection, error) {
	return a.slack.Connect(a.callContext())
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
	return a.slack.Disconnect(a.callContext(), accountID)
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
