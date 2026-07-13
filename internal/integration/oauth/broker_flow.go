package oauth

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
	"runtime"
	"strings"
	"sync"
	"time"

	"connectrpc.com/connect"
	brokerv1 "github.com/dylanbr0wn/shiet/gen/shiet/broker/v1"
	"github.com/dylanbr0wn/shiet/gen/shiet/broker/v1/brokerv1connect"
	"github.com/dylanbr0wn/shiet/internal/broker/codes"
	"github.com/dylanbr0wn/shiet/internal/integration/secrets"
	"github.com/dylanbr0wn/shiet/internal/oauthpages"
	"github.com/pkg/browser"
	"google.golang.org/protobuf/types/known/timestamppb"
)

const (
	brokerHandoffPath   = "/oauth/handoff"
	brokerAuthTimeout   = 5 * time.Minute
	brokerShutdownGrace = 250 * time.Millisecond
)

var (
	ErrBrokerUnavailable    = errors.New("OAuth broker is unavailable")
	ErrBrokerRejected       = errors.New("OAuth broker rejected the request")
	ErrHandoffReplay        = errors.New("OAuth handoff was already used")
	ErrHandoffExpired       = errors.New("OAuth handoff expired")
	ErrHandoffStateMismatch = errors.New("OAuth handoff state mismatch")
	ErrHandoffVerifier      = errors.New("OAuth handoff verifier mismatch")
)

// BrokerFlow runs the provider-neutral desktop half of the secret-only OAuth
// broker protocol. Provider packages supply the expected authorization host
// and path and retain provider-specific refresh/revoke behavior.
type BrokerFlow struct {
	Provider string
	BaseURL  string
	// AuthURLHost and AuthURLPaths are retained for source compatibility;
	// authorization URL validation is centralized in the provider registry.
	AuthURLHost   string
	AuthURLPaths  []string
	DefaultScopes []string
	HTTPClient    *http.Client
	OpenURL       BrowserOpener
	AppVersion    string
	Platform      string
}

type handoffCallback struct {
	BrokerState string
	HandoffCode string
}

