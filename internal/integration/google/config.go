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

	// Fill these with Clockr's Google Desktop OAuth credentials once the Google
	// Cloud OAuth consent/app setup is ready. The client secret is part of
	// Google's desktop credential bundle, but desktop apps cannot keep it
	// confidential once shipped.
	defaultDesktopClientID     = ""
	defaultDesktopClientSecret = ""
)

// OAuthConfig builds reusable Google OAuth settings for the calendar provider.
// Google desktop clients are public OAuth clients: clientSecret may be needed by
// Google's token endpoint, but must not be treated as a confidential secret.
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
