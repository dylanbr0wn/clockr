package bitbucket

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strings"

	"connectrpc.com/connect"
	brokerv1 "github.com/dylanbr0wn/shiet/gen/shiet/broker/v1"
	"github.com/dylanbr0wn/shiet/gen/shiet/broker/v1/brokerv1connect"
	"github.com/dylanbr0wn/shiet/internal/broker/codes"
	"github.com/dylanbr0wn/shiet/internal/config"
	"github.com/dylanbr0wn/shiet/internal/integration/oauth"
	"github.com/dylanbr0wn/shiet/internal/integration/secrets"
	"github.com/dylanbr0wn/shiet/internal/service"
)

var (
	ErrBrokerUnavailable   = oauth.ErrBrokerUnavailable
	ErrBrokerRejected      = oauth.ErrBrokerRejected
	ErrHandoffReplay       = oauth.ErrHandoffReplay
	ErrHandoffExpired      = oauth.ErrHandoffExpired
	ErrHandoffStateMismatch = oauth.ErrHandoffStateMismatch
	ErrHandoffVerifier     = oauth.ErrHandoffVerifier
	ErrInvalidRefreshToken = errors.New("Bitbucket OAuth refresh token is invalid")
)

// BrokerFlow is the Bitbucket desktop client for the provider-neutral secret-only
// OAuth broker.
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
		return oauth.Result{}, fmt.Errorf("%w: set bitbucket.broker_base_url or SHIET_BITBUCKET_BROKER_BASE_URL", config.ErrBitbucketBrokerConfig)
	}
	desc := oauth.MustLookup(oauth.ProviderBitbucket)
	flow := oauth.BrokerFlow{
		Provider:      service.ProviderBitbucket,
		BaseURL:       base,
		DefaultScopes: append([]string(nil), desc.DefaultScopes...),
		HTTPClient:    f.HTTPClient,
		OpenURL:       f.OpenURL,
		AppVersion:    f.AppVersion,
		Platform:      f.Platform,
	}
	return flow.Authorize(ctx, accountID)
}

func (f *BrokerFlow) RefreshToken(ctx context.Context, refreshToken string, scopes []string) (secrets.Token, error) {
	base := strings.TrimRight(strings.TrimSpace(f.BaseURL), "/")
	if base == "" {
		return secrets.Token{}, fmt.Errorf("%w: set bitbucket.broker_base_url or SHIET_BITBUCKET_BROKER_BASE_URL", config.ErrBitbucketBrokerConfig)
	}
	refreshToken = strings.TrimSpace(refreshToken)
	if refreshToken == "" {
		return secrets.Token{}, fmt.Errorf("%w: refresh token is empty", ErrInvalidRefreshToken)
	}

	request := &brokerv1.RefreshTokenRequest{
		Provider:     brokerv1.Provider_PROVIDER_BITBUCKET,
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

func (f *BrokerFlow) brokerClient(base string) brokerv1connect.OAuthBrokerServiceClient {
	client := f.HTTPClient
	if client == nil {
		client = http.DefaultClient
	}
	return brokerv1connect.NewOAuthBrokerServiceClient(client, base)
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
	return "desktop"
}

func (f *BrokerFlow) mapBrokerRPCError(err error, op string) error {
	code := oauth.BrokerErrorCode(err)
	switch code {
	case codes.InvalidRefreshToken:
		return fmt.Errorf("%w: reconnect Bitbucket", ErrInvalidRefreshToken)
	case codes.RateLimited:
		return fmt.Errorf("%w: too many requests; try again later", ErrBrokerRejected)
	case codes.AuthDisabled, codes.RefreshDisabled, codes.AppVersionDisabled:
		return fmt.Errorf("%w: Bitbucket auth is temporarily unavailable", ErrBrokerRejected)
	}
	if connect.CodeOf(err) == connect.CodeUnavailable || connect.CodeOf(err) == connect.CodeInternal {
		return fmt.Errorf("%w: broker %s unavailable", ErrBrokerUnavailable, op)
	}
	if code != "" {
		return fmt.Errorf("%w: broker %s error %s", ErrBrokerRejected, op, code)
	}
	return fmt.Errorf("%w: broker %s rejected request", ErrBrokerRejected, op)
}
