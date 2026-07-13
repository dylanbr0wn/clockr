package appapi

import (
	"context"
	"errors"
	"strings"

	"connectrpc.com/connect"
	appv1 "github.com/dylanbr0wn/shiet/gen/shiet/app/v1"
	"github.com/dylanbr0wn/shiet/internal/integration/catalog"
	"github.com/dylanbr0wn/shiet/internal/integration/connection"
)

func (s *IntegrationService) ListIntegrationProviders(_ context.Context, _ *connect.Request[appv1.ListIntegrationProvidersRequest]) (*connect.Response[appv1.ListIntegrationProvidersResponse], error) {
	items := catalog.All()
	out := make([]*appv1.IntegrationDescriptor, len(items))
	for i, item := range items {
		out[i] = catalog.ToProto(item, s.oauthAvailable(item.ID))
	}
	return connect.NewResponse(&appv1.ListIntegrationProvidersResponse{Providers: out}), nil
}

func (s *IntegrationService) GetIntegrationAuthStatus(_ context.Context, req *connect.Request[appv1.GetIntegrationAuthStatusRequest]) (*connect.Response[appv1.GetIntegrationAuthStatusResponse], error) {
	provider := strings.TrimSpace(req.Msg.Provider)
	if provider == "" {
		return nil, invalidArgument("provider is required")
	}
	entry, ok := catalog.Lookup(provider)
	if !ok {
		return nil, invalidArgument("unknown provider " + provider)
	}
	status, err := s.integrationAuthStatus(provider)
	if err != nil {
		return nil, err
	}
	status.SupportsPat = entry.SupportsPAT
	return connect.NewResponse(status), nil
}

func (s *IntegrationService) ConnectIntegration(ctx context.Context, req *connect.Request[appv1.ConnectIntegrationRequest]) (*connect.Response[appv1.ConnectIntegrationResponse], error) {
	provider := strings.TrimSpace(req.Msg.Provider)
	if provider == "" {
		return nil, invalidArgument("provider is required")
	}
	if _, ok := catalog.Lookup(provider); !ok {
		return nil, invalidArgument("unknown provider " + provider)
	}

	var (
		conn connection.Connection
		err  error
	)
	switch provider {
	case "google":
		if s.google == nil {
			return nil, connect.NewError(connect.CodeUnimplemented, errors.New("Google connect is unavailable"))
		}
		conn, err = s.google.Connect(ctx, req.Msg.AccountId, req.Msg.AccountLabel)
	case "github":
		if s.github == nil {
			return nil, connect.NewError(connect.CodeUnimplemented, errors.New("GitHub connect is unavailable"))
		}
		conn, err = s.github.Connect(ctx, req.Msg.Pat)
	case "slack":
		if s.slack == nil {
			return nil, connect.NewError(connect.CodeUnimplemented, errors.New("Slack connect is unavailable"))
		}
		conn, err = s.slack.Connect(ctx)
	case "bitbucket":
		if s.bitbucket == nil {
			return nil, connect.NewError(connect.CodeUnimplemented, errors.New("Bitbucket connect is unavailable"))
		}
		conn, err = s.bitbucket.Connect(ctx)
	default:
		return nil, invalidArgument("unknown provider " + provider)
	}
	if err != nil {
		return nil, mapServiceError(err)
	}
	return connect.NewResponse(&appv1.ConnectIntegrationResponse{Connection: mapIntegrationConnection(conn)}), nil
}

