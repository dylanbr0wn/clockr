package github

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
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
	apiBaseURL     = "https://api.github.com"
	userPath       = "/user"
	userReposPath  = "/user/repos"
	defaultPerPage = 100
	providerGitHub = service.ProviderGitHub
)

// Provider implements GitHub account connect via personal access token and
// repo sync for evidence-source selection.
type Provider struct {
	Config        oauth.ProviderConfig
	Store         secrets.TokenStore
	Registry      *connection.Registry
	Queries       *sqlc.Queries
	HTTP          *http.Client // optional; used for pre-store PAT validation
	BaseURL       string       // override for tests
	AuthMode      string
	BrokerBaseURL string
	Authorizer    Authorizer
	Revoker       TokenRevoker
}

// Authorizer runs a GitHub OAuth connect flow and returns token material
// without deciding the GitHub account identity. Connect always revalidates
// identity through GET /user before persisting the token.
type Authorizer interface {
	Authorize(ctx context.Context, accountID string) (oauth.Result, error)
}

// TokenRevoker revokes a broker-issued GitHub OAuth access token.
type TokenRevoker interface {
	Revoke(ctx context.Context, accessToken string) error
}

// Connect validates a PAT against the GitHub API, stores it in the keychain,
// upserts connection metadata, and syncs accessible repos.
func (p *Provider) Connect(ctx context.Context, pat string) (connection.Connection, error) {
	pat = strings.TrimSpace(pat)
	if pat != "" {
		return p.connectWithToken(ctx, secrets.Token{AccessToken: pat, TokenType: "Bearer", CredentialSource: secrets.CredentialSourcePAT}, nil, true)
	}
	authorizer := p.Authorizer
	if authorizer == nil {
		if p.usesBrokerAuth() {
			// Broker mode must never fall through to local desktop OAuth, even when
			// a BYO client_id is present in config from a previous local setup.
			base := strings.TrimSpace(p.BrokerBaseURL)
			if base == "" {
				return connection.Connection{}, fmt.Errorf("%w: set github.broker_base_url or SHIET_GITHUB_BROKER_BASE_URL", config.ErrGitHubBrokerConfig)
			}
			authorizer = &BrokerFlow{BaseURL: base, HTTPClient: p.HTTP}
		} else if strings.TrimSpace(p.Config.ClientID) != "" {
			authorizer = &oauth.Flow{Config: p.Config, Store: transientTokenStore{}}
		} else {
			return connection.Connection{}, errors.New("personal access token is required")
		}
	}
	result, err := authorizer.Authorize(ctx, "github")
	if err != nil {
		return connection.Connection{}, fmt.Errorf("authorize github: %w", err)
	}
	if strings.TrimSpace(result.Token.AccessToken) == "" {
		return connection.Connection{}, errors.New("GitHub OAuth returned an empty access token")
	}
	if p.usesBrokerAuth() {
		result.Token.CredentialSource = secrets.CredentialSourceBroker
	} else {
		result.Token.CredentialSource = secrets.CredentialSourceLocalOAuth
	}
	return p.connectWithToken(ctx, result.Token, result.Scopes, false)
}

type transientTokenStore struct{}

func (transientTokenStore) Get(string, string) (secrets.Token, error) {
	return secrets.Token{}, secrets.ErrNotFound
}
func (transientTokenStore) Set(string, string, secrets.Token) error { return nil }
func (transientTokenStore) Delete(string, string) error             { return nil }

func (p *Provider) connectWithToken(ctx context.Context, token secrets.Token, scopes []string, personalAccessToken bool) (connection.Connection, error) {
	if p.Store == nil {
		return connection.Connection{}, errors.New("token store is required")
	}
	if p.Registry == nil {
		return connection.Connection{}, errors.New("connection registry is required")
	}

	user, err := p.fetchUser(ctx, token.AccessToken, personalAccessToken)
	if err != nil {
		return connection.Connection{}, err
	}
	login := strings.TrimSpace(user.Login)
	if login == "" {
		return connection.Connection{}, errors.New("github user login is empty")
	}

	if strings.TrimSpace(token.TokenType) == "" {
		token.TokenType = "Bearer"
	}
	if err := p.Store.Set(providerGitHub, login, token); err != nil {
		return connection.Connection{}, fmt.Errorf("persist token: %w", err)
	}

	label := strings.TrimSpace(user.Name)
	if label == "" {
		label = login
	}

	conn, err := p.Registry.Upsert(ctx, connection.UpsertInput{
		Provider:     providerGitHub,
		AccountLabel: label,
		AccountID:    login,
		Scopes:       append([]string(nil), scopes...),
		Status:       connection.StatusConnected,
	})
	if err != nil {
		_ = p.Store.Delete(providerGitHub, login)
		return connection.Connection{}, err
	}

	if p.Queries != nil {
		if _, err := p.SyncRepos(ctx, login); err != nil {
			_ = p.Disconnect(ctx, login)
			return connection.Connection{}, fmt.Errorf("sync repos: %w", err)
		}
	}

	return conn, nil
}

