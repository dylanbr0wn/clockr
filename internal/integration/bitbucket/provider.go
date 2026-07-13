package bitbucket

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/dylanbr0wn/shiet/internal/config"
	"github.com/dylanbr0wn/shiet/internal/db/sqlc"
	"github.com/dylanbr0wn/shiet/internal/integration/connection"
	"github.com/dylanbr0wn/shiet/internal/integration/httpclient"
	"github.com/dylanbr0wn/shiet/internal/integration/oauth"
	"github.com/dylanbr0wn/shiet/internal/integration/secrets"
	"github.com/dylanbr0wn/shiet/internal/service"
	"golang.org/x/oauth2"
)

const (
	apiBaseURL      = "https://api.bitbucket.org/2.0"
	userPath        = "/user"
	workspacesPath  = "/workspaces"
	repositoriesPath = "/repositories"
	defaultPageSize = 100
	providerBitbucket = service.ProviderBitbucket
)

// Provider implements Bitbucket account connect via OAuth and workspace/repo sync
// for evidence-source selection.
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
}

// Authorizer runs a Bitbucket OAuth connect flow and returns token material
// without deciding account identity. Connect always revalidates identity through
// GET /user before persisting the token.
type Authorizer interface {
	Authorize(ctx context.Context, accountID string) (oauth.Result, error)
}

// Connect runs Bitbucket OAuth, stores the token in the keychain, upserts
// connection metadata, and syncs accessible workspaces and repos.
func (p *Provider) Connect(ctx context.Context) (connection.Connection, error) {
	authorizer := p.Authorizer
	if authorizer == nil {
		if p.usesBrokerAuth() {
			base := strings.TrimSpace(p.BrokerBaseURL)
			if base == "" {
				return connection.Connection{}, fmt.Errorf("%w: set bitbucket.broker_base_url or SHIET_BITBUCKET_BROKER_BASE_URL", config.ErrBitbucketBrokerConfig)
			}
			authorizer = &BrokerFlow{BaseURL: base, HTTPClient: p.HTTP}
		} else if strings.TrimSpace(p.Config.ClientID) != "" {
			authorizer = &oauth.Flow{Config: p.Config, Store: transientTokenStore{}}
		} else {
			return connection.Connection{}, errors.New("Bitbucket OAuth is not configured")
		}
	}
	result, err := authorizer.Authorize(ctx, "bitbucket")
	if err != nil {
		return connection.Connection{}, fmt.Errorf("authorize bitbucket: %w", err)
	}
	if strings.TrimSpace(result.Token.AccessToken) == "" {
		return connection.Connection{}, errors.New("Bitbucket OAuth returned an empty access token")
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

	user, err := p.fetchUser(ctx, token.AccessToken)
	if err != nil {
		return connection.Connection{}, err
	}
	accountID := strings.TrimSpace(user.UUID)
	if accountID == "" {
		return connection.Connection{}, errors.New("bitbucket user uuid is empty")
	}

	if strings.TrimSpace(token.TokenType) == "" {
		token.TokenType = "Bearer"
	}
	if err := p.Store.Set(providerBitbucket, accountID, token); err != nil {
		return connection.Connection{}, fmt.Errorf("persist token: %w", err)
	}

	label := strings.TrimSpace(user.DisplayName)
	if label == "" {
		label = strings.TrimSpace(user.Username)
	}
	if label == "" {
		label = accountID
	}

	conn, err := p.Registry.Upsert(ctx, connection.UpsertInput{
		Provider:     providerBitbucket,
		AccountLabel: label,
		AccountID:    accountID,
		Scopes:       append([]string(nil), scopes...),
		Status:       connection.StatusConnected,
	})
	if err != nil {
		_ = p.Store.Delete(providerBitbucket, accountID)
		return connection.Connection{}, err
	}

	if p.Queries != nil {
		if _, _, err := p.SyncWorkspacesRepos(ctx, accountID); err != nil {
			_ = p.Disconnect(ctx, accountID)
			return connection.Connection{}, fmt.Errorf("sync resources: %w", err)
		}
	}

	return conn, nil
}

// Disconnect removes the token from the keychain, clears synced resources, and
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
		if err := p.Queries.DeleteBitbucketReposByAccount(ctx, accountID); err != nil {
			return fmt.Errorf("clear repos: %w", err)
		}
		if err := p.Queries.DeleteBitbucketWorkspacesByAccount(ctx, accountID); err != nil {
			return fmt.Errorf("clear workspaces: %w", err)
		}
	}

	if p.Store != nil {
		if err := p.Store.Delete(providerBitbucket, accountID); err != nil && !errors.Is(err, secrets.ErrNotFound) {
			return fmt.Errorf("delete token: %w", err)
		}
	}
	return p.Registry.Disconnect(ctx, providerBitbucket, accountID)
}

