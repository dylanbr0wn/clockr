package google

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"runtime"
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

// Sentinel errors for brokered connect failure modes. Callers can errors.Is
// these for actionable UI copy. Authorize delegates to the shared desktop
// broker engine; refresh/revoke remain Google-specific adapters.
var (
	ErrHandoffReplay        = oauth.ErrHandoffReplay
	ErrHandoffExpired       = oauth.ErrHandoffExpired
	ErrHandoffStateMismatch = oauth.ErrHandoffStateMismatch
	ErrHandoffVerifier      = oauth.ErrHandoffVerifier
	ErrBrokerRejected       = oauth.ErrBrokerRejected
	ErrBrokerUnavailable    = oauth.ErrBrokerUnavailable
	ErrInvalidRefreshToken  = errors.New("Google OAuth refresh token is invalid")
)

// BrowserOpener opens a URL in the system browser. Injectable for tests.
type BrowserOpener = oauth.BrowserOpener

// BrokerFlow is the Google desktop adapter over the shared oauth.BrokerFlow
// authorization engine, plus Google-only refresh and revoke.
type BrokerFlow struct {
	BaseURL    string
	HTTPClient *http.Client
	OpenURL    BrowserOpener
	AppVersion string
	Platform   string
}

// Authorize implements Authorizer for broker mode via the shared engine.
func (f *BrokerFlow) Authorize(ctx context.Context, accountID string) (oauth.Result, error) {
	accountID = strings.TrimSpace(accountID)
	if accountID == "" {
		return oauth.Result{}, errors.New("account_id is required")
	}
	base := strings.TrimSpace(f.BaseURL)
	if base == "" {
		return oauth.Result{}, fmt.Errorf("%w: set google.broker_base_url or SHIET_GOOGLE_BROKER_BASE_URL", config.ErrBrokerConfig)
	}
	desc := oauth.MustLookup(oauth.ProviderGoogle)
	flow := oauth.BrokerFlow{
		Provider:      service.ProviderGoogle,
		BaseURL:       base,
		DefaultScopes: append([]string(nil), desc.DefaultScopes...),
		HTTPClient:    f.HTTPClient,
		OpenURL:       f.OpenURL,
		AppVersion:    f.AppVersion,
		Platform:      f.Platform,
	}
	return flow.Authorize(ctx, accountID)
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
	if connect.CodeOf(err) == connect.CodeUnavailable || connect.CodeOf(err) == connect.CodeInternal {
		return fmt.Errorf("%w: broker %s unavailable", ErrBrokerUnavailable, op)
	}
	if code != "" {
		return fmt.Errorf("%w: broker %s error %s", ErrBrokerRejected, op, code)
	}
	return fmt.Errorf("%w: broker %s rejected request", ErrBrokerRejected, op)
}
