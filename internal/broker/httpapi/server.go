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

	"github.com/dylanbr0wn/shiet/internal/broker/codes"
	brokerconfig "github.com/dylanbr0wn/shiet/internal/broker/config"
	"github.com/dylanbr0wn/shiet/internal/broker/observe"
	"github.com/dylanbr0wn/shiet/internal/broker/ratelimit"
	"github.com/dylanbr0wn/shiet/internal/broker/store"
	"github.com/dylanbr0wn/shiet/internal/integration/oauth"
)

const (
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
	ConsumeOAuthState(context.Context, string, string, time.Time) (store.OAuthState, error)
	SaveHandoff(context.Context, store.HandoffRecord) error
	ConsumeHandoff(context.Context, string, string, string, string, string, time.Time) (store.HandoffRecord, error)
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
	GitHubTokenURL  string // override for tests
	GitHubRevokeURL string // override for tests
	Limiter         Limiter
	Metrics         *observe.Metrics
	Logger          *slog.Logger
}

type startRequest = oauth.BrokerStartRequest
type startResponse = oauth.BrokerStartResponse
type handoffRequest = oauth.BrokerHandoffRequest
type handoffResponse = oauth.BrokerHandoffResponse
type refreshRequest = oauth.BrokerRefreshRequest
type refreshResponse = oauth.BrokerRefreshResponse
type revokeRequest = oauth.BrokerRevokeRequest
type revokeResponse = oauth.BrokerRevokeResponse
type errorResponse = oauth.BrokerErrorResponse

type statusResponse struct {
	Status string `json:"status"`
}

type providerTokenResponse struct {
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
	for _, provider := range oauth.All() {
		id := provider.ID
		mux.HandleFunc("POST /v1/"+id+"/oauth/start", s.startHandler(id))
		mux.HandleFunc("GET /v1/"+id+"/oauth/callback", s.callbackHandler(id))
		mux.HandleFunc("POST /v1/"+id+"/oauth/handoff", s.handoffHandler(id))
		if provider.Capabilities.Refresh {
			mux.HandleFunc("POST /v1/"+id+"/oauth/refresh", s.refreshGoogleOAuth)
		}
		if provider.Capabilities.Revoke {
			switch id {
			case oauth.ProviderGoogle:
				mux.HandleFunc("POST /v1/"+id+"/oauth/revoke", s.revokeGoogleOAuth)
			case oauth.ProviderGitHub:
				mux.HandleFunc("POST /v1/"+id+"/oauth/revoke", s.revokeGitHubOAuth)
			}
		}
	}
	return mux
}

func (s Server) startHandler(provider string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		s.startOAuth(w, r, provider)
	}
}

func (s Server) callbackHandler(provider string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		s.oauthCallback(w, r, provider)
	}
}

func (s Server) handoffHandler(provider string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		s.exchangeProviderHandoff(w, r, provider)
	}
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
		writeJSON(w, http.StatusServiceUnavailable, errorResponse{Error: codes.InvalidConfig})
		return
	}
	if s.Store == nil {
		writeJSON(w, http.StatusServiceUnavailable, errorResponse{Error: codes.DatastoreUnavailable})
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), time.Second)
	defer cancel()
	if err := s.Store.Ping(ctx); err != nil {
		writeJSON(w, http.StatusServiceUnavailable, errorResponse{Error: codes.DatastoreUnavailable})
		return
	}
	writeJSON(w, http.StatusOK, statusResponse{Status: "ready"})
}

