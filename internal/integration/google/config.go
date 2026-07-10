package google

import (
	"strings"

	"github.com/dylanbr0wn/shiet/internal/config"
	"github.com/dylanbr0wn/shiet/internal/integration/oauth"
	"github.com/dylanbr0wn/shiet/internal/service"
)

const (
	apiBaseURL        = "https://www.googleapis.com/calendar/v3"
	scopeCalendarRead = "https://www.googleapis.com/auth/calendar.readonly"
	calendarListPath  = "/users/me/calendarList"
	eventsListPath    = "/calendars/%s/events"

	// Local/BYO desktop OAuth fallbacks stay empty on purpose. Public builds use
	// broker mode and must not embed a shared Google client_secret.
	defaultDesktopClientID     = ""
	defaultDesktopClientSecret = ""
)

// AuthSettings carries Google Calendar auth mode from app config into the
// provider. Broker mode never requires a desktop client_secret.
type AuthSettings struct {
	Mode          string
	BrokerBaseURL string
	ClientID      string
	ClientSecret  string
}

// AuthStatus is the read-only view of Google auth mode for Settings UI.
// It never includes client secrets or token material.
type AuthStatus struct {
	Mode          string `json:"mode"`          // "broker" | "local"
	BrokerBaseURL string `json:"brokerBaseUrl"` // set in broker mode
}

// AuthSettingsFromConfig maps runtime config into provider auth settings.
// In broker mode the desktop client_secret is intentionally not copied.
func AuthSettingsFromConfig(cfg config.Config) AuthSettings {
	settings := AuthSettings{
		Mode:          cfg.Google.AuthMode,
		BrokerBaseURL: cfg.Google.BrokerBaseURL,
		ClientID:      cfg.Google.ClientID,
	}
	if cfg.UsesBrokerAuth() {
		return settings
	}
	settings.ClientSecret = cfg.Google.ClientSecret
	return settings
}

// Status returns the active Google auth mode for display. Nil-safe: defaults to
// broker when the provider is unset.
func (p *Provider) Status() AuthStatus {
	if p == nil {
		return AuthStatus{Mode: config.AuthModeBroker}
	}
	mode := strings.ToLower(strings.TrimSpace(p.AuthMode))
	if mode == "" {
		mode = config.AuthModeBroker
	}
	status := AuthStatus{Mode: mode}
	if strings.EqualFold(mode, config.AuthModeBroker) {
		status.BrokerBaseURL = strings.TrimSpace(p.BrokerBaseURL)
	}
	return status
}

// OAuthConfig builds reusable Google OAuth settings for local/BYO desktop OAuth.
// Google desktop clients are public OAuth clients: clientSecret may be needed by
// Google's token endpoint, but must not be treated as a confidential secret.
// Broker mode must not call this with a shared shipped secret.
func OAuthConfig(clientID, clientSecret string) oauth.ProviderConfig {
	if clientID == "" {
		clientID = defaultDesktopClientID
	}
	if clientSecret == "" {
		clientSecret = defaultDesktopClientSecret
	}
	desc := oauth.MustLookup(oauth.ProviderGoogle)
	cfg := desc.ProviderConfig(oauth.ClientCredentials{
		ClientID:     clientID,
		ClientSecret: clientSecret,
	})
	cfg.Provider = service.ProviderGoogle
	return cfg
}
