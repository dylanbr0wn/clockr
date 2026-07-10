package oauth

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"golang.org/x/oauth2"
)

const maxTokenResponseBytes = 1 << 20

// TokenResponse is the normalized result of an authorization-code exchange.
type TokenResponse struct {
	AccessToken  string
	RefreshToken string
	TokenType    string
	ExpiresIn    int64
	Expiry       time.Time
	Scope        string
	Raw          map[string]any
}

// ExchangeError is returned when a provider token endpoint rejects the request.
type ExchangeError struct {
	Code        string
	Description string
	Status      int
}

func (e *ExchangeError) Error() string {
	if e.Description != "" {
		return fmt.Sprintf("oauth exchange: %s: %s", e.Code, e.Description)
	}
	if e.Code != "" {
		return fmt.Sprintf("oauth exchange: %s", e.Code)
	}
	return fmt.Sprintf("oauth exchange failed with status %d", e.Status)
}

// BuildAuthorizationURL constructs the provider authorization URL with PKCE.
// Credentials are supplied by the caller; the provider descriptor never holds them.
func BuildAuthorizationURL(p Provider, creds ClientCredentials, redirectURL, state, codeChallenge string, scopes []string) (string, error) {
	if strings.TrimSpace(creds.ClientID) == "" {
		return "", errors.New("client_id is required")
	}
	if strings.TrimSpace(redirectURL) == "" {
		return "", errors.New("redirect_uri is required")
	}
	if strings.TrimSpace(state) == "" {
		return "", errors.New("state is required")
	}
	if strings.TrimSpace(codeChallenge) == "" {
		return "", errors.New("code_challenge is required")
	}
	if len(scopes) == 0 {
		scopes = append([]string(nil), p.DefaultScopes...)
	}
	scopeParam := strings.TrimSpace(p.ScopeParam)
	if scopeParam == "" {
		scopeParam = "scope"
	}
	scopeSep := " "
	if p.ScopeSplitComma {
		scopeSep = ","
	}
	cfg := oauth2.Config{
		ClientID:    creds.ClientID,
		RedirectURL: redirectURL,
		Endpoint: oauth2.Endpoint{
			AuthURL:   p.AuthURL,
			TokenURL:  p.TokenURL,
			AuthStyle: p.AuthStyle,
		},
	}
	opts := []oauth2.AuthCodeOption{
		oauth2.SetAuthURLParam("code_challenge", codeChallenge),
		oauth2.SetAuthURLParam("code_challenge_method", "S256"),
		oauth2.SetAuthURLParam(scopeParam, strings.Join(scopes, scopeSep)),
	}
	for _, param := range p.AuthURLParams {
		opts = append(opts, oauth2.SetAuthURLParam(param.Key, param.Value))
	}
	return cfg.AuthCodeURL(state, opts...), nil
}

// ExchangeOptions configures authorization-code exchange execution.
type ExchangeOptions struct {
	HTTPClient *http.Client
	// TokenURL overrides the provider token endpoint (tests / local stubs).
	TokenURL string
}

