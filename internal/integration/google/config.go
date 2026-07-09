package google

import (
	"github.com/dylanbr0wn/shiet/internal/config"
	"github.com/dylanbr0wn/shiet/internal/integration/oauth"
	"github.com/dylanbr0wn/shiet/internal/service"
	"golang.org/x/oauth2"
)

const (
	apiBaseURL        = "https://www.googleapis.com/calendar/v3"
	scopeCalendarRead = "https://www.googleapis.com/auth/calendar.readonly"
	authURL           = "https://accounts.google.com/o/oauth2/v2/auth"
	tokenURL          = "https://oauth2.googleapis.com/token"
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
	return oauth.ProviderConfig{
		Provider:     service.ProviderGoogle,
		ClientID:     clientID,
		ClientSecret: clientSecret,
		AuthURL:      authURL,
		TokenURL:     tokenURL,
		AuthStyle:    oauth2.AuthStyleInParams,
		Scopes:       []string{scopeCalendarRead},
	}
}
