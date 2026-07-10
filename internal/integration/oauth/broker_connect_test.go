package oauth_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"connectrpc.com/connect"
	brokerv1 "github.com/dylanbr0wn/shiet/gen/shiet/broker/v1"
	"github.com/dylanbr0wn/shiet/gen/shiet/broker/v1/brokerv1connect"
	"github.com/dylanbr0wn/shiet/internal/integration/oauth"
	"google.golang.org/protobuf/types/known/timestamppb"
)

func TestBrokerFlowCompletesStartAndHandoffThroughConnect(t *testing.T) {
	t.Parallel()

	stub := &desktopConnectBroker{t: t}
	_, handler := brokerv1connect.NewOAuthBrokerServiceHandler(stub)
	server := httptest.NewServer(handler)
	t.Cleanup(server.Close)

	flow := oauth.BrokerFlow{
		Provider:      "github",
		BaseURL:       server.URL,
		AuthURLHost:   "github.com",
		AuthURLPaths:  []string{"/login/oauth/authorize"},
		DefaultScopes: []string{"repo"},
		HTTPClient:    server.Client(),
		OpenURL:       func(string) error { return nil },
		AppVersion:    "1.2.3",
		Platform:      "test",
	}
	result, err := flow.Authorize(context.Background(), "account-1")
	if err != nil {
		t.Fatal(err)
	}
	if result.Token.AccessToken != "github-access" || result.Provider != "github" {
		t.Fatalf("result = %+v", result)
	}
}

type desktopConnectBroker struct {
	t *testing.T
}

func (s *desktopConnectBroker) StartAuthorization(_ context.Context, req *connect.Request[brokerv1.StartAuthorizationRequest]) (*connect.Response[brokerv1.StartAuthorizationResponse], error) {
	if req.Msg.Provider != brokerv1.Provider_PROVIDER_GITHUB || req.Msg.Application.GetAppVersion() != "1.2.3" {
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
		AuthUrl:     "https://github.com/login/oauth/authorize?state=state-1",
		BrokerState: "state-1",
		ExpiresAt:   timestamppb.New(time.Now().Add(time.Minute)),
	}), nil
}

func (s *desktopConnectBroker) ExchangeHandoff(_ context.Context, req *connect.Request[brokerv1.ExchangeHandoffRequest]) (*connect.Response[brokerv1.ExchangeHandoffResponse], error) {
	if req.Msg.Provider != brokerv1.Provider_PROVIDER_GITHUB || req.Msg.Application.GetAppVersion() != "1.2.3" || req.Msg.HandoffCode != "code-1" {
		s.t.Fatalf("handoff request = %+v", req.Msg)
	}
	return connect.NewResponse(&brokerv1.ExchangeHandoffResponse{
		Provider: brokerv1.Provider_PROVIDER_GITHUB,
		Scopes:   []string{"repo"},
		Token: &brokerv1.TokenMaterial{
			AccessToken: "github-access",
			TokenType:   "Bearer",
		},
	}), nil
}

func (*desktopConnectBroker) RefreshToken(context.Context, *connect.Request[brokerv1.RefreshTokenRequest]) (*connect.Response[brokerv1.RefreshTokenResponse], error) {
	return nil, connect.NewError(connect.CodeUnimplemented, nil)
}

func (*desktopConnectBroker) RevokeToken(context.Context, *connect.Request[brokerv1.RevokeTokenRequest]) (*connect.Response[brokerv1.RevokeTokenResponse], error) {
	return nil, connect.NewError(connect.CodeUnimplemented, nil)
}
