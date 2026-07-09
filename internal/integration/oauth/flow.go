package oauth

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"errors"
	"fmt"
	"html"
	"io"
	"log"
	"net"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/dylanbr0wn/shiet/internal/integration/secrets"
	"github.com/pkg/browser"
	"golang.org/x/oauth2"
)

const (
	callbackPath    = "/oauth/callback"
	loopbackHost    = "127.0.0.1"
	authWaitTimeout = 5 * time.Minute
	shutdownGrace   = 250 * time.Millisecond
)

// BrowserOpener opens a URL in the system browser. Injectable for tests.
type BrowserOpener func(url string) error

// Flow runs desktop OAuth with PKCE and a loopback redirect.
type Flow struct {
	Config  ProviderConfig
	Store   secrets.TokenStore
	OpenURL BrowserOpener
}

// Result is returned after a successful authorization.
type Result struct {
	Provider  string
	AccountID string
	Token     secrets.Token
	Scopes    []string
}

type callbackResult struct {
	status  int
	message string
}

// Authorize opens the system browser, waits for the loopback redirect, exchanges
// the code for tokens, and persists them in the token store.
func (f *Flow) Authorize(ctx context.Context, accountID string) (Result, error) {
	if f.Store == nil {
		return Result{}, errors.New("token store is required")
	}
	accountID = strings.TrimSpace(accountID)
	if accountID == "" {
		return Result{}, errors.New("account_id is required")
	}

	ln, redirectURL, err := listenLoopback()
	if err != nil {
		return Result{}, err
	}
	defer ln.Close()

	state, err := randomString(32)
	if err != nil {
		return Result{}, err
	}
	verifier, err := randomString(64)
	if err != nil {
		return Result{}, err
	}

	oauthCfg := f.Config.OAuth2Config(redirectURL)
	authURL := oauthCfg.AuthCodeURL(
		state,
		oauth2.AccessTypeOffline,
		oauth2.ApprovalForce,
		oauth2.S256ChallengeOption(verifier),
	)

	codeCh := make(chan string, 1)
	errCh := make(chan error, 1)
	resultCh := make(chan callbackResult, 1)

	srv := &http.Server{
		Handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.URL.Path != callbackPath {
				http.NotFound(w, r)
				return
			}
			if r.URL.Query().Get("state") != state {
				errCh <- errors.New("oauth state mismatch")
				http.Error(w, "state mismatch", http.StatusBadRequest)
				return
			}
			if errMsg := r.URL.Query().Get("error"); errMsg != "" {
				desc := r.URL.Query().Get("error_description")
				errCh <- fmt.Errorf("oauth error: %s: %s", errMsg, desc)
				http.Error(w, errMsg, http.StatusBadRequest)
				return
			}
			code := r.URL.Query().Get("code")
			if code == "" {
				errCh <- errors.New("missing oauth code")
				http.Error(w, "missing code", http.StatusBadRequest)
				return
			}
			codeCh <- code

			select {
			case result := <-resultCh:
				w.Header().Set("Content-Type", "text/html; charset=utf-8")
				w.WriteHeader(result.status)
				_, _ = io.WriteString(w, callbackPage(result.message))
			case <-r.Context().Done():
			}
		}),
	}

	var serveWG sync.WaitGroup
	serveWG.Go(func() {
		if serveErr := srv.Serve(ln); serveErr != nil && !errors.Is(serveErr, http.ErrServerClosed) {
			errCh <- serveErr
		}
	})

	open := f.OpenURL
	if open == nil {
		open = browser.OpenURL
	}
	if err := open(authURL); err != nil {
		shutdownAndWait(srv, &serveWG)
		return Result{}, fmt.Errorf("open browser: %w", err)
	}

	waitCtx, cancel := context.WithTimeout(ctx, authWaitTimeout)
	defer cancel()

	var code string
	select {
	case <-waitCtx.Done():
		shutdownAndWait(srv, &serveWG)
		return Result{}, fmt.Errorf("oauth timed out: %w", waitCtx.Err())
	case err := <-errCh:
		shutdownAndWait(srv, &serveWG)
		return Result{}, err
	case code = <-codeCh:
	}

	oauthTok, err := oauthCfg.Exchange(ctx, code, oauth2.VerifierOption(verifier))
	if err != nil {
		exchangeErr := fmt.Errorf("exchange code: %w", describeExchangeError(err))
		sendCallbackResult(resultCh, callbackResult{
			status:  http.StatusBadGateway,
			message: exchangeErr.Error(),
		})
		shutdownAndWait(srv, &serveWG)
		return Result{}, exchangeErr
	}

	token := secrets.TokenFromOAuth2(oauthTok)
	if err := f.Store.Set(f.Config.Provider, accountID, token); err != nil {
		sendCallbackResult(resultCh, callbackResult{
			status:  http.StatusInternalServerError,
			message: "Authorization succeeded, but shiet could not save the token. Return to shiet for details.",
		})
		shutdownAndWait(srv, &serveWG)
		return Result{}, fmt.Errorf("persist token: %w", err)
	}

	sendCallbackResult(resultCh, callbackResult{
		status:  http.StatusOK,
		message: "Authorization complete. You can close this window and return to shiet.",
	})
	shutdownAndWait(srv, &serveWG)

	return Result{
		Provider:  f.Config.Provider,
		AccountID: accountID,
		Token:     token,
		Scopes:    append([]string(nil), f.Config.Scopes...),
	}, nil
}

