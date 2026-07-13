package httpapi

import (
	"context"
	"errors"
	"net/url"
	"strings"
	"time"

	brokerv1 "github.com/dylanbr0wn/shiet/gen/shiet/broker/v1"
	"github.com/dylanbr0wn/shiet/internal/broker/codes"
	"github.com/dylanbr0wn/shiet/internal/broker/ratelimit"
	"github.com/dylanbr0wn/shiet/internal/broker/store"
	"github.com/dylanbr0wn/shiet/internal/integration/oauth"
	"google.golang.org/protobuf/types/known/timestamppb"
)

type requestMetadata struct {
	ipBucket string
}

// BrokerService owns broker policy independently of the Connect adapter and
// provider callback routes. Server is embedded only as the dependency bundle
// while the broker is split out of its historical single-file HTTP implementation.
type BrokerService struct {
	Server
}

func (s Server) service() BrokerService {
	return BrokerService{Server: s}
}

type operationError struct {
	code string
}

func (e *operationError) Error() string { return e.code }

func opError(code string) *operationError {
	return &operationError{code: code}
}

func (s BrokerService) startAuthorization(ctx context.Context, req *brokerv1.StartAuthorizationRequest, meta requestMetadata) (*brokerv1.StartAuthorizationResponse, *operationError) {
	provider, ok := providerName(req.Provider)
	if !ok {
		return nil, opError(codes.ProviderNotConfigured)
	}
	if s.Store == nil {
		return nil, opError(codes.DatastoreUnavailable)
	}
	if !s.providerConfigured(provider) {
		return nil, opError(codes.ProviderNotConfigured)
	}
	if s.Config.AuthDisabled {
		s.Metrics.IncKillSwitch(codes.SurfaceStart)
		s.logInfo(codes.EventKillSwitch, "surface", codes.SurfaceStart, "reason", codes.AuthDisabled)
		return nil, opError(codes.AuthDisabled)
	}
	if !s.allow(ratelimit.Key(codes.LimitKeyStart, meta.ipBucket), limitStart) {
		s.Metrics.IncRateLimited(codes.SurfaceStart)
		s.logInfo(codes.EventRateLimited, "surface", codes.SurfaceStart, "ip_bucket", meta.ipBucket)
		return nil, opError(codes.RateLimited)
	}

	desktopSessionID := strings.TrimSpace(req.DesktopSessionId)
	handoffChallenge := strings.TrimSpace(req.HandoffChallenge)
	desktopRedirect := strings.TrimSpace(req.DesktopHandoffRedirect)
	appVersion := ""
	platform := ""
	if req.Application != nil {
		appVersion = strings.TrimSpace(req.Application.AppVersion)
		platform = strings.TrimSpace(req.Application.Platform)
	}
	if desktopSessionID == "" || handoffChallenge == "" {
		s.Metrics.IncAuthStartFail()
		return nil, opError(codes.DesktopSessionAndHandoffChallengeRequired)
	}
	if s.Config.AppVersionDisabled(appVersion) {
		s.Metrics.IncKillSwitch(codes.SurfaceStart + codes.KillSwitchVersionSuffix)
		s.logInfo(codes.EventKillSwitch, "surface", codes.SurfaceStart, "reason", codes.AppVersionDisabled, "app_version", appVersion)
		return nil, opError(codes.AppVersionDisabled)
	}
	if desktopRedirect != "" {
		if err := validateDesktopHandoffRedirect(desktopRedirect); err != nil {
			s.Metrics.IncAuthStartFail()
			return nil, opError(codes.InvalidDesktopHandoffRedirect)
		}
	}

	state, err := randomString(32)
	if err != nil {
		s.Metrics.IncAuthStartFail()
		return nil, opError(codes.RandomStateFailed)
	}
	verifier, err := randomString(64)
	if err != nil {
		s.Metrics.IncAuthStartFail()
		return nil, opError(codes.RandomVerifierFailed)
	}
	challenge := pkceS256(verifier)
	now := s.now()
	expiresAt := now.Add(s.Config.StateTTL)
	record := store.OAuthState{
		ID:                     state,
		Provider:               provider,
		DesktopSessionID:       desktopSessionID,
		PKCEVerifier:           verifier,
		PKCEChallenge:          challenge,
		HandoffChallenge:       handoffChallenge,
		DesktopHandoffRedirect: desktopRedirect,
		Scopes:                 append([]string(nil), s.providerScopes(provider)...),
		AppVersion:             appVersion,
		Platform:               platform,
		SourceIPBucket:         meta.ipBucket,
		ExpiresAt:              expiresAt,
	}
	if err := s.Store.SaveOAuthState(ctx, record); err != nil {
		s.Metrics.IncAuthStartFail()
		return nil, opError(codes.StatePersistFailed)
	}
	authURL, err := s.authURL(provider, state, challenge)
	if err != nil {
		s.Metrics.IncAuthStartFail()
		return nil, opError(codes.AuthURLFailed)
	}
	s.Metrics.IncAuthStart()
	s.logInfo(codes.EventAuthStart, "outcome", codes.OutcomeOK, "app_version", appVersion, "platform", platform, "ip_bucket", meta.ipBucket)
	return &brokerv1.StartAuthorizationResponse{
		AuthUrl:     authURL,
		BrokerState: state,
		ExpiresAt:   timestamppb.New(expiresAt),
	}, nil
}

