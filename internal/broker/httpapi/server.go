// Package httpapi exposes the OAuth broker's HTTP service surface.
package httpapi

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"html"
	"io"
	"log/slog"
	"net"
	"net/http"
	"net/url"
	"strings"
	"time"

	brokerconfig "github.com/dylanbr0wn/shiet/internal/broker/config"
	"github.com/dylanbr0wn/shiet/internal/broker/observe"
	"github.com/dylanbr0wn/shiet/internal/broker/ratelimit"
	"github.com/dylanbr0wn/shiet/internal/broker/store"
)

const (
	googleAuthURL       = "https://accounts.google.com/o/oauth2/v2/auth"
	defaultGoogleToken  = "https://oauth2.googleapis.com/token"
	defaultGoogleRevoke = "https://oauth2.googleapis.com/revoke"

	limitStart          = 10
	limitCallback       = 30
	limitHandoff        = 20
	limitHandoffFailure = 5
	limitRefresh        = 60
	limitRefreshFailure = 10
	limitRevoke         = 20
)

type Store interface {
	Ping(context.Context) error
	SaveOAuthState(context.Context, store.OAuthState) error
	ConsumeOAuthState(context.Context, string, time.Time) (store.OAuthState, error)
	SaveHandoff(context.Context, store.HandoffRecord) error
	ConsumeHandoff(context.Context, string, string, string, string, time.Time) (store.HandoffRecord, error)
}

// Limiter is the rate-limit seam used by the HTTP handlers.
type Limiter interface {
	Allow(key string, limit int) bool
}

type Server struct {
	Config          brokerconfig.Config
	Store           Store
	Clock           func() time.Time
	HTTPClient      *http.Client
	GoogleTokenURL  string // override for tests
	GoogleRevokeURL string // override for tests
	Limiter         Limiter
	Metrics         *observe.Metrics
	Logger          *slog.Logger
}

type startRequest struct {
	DesktopSessionID       string `json:"desktop_session_id"`
	HandoffChallenge       string `json:"handoff_challenge"`
	AppVersion             string `json:"app_version"`
	Platform               string `json:"platform"`
	DesktopHandoffRedirect string `json:"desktop_handoff_redirect,omitempty"`
}

type startResponse struct {
	AuthURL     string    `json:"auth_url"`
	BrokerState string    `json:"broker_state"`
	ExpiresAt   time.Time `json:"expires_at"`
}

type handoffRequest struct {
	DesktopSessionID string `json:"desktop_session_id"`
	BrokerState      string `json:"broker_state"`
	HandoffCode      string `json:"handoff_code"`
	HandoffVerifier  string `json:"handoff_verifier"`
}

type handoffResponse struct {
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

type refreshRequest struct {
	RefreshToken string   `json:"refresh_token"`
	Scope        []string `json:"scope,omitempty"`
	AppVersion   string   `json:"app_version,omitempty"`
	Platform     string   `json:"platform,omitempty"`
}

type refreshResponse struct {
	AccessToken  string    `json:"access_token"`
	RefreshToken string    `json:"refresh_token,omitempty"`
	TokenType    string    `json:"token_type"`
	Expiry       time.Time `json:"expiry"`
}

type statusResponse struct {
	Status string `json:"status"`
}

type revokeRequest struct {
	RefreshToken string `json:"refresh_token"`
	Reason       string `json:"reason,omitempty"`
}

type revokeResponse struct {
	Revoked bool `json:"revoked"`
}

type errorResponse struct {
	Error string `json:"error"`
}

type googleTokenResponse struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
	TokenType    string `json:"token_type"`
	ExpiresIn    int64  `json:"expires_in"`
	Scope        string `json:"scope"`
	Error        string `json:"error"`
	ErrorDesc    string `json:"error_description"`
}

func (s Server) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", s.health)
	mux.HandleFunc("GET /readyz", s.ready)
	mux.HandleFunc("GET /metrics", s.metrics)
	mux.HandleFunc("POST /v1/google/oauth/start", s.startGoogleOAuth)
	mux.HandleFunc("GET /v1/google/oauth/callback", s.googleCallback)
	mux.HandleFunc("POST /v1/google/oauth/handoff", s.exchangeHandoff)
	mux.HandleFunc("POST /v1/google/oauth/refresh", s.refreshGoogleOAuth)
	mux.HandleFunc("POST /v1/google/oauth/revoke", s.revokeGoogleOAuth)
	return mux
}

