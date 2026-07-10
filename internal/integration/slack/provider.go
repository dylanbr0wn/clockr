package slack

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"

	"github.com/dylanbr0wn/shiet/internal/config"
	"github.com/dylanbr0wn/shiet/internal/db/sqlc"
	"github.com/dylanbr0wn/shiet/internal/integration/connection"
	"github.com/dylanbr0wn/shiet/internal/integration/httpclient"
	"github.com/dylanbr0wn/shiet/internal/integration/oauth"
	"github.com/dylanbr0wn/shiet/internal/integration/secrets"
	"github.com/dylanbr0wn/shiet/internal/service"
)

const (
	apiBaseURL      = "https://slack.com/api"
	authTestPath    = "/auth.test"
	conversationsPath = "/users.conversations"
	defaultPageSize = 200
	providerSlack   = service.ProviderSlack
)

// Provider implements Slack workspace connect via OAuth and channel sync for
// evidence-source selection.
type Provider struct {
	Config        oauth.ProviderConfig
	Store         secrets.TokenStore
	Registry      *connection.Registry
	Queries       *sqlc.Queries
	HTTP          *http.Client
	BaseURL       string
	AuthMode      string
	BrokerBaseURL string
	Authorizer    Authorizer
	Revoker       TokenRevoker
}

// Authorizer runs a Slack OAuth connect flow and returns token material
// without deciding workspace identity. Connect always revalidates identity
// through auth.test before persisting the token.
type Authorizer interface {
	Authorize(ctx context.Context, accountID string) (oauth.Result, error)
}

// TokenRevoker revokes a broker-issued Slack OAuth access token.
type TokenRevoker interface {
	Revoke(ctx context.Context, accessToken string) error
}

// Connect runs Slack OAuth, stores the user token in the keychain, upserts
// connection metadata, and syncs accessible channels.
func (p *Provider) Connect(ctx context.Context) (connection.Connection, error) {
	authorizer := p.Authorizer
	if authorizer == nil {
		if p.usesBrokerAuth() {
			// Broker mode must never fall through to local desktop OAuth.
			base := strings.TrimSpace(p.BrokerBaseURL)
			if base == "" {
				return connection.Connection{}, fmt.Errorf("%w: set slack.broker_base_url or SHIET_SLACK_BROKER_BASE_URL", config.ErrSlackBrokerConfig)
			}
			authorizer = &BrokerFlow{BaseURL: base, HTTPClient: p.HTTP}
		} else if strings.TrimSpace(p.Config.ClientID) != "" {
			authorizer = &oauth.Flow{Config: p.Config, Store: transientTokenStore{}}
		} else {
			return connection.Connection{}, errors.New("Slack OAuth is not configured")
		}
	}
	result, err := authorizer.Authorize(ctx, "slack")
	if err != nil {
		return connection.Connection{}, fmt.Errorf("authorize slack: %w", err)
	}
	if strings.TrimSpace(result.Token.AccessToken) == "" {
		return connection.Connection{}, errors.New("Slack OAuth returned an empty access token")
	}
	if p.usesBrokerAuth() {
		result.Token.CredentialSource = secrets.CredentialSourceBroker
	} else {
		result.Token.CredentialSource = secrets.CredentialSourceLocalOAuth
	}
	return p.connectWithToken(ctx, result.Token, result.Scopes)
}

type transientTokenStore struct{}

func (transientTokenStore) Get(string, string) (secrets.Token, error) {
	return secrets.Token{}, secrets.ErrNotFound
}
func (transientTokenStore) Set(string, string, secrets.Token) error { return nil }
func (transientTokenStore) Delete(string, string) error             { return nil }

