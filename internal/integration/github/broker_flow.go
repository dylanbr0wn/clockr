package github

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/dylanbr0wn/shiet/internal/config"
	"github.com/dylanbr0wn/shiet/internal/integration/oauth"
	"github.com/dylanbr0wn/shiet/internal/service"
)

var (
	ErrBrokerUnavailable    = oauth.ErrBrokerUnavailable
	ErrBrokerRejected       = oauth.ErrBrokerRejected
	ErrHandoffReplay        = oauth.ErrHandoffReplay
	ErrHandoffExpired       = oauth.ErrHandoffExpired
	ErrHandoffStateMismatch = oauth.ErrHandoffStateMismatch
	ErrHandoffVerifier      = oauth.ErrHandoffVerifier
)

// BrokerFlow is the GitHub desktop client for the provider-neutral secret-only
// OAuth broker. GitHub OAuth App tokens are non-expiring and have no refresh
// path; Revoke removes a single token through the broker.
type BrokerFlow struct {
	BaseURL    string
	HTTPClient *http.Client
	OpenURL    oauth.BrowserOpener
	AppVersion string
	Platform   string
}

func (f *BrokerFlow) Authorize(ctx context.Context, accountID string) (oauth.Result, error) {
	base := strings.TrimSpace(f.BaseURL)
	if base == "" {
		return oauth.Result{}, fmt.Errorf("%w: set github.broker_base_url or SHIET_GITHUB_BROKER_BASE_URL", config.ErrGitHubBrokerConfig)
	}
	desc := oauth.MustLookup(oauth.ProviderGitHub)
	flow := oauth.BrokerFlow{
		Provider:      service.ProviderGitHub,
		BaseURL:       base,
		DefaultScopes: append([]string(nil), desc.DefaultScopes...),
		HTTPClient:    f.HTTPClient,
		OpenURL:       f.OpenURL,
		AppVersion:    f.AppVersion,
		Platform:      f.Platform,
	}
	return flow.Authorize(ctx, accountID)
}

func (f *BrokerFlow) Revoke(ctx context.Context, accessToken string) error {
	accessToken = strings.TrimSpace(accessToken)
	if accessToken == "" {
		return errors.New("access_token is required")
	}
	base := strings.TrimRight(strings.TrimSpace(f.BaseURL), "/")
	if base == "" {
		return fmt.Errorf("%w: set github.broker_base_url or SHIET_GITHUB_BROKER_BASE_URL", config.ErrGitHubBrokerConfig)
	}
	body, _ := json.Marshal(oauth.BrokerRevokeRequest{
		AccessToken: accessToken,
		Reason:      "user_disconnect",
	})
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, base+"/v1/github/oauth/revoke", strings.NewReader(string(body)))
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
		return fmt.Errorf("%w: contact broker revoke: %v", ErrBrokerUnavailable, err)
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("%w: broker revoke returned %d: %s", ErrBrokerRejected, resp.StatusCode, strings.TrimSpace(string(raw)))
	}
	var out oauth.BrokerRevokeResponse
	if err := json.Unmarshal(raw, &out); err != nil || !out.Revoked {
		return fmt.Errorf("%w: invalid revoke response", ErrBrokerRejected)
	}
	return nil
}