func (s Server) startOAuth(w http.ResponseWriter, r *http.Request, provider string) {
	if s.Store == nil {
		writeJSON(w, http.StatusServiceUnavailable, errorResponse{Error: codes.DatastoreUnavailable})
		return
	}
	if !s.providerConfigured(provider) {
		writeJSON(w, http.StatusServiceUnavailable, errorResponse{Error: codes.ProviderNotConfigured})
		return
	}
	if s.rejectAuthDisabled(w, codes.SurfaceStart) {
		return
	}
	ipBucket := sourceIPBucket(r.RemoteAddr)
	if s.rejectRateLimited(w, codes.SurfaceStart, ratelimit.Key(codes.LimitKeyStart, ipBucket), limitStart) {
		return
	}

	var req startRequest
	if err := decodeJSON(r.Body, &req); err != nil {
		s.Metrics.IncAuthStartFail()
		writeJSON(w, http.StatusBadRequest, errorResponse{Error: codes.InvalidJSON})
		return
	}
	req.DesktopSessionID = strings.TrimSpace(req.DesktopSessionID)
	req.HandoffChallenge = strings.TrimSpace(req.HandoffChallenge)
	req.DesktopHandoffRedirect = strings.TrimSpace(req.DesktopHandoffRedirect)
	req.AppVersion = strings.TrimSpace(req.AppVersion)
	if req.DesktopSessionID == "" || req.HandoffChallenge == "" {
		s.Metrics.IncAuthStartFail()
		writeJSON(w, http.StatusBadRequest, errorResponse{Error: codes.DesktopSessionAndHandoffChallengeRequired})
		return
	}
	if s.rejectAppVersionDisabled(w, codes.SurfaceStart, req.AppVersion) {
		return
	}
	if req.DesktopHandoffRedirect != "" {
		if err := validateDesktopHandoffRedirect(req.DesktopHandoffRedirect); err != nil {
			s.Metrics.IncAuthStartFail()
			writeJSON(w, http.StatusBadRequest, errorResponse{Error: codes.InvalidDesktopHandoffRedirect})
			return
		}
	}

	state, err := randomString(32)
	if err != nil {
		s.Metrics.IncAuthStartFail()
		writeJSON(w, http.StatusInternalServerError, errorResponse{Error: codes.RandomStateFailed})
		return
	}
	verifier, err := randomString(64)
	if err != nil {
		s.Metrics.IncAuthStartFail()
		writeJSON(w, http.StatusInternalServerError, errorResponse{Error: codes.RandomVerifierFailed})
		return
	}
	challenge := pkceS256(verifier)
	now := s.now()
	expiresAt := now.Add(s.Config.StateTTL)

	rec := store.OAuthState{
		ID:                     state,
		Provider:               provider,
		DesktopSessionID:       req.DesktopSessionID,
		PKCEVerifier:           verifier,
		PKCEChallenge:          challenge,
		HandoffChallenge:       req.HandoffChallenge,
		DesktopHandoffRedirect: req.DesktopHandoffRedirect,
		Scopes:                 append([]string(nil), s.providerScopes(provider)...),
		AppVersion:             req.AppVersion,
		Platform:               strings.TrimSpace(req.Platform),
		SourceIPBucket:         ipBucket,
		ExpiresAt:              expiresAt,
	}
	if err := s.Store.SaveOAuthState(r.Context(), rec); err != nil {
		s.Metrics.IncAuthStartFail()
		writeJSON(w, http.StatusInternalServerError, errorResponse{Error: codes.StatePersistFailed})
		return
	}

	authURL, err := s.authURL(provider, state, challenge)
	if err != nil {
		s.Metrics.IncAuthStartFail()
		writeJSON(w, http.StatusInternalServerError, errorResponse{Error: codes.AuthURLFailed})
		return
	}
	s.Metrics.IncAuthStart()
	s.logInfo(codes.EventAuthStart, "outcome", codes.OutcomeOK, "app_version", req.AppVersion, "platform", strings.TrimSpace(req.Platform), "ip_bucket", ipBucket)
	writeJSON(w, http.StatusCreated, startResponse{
		AuthURL:     authURL,
		BrokerState: state,
		ExpiresAt:   expiresAt,
	})
}