func (p *Provider) connectWithToken(ctx context.Context, token secrets.Token, scopes []string) (connection.Connection, error) {
	if p.Store == nil {
		return connection.Connection{}, errors.New("token store is required")
	}
	if p.Registry == nil {
		return connection.Connection{}, errors.New("connection registry is required")
	}

	team, err := p.fetchAuthTest(ctx, token.AccessToken)
	if err != nil {
		return connection.Connection{}, err
	}
	teamID := strings.TrimSpace(team.TeamID)
	if teamID == "" {
		return connection.Connection{}, errors.New("slack team id is empty")
	}

	if strings.TrimSpace(token.TokenType) == "" {
		token.TokenType = "Bearer"
	}
	if err := p.Store.Set(providerSlack, teamID, token); err != nil {
		return connection.Connection{}, fmt.Errorf("persist token: %w", err)
	}

	label := strings.TrimSpace(team.Team)
	if label == "" {
		label = teamID
	}

	conn, err := p.Registry.Upsert(ctx, connection.UpsertInput{
		Provider:     providerSlack,
		AccountLabel: label,
		AccountID:    teamID,
		Scopes:       append([]string(nil), scopes...),
		Status:       connection.StatusConnected,
	})
	if err != nil {
		_ = p.Store.Delete(providerSlack, teamID)
		return connection.Connection{}, err
	}

	if p.Queries != nil {
		if _, err := p.SyncChannels(ctx, teamID); err != nil {
			_ = p.Disconnect(ctx, teamID)
			return connection.Connection{}, fmt.Errorf("sync channels: %w", err)
		}
	}

	return conn, nil
}

// Disconnect removes the token from the keychain, clears synced channels, and
// marks the connection disconnected.
func (p *Provider) Disconnect(ctx context.Context, accountID string) error {
	accountID = strings.TrimSpace(accountID)
	if accountID == "" {
		return errors.New("account_id is required")
	}
	if p.Registry == nil {
		return errors.New("connection registry is required")
	}

	if p.Queries != nil {
		if err := p.Queries.DeleteSlackChannelsByAccount(ctx, accountID); err != nil {
			return fmt.Errorf("clear channels: %w", err)
		}
	}

	var accessToken string
	var credentialSource secrets.CredentialSource
	if p.Store != nil {
		if token, err := p.Store.Get(providerSlack, accountID); err == nil {
			accessToken = strings.TrimSpace(token.AccessToken)
			credentialSource = token.CredentialSource
		}
	}
	if credentialSource == secrets.CredentialSourceBroker && accessToken != "" {
		revoker := p.Revoker
		if revoker == nil && strings.TrimSpace(p.BrokerBaseURL) != "" {
			revoker = &BrokerFlow{BaseURL: p.BrokerBaseURL, HTTPClient: p.HTTP}
		}
		if revoker != nil {
			_ = revoker.Revoke(ctx, accessToken)
		}
	}

	if p.Store != nil {
		if err := p.Store.Delete(providerSlack, accountID); err != nil && !errors.Is(err, secrets.ErrNotFound) {
			return fmt.Errorf("delete token: %w", err)
		}
	}
	return p.Registry.Disconnect(ctx, providerSlack, accountID)
}

func (p *Provider) usesBrokerAuth() bool {
	mode := strings.TrimSpace(p.AuthMode)
	// Empty mode matches the public-build default (broker), same as Google.
	if mode == "" {
		return true
	}
	return strings.EqualFold(mode, config.AuthModeBroker)
}

// OAuthAvailable reports whether the configured mode can start browser OAuth.
func (p *Provider) OAuthAvailable() bool {
	if p.usesBrokerAuth() {
		return strings.TrimSpace(p.BrokerBaseURL) != ""
	}
	return strings.TrimSpace(p.Config.ClientID) != "" && strings.TrimSpace(p.Config.ClientSecret) != ""
}

// SyncChannels lists channels visible to the connected workspace and upserts
// local slack_channel rows. Existing selected flags are preserved on conflict.
func (p *Provider) SyncChannels(ctx context.Context, accountID string) ([]sqlc.SlackChannel, error) {
	accountID = strings.TrimSpace(accountID)
	if accountID == "" {
		return nil, errors.New("account_id is required")
	}
	if p.Queries == nil {
		return nil, errors.New("queries are required")
	}

	var out []sqlc.SlackChannel
	cursor := ""
	for {
		q := url.Values{}
		q.Set("types", "public_channel,private_channel")
		q.Set("exclude_archived", "true")
		q.Set("limit", fmt.Sprintf("%d", defaultPageSize))
		if cursor != "" {
			q.Set("cursor", cursor)
		}

		var resp conversationsResponse
		if err := p.getJSON(ctx, accountID, conversationsPath, q, &resp); err != nil {
			return nil, err
		}
		if !resp.OK {
			return nil, fmt.Errorf("slack api %s: %s", conversationsPath, strings.TrimSpace(resp.Error))
		}

		for _, item := range resp.Channels {
			channelID := strings.TrimSpace(item.ID)
			name := strings.TrimSpace(item.Name)
			if channelID == "" || name == "" {
				continue
			}
			isPrivate := int64(0)
			if item.IsPrivate {
				isPrivate = 1
			}
			channel, err := p.Queries.UpsertSlackChannel(ctx, sqlc.UpsertSlackChannelParams{
				AccountID:  accountID,
				ExternalID: channelID,
				Name:       name,
				IsPrivate:  isPrivate,
				Column5:    0,
			})
			if err != nil {
				return nil, fmt.Errorf("upsert channel %q: %w", name, err)
			}
			out = append(out, channel)
		}

		cursor = strings.TrimSpace(resp.ResponseMetadata.NextCursor)
		if cursor == "" {
			break
		}
	}
	return out, nil
}