func (s Server) metrics(w http.ResponseWriter, r *http.Request) {
	if s.Metrics == nil {
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		w.WriteHeader(http.StatusOK)
		return
	}
	s.Metrics.Handler().ServeHTTP(w, r)
}

func (s Server) health(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, statusResponse{Status: "ok"})
}

func (s Server) ready(w http.ResponseWriter, r *http.Request) {
	if err := s.Config.Validate(); err != nil {
		writeJSON(w, http.StatusServiceUnavailable, errorResponse{Error: "invalid_config"})
		return
	}
	if s.Store == nil {
		writeJSON(w, http.StatusServiceUnavailable, errorResponse{Error: "datastore_unavailable"})
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), time.Second)
	defer cancel()
	if err := s.Store.Ping(ctx); err != nil {
		writeJSON(w, http.StatusServiceUnavailable, errorResponse{Error: "datastore_unavailable"})
		return
	}
	writeJSON(w, http.StatusOK, statusResponse{Status: "ready"})
}

func (s Server) startGoogleOAuth(w http.ResponseWriter, r *http.Request) {
	if s.Store == nil {
		writeJSON(w, http.StatusServiceUnavailable, errorResponse{Error: "datastore_unavailable"})
		return
	}
	if s.rejectAuthDisabled(w, "start") {
		return
	}
	ipBucket := sourceIPBucket(r.RemoteAddr)
	if s.rejectRateLimited(w, "start", ratelimit.Key("start", ipBucket), limitStart) {
		return
	}

	var req startRequest
	if err := decodeJSON(r.Body, &req); err != nil {
		s.Metrics.IncAuthStartFail()
		writeJSON(w, http.StatusBadRequest, errorResponse{Error: "invalid_json"})
		return
	}
	req.DesktopSessionID = strings.TrimSpace(req.DesktopSessionID)
	req.HandoffChallenge = strings.TrimSpace(req.HandoffChallenge)
	req.DesktopHandoffRedirect = strings.TrimSpace(req.DesktopHandoffRedirect)
	req.AppVersion = strings.TrimSpace(req.AppVersion)
	if req.DesktopSessionID == "" || req.HandoffChallenge == "" {
		s.Metrics.IncAuthStartFail()
		writeJSON(w, http.StatusBadRequest, errorResponse{Error: "desktop_session_id_and_handoff_challenge_required"})
		return
	}
	if s.rejectAppVersionDisabled(w, "start", req.AppVersion) {
		return
	}
	if req.DesktopHandoffRedirect != "" {
		if err := validateDesktopHandoffRedirect(req.DesktopHandoffRedirect); err != nil {
			s.Metrics.IncAuthStartFail()
			writeJSON(w, http.StatusBadRequest, errorResponse{Error: "invalid_desktop_handoff_redirect"})
			return
		}
	}

	state, err := randomString(32)
	if err != nil {
		s.Metrics.IncAuthStartFail()
		writeJSON(w, http.StatusInternalServerError, errorResponse{Error: "random_state_failed"})
		return
	}
	verifier, err := randomString(64)
	if err != nil {
		s.Metrics.IncAuthStartFail()
		writeJSON(w, http.StatusInternalServerError, errorResponse{Error: "random_verifier_failed"})
		return
	}
	challenge := pkceS256(verifier)
	now := s.now()
	expiresAt := now.Add(s.Config.StateTTL)

	rec := store.OAuthState{
		ID:                     state,
		DesktopSessionID:       req.DesktopSessionID,
		PKCEVerifier:           verifier,
		PKCEChallenge:          challenge,
		HandoffChallenge:       req.HandoffChallenge,
		DesktopHandoffRedirect: req.DesktopHandoffRedirect,
		Scopes:                 append([]string(nil), s.Config.GoogleScopes...),
		AppVersion:             req.AppVersion,
		Platform:               strings.TrimSpace(req.Platform),
		SourceIPBucket:         ipBucket,
		ExpiresAt:              expiresAt,
	}
	if err := s.Store.SaveOAuthState(r.Context(), rec); err != nil {
		s.Metrics.IncAuthStartFail()
		writeJSON(w, http.StatusInternalServerError, errorResponse{Error: "state_persist_failed"})
		return
	}

	authURL, err := s.authURL(state, challenge)
	if err != nil {
		s.Metrics.IncAuthStartFail()
		writeJSON(w, http.StatusInternalServerError, errorResponse{Error: "auth_url_failed"})
		return
	}
	s.Metrics.IncAuthStart()
	s.logInfo("auth_start", "outcome", "ok", "app_version", req.AppVersion, "platform", strings.TrimSpace(req.Platform), "ip_bucket", ipBucket)
	writeJSON(w, http.StatusCreated, startResponse{
		AuthURL:     authURL,
		BrokerState: state,
		ExpiresAt:   expiresAt,
	})
}

