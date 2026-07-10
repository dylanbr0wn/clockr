package httpapi

import (
	"connectrpc.com/connect"
	"github.com/dylanbr0wn/shiet/internal/broker/codes"
)

func connectCode(code string) connect.Code {
	switch code {
	case codes.HandoffAlreadyUsed:
		return connect.CodeAlreadyExists
	case codes.HandoffExpired:
		return connect.CodeDeadlineExceeded
	case codes.HandoffStateMismatch, codes.HandoffVerifierMismatch:
		return connect.CodePermissionDenied
	case codes.HandoffNotFound:
		return connect.CodeNotFound
	case codes.RateLimited:
		return connect.CodeResourceExhausted
	case codes.AuthDisabled, codes.RefreshDisabled, codes.AppVersionDisabled:
		return connect.CodeFailedPrecondition
	case codes.ProviderNotConfigured,
		codes.DatastoreUnavailable,
		codes.StatePersistFailed,
		codes.HandoffConsumeFailed,
		codes.GoogleTokenRefreshFailed,
		codes.GoogleRevokeFailed,
		codes.GitHubRevokeFailed,
		codes.SlackRevokeFailed:
		return connect.CodeUnavailable
	case codes.OperationNotSupported:
		return connect.CodeUnimplemented
	case codes.DesktopSessionAndHandoffChallengeRequired,
		codes.InvalidDesktopHandoffRedirect,
		codes.HandoffFieldsRequired,
		codes.RefreshTokenRequired,
		codes.AccessTokenRequired,
		codes.InvalidRefreshToken:
		return connect.CodeInvalidArgument
	default:
		return connect.CodeInternal
	}
}
