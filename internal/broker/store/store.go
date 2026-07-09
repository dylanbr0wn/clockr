// Package store persists the broker's short-lived coordination records.
package store

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	_ "modernc.org/sqlite"
)

var (
	ErrNotFound    = errors.New("record not found")
	ErrExpired     = errors.New("record expired")
	ErrAlreadyUsed = errors.New("record already used")
	ErrMismatch    = errors.New("record binding mismatch")
)

// SQLiteStore stores broker state in a minimal SQLite datastore. It intentionally
// has no durable Google access-token or refresh-token tables.
type SQLiteStore struct {
	db *sql.DB
}

type OAuthState struct {
	ID                     string
	DesktopSessionID       string
	PKCEVerifier           string
	PKCEChallenge          string
	HandoffChallenge       string
	DesktopHandoffRedirect string
	Scopes                 []string
	AppVersion             string
	Platform               string
	SourceIPBucket         string
	ExpiresAt              time.Time
	UsedAt                 *time.Time
}

type HandoffRecord struct {
	CodeHash              string
	StateID               string
	DesktopSessionID      string
	HandoffChallenge      string
	EncryptedTokenPayload []byte
	AccountHint           string
	Scopes                []string
	ExpiresAt             time.Time
	UsedAt                *time.Time
}

func Open(ctx context.Context, dsn string) (*SQLiteStore, error) {
	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, fmt.Errorf("open broker datastore: %w", err)
	}
	store := &SQLiteStore{db: db}
	if err := store.Migrate(ctx); err != nil {
		_ = db.Close()
		return nil, err
	}
	return store, nil
}

func (s *SQLiteStore) Close() error {
	return s.db.Close()
}

func (s *SQLiteStore) Ping(ctx context.Context) error {
	return s.db.PingContext(ctx)
}

func (s *SQLiteStore) Migrate(ctx context.Context) error {
	if _, err := s.db.ExecContext(ctx, schema); err != nil {
		return fmt.Errorf("migrate broker datastore: %w", err)
	}
	// Existing DYL-81 databases may lack the desktop handoff redirect column.
	if _, err := s.db.ExecContext(ctx, `
ALTER TABLE broker_oauth_states ADD COLUMN desktop_handoff_redirect TEXT NOT NULL DEFAULT ''`); err != nil {
		if !strings.Contains(strings.ToLower(err.Error()), "duplicate column") {
			return fmt.Errorf("migrate broker oauth state redirect column: %w", err)
		}
	}
	return nil
}

func (s *SQLiteStore) SaveOAuthState(ctx context.Context, rec OAuthState) error {
	scopes, err := json.Marshal(rec.Scopes)
	if err != nil {
		return fmt.Errorf("marshal state scopes: %w", err)
	}
	_, err = s.db.ExecContext(ctx, `
INSERT INTO broker_oauth_states (
	id, desktop_session_id, pkce_verifier, pkce_challenge, handoff_challenge,
	desktop_handoff_redirect, scopes_json, app_version, platform, source_ip_bucket, expires_at
) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		rec.ID, rec.DesktopSessionID, rec.PKCEVerifier, rec.PKCEChallenge,
		rec.HandoffChallenge, rec.DesktopHandoffRedirect, string(scopes), rec.AppVersion, rec.Platform,
		rec.SourceIPBucket, formatTime(rec.ExpiresAt),
	)
	if err != nil {
		return fmt.Errorf("save oauth state: %w", err)
	}
	return nil
}

func (s *SQLiteStore) ConsumeOAuthState(ctx context.Context, id string, now time.Time) (OAuthState, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return OAuthState{}, err
	}
	defer tx.Rollback()

	rec, err := selectOAuthState(ctx, tx, id)
	if err != nil {
		return OAuthState{}, err
	}
	if rec.UsedAt != nil {
		return OAuthState{}, ErrAlreadyUsed
	}
	if !now.Before(rec.ExpiresAt) {
		return OAuthState{}, ErrExpired
	}
	usedAt := formatTime(now)
	result, err := tx.ExecContext(ctx, `
UPDATE broker_oauth_states SET used_at = ? WHERE id = ? AND used_at IS NULL`,
		usedAt, id,
	)
	if err != nil {
		return OAuthState{}, fmt.Errorf("consume oauth state: %w", err)
	}
	if rows, err := result.RowsAffected(); err != nil {
		return OAuthState{}, err
	} else if rows != 1 {
		return OAuthState{}, ErrAlreadyUsed
	}
	if err := tx.Commit(); err != nil {
		return OAuthState{}, err
	}
	rec.UsedAt = &now
	return rec, nil
}

func (s *SQLiteStore) SaveHandoff(ctx context.Context, rec HandoffRecord) error {
	scopes, err := json.Marshal(rec.Scopes)
	if err != nil {
		return fmt.Errorf("marshal handoff scopes: %w", err)
	}
	_, err = s.db.ExecContext(ctx, `
INSERT INTO broker_handoffs (
	code_hash, state_id, desktop_session_id, handoff_challenge,
	encrypted_token_payload, account_hint, scopes_json, expires_at
) VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
		rec.CodeHash, rec.StateID, rec.DesktopSessionID, rec.HandoffChallenge,
		rec.EncryptedTokenPayload, rec.AccountHint, string(scopes), formatTime(rec.ExpiresAt),
	)
	if err != nil {
		return fmt.Errorf("save handoff: %w", err)
	}
	return nil
}