func (s Server) googleCallback(w http.ResponseWriter, r *http.Request) {
	if s.Store == nil {
		writeHTMLError(w, http.StatusServiceUnavailable, "Broker datastore unavailable. Return to shiet and retry.")
		return
	}
	if s.Config.AuthDisabled {
		s.Metrics.IncKillSwitch("callback")
		s.logInfo("kill_switch", "surface", "callback", "reason", "auth_disabled")
		writeHTMLError(w, http.StatusForbidden, "Google connect is temporarily disabled. Return to shiet and try again later.")
		return
	}
	ipBucket := sourceIPBucket(r.RemoteAddr)
	if !s.allow(ratelimit.Key("callback", ipBucket), limitCallback) {
		s.Metrics.IncRateLimited("callback")
		s.logInfo("rate_limited", "surface", "callback", "ip_bucket", ipBucket)
		writeHTMLError(w, http.StatusTooManyRequests, "Too many authorization attempts. Return to shiet and try again later.")
		return
	}

	q := r.URL.Query()
	if errMsg := strings.TrimSpace(q.Get("error")); errMsg != "" {
		desc := strings.TrimSpace(q.Get("error_description"))
		msg := "Google authorization failed."
		if desc != "" {
			msg = "Google authorization failed: " + desc
		}
		s.Metrics.IncCallback("google_error")
		s.logInfo("callback", "outcome", "google_error")
		writeHTMLError(w, http.StatusBadRequest, msg+" Return to shiet and retry.")
		return
	}

	code := strings.TrimSpace(q.Get("code"))
	stateID := strings.TrimSpace(q.Get("state"))
	if code == "" || stateID == "" {
		s.Metrics.IncCallback("missing_params")
		writeHTMLError(w, http.StatusBadRequest, "Missing OAuth code or state. Return to shiet and retry.")
		return
	}

	now := s.now()
	state, err := s.Store.ConsumeOAuthState(r.Context(), stateID, now)
	if err != nil {
		reason := "state_error"
		switch {
		case errors.Is(err, store.ErrAlreadyUsed):
			reason = "state_already_used"
			s.Metrics.IncQuotaRisk("state_replay")
			writeHTMLError(w, http.StatusBadRequest, "This Google authorization was already used. Return to shiet and start a new connect.")
		case errors.Is(err, store.ErrExpired):
			reason = "state_expired"
			writeHTMLError(w, http.StatusBadRequest, "This Google authorization expired. Return to shiet and start a new connect.")
		case errors.Is(err, store.ErrNotFound):
			reason = "state_not_found"
			writeHTMLError(w, http.StatusBadRequest, "Unknown Google authorization state. Return to shiet and start a new connect.")
		default:
			writeHTMLError(w, http.StatusInternalServerError, "Broker could not validate authorization state. Return to shiet and retry.")
		}
		s.Metrics.IncCallback(reason)
		s.logInfo("callback", "outcome", reason)
		return
	}

	tok, err := s.exchangeGoogleCode(r.Context(), code, state.PKCEVerifier)
	if err != nil {
		s.Metrics.IncCallback("token_exchange_failed")
		s.logInfo("callback", "outcome", "token_exchange_failed")
		writeHTMLError(w, http.StatusBadGateway, "Broker could not exchange the Google authorization code. Return to shiet and retry.")
		return
	}

	handoffCode, err := randomString(32)
	if err != nil {
		s.Metrics.IncCallback("handoff_mint_failed")
		writeHTMLError(w, http.StatusInternalServerError, "Broker could not create a handoff code. Return to shiet and retry.")
		return
	}
	payload, err := encryptTokenPayload(
		s.Config.GoogleClientSecret,
		handoffAAD(state.ID, state.DesktopSessionID, state.HandoffChallenge),
		tokenPayload{
			AccessToken:  tok.AccessToken,
			RefreshToken: tok.RefreshToken,
			TokenType:    tok.TokenType,
			Expiry:       now.Add(time.Duration(tok.ExpiresIn) * time.Second),
		},
	)
	if err != nil {
		s.Metrics.IncCallback("seal_failed")
		writeHTMLError(w, http.StatusInternalServerError, "Broker could not seal token material. Return to shiet and retry.")
		return
	}

	scopes := state.Scopes
	if tok.Scope != "" {
		scopes = strings.Fields(tok.Scope)
	}
	handoff := store.HandoffRecord{
		CodeHash:              hashHandoffCode(handoffCode),
		StateID:               state.ID,
		DesktopSessionID:      state.DesktopSessionID,
		HandoffChallenge:      state.HandoffChallenge,
		EncryptedTokenPayload: payload,
		AccountHint:           "",
		Scopes:                scopes,
		ExpiresAt:             now.Add(s.Config.HandoffTTL),
	}
	if err := s.Store.SaveHandoff(r.Context(), handoff); err != nil {
		s.Metrics.IncCallback("handoff_persist_failed")
		writeHTMLError(w, http.StatusInternalServerError, "Broker could not persist the handoff. Return to shiet and retry.")
		return
	}

	handoffURL, err := s.buildHandoffURL(state, handoffCode)
	if err != nil {
		s.Metrics.IncCallback("handoff_url_failed")
		writeHTMLError(w, http.StatusInternalServerError, "Broker could not build the desktop return link. Return to shiet and retry.")
		return
	}

	s.Metrics.IncCallback("ok")
	s.logInfo("callback", "outcome", "ok", "ip_bucket", ipBucket)
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	_, _ = io.WriteString(w, callbackSuccessPage(handoffURL))
}

