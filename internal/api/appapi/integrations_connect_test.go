package appapi_test

import (
	"context"
	"net/http"
	"testing"

	"connectrpc.com/connect"
	appv1 "github.com/dylanbr0wn/shiet/gen/shiet/app/v1"
	"github.com/dylanbr0wn/shiet/gen/shiet/app/v1/appv1connect"
	"github.com/dylanbr0wn/shiet/internal/api/appapi"
	"github.com/dylanbr0wn/shiet/internal/config"
	"github.com/dylanbr0wn/shiet/internal/integration/bitbucket"
	"github.com/dylanbr0wn/shiet/internal/integration/github"
	"github.com/dylanbr0wn/shiet/internal/integration/google"
	"github.com/dylanbr0wn/shiet/internal/integration/slack"
)

func TestIntegrationConnectRPCs(t *testing.T) {
	t.Parallel()
	handler := appapi.NewHandler(appapi.Dependencies{
		Google: &google.Provider{AuthMode: config.AuthModeBroker, BrokerBaseURL: "https://auth.shiet.app"},
		GitHub: &github.Provider{AuthMode: config.AuthModeBroker, BrokerBaseURL: "https://auth.shiet.app", Config: github.OAuthConfig("client-id", "client-secret")},
		Slack:     &slack.Provider{AuthMode: config.AuthModeBroker, BrokerBaseURL: "https://auth.shiet.app"},
		Bitbucket: &bitbucket.Provider{AuthMode: config.AuthModeBroker, BrokerBaseURL: "https://auth.shiet.app"},
	})
	client := appv1connect.NewIntegrationServiceClient(&http.Client{Transport: handlerTransport{handler: handler}}, "http://shiet.test")

	providers, err := client.ListIntegrationProviders(context.Background(), connect.NewRequest(&appv1.ListIntegrationProvidersRequest{}))
	if err != nil {
		t.Fatal(err)
	}
	if len(providers.Msg.Providers) != 4 {
		t.Fatalf("providers = %#v", providers.Msg.Providers)
	}
	byID := map[string]*appv1.IntegrationDescriptor{}
	for _, provider := range providers.Msg.Providers {
		byID[provider.Id] = provider
	}
	if googleDesc := byID["google"]; googleDesc == nil || googleDesc.Kind != appv1.IntegrationKind_INTEGRATION_KIND_CALENDAR_SOURCE || !googleDesc.Connect.NeedsAccountHint || googleDesc.Connect.SupportsPat || !googleDesc.Connect.OauthAvailable {
		t.Fatalf("google descriptor: %#v", googleDesc)
	}
	if githubDesc := byID["github"]; githubDesc == nil || githubDesc.Kind != appv1.IntegrationKind_INTEGRATION_KIND_ACTIVITY_EVIDENCE || !githubDesc.Connect.SupportsPat || !githubDesc.Connect.OauthAvailable {
		t.Fatalf("github descriptor: %#v", githubDesc)
	}
	if bitbucketDesc := byID["bitbucket"]; bitbucketDesc == nil || bitbucketDesc.Kind != appv1.IntegrationKind_INTEGRATION_KIND_ACTIVITY_EVIDENCE || bitbucketDesc.Connect.SupportsPat || !bitbucketDesc.Connect.OauthAvailable {
		t.Fatalf("bitbucket descriptor: %#v", bitbucketDesc)
	}

	googleAuth, err := client.GetIntegrationAuthStatus(context.Background(), connect.NewRequest(&appv1.GetIntegrationAuthStatusRequest{Provider: "google"}))
	if err != nil || googleAuth.Msg.Mode != config.AuthModeBroker || googleAuth.Msg.BrokerBaseUrl != "https://auth.shiet.app" || !googleAuth.Msg.OauthAvailable || googleAuth.Msg.SupportsPat {
		t.Fatalf("google auth = %#v err=%v", googleAuth, err)
	}
	githubAuth, err := client.GetIntegrationAuthStatus(context.Background(), connect.NewRequest(&appv1.GetIntegrationAuthStatusRequest{Provider: "github"}))
	if err != nil || !githubAuth.Msg.SupportsPat || !githubAuth.Msg.OauthAvailable {
		t.Fatalf("github auth = %#v err=%v", githubAuth, err)
	}

	bitbucketAuth, err := client.GetIntegrationAuthStatus(context.Background(), connect.NewRequest(&appv1.GetIntegrationAuthStatusRequest{Provider: "bitbucket"}))
	if err != nil || bitbucketAuth.Msg.Mode != config.AuthModeBroker || !bitbucketAuth.Msg.OauthAvailable || bitbucketAuth.Msg.SupportsPat {
		t.Fatalf("bitbucket auth = %#v err=%v", bitbucketAuth, err)
	}
	_, err = client.DisconnectIntegration(context.Background(), connect.NewRequest(&appv1.DisconnectIntegrationRequest{Provider: "github"}))
	if connect.CodeOf(err) != connect.CodeInvalidArgument {
		t.Fatalf("disconnect missing account_id code = %v", connect.CodeOf(err))
	}
}

func TestIntegrationConnectRPCsUnimplementedWithoutProviders(t *testing.T) {
	t.Parallel()
	handler := appapi.NewHandler(appapi.Dependencies{})
	client := appv1connect.NewIntegrationServiceClient(&http.Client{Transport: handlerTransport{handler: handler}}, "http://shiet.test")

	providers, err := client.ListIntegrationProviders(context.Background(), connect.NewRequest(&appv1.ListIntegrationProvidersRequest{}))
	if err != nil || len(providers.Msg.Providers) != 4 {
		t.Fatalf("providers = %#v err=%v", providers, err)
	}
	for _, provider := range providers.Msg.Providers {
		if provider.Connect.OauthAvailable {
			t.Fatalf("oauth should be unavailable without providers: %#v", provider)
		}
	}

	_, err = client.GetIntegrationAuthStatus(context.Background(), connect.NewRequest(&appv1.GetIntegrationAuthStatusRequest{Provider: "google"}))
	if connect.CodeOf(err) != connect.CodeUnimplemented {
		t.Fatalf("google auth code = %v", connect.CodeOf(err))
	}
	_, err = client.ConnectIntegration(context.Background(), connect.NewRequest(&appv1.ConnectIntegrationRequest{Provider: "slack"}))
	if connect.CodeOf(err) != connect.CodeUnimplemented {
		t.Fatalf("slack connect code = %v", connect.CodeOf(err))
	}
}
