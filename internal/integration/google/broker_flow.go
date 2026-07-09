package google

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
	"runtime"
	"strings"
	"sync"
	"time"

	"github.com/dylanbr0wn/shiet/internal/config"
	"github.com/dylanbr0wn/shiet/internal/integration/oauth"
	"github.com/dylanbr0wn/shiet/internal/integration/secrets"
	"github.com/dylanbr0wn/shiet/internal/service"
	"github.com/pkg/browser"
)

const (
	brokerHandoffPath   = "/oauth/handoff"
	brokerAuthTimeout   = 5 * time.Minute
	brokerShutdownGrace = 250 * time.Millisecond
)

// Sentinel errors for brokered connect failure modes. Callers can errors.Is
// these for actionable UI copy.
var (
	ErrHandoffReplay        = errors.New("Google OAuth handoff was already used")
	ErrHandoffExpired       = errors.New("Google OAuth handoff expired")
	ErrHandoffStateMismatch = errors.New("Google OAuth handoff state mismatch")
	ErrHandoffVerifier      = errors.New("Google OAuth handoff verifier mismatch")
	ErrBrokerRejected       = errors.New("Google OAuth broker rejected the request")
)

// BrowserOpener opens a URL in the system browser. Injectable for tests.
type BrowserOpener func(url string) error

// BrokerFlow runs the ADR-0001 secret-only broker connect: start auth at the
// broker, wait for a one-time handoff on a local loopback redirect, then
// exchange the handoff for Google token material.
type BrokerFlow struct {
	BaseURL    string
	HTTPClient *http.Client
	OpenURL    BrowserOpener
	AppVersion string
	Platform   string
}

type brokerStartResponse struct {
	AuthURL     string    `json:"auth_url"`
	BrokerState string    `json:"broker_state"`
	ExpiresAt   time.Time `json:"expires_at"`
}

type brokerHandoffResponse struct {
	Provider    string   `json:"provider"`
	AccountHint string   `json:"account_hint"`
	Scope       []string `json:"scope"`
	Token       struct {
		AccessToken  string    `json:"access_token"`
		RefreshToken string    `json:"refresh_token"`
		TokenType    string    `json:"token_type"`
		Expiry       time.Time `json:"expiry"`
	} `json:"token"`
}

type brokerErrorResponse struct {
	Error string `json:"error"`
}

type handoffCallback struct {
	BrokerState string
	HandoffCode string
}