func (s Server) exchangeHandoff(w http.ResponseWriter, r *http.Request) {
	if s.Store == nil {
		writeJSON(w, http.StatusServiceUnavailable, errorResponse{Error: "datastore_unavailable"})
		return
	}
	if s.rejectAuthDisabled(w, "handoff") {
		return
	}
	ipBucket := sourceIPBucket(r.RemoteAddr)
	if s.rejectRateLimited(w, "handoff", ratelimit.Key("handoff", ipBucket), limitHandoff) {
		return
	}

	var req handoffRequest
	if err := decodeJSON(r.Body, &req); err != nil {
		writeJSON(w, http.StatusBadRequest, errorResponse{Error: "invalid_json"})
		return
	}
	req.DesktopSessionID = strings.TrimSpace(req.DesktopSessionID)
	req.BrokerState = strings.TrimSpace(req.BrokerState)
	req.HandoffCode = strings.TrimSpace(req.HandoffCode)
	req.HandoffVerifier = strings.TrimSpace(req.HandoffVerifier)
	if req.DesktopSessionID == "" || req.BrokerState == "" || req.HandoffCode == "" || req.HandoffVerifier == "" {
		writeJSON(w, http.StatusBadRequest, errorResponse{Error: "handoff_fields_required"})
		return
	}

	codeHash := hashHandoffCode(req.HandoffCode)
	failKey := ratelimit.Key("handoff_fail", ipBucket+"|"+req.DesktopSessionID+"|"+codeHash)
	now := s.now()
	challenge := pkceS256(req.HandoffVerifier)
	rec, err := s.Store.ConsumeHandoff(
		r.Context(),
		codeHash,
		req.DesktopSessionID,
		req.BrokerState,
		challenge,
		now,
	)
	if err != nil {
		reason := "consume_failed"
		code := "handoff_consume_failed"
		status := http.StatusInternalServerError
		switch {
		case errors.Is(err, store.ErrAlreadyUsed):
			reason, code, status = "already_used", "handoff_already_used", http.StatusBadRequest
			s.Metrics.IncQuotaRisk("handoff_replay")
		case errors.Is(err, store.ErrExpired):
			reason, code, status = "expired", "handoff_expired", http.StatusBadRequest
		case errors.Is(err, store.ErrNotFound):
			reason, code, status = "not_found", "handoff_not_found", http.StatusBadRequest
		case errors.Is(err, store.ErrMismatch):
			reason, code, status = "state_mismatch", "handoff_state_mismatch", http.StatusBadRequest
			s.Metrics.IncQuotaRisk("handoff_mismatch")
		}
		s.Metrics.IncHandoffFailure(reason)
		s.logInfo("handoff", "outcome", reason, "ip_bucket", ipBucket)
		if !s.allow(failKey, limitHandoffFailure) {
			s.Metrics.IncRateLimited("handoff_failure")
			s.logInfo("rate_limited", "surface", "handoff_failure", "ip_bucket", ipBucket)
			writeJSON(w, http.StatusTooManyRequests, errorResponse{Error: "rate_limited"})
			return
		}
		writeJSON(w, status, errorResponse{Error: code})
		return
	}

	payload, err := decryptTokenPayload(
		s.Config.GoogleClientSecret,
		handoffAAD(rec.StateID, rec.DesktopSessionID, rec.HandoffChallenge),
		rec.EncryptedTokenPayload,
	)
	if err != nil {
		s.Metrics.IncHandoffFailure("payload_invalid")
		writeJSON(w, http.StatusInternalServerError, errorResponse{Error: "handoff_payload_invalid"})
		return
	}

	var resp handoffResponse
	resp.Provider = "google"
	resp.AccountHint = rec.AccountHint
	resp.Scope = append([]string(nil), rec.Scopes...)
	resp.Token.AccessToken = payload.AccessToken
	resp.Token.RefreshToken = payload.RefreshToken
	resp.Token.TokenType = payload.TokenType
	if resp.Token.TokenType == "" {
		resp.Token.TokenType = "Bearer"
	}
	resp.Token.Expiry = payload.Expiry
	s.Metrics.IncHandoffOK()
	s.logInfo("handoff", "outcome", "ok", "ip_bucket", ipBucket)
	writeJSON(w, http.StatusOK, resp)
}

