package oauth

import (
	"fmt"
	"net/url"
	"strings"

	"golang.org/x/oauth2"
)

// ClientCredentials are runtime OAuth client values injected by the desktop
// (BYO/local) or broker (environment-backed confidential client). They must
// never be embedded in a Provider descriptor.
type ClientCredentials struct {
	ClientID     string
	ClientSecret string
}

// AuthURLParam is a provider-specific authorization query parameter.
type AuthURLParam struct {
	Key   string
	Value string
}

// Capabilities declares which post-authorization token operations a provider
// supports through the broker or local adapters.
type Capabilities struct {
	Refresh bool
	Revoke  bool
}

// Provider is static, shareable OAuth protocol metadata safe to compile into
// both the desktop app and the broker binary. It never carries secrets or
// deployment-specific credential values.
type Provider struct {
	ID            string
	DisplayName   string
	AuthURL       string
	TokenURL      string
	RevokeURL     string
	AuthStyle     oauth2.AuthStyle
	DefaultScopes []string
	// AuthURLHost and AuthURLPaths constrain broker-returned authorization URLs
	// before the desktop opens them.
	AuthURLHost  string
	AuthURLPaths []string
	// AuthURLParams are appended when building the provider authorization URL
	// (for example Google access_type=offline and prompt=consent).
	AuthURLParams []AuthURLParam
	// AcceptJSON asks token requests to send Accept: application/json (GitHub).
	AcceptJSON bool
	// ScopeSplitComma treats commas as scope separators when parsing token scope.
	ScopeSplitComma bool
	Capabilities    Capabilities
}

// ProviderConfig builds the local/BYO oauth.Flow config from static metadata
// plus runtime credentials.
func (p Provider) ProviderConfig(creds ClientCredentials) ProviderConfig {
	return ProviderConfig{
		Provider:     p.ID,
		ClientID:     creds.ClientID,
		ClientSecret: creds.ClientSecret,
		AuthURL:      p.AuthURL,
		TokenURL:     p.TokenURL,
		AuthStyle:    p.AuthStyle,
		Scopes:       append([]string(nil), p.DefaultScopes...),
	}
}

// ValidateAuthorizationURL checks that a broker-returned auth URL matches this
// provider's expected host and path allowlist.
func (p Provider) ValidateAuthorizationURL(raw string) error {
	u, err := url.Parse(raw)
	if err != nil || u.Scheme != "https" {
		return fmt.Errorf("%s authorization URL must use https", p.DisplayName)
	}
	if u.Host != p.AuthURLHost {
		return fmt.Errorf("%s authorization URL host must be %s", p.DisplayName, p.AuthURLHost)
	}
	for _, path := range p.AuthURLPaths {
		if u.Path == path {
			return nil
		}
	}
	return fmt.Errorf("%s authorization URL path is not allowed", p.DisplayName)
}

// SplitScopes parses a provider token scope string.
func (p Provider) SplitScopes(raw string) []string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil
	}
	if p.ScopeSplitComma {
		return strings.FieldsFunc(raw, func(r rune) bool { return r == ',' || r == ' ' })
	}
	return strings.Fields(raw)
}

// Lookup returns the registered provider descriptor for id.
func Lookup(id string) (Provider, bool) {
	p, ok := registry[strings.ToLower(strings.TrimSpace(id))]
	return p, ok
}

// MustLookup returns the provider or panics. Intended for package init wiring
// where a missing provider is a programmer error.
func MustLookup(id string) Provider {
	p, ok := Lookup(id)
	if !ok {
		panic("oauth: unknown provider " + id)
	}
	return p
}

// All returns registered providers in stable id order.
func All() []Provider {
	out := make([]Provider, 0, len(registryOrder))
	for _, id := range registryOrder {
		out = append(out, registry[id])
	}
	return out
}

var (
	registry      = map[string]Provider{}
	registryOrder []string
)

func register(p Provider) {
	id := strings.ToLower(strings.TrimSpace(p.ID))
	if id == "" {
		panic("oauth: provider id is required")
	}
	if _, exists := registry[id]; exists {
		panic("oauth: duplicate provider " + id)
	}
	p.ID = id
	registry[id] = p
	registryOrder = append(registryOrder, id)
}