func (p *Provider) usesBrokerAuth() bool {
	mode := strings.TrimSpace(p.AuthMode)
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

// SyncWorkspacesRepos lists workspaces and repositories visible to the connected
// account and upserts local rows. Existing selected flags are preserved on conflict.
func (p *Provider) SyncWorkspacesRepos(ctx context.Context, accountID string) ([]sqlc.BitbucketWorkspace, []sqlc.BitbucketRepo, error) {
	accountID = strings.TrimSpace(accountID)
	if accountID == "" {
		return nil, nil, errors.New("account_id is required")
	}
	if p.Queries == nil {
		return nil, nil, errors.New("queries are required")
	}

	workspaces, err := p.syncWorkspaces(ctx, accountID)
	if err != nil {
		return nil, nil, err
	}
	repos, err := p.syncRepos(ctx, accountID)
	if err != nil {
		return nil, nil, err
	}
	return workspaces, repos, nil
}

func (p *Provider) syncWorkspaces(ctx context.Context, accountID string) ([]sqlc.BitbucketWorkspace, error) {
	var out []sqlc.BitbucketWorkspace
	nextURL := p.baseURL() + workspacesPath + "?role=member&pagelen=" + fmt.Sprintf("%d", defaultPageSize)
	for nextURL != "" {
		var page paginatedResponse[workspaceItem]
		if err := p.getAbsoluteJSON(ctx, accountID, nextURL, &page); err != nil {
			return nil, err
		}
		for _, item := range page.Values {
			externalID := strings.TrimSpace(item.UUID)
			slug := strings.TrimSpace(item.Slug)
			name := strings.TrimSpace(item.Name)
			if externalID == "" || slug == "" {
				continue
			}
			if name == "" {
				name = slug
			}
			row, err := p.Queries.UpsertBitbucketWorkspace(ctx, sqlc.UpsertBitbucketWorkspaceParams{
				AccountID:  accountID,
				ExternalID: externalID,
				Slug:       slug,
				Name:       name,
				Column5:    0,
			})
			if err != nil {
				return nil, fmt.Errorf("upsert workspace %q: %w", slug, err)
			}
			out = append(out, row)
		}
		nextURL = strings.TrimSpace(page.Next)
	}
	return out, nil
}

func (p *Provider) syncRepos(ctx context.Context, accountID string) ([]sqlc.BitbucketRepo, error) {
	var out []sqlc.BitbucketRepo
	nextURL := p.baseURL() + repositoriesPath + "?role=member&pagelen=" + fmt.Sprintf("%d", defaultPageSize)
	for nextURL != "" {
		var page paginatedResponse[repoItem]
		if err := p.getAbsoluteJSON(ctx, accountID, nextURL, &page); err != nil {
			return nil, err
		}
		for _, item := range page.Values {
			externalID := strings.TrimSpace(item.UUID)
			fullName := strings.TrimSpace(item.FullName)
			if externalID == "" || fullName == "" {
				continue
			}
			name := strings.TrimSpace(item.Name)
			if name == "" {
				name = fullName
			}
			workspaceUUID := strings.TrimSpace(item.Workspace.UUID)
			if workspaceUUID == "" {
				continue
			}
			private := int64(0)
			if item.IsPrivate {
				private = 1
			}
			row, err := p.Queries.UpsertBitbucketRepo(ctx, sqlc.UpsertBitbucketRepoParams{
				AccountID:     accountID,
				WorkspaceUuid: workspaceUUID,
				ExternalID:    externalID,
				Name:          name,
				FullName:      fullName,
				Private:       private,
				Column7:       0,
			})
			if err != nil {
				return nil, fmt.Errorf("upsert repo %q: %w", fullName, err)
			}
			out = append(out, row)
		}
		nextURL = strings.TrimSpace(page.Next)
	}
	return out, nil
}

func (p *Provider) ListWorkspaces(ctx context.Context) ([]sqlc.BitbucketWorkspace, error) {
	if p.Queries == nil {
		return nil, errors.New("queries are required")
	}
	return p.Queries.ListBitbucketWorkspaces(ctx)
}

func (p *Provider) ListRepos(ctx context.Context) ([]sqlc.BitbucketRepo, error) {
	if p.Queries == nil {
		return nil, errors.New("queries are required")
	}
	return p.Queries.ListBitbucketRepos(ctx)
}

func (p *Provider) SetWorkspaceSelected(ctx context.Context, workspaceID int64, selected bool) error {
	if p.Queries == nil {
		return errors.New("queries are required")
	}
	sel := int64(0)
	if selected {
		sel = 1
	}
	return p.Queries.SetBitbucketWorkspaceSelected(ctx, sqlc.SetBitbucketWorkspaceSelectedParams{
		Selected: sel,
		ID:       workspaceID,
	})
}

func (p *Provider) SetRepoSelected(ctx context.Context, repoID int64, selected bool) error {
	if p.Queries == nil {
		return errors.New("queries are required")
	}
	sel := int64(0)
	if selected {
		sel = 1
	}
	return p.Queries.SetBitbucketRepoSelected(ctx, sqlc.SetBitbucketRepoSelectedParams{
		Selected: sel,
		ID:       repoID,
	})
}

func (p *Provider) fetchUser(ctx context.Context, accessToken string) (userResponse, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, p.baseURL()+userPath, nil)
	if err != nil {
		return userResponse{}, err
	}
	req.Header.Set("Authorization", "Bearer "+accessToken)
	req.Header.Set("Accept", "application/json")

	client := p.HTTP
	if client == nil {
		client = http.DefaultClient
	}
	resp, err := client.Do(req)
	if err != nil {
		return userResponse{}, fmt.Errorf("bitbucket user: %w", err)
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return userResponse{}, fmt.Errorf("read bitbucket user: %w", err)
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return userResponse{}, fmt.Errorf("invalid Bitbucket OAuth token (bitbucket api %s: %s)", userPath, strings.TrimSpace(string(body)))
	}
	var user userResponse
	if err := json.Unmarshal(body, &user); err != nil {
		return userResponse{}, fmt.Errorf("decode bitbucket user: %w", err)
	}
	return user, nil
}

