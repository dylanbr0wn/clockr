package google

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/dylanbr0wn/shiet/internal/config"
	"github.com/dylanbr0wn/shiet/internal/db/sqlc"
	"github.com/dylanbr0wn/shiet/internal/integration/connection"
	"github.com/dylanbr0wn/shiet/internal/integration/httpclient"
	"github.com/dylanbr0wn/shiet/internal/integration/oauth"
	"github.com/dylanbr0wn/shiet/internal/integration/secrets"
	"github.com/dylanbr0wn/shiet/internal/service"
)

var (
	errCancelled = errors.New("cancelled event")

	// ErrBrokerUnavailable is returned when broker-mode Google auth cannot proceed
	// (broker down, unreachable, or connect flow not ready). Distinct from
	// config.ErrBrokerConfig (bad/missing broker settings) and
	// config.ErrLocalCredentials (missing BYO desktop credentials).
	ErrBrokerUnavailable = errors.New("Google OAuth broker is unavailable")
)

// Authorizer runs desktop OAuth and persists tokens.
type Authorizer interface {
	Authorize(ctx context.Context, accountID string) (oauth.Result, error)
}

// TokenRevoker asks the OAuth broker to revoke a Google refresh token.
type TokenRevoker interface {
	Revoke(ctx context.Context, refreshToken string) error
}

// Provider implements Google Calendar list + pull against the shared integration platform.
type Provider struct {
	Config        oauth.ProviderConfig
	AuthMode      string
	BrokerBaseURL string
	Store         secrets.TokenStore
	Registry      *connection.Registry
	Queries       *sqlc.Queries
	Authorizer    Authorizer
	Revoker       TokenRevoker // optional; defaults to BrokerFlow in broker mode
	BaseURL       string       // override for tests
}

// Connect runs OAuth, stores the token in the keychain, and upserts connection metadata.
func (p *Provider) Connect(ctx context.Context, accountID, accountLabel string) (connection.Connection, error) {
	accountID = strings.TrimSpace(accountID)
	if accountID == "" {
		return connection.Connection{}, errors.New("account_id is required")
	}
	if p.Store == nil {
		return connection.Connection{}, errors.New("token store is required")
	}
	if p.Registry == nil {
		return connection.Connection{}, errors.New("connection registry is required")
	}

	auth := p.Authorizer
	if auth == nil {
		if p.usesBrokerAuth() {
			base := strings.TrimSpace(p.BrokerBaseURL)
			if base == "" {
				return connection.Connection{}, fmt.Errorf("%w: set google.broker_base_url or SHIET_GOOGLE_BROKER_BASE_URL", config.ErrBrokerConfig)
			}
			auth = &BrokerFlow{BaseURL: base}
		} else {
			if strings.TrimSpace(p.Config.ClientID) == "" {
				return connection.Connection{}, fmt.Errorf("%w: set google.client_id or SHIET_GOOGLE_CLIENT_ID for local/BYO Google OAuth", config.ErrLocalCredentials)
			}
			auth = &oauth.Flow{Config: p.Config, Store: p.Store}
		}
	}

	result, err := auth.Authorize(ctx, accountID)
	if err != nil {
		return connection.Connection{}, fmt.Errorf("authorize google: %w", err)
	}
	if err := p.Store.Set(p.Config.Provider, accountID, result.Token); err != nil {
		return connection.Connection{}, fmt.Errorf("persist token: %w", err)
	}

	label := strings.TrimSpace(accountLabel)
	if label == "" {
		label = accountID
	}

	conn, err := p.Registry.Upsert(ctx, connection.UpsertInput{
		Provider:     p.Config.Provider,
		AccountLabel: label,
		AccountID:    accountID,
		Scopes:       result.Scopes,
		Status:       connection.StatusConnected,
	})
	if err != nil {
		return connection.Connection{}, err
	}

	if p.Queries != nil {
		if _, err := p.SyncCalendars(ctx, accountID); err != nil {
			return connection.Connection{}, fmt.Errorf("refresh calendars: %w", err)
		}
	}

	return conn, nil
}

// Disconnect removes tokens and the connection row. In broker mode, best-effort
// revoke runs first when a refresh token is present; local cleanup always
// proceeds so already-revoked or broker failures still disconnect cleanly.
func (p *Provider) Disconnect(ctx context.Context, accountID string) error {
	accountID = strings.TrimSpace(accountID)
	if accountID == "" {
		return errors.New("account_id is required")
	}
	if p.Registry == nil {
		return errors.New("connection registry is required")
	}

	var refreshToken string
	if p.Store != nil {
		if tok, err := p.Store.Get(p.Config.Provider, accountID); err == nil {
			refreshToken = strings.TrimSpace(tok.RefreshToken)
		}
	}

	if p.usesBrokerAuth() && refreshToken != "" {
		if revoker := p.tokenRevoker(); revoker != nil {
			_ = revoker.Revoke(ctx, refreshToken)
		}
	}

	if p.Store != nil {
		if err := p.Store.Delete(p.Config.Provider, accountID); err != nil && !errors.Is(err, secrets.ErrNotFound) {
			return fmt.Errorf("delete token: %w", err)
		}
	}
	return p.Registry.Disconnect(ctx, p.Config.Provider, accountID)
}

func (p *Provider) tokenRevoker() TokenRevoker {
	if p.Revoker != nil {
		return p.Revoker
	}
	base := strings.TrimSpace(p.BrokerBaseURL)
	if base == "" {
		return nil
	}
	return &BrokerFlow{BaseURL: base}
}