func (s BrokerService) exchangeHandoff(ctx context.Context, req *brokerv1.ExchangeHandoffRequest, meta requestMetadata) (*brokerv1.ExchangeHandoffResponse, *operationError) {
	provider, ok := providerName(req.Provider)
	if !ok || !s.providerConfigured(provider) {
		return nil, opError(codes.ProviderNotConfigured)
	}
	if s.Store == nil {
		return nil, opError(codes.DatastoreUnavailable)
	}
	if s.Config.AuthDisabled {
		s.Metrics.IncKillSwitch(codes.SurfaceHandoff)
		s.logInfo(codes.EventKillSwitch, "surface", codes.SurfaceHandoff, "reason", codes.AuthDisabled)
		return nil, opError(codes.AuthDisabled)
	}
	if !s.allow(ratelimit.Key(codes.LimitKeyHandoff, meta.ipBucket), limitHandoff) {
		s.Metrics.IncRateLimited(codes.SurfaceHandoff)
		return nil, opError(codes.RateLimited)
	}

	desktopSessionID := strings.TrimSpace(req.DesktopSessionId)
	brokerState := strings.TrimSpace(req.BrokerState)
	handoffCode := strings.TrimSpace(req.HandoffCode)
	handoffVerifier := strings.TrimSpace(req.HandoffVerifier)
	appVersion := ""
	if req.Application != nil {
		appVersion = strings.TrimSpace(req.Application.AppVersion)
	}
	if desktopSessionID == "" || brokerState == "" || handoffCode == "" || handoffVerifier == "" {
		return nil, opError(codes.HandoffFieldsRequired)
	}

	codeHash := hashHandoffCode(handoffCode)
	failDimension := meta.ipBucket + "|" + desktopSessionID + "|" + codeHash
	failKey := ratelimit.Key(codes.LimitKeyHandoffFail, failDimension)
	versionFailKey := ratelimit.Key(codes.LimitKeyHandoffFail, failDimension+"|"+appVersion)
	record, err := s.Store.ConsumeHandoff(ctx, codeHash, provider, desktopSessionID, brokerState, pkceS256(handoffVerifier), s.now())
	if err != nil {
		reason := codes.OutcomeConsumeFailed
		brokerCode := codes.HandoffConsumeFailed
		switch {
		case errors.Is(err, store.ErrAlreadyUsed):
			reason, brokerCode = codes.OutcomeAlreadyUsed, codes.HandoffAlreadyUsed
			s.Metrics.IncQuotaRisk(codes.QuotaHandoffReplay)
		case errors.Is(err, store.ErrExpired):
			reason, brokerCode = codes.OutcomeExpired, codes.HandoffExpired
		case errors.Is(err, store.ErrNotFound):
			reason, brokerCode = codes.OutcomeNotFound, codes.HandoffNotFound
		case errors.Is(err, store.ErrMismatch):
			reason, brokerCode = codes.OutcomeStateMismatch, codes.HandoffStateMismatch
			s.Metrics.IncQuotaRisk(codes.QuotaHandoffMismatch)
		}
		s.Metrics.IncHandoffFailure(reason)
		s.logInfo(codes.EventHandoff, "outcome", reason, "ip_bucket", meta.ipBucket)
		allowed := s.allow(failKey, limitHandoffFailure)
		if appVersion != "" {
			allowed = s.allow(versionFailKey, limitHandoffFailure) && allowed
		}
		if !allowed {
			s.Metrics.IncRateLimited(codes.SurfaceHandoffFailure)
			return nil, opError(codes.RateLimited)
		}
		return nil, opError(brokerCode)
	}
	payload, err := decryptTokenPayload(
		s.providerClientSecret(provider),
		handoffAAD(record.StateID, record.DesktopSessionID, record.HandoffChallenge),
		record.EncryptedTokenPayload,
	)
	if err != nil {
		s.Metrics.IncHandoffFailure(codes.OutcomePayloadInvalid)
		return nil, opError(codes.HandoffPayloadInvalid)
	}
	tokenType := payload.TokenType
	if tokenType == "" {
		tokenType = "Bearer"
	}
	s.Metrics.IncHandoffOK()
	s.logInfo(codes.EventHandoff, "outcome", codes.OutcomeOK, "ip_bucket", meta.ipBucket)
	return &brokerv1.ExchangeHandoffResponse{
		Provider:    req.Provider,
		AccountHint: record.AccountHint,
		Scopes:      append([]string(nil), record.Scopes...),
		Token: &brokerv1.TokenMaterial{
			AccessToken:  payload.AccessToken,
			RefreshToken: payload.RefreshToken,
			TokenType:    tokenType,
			Expiry:       timestamppb.New(payload.Expiry),
		},
	}, nil
}

