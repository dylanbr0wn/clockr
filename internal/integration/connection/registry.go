package connection

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/dylanbr0wn/shiet/internal/db/sqlc"
)

// Status values persisted for integration connections.
const (
	StatusConnected   = "connected"
	StatusNeedsReauth = "needs_reauth"
	StatusDisconnected = "disconnected"
)

// Connection is non-secret metadata for a connected integration account.
type Connection struct {
	ID           int64    `json:"id"`
	Provider     string   `json:"provider"`
	AccountLabel string   `json:"accountLabel"`
	AccountID    string   `json:"accountId"`
	Scopes       []string `json:"scopes"`
	Status       string   `json:"status"`
	ConnectedAt  string   `json:"connectedAt"`
	UpdatedAt    string   `json:"updatedAt"`
}

// UpsertInput describes a connection to create or update.
type UpsertInput struct {
	Provider     string
	AccountLabel string
	AccountID    string
	Scopes       []string
	Status       string
}

// Registry persists connected integration accounts in SQLite.
type Registry struct {
	q *sqlc.Queries
}

// NewRegistry builds a connection registry over sqlc queries.
func NewRegistry(db *sql.DB) *Registry {
	return &Registry{q: sqlc.New(db)}
}

// List returns all connections ordered by provider and label.
func (r *Registry) List(ctx context.Context) ([]Connection, error) {
	rows, err := r.q.ListIntegrationConnections(ctx)
	if err != nil {
		return nil, fmt.Errorf("list connections: %w", err)
	}
	out := make([]Connection, len(rows))
	for i, row := range rows {
		conn, err := toConnection(row)
		if err != nil {
			return nil, err
		}
		out[i] = conn
	}
	return out, nil
}

// ListByProvider returns connections for a single provider.
func (r *Registry) ListByProvider(ctx context.Context, provider string) ([]Connection, error) {
	rows, err := r.q.ListIntegrationConnectionsByProvider(ctx, provider)
	if err != nil {
		return nil, fmt.Errorf("list connections: %w", err)
	}
	out := make([]Connection, len(rows))
	for i, row := range rows {
		conn, err := toConnection(row)
		if err != nil {
			return nil, err
		}
		out[i] = conn
	}
	return out, nil
}

// Get returns a single connection by provider and account id.
func (r *Registry) Get(ctx context.Context, provider, accountID string) (Connection, error) {
	row, err := r.q.GetIntegrationConnection(ctx, sqlc.GetIntegrationConnectionParams{
		Provider:  provider,
		AccountID: accountID,
	})
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return Connection{}, ErrNotFound
		}
		return Connection{}, fmt.Errorf("get connection: %w", err)
	}
	return toConnection(row)
}

// Upsert creates or updates a connection row.
func (r *Registry) Upsert(ctx context.Context, input UpsertInput) (Connection, error) {
	status := input.Status
	if status == "" {
		status = StatusConnected
	}
	scopesJSON, err := encodeScopes(input.Scopes)
	if err != nil {
		return Connection{}, err
	}

	row, err := r.q.UpsertIntegrationConnection(ctx, sqlc.UpsertIntegrationConnectionParams{
		Provider:     strings.TrimSpace(input.Provider),
		AccountLabel: strings.TrimSpace(input.AccountLabel),
		AccountID:    strings.TrimSpace(input.AccountID),
		Scopes:       scopesJSON,
		Status:       status,
	})
	if err != nil {
		return Connection{}, fmt.Errorf("upsert connection: %w", err)
	}
	return toConnection(row)
}

// SetStatus updates the connection status (for example after token refresh failure).
func (r *Registry) SetStatus(ctx context.Context, provider, accountID, status string) error {
	if err := r.q.UpdateIntegrationConnectionStatus(ctx, sqlc.UpdateIntegrationConnectionStatusParams{
		Status:    status,
		Provider:  provider,
		AccountID: accountID,
	}); err != nil {
		return fmt.Errorf("update connection status: %w", err)
	}
	return nil
}

// Disconnect marks a connection disconnected and removes its row.
func (r *Registry) Disconnect(ctx context.Context, provider, accountID string) error {
	if err := r.q.DeleteIntegrationConnection(ctx, sqlc.DeleteIntegrationConnectionParams{
		Provider:  provider,
		AccountID: accountID,
	}); err != nil {
		return fmt.Errorf("disconnect: %w", err)
	}
	return nil
}

// ErrNotFound is returned when a connection row does not exist.
var ErrNotFound = errors.New("connection not found")

func encodeScopes(scopes []string) (string, error) {
	if scopes == nil {
		scopes = []string{}
	}
	b, err := json.Marshal(scopes)
	if err != nil {
		return "", fmt.Errorf("encode scopes: %w", err)
	}
	return string(b), nil
}

func decodeScopes(raw string) ([]string, error) {
	var scopes []string
	if raw == "" {
		return []string{}, nil
	}
	if err := json.Unmarshal([]byte(raw), &scopes); err != nil {
		return nil, fmt.Errorf("decode scopes: %w", err)
	}
	return scopes, nil
}

func toConnection(row sqlc.IntegrationConnection) (Connection, error) {
	scopes, err := decodeScopes(row.Scopes)
	if err != nil {
		return Connection{}, err
	}
	return Connection{
		ID:           row.ID,
		Provider:     row.Provider,
		AccountLabel: row.AccountLabel,
		AccountID:    row.AccountID,
		Scopes:       scopes,
		Status:       row.Status,
		ConnectedAt:  row.ConnectedAt,
		UpdatedAt:    row.UpdatedAt,
	}, nil
}
