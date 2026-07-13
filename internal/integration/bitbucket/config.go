package bitbucket

import (
	"github.com/dylanbr0wn/shiet/internal/config"
	"github.com/dylanbr0wn/shiet/internal/integration/oauth"
	"github.com/dylanbr0wn/shiet/internal/service"
)

// AuthSettings carries Bitbucket auth mode into the provider.
type AuthSettings struct {
	Mode          string
	BrokerBaseURL string
	ClientID      string
	ClientSecret  string
}

func OAuthConfig(clientID, clientSecret string) oauth.ProviderConfig {
	desc := oauth.MustLookup(oauth.ProviderBitbucket)
	cfg := desc.ProviderConfig(oauth.ClientCredentials{
		ClientID:     clientID,
		ClientSecret: clientSecret,
	})
	cfg.Provider = service.ProviderBitbucket
	return cfg
}

func AuthSettingsFromConfig(cfg config.Config) AuthSettings {
	settings := AuthSettings{
		Mode:          cfg.Bitbucket.AuthMode,
		BrokerBaseURL: cfg.Bitbucket.BrokerBaseURL,
	}
	if cfg.UsesBitbucketBrokerAuth() {
		return settings
	}
	settings.ClientID = cfg.Bitbucket.ClientID
	settings.ClientSecret = cfg.Bitbucket.ClientSecret
	return settings
}
