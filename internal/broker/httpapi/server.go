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

	"github.com/dylanbr0wn/shiet/gen/shiet/broker/v1/brokerv1connect"
	"github.com/dylanbr0wn/shiet/internal/broker/codes"
	brokerconfig "github.com/dylanbr0wn/shiet/internal/broker/config"
	"github.com/dylanbr0wn/shiet/internal/broker/observe"
	"github.com/dylanbr0wn/shiet/internal/broker/ratelimit"
	"github.com/dylanbr0wn/shiet/internal/broker/store"
)

const (
	googleAuthURL       = "https://accounts.google.com/o/oauth2/v2/auth"
	githubAuthURL       = "https://github.com/login/oauth/authorize"
	defaultGoogleToken  = "https://oauth2.googleapis.com/token"
	defaultGoogleRevoke = "https://oauth2.googleapis.com/revoke"
	defaultGitHubToken  = "https://github.com/login/oauth/access_token"
	defaultGitHubRevoke = "https://api.github.com"

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

type providerTokenResponse struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
	TokenType    string `json:"token_type"`
	ExpiresIn    int64  `json:"expires_in"`
	Scope        string `json:"scope"`
	Error        string `json:"error"`
	ErrorDesc    string `json:"error_description"`
}

type statusResponse struct {
	Status string `json:"status"`
}

type errorResponse struct {
	Error string `json:"error"`
}

func (s Server) Handler() http.Handler {
	mux := http.NewServeMux()
	connectPath, connectHandler := brokerv1connect.NewOAuthBrokerServiceHandler(connectBrokerService{service: s.service()})
	mux.Handle(connectPath, connectHandler)
	mux.HandleFunc("GET /healthz", s.health)
	mux.HandleFunc("GET /readyz", s.ready)
	mux.HandleFunc("GET /metrics", s.metrics)
	mux.HandleFunc("GET /v1/google/oauth/callback", s.googleCallback)
	mux.HandleFunc("GET /v1/github/oauth/callback", s.githubCallback)
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
	writeOperationalJSON(w, http.StatusOK, statusResponse{Status: "ok"})
}

func (s Server) ready(w http.ResponseWriter, r *http.Request) {
	if err := s.Config.Validate(); err != nil {
		writeOperationalJSON(w, http.StatusServiceUnavailable, errorResponse{Error: codes.InvalidConfig})
		return
	}
	if s.Store == nil {
		writeOperationalJSON(w, http.StatusServiceUnavailable, errorResponse{Error: codes.DatastoreUnavailable})
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), time.Second)
	defer cancel()
	if err := s.Store.Ping(ctx); err != nil {
		writeOperationalJSON(w, http.StatusServiceUnavailable, errorResponse{Error: codes.DatastoreUnavailable})
		return
	}
	writeOperationalJSON(w, http.StatusOK, statusResponse{Status: "ready"})
}

func (s Server) googleCallback(w http.ResponseWriter, r *http.Request) {
	s.oauthCallback(w, r, "google")
}

func (s Server) githubCallback(w http.ResponseWriter, r *http.Request) {
	s.oauthCallback(w, r, "github")
}