func (s Server) refreshGoogleOAuth(w http.ResponseWriter, r *http.Request) {
	if s.Config.RefreshDisabled {
		s.Metrics.IncKillSwitch("refresh")
		s.logInfo("kill_switch", "surface", "refresh", "reason", "refresh_disabled")
		writeJSON(w, http.StatusForbidden, errorResponse{Error: "refresh_disabled"})
		return
	}
	ipBucket := sourceIPBucket(r.RemoteAddr)
	if s.rejectRateLimited(w, "refresh", ratelimit.Key("refresh", ipBucket), limitRefresh) {
		return
	}

	var req refreshRequest
	if err := decodeJSON(r.Body, &req); err != nil {
		writeJSON(w, http.StatusBadRequest, errorResponse{Error: "invalid_json"})
		return
	}
	req.RefreshToken = strings.TrimSpace(req.RefreshToken)
	req.AppVersion = strings.TrimSpace(req.AppVersion)
	if req.RefreshToken == "" {
		writeJSON(w, http.StatusBadRequest, errorResponse{Error: "refresh_token_required"})
		return
	}
	if s.rejectAppVersionDisabled(w, "refresh", req.AppVersion) {
		return
	}

	form := url.Values{}
	form.Set("grant_type", "refresh_token")
	form.Set("refresh_token", req.RefreshToken)
	form.Set("client_id", s.Config.GoogleClientID)
	form.Set("client_secret", s.Config.GoogleClientSecret)
	if len(req.Scope) > 0 {
		form.Set("scope", strings.Join(req.Scope, " "))
	}

	tok, err := s.postGoogleToken(r.Context(), form)
	if err != nil {
		failKey := ratelimit.Key("refresh_fail", ipBucket)
		if !s.allow(failKey, limitRefreshFailure) {
			s.Metrics.IncRateLimited("refresh_failure")
			s.logInfo("rate_limited", "surface", "refresh_failure", "ip_bucket", ipBucket)
			writeJSON(w, http.StatusTooManyRequests, errorResponse{Error: "rate_limited"})
			return
		}
		var ge *googleTokenError
		if errors.As(err, &ge) && ge.Code == "invalid_grant" {
			s.Metrics.IncRefreshFailure("invalid_grant")
			s.Metrics.IncQuotaRisk("invalid_grant")
			s.logInfo("refresh", "outcome", "invalid_grant", "ip_bucket", ipBucket, "app_version", req.AppVersion)
			writeJSON(w, http.StatusBadRequest, errorResponse{Error: "invalid_refresh_token"})
			return
		}
		s.Metrics.IncRefreshFailure("google_failed")
		s.logInfo("refresh", "outcome", "google_failed", "ip_bucket", ipBucket)
		writeJSON(w, http.StatusBadGateway, errorResponse{Error: "google_token_refresh_failed"})
		return
	}

	now := s.now()
	resp := refreshResponse{
		AccessToken:  tok.AccessToken,
		RefreshToken: tok.RefreshToken,
		TokenType:    tok.TokenType,
		Expiry:       now.Add(time.Duration(tok.ExpiresIn) * time.Second),
	}
	s.Metrics.IncRefreshOK()
	s.logInfo("refresh", "outcome", "ok", "ip_bucket", ipBucket, "app_version", req.AppVersion)
	writeJSON(w, http.StatusOK, resp)
}

