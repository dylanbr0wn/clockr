package oauth

import (
	"golang.org/x/oauth2"
)

const (
	ProviderGoogle = "google"
	ProviderGitHub = "github"

	googleCalendarReadScope = "https://www.googleapis.com/auth/calendar.readonly"
)

func init() {
	register(Provider{
		ID:            ProviderGoogle,
		DisplayName:   "Google",
		AuthURL:       "https://accounts.google.com/o/oauth2/v2/auth",
		TokenURL:      "https://oauth2.googleapis.com/token",
		RevokeURL:     "https://oauth2.googleapis.com/revoke",
		AuthStyle:     oauth2.AuthStyleInParams,
		DefaultScopes: []string{googleCalendarReadScope},
		AuthURLHost:   "accounts.google.com",
		AuthURLPaths:  []string{"/o/oauth2/v2/auth", "/o/oauth2/auth"},
		AuthURLParams: []AuthURLParam{
			{Key: "access_type", Value: "offline"},
			{Key: "prompt", Value: "consent"},
		},
		Capabilities: Capabilities{Refresh: true, Revoke: true},
	})
	register(Provider{
		ID:              ProviderGitHub,
		DisplayName:     "GitHub",
		AuthURL:         "https://github.com/login/oauth/authorize",
		TokenURL:        "https://github.com/login/oauth/access_token",
		RevokeURL:       "https://api.github.com",
		AuthStyle:       oauth2.AuthStyleInParams,
		DefaultScopes:   []string{"repo"},
		AuthURLHost:     "github.com",
		AuthURLPaths:    []string{"/login/oauth/authorize"},
		AcceptJSON:      true,
		ScopeSplitComma: true,
		Capabilities:    Capabilities{Refresh: false, Revoke: true},
	})
}