func (s Server) oauthCallback(w http.ResponseWriter, r *http.Request, provider string) {
	providerName := "Google"
	if provider == "github" {
		providerName = "GitHub"
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
		outcome := codes.OutcomeGoogleError
		if provider != "google" {
			outcome = codes.OutcomeProviderError
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

func (s Server) revokeGitHubToken(ctx context.Context, accessToken string) error {
	base := strings.TrimRight(strings.TrimSpace(s.GitHubRevokeURL), "/")
	if base == "" {
		base = defaultGitHubRevoke
	}
	body, err := json.Marshal(map[string]string{"access_token": accessToken})
	if err != nil {
		return err
	}
	revokeURL := base + "/applications/" + url.PathEscape(s.Config.GitHubClientID) + "/token"
	req, err := http.NewRequestWithContext(ctx, http.MethodDelete, revokeURL, strings.NewReader(string(body)))
	if err != nil {
		return err
	}
	req.SetBasicAuth(s.Config.GitHubClientID, s.Config.GitHubClientSecret)
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

func (s Server) exchangeGoogleCode(ctx context.Context, code, pkceVerifier string) (providerTokenResponse, error) {
	form := url.Values{}
	form.Set("grant_type", "authorization_code")
	form.Set("code", code)
	form.Set("client_id", s.Config.GoogleClientID)
	form.Set("client_secret", s.Config.GoogleClientSecret)
	form.Set("redirect_uri", s.Config.RedirectURI())
	form.Set("code_verifier", pkceVerifier)

	tok, err := s.postGoogleToken(ctx, form)
	if err != nil {
		return providerTokenResponse{}, fmt.Errorf("google token exchange failed")
	}
	return tok, nil
}

func (s Server) exchangeProviderCode(ctx context.Context, provider, code, pkceVerifier string) (providerTokenResponse, error) {
	if provider == "github" {
		return s.exchangeGitHubCode(ctx, code, pkceVerifier)
	}
	return s.exchangeGoogleCode(ctx, code, pkceVerifier)
}

func (s Server) exchangeGitHubCode(ctx context.Context, code, pkceVerifier string) (providerTokenResponse, error) {
	form := url.Values{}
	form.Set("code", code)
	form.Set("client_id", s.Config.GitHubClientID)
	form.Set("client_secret", s.Config.GitHubClientSecret)
	form.Set("redirect_uri", s.Config.GitHubRedirectURI())
	form.Set("code_verifier", pkceVerifier)

	tokenURL := s.GitHubTokenURL
	if tokenURL == "" {
		tokenURL = defaultGitHubToken
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, tokenURL, strings.NewReader(form.Encode()))
	if err != nil {
		return providerTokenResponse{}, err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Accept", "application/json")
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
		return providerTokenResponse{}, fmt.Errorf("github token exchange failed")
	}
	if tok.TokenType == "" {
		tok.TokenType = "Bearer"
	}
	return tok, nil
}

func (s Server) postGoogleToken(ctx context.Context, form url.Values) (providerTokenResponse, error) {
	tokenURL := s.GoogleTokenURL
	if tokenURL == "" {
		tokenURL = defaultGoogleToken
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
		return providerTokenResponse{}, &googleTokenError{Code: code, Desc: tok.ErrorDesc}
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
		if providerOrGoogle(state.Provider) == "github" {
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
	redirectURI := s.Config.RedirectURI()
	providerAuthURL := googleAuthURL
	clientID := s.Config.GoogleClientID
	if provider == "github" {
		redirectURI = s.Config.GitHubRedirectURI()
		providerAuthURL = githubAuthURL
		clientID = s.Config.GitHubClientID
	}
	if redirectURI == "" {
		return "", errors.New("missing redirect uri")
	}
	u, err := url.Parse(providerAuthURL)
	if err != nil {
		return "", err
	}
	q := u.Query()
	q.Set("client_id", clientID)
	q.Set("redirect_uri", redirectURI)
	q.Set("response_type", "code")
	q.Set("scope", strings.Join(s.providerScopes(provider), " "))
	q.Set("state", state)
	if provider == "google" {
		q.Set("access_type", "offline")
		q.Set("prompt", "consent")
	}
	q.Set("code_challenge", codeChallenge)
	q.Set("code_challenge_method", "S256")
	u.RawQuery = q.Encode()
	return u.String(), nil
}

func (s Server) providerScopes(provider string) []string {
	if provider == "github" {
		return s.Config.GitHubScopes
	}
	return s.Config.GoogleScopes
}

func (s Server) providerClientSecret(provider string) string {
	if provider == "github" {
		return s.Config.GitHubClientSecret
	}
	return s.Config.GoogleClientSecret
}

func (s Server) providerConfigured(provider string) bool {
	if provider == "github" {
		return strings.TrimSpace(s.Config.GitHubClientID) != "" && strings.TrimSpace(s.Config.GitHubClientSecret) != ""
	}
	return strings.TrimSpace(s.Config.GoogleClientID) != "" && strings.TrimSpace(s.Config.GoogleClientSecret) != ""
}

func providerOrGoogle(provider string) string {
	provider = strings.TrimSpace(provider)
	if provider == "" {
		return "google"
	}
	return provider
}

func splitProviderScopes(provider, raw string) []string {
	if provider == "github" {
		return strings.FieldsFunc(raw, func(r rune) bool { return r == ',' || r == ' ' })
	}
	return strings.Fields(raw)
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

func writeOperationalJSON(w http.ResponseWriter, status int, payload any) {
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

func (s Server) logInfo(msg string, args ...any) {
	if s.Logger == nil {
		return
	}
	s.Logger.Info(msg, args...)
}