func (s BrokerService) refreshToken(ctx context.Context, req *brokerv1.RefreshTokenRequest, meta requestMetadata) (*brokerv1.RefreshTokenResponse, *operationError) {
	if req.Provider == brokerv1.Provider_PROVIDER_GITHUB || req.Provider == brokerv1.Provider_PROVIDER_SLACK {
		return nil, opError(codes.OperationNotSupported)
	}
	provider, ok := providerName(req.Provider)
	if !ok {
		return nil, opError(codes.ProviderNotConfigured)
	}
	if !s.providerConfigured(provider) {
		return nil, opError(codes.ProviderNotConfigured)
	}
	if s.Config.RefreshDisabled {
		s.Metrics.IncKillSwitch(codes.SurfaceRefresh)
		s.logInfo(codes.EventKillSwitch, "surface", codes.SurfaceRefresh, "reason", codes.RefreshDisabled)
		return nil, opError(codes.RefreshDisabled)
	}
	if !s.allow(ratelimit.Key(codes.LimitKeyRefresh, meta.ipBucket), limitRefresh) {
		s.Metrics.IncRateLimited(codes.SurfaceRefresh)
		return nil, opError(codes.RateLimited)
	}
	refreshToken := strings.TrimSpace(req.RefreshToken)
	appVersion := ""
	if req.Application != nil {
		appVersion = strings.TrimSpace(req.Application.AppVersion)
	}
	if refreshToken == "" {
		return nil, opError(codes.RefreshTokenRequired)
	}
	if s.Config.AppVersionDisabled(appVersion) {
		s.Metrics.IncKillSwitch(codes.SurfaceRefresh + codes.KillSwitchVersionSuffix)
		return nil, opError(codes.AppVersionDisabled)
	}

	form := url.Values{}
	form.Set("grant_type", "refresh_token")
	form.Set("refresh_token", refreshToken)
	switch req.Provider {
	case brokerv1.Provider_PROVIDER_GOOGLE:
		form.Set("client_id", s.Config.GoogleClientID)
		form.Set("client_secret", s.Config.GoogleClientSecret)
		if len(req.Scopes) > 0 {
			form.Set("scope", strings.Join(req.Scopes, " "))
		}
	default:
		creds, ok := s.providerCredentials(provider)
		if !ok {
			return nil, opError(codes.ProviderNotConfigured)
		}
		form.Set("client_id", creds.ClientID)
		form.Set("client_secret", creds.ClientSecret)
	}

	var (
		token providerTokenResponse
		err   error
	)
	switch req.Provider {
	case brokerv1.Provider_PROVIDER_GOOGLE:
		token, err = s.postGoogleToken(ctx, form)
	default:
		token, err = s.postProviderToken(ctx, provider, form)
	}
	if err != nil {
		if !s.allow(ratelimit.Key(codes.LimitKeyRefreshFail, meta.ipBucket), limitRefreshFailure) {
			s.Metrics.IncRateLimited(codes.SurfaceRefreshFailure)
			return nil, opError(codes.RateLimited)
		}
		var googleErr *providerTokenError
		if errors.As(err, &googleErr) && googleErr.Code == codes.GoogleInvalidGrant {
			s.Metrics.IncRefreshFailure(codes.OutcomeInvalidGrant)
			s.Metrics.IncQuotaRisk(codes.QuotaInvalidGrant)
			s.logInfo(codes.EventRefresh, "outcome", codes.OutcomeInvalidGrant, "ip_bucket", meta.ipBucket, "app_version", appVersion)
			return nil, opError(codes.InvalidRefreshToken)
		}
		if req.Provider == brokerv1.Provider_PROVIDER_BITBUCKET {
			s.Metrics.IncRefreshFailure(codes.OutcomeGoogleFailed)
			s.logInfo(codes.EventRefresh, "outcome", codes.OutcomeGoogleFailed, "ip_bucket", meta.ipBucket, "app_version", appVersion)
			return nil, opError(codes.InvalidRefreshToken)
		}
		s.Metrics.IncRefreshFailure(codes.OutcomeGoogleFailed)
		s.logInfo(codes.EventRefresh, "outcome", codes.OutcomeGoogleFailed, "ip_bucket", meta.ipBucket)
		return nil, opError(codes.GoogleTokenRefreshFailed)
	}
	tokenType := token.TokenType
	if tokenType == "" {
		tokenType = "Bearer"
	}
	s.Metrics.IncRefreshOK()
	s.logInfo(codes.EventRefresh, "outcome", codes.OutcomeOK, "ip_bucket", meta.ipBucket, "app_version", appVersion)
	return &brokerv1.RefreshTokenResponse{Token: &brokerv1.TokenMaterial{
		AccessToken:  token.AccessToken,
		RefreshToken: token.RefreshToken,
		TokenType:    tokenType,
		Expiry:       timestamppb.New(s.now().Add(time.Duration(token.ExpiresIn) * time.Second)),
	}}, nil
}

