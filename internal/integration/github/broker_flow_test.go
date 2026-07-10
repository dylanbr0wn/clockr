package github_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	brokerconfig "github.com/dylanbr0wn/shiet/internal/broker/config"
	"github.com/dylanbr0wn/shiet/internal/broker/httpapi"
	"github.com/dylanbr0wn/shiet/internal/integration/github"
)

func TestBrokerFlowRevokeThroughConnect(t *testing.T) {
	t.Parallel()

	provider := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodDelete || r.URL.Path != "/applications/github-client-id/token" {
			t.Fatalf("provider request: %s %s", r.Method, r.URL.Path)
		}
		w.WriteHeader(http.StatusNoContent)
	}))
	t.Cleanup(provider.Close)

	brokerServer := httpapi.Server{
		Config:     brokerconfig.Config{GitHubClientID: "github-client-id", GitHubClientSecret: "github-client-secret"},
		HTTPClient: provider.Client(), GitHubRevokeURL: provider.URL,
	}
	broker := httptest.NewServer(brokerServer.Handler())
	t.Cleanup(broker.Close)

	flow := &github.BrokerFlow{BaseURL: broker.URL, HTTPClient: broker.Client()}
	if err := flow.Revoke(context.Background(), "github-access"); err != nil {
		t.Fatal(err)
	}
}
