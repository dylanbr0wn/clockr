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
	"io"
	"net"
	"net/http"
	"net/url"
	"strings"
	"time"

	brokerconfig "github.com/dylanbr0wn/shiet/internal/broker/config"
	"github.com/dylanbr0wn/shiet/internal/broker/store"
)

const googleAuthURL = "https://accounts.google.com/o/oauth2/v2/auth"

type Store interface {
	Ping(context.Context) error
	SaveOAuthState(context.Context, store.OAuthState) error
}

type Server struct {
	Config brokerconfig.Config
	Store  Store
	Clock  func() time.Time
}

type startRequest struct {
	DesktopSessionID string `json:"desktop_session_id"`
	HandoffChallenge string `json:"handoff_challenge"`
	AppVersion       string `json:"app_version"`
	Platform         string `json:"platform"`
}

type startResponse struct {
	AuthURL     string    `json:"auth_url"`
	BrokerState string    `json:"broker_state"`
	ExpiresAt   time.Time `json:"expires_at"`
}

type statusResponse struct {
	Status string `json:"status"`
}

type errorResponse struct {
	Error string `json:"error"`
}

func (s Server) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", s.health)
	mux.HandleFunc("GET /readyz", s.ready)
	mux.HandleFunc("POST /v1/google/oauth/start", s.startGoogleOAuth)
	mux.HandleFunc("GET /v1/google/oauth/callback", notImplemented("google_callback"))
	mux.HandleFunc("POST /v1/google/oauth/handoff", notImplemented("google_handoff"))
	mux.HandleFunc("POST /v1/google/oauth/refresh", notImplemented("google_refresh"))
	mux.HandleFunc("POST /v1/google/oauth/revoke", notImplemented("google_revoke"))
	return mux
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
	var req startRequest
	if err := decodeJSON(r.Body, &req); err != nil {
		writeJSON(w, http.StatusBadRequest, errorResponse{Error: "invalid_json"})
		return
	}
	req.DesktopSessionID = strings.TrimSpace(req.DesktopSessionID)
	req.HandoffChallenge = strings.TrimSpace(req.HandoffChallenge)
	if req.DesktopSessionID == "" || req.HandoffChallenge == "" {
		writeJSON(w, http.StatusBadRequest, errorResponse{Error: "desktop_session_id_and_handoff_challenge_required"})
		return
	}

	state, err := randomString(32)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, errorResponse{Error: "random_state_failed"})
		return
	}
	verifier, err := randomString(64)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, errorResponse{Error: "random_verifier_failed"})
		return
	}
	challenge := pkceS256(verifier)
	now := s.now()
	expiresAt := now.Add(s.Config.StateTTL)

	rec := store.OAuthState{
		ID:               state,
		DesktopSessionID: req.DesktopSessionID,
		PKCEVerifier:     verifier,
		PKCEChallenge:    challenge,
		HandoffChallenge: req.HandoffChallenge,
		Scopes:           append([]string(nil), s.Config.GoogleScopes...),
		AppVersion:       strings.TrimSpace(req.AppVersion),
		Platform:         strings.TrimSpace(req.Platform),
		SourceIPBucket:   sourceIPBucket(r.RemoteAddr),
		ExpiresAt:        expiresAt,
	}
	if err := s.Store.SaveOAuthState(r.Context(), rec); err != nil {
		writeJSON(w, http.StatusInternalServerError, errorResponse{Error: "state_persist_failed"})
		return
	}

	authURL, err := s.authURL(state, challenge)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, errorResponse{Error: "auth_url_failed"})
		return
	}
	writeJSON(w, http.StatusCreated, startResponse{
		AuthURL:     authURL,
		BrokerState: state,
		ExpiresAt:   expiresAt,
	})
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