// Authorize implements Authorizer for broker mode.
func (f *BrokerFlow) Authorize(ctx context.Context, accountID string) (oauth.Result, error) {
	accountID = strings.TrimSpace(accountID)
	if accountID == "" {
		return oauth.Result{}, errors.New("account_id is required")
	}
	base := strings.TrimRight(strings.TrimSpace(f.BaseURL), "/")
	if base == "" {
		return oauth.Result{}, fmt.Errorf("%w: set google.broker_base_url or SHIET_GOOGLE_BROKER_BASE_URL", config.ErrBrokerConfig)
	}

	ln, redirectURL, err := listenBrokerHandoff()
	if err != nil {
		return oauth.Result{}, err
	}
	defer ln.Close()

	sessionID, err := randomBrokerString(32)
	if err != nil {
		return oauth.Result{}, err
	}
	verifier, err := randomBrokerString(64)
	if err != nil {
		return oauth.Result{}, err
	}
	challenge := brokerPKCES256(verifier)

	codeCh := make(chan handoffCallback, 1)
	errCh := make(chan error, 1)
	var expectedState string
	var expectedMu sync.Mutex

	srv := &http.Server{
		Handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.URL.Path != brokerHandoffPath {
				http.NotFound(w, r)
				return
			}
			state := strings.TrimSpace(r.URL.Query().Get("broker_state"))
			code := strings.TrimSpace(r.URL.Query().Get("handoff_code"))
			if state == "" || code == "" {
				errCh <- errors.New("missing broker_state or handoff_code")
				http.Error(w, "missing handoff parameters", http.StatusBadRequest)
				return
			}
			expectedMu.Lock()
			wantState := expectedState
			expectedMu.Unlock()
			if wantState != "" && state != wantState {
				errCh <- fmt.Errorf("%w: broker_state does not match start response", ErrHandoffStateMismatch)
				http.Error(w, "state mismatch", http.StatusBadRequest)
				return
			}
			select {
			case codeCh <- handoffCallback{BrokerState: state, HandoffCode: code}:
			default:
			}
			w.Header().Set("Content-Type", "text/html; charset=utf-8")
			w.WriteHeader(http.StatusOK)
			_, _ = io.WriteString(w, "<!doctype html><html><body><p>Handoff received. You can close this window and return to shiet.</p></body></html>")
		}),
	}

	var serveWG sync.WaitGroup
	serveWG.Go(func() {
		if serveErr := srv.Serve(ln); serveErr != nil && !errors.Is(serveErr, http.ErrServerClosed) {
			errCh <- serveErr
		}
	})

	start, err := f.startAuth(ctx, base, sessionID, challenge, redirectURL)
	if err != nil {
		shutdownBrokerServer(srv, &serveWG)
		return oauth.Result{}, err
	}
	expectedMu.Lock()
	expectedState = start.BrokerState
	expectedMu.Unlock()

	open := f.OpenURL
	if open == nil {
		open = browser.OpenURL
	}
	if err := validateGoogleAuthURL(start.AuthURL); err != nil {
		shutdownBrokerServer(srv, &serveWG)
		return oauth.Result{}, fmt.Errorf("%w: %v", ErrBrokerRejected, err)
	}
	if err := open(start.AuthURL); err != nil {
		shutdownBrokerServer(srv, &serveWG)
		return oauth.Result{}, fmt.Errorf("%w: open browser: %v", ErrBrokerUnavailable, err)
	}

	waitCtx, cancel := context.WithTimeout(ctx, brokerAuthTimeout)
	defer cancel()

	var cb handoffCallback
	select {
	case <-waitCtx.Done():
		shutdownBrokerServer(srv, &serveWG)
		return oauth.Result{}, fmt.Errorf("%w: timed out waiting for broker handoff", ErrBrokerUnavailable)
	case err := <-errCh:
		shutdownBrokerServer(srv, &serveWG)
		return oauth.Result{}, err
	case cb = <-codeCh:
	}
	shutdownBrokerServer(srv, &serveWG)

	if cb.BrokerState != start.BrokerState {
		return oauth.Result{}, fmt.Errorf("%w: broker_state does not match start response", ErrHandoffStateMismatch)
	}

	handoff, err := f.exchangeHandoff(ctx, base, sessionID, cb.BrokerState, cb.HandoffCode, verifier)
	if err != nil {
		return oauth.Result{}, err
	}

	token := secrets.Token{
		AccessToken:  handoff.Token.AccessToken,
		RefreshToken: handoff.Token.RefreshToken,
		TokenType:    handoff.Token.TokenType,
		Expiry:       handoff.Token.Expiry,
	}
	scopes := handoff.Scope
	if len(scopes) == 0 {
		scopes = []string{scopeCalendarRead}
	}
	return oauth.Result{
		Provider:  service.ProviderGoogle,
		AccountID: accountID,
		Token:     token,
		Scopes:    scopes,
	}, nil
}