// ConsumeHandoff verifies desktop session, broker state, and handoff challenge
// bindings before marking the record used and scrubbing the sealed payload.
// Binding mismatches leave the handoff reusable so a wrong guess cannot burn it.
func (s *SQLiteStore) ConsumeHandoff(
	ctx context.Context,
	codeHash, desktopSessionID, stateID, handoffChallenge string,
	now time.Time,
) (HandoffRecord, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return HandoffRecord{}, err
	}
	defer tx.Rollback()

	rec, err := selectHandoff(ctx, tx, codeHash)
	if err != nil {
		return HandoffRecord{}, err
	}
	if rec.UsedAt != nil {
		return HandoffRecord{}, ErrAlreadyUsed
	}
	if !now.Before(rec.ExpiresAt) {
		return HandoffRecord{}, ErrExpired
	}
	if rec.DesktopSessionID != desktopSessionID || rec.StateID != stateID || rec.HandoffChallenge != handoffChallenge {
		return HandoffRecord{}, ErrMismatch
	}
	usedAt := formatTime(now)
	result, err := tx.ExecContext(ctx, `
UPDATE broker_handoffs
SET used_at = ?, encrypted_token_payload = NULL
WHERE code_hash = ? AND used_at IS NULL`,
		usedAt, codeHash,
	)
	if err != nil {
		return HandoffRecord{}, fmt.Errorf("consume handoff: %w", err)
	}
	if rows, err := result.RowsAffected(); err != nil {
		return HandoffRecord{}, err
	} else if rows != 1 {
		return HandoffRecord{}, ErrAlreadyUsed
	}
	if err := tx.Commit(); err != nil {
		return HandoffRecord{}, err
	}
	rec.UsedAt = &now
	return rec, nil
}

func (s *SQLiteStore) PurgeExpired(ctx context.Context, now time.Time) error {
	cutoff := formatTime(now)
	if _, err := s.db.ExecContext(ctx, `DELETE FROM broker_oauth_states WHERE expires_at <= ?`, cutoff); err != nil {
		return fmt.Errorf("purge expired oauth states: %w", err)
	}
	if _, err := s.db.ExecContext(ctx, `DELETE FROM broker_handoffs WHERE expires_at <= ?`, cutoff); err != nil {
		return fmt.Errorf("purge expired handoffs: %w", err)
	}
	return nil
}