// ListChannels returns all synced Slack channels for all accounts.
func (p *Provider) ListChannels(ctx context.Context) ([]sqlc.SlackChannel, error) {
	if p.Queries == nil {
		return nil, errors.New("queries are required")
	}
	return p.Queries.ListSlackChannels(ctx)
}

// SetChannelSelected toggles whether a channel is included as an evidence source.
func (p *Provider) SetChannelSelected(ctx context.Context, channelID int64, selected bool) error {
	if p.Queries == nil {
		return errors.New("queries are required")
	}
	sel := int64(0)
	if selected {
		sel = 1
	}
	return p.Queries.SetSlackChannelSelected(ctx, sqlc.SetSlackChannelSelectedParams{
		Selected: sel,
		ID:       channelID,
	})
}

func (p *Provider) fetchAuthTest(ctx context.Context, accessToken string) (authTestResponse, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, p.baseURL()+authTestPath, nil)
	if err != nil {
		return authTestResponse{}, err
	}
	req.Header.Set("Authorization", "Bearer "+accessToken)
	req.Header.Set("Accept", "application/json")

	client := p.HTTP
	if client == nil {
		client = http.DefaultClient
	}
	resp, err := client.Do(req)
	if err != nil {
		return authTestResponse{}, fmt.Errorf("slack auth.test: %w", err)
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return authTestResponse{}, fmt.Errorf("read slack auth.test: %w", err)
	}
	var team authTestResponse
	if err := json.Unmarshal(body, &team); err != nil {
		return authTestResponse{}, fmt.Errorf("decode slack auth.test: %w", err)
	}
	if !team.OK {
		return authTestResponse{}, fmt.Errorf("invalid Slack OAuth token (slack api %s: %s)", authTestPath, strings.TrimSpace(team.Error))
	}
	return team, nil
}

func (p *Provider) getJSON(ctx context.Context, accountID, path string, query url.Values, dest any) error {
	client := p.httpClient(accountID)
	rawURL := p.baseURL() + path
	if len(query) > 0 {
		rawURL += "?" + query.Encode()
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)
	if err != nil {
		return err
	}
	req.Header.Set("Accept", "application/json")

	resp, err := client.Do(ctx, req)
	if err != nil {
		return err
	}
	body, err := httpclient.ReadBody(resp)
	if err != nil {
		return err
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("slack api %s: %s", path, strings.TrimSpace(string(body)))
	}
	if err := json.Unmarshal(body, dest); err != nil {
		return fmt.Errorf("decode slack api response: %w", err)
	}
	return nil
}

func (p *Provider) httpClient(accountID string) *httpclient.Client {
	return &httpclient.Client{
		Provider:  providerSlack,
		AccountID: accountID,
		Config:    oauth.ProviderConfig{Provider: providerSlack},
		Store:     p.Store,
		Registry:  p.Registry,
		HTTP:      p.HTTP,
		Refresher: slackNoRefresh{},
	}
}

type slackNoRefresh struct{}

func (slackNoRefresh) Refresh(context.Context, secrets.Token) (secrets.Token, error) {
	return secrets.Token{}, errors.New("Slack user tokens do not support refresh in this integration")
}

func (p *Provider) baseURL() string {
	if strings.TrimSpace(p.BaseURL) != "" {
		return strings.TrimRight(p.BaseURL, "/")
	}
	return apiBaseURL
}

type authTestResponse struct {
	OK    bool   `json:"ok"`
	Error string `json:"error"`
	Team  string `json:"team"`
	TeamID string `json:"team_id"`
}

type conversationsResponse struct {
	OK               bool          `json:"ok"`
	Error            string        `json:"error"`
	Channels         []channelItem `json:"channels"`
	ResponseMetadata struct {
		NextCursor string `json:"next_cursor"`
	} `json:"response_metadata"`
}

type channelItem struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	IsPrivate bool   `json:"is_private"`
}