type googleTokenError struct {
	Code string
	Desc string
}

func (e *googleTokenError) Error() string {
	if e.Desc != "" {
		return fmt.Sprintf("google token error %s: %s", e.Code, e.Desc)
	}
	return fmt.Sprintf("google token error %s", e.Code)
}

// revokeGoogleOAuth asks Google to revoke a refresh token supplied by the
// desktop. The broker does not persist the token or any account record.
// Revoke stays available when auth/refresh kill switches are on so users can
// disconnect during an incident.
func (s Server) revokeGoogleOAuth(w http.ResponseWriter, r *http.Request) {
	ipBucket := sourceIPBucket(r.RemoteAddr)
	if s.rejectRateLimited(w, "revoke", ratelimit.Key("revoke", ipBucket), limitRevoke) {
		return
	}

	var req revokeRequest
	if err := decodeJSON(r.Body, &req); err != nil {
		writeJSON(w, http.StatusBadRequest, errorResponse{Error: "invalid_json"})
		return
	}
	req.RefreshToken = strings.TrimSpace(req.RefreshToken)
	req.Reason = strings.TrimSpace(req.Reason)
	if req.RefreshToken == "" {
		writeJSON(w, http.StatusBadRequest, errorResponse{Error: "refresh_token_required"})
		return
	}

	if err := s.revokeGoogleToken(r.Context(), req.RefreshToken); err != nil {
		if errors.Is(err, errGoogleTokenAlreadyRevoked) {
			s.Metrics.IncRevokeOK()
			s.Metrics.IncRevokeOutcome("already_revoked")
			s.logInfo("revoke", "outcome", "already_revoked", "reason", req.Reason, "ip_bucket", ipBucket)
			writeJSON(w, http.StatusOK, revokeResponse{Revoked: true})
			return
		}
		s.Metrics.IncRevokeOutcome("google_failed")
		s.logInfo("revoke", "outcome", "google_failed", "reason", req.Reason, "ip_bucket", ipBucket)
		writeJSON(w, http.StatusBadGateway, errorResponse{Error: "google_revoke_failed"})
		return
	}
	s.Metrics.IncRevokeOK()
	s.Metrics.IncRevokeOutcome("ok")
	s.logInfo("revoke", "outcome", "ok", "reason", req.Reason, "ip_bucket", ipBucket)
	writeJSON(w, http.StatusOK, revokeResponse{Revoked: true})
}

var errGoogleTokenAlreadyRevoked = errors.New("google token already revoked")

func (s Server) revokeGoogleToken(ctx context.Context, refreshToken string) error {
	form := url.Values{}
	form.Set("token", refreshToken)

	revokeURL := s.GoogleRevokeURL
	if revokeURL == "" {
		revokeURL = defaultGoogleRevoke
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, revokeURL, strings.NewReader(form.Encode()))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	client := s.HTTPClient
	if client == nil {
		client = http.DefaultClient
	}
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return err
	}
	if resp.StatusCode >= 200 && resp.StatusCode < 300 {
		return nil
	}
	if isGoogleInvalidToken(resp.StatusCode, body) {
		return errGoogleTokenAlreadyRevoked
	}
	return fmt.Errorf("google revoke failed: status %d", resp.StatusCode)
}