func (p *Provider) getAbsoluteJSON(ctx context.Context, accountID, rawURL string, dest any) error {
	client := p.httpClient(accountID)
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
		return fmt.Errorf("bitbucket api %s: %s", rawURL, strings.TrimSpace(string(body)))
	}
	if err := json.Unmarshal(body, dest); err != nil {
		return fmt.Errorf("decode bitbucket api response: %w", err)
	}
	return nil
}

func (p *Provider) httpClient(accountID string) *httpclient.Client {
	client := &httpclient.Client{
		Provider:  providerBitbucket,
		AccountID: accountID,
		Config:    p.Config,
		Store:     p.Store,
		Registry:  p.Registry,
		HTTP:      p.HTTP,
	}
	if p.usesBrokerAuth() {
		base := strings.TrimSpace(p.BrokerBaseURL)
		client.Refresher = &brokerTokenRefresher{
			flow:   &BrokerFlow{BaseURL: base, HTTPClient: p.HTTP},
			scopes: append([]string(nil), p.Config.Scopes...),
		}
	} else if strings.TrimSpace(p.Config.ClientID) != "" && strings.TrimSpace(p.Config.ClientSecret) != "" {
		client.Refresher = &localTokenRefresher{config: p.Config}
	}
	return client
}

type brokerTokenRefresher struct {
	flow   *BrokerFlow
	scopes []string
}

func (r *brokerTokenRefresher) Refresh(ctx context.Context, current secrets.Token) (secrets.Token, error) {
	return r.flow.RefreshToken(ctx, current.RefreshToken, r.scopes)
}

type localTokenRefresher struct {
	config oauth.ProviderConfig
}

func (r *localTokenRefresher) Refresh(ctx context.Context, current secrets.Token) (secrets.Token, error) {
	desc := oauth.MustLookup(oauth.ProviderBitbucket)
	cfg := &oauth2.Config{
		ClientID:     r.config.ClientID,
		ClientSecret: r.config.ClientSecret,
		Endpoint: oauth2.Endpoint{
			AuthURL:  desc.AuthURL,
			TokenURL: desc.TokenURL,
			AuthStyle: desc.AuthStyle,
		},
	}
	tok, err := cfg.TokenSource(ctx, &oauth2.Token{RefreshToken: current.RefreshToken}).Token()
	if err != nil {
		return secrets.Token{}, err
	}
	tokenType := strings.TrimSpace(tok.TokenType)
	if tokenType == "" {
		tokenType = "Bearer"
	}
	nextRefresh := strings.TrimSpace(tok.RefreshToken)
	if nextRefresh == "" {
		nextRefresh = current.RefreshToken
	}
	return secrets.Token{
		AccessToken:  tok.AccessToken,
		RefreshToken: nextRefresh,
		TokenType:    tokenType,
		Expiry:       tok.Expiry,
	}, nil
}

func (p *Provider) baseURL() string {
	if strings.TrimSpace(p.BaseURL) != "" {
		return strings.TrimRight(p.BaseURL, "/")
	}
	return apiBaseURL
}

type paginatedResponse[T any] struct {
	Next   string `json:"next"`
	Values []T    `json:"values"`
}

type userResponse struct {
	UUID        string `json:"uuid"`
	DisplayName string `json:"display_name"`
	Username    string `json:"username"`
}

type workspaceItem struct {
	UUID string `json:"uuid"`
	Slug string `json:"slug"`
	Name string `json:"name"`
}

type repoItem struct {
	UUID      string `json:"uuid"`
	Name      string `json:"name"`
	FullName  string `json:"full_name"`
	IsPrivate bool   `json:"is_private"`
	Workspace struct {
		UUID string `json:"uuid"`
		Slug string `json:"slug"`
	} `json:"workspace"`
}
