package secrets

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"golang.org/x/oauth2"
)

const keyringService = "shiet"

// ErrNotFound is returned when no token exists for a provider/account pair.
var ErrNotFound = errors.New("token not found")

// CredentialSource records how a token was issued so provider cleanup follows
// the credential's origin even if runtime auth mode changes later.
type CredentialSource string

const (
	CredentialSourceBroker     CredentialSource = "broker"
	CredentialSourceLocalOAuth CredentialSource = "local_oauth"
	CredentialSourcePAT        CredentialSource = "pat"
)

// TokenStore persists OAuth tokens outside SQLite.
type TokenStore interface {
	Get(provider, accountID string) (Token, error)
	Set(provider, accountID string, token Token) error
	Delete(provider, accountID string) error
}

// Token is the persisted OAuth credential bundle.
type Token struct {
	AccessToken      string           `json:"access_token"`
	TokenType        string           `json:"token_type,omitempty"`
	RefreshToken     string           `json:"refresh_token,omitempty"`
	Expiry           time.Time        `json:"expiry,omitempty"`
	CredentialSource CredentialSource `json:"credential_source,omitempty"`
}

// ToOAuth2 converts the stored token to an oauth2.Token.
func (t Token) ToOAuth2() *oauth2.Token {
	return &oauth2.Token{
		AccessToken:  t.AccessToken,
		TokenType:    t.TokenType,
		RefreshToken: t.RefreshToken,
		Expiry:       t.Expiry,
	}
}

// TokenFromOAuth2 builds a Token from an oauth2.Token.
func TokenFromOAuth2(tok *oauth2.Token) Token {
	if tok == nil {
		return Token{}
	}
	return Token{
		AccessToken:  tok.AccessToken,
		TokenType:    tok.TokenType,
		RefreshToken: tok.RefreshToken,
		Expiry:       tok.Expiry,
	}
}

func storageKey(provider, accountID string) (string, error) {
	provider = strings.TrimSpace(provider)
	accountID = strings.TrimSpace(accountID)
	if provider == "" || accountID == "" {
		return "", fmt.Errorf("provider and account_id are required")
	}
	return provider + ":" + accountID, nil
}

func encodeToken(token Token) (string, error) {
	b, err := json.Marshal(token)
	if err != nil {
		return "", fmt.Errorf("encode token: %w", err)
	}
	return string(b), nil
}

func decodeToken(raw string) (Token, error) {
	var token Token
	if err := json.Unmarshal([]byte(raw), &token); err != nil {
		return Token{}, fmt.Errorf("decode token: %w", err)
	}
	return token, nil
}