func (f *BrokerFlow) startAuth(ctx context.Context, base, sessionID, challenge, redirectURL string) (brokerStartResponse, error) {
	payload := map[string]string{
		"desktop_session_id":        sessionID,
		"handoff_challenge":         challenge,
		"app_version":               f.appVersion(),
		"platform":                  f.platform(),
		"desktop_handoff_redirect":  redirectURL,
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return brokerStartResponse{}, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, base+"/v1/google/oauth/start", strings.NewReader(string(body)))
	if err != nil {
		return brokerStartResponse{}, err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := f.httpClient().Do(req)
	if err != nil {
		return brokerStartResponse{}, fmt.Errorf("%w: contact broker start: %v", ErrBrokerUnavailable, err)
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return brokerStartResponse{}, mapBrokerHTTPError(resp.StatusCode, raw, "start")
	}
	var out brokerStartResponse
	if err := json.Unmarshal(raw, &out); err != nil {
		return brokerStartResponse{}, fmt.Errorf("%w: decode start response", ErrBrokerUnavailable)
	}
	if out.AuthURL == "" || out.BrokerState == "" {
		return brokerStartResponse{}, fmt.Errorf("%w: start response missing auth_url or broker_state", ErrBrokerUnavailable)
	}
	return out, nil
}

func (f *BrokerFlow) exchangeHandoff(ctx context.Context, base, sessionID, brokerState, handoffCode, verifier string) (brokerHandoffResponse, error) {
	payload := map[string]string{
		"desktop_session_id": sessionID,
		"broker_state":       brokerState,
		"handoff_code":       handoffCode,
		"handoff_verifier":   verifier,
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return brokerHandoffResponse{}, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, base+"/v1/google/oauth/handoff", strings.NewReader(string(body)))
	if err != nil {
		return brokerHandoffResponse{}, err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := f.httpClient().Do(req)
	if err != nil {
		return brokerHandoffResponse{}, fmt.Errorf("%w: contact broker handoff: %v", ErrBrokerUnavailable, err)
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return brokerHandoffResponse{}, mapBrokerHTTPError(resp.StatusCode, raw, "handoff")
	}
	var out brokerHandoffResponse
	if err := json.Unmarshal(raw, &out); err != nil {
		return brokerHandoffResponse{}, fmt.Errorf("%w: decode handoff response", ErrBrokerUnavailable)
	}
	if out.Token.AccessToken == "" {
		return brokerHandoffResponse{}, fmt.Errorf("%w: handoff response missing access_token", ErrBrokerUnavailable)
	}
	return out, nil
}

func (f *BrokerFlow) httpClient() *http.Client {
	if f.HTTPClient != nil {
		return f.HTTPClient
	}
	return http.DefaultClient
}

func (f *BrokerFlow) appVersion() string {
	if v := strings.TrimSpace(f.AppVersion); v != "" {
		return v
	}
	return "dev"
}

func (f *BrokerFlow) platform() string {
	if p := strings.TrimSpace(f.Platform); p != "" {
		return p
	}
	return runtime.GOOS + "-" + runtime.GOARCH
}

func mapBrokerHTTPError(status int, raw []byte, op string) error {
	var er brokerErrorResponse
	_ = json.Unmarshal(raw, &er)
	code := strings.TrimSpace(er.Error)
	switch code {
	case "handoff_already_used":
		return fmt.Errorf("%w: complete a fresh Google connect", ErrHandoffReplay)
	case "handoff_expired":
		return fmt.Errorf("%w: start a new Google connect", ErrHandoffExpired)
	case "handoff_state_mismatch":
		return fmt.Errorf("%w: start a new Google connect", ErrHandoffStateMismatch)
	case "handoff_verifier_mismatch":
		return fmt.Errorf("%w: start a new Google connect", ErrHandoffVerifier)
	case "handoff_not_found":
		return fmt.Errorf("%w: handoff not found; start a new Google connect", ErrBrokerRejected)
	}
	if status >= 500 || status == http.StatusNotImplemented || status == http.StatusServiceUnavailable {
		return fmt.Errorf("%w: broker %s returned %d", ErrBrokerUnavailable, op, status)
	}
	if code != "" {
		return fmt.Errorf("%w: broker %s error %s", ErrBrokerRejected, op, code)
	}
	return fmt.Errorf("%w: broker %s returned %d", ErrBrokerRejected, op, status)
}

func listenBrokerHandoff() (net.Listener, string, error) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return nil, "", fmt.Errorf("listen broker handoff loopback: %w", err)
	}
	_, port, err := net.SplitHostPort(ln.Addr().String())
	if err != nil {
		_ = ln.Close()
		return nil, "", err
	}
	redirectURL := fmt.Sprintf("http://127.0.0.1:%s%s", port, brokerHandoffPath)
	return ln, redirectURL, nil
}

func validateGoogleAuthURL(raw string) error {
	u, err := url.Parse(raw)
	if err != nil {
		return fmt.Errorf("invalid broker auth_url")
	}
	if u.Scheme != "https" {
		return fmt.Errorf("broker auth_url must use https")
	}
	if u.Host != "accounts.google.com" {
		return fmt.Errorf("broker auth_url host must be accounts.google.com")
	}
	if u.Path != "/o/oauth2/v2/auth" && u.Path != "/o/oauth2/auth" {
		return fmt.Errorf("broker auth_url path is not a Google OAuth authorize endpoint")
	}
	return nil
}

func shutdownBrokerServer(srv *http.Server, serveWG *sync.WaitGroup) {
	shutdownCtx, cancel := context.WithTimeout(context.Background(), brokerShutdownGrace)
	defer cancel()
	_ = srv.Shutdown(shutdownCtx)
	done := make(chan struct{})
	go func() {
		serveWG.Wait()
		close(done)
	}()
	select {
	case <-done:
	case <-shutdownCtx.Done():
	}
}

func randomBrokerString(n int) (string, error) {
	b := make([]byte, n)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(b), nil
}

func brokerPKCES256(verifier string) string {
	sum := sha256.Sum256([]byte(verifier))
	return base64.RawURLEncoding.EncodeToString(sum[:])
}
