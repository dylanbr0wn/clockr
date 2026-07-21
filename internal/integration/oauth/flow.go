package oauth

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
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
	applog "github.com/dylanbr0wn/shiet/internal/log"
	"github.com/dylanbr0wn/shiet/internal/oauthpages"
	"github.com/pkg/browser"
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

	provider, ok := Lookup(f.Config.Provider)
	if !ok {
		// Fall back to config endpoints when an unknown provider id is used in tests.
		provider = Provider{
			ID:            f.Config.Provider,
			AuthURL:       f.Config.AuthURL,
			TokenURL:      f.Config.TokenURL,
			AuthStyle:     f.Config.AuthStyle,
			DefaultScopes: append([]string(nil), f.Config.Scopes...),
		}
	} else {
		// Local/BYO tests may override the token URL while keeping registry metadata.
		if strings.TrimSpace(f.Config.TokenURL) != "" {
			provider.TokenURL = f.Config.TokenURL
		}
		if strings.TrimSpace(f.Config.AuthURL) != "" {
			provider.AuthURL = f.Config.AuthURL
		}
	}
	challenge := pkceS256(verifier)
	authURL, err := BuildAuthorizationURL(provider, ClientCredentials{
		ClientID:     f.Config.ClientID,
		ClientSecret: f.Config.ClientSecret,
	}, redirectURL, state, challenge, f.Config.Scopes)
	if err != nil {
		return Result{}, err
	}

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

	exchanged, err := ExchangeAuthorizationCode(ctx, provider, ClientCredentials{
		ClientID:     f.Config.ClientID,
		ClientSecret: f.Config.ClientSecret,
	}, redirectURL, code, verifier, ExchangeOptions{
		TokenURL: f.Config.TokenURL,
	})
	if err != nil {
		exchangeErr := fmt.Errorf("exchange code: %w", describeExchangeError(err, f.Config.Provider))
		sendCallbackResult(resultCh, callbackResult{
			status:  http.StatusBadGateway,
			message: exchangeErr.Error(),
		})
		shutdownAndWait(srv, &serveWG)
		return Result{}, exchangeErr
	}

	token := secrets.Token{
		AccessToken:  exchanged.AccessToken,
		RefreshToken: exchanged.RefreshToken,
		TokenType:    exchanged.TokenType,
		Expiry:       exchanged.Expiry,
	}
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

func describeExchangeError(err error, provider string) error {
	var exchangeErr *ExchangeError
	if errors.As(err, &exchangeErr) && isDesktopClientTypeExchangeError(exchangeErr) {
		if strings.EqualFold(strings.TrimSpace(provider), ProviderGitHub) {
			return fmt.Errorf("%w. GitHub rejected the local OAuth token exchange because the configured OAuth App credentials are incomplete. Set github.client_id and github.client_secret (github.auth_mode=local), or use github.auth_mode=broker for public builds", err)
		}
		return fmt.Errorf("%w. Google rejected the OAuth token exchange because the configured local/BYO client requires a client secret. Set google.client_secret or SHIET_GOOGLE_CLIENT_SECRET from the Google Desktop OAuth credential bundle (google.auth_mode=local); desktop apps cannot keep this value confidential, so treat it as a provider-required public credential rather than a security boundary. Public builds should use google.auth_mode=broker instead of shipping a shared secret", err)
	}
	return err
}

func isDesktopClientTypeExchangeError(err *ExchangeError) bool {
	if err.Code == "invalid_client" {
		return true
	}
	return err.Code == "invalid_request" &&
		strings.Contains(strings.ToLower(err.Description), "client_secret")
}

func callbackPage(message string) string {
	page, err := oauthpages.Close(message)
	if err != nil {
		return "<!doctype html><html><body><p>" + html.EscapeString(message) + "</p></body></html>"
	}
	return page
}

func shutdownAndWait(srv *http.Server, serveWG *sync.WaitGroup) {
	shutdownCtx, cancel := context.WithTimeout(context.Background(), shutdownGrace)
	defer cancel()
	if err := srv.Shutdown(shutdownCtx); err != nil {
		log.Printf("op=oauth.callback_shutdown reason=%s", applog.Reason(err))
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

func pkceS256(verifier string) string {
	sum := sha256.Sum256([]byte(verifier))
	return base64.RawURLEncoding.EncodeToString(sum[:])
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
