package store

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"
)

func TestOAuthStateOneTimeUseAndExpiry(t *testing.T) {
	ctx := context.Background()
	s := newTestStore(t)
	now := time.Date(2026, 7, 8, 12, 0, 0, 0, time.UTC)

	if err := s.SaveOAuthState(ctx, OAuthState{
		ID:               "state-1",
		DesktopSessionID: "desktop-1",
		PKCEVerifier:     "verifier",
		PKCEChallenge:    "challenge",
		HandoffChallenge: "handoff-challenge",
		Scopes:           []string{"scope"},
		ExpiresAt:        now.Add(time.Minute),
	}); err != nil {
		t.Fatal(err)
	}

	rec, err := s.ConsumeOAuthState(ctx, "state-1", now)
	if err != nil {
		t.Fatalf("consume state: %v", err)
	}
	if rec.UsedAt == nil {
		t.Fatal("expected consumed state to have used_at")
	}
	_, err = s.ConsumeOAuthState(ctx, "state-1", now)
	if !errors.Is(err, ErrAlreadyUsed) {
		t.Fatalf("second consume: got %v want %v", err, ErrAlreadyUsed)
	}

	if err := s.SaveOAuthState(ctx, OAuthState{
		ID:               "state-expired",
		DesktopSessionID: "desktop-1",
		PKCEVerifier:     "verifier",
		PKCEChallenge:    "challenge",
		HandoffChallenge: "handoff-challenge",
		Scopes:           []string{"scope"},
		ExpiresAt:        now.Add(-time.Second),
	}); err != nil {
		t.Fatal(err)
	}
	_, err = s.ConsumeOAuthState(ctx, "state-expired", now)
	if !errors.Is(err, ErrExpired) {
		t.Fatalf("expired consume: got %v want %v", err, ErrExpired)
	}
}

func TestHandoffOneTimeUseScrubsPayloadAndExpiry(t *testing.T) {
	ctx := context.Background()
	s := newTestStore(t)
	now := time.Date(2026, 7, 8, 12, 0, 0, 0, time.UTC)

	if err := s.SaveOAuthState(ctx, OAuthState{
		ID:               "state-1",
		DesktopSessionID: "desktop-1",
		PKCEVerifier:     "verifier",
		PKCEChallenge:    "challenge",
		HandoffChallenge: "handoff-challenge",
		Scopes:           []string{"scope"},
		ExpiresAt:        now.Add(time.Minute),
	}); err != nil {
		t.Fatal(err)
	}
	if err := s.SaveHandoff(ctx, HandoffRecord{
		CodeHash:              "hash-1",
		StateID:               "state-1",
		DesktopSessionID:      "desktop-1",
		HandoffChallenge:      "handoff-challenge",
		EncryptedTokenPayload: []byte("ciphertext"),
		AccountHint:           "user@example.com",
		Scopes:                []string{"scope"},
		ExpiresAt:             now.Add(time.Minute),
	}); err != nil {
		t.Fatal(err)
	}

	_, err := s.ConsumeHandoff(ctx, "hash-1", "wrong-desktop", "state-1", "handoff-challenge", now)
	if !errors.Is(err, ErrMismatch) {
		t.Fatalf("binding mismatch: got %v want %v", err, ErrMismatch)
	}

	rec, err := s.ConsumeHandoff(ctx, "hash-1", "desktop-1", "state-1", "handoff-challenge", now)
	if err != nil {
		t.Fatalf("consume handoff: %v", err)
	}
	if string(rec.EncryptedTokenPayload) != "ciphertext" {
		t.Fatalf("returned payload: got %q", rec.EncryptedTokenPayload)
	}
	_, err = s.ConsumeHandoff(ctx, "hash-1", "desktop-1", "state-1", "handoff-challenge", now)
	if !errors.Is(err, ErrAlreadyUsed) {
		t.Fatalf("second consume: got %v want %v", err, ErrAlreadyUsed)
	}

	var storedPayload []byte
	if err := s.db.QueryRowContext(ctx, `SELECT encrypted_token_payload FROM broker_handoffs WHERE code_hash = ?`, "hash-1").Scan(&storedPayload); err != nil {
		t.Fatal(err)
	}
	if storedPayload != nil {
		t.Fatalf("expected stored payload to be scrubbed, got %q", storedPayload)
	}
}

func TestSchemaDoesNotCreatePersistentGoogleTokenColumns(t *testing.T) {
	ctx := context.Background()
	s := newTestStore(t)

	rows, err := s.db.QueryContext(ctx, `
SELECT m.name, p.name
FROM sqlite_master AS m
JOIN pragma_table_info(m.name) AS p
WHERE m.type = 'table'
ORDER BY m.name, p.cid`)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()

	var columns []string
	for rows.Next() {
		var table, column string
		if err := rows.Scan(&table, &column); err != nil {
			t.Fatal(err)
		}
		columns = append(columns, table+"."+column)
	}
	if err := rows.Err(); err != nil {
		t.Fatal(err)
	}

	joined := strings.Join(columns, "\n")
	for _, forbidden := range []string{"access_token", "refresh_token", "google_access_token", "google_refresh_token"} {
		if strings.Contains(joined, forbidden) {
			t.Fatalf("schema contains forbidden token column %q:\n%s", forbidden, joined)
		}
	}
}

func newTestStore(t *testing.T) *SQLiteStore {
	t.Helper()
	s, err := Open(context.Background(), "file:"+t.Name()+"?mode=memory&cache=shared")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := s.Close(); err != nil {
			t.Fatal(err)
		}
	})
	return s
}