func (s Server) oauthCallback(w http.ResponseWriter, r *http.Request, provider string) {
	desc, ok := oauth.Lookup(provider)
	providerName := provider
	if ok {
		providerName = desc.DisplayName
	}
	if s.Store == nil {
		writeHTMLError(w, http.StatusServiceUnavailable, "Broker datastore unavailable. Return to shiet and retry.")
		return
	}
	if !s.providerConfigured(provider) {
		writeHTMLError(w, http.StatusServiceUnavailable, providerName+" OAuth is not configured on this broker.")
		return
	}
	if s.Config.AuthDisabled {
		s.Metrics.IncKillSwitch(codes.SurfaceCallback)
		s.logInfo(codes.EventKillSwitch, "surface", codes.SurfaceCallback, "reason", codes.AuthDisabled)
		writeHTMLError(w, http.StatusForbidden, providerName+" connect is temporarily disabled. Return to shiet and try again later.")
		return
	}
	ipBucket := sourceIPBucket(r.RemoteAddr)
	if !s.allow(ratelimit.Key(codes.LimitKeyCallback, ipBucket), limitCallback) {
		s.Metrics.IncRateLimited(codes.SurfaceCallback)
		s.logInfo(codes.EventRateLimited, "surface", codes.SurfaceCallback, "ip_bucket", ipBucket)
		writeHTMLError(w, http.StatusTooManyRequests, "Too many authorization attempts. Return to shiet and try again later.")
		return
	}

	q := r.URL.Query()
	if errMsg := strings.TrimSpace(q.Get("error")); errMsg != "" {
		desc := strings.TrimSpace(q.Get("error_description"))
		msg := providerName + " authorization failed."
		if desc != "" {
			msg = providerName + " authorization failed: " + desc
		}
		outcome := codes.OutcomeProviderError
		if provider == oauth.ProviderGoogle {
			outcome = codes.OutcomeGoogleError
		}
		s.Metrics.IncCallback(outcome)
		s.logInfo(codes.EventCallback, "provider", provider, "outcome", outcome)
		writeHTMLError(w, http.StatusBadRequest, msg+" Return to shiet and retry.")
		return
	}

	code := strings.TrimSpace(q.Get("code"))
	stateID := strings.TrimSpace(q.Get("state"))
	if code == "" || stateID == "" {
		s.Metrics.IncCallback(codes.OutcomeMissingParams)
		writeHTMLError(w, http.StatusBadRequest, "Missing OAuth code or state. Return to shiet and retry.")
		return
	}

	now := s.now()
	state, err := s.Store.ConsumeOAuthState(r.Context(), stateID, provider, now)
	if err != nil {
		reason := codes.OutcomeStateError
		switch {
		case errors.Is(err, store.ErrAlreadyUsed):
			reason = codes.OutcomeStateAlreadyUsed
			s.Metrics.IncQuotaRisk(codes.QuotaStateReplay)
			writeHTMLError(w, http.StatusBadRequest, "This "+providerName+" authorization was already used. Return to shiet and start a new connect.")
		case errors.Is(err, store.ErrExpired):
			reason = codes.OutcomeStateExpired
			writeHTMLError(w, http.StatusBadRequest, "This "+providerName+" authorization expired. Return to shiet and start a new connect.")
		case errors.Is(err, store.ErrNotFound):
			reason = codes.OutcomeStateNotFound
			writeHTMLError(w, http.StatusBadRequest, "Unknown "+providerName+" authorization state. Return to shiet and start a new connect.")
		case errors.Is(err, store.ErrMismatch):
			reason = codes.OutcomeStateMismatch
			writeHTMLError(w, http.StatusBadRequest, "OAuth provider mismatch. Return to shiet and start a new connect.")
		default:
			writeHTMLError(w, http.StatusInternalServerError, "Broker could not validate authorization state. Return to shiet and retry.")
		}
		s.Metrics.IncCallback(reason)
		s.logInfo(codes.EventCallback, "outcome", reason)
		return
	}
	tok, err := s.exchangeProviderCode(r.Context(), provider, code, state.PKCEVerifier)
	if err != nil {
		s.Metrics.IncCallback(codes.OutcomeTokenExchangeFail)
		s.logInfo(codes.EventCallback, "outcome", codes.OutcomeTokenExchangeFail)
		writeHTMLError(w, http.StatusBadGateway, "Broker could not exchange the "+providerName+" authorization code. Return to shiet and retry.")
		return
	}

	handoffCode, err := randomString(32)
	if err != nil {
		s.Metrics.IncCallback(codes.OutcomeHandoffMintFailed)
		writeHTMLError(w, http.StatusInternalServerError, "Broker could not create a handoff code. Return to shiet and retry.")
		return
	}
	payload, err := encryptTokenPayload(
		s.providerClientSecret(provider),
		handoffAAD(state.ID, state.DesktopSessionID, state.HandoffChallenge),
		tokenPayload{
			AccessToken:  tok.AccessToken,
			RefreshToken: tok.RefreshToken,
			TokenType:    tok.TokenType,
			Expiry:       tokenExpiry(now, tok.ExpiresIn),
		},
	)
	if err != nil {
		s.Metrics.IncCallback(codes.OutcomeSealFailed)
		writeHTMLError(w, http.StatusInternalServerError, "Broker could not seal token material. Return to shiet and retry.")
		return
	}

	scopes := state.Scopes
	if tok.Scope != "" {
		scopes = splitProviderScopes(provider, tok.Scope)
	}
	handoff := store.HandoffRecord{
		CodeHash:              hashHandoffCode(handoffCode),
		Provider:              provider,
		StateID:               state.ID,
		DesktopSessionID:      state.DesktopSessionID,
		HandoffChallenge:      state.HandoffChallenge,
		EncryptedTokenPayload: payload,
		AccountHint:           "",
		Scopes:                scopes,
		ExpiresAt:             now.Add(s.Config.HandoffTTL),
	}
	if err := s.Store.SaveHandoff(r.Context(), handoff); err != nil {
		s.Metrics.IncCallback(codes.OutcomeHandoffPersistFail)
		writeHTMLError(w, http.StatusInternalServerError, "Broker could not persist the handoff. Return to shiet and retry.")
		return
	}

	handoffURL, err := s.buildHandoffURL(state, handoffCode)
	if err != nil {
		s.Metrics.IncCallback(codes.OutcomeHandoffURLFailed)
		writeHTMLError(w, http.StatusInternalServerError, "Broker could not build the desktop return link. Return to shiet and retry.")
		return
	}

	s.Metrics.IncCallback(codes.OutcomeOK)
	s.logInfo(codes.EventCallback, "outcome", codes.OutcomeOK, "ip_bucket", ipBucket)
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	_, _ = io.WriteString(w, callbackSuccessPage(providerName, handoffURL))
}