// Authorize implements the integration Authorizer contract.
func (f *BrokerFlow) Authorize(ctx context.Context, accountID string) (Result, error) {
	provider := strings.TrimSpace(f.Provider)
	if provider == "" {
		return Result{}, errors.New("provider is required")
	}
	desc, ok := Lookup(provider)
	if !ok {
		return Result{}, fmt.Errorf("unknown OAuth provider %q", provider)
	}
	base := strings.TrimRight(strings.TrimSpace(f.BaseURL), "/")
	if base == "" {
		return Result{}, errors.New("broker base URL is required")
	}

	ln, redirectURL, err := listenBrokerHandoff()
	if err != nil {
		return Result{}, err
	}
	defer ln.Close()
	sessionID, err := randomBrokerString(32)
	if err != nil {
		return Result{}, err
	}
	verifier, err := randomBrokerString(64)
	if err != nil {
		return Result{}, err
	}
	challenge := brokerPKCES256(verifier)

	codeCh := make(chan handoffCallback, 1)
	errCh := make(chan error, 1)
	var expectedState string
	var expectedMu sync.Mutex
	srv := &http.Server{Handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != brokerHandoffPath {
			http.NotFound(w, r)
			return
		}
		state := strings.TrimSpace(r.URL.Query().Get("broker_state"))
		code := strings.TrimSpace(r.URL.Query().Get("handoff_code"))
		if state == "" || code == "" {
			select {
			case errCh <- errors.New("missing broker_state or handoff_code"):
			default:
			}
			http.Error(w, "missing handoff parameters", http.StatusBadRequest)
			return
		}
		expectedMu.Lock()
		wantState := expectedState
		expectedMu.Unlock()
		if wantState != "" && state != wantState {
			select {
			case errCh <- fmt.Errorf("%w: broker_state does not match start response", ErrHandoffStateMismatch):
			default:
			}
			http.Error(w, "state mismatch", http.StatusBadRequest)
			return
		}
		select {
		case codeCh <- handoffCallback{BrokerState: state, HandoffCode: code}:
		default:
		}
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		page, err := oauthpages.Close("Handoff received. You can close this window and return to shiet.")
		if err != nil {
			page = "<!doctype html><html><body><p>Handoff received. You can close this window and return to shiet.</p></body></html>"
		}
		_, _ = io.WriteString(w, page)
	})}

	var serveWG sync.WaitGroup
	serveWG.Go(func() {
		if serveErr := srv.Serve(ln); serveErr != nil && !errors.Is(serveErr, http.ErrServerClosed) {
			select {
			case errCh <- serveErr:
			default:
			}
		}
	})
	start, err := f.startAuth(ctx, base, sessionID, challenge, redirectURL)
	if err != nil {
		shutdownBrokerServer(srv, &serveWG)
		return Result{}, err
	}
	expectedMu.Lock()
	expectedState = start.BrokerState
	expectedMu.Unlock()
	if err := desc.ValidateAuthorizationURL(start.AuthUrl); err != nil {
		shutdownBrokerServer(srv, &serveWG)
		return Result{}, fmt.Errorf("%w: %v", ErrBrokerRejected, err)
	}
	open := f.OpenURL
	if open == nil {
		open = browser.OpenURL
	}
	if err := open(start.AuthUrl); err != nil {
		shutdownBrokerServer(srv, &serveWG)
		return Result{}, fmt.Errorf("%w: open browser: %v", ErrBrokerUnavailable, err)
	}

	waitCtx, cancel := context.WithTimeout(ctx, brokerAuthTimeout)
	defer cancel()
	var cb handoffCallback
	select {
	case <-waitCtx.Done():
		shutdownBrokerServer(srv, &serveWG)
		return Result{}, fmt.Errorf("%w: timed out waiting for broker handoff", ErrBrokerUnavailable)
	case err := <-errCh:
		shutdownBrokerServer(srv, &serveWG)
		return Result{}, err
	case cb = <-codeCh:
	}
	shutdownBrokerServer(srv, &serveWG)

	handoff, err := f.exchangeHandoff(ctx, base, sessionID, cb.BrokerState, cb.HandoffCode, verifier)
	if err != nil {
		return Result{}, err
	}
	if handoff.Provider != brokerv1.Provider_PROVIDER_UNSPECIFIED && handoff.Provider != brokerProvider(provider) {
		return Result{}, fmt.Errorf("%w: handoff provider mismatch", ErrBrokerRejected)
	}
	scopes := handoff.Scopes
	if len(scopes) == 0 {
		scopes = append([]string(nil), f.DefaultScopes...)
	}
	if len(scopes) == 0 {
		scopes = append([]string(nil), desc.DefaultScopes...)
	}
	return Result{
		Provider:  provider,
		AccountID: strings.TrimSpace(accountID),
		Token: secrets.Token{
			AccessToken:  handoff.Token.AccessToken,
			RefreshToken: handoff.Token.RefreshToken,
			TokenType:    handoff.Token.TokenType,
			Expiry:       timestampTime(handoff.Token.Expiry),
		},
		Scopes: scopes,
	}, nil
}

func (f *BrokerFlow) startAuth(ctx context.Context, base, sessionID, challenge, redirectURL string) (*brokerv1.StartAuthorizationResponse, error) {
	request := &brokerv1.StartAuthorizationRequest{
		Provider:               brokerProvider(f.Provider),
		DesktopSessionId:       sessionID,
		HandoffChallenge:       challenge,
		DesktopHandoffRedirect: redirectURL,
		Application:            &brokerv1.ApplicationMetadata{AppVersion: f.appVersion(), Platform: f.platform()},
	}
	response, err := f.brokerClient(base).StartAuthorization(ctx, connect.NewRequest(request))
	if err != nil {
		return nil, mapBrokerRPCError(err, "start")
	}
	if response.Msg.AuthUrl == "" || response.Msg.BrokerState == "" {
		return nil, fmt.Errorf("%w: start response missing auth_url or broker_state", ErrBrokerUnavailable)
	}
	return response.Msg, nil
}

