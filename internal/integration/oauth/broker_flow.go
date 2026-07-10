package oauth

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
	"runtime"
	"strings"
	"sync"
	"time"

	"github.com/dylanbr0wn/shiet/internal/broker/codes"
	"github.com/dylanbr0wn/shiet/internal/integration/secrets"
	"github.com/pkg/browser"
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
// broker protocol. Provider packages supply the provider id and retain
// provider-specific refresh/revoke behavior.
type BrokerFlow struct {
	Provider      string
	BaseURL       string
	DefaultScopes []string
	HTTPClient    *http.Client
	OpenURL       BrowserOpener
	AppVersion    string
	Platform      string
}

// Authorize implements the integration Authorizer contract.
func (f *BrokerFlow) Authorize(ctx context.Context, accountID string) (Result, error) {
	providerID := strings.TrimSpace(f.Provider)
	if providerID == "" {
		return Result{}, errors.New("provider is required")
	}
	desc, ok := Lookup(providerID)
	if !ok {
		return Result{}, fmt.Errorf("unknown OAuth provider %q", providerID)
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
		_, _ = io.WriteString(w, "<!doctype html><html><body><p>Handoff received. You can close this window and return to shiet.</p></body></html>")
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
	if err := desc.ValidateAuthorizationURL(start.AuthURL); err != nil {
		shutdownBrokerServer(srv, &serveWG)
		return Result{}, fmt.Errorf("%w: %v", ErrBrokerRejected, err)
	}
	open := f.OpenURL
	if open == nil {
		open = browser.OpenURL
	}
	if err := open(start.AuthURL); err != nil {
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
	if handoff.Provider != "" && handoff.Provider != providerID {
		return Result{}, fmt.Errorf("%w: handoff provider mismatch", ErrBrokerRejected)
	}
	scopes := handoff.Scope
	if len(scopes) == 0 {
		scopes = append([]string(nil), f.DefaultScopes...)
	}
	if len(scopes) == 0 {
		scopes = append([]string(nil), desc.DefaultScopes...)
	}
	return Result{
		Provider:  providerID,
		AccountID: strings.TrimSpace(accountID),
		Token: secrets.Token{
			AccessToken:  handoff.Token.AccessToken,
			RefreshToken: handoff.Token.RefreshToken,
			TokenType:    handoff.Token.TokenType,
			Expiry:       handoff.Token.Expiry,
		},
		Scopes: scopes,
	}, nil
}

func (f *BrokerFlow) startAuth(ctx context.Context, base, sessionID, challenge, redirectURL string) (BrokerStartResponse, error) {
	body, _ := json.Marshal(BrokerStartRequest{
		DesktopSessionID:       sessionID,
		HandoffChallenge:       challenge,
		AppVersion:             f.appVersion(),
		Platform:               f.platform(),
		DesktopHandoffRedirect: redirectURL,
	})
	var out BrokerStartResponse
	if err := f.postJSON(ctx, base+"/v1/"+f.Provider+"/oauth/start", body, &out, "start"); err != nil {
		return BrokerStartResponse{}, err
	}
	if out.AuthURL == "" || out.BrokerState == "" {
		return BrokerStartResponse{}, fmt.Errorf("%w: start response missing auth_url or broker_state", ErrBrokerUnavailable)
	}
	return out, nil
}

func (f *BrokerFlow) exchangeHandoff(ctx context.Context, base, sessionID, state, code, verifier string) (BrokerHandoffResponse, error) {
	body, _ := json.Marshal(BrokerHandoffRequest{
		DesktopSessionID: sessionID,
		BrokerState:      state,
		HandoffCode:      code,
		HandoffVerifier:  verifier,
	})
	var out BrokerHandoffResponse
	if err := f.postJSON(ctx, base+"/v1/"+f.Provider+"/oauth/handoff", body, &out, "handoff"); err != nil {
		return BrokerHandoffResponse{}, err
	}
	if strings.TrimSpace(out.Token.AccessToken) == "" {
		return BrokerHandoffResponse{}, fmt.Errorf("%w: handoff response missing access_token", ErrBrokerUnavailable)
	}
	return out, nil
}

func (f *BrokerFlow) postJSON(ctx context.Context, endpoint string, body []byte, out any, op string) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, strings.NewReader(string(body)))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	client := f.HTTPClient
	if client == nil {
		client = http.DefaultClient
	}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("%w: contact broker %s: %v", ErrBrokerUnavailable, op, err)
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return mapBrokerHTTPError(resp.StatusCode, raw, op)
	}
	if err := json.Unmarshal(raw, out); err != nil {
		return fmt.Errorf("%w: decode %s response", ErrBrokerUnavailable, op)
	}
	return nil
}

func mapBrokerHTTPError(status int, raw []byte, op string) error {
	var er BrokerErrorResponse
	_ = json.Unmarshal(raw, &er)
	switch strings.TrimSpace(er.Error) {
	case codes.HandoffAlreadyUsed:
		return ErrHandoffReplay
	case codes.HandoffExpired:
		return ErrHandoffExpired
	case codes.HandoffStateMismatch:
		return ErrHandoffStateMismatch
	case codes.HandoffVerifierMismatch:
		return ErrHandoffVerifier
	case codes.RateLimited, codes.AuthDisabled, codes.AppVersionDisabled:
		return fmt.Errorf("%w: %s", ErrBrokerRejected, er.Error)
	}
	if status >= 500 {
		return fmt.Errorf("%w: broker %s returned %d", ErrBrokerUnavailable, op, status)
	}
	return fmt.Errorf("%w: broker %s returned %d", ErrBrokerRejected, op, status)
}

type handoffCallback struct {
	BrokerState string
	HandoffCode string
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
