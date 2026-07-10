package httpapi

import (
	"context"
	"errors"

	"connectrpc.com/connect"
	brokerv1 "github.com/dylanbr0wn/shiet/gen/shiet/broker/v1"
	"google.golang.org/protobuf/proto"
)

type connectBrokerService struct {
	service BrokerService
}

func (s connectBrokerService) StartAuthorization(ctx context.Context, req *connect.Request[brokerv1.StartAuthorizationRequest]) (*connect.Response[brokerv1.StartAuthorizationResponse], error) {
	response, opErr := s.service.startAuthorization(ctx, req.Msg, requestMetadata{ipBucket: sourceIPBucket(req.Peer().Addr)})
	if opErr != nil {
		return nil, toConnectError(opErr)
	}
	return connect.NewResponse(response), nil
}

func (s connectBrokerService) ExchangeHandoff(ctx context.Context, req *connect.Request[brokerv1.ExchangeHandoffRequest]) (*connect.Response[brokerv1.ExchangeHandoffResponse], error) {
	response, opErr := s.service.exchangeHandoff(ctx, req.Msg, requestMetadata{ipBucket: sourceIPBucket(req.Peer().Addr)})
	if opErr != nil {
		return nil, toConnectError(opErr)
	}
	return connect.NewResponse(response), nil
}

func (s connectBrokerService) RefreshToken(ctx context.Context, req *connect.Request[brokerv1.RefreshTokenRequest]) (*connect.Response[brokerv1.RefreshTokenResponse], error) {
	response, opErr := s.service.refreshToken(ctx, req.Msg, requestMetadata{ipBucket: sourceIPBucket(req.Peer().Addr)})
	if opErr != nil {
		return nil, toConnectError(opErr)
	}
	return connect.NewResponse(response), nil
}

func (s connectBrokerService) RevokeToken(ctx context.Context, req *connect.Request[brokerv1.RevokeTokenRequest]) (*connect.Response[brokerv1.RevokeTokenResponse], error) {
	response, opErr := s.service.revokeToken(ctx, req.Msg, requestMetadata{ipBucket: sourceIPBucket(req.Peer().Addr)})
	if opErr != nil {
		return nil, toConnectError(opErr)
	}
	return connect.NewResponse(response), nil
}

func toConnectError(opErr *operationError) error {
	err := connect.NewError(connectCode(opErr.code), errors.New(opErr.code))
	detail, detailErr := connect.NewErrorDetail(proto.Message(&brokerv1.BrokerErrorDetail{Code: opErr.code}))
	if detailErr == nil {
		err.AddDetail(detail)
	}
	return err
}