func (f *BrokerFlow) exchangeHandoff(ctx context.Context, base, sessionID, state, code, verifier string) (*brokerv1.ExchangeHandoffResponse, error) {
	request := &brokerv1.ExchangeHandoffRequest{
		Provider:         brokerProvider(f.Provider),
		DesktopSessionId: sessionID,
		BrokerState:      state,
		HandoffCode:      code,
		HandoffVerifier:  verifier,
		Application:      &brokerv1.ApplicationMetadata{AppVersion: f.appVersion(), Platform: f.platform()},
	}
	response, err := f.brokerClient(base).ExchangeHandoff(ctx, connect.NewRequest(request))
	if err != nil {
		return nil, mapBrokerRPCError(err, "handoff")
	}
	if response.Msg.Token == nil || strings.TrimSpace(response.Msg.Token.AccessToken) == "" {
		return nil, fmt.Errorf("%w: handoff response missing access_token", ErrBrokerUnavailable)
	}
	return response.Msg, nil
}

func (f *BrokerFlow) brokerClient(base string) brokerv1connect.OAuthBrokerServiceClient {
	client := f.HTTPClient
	if client == nil {
		client = http.DefaultClient
	}
	return brokerv1connect.NewOAuthBrokerServiceClient(client, base)
}

func mapBrokerRPCError(err error, op string) error {
	code := BrokerErrorCode(err)
	switch code {
	case codes.HandoffAlreadyUsed:
		return ErrHandoffReplay
	case codes.HandoffExpired:
		return ErrHandoffExpired
	case codes.HandoffStateMismatch:
		return ErrHandoffStateMismatch
	case codes.HandoffVerifierMismatch:
		return ErrHandoffVerifier
	case codes.RateLimited, codes.AuthDisabled, codes.AppVersionDisabled:
		return fmt.Errorf("%w: %s", ErrBrokerRejected, code)
	}
	if connect.CodeOf(err) == connect.CodeUnavailable || connect.CodeOf(err) == connect.CodeInternal {
		return fmt.Errorf("%w: broker %s unavailable", ErrBrokerUnavailable, op)
	}
	return fmt.Errorf("%w: broker %s rejected request: %s", ErrBrokerRejected, op, code)
}

func BrokerErrorCode(err error) string {
	var connectErr *connect.Error
	if !errors.As(err, &connectErr) {
		return ""
	}
	for _, detail := range connectErr.Details() {
		value, valueErr := detail.Value()
		if valueErr != nil {
			continue
		}
		if brokerDetail, ok := value.(*brokerv1.BrokerErrorDetail); ok {
			return strings.TrimSpace(brokerDetail.Code)
		}
	}
	return strings.TrimSpace(connectErr.Message())
}

func brokerProvider(provider string) brokerv1.Provider {
	switch strings.ToLower(strings.TrimSpace(provider)) {
	case ProviderGitHub:
		return brokerv1.Provider_PROVIDER_GITHUB
	case ProviderSlack:
		return brokerv1.Provider_PROVIDER_SLACK
	case ProviderBitbucket:
		return brokerv1.Provider_PROVIDER_BITBUCKET
	default:
		return brokerv1.Provider_PROVIDER_GOOGLE
	}
}

func timestampTime(timestamp *timestamppb.Timestamp) time.Time {
	if timestamp == nil {
		return time.Time{}
	}
	return timestamp.AsTime()
}

func (f *BrokerFlow) appVersion() string {
	if value := strings.TrimSpace(f.AppVersion); value != "" {
		return value
	}
	return "dev"
}

func (f *BrokerFlow) platform() string {
	if value := strings.TrimSpace(f.Platform); value != "" {
		return value
	}
	return runtime.GOOS + "-" + runtime.GOARCH
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
	return ln, fmt.Sprintf("http://127.0.0.1:%s%s", port, brokerHandoffPath), nil
}

func shutdownBrokerServer(srv *http.Server, wg *sync.WaitGroup) {
	ctx, cancel := context.WithTimeout(context.Background(), brokerShutdownGrace)
	defer cancel()
	_ = srv.Shutdown(ctx)
	done := make(chan struct{})
	go func() { wg.Wait(); close(done) }()
	select {
	case <-done:
	case <-ctx.Done():
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
