package github

import (
	"github.com/dylanbr0wn/shiet/internal/config"
	"github.com/dylanbr0wn/shiet/internal/integration/oauth"
	"github.com/dylanbr0wn/shiet/internal/service"
	"golang.org/x/oauth2"
)

const (
	githubAuthURL  = "https://github.com/login/oauth/authorize"
	githubTokenURL = "https://github.com/login/oauth/access_token"
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
	return oauth.ProviderConfig{
		Provider:     service.ProviderGitHub,
		ClientID:     clientID,
		ClientSecret: clientSecret,
		AuthURL:      githubAuthURL,
		TokenURL:     githubTokenURL,
		AuthStyle:    oauth2.AuthStyleInParams,
		Scopes:       []string{"repo"},
	}
}

func AuthSettingsFromConfig(cfg config.Config) AuthSettings {
	settings := AuthSettings{
		Mode:          cfg.GitHub.AuthMode,
		BrokerBaseURL: cfg.GitHub.BrokerBaseURL,
		ClientID:      cfg.GitHub.ClientID,
	}
	if !cfg.UsesGitHubBrokerAuth() {
		settings.ClientSecret = cfg.GitHub.ClientSecret
	}
	return settings
}
