package oauth

import "time"

// Shared broker↔desktop JSON contract types. Keeping these in one package
// prevents request/response drift without coupling provider internals.

// BrokerStartRequest is POST /v1/{provider}/oauth/start.
type BrokerStartRequest struct {
	DesktopSessionID       string `json:"desktop_session_id"`
	HandoffChallenge       string `json:"handoff_challenge"`
	AppVersion             string `json:"app_version"`
	Platform               string `json:"platform"`
	DesktopHandoffRedirect string `json:"desktop_handoff_redirect,omitempty"`
}

// BrokerStartResponse is returned by a successful start.
type BrokerStartResponse struct {
	AuthURL     string    `json:"auth_url"`
	BrokerState string    `json:"broker_state"`
	ExpiresAt   time.Time `json:"expires_at"`
}

// BrokerHandoffRequest is POST /v1/{provider}/oauth/handoff.
type BrokerHandoffRequest struct {
	DesktopSessionID string `json:"desktop_session_id"`
	BrokerState      string `json:"broker_state"`
	HandoffCode      string `json:"handoff_code"`
	HandoffVerifier  string `json:"handoff_verifier"`
}

// BrokerHandoffResponse returns one-time token material to the desktop.
type BrokerHandoffResponse struct {
	Provider    string   `json:"provider"`
	AccountHint string   `json:"account_hint"`
	Scope       []string `json:"scope"`
	Token       struct {
		AccessToken  string    `json:"access_token"`
		RefreshToken string    `json:"refresh_token,omitempty"`
		TokenType    string    `json:"token_type"`
		Expiry       time.Time `json:"expiry"`
	} `json:"token"`
}

// BrokerErrorResponse is the common JSON error envelope.
type BrokerErrorResponse struct {
	Error string `json:"error"`
}

// BrokerRevokeRequest is POST /v1/{provider}/oauth/revoke.
// Google uses refresh_token; GitHub uses access_token.
type BrokerRevokeRequest struct {
	RefreshToken string `json:"refresh_token,omitempty"`
	AccessToken  string `json:"access_token,omitempty"`
	Reason       string `json:"reason,omitempty"`
}

// BrokerRevokeResponse confirms provider revocation.
type BrokerRevokeResponse struct {
	Revoked bool `json:"revoked"`
}

// BrokerRefreshRequest is POST /v1/google/oauth/refresh (Google-only today).
type BrokerRefreshRequest struct {
	RefreshToken string   `json:"refresh_token"`
	Scope        []string `json:"scope,omitempty"`
	AppVersion   string   `json:"app_version,omitempty"`
	Platform     string   `json:"platform,omitempty"`
}

// BrokerRefreshResponse returns refreshed Google token material.
type BrokerRefreshResponse struct {
	AccessToken  string    `json:"access_token"`
	RefreshToken string    `json:"refresh_token,omitempty"`
	TokenType    string    `json:"token_type"`
	Expiry       time.Time `json:"expiry"`
}