func sendCallbackResult(ch chan<- callbackResult, result callbackResult) {
	select {
	case ch <- result:
	default:
	}
}

func describeExchangeError(err error) error {
	var retrieveErr *oauth2.RetrieveError
	if errors.As(err, &retrieveErr) && isDesktopClientTypeError(retrieveErr) {
		return fmt.Errorf("%w. Google rejected the OAuth token exchange because the configured local/BYO client requires a client secret. Set google.client_secret or SHIET_GOOGLE_CLIENT_SECRET from the Google Desktop OAuth credential bundle (google.auth_mode=local); desktop apps cannot keep this value confidential, so treat it as a provider-required public credential rather than a security boundary. Public builds should use google.auth_mode=broker instead of shipping a shared secret", err)
	}
	return err
}

func isDesktopClientTypeError(err *oauth2.RetrieveError) bool {
	if err.ErrorCode == "invalid_client" {
		return true
	}
	return err.ErrorCode == "invalid_request" &&
		strings.Contains(strings.ToLower(err.ErrorDescription), "client_secret")
}

func callbackPage(message string) string {
	return "<!doctype html><html><body><p>" + html.EscapeString(message) + "</p></body></html>"
}

func shutdownAndWait(srv *http.Server, serveWG *sync.WaitGroup) {
	shutdownCtx, cancel := context.WithTimeout(context.Background(), shutdownGrace)
	defer cancel()
	if err := srv.Shutdown(shutdownCtx); err != nil {
		log.Printf("oauth callback server shutdown: %v", err)
	}

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

func listenLoopback() (net.Listener, string, error) {
	ln, err := net.Listen("tcp", loopbackHost+":0")
	if err != nil {
		return nil, "", fmt.Errorf("listen loopback: %w", err)
	}
	_, port, err := net.SplitHostPort(ln.Addr().String())
	if err != nil {
		_ = ln.Close()
		return nil, "", err
	}
	redirectURL := fmt.Sprintf("http://%s:%s%s", loopbackHost, port, callbackPath)
	return ln, redirectURL, nil
}

func randomString(n int) (string, error) {
	b := make([]byte, n)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(b), nil
}

// ParseCallback extracts the authorization code from a loopback callback URL.
func ParseCallback(rawURL, expectedState string) (string, error) {
	u, err := url.Parse(rawURL)
	if err != nil {
		return "", err
	}
	if u.Query().Get("state") != expectedState {
		return "", errors.New("oauth state mismatch")
	}
	if errMsg := u.Query().Get("error"); errMsg != "" {
		return "", fmt.Errorf("oauth error: %s", errMsg)
	}
	code := u.Query().Get("code")
	if code == "" {
		return "", errors.New("missing oauth code")
	}
	return code, nil
}