func (s Server) exchangeProviderHandoff(w http.ResponseWriter, r *http.Request, provider string) {
	if s.Store == nil {
		writeJSON(w, http.StatusServiceUnavailable, errorResponse{Error: codes.DatastoreUnavailable})
		return
	}
	if !s.providerConfigured(provider) {
		writeJSON(w, http.StatusServiceUnavailable, errorResponse{Error: codes.ProviderNotConfigured})
		return
	}
	if s.rejectAuthDisabled(w, codes.SurfaceHandoff) {
		return
	}
	ipBucket := sourceIPBucket(r.RemoteAddr)
	if s.rejectRateLimited(w, codes.SurfaceHandoff, ratelimit.Key(codes.LimitKeyHandoff, ipBucket), limitHandoff) {
		return
	}

	var req handoffRequest
	if err := decodeJSON(r.Body, &req); err != nil {
		writeJSON(w, http.StatusBadRequest, errorResponse{Error: codes.InvalidJSON})
		return
	}
	req.DesktopSessionID = strings.TrimSpace(req.DesktopSessionID)
	req.BrokerState = strings.TrimSpace(req.BrokerState)
	req.HandoffCode = strings.TrimSpace(req.HandoffCode)
	req.HandoffVerifier = strings.TrimSpace(req.HandoffVerifier)
	if req.DesktopSessionID == "" || req.BrokerState == "" || req.HandoffCode == "" || req.HandoffVerifier == "" {
		writeJSON(w, http.StatusBadRequest, errorResponse{Error: codes.HandoffFieldsRequired})
		return
	}

	codeHash := hashHandoffCode(req.HandoffCode)
	failKey := ratelimit.Key(codes.LimitKeyHandoffFail, ipBucket+"|"+req.DesktopSessionID+"|"+codeHash)
	now := s.now()
	challenge := pkceS256(req.HandoffVerifier)
	rec, err := s.Store.ConsumeHandoff(
		r.Context(),
		codeHash,
		provider,
		req.DesktopSessionID,
		req.BrokerState,
		challenge,
		now,
	)
	if err != nil {
		reason := codes.OutcomeConsumeFailed
		code := codes.HandoffConsumeFailed
		status := http.StatusInternalServerError
		switch {
		case errors.Is(err, store.ErrAlreadyUsed):
			reason, code, status = codes.OutcomeAlreadyUsed, codes.HandoffAlreadyUsed, http.StatusBadRequest
			s.Metrics.IncQuotaRisk(codes.QuotaHandoffReplay)
		case errors.Is(err, store.ErrExpired):
			reason, code, status = codes.OutcomeExpired, codes.HandoffExpired, http.StatusBadRequest
		case errors.Is(err, store.ErrNotFound):
			reason, code, status = codes.OutcomeNotFound, codes.HandoffNotFound, http.StatusBadRequest
		case errors.Is(err, store.ErrMismatch):
			reason, code, status = codes.OutcomeStateMismatch, codes.HandoffStateMismatch, http.StatusBadRequest
			s.Metrics.IncQuotaRisk(codes.QuotaHandoffMismatch)
		}
		s.Metrics.IncHandoffFailure(reason)
		s.logInfo(codes.EventHandoff, "outcome", reason, "ip_bucket", ipBucket)
		if !s.allow(failKey, limitHandoffFailure) {
			s.Metrics.IncRateLimited(codes.SurfaceHandoffFailure)
			s.logInfo(codes.EventRateLimited, "surface", codes.SurfaceHandoffFailure, "ip_bucket", ipBucket)
			writeJSON(w, http.StatusTooManyRequests, errorResponse{Error: codes.RateLimited})
			return
		}
		writeJSON(w, status, errorResponse{Error: code})
		return
	}
	payload, err := decryptTokenPayload(
		s.providerClientSecret(provider),
		handoffAAD(rec.StateID, rec.DesktopSessionID, rec.HandoffChallenge),
		rec.EncryptedTokenPayload,
	)
	if err != nil {
		s.Metrics.IncHandoffFailure(codes.OutcomePayloadInvalid)
		writeJSON(w, http.StatusInternalServerError, errorResponse{Error: codes.HandoffPayloadInvalid})
		return
	}

	var resp handoffResponse
	resp.Provider = provider
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
	s.logInfo(codes.EventHandoff, "outcome", codes.OutcomeOK, "ip_bucket", ipBucket)
	writeJSON(w, http.StatusOK, resp)
}