func (s BrokerService) revokeToken(ctx context.Context, req *brokerv1.RevokeTokenRequest, meta requestMetadata) (*brokerv1.RevokeTokenResponse, *operationError) {
	provider, ok := providerName(req.Provider)
	if !ok || !s.providerConfigured(provider) {
		return nil, opError(codes.ProviderNotConfigured)
	}
	limitDimension := meta.ipBucket
	if provider != oauth.ProviderGoogle {
		limitDimension = provider + "|" + limitDimension
	}
	if !s.allow(ratelimit.Key(codes.LimitKeyRevoke, limitDimension), limitRevoke) {
		s.Metrics.IncRateLimited(codes.SurfaceRevoke)
		return nil, opError(codes.RateLimited)
	}
	reason := strings.TrimSpace(req.Reason)
	if provider == "google" {
		refreshToken := strings.TrimSpace(req.GetRefreshToken())
		if refreshToken == "" || req.GetAccessToken() != "" {
			return nil, opError(codes.RefreshTokenRequired)
		}
		if err := s.revokeGoogleToken(ctx, refreshToken); err != nil {
			if errors.Is(err, errGoogleTokenAlreadyRevoked) {
				s.Metrics.IncRevokeOK()
				s.Metrics.IncRevokeOutcome(codes.OutcomeAlreadyRevoked)
				s.logInfo(codes.EventRevoke, "outcome", codes.OutcomeAlreadyRevoked, "reason", reason, "ip_bucket", meta.ipBucket)
				return &brokerv1.RevokeTokenResponse{Revoked: true}, nil
			}
			s.Metrics.IncRevokeOutcome(codes.OutcomeGoogleFailed)
			return nil, opError(codes.GoogleRevokeFailed)
		}
	} else {
		accessToken := strings.TrimSpace(req.GetAccessToken())
		if accessToken == "" || req.GetRefreshToken() != "" {
			return nil, opError(codes.AccessTokenRequired)
		}
		if provider == oauth.ProviderGitHub {
			if err := s.revokeGitHubToken(ctx, accessToken); err != nil {
				s.Metrics.IncRevokeOutcome(codes.OutcomeGitHubFailed)
				return nil, opError(codes.GitHubRevokeFailed)
			}
		} else if err := s.revokeSlackToken(ctx, accessToken); err != nil {
			s.Metrics.IncRevokeOutcome(codes.OutcomeSlackFailed)
			return nil, opError(codes.SlackRevokeFailed)
		}
	}
	s.Metrics.IncRevokeOK()
	s.Metrics.IncRevokeOutcome(codes.OutcomeOK)
	s.logInfo(codes.EventRevoke, "provider", provider, "outcome", codes.OutcomeOK, "reason", reason, "ip_bucket", meta.ipBucket)
	return &brokerv1.RevokeTokenResponse{Revoked: true}, nil
}

func providerName(provider brokerv1.Provider) (string, bool) {
	switch provider {
	case brokerv1.Provider_PROVIDER_GOOGLE:
		return "google", true
	case brokerv1.Provider_PROVIDER_GITHUB:
		return "github", true
	case brokerv1.Provider_PROVIDER_SLACK:
		return "slack", true
	case brokerv1.Provider_PROVIDER_BITBUCKET:
		return "bitbucket", true
	default:
		return "", false
	}
}
