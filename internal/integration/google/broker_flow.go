package google

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
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

	"connectrpc.com/connect"
	brokerv1 "github.com/dylanbr0wn/shiet/gen/shiet/broker/v1"
	"github.com/dylanbr0wn/shiet/gen/shiet/broker/v1/brokerv1connect"
	"github.com/dylanbr0wn/shiet/internal/broker/codes"
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
	ErrInvalidRefreshToken  = errors.New("Google OAuth refresh token is invalid")
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
	if err := validateGoogleAuthURL(start.AuthUrl); err != nil {
		shutdownBrokerServer(srv, &serveWG)
		return oauth.Result{}, fmt.Errorf("%w: %v", ErrBrokerRejected, err)
	}
	if err := open(start.AuthUrl); err != nil {
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

	if handoff.Provider != brokerv1.Provider_PROVIDER_GOOGLE || handoff.Token == nil {
		return oauth.Result{}, fmt.Errorf("%w: handoff provider mismatch", ErrBrokerRejected)
	}
	token := secrets.Token{
		AccessToken:  handoff.Token.AccessToken,
		RefreshToken: handoff.Token.RefreshToken,
		TokenType:    handoff.Token.TokenType,
		Expiry:       handoff.Token.Expiry.AsTime(),
	}
	scopes := handoff.Scopes
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

func (f *BrokerFlow) startAuth(ctx context.Context, base, sessionID, challenge, redirectURL string) (*brokerv1.StartAuthorizationResponse, error) {
	request := &brokerv1.StartAuthorizationRequest{
		Provider:               brokerv1.Provider_PROVIDER_GOOGLE,
		DesktopSessionId:       sessionID,
		HandoffChallenge:       challenge,
		DesktopHandoffRedirect: redirectURL,
		Application:            &brokerv1.ApplicationMetadata{AppVersion: f.appVersion(), Platform: f.platform()},
	}
	response, err := f.brokerClient(base).StartAuthorization(ctx, connect.NewRequest(request))
	if err != nil {
		return nil, f.mapBrokerRPCError(err, "start")
	}
	if response.Msg.AuthUrl == "" || response.Msg.BrokerState == "" {
		return nil, fmt.Errorf("%w: start response missing auth_url or broker_state", ErrBrokerUnavailable)
	}
	return response.Msg, nil
}

func (f *BrokerFlow) exchangeHandoff(ctx context.Context, base, sessionID, brokerState, handoffCode, verifier string) (*brokerv1.ExchangeHandoffResponse, error) {
	request := &brokerv1.ExchangeHandoffRequest{
		Provider:         brokerv1.Provider_PROVIDER_GOOGLE,
		DesktopSessionId: sessionID,
		BrokerState:      brokerState,
		HandoffCode:      handoffCode,
		HandoffVerifier:  verifier,
		Application:      &brokerv1.ApplicationMetadata{AppVersion: f.appVersion(), Platform: f.platform()},
	}
	response, err := f.brokerClient(base).ExchangeHandoff(ctx, connect.NewRequest(request))
	if err != nil {
		return nil, f.mapBrokerRPCError(err, "handoff")
	}
	if response.Msg.Token == nil || response.Msg.Token.AccessToken == "" {
		return nil, fmt.Errorf("%w: handoff response missing access_token", ErrBrokerUnavailable)
	}
	return response.Msg, nil
}

// RefreshToken asks the broker to exchange a Google refresh token for new
// access-token material using the server-side client secret. Tokens are not
// persisted by the broker; the caller must write the result to the keychain.
func (f *BrokerFlow) RefreshToken(ctx context.Context, refreshToken string, scopes []string) (secrets.Token, error) {
	base := strings.TrimRight(strings.TrimSpace(f.BaseURL), "/")
	if base == "" {
		return secrets.Token{}, fmt.Errorf("%w: set google.broker_base_url or SHIET_GOOGLE_BROKER_BASE_URL", config.ErrBrokerConfig)
	}
	refreshToken = strings.TrimSpace(refreshToken)
	if refreshToken == "" {
		return secrets.Token{}, fmt.Errorf("%w: refresh token is empty", ErrInvalidRefreshToken)
	}

	request := &brokerv1.RefreshTokenRequest{
		Provider:     brokerv1.Provider_PROVIDER_GOOGLE,
		RefreshToken: refreshToken,
		Scopes:       append([]string(nil), scopes...),
		Application:  &brokerv1.ApplicationMetadata{AppVersion: f.appVersion(), Platform: f.platform()},
	}
	response, err := f.brokerClient(base).RefreshToken(ctx, connect.NewRequest(request))
	if err != nil {
		return secrets.Token{}, f.mapBrokerRPCError(err, "refresh")
	}
	out := response.Msg.Token
	if out == nil || strings.TrimSpace(out.AccessToken) == "" {
		return secrets.Token{}, fmt.Errorf("%w: refresh response missing access_token", ErrBrokerUnavailable)
	}
	tokenType := strings.TrimSpace(out.TokenType)
	if tokenType == "" {
		tokenType = "Bearer"
	}
	nextRefresh := strings.TrimSpace(out.RefreshToken)
	if nextRefresh == "" {
		nextRefresh = refreshToken
	}
	return secrets.Token{
		AccessToken:  out.AccessToken,
		RefreshToken: nextRefresh,
		TokenType:    tokenType,
		Expiry:       out.Expiry.AsTime(),
	}, nil
}

// Revoke asks the broker to revoke a Google refresh token. The broker does not
// retain the token; callers still delete local keychain material.
func (f *BrokerFlow) Revoke(ctx context.Context, refreshToken string) error {
	refreshToken = strings.TrimSpace(refreshToken)
	if refreshToken == "" {
		return errors.New("refresh_token is required")
	}
	base := strings.TrimRight(strings.TrimSpace(f.BaseURL), "/")
	if base == "" {
		return fmt.Errorf("%w: set google.broker_base_url or SHIET_GOOGLE_BROKER_BASE_URL", config.ErrBrokerConfig)
	}

	request := &brokerv1.RevokeTokenRequest{
		Provider:   brokerv1.Provider_PROVIDER_GOOGLE,
		Credential: &brokerv1.RevokeTokenRequest_RefreshToken{RefreshToken: refreshToken},
		Reason:     "user_disconnect",
	}
	response, err := f.brokerClient(base).RevokeToken(ctx, connect.NewRequest(request))
	if err != nil {
		return f.mapBrokerRPCError(err, "revoke")
	}
	if !response.Msg.Revoked {
		return fmt.Errorf("%w: revoke response missing revoked=true", ErrBrokerRejected)
	}
	return nil
}

func (f *BrokerFlow) httpClient() *http.Client {
	if f.HTTPClient != nil {
		return f.HTTPClient
	}
	return http.DefaultClient
}

func (f *BrokerFlow) brokerClient(base string) brokerv1connect.OAuthBrokerServiceClient {
	return brokerv1connect.NewOAuthBrokerServiceClient(f.httpClient(), base)
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

func (f *BrokerFlow) mapBrokerRPCError(err error, op string) error {
	code := oauth.BrokerErrorCode(err)
	switch code {
	case codes.HandoffAlreadyUsed:
		return fmt.Errorf("%w: complete a fresh Google connect", ErrHandoffReplay)
	case codes.HandoffExpired:
		return fmt.Errorf("%w: start a new Google connect", ErrHandoffExpired)
	case codes.HandoffStateMismatch:
		return fmt.Errorf("%w: start a new Google connect", ErrHandoffStateMismatch)
	case codes.HandoffVerifierMismatch:
		return fmt.Errorf("%w: start a new Google connect", ErrHandoffVerifier)
	case codes.HandoffNotFound:
		return fmt.Errorf("%w: handoff not found; start a new Google connect", ErrBrokerRejected)
	case codes.InvalidRefreshToken:
		return fmt.Errorf("%w: reconnect Google Calendar", ErrInvalidRefreshToken)
	case codes.RateLimited:
		return fmt.Errorf("%w: too many requests; try again later", ErrBrokerRejected)
	case codes.AuthDisabled:
		return fmt.Errorf("%w: Google connect is temporarily unavailable", ErrBrokerRejected)
	case codes.RefreshDisabled:
		return fmt.Errorf("%w: Google token refresh is temporarily unavailable", ErrBrokerRejected)
	case codes.AppVersionDisabled:
		return fmt.Errorf("%w: this app version can no longer use broker auth; update shiet", ErrBrokerRejected)
	}
	if connect.CodeOf(err) == connect.CodeResourceExhausted {
		return fmt.Errorf("%w: too many requests; try again later", ErrBrokerRejected)
	}
	if connect.CodeOf(err) == connect.CodeUnavailable || connect.CodeOf(err) == connect.CodeInternal || connect.CodeOf(err) == connect.CodeUnimplemented {
		return fmt.Errorf("%w: broker %s unavailable", ErrBrokerUnavailable, op)
	}
	if code != "" {
		return fmt.Errorf("%w: broker %s error %s", ErrBrokerRejected, op, code)
	}
	return fmt.Errorf("%w: broker %s rejected request", ErrBrokerRejected, op)
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