// Disconnect removes the PAT from the keychain, clears synced repos, and
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
		if err := p.Queries.DeleteGitHubReposByAccount(ctx, accountID); err != nil {
			return fmt.Errorf("clear repos: %w", err)
		}
	}

	var accessToken string
	var credentialSource secrets.CredentialSource
	if p.Store != nil {
		if token, err := p.Store.Get(providerGitHub, accountID); err == nil {
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
		if err := p.Store.Delete(providerGitHub, accountID); err != nil && !errors.Is(err, secrets.ErrNotFound) {
			return fmt.Errorf("delete token: %w", err)
		}
	}
	return p.Registry.Disconnect(ctx, providerGitHub, accountID)
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
// PAT connect remains available independently.
func (p *Provider) OAuthAvailable() bool {
	if p.usesBrokerAuth() {
		return strings.TrimSpace(p.BrokerBaseURL) != ""
	}
	return strings.TrimSpace(p.Config.ClientID) != "" && strings.TrimSpace(p.Config.ClientSecret) != ""
}

// SyncRepos lists repositories visible to the connected account and upserts
// local github_repo rows. Existing selected flags are preserved on conflict.
func (p *Provider) SyncRepos(ctx context.Context, accountID string) ([]sqlc.GithubRepo, error) {
	accountID = strings.TrimSpace(accountID)
	if accountID == "" {
		return nil, errors.New("account_id is required")
	}
	if p.Queries == nil {
		return nil, errors.New("queries are required")
	}

	var out []sqlc.GithubRepo
	for page := 1; ; page++ {
		q := url.Values{}
		q.Set("per_page", strconv.Itoa(defaultPerPage))
		q.Set("page", strconv.Itoa(page))
		q.Set("affiliation", "owner,collaborator,organization_member")
		q.Set("sort", "full_name")

		var items []repoItem
		if err := p.getJSON(ctx, accountID, userReposPath, q, &items); err != nil {
			return nil, err
		}
		if len(items) == 0 {
			break
		}

		for _, item := range items {
			fullName := strings.TrimSpace(item.FullName)
			if fullName == "" {
				continue
			}
			name := strings.TrimSpace(item.Name)
			if name == "" {
				name = fullName
			}
			private := int64(0)
			if item.Private {
				private = 1
			}
			repo, err := p.Queries.UpsertGitHubRepo(ctx, sqlc.UpsertGitHubRepoParams{
				AccountID:  accountID,
				ExternalID: fullName,
				Name:       name,
				FullName:   fullName,
				Private:    private,
				Column6:    0, // new rows default unselected; existing selection preserved
			})
			if err != nil {
				return nil, fmt.Errorf("upsert repo %q: %w", fullName, err)
			}
			out = append(out, repo)
		}

		if len(items) < defaultPerPage {
			break
		}
	}
	return out, nil
}

// ListRepos returns all synced GitHub repos for all accounts.
func (p *Provider) ListRepos(ctx context.Context) ([]sqlc.GithubRepo, error) {
	if p.Queries == nil {
		return nil, errors.New("queries are required")
	}
	return p.Queries.ListGitHubRepos(ctx)
}

// SetRepoSelected toggles whether a repo is included as an evidence source.
func (p *Provider) SetRepoSelected(ctx context.Context, repoID int64, selected bool) error {
	if p.Queries == nil {
		return errors.New("queries are required")
	}
	sel := int64(0)
	if selected {
		sel = 1
	}
	return p.Queries.SetGitHubRepoSelected(ctx, sqlc.SetGitHubRepoSelectedParams{
		Selected: sel,
		ID:       repoID,
	})
}

func (p *Provider) fetchUser(ctx context.Context, accessToken string, personalAccessToken bool) (userResponse, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, p.baseURL()+userPath, nil)
	if err != nil {
		return userResponse{}, err
	}
	req.Header.Set("Authorization", "Bearer "+accessToken)
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("X-GitHub-Api-Version", "2022-11-28")

	client := p.HTTP
	if client == nil {
		client = http.DefaultClient
	}
	resp, err := client.Do(req)
	if err != nil {
		return userResponse{}, fmt.Errorf("github user: %w", err)
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return userResponse{}, fmt.Errorf("read github user: %w", err)
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		if personalAccessToken {
			return userResponse{}, fmt.Errorf("invalid personal access token (github api %s: %s)", userPath, strings.TrimSpace(string(body)))
		}
		return userResponse{}, fmt.Errorf("invalid GitHub OAuth token (github api %s: %s)", userPath, strings.TrimSpace(string(body)))
	}
	var user userResponse
	if err := json.Unmarshal(body, &user); err != nil {
		return userResponse{}, fmt.Errorf("decode github user: %w", err)
	}
	return user, nil
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
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("X-GitHub-Api-Version", "2022-11-28")

	resp, err := client.Do(ctx, req)
	if err != nil {
		return err
	}
	body, err := httpclient.ReadBody(resp)
	if err != nil {
		return err
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("github api %s: %s", path, strings.TrimSpace(string(body)))
	}
	if err := json.Unmarshal(body, dest); err != nil {
		return fmt.Errorf("decode github api response: %w", err)
	}
	return nil
}

func (p *Provider) httpClient(accountID string) *httpclient.Client {
	// GitHub OAuth App user tokens and PATs have no refresh exchange. On 401,
	// httpclient marks the connection needs_reauth without rewriting the token.
	return &httpclient.Client{
		Provider:  providerGitHub,
		AccountID: accountID,
		Config:    oauth.ProviderConfig{Provider: providerGitHub},
		Store:     p.Store,
		Registry:  p.Registry,
		HTTP:      p.HTTP,
		Refresher: githubNoRefresh{},
	}
}

type githubNoRefresh struct{}

func (githubNoRefresh) Refresh(context.Context, secrets.Token) (secrets.Token, error) {
	return secrets.Token{}, errors.New("GitHub OAuth App and PAT tokens do not support refresh")
}

func (p *Provider) baseURL() string {
	if strings.TrimSpace(p.BaseURL) != "" {
		return strings.TrimRight(p.BaseURL, "/")
	}
	return apiBaseURL
}

type userResponse struct {
	Login string `json:"login"`
	Name  string `json:"name"`
}

type repoItem struct {
	Name     string `json:"name"`
	FullName string `json:"full_name"`
	Private  bool   `json:"private"`
}
