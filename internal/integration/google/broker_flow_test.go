package google_test

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"connectrpc.com/connect"
	brokerv1 "github.com/dylanbr0wn/shiet/gen/shiet/broker/v1"
	"github.com/dylanbr0wn/shiet/gen/shiet/broker/v1/brokerv1connect"
	brokerconfig "github.com/dylanbr0wn/shiet/internal/broker/config"
	"github.com/dylanbr0wn/shiet/internal/broker/httpapi"
	"github.com/dylanbr0wn/shiet/internal/integration/google"
	"google.golang.org/protobuf/types/known/timestamppb"
)

func TestBrokerFlowAuthorizeThroughConnect(t *testing.T) {
	t.Parallel()

	stub := &googleConnectBroker{t: t}
	_, handler := brokerv1connect.NewOAuthBrokerServiceHandler(stub)
	server := httptest.NewServer(handler)
	t.Cleanup(server.Close)

	flow := &google.BrokerFlow{
		BaseURL: server.URL, HTTPClient: server.Client(), OpenURL: func(string) error { return nil }, AppVersion: "1.2.3", Platform: "test",
	}
	result, err := flow.Authorize(context.Background(), "user@example.com")
	if err != nil {
		t.Fatal(err)
	}
	if result.Token.AccessToken != "google-access" || result.Token.RefreshToken != "google-refresh" || result.Provider != "google" {
		t.Fatalf("result = %+v", result)
	}
}

func TestBrokerFlowRefreshAndRevokeThroughConnect(t *testing.T) {
	t.Parallel()

	provider := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/token":
			_ = json.NewEncoder(w).Encode(map[string]any{"access_token": "connect-access", "token_type": "Bearer", "expires_in": 3600})
		case "/revoke":
			w.WriteHeader(http.StatusOK)
		default:
			http.NotFound(w, r)
		}
	}))
	t.Cleanup(provider.Close)

	brokerServer := httpapi.Server{
		Config:     brokerconfig.Config{GoogleClientID: "google-client-id", GoogleClientSecret: "google-client-secret"},
		HTTPClient: provider.Client(), GoogleTokenURL: provider.URL + "/token", GoogleRevokeURL: provider.URL + "/revoke",
	}
	broker := httptest.NewServer(brokerServer.Handler())
	t.Cleanup(broker.Close)

	flow := &google.BrokerFlow{BaseURL: broker.URL, HTTPClient: broker.Client(), AppVersion: "1.2.3", Platform: "test"}
	token, err := flow.RefreshToken(context.Background(), "connect-refresh", nil)
	if err != nil {
		t.Fatal(err)
	}
	if token.AccessToken != "connect-access" || token.RefreshToken != "connect-refresh" {
		t.Fatalf("token = %+v", token)
	}
	if err := flow.Revoke(context.Background(), "connect-refresh"); err != nil {
		t.Fatal(err)
	}
}

func TestBrokerFlowDoesNotFallbackWhenConnectIsUnavailable(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		http.Error(w, "unavailable", http.StatusServiceUnavailable)
	}))
	t.Cleanup(server.Close)

	flow := &google.BrokerFlow{BaseURL: server.URL, HTTPClient: server.Client()}
	_, err := flow.RefreshToken(context.Background(), "refresh", nil)
	if !errors.Is(err, google.ErrBrokerUnavailable) {
		t.Fatalf("want broker unavailable, got %v", err)
	}
}

type googleConnectBroker struct {
	t *testing.T
}

func (s *googleConnectBroker) StartAuthorization(_ context.Context, req *connect.Request[brokerv1.StartAuthorizationRequest]) (*connect.Response[brokerv1.StartAuthorizationResponse], error) {
	if req.Msg.Provider != brokerv1.Provider_PROVIDER_GOOGLE || req.Msg.Application.GetAppVersion() != "1.2.3" {
		s.t.Fatalf("start request = %+v", req.Msg)
	}
	redirect := req.Msg.DesktopHandoffRedirect
	go func() {
		time.Sleep(20 * time.Millisecond)
		response, err := http.Get(redirect + "?broker_state=state-1&handoff_code=code-1")
		if err != nil {
			s.t.Errorf("deliver handoff: %v", err)
			return
		}
		_ = response.Body.Close()
	}()
	return connect.NewResponse(&brokerv1.StartAuthorizationResponse{
		AuthUrl: "https://accounts.google.com/o/oauth2/v2/auth?state=state-1", BrokerState: "state-1", ExpiresAt: timestamppb.New(time.Now().Add(time.Minute)),
	}), nil
}

func (s *googleConnectBroker) ExchangeHandoff(_ context.Context, req *connect.Request[brokerv1.ExchangeHandoffRequest]) (*connect.Response[brokerv1.ExchangeHandoffResponse], error) {
	if req.Msg.Provider != brokerv1.Provider_PROVIDER_GOOGLE || req.Msg.Application.GetAppVersion() != "1.2.3" || req.Msg.HandoffCode != "code-1" {
		s.t.Fatalf("handoff request = %+v", req.Msg)
	}
	return connect.NewResponse(&brokerv1.ExchangeHandoffResponse{
		Provider: brokerv1.Provider_PROVIDER_GOOGLE,
		Scopes:   []string{"https://www.googleapis.com/auth/calendar.readonly"},
		Token:    &brokerv1.TokenMaterial{AccessToken: "google-access", RefreshToken: "google-refresh", TokenType: "Bearer"},
	}), nil
}

func (*googleConnectBroker) RefreshToken(context.Context, *connect.Request[brokerv1.RefreshTokenRequest]) (*connect.Response[brokerv1.RefreshTokenResponse], error) {
	return nil, connect.NewError(connect.CodeUnimplemented, errors.New("not used"))
}

func (*googleConnectBroker) RevokeToken(context.Context, *connect.Request[brokerv1.RevokeTokenRequest]) (*connect.Response[brokerv1.RevokeTokenResponse], error) {
	return nil, connect.NewError(connect.CodeUnimplemented, errors.New("not used"))
}