// SyncCalendars pulls calendarList.list and upserts calendar rows for provider=google.
func (p *Provider) SyncCalendars(ctx context.Context, accountID string) ([]sqlc.Calendar, error) {
	if p.Queries == nil {
		return nil, errors.New("queries are required")
	}

	var out []sqlc.Calendar
	pageToken := ""
	for {
		q := url.Values{}
		if pageToken != "" {
			q.Set("pageToken", pageToken)
		}
		var resp calendarListResponse
		if err := p.getJSON(ctx, accountID, calendarListPath, q, &resp); err != nil {
			return nil, err
		}

		for _, item := range resp.Items {
			name := strings.TrimSpace(item.Summary)
			if name == "" {
				name = item.ID
			}
			primary := int64(0)
			if item.Primary {
				primary = 1
			}
			cal, err := p.Queries.UpsertCalendar(ctx, sqlc.UpsertCalendarParams{
				Provider:   service.ProviderGoogle,
				ExternalID: item.ID,
				Name:       name,
				IsPrimary:  primary,
				Column5:    primary,
			})
			if err != nil {
				return nil, fmt.Errorf("upsert calendar %q: %w", item.ID, err)
			}
			out = append(out, cal)
		}

		pageToken = resp.NextPageToken
		if pageToken == "" {
			break
		}
	}
	return out, nil
}

// FetchEvents pulls events.list for the given calendars across a pay-period date range.
// periodStart and periodEnd are inclusive YYYY-MM-DD bounds.
func (p *Provider) FetchEvents(
	ctx context.Context,
	accountID string,
	periodStart, periodEnd string,
	calendars []sqlc.Calendar,
) ([]service.IncomingEvent, error) {
	if len(calendars) == 0 {
		return nil, nil
	}

	timeMin, timeMax, err := periodBounds(periodStart, periodEnd)
	if err != nil {
		return nil, err
	}

	var out []service.IncomingEvent
	for _, cal := range calendars {
		pageToken := ""
		for {
			q := url.Values{}
			q.Set("singleEvents", "true")
			q.Set("orderBy", "startTime")
			q.Set("timeMin", timeMin)
			q.Set("timeMax", timeMax)
			if pageToken != "" {
				q.Set("pageToken", pageToken)
			}

			path := fmt.Sprintf(eventsListPath, url.PathEscape(cal.ExternalID))
			var resp eventsListResponse
			if err := p.getJSON(ctx, accountID, path, q, &resp); err != nil {
				return nil, fmt.Errorf("list events for %q: %w", cal.ExternalID, err)
			}

			for _, item := range resp.Items {
				inc, err := mapEvent(cal.ID, item)
				if errors.Is(err, errCancelled) {
					continue
				}
				if err != nil {
					return nil, fmt.Errorf("map event %q: %w", item.ID, err)
				}
				out = append(out, inc)
			}

			pageToken = resp.NextPageToken
			if pageToken == "" {
				break
			}
		}
	}
	return out, nil
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

	resp, err := client.Do(ctx, req)
	if err != nil {
		return err
	}
	body, err := httpclient.ReadBody(resp)
	if err != nil {
		return err
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("google api %s: %s", path, strings.TrimSpace(string(body)))
	}
	if err := json.Unmarshal(body, dest); err != nil {
		return fmt.Errorf("decode google api response: %w", err)
	}
	return nil
}

func (p *Provider) httpClient(accountID string) *httpclient.Client {
	client := &httpclient.Client{
		Provider:  p.Config.Provider,
		AccountID: accountID,
		Config:    p.Config,
		Store:     p.Store,
		Registry:  p.Registry,
	}
	if p.usesBrokerAuth() {
		base := strings.TrimSpace(p.BrokerBaseURL)
		client.Refresher = &brokerTokenRefresher{
			flow:   &BrokerFlow{BaseURL: base},
			scopes: append([]string(nil), p.Config.Scopes...),
		}
	}
	return client
}

// brokerTokenRefresher adapts BrokerFlow to httpclient.TokenRefresher.
type brokerTokenRefresher struct {
	flow   *BrokerFlow
	scopes []string
}

func (r *brokerTokenRefresher) Refresh(ctx context.Context, current secrets.Token) (secrets.Token, error) {
	return r.flow.RefreshToken(ctx, current.RefreshToken, r.scopes)
}

func (p *Provider) baseURL() string {
	if strings.TrimSpace(p.BaseURL) != "" {
		return strings.TrimRight(p.BaseURL, "/")
	}
	return apiBaseURL
}

func (p *Provider) usesBrokerAuth() bool {
	mode := strings.TrimSpace(p.AuthMode)
	// Empty mode matches public-build default (broker). WireIntegrations always
	// sets AuthMode from config; this guard keeps tests/callers safe.
	if mode == "" {
		return true
	}
	return strings.EqualFold(mode, config.AuthModeBroker)
}

func periodBounds(startDate, endDate string) (timeMin, timeMax string, err error) {
	start, err := time.Parse("2006-01-02", startDate)
	if err != nil {
		return "", "", fmt.Errorf("parse period start %q: %w", startDate, err)
	}
	end, err := time.Parse("2006-01-02", endDate)
	if err != nil {
		return "", "", fmt.Errorf("parse period end %q: %w", endDate, err)
	}
	if end.Before(start) {
		return "", "", fmt.Errorf("period end %q before start %q", endDate, startDate)
	}
	return start.UTC().Format(time.RFC3339), end.Add(24 * time.Hour).UTC().Format(time.RFC3339), nil
}
