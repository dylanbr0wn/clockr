package httpclient

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/dylanbr0wn/shiet/internal/integration/connection"
	"github.com/dylanbr0wn/shiet/internal/integration/oauth"
	"github.com/dylanbr0wn/shiet/internal/integration/secrets"
)

const (
	defaultTimeout   = 30 * time.Second
	maxRateRetries   = 3
	defaultRateDelay = time.Second
)

// TokenRefresher refreshes OAuth tokens without using local client credentials.
// Used in broker mode so httpclient stays free of a google package import cycle.
type TokenRefresher interface {
	Refresh(ctx context.Context, current secrets.Token) (secrets.Token, error)
}

// Client performs authenticated provider HTTP calls with token injection,
// refresh-on-401, and basic rate-limit backoff.
type Client struct {
	Provider  string
	AccountID string
	Config    oauth.ProviderConfig
	Store     secrets.TokenStore
	Registry  *connection.Registry
	HTTP      *http.Client
	// Refresher, when set, replaces the local oauth2 TokenSource refresh path
	// (broker mode). Local/BYO leaves this nil.
	Refresher TokenRefresher
}

// Do executes an HTTP request with bearer auth. On 401 it refreshes the token once
// and retries. On 429 it backs off using Retry-After when present.
func (c *Client) Do(ctx context.Context, req *http.Request) (*http.Response, error) {
	if c.Store == nil {
		return nil, fmt.Errorf("token store is required")
	}

	token, err := c.Store.Get(c.Provider, c.AccountID)
	if err != nil {
		return nil, errors.Join(fmt.Errorf("load token: %w", err), c.markNeedsReauth(ctx))
	}

	var lastResp *http.Response
	refreshed := false
	for attempt := 0; attempt <= maxRateRetries; attempt++ {
		cloned := req.Clone(ctx)
		cloned.Header.Set("Authorization", formatBearer(token))

		httpClient := c.HTTP
		if httpClient == nil {
			httpClient = &http.Client{Timeout: defaultTimeout}
		}

		resp, err := httpClient.Do(cloned)
		if err != nil {
			return nil, err
		}

		if resp.StatusCode == http.StatusUnauthorized {
			resp.Body.Close()
			if refreshed {
				return nil, errors.Join(
					fmt.Errorf("unauthorized after token refresh"),
					c.markNeedsReauth(ctx),
				)
			}
			next, refreshErr := c.refreshToken(ctx, token)
			if refreshErr != nil {
				return nil, errors.Join(refreshErr, c.markNeedsReauth(ctx))
			}
			token = next
			refreshed = true
			continue
		}

		if resp.StatusCode == http.StatusTooManyRequests && attempt < maxRateRetries {
			delay := retryAfter(resp.Header.Get("Retry-After"))
			resp.Body.Close()
			if err := sleep(ctx, delay); err != nil {
				return nil, err
			}
			continue
		}

		lastResp = resp
		break
	}

	if lastResp == nil {
		return nil, fmt.Errorf("request failed after retries")
	}
	return lastResp, nil
}

func (c *Client) refreshToken(ctx context.Context, current secrets.Token) (secrets.Token, error) {
	var next secrets.Token
	if c.Refresher != nil {
		refreshed, err := c.Refresher.Refresh(ctx, current)
		if err != nil {
			return secrets.Token{}, fmt.Errorf("refresh token: %w", err)
		}
		next = refreshed
	} else {
		oauthCfg := c.Config.OAuth2Config("")
		src := oauthCfg.TokenSource(ctx, current.ToOAuth2())
		oauthTok, err := src.Token()
		if err != nil {
			return secrets.Token{}, fmt.Errorf("refresh token: %w", err)
		}
		next = secrets.TokenFromOAuth2(oauthTok)
	}
	if next.CredentialSource == "" {
		next.CredentialSource = current.CredentialSource
	}

	if err := c.Store.Set(c.Provider, c.AccountID, next); err != nil {
		return secrets.Token{}, fmt.Errorf("persist refreshed token: %w", err)
	}
	return next, nil
}

func (c *Client) markNeedsReauth(ctx context.Context) error {
	if c.Registry == nil {
		return nil
	}
	return c.Registry.SetStatus(ctx, c.Provider, c.AccountID, connection.StatusNeedsReauth)
}

func formatBearer(token secrets.Token) string {
	tokenType := strings.TrimSpace(token.TokenType)
	if tokenType == "" {
		tokenType = "Bearer"
	}
	return tokenType + " " + token.AccessToken
}

func retryAfter(raw string) time.Duration {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return defaultRateDelay
	}
	if seconds, err := strconv.Atoi(raw); err == nil {
		return time.Duration(seconds) * time.Second
	}
	if when, err := http.ParseTime(raw); err == nil {
		delay := time.Until(when)
		if delay > 0 {
			return delay
		}
	}
	return defaultRateDelay
}

func sleep(ctx context.Context, d time.Duration) error {
	if d <= 0 {
		return nil
	}
	timer := time.NewTimer(d)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}

// ReadBody reads and closes an HTTP response body.
func ReadBody(resp *http.Response) ([]byte, error) {
	defer resp.Body.Close()
	return io.ReadAll(resp.Body)
}