// ExchangeAuthorizationCode exchanges an authorization code for tokens using
// the provider's token endpoint semantics. Local/BYO and broker modes both call
// this with their respective runtime credentials.
func ExchangeAuthorizationCode(ctx context.Context, p Provider, creds ClientCredentials, redirectURL, code, codeVerifier string, opts ExchangeOptions) (TokenResponse, error) {
	if strings.TrimSpace(creds.ClientID) == "" {
		return TokenResponse{}, errors.New("client_id is required")
	}
	if strings.TrimSpace(redirectURL) == "" {
		return TokenResponse{}, errors.New("redirect_uri is required")
	}
	if strings.TrimSpace(code) == "" {
		return TokenResponse{}, errors.New("code is required")
	}
	if strings.TrimSpace(codeVerifier) == "" {
		return TokenResponse{}, errors.New("code_verifier is required")
	}

	tokenURL := strings.TrimSpace(opts.TokenURL)
	if tokenURL == "" {
		tokenURL = p.TokenURL
	}

	form := url.Values{}
	form.Set("grant_type", "authorization_code")
	form.Set("code", code)
	form.Set("redirect_uri", redirectURL)
	form.Set("code_verifier", codeVerifier)
	useHeaderAuth := p.AuthStyle == oauth2.AuthStyleInHeader
	if !useHeaderAuth {
		form.Set("client_id", creds.ClientID)
		if strings.TrimSpace(creds.ClientSecret) != "" {
			form.Set("client_secret", creds.ClientSecret)
		}
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, tokenURL, strings.NewReader(form.Encode()))
	if err != nil {
		return TokenResponse{}, err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	if p.AcceptJSON {
		req.Header.Set("Accept", "application/json")
	}
	if useHeaderAuth {
		req.SetBasicAuth(url.QueryEscape(creds.ClientID), url.QueryEscape(creds.ClientSecret))
	}

	client := opts.HTTPClient
	if client == nil {
		client = http.DefaultClient
	}
	resp, err := client.Do(req)
	if err != nil {
		return TokenResponse{}, err
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(io.LimitReader(resp.Body, maxTokenResponseBytes))
	if err != nil {
		return TokenResponse{}, err
	}

	tok, err := decodeTokenResponse(body)
	if err != nil {
		return TokenResponse{}, err
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 || tok.AccessToken == "" || tok.errorCode() != "" {
		return TokenResponse{}, &ExchangeError{
			Code:        firstNonEmpty(tok.errorCode(), "token_request_failed"),
			Description: tok.ErrorDescription,
			Status:      resp.StatusCode,
		}
	}
	if tok.TokenType == "" {
		tok.TokenType = "Bearer"
	}
	out := TokenResponse{
		AccessToken:  tok.AccessToken,
		RefreshToken: tok.RefreshToken,
		TokenType:    tok.TokenType,
		ExpiresIn:    tok.ExpiresIn,
		Scope:        tok.Scope,
		Raw:          tok.Raw,
	}
	if tok.ExpiresIn > 0 {
		out.Expiry = time.Now().Add(time.Duration(tok.ExpiresIn) * time.Second)
	}
	return out, nil
}

type rawTokenResponse struct {
	OK               bool           `json:"ok"`
	AccessToken      string         `json:"access_token"`
	RefreshToken     string         `json:"refresh_token"`
	TokenType        string         `json:"token_type"`
	ExpiresIn        int64          `json:"expires_in"`
	Scope            string         `json:"scope"`
	Error            string         `json:"error"`
	ErrorDescription string         `json:"error_description"`
	Raw              map[string]any `json:"-"`
}

func (t rawTokenResponse) errorCode() string {
	if strings.TrimSpace(t.Error) != "" {
		return strings.TrimSpace(t.Error)
	}
	if t.OK == false && t.AccessToken == "" {
		return "token_request_failed"
	}
	return ""
}

func decodeTokenResponse(body []byte) (rawTokenResponse, error) {
	var tok rawTokenResponse
	var raw map[string]any
	if err := json.Unmarshal(body, &tok); err == nil {
		_ = json.Unmarshal(body, &raw)
		tok.Raw = raw
		if authedUser, ok := raw["authed_user"].(map[string]any); ok {
			if tok.AccessToken == "" {
				if accessToken, ok := authedUser["access_token"].(string); ok {
					tok.AccessToken = accessToken
				}
			}
			if tok.Scope == "" {
				if scope, ok := authedUser["scope"].(string); ok {
					tok.Scope = scope
				}
			}
		}
		if tok.AccessToken != "" || tok.Error != "" || tok.OK || raw["authed_user"] != nil {
			return tok, nil
		}
	}
	// Providers may return form-urlencoded token responses.
	vals, err := url.ParseQuery(string(body))
	if err != nil {
		return rawTokenResponse{}, fmt.Errorf("decode token response: %w", err)
	}
	tok = rawTokenResponse{
		AccessToken:      vals.Get("access_token"),
		RefreshToken:     vals.Get("refresh_token"),
		TokenType:        vals.Get("token_type"),
		Scope:            vals.Get("scope"),
		Error:            vals.Get("error"),
		ErrorDescription: vals.Get("error_description"),
	}
	if v := vals.Get("expires_in"); v != "" {
		var n int64
		_, _ = fmt.Sscan(v, &n)
		tok.ExpiresIn = n
	}
	return tok, nil
}

func firstNonEmpty(values ...string) string {
	for _, v := range values {
		if strings.TrimSpace(v) != "" {
			return v
		}
	}
	return ""
}