func (s Server) refreshGoogleOAuth(w http.ResponseWriter, r *http.Request) {
	if s.Config.RefreshDisabled {
		s.Metrics.IncKillSwitch(codes.SurfaceRefresh)
		s.logInfo(codes.EventKillSwitch, "surface", codes.SurfaceRefresh, "reason", codes.RefreshDisabled)
		writeJSON(w, http.StatusForbidden, errorResponse{Error: codes.RefreshDisabled})
		return
	}
	ipBucket := sourceIPBucket(r.RemoteAddr)
	if s.rejectRateLimited(w, codes.SurfaceRefresh, ratelimit.Key(codes.LimitKeyRefresh, ipBucket), limitRefresh) {
		return
	}

	var req refreshRequest
	if err := decodeJSON(r.Body, &req); err != nil {
		writeJSON(w, http.StatusBadRequest, errorResponse{Error: codes.InvalidJSON})
		return
	}
	req.RefreshToken = strings.TrimSpace(req.RefreshToken)
	req.AppVersion = strings.TrimSpace(req.AppVersion)
	if req.RefreshToken == "" {
		writeJSON(w, http.StatusBadRequest, errorResponse{Error: codes.RefreshTokenRequired})
		return
	}
	if s.rejectAppVersionDisabled(w, codes.SurfaceRefresh, req.AppVersion) {
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
		failKey := ratelimit.Key(codes.LimitKeyRefreshFail, ipBucket)
		if !s.allow(failKey, limitRefreshFailure) {
			s.Metrics.IncRateLimited(codes.SurfaceRefreshFailure)
			s.logInfo(codes.EventRateLimited, "surface", codes.SurfaceRefreshFailure, "ip_bucket", ipBucket)
			writeJSON(w, http.StatusTooManyRequests, errorResponse{Error: codes.RateLimited})
			return
		}
		var ge *providerTokenError
		if errors.As(err, &ge) && ge.Code == codes.GoogleInvalidGrant {
			s.Metrics.IncRefreshFailure(codes.OutcomeInvalidGrant)
			s.Metrics.IncQuotaRisk(codes.QuotaInvalidGrant)
			s.logInfo(codes.EventRefresh, "outcome", codes.OutcomeInvalidGrant, "ip_bucket", ipBucket, "app_version", req.AppVersion)
			writeJSON(w, http.StatusBadRequest, errorResponse{Error: codes.InvalidRefreshToken})
			return
		}
		s.Metrics.IncRefreshFailure(codes.OutcomeGoogleFailed)
		s.logInfo(codes.EventRefresh, "outcome", codes.OutcomeGoogleFailed, "ip_bucket", ipBucket)
		writeJSON(w, http.StatusBadGateway, errorResponse{Error: codes.GoogleTokenRefreshFailed})
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
	s.logInfo(codes.EventRefresh, "outcome", codes.OutcomeOK, "ip_bucket", ipBucket, "app_version", req.AppVersion)
	writeJSON(w, http.StatusOK, resp)
}

type providerTokenError struct {
	Code string
	Desc string
}

func (e *providerTokenError) Error() string {
	if e.Desc != "" {
		return fmt.Sprintf("provider token error %s: %s", e.Code, e.Desc)
	}
	return fmt.Sprintf("provider token error %s", e.Code)
}

// revokeGoogleOAuth asks Google to revoke a refresh token supplied by the
// desktop. The broker does not persist the token or any account record.
// Revoke stays available when auth/refresh kill switches are on so users can
// disconnect during an incident.
func (s Server) revokeGoogleOAuth(w http.ResponseWriter, r *http.Request) {
	ipBucket := sourceIPBucket(r.RemoteAddr)
	if s.rejectRateLimited(w, codes.SurfaceRevoke, ratelimit.Key(codes.LimitKeyRevoke, ipBucket), limitRevoke) {
		return
	}

	var req revokeRequest
	if err := decodeJSON(r.Body, &req); err != nil {
		writeJSON(w, http.StatusBadRequest, errorResponse{Error: codes.InvalidJSON})
		return
	}
	req.RefreshToken = strings.TrimSpace(req.RefreshToken)
	req.Reason = strings.TrimSpace(req.Reason)
	if req.RefreshToken == "" {
		writeJSON(w, http.StatusBadRequest, errorResponse{Error: codes.RefreshTokenRequired})
		return
	}

	if err := s.revokeGoogleToken(r.Context(), req.RefreshToken); err != nil {
		if errors.Is(err, errGoogleTokenAlreadyRevoked) {
			s.Metrics.IncRevokeOK()
			s.Metrics.IncRevokeOutcome(codes.OutcomeAlreadyRevoked)
			s.logInfo(codes.EventRevoke, "outcome", codes.OutcomeAlreadyRevoked, "reason", req.Reason, "ip_bucket", ipBucket)
			writeJSON(w, http.StatusOK, revokeResponse{Revoked: true})
			return
		}
		s.Metrics.IncRevokeOutcome(codes.OutcomeGoogleFailed)
		s.logInfo(codes.EventRevoke, "outcome", codes.OutcomeGoogleFailed, "reason", req.Reason, "ip_bucket", ipBucket)
		writeJSON(w, http.StatusBadGateway, errorResponse{Error: codes.GoogleRevokeFailed})
		return
	}
	s.Metrics.IncRevokeOK()
	s.Metrics.IncRevokeOutcome(codes.OutcomeOK)
	s.logInfo(codes.EventRevoke, "outcome", codes.OutcomeOK, "reason", req.Reason, "ip_bucket", ipBucket)
	writeJSON(w, http.StatusOK, revokeResponse{Revoked: true})
}

func (s Server) revokeGitHubOAuth(w http.ResponseWriter, r *http.Request) {
	if !s.providerConfigured(oauth.ProviderGitHub) {
		writeJSON(w, http.StatusServiceUnavailable, errorResponse{Error: codes.ProviderNotConfigured})
		return
	}
	ipBucket := sourceIPBucket(r.RemoteAddr)
	if s.rejectRateLimited(w, codes.SurfaceRevoke, ratelimit.Key(codes.LimitKeyRevoke, oauth.ProviderGitHub+"|"+ipBucket), limitRevoke) {
		return
	}
	var req revokeRequest
	if err := decodeJSON(r.Body, &req); err != nil {
		writeJSON(w, http.StatusBadRequest, errorResponse{Error: codes.InvalidJSON})
		return
	}
	req.AccessToken = strings.TrimSpace(req.AccessToken)
	req.Reason = strings.TrimSpace(req.Reason)
	if req.AccessToken == "" {
		writeJSON(w, http.StatusBadRequest, errorResponse{Error: codes.AccessTokenRequired})
		return
	}
	if err := s.revokeGitHubToken(r.Context(), req.AccessToken); err != nil {
		s.Metrics.IncRevokeOutcome(codes.OutcomeGitHubFailed)
		s.logInfo(codes.EventRevoke, "provider", oauth.ProviderGitHub, "outcome", codes.OutcomeGitHubFailed, "reason", req.Reason, "ip_bucket", ipBucket)
		writeJSON(w, http.StatusBadGateway, errorResponse{Error: codes.GitHubRevokeFailed})
		return
	}
	s.Metrics.IncRevokeOK()
	s.Metrics.IncRevokeOutcome(codes.OutcomeOK)
	s.logInfo(codes.EventRevoke, "provider", oauth.ProviderGitHub, "outcome", codes.OutcomeOK, "reason", req.Reason, "ip_bucket", ipBucket)
	writeJSON(w, http.StatusOK, revokeResponse{Revoked: true})
}

func (s Server) revokeGitHubToken(ctx context.Context, accessToken string) error {
	desc := oauth.MustLookup(oauth.ProviderGitHub)
	base := strings.TrimRight(strings.TrimSpace(s.GitHubRevokeURL), "/")
	if base == "" {
		base = strings.TrimRight(desc.RevokeURL, "/")
	}
	body, err := json.Marshal(map[string]string{"access_token": accessToken})
	if err != nil {
		return err
	}
	creds, ok := s.providerCredentials(oauth.ProviderGitHub)
	if !ok {
		return errors.New("github credentials not configured")
	}
	revokeURL := base + "/applications/" + url.PathEscape(creds.ClientID) + "/token"
	req, err := http.NewRequestWithContext(ctx, http.MethodDelete, revokeURL, strings.NewReader(string(body)))
	if err != nil {
		return err
	}
	req.SetBasicAuth(creds.ClientID, creds.ClientSecret)
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-GitHub-Api-Version", "2022-11-28")
	client := s.HTTPClient
	if client == nil {
		client = http.DefaultClient
	}
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	_, _ = io.Copy(io.Discard, io.LimitReader(resp.Body, 1<<20))
	if resp.StatusCode == http.StatusNoContent {
		return nil
	}
	return fmt.Errorf("github revoke failed: status %d", resp.StatusCode)
}

var errGoogleTokenAlreadyRevoked = errors.New("google token already revoked")

func (s Server) revokeGoogleToken(ctx context.Context, refreshToken string) error {
	form := url.Values{}
	form.Set("token", refreshToken)

	desc := oauth.MustLookup(oauth.ProviderGoogle)
	revokeURL := s.GoogleRevokeURL
	if revokeURL == "" {
		revokeURL = desc.RevokeURL
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

func (s Server) exchangeProviderCode(ctx context.Context, provider, code, pkceVerifier string) (providerTokenResponse, error) {
	desc, ok := oauth.Lookup(provider)
	if !ok {
		return providerTokenResponse{}, fmt.Errorf("unknown provider %q", provider)
	}
	creds, ok := s.providerCredentials(provider)
	if !ok {
		return providerTokenResponse{}, fmt.Errorf("%s credentials not configured", provider)
	}
	tok, err := oauth.ExchangeAuthorizationCode(ctx, desc, creds, s.providerRedirectURI(provider), code, pkceVerifier, oauth.ExchangeOptions{
		HTTPClient: s.HTTPClient,
		TokenURL:   s.providerTokenURL(desc),
	})
	if err != nil {
		var ee *oauth.ExchangeError
		if errors.As(err, &ee) {
			return providerTokenResponse{}, &providerTokenError{Code: ee.Code, Desc: ee.Description}
		}
		return providerTokenResponse{}, err
	}
	out := providerTokenResponse{
		AccessToken:  tok.AccessToken,
		RefreshToken: tok.RefreshToken,
		TokenType:    tok.TokenType,
		ExpiresIn:    tok.ExpiresIn,
		Scope:        tok.Scope,
	}
	if out.ExpiresIn <= 0 && provider == oauth.ProviderGoogle {
		out.ExpiresIn = 3600
	}
	return out, nil
}

func (s Server) postGoogleToken(ctx context.Context, form url.Values) (providerTokenResponse, error) {
	desc := oauth.MustLookup(oauth.ProviderGoogle)
	tokenURL := s.GoogleTokenURL
	if tokenURL == "" {
		tokenURL = desc.TokenURL
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, tokenURL, strings.NewReader(form.Encode()))
	if err != nil {
		return providerTokenResponse{}, err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	client := s.HTTPClient
	if client == nil {
		client = http.DefaultClient
	}
	resp, err := client.Do(req)
	if err != nil {
		return providerTokenResponse{}, err
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return providerTokenResponse{}, err
	}
	var tok providerTokenResponse
	if err := json.Unmarshal(body, &tok); err != nil {
		return providerTokenResponse{}, err
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 || tok.AccessToken == "" || tok.Error != "" {
		code := tok.Error
		if code == "" {
			code = "token_request_failed"
		}
		return providerTokenResponse{}, &providerTokenError{Code: code, Desc: tok.ErrorDesc}
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
		if providerOrGoogle(state.Provider) == oauth.ProviderGitHub {
			base = strings.TrimSpace(s.Config.GitHubDesktopHandoffURL)
		} else {
			base = strings.TrimSpace(s.Config.DesktopHandoffURL)
		}
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

func (s Server) authURL(provider, state, codeChallenge string) (string, error) {
	desc, ok := oauth.Lookup(provider)
	if !ok {
		return "", fmt.Errorf("unknown provider %q", provider)
	}
	creds, ok := s.providerCredentials(provider)
	if !ok {
		return "", fmt.Errorf("%s credentials not configured", provider)
	}
	return oauth.BuildAuthorizationURL(desc, creds, s.providerRedirectURI(provider), state, codeChallenge, s.providerScopes(provider))
}

func (s Server) providerScopes(provider string) []string {
	switch provider {
	case oauth.ProviderGitHub:
		if len(s.Config.GitHubScopes) > 0 {
			return s.Config.GitHubScopes
		}
	case oauth.ProviderGoogle:
		if len(s.Config.GoogleScopes) > 0 {
			return s.Config.GoogleScopes
		}
	}
	if desc, ok := oauth.Lookup(provider); ok {
		return append([]string(nil), desc.DefaultScopes...)
	}
	return nil
}

func (s Server) providerCredentials(provider string) (oauth.ClientCredentials, bool) {
	switch provider {
	case oauth.ProviderGitHub:
		id := strings.TrimSpace(s.Config.GitHubClientID)
		secret := strings.TrimSpace(s.Config.GitHubClientSecret)
		if id == "" || secret == "" {
			return oauth.ClientCredentials{}, false
		}
		return oauth.ClientCredentials{ClientID: id, ClientSecret: secret}, true
	case oauth.ProviderGoogle:
		id := strings.TrimSpace(s.Config.GoogleClientID)
		secret := strings.TrimSpace(s.Config.GoogleClientSecret)
		if id == "" || secret == "" {
			return oauth.ClientCredentials{}, false
		}
		return oauth.ClientCredentials{ClientID: id, ClientSecret: secret}, true
	default:
		return oauth.ClientCredentials{}, false
	}
}

func (s Server) providerClientSecret(provider string) string {
	creds, ok := s.providerCredentials(provider)
	if !ok {
		return ""
	}
	return creds.ClientSecret
}

func (s Server) providerConfigured(provider string) bool {
	_, ok := s.providerCredentials(provider)
	return ok
}

func (s Server) providerRedirectURI(provider string) string {
	return s.Config.ProviderRedirectURI(provider)
}

func (s Server) providerTokenURL(desc oauth.Provider) string {
	switch desc.ID {
	case oauth.ProviderGoogle:
		if strings.TrimSpace(s.GoogleTokenURL) != "" {
			return s.GoogleTokenURL
		}
	case oauth.ProviderGitHub:
		if strings.TrimSpace(s.GitHubTokenURL) != "" {
			return s.GitHubTokenURL
		}
	}
	return desc.TokenURL
}

func providerOrGoogle(provider string) string {
	provider = strings.TrimSpace(provider)
	if provider == "" {
		return oauth.ProviderGoogle
	}
	return provider
}

func splitProviderScopes(provider, raw string) []string {
	desc, ok := oauth.Lookup(provider)
	if !ok {
		return strings.Fields(raw)
	}
	return desc.SplitScopes(raw)
}

func tokenExpiry(now time.Time, expiresIn int64) time.Time {
	if expiresIn <= 0 {
		return time.Time{}
	}
	return now.Add(time.Duration(expiresIn) * time.Second)
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

func callbackSuccessPage(providerName, handoffURL string) string {
	safe := html.EscapeString(handoffURL)
	return "<!doctype html><html><body>" +
		"<p>Authorization complete. Return to shiet to finish connecting " + html.EscapeString(providerName) + ".</p>" +
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
	s.logInfo(codes.EventRateLimited, "surface", surface, "ip_bucket", ipBucket)
	writeJSON(w, http.StatusTooManyRequests, errorResponse{Error: codes.RateLimited})
	return true
}

func (s Server) rejectAuthDisabled(w http.ResponseWriter, surface string) bool {
	if !s.Config.AuthDisabled {
		return false
	}
	s.Metrics.IncKillSwitch(surface)
	s.logInfo(codes.EventKillSwitch, "surface", surface, "reason", codes.AuthDisabled)
	writeJSON(w, http.StatusForbidden, errorResponse{Error: codes.AuthDisabled})
	return true
}

func (s Server) rejectAppVersionDisabled(w http.ResponseWriter, surface, appVersion string) bool {
	if !s.Config.AppVersionDisabled(appVersion) {
		return false
	}
	s.Metrics.IncKillSwitch(surface + codes.KillSwitchVersionSuffix)
	s.logInfo(codes.EventKillSwitch, "surface", surface, "reason", codes.AppVersionDisabled, "app_version", appVersion)
	writeJSON(w, http.StatusForbidden, errorResponse{Error: codes.AppVersionDisabled})
	return true
}

func (s Server) logInfo(msg string, args ...any) {
	if s.Logger == nil {
		return
	}
	s.Logger.Info(msg, args...)
}
