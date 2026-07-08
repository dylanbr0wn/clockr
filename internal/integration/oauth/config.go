package oauth

import (
	"golang.org/x/oauth2"
)

// ProviderConfig holds reusable OAuth settings for an integration provider.
type ProviderConfig struct {
	Provider     string
	ClientID     string
	ClientSecret string
	AuthURL      string
	TokenURL     string
	AuthStyle    oauth2.AuthStyle
	Scopes       []string
}

// OAuth2Config converts the provider config to an oauth2.Config without a redirect URL.
// Callers must set RedirectURL on the returned config after choosing a loopback port.
func (c ProviderConfig) OAuth2Config(redirectURL string) oauth2.Config {
	return oauth2.Config{
		ClientID:     c.ClientID,
		ClientSecret: c.ClientSecret,
		RedirectURL:  redirectURL,
		Scopes:       append([]string(nil), c.Scopes...),
		Endpoint: oauth2.Endpoint{
			AuthURL:   c.AuthURL,
			TokenURL:  c.TokenURL,
			AuthStyle: c.AuthStyle,
		},
	}
}