func (s *IntegrationService) DisconnectIntegration(ctx context.Context, req *connect.Request[appv1.DisconnectIntegrationRequest]) (*connect.Response[appv1.DisconnectIntegrationResponse], error) {
	provider := strings.TrimSpace(req.Msg.Provider)
	if provider == "" {
		return nil, invalidArgument("provider is required")
	}
	if strings.TrimSpace(req.Msg.AccountId) == "" {
		return nil, invalidArgument("account_id is required")
	}
	if _, ok := catalog.Lookup(provider); !ok {
		return nil, invalidArgument("unknown provider " + provider)
	}

	var err error
	switch provider {
	case "google":
		if s.google == nil {
			return nil, connect.NewError(connect.CodeUnimplemented, errors.New("Google disconnect is unavailable"))
		}
		err = s.google.Disconnect(ctx, req.Msg.AccountId)
	case "github":
		if s.github == nil {
			return nil, connect.NewError(connect.CodeUnimplemented, errors.New("GitHub disconnect is unavailable"))
		}
		err = s.github.Disconnect(ctx, req.Msg.AccountId)
	case "slack":
		if s.slack == nil {
			return nil, connect.NewError(connect.CodeUnimplemented, errors.New("Slack disconnect is unavailable"))
		}
		err = s.slack.Disconnect(ctx, req.Msg.AccountId)
	case "bitbucket":
		if s.bitbucket == nil {
			return nil, connect.NewError(connect.CodeUnimplemented, errors.New("Bitbucket disconnect is unavailable"))
		}
		err = s.bitbucket.Disconnect(ctx, req.Msg.AccountId)
	default:
		return nil, invalidArgument("unknown provider " + provider)
	}
	if err != nil {
		return nil, mapServiceError(err)
	}
	return connect.NewResponse(&appv1.DisconnectIntegrationResponse{}), nil
}

func (s *IntegrationService) oauthAvailable(provider string) bool {
	switch provider {
	case "google":
		if s.google == nil {
			return false
		}
		return s.google.OAuthAvailable()
	case "github":
		if s.github == nil {
			return false
		}
		return s.github.OAuthAvailable()
	case "slack":
		if s.slack == nil {
			return false
		}
		return s.slack.OAuthAvailable()
	case "bitbucket":
		if s.bitbucket == nil {
			return false
		}
		return s.bitbucket.OAuthAvailable()
	default:
		return false
	}
}

func (s *IntegrationService) integrationAuthStatus(provider string) (*appv1.GetIntegrationAuthStatusResponse, error) {
	switch provider {
	case "google":
		if s.google == nil {
			return nil, connect.NewError(connect.CodeUnimplemented, errors.New("Google auth status is unavailable"))
		}
		status := s.google.Status()
		return &appv1.GetIntegrationAuthStatusResponse{
			Provider:       provider,
			Mode:           status.Mode,
			BrokerBaseUrl:  status.BrokerBaseURL,
			OauthAvailable: s.google.OAuthAvailable(),
		}, nil
	case "github":
		if s.github == nil {
			return nil, connect.NewError(connect.CodeUnimplemented, errors.New("GitHub auth status is unavailable"))
		}
		return authStatusFromProvider(provider, s.github.AuthMode, s.github.BrokerBaseURL, s.github.OAuthAvailable()), nil
	case "slack":
		if s.slack == nil {
			return nil, connect.NewError(connect.CodeUnimplemented, errors.New("Slack auth status is unavailable"))
		}
		return authStatusFromProvider(provider, s.slack.AuthMode, s.slack.BrokerBaseURL, s.slack.OAuthAvailable()), nil
	case "bitbucket":
		if s.bitbucket == nil {
			return nil, connect.NewError(connect.CodeUnimplemented, errors.New("Bitbucket auth status is unavailable"))
		}
		return authStatusFromProvider(provider, s.bitbucket.AuthMode, s.bitbucket.BrokerBaseURL, s.bitbucket.OAuthAvailable()), nil
	default:
		return nil, invalidArgument("unknown provider " + provider)
	}
}

func authStatusFromProvider(provider, mode, brokerBaseURL string, oauthAvailable bool) *appv1.GetIntegrationAuthStatusResponse {
	mode = strings.ToLower(strings.TrimSpace(mode))
	if mode == "" {
		mode = "broker"
	}
	status := &appv1.GetIntegrationAuthStatusResponse{
		Provider:       provider,
		Mode:           mode,
		OauthAvailable: oauthAvailable,
	}
	if mode == "broker" {
		status.BrokerBaseUrl = strings.TrimSpace(brokerBaseURL)
	}
	return status
}

func mapIntegrationConnection(item connection.Connection) *appv1.IntegrationConnection {
	return &appv1.IntegrationConnection{
		Id:           item.ID,
		Provider:     item.Provider,
		AccountLabel: item.AccountLabel,
		AccountId:    item.AccountID,
		Scopes:       append([]string(nil), item.Scopes...),
		Status:       item.Status,
		ConnectedAt:  item.ConnectedAt,
		UpdatedAt:    item.UpdatedAt,
	}
}