func isGoogleInvalidToken(status int, body []byte) bool {
	if status != http.StatusBadRequest {
		return false
	}
	var er struct {
		Error string `json:"error"`
	}
	if err := json.Unmarshal(body, &er); err == nil {
		if strings.EqualFold(strings.TrimSpace(er.Error), "invalid_token") {
			return true
		}
	}
	// Google sometimes returns plain text or form-ish bodies.
	return strings.Contains(strings.ToLower(string(body)), "invalid_token")
}

func (s Server) exchangeGoogleCode(ctx context.Context, code, pkceVerifier string) (googleTokenResponse, error) {
	form := url.Values{}
	form.Set("grant_type", "authorization_code")
	form.Set("code", code)
	form.Set("client_id", s.Config.GoogleClientID)
	form.Set("client_secret", s.Config.GoogleClientSecret)
	form.Set("redirect_uri", s.Config.RedirectURI())
	form.Set("code_verifier", pkceVerifier)

	tok, err := s.postGoogleToken(ctx, form)
	if err != nil {
		return googleTokenResponse{}, fmt.Errorf("google token exchange failed")
	}
	return tok, nil
}

func (s Server) postGoogleToken(ctx context.Context, form url.Values) (googleTokenResponse, error) {
	tokenURL := s.GoogleTokenURL
	if tokenURL == "" {
		tokenURL = defaultGoogleToken
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, tokenURL, strings.NewReader(form.Encode()))
	if err != nil {
		return googleTokenResponse{}, err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	client := s.HTTPClient
	if client == nil {
		client = http.DefaultClient
	}
	resp, err := client.Do(req)
	if err != nil {
		return googleTokenResponse{}, err
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return googleTokenResponse{}, err
	}
	var tok googleTokenResponse
	if err := json.Unmarshal(body, &tok); err != nil {
		return googleTokenResponse{}, err
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 || tok.AccessToken == "" || tok.Error != "" {
		code := tok.Error
		if code == "" {
			code = "token_request_failed"
		}
		return googleTokenResponse{}, &googleTokenError{Code: code, Desc: tok.ErrorDesc}
	}
	if tok.TokenType == "" {
		tok.TokenType = "Bearer"
	}
	if tok.ExpiresIn <= 0 {
		tok.ExpiresIn = 3600
	}
	return tok, nil
}

func (s Server) buildHandoffURL(state store.OAuthState, handoffCode string) (string, error) {
	base := strings.TrimSpace(state.DesktopHandoffRedirect)
	if base == "" {
		base = strings.TrimSpace(s.Config.DesktopHandoffURL)
	}
	u, err := url.Parse(base)
	if err != nil {
		return "", err
	}
	q := u.Query()
	q.Set("broker_state", state.ID)
	q.Set("handoff_code", handoffCode)
	u.RawQuery = q.Encode()
	return u.String(), nil
}

func (s Server) authURL(state, codeChallenge string) (string, error) {
	redirectURI := s.Config.RedirectURI()
	if redirectURI == "" {
		return "", errors.New("missing redirect uri")
	}
	u, err := url.Parse(googleAuthURL)
	if err != nil {
		return "", err
	}
	q := u.Query()
	q.Set("client_id", s.Config.GoogleClientID)
	q.Set("redirect_uri", redirectURI)
	q.Set("response_type", "code")
	q.Set("scope", strings.Join(s.Config.GoogleScopes, " "))
	q.Set("state", state)
	q.Set("access_type", "offline")
	q.Set("prompt", "consent")
	q.Set("code_challenge", codeChallenge)
	q.Set("code_challenge_method", "S256")
	u.RawQuery = q.Encode()
	return u.String(), nil
}

func (s Server) now() time.Time {
	if s.Clock != nil {
		return s.Clock()
	}
	return time.Now().UTC()
}

func validateDesktopHandoffRedirect(raw string) error {
	u, err := url.Parse(raw)
	if err != nil {
		return err
	}
	if u.Scheme != "http" {
		return errors.New("must be http loopback")
	}
	if u.User != nil {
		return errors.New("must not include userinfo")
	}
	if u.Hostname() != "127.0.0.1" {
		return errors.New("must be 127.0.0.1")
	}
	if u.Path == "" || u.RawQuery != "" || u.Fragment != "" {
		return errors.New("path required without query or fragment")
	}
	return nil
}

func callbackSuccessPage(handoffURL string) string {
	safe := html.EscapeString(handoffURL)
	return "<!doctype html><html><body>" +
		"<p>Authorization complete. Return to shiet to finish connecting Google Calendar.</p>" +
		`<p><a href="` + safe + `">Open shiet</a></p>` +
		`<meta http-equiv="refresh" content="0;url=` + safe + `">` +
		"</body></html>"
}

func writeHTMLError(w http.ResponseWriter, status int, message string) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(status)
	_, _ = io.WriteString(w, "<!doctype html><html><body><p>"+html.EscapeString(message)+"</p></body></html>")
}

func notImplemented(endpoint string) http.HandlerFunc {
	return func(w http.ResponseWriter, _ *http.Request) {
		writeJSON(w, http.StatusNotImplemented, errorResponse{Error: fmt.Sprintf("%s_not_implemented", endpoint)})
	}
}

func decodeJSON(body io.Reader, out any) error {
	dec := json.NewDecoder(io.LimitReader(body, 1<<20))
	dec.DisallowUnknownFields()
	return dec.Decode(out)
}

func writeJSON(w http.ResponseWriter, status int, payload any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(payload)
}

func randomString(bytes int) (string, error) {
	buf := make([]byte, bytes)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(buf), nil
}

func pkceS256(verifier string) string {
	sum := sha256.Sum256([]byte(verifier))
	return base64.RawURLEncoding.EncodeToString(sum[:])
}

func sourceIPBucket(remoteAddr string) string {
	host, _, err := net.SplitHostPort(remoteAddr)
	if err != nil {
		host = remoteAddr
	}
	ip := net.ParseIP(host)
	if ip == nil {
		return ""
	}
	if ip4 := ip.To4(); ip4 != nil {
		return fmt.Sprintf("%d.%d.%d.0/24", ip4[0], ip4[1], ip4[2])
	}
	ip16 := ip.To16()
	if ip16 == nil {
		return ""
	}
	return fmt.Sprintf("%x:%x:%x:%x::/64",
		uint16(ip16[0])<<8|uint16(ip16[1]),
		uint16(ip16[2])<<8|uint16(ip16[3]),
		uint16(ip16[4])<<8|uint16(ip16[5]),
		uint16(ip16[6])<<8|uint16(ip16[7]),
	)
}

func (s Server) allow(key string, limit int) bool {
	if s.Limiter == nil {
		return true
	}
	return s.Limiter.Allow(key, limit)
}

func (s Server) rejectRateLimited(w http.ResponseWriter, surface, key string, limit int) bool {
	if s.allow(key, limit) {
		return false
	}
	s.Metrics.IncRateLimited(surface)
	ipBucket := ""
	if parts := strings.SplitN(key, "|", 2); len(parts) == 2 {
		ipBucket = parts[1]
	}
	s.logInfo("rate_limited", "surface", surface, "ip_bucket", ipBucket)
	writeJSON(w, http.StatusTooManyRequests, errorResponse{Error: "rate_limited"})
	return true
}

func (s Server) rejectAuthDisabled(w http.ResponseWriter, surface string) bool {
	if !s.Config.AuthDisabled {
		return false
	}
	s.Metrics.IncKillSwitch(surface)
	s.logInfo("kill_switch", "surface", surface, "reason", "auth_disabled")
	writeJSON(w, http.StatusForbidden, errorResponse{Error: "auth_disabled"})
	return true
}

func (s Server) rejectAppVersionDisabled(w http.ResponseWriter, surface, appVersion string) bool {
	if !s.Config.AppVersionDisabled(appVersion) {
		return false
	}
	s.Metrics.IncKillSwitch(surface + "_version")
	s.logInfo("kill_switch", "surface", surface, "reason", "app_version_disabled", "app_version", appVersion)
	writeJSON(w, http.StatusForbidden, errorResponse{Error: "app_version_disabled"})
	return true
}

func (s Server) logInfo(msg string, args ...any) {
	if s.Logger == nil {
		return
	}
	s.Logger.Info(msg, args...)
}
