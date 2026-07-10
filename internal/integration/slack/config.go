package slack

import (
	"github.com/dylanbr0wn/shiet/internal/config"
	"github.com/dylanbr0wn/shiet/internal/integration/oauth"
	"github.com/dylanbr0wn/shiet/internal/service"
)

// AuthSettings carries Slack auth mode into the provider.
type AuthSettings struct {
	Mode          string
	BrokerBaseURL string
	ClientID      string
	ClientSecret  string
}

func OAuthConfig(clientID, clientSecret string) oauth.ProviderConfig {
	desc := oauth.MustLookup(oauth.ProviderSlack)
	cfg := desc.ProviderConfig(oauth.ClientCredentials{
		ClientID:     clientID,
		ClientSecret: clientSecret,
	})
	cfg.Provider = service.ProviderSlack
	return cfg
}

func AuthSettingsFromConfig(cfg config.Config) AuthSettings {
	settings := AuthSettings{
		Mode:          cfg.Slack.AuthMode,
		BrokerBaseURL: cfg.Slack.BrokerBaseURL,
	}
	// Broker mode must not carry desktop BYO OAuth credentials into the provider.
	if cfg.UsesSlackBrokerAuth() {
		return settings
	}
	settings.ClientID = cfg.Slack.ClientID
	settings.ClientSecret = cfg.Slack.ClientSecret
	return settings
}
