package github

import (
	"github.com/dylanbr0wn/shiet/internal/config"
	"github.com/dylanbr0wn/shiet/internal/integration/oauth"
	"github.com/dylanbr0wn/shiet/internal/service"
)

// AuthSettings carries GitHub auth mode into the provider. Broker mode never
// copies a desktop client secret; local mode retains PAT/BYO configuration.
type AuthSettings struct {
	Mode          string
	BrokerBaseURL string
	ClientID      string
	ClientSecret  string
}

func OAuthConfig(clientID, clientSecret string) oauth.ProviderConfig {
	desc := oauth.MustLookup(oauth.ProviderGitHub)
	cfg := desc.ProviderConfig(oauth.ClientCredentials{
		ClientID:     clientID,
		ClientSecret: clientSecret,
	})
	cfg.Provider = service.ProviderGitHub
	return cfg
}

func AuthSettingsFromConfig(cfg config.Config) AuthSettings {
	settings := AuthSettings{
		Mode:          cfg.GitHub.AuthMode,
		BrokerBaseURL: cfg.GitHub.BrokerBaseURL,
	}
	// Broker mode must not carry desktop BYO OAuth credentials into the provider;
	// the hosted broker owns the shared GitHub OAuth App secret.
	if cfg.UsesGitHubBrokerAuth() {
		return settings
	}
	settings.ClientID = cfg.GitHub.ClientID
	settings.ClientSecret = cfg.GitHub.ClientSecret
	return settings
}
