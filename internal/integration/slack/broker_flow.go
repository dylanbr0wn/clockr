package slack

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strings"

	"connectrpc.com/connect"
	brokerv1 "github.com/dylanbr0wn/shiet/gen/shiet/broker/v1"
	"github.com/dylanbr0wn/shiet/gen/shiet/broker/v1/brokerv1connect"
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

// BrokerFlow is the Slack desktop client for the provider-neutral secret-only
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
		return oauth.Result{}, fmt.Errorf("%w: set slack.broker_base_url or SHIET_SLACK_BROKER_BASE_URL", config.ErrSlackBrokerConfig)
	}
	desc := oauth.MustLookup(oauth.ProviderSlack)
	flow := oauth.BrokerFlow{
		Provider:      service.ProviderSlack,
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
		return fmt.Errorf("%w: set slack.broker_base_url or SHIET_SLACK_BROKER_BASE_URL", config.ErrSlackBrokerConfig)
	}
	client := f.HTTPClient
	if client == nil {
		client = http.DefaultClient
	}
	request := &brokerv1.RevokeTokenRequest{
		Provider:   brokerv1.Provider_PROVIDER_SLACK,
		Credential: &brokerv1.RevokeTokenRequest_AccessToken{AccessToken: accessToken},
		Reason:     "user_disconnect",
	}
	response, err := brokerv1connect.NewOAuthBrokerServiceClient(client, base).RevokeToken(ctx, connect.NewRequest(request))
	if err != nil {
		code := oauth.BrokerErrorCode(err)
		if connect.CodeOf(err) == connect.CodeUnavailable || connect.CodeOf(err) == connect.CodeInternal {
			return fmt.Errorf("%w: contact broker revoke", ErrBrokerUnavailable)
		}
		return fmt.Errorf("%w: broker revoke error %s", ErrBrokerRejected, code)
	}
	if !response.Msg.Revoked {
		return fmt.Errorf("%w: invalid revoke response", ErrBrokerRejected)
	}
	return nil
}