func selectOAuthState(ctx context.Context, q queryer, id string) (OAuthState, error) {
	var rec OAuthState
	var scopesJSON, expiresAt, usedAt sql.NullString
	err := q.QueryRowContext(ctx, `
SELECT id, desktop_session_id, pkce_verifier, pkce_challenge, handoff_challenge,
	desktop_handoff_redirect, scopes_json, app_version, platform, source_ip_bucket, expires_at, used_at
FROM broker_oauth_states WHERE id = ?`, id).
		Scan(&rec.ID, &rec.DesktopSessionID, &rec.PKCEVerifier, &rec.PKCEChallenge,
			&rec.HandoffChallenge, &rec.DesktopHandoffRedirect, &scopesJSON, &rec.AppVersion, &rec.Platform,
			&rec.SourceIPBucket, &expiresAt, &usedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return OAuthState{}, ErrNotFound
	}
	if err != nil {
		return OAuthState{}, err
	}
	if scopesJSON.Valid {
		if err := json.Unmarshal([]byte(scopesJSON.String), &rec.Scopes); err != nil {
			return OAuthState{}, err
		}
	}
	rec.ExpiresAt = parseTime(expiresAt.String)
	if usedAt.Valid {
		t := parseTime(usedAt.String)
		rec.UsedAt = &t
	}
	return rec, nil
}

func selectHandoff(ctx context.Context, q queryer, codeHash string) (HandoffRecord, error) {
	var rec HandoffRecord
	var scopesJSON, expiresAt, usedAt sql.NullString
	err := q.QueryRowContext(ctx, `
SELECT code_hash, state_id, desktop_session_id, handoff_challenge,
	encrypted_token_payload, account_hint, scopes_json, expires_at, used_at
FROM broker_handoffs WHERE code_hash = ?`, codeHash).
		Scan(&rec.CodeHash, &rec.StateID, &rec.DesktopSessionID, &rec.HandoffChallenge,
			&rec.EncryptedTokenPayload, &rec.AccountHint, &scopesJSON, &expiresAt, &usedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return HandoffRecord{}, ErrNotFound
	}
	if err != nil {
		return HandoffRecord{}, err
	}
	if scopesJSON.Valid {
		if err := json.Unmarshal([]byte(scopesJSON.String), &rec.Scopes); err != nil {
			return HandoffRecord{}, err
		}
	}
	rec.ExpiresAt = parseTime(expiresAt.String)
	if usedAt.Valid {
		t := parseTime(usedAt.String)
		rec.UsedAt = &t
	}
	return rec, nil
}

type queryer interface {
	QueryRowContext(context.Context, string, ...any) *sql.Row
}

func formatTime(t time.Time) string {
	return t.UTC().Format(time.RFC3339Nano)
}

func parseTime(raw string) time.Time {
	t, _ := time.Parse(time.RFC3339Nano, raw)
	return t
}

const schema = `
CREATE TABLE IF NOT EXISTS broker_oauth_states (
	id TEXT PRIMARY KEY,
	desktop_session_id TEXT NOT NULL,
	pkce_verifier TEXT NOT NULL,
	pkce_challenge TEXT NOT NULL,
	handoff_challenge TEXT NOT NULL,
	desktop_handoff_redirect TEXT NOT NULL DEFAULT '',
	scopes_json TEXT NOT NULL,
	app_version TEXT NOT NULL DEFAULT '',
	platform TEXT NOT NULL DEFAULT '',
	source_ip_bucket TEXT NOT NULL DEFAULT '',
	expires_at TEXT NOT NULL,
	used_at TEXT
);

CREATE INDEX IF NOT EXISTS broker_oauth_states_expires_idx
	ON broker_oauth_states (expires_at);

CREATE TABLE IF NOT EXISTS broker_handoffs (
	code_hash TEXT PRIMARY KEY,
	state_id TEXT NOT NULL,
	desktop_session_id TEXT NOT NULL,
	handoff_challenge TEXT NOT NULL,
	encrypted_token_payload BLOB,
	account_hint TEXT NOT NULL DEFAULT '',
	scopes_json TEXT NOT NULL,
	expires_at TEXT NOT NULL,
	used_at TEXT,
	FOREIGN KEY (state_id) REFERENCES broker_oauth_states(id)
);

CREATE INDEX IF NOT EXISTS broker_handoffs_expires_idx
	ON broker_handoffs (expires_at);
`
