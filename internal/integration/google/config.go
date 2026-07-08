package google

import (
	"github.com/dylanbr0wn/clockr/internal/integration/oauth"
	"github.com/dylanbr0wn/clockr/internal/service"
	"golang.org/x/oauth2"
)

const (
	apiBaseURL        = "https://www.googleapis.com/calendar/v3"
	scopeCalendarRead = "https://www.googleapis.com/auth/calendar.readonly"
	authURL           = "https://accounts.google.com/o/oauth2/v2/auth"
	tokenURL          = "https://oauth2.googleapis.com/token"
	calendarListPath  = "/users/me/calendarList"
	eventsListPath    = "/calendars/%s/events"

	// Fill this with Clockr's public Google Desktop OAuth client ID once the
	// Google Cloud OAuth consent/app setup is ready.
	defaultDesktopClientID = ""
)

// OAuthConfig builds reusable Google OAuth settings for the calendar provider.
// Google desktop clients are public OAuth clients, so no shared client secret is
// included or required.
func OAuthConfig(clientID string) oauth.ProviderConfig {
	if clientID == "" {
		clientID = defaultDesktopClientID
	}
	return oauth.ProviderConfig{
		Provider:  service.ProviderGoogle,
		ClientID:  clientID,
		AuthURL:   authURL,
		TokenURL:  tokenURL,
		AuthStyle: oauth2.AuthStyleInParams,
		Scopes:    []string{scopeCalendarRead},
	}
}
