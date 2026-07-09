package secrets_test

import (
	"errors"
	"testing"
	"time"

	"github.com/dylanbr0wn/shiet/internal/integration/secrets"
)

func TestMemoryStoreRoundTrip(t *testing.T) {
	store := secrets.NewMemoryStore()
	token := secrets.Token{
		AccessToken:  "access",
		TokenType:    "Bearer",
		RefreshToken: "refresh",
		Expiry:       time.Now().UTC().Add(time.Hour),
	}

	if err := store.Set("google", "user@example.com", token); err != nil {
		t.Fatal(err)
	}

	got, err := store.Get("google", "user@example.com")
	if err != nil {
		t.Fatal(err)
	}
	if got.AccessToken != token.AccessToken || got.RefreshToken != token.RefreshToken {
		t.Fatalf("unexpected token: %+v", got)
	}

	if err := store.Delete("google", "user@example.com"); err != nil {
		t.Fatal(err)
	}
	_, err = store.Get("google", "user@example.com")
	if !errors.Is(err, secrets.ErrNotFound) {
		t.Fatalf("want ErrNotFound, got %v", err)
	}
}

func TestMemoryStoreRequiresIDs(t *testing.T) {
	store := secrets.NewMemoryStore()
	err := store.Set("", "acct", secrets.Token{AccessToken: "x"})
	if err == nil {
		t.Fatal("expected error for empty provider")
	}
}

func TestTokenOAuth2RoundTrip(t *testing.T) {
	expiry := time.Now().UTC().Add(2 * time.Hour).Truncate(time.Second)
	token := secrets.Token{
		AccessToken:  "a",
		TokenType:    "Bearer",
		RefreshToken: "r",
		Expiry:       expiry,
	}
	back := secrets.TokenFromOAuth2(token.ToOAuth2())
	if back.AccessToken != "a" || back.RefreshToken != "r" {
		t.Fatalf("unexpected round trip: %+v", back)
	}
}
