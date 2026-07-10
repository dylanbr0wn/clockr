package httpapi

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"connectrpc.com/connect"
	brokerv1 "github.com/dylanbr0wn/shiet/gen/shiet/broker/v1"
	"github.com/dylanbr0wn/shiet/gen/shiet/broker/v1/brokerv1connect"
	"github.com/dylanbr0wn/shiet/internal/broker/codes"
	"github.com/dylanbr0wn/shiet/internal/broker/observe"
	"github.com/dylanbr0wn/shiet/internal/broker/ratelimit"
	"github.com/dylanbr0wn/shiet/internal/broker/store"
)

func TestConnectStartAuthorizationMatchesBrokerContract(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 7, 9, 12, 0, 0, 0, time.UTC)
	mem := &memoryStore{}
	srv := Server{Config: testConfig(), Store: mem, Clock: func() time.Time { return now }}
	client := brokerv1connect.NewOAuthBrokerServiceClient(&http.Client{
		Transport: brokerHandlerTransport{handler: srv.Handler()},
	}, "http://broker.test")

	response, err := client.StartAuthorization(context.Background(), connect.NewRequest(&brokerv1.StartAuthorizationRequest{
		Provider:         brokerv1.Provider_PROVIDER_GOOGLE,
		DesktopSessionId: "desktop-1",
		HandoffChallenge: "challenge-1",
		Application: &brokerv1.ApplicationMetadata{
			AppVersion: "0.1.0",
			Platform:   "darwin-arm64",
		},
	}))
	if err != nil {
		t.Fatal(err)
	}
	if response.Msg.BrokerState == "" || response.Msg.AuthUrl == "" {
		t.Fatalf("missing start response fields: %#v", response.Msg)
	}
	if got := response.Msg.ExpiresAt.AsTime(); !got.Equal(now.Add(5 * time.Minute)) {
		t.Fatalf("expires_at = %s", got)
	}
	if len(mem.states) != 1 || mem.states[0].Provider != "google" {
		t.Fatalf("google state not persisted: %+v", mem.states)
	}
}

func TestConnectErrorsIncludeStableBrokerCode(t *testing.T) {
	t.Parallel()

	srv := Server{Config: testConfig(), Store: &memoryStore{}}
	client := brokerv1connect.NewOAuthBrokerServiceClient(&http.Client{
		Transport: brokerHandlerTransport{handler: srv.Handler()},
	}, "http://broker.test")

	_, err := client.StartAuthorization(context.Background(), connect.NewRequest(&brokerv1.StartAuthorizationRequest{
		Provider: brokerv1.Provider_PROVIDER_GOOGLE,
	}))
	var connectErr *connect.Error
	if !errors.As(err, &connectErr) {
		t.Fatalf("expected Connect error, got %v", err)
	}
	if connectErr.Code() != connect.CodeInvalidArgument {
		t.Fatalf("code = %v", connectErr.Code())
	}
	found := false
	for _, detail := range connectErr.Details() {
		value, detailErr := detail.Value()
		if detailErr != nil {
			t.Fatal(detailErr)
		}
		if brokerDetail, ok := value.(*brokerv1.BrokerErrorDetail); ok && brokerDetail.Code == "desktop_session_id_and_handoff_challenge_required" {
			found = true
		}
	}
	if !found {
		t.Fatalf("stable broker detail missing: %v", connectErr.Details())
	}
}

func TestConnectDatastoreAndConfigurationFailuresAreUnavailable(t *testing.T) {
	t.Parallel()

	t.Run("state persistence", func(t *testing.T) {
		srv := Server{Config: testConfig(), Store: &saveFailureStore{}}
		client := connectTestClient(srv)
		_, err := client.StartAuthorization(context.Background(), connect.NewRequest(&brokerv1.StartAuthorizationRequest{
			Provider:         brokerv1.Provider_PROVIDER_GOOGLE,
			DesktopSessionId: "desktop-1",
			HandoffChallenge: "challenge-1",
		}))
		assertConnectBrokerError(t, err, connect.CodeUnavailable, codes.StatePersistFailed)
	})

	t.Run("handoff datastore", func(t *testing.T) {
		srv := Server{Config: testConfig(), Store: &consumeFailureStore{}}
		client := connectTestClient(srv)
		_, err := client.ExchangeHandoff(context.Background(), connect.NewRequest(&brokerv1.ExchangeHandoffRequest{
			Provider:         brokerv1.Provider_PROVIDER_GOOGLE,
			DesktopSessionId: "desktop-1",
			BrokerState:      "state-1",
			HandoffCode:      "code-1",
			HandoffVerifier:  "verifier-1",
		}))
		assertConnectBrokerError(t, err, connect.CodeUnavailable, codes.HandoffConsumeFailed)
	})

	t.Run("missing Google configuration", func(t *testing.T) {
		cfg := testConfig()
		cfg.GoogleClientSecret = ""
		srv := Server{Config: cfg, Store: &memoryStore{}}
		client := connectTestClient(srv)
		_, err := client.RefreshToken(context.Background(), connect.NewRequest(&brokerv1.RefreshTokenRequest{
			Provider:     brokerv1.Provider_PROVIDER_GOOGLE,
			RefreshToken: "refresh-1",
		}))
		assertConnectBrokerError(t, err, connect.CodeUnavailable, codes.ProviderNotConfigured)
	})
}

func TestConnectHandoffFailureLimitIncludesApplicationVersion(t *testing.T) {
	t.Parallel()

	limiter := &recordingLimiter{}
	srv := Server{Config: testConfig(), Store: &memoryStore{}, Limiter: limiter}
	client := connectTestClient(srv)
	_, _ = client.ExchangeHandoff(context.Background(), connect.NewRequest(&brokerv1.ExchangeHandoffRequest{
		Provider:         brokerv1.Provider_PROVIDER_GOOGLE,
		DesktopSessionId: "desktop-1",
		BrokerState:      "state-1",
		HandoffCode:      "code-1",
		HandoffVerifier:  "verifier-1",
		Application:      &brokerv1.ApplicationMetadata{AppVersion: "9.8.7", Platform: "test"},
	}))
	if !strings.Contains(strings.Join(limiter.keys, "\n"), "9.8.7") {
		t.Fatalf("handoff failure limiter keys omit app version: %v", limiter.keys)
	}
}

func TestConnectKillSwitchesReturnStableErrorsAndMetrics(t *testing.T) {
	t.Parallel()

	t.Run("auth disabled", func(t *testing.T) {
		cfg := testConfig()
		cfg.AuthDisabled = true
		metrics := observe.NewMetrics()
		_, err := connectTestClient(Server{Config: cfg, Store: &memoryStore{}, Metrics: metrics}).StartAuthorization(context.Background(), connect.NewRequest(&brokerv1.StartAuthorizationRequest{
			Provider: brokerv1.Provider_PROVIDER_GOOGLE, DesktopSessionId: "desktop-1", HandoffChallenge: "challenge-1",
		}))
		assertConnectBrokerError(t, err, connect.CodeFailedPrecondition, codes.AuthDisabled)
		if metrics.KillSwitchCount(codes.SurfaceStart) != 1 {
			t.Fatalf("start kill-switch metric = %d", metrics.KillSwitchCount(codes.SurfaceStart))
		}
	})

	t.Run("refresh disabled", func(t *testing.T) {
		cfg := testConfig()
		cfg.RefreshDisabled = true
		metrics := observe.NewMetrics()
		_, err := connectTestClient(Server{Config: cfg, Store: &memoryStore{}, Metrics: metrics}).RefreshToken(context.Background(), connect.NewRequest(&brokerv1.RefreshTokenRequest{
			Provider: brokerv1.Provider_PROVIDER_GOOGLE, RefreshToken: "refresh-1",
		}))
		assertConnectBrokerError(t, err, connect.CodeFailedPrecondition, codes.RefreshDisabled)
		if metrics.KillSwitchCount(codes.SurfaceRefresh) != 1 {
			t.Fatalf("refresh kill-switch metric = %d", metrics.KillSwitchCount(codes.SurfaceRefresh))
		}
	})

	t.Run("app version disabled", func(t *testing.T) {
		cfg := testConfig()
		cfg.DisabledAppVersions = []string{"0.1.0"}
		_, err := connectTestClient(Server{Config: cfg, Store: &memoryStore{}, Metrics: observe.NewMetrics()}).StartAuthorization(context.Background(), connect.NewRequest(&brokerv1.StartAuthorizationRequest{
			Provider: brokerv1.Provider_PROVIDER_GOOGLE, DesktopSessionId: "desktop-1", HandoffChallenge: "challenge-1",
			Application: &brokerv1.ApplicationMetadata{AppVersion: "0.1.0"},
		}))
		assertConnectBrokerError(t, err, connect.CodeFailedPrecondition, codes.AppVersionDisabled)
	})
}

func TestConnectRateLimitAndFailureMetrics(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 7, 9, 12, 0, 0, 0, time.UTC)
	metrics := observe.NewMetrics()
	limiter := ratelimit.New(time.Minute, func() time.Time { return now })
	client := connectTestClient(Server{Config: testConfig(), Store: &memoryStore{}, Clock: func() time.Time { return now }, Limiter: limiter, Metrics: metrics})
	request := &brokerv1.StartAuthorizationRequest{Provider: brokerv1.Provider_PROVIDER_GOOGLE, DesktopSessionId: "desktop-1", HandoffChallenge: "challenge-1"}
	for index := 0; index < limitStart; index++ {
		if _, err := client.StartAuthorization(context.Background(), connect.NewRequest(request)); err != nil {
			t.Fatalf("start %d: %v", index+1, err)
		}
	}
	_, err := client.StartAuthorization(context.Background(), connect.NewRequest(request))
	assertConnectBrokerError(t, err, connect.CodeResourceExhausted, codes.RateLimited)
	if metrics.RateLimitedCount(codes.SurfaceStart) != 1 {
		t.Fatalf("rate-limit metric = %d", metrics.RateLimitedCount(codes.SurfaceStart))
	}

	handoffMetrics := observe.NewMetrics()
	handoffClient := connectTestClient(Server{Config: testConfig(), Store: &memoryStore{}, Metrics: handoffMetrics})
	_, _ = handoffClient.ExchangeHandoff(context.Background(), connect.NewRequest(&brokerv1.ExchangeHandoffRequest{
		Provider: brokerv1.Provider_PROVIDER_GOOGLE, DesktopSessionId: "desktop-1", BrokerState: "missing", HandoffCode: "code-1", HandoffVerifier: "verifier-1",
	}))
	if handoffMetrics.HandoffFailureCount(codes.OutcomeNotFound) != 1 {
		t.Fatalf("handoff failure metric = %d", handoffMetrics.HandoffFailureCount(codes.OutcomeNotFound))
	}
}

func TestConnectRefreshTokenSupportsGoogleOnly(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 7, 9, 12, 0, 0, 0, time.UTC)
	providerClient := &http.Client{Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
		if err := req.ParseForm(); err != nil {
			t.Fatal(err)
		}
		if req.Form.Get("refresh_token") != "refresh-1" || req.Form.Get("client_secret") != "google-client-secret" {
			t.Fatalf("unexpected refresh form: %v", req.Form)
		}
		return &http.Response{
			StatusCode: http.StatusOK,
			Header:     make(http.Header),
			Body:       io.NopCloser(strings.NewReader(`{"access_token":"access-2","token_type":"Bearer","expires_in":3600}`)),
		}, nil
	})}
	srv := Server{Config: testConfig(), Store: &memoryStore{}, Clock: func() time.Time { return now }, HTTPClient: providerClient}
	client := brokerv1connect.NewOAuthBrokerServiceClient(&http.Client{
		Transport: brokerHandlerTransport{handler: srv.Handler()},
	}, "http://broker.test")

	response, err := client.RefreshToken(context.Background(), connect.NewRequest(&brokerv1.RefreshTokenRequest{
		Provider:     brokerv1.Provider_PROVIDER_GOOGLE,
		RefreshToken: "refresh-1",
		Scopes:       []string{"calendar.readonly"},
		Application:  &brokerv1.ApplicationMetadata{AppVersion: "0.1.0", Platform: "darwin-arm64"},
	}))
	if err != nil {
		t.Fatal(err)
	}
	if response.Msg.Token == nil || response.Msg.Token.AccessToken != "access-2" {
		t.Fatalf("unexpected token: %#v", response.Msg.Token)
	}
	if got := response.Msg.Token.Expiry.AsTime(); !got.Equal(now.Add(time.Hour)) {
		t.Fatalf("expiry = %s", got)
	}

	_, err = client.RefreshToken(context.Background(), connect.NewRequest(&brokerv1.RefreshTokenRequest{
		Provider:     brokerv1.Provider_PROVIDER_GITHUB,
		RefreshToken: "refresh-1",
	}))
	if connect.CodeOf(err) != connect.CodeUnimplemented {
		t.Fatalf("GitHub refresh code = %v, want %v", connect.CodeOf(err), connect.CodeUnimplemented)
	}
}

func TestConnectRevokeTokenValidatesProviderCredentialAndRevokes(t *testing.T) {
	t.Parallel()

	providerClient := &http.Client{Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
		switch {
		case req.Method == http.MethodPost:
			if err := req.ParseForm(); err != nil {
				t.Fatal(err)
			}
			if req.Form.Get("token") != "google-refresh" {
				t.Fatalf("google revoke token = %q", req.Form.Get("token"))
			}
			return &http.Response{StatusCode: http.StatusOK, Header: make(http.Header), Body: io.NopCloser(strings.NewReader(`{}`))}, nil
		case req.Method == http.MethodDelete:
			body, err := io.ReadAll(req.Body)
			if err != nil {
				t.Fatal(err)
			}
			if !strings.Contains(string(body), "github-access") {
				t.Fatalf("GitHub revoke body = %s", body)
			}
			return &http.Response{StatusCode: http.StatusNoContent, Header: make(http.Header), Body: io.NopCloser(strings.NewReader(""))}, nil
		default:
			t.Fatalf("unexpected provider request: %s %s", req.Method, req.URL)
			return nil, nil
		}
	})}
	srv := Server{Config: testConfig(), Store: &memoryStore{}, HTTPClient: providerClient}
	client := brokerv1connect.NewOAuthBrokerServiceClient(&http.Client{
		Transport: brokerHandlerTransport{handler: srv.Handler()},
	}, "http://broker.test")

	googleResponse, err := client.RevokeToken(context.Background(), connect.NewRequest(&brokerv1.RevokeTokenRequest{
		Provider:   brokerv1.Provider_PROVIDER_GOOGLE,
		Credential: &brokerv1.RevokeTokenRequest_RefreshToken{RefreshToken: "google-refresh"},
		Reason:     "user_disconnect",
	}))
	if err != nil || !googleResponse.Msg.Revoked {
		t.Fatalf("Google revoke = %#v, %v", googleResponse, err)
	}

	githubResponse, err := client.RevokeToken(context.Background(), connect.NewRequest(&brokerv1.RevokeTokenRequest{
		Provider:   brokerv1.Provider_PROVIDER_GITHUB,
		Credential: &brokerv1.RevokeTokenRequest_AccessToken{AccessToken: "github-access"},
		Reason:     "user_disconnect",
	}))
	if err != nil || !githubResponse.Msg.Revoked {
		t.Fatalf("GitHub revoke = %#v, %v", githubResponse, err)
	}

	_, err = client.RevokeToken(context.Background(), connect.NewRequest(&brokerv1.RevokeTokenRequest{
		Provider:   brokerv1.Provider_PROVIDER_GITHUB,
		Credential: &brokerv1.RevokeTokenRequest_RefreshToken{RefreshToken: "wrong-kind"},
	}))
	if connect.CodeOf(err) != connect.CodeInvalidArgument {
		t.Fatalf("credential mismatch code = %v, want %v", connect.CodeOf(err), connect.CodeInvalidArgument)
	}
}

func TestConnectExchangeHandoffReturnsTokenOnce(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 7, 9, 12, 0, 0, 0, time.UTC)
	const verifier = "handoff-verifier"
	payload, err := encryptTokenPayload(
		"google-client-secret",
		handoffAAD("state-1", "desktop-1", pkceS256(verifier)),
		tokenPayload{AccessToken: "access-1", RefreshToken: "refresh-1", TokenType: "Bearer", Expiry: now.Add(time.Hour)},
	)
	if err != nil {
		t.Fatal(err)
	}
	mem := &memoryStore{}
	if err := mem.SaveHandoff(context.Background(), store.HandoffRecord{
		CodeHash:              hashHandoffCode("handoff-code"),
		Provider:              "google",
		StateID:               "state-1",
		DesktopSessionID:      "desktop-1",
		HandoffChallenge:      pkceS256(verifier),
		EncryptedTokenPayload: payload,
		Scopes:                []string{"calendar.readonly"},
		ExpiresAt:             now.Add(time.Minute),
	}); err != nil {
		t.Fatal(err)
	}
	srv := Server{Config: testConfig(), Store: mem, Clock: func() time.Time { return now }}
	client := brokerv1connect.NewOAuthBrokerServiceClient(&http.Client{
		Transport: brokerHandlerTransport{handler: srv.Handler()},
	}, "http://broker.test")

	response, err := client.ExchangeHandoff(context.Background(), connect.NewRequest(&brokerv1.ExchangeHandoffRequest{
		Provider:         brokerv1.Provider_PROVIDER_GOOGLE,
		DesktopSessionId: "desktop-1",
		BrokerState:      "state-1",
		HandoffCode:      "handoff-code",
		HandoffVerifier:  verifier,
	}))
	if err != nil {
		t.Fatal(err)
	}
	if response.Msg.Token == nil || response.Msg.Token.AccessToken != "access-1" || response.Msg.Token.RefreshToken != "refresh-1" {
		t.Fatalf("unexpected token: %#v", response.Msg.Token)
	}

	_, err = client.ExchangeHandoff(context.Background(), connect.NewRequest(&brokerv1.ExchangeHandoffRequest{
		Provider:         brokerv1.Provider_PROVIDER_GOOGLE,
		DesktopSessionId: "desktop-1",
		BrokerState:      "state-1",
		HandoffCode:      "handoff-code",
		HandoffVerifier:  verifier,
	}))
	if connect.CodeOf(err) != connect.CodeAlreadyExists {
		t.Fatalf("replay code = %v, want %v", connect.CodeOf(err), connect.CodeAlreadyExists)
	}
}

type brokerHandlerTransport struct {
	handler http.Handler
}

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return f(req)
}

func (t brokerHandlerTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	recorder := httptest.NewRecorder()
	t.handler.ServeHTTP(recorder, req)
	return recorder.Result(), nil
}

func connectTestClient(srv Server) brokerv1connect.OAuthBrokerServiceClient {
	return brokerv1connect.NewOAuthBrokerServiceClient(&http.Client{
		Transport: brokerHandlerTransport{handler: srv.Handler()},
	}, "http://broker.test")
}

func assertConnectBrokerError(t *testing.T, err error, wantConnect connect.Code, wantBroker string) {
	t.Helper()
	var connectErr *connect.Error
	if !errors.As(err, &connectErr) {
		t.Fatalf("expected Connect error, got %v", err)
	}
	if connectErr.Code() != wantConnect {
		t.Fatalf("Connect code = %v, want %v", connectErr.Code(), wantConnect)
	}
	for _, detail := range connectErr.Details() {
		value, detailErr := detail.Value()
		if detailErr != nil {
			t.Fatal(detailErr)
		}
		if brokerDetail, ok := value.(*brokerv1.BrokerErrorDetail); ok && brokerDetail.Code == wantBroker {
			return
		}
	}
	t.Fatalf("broker code %q missing from %v", wantBroker, connectErr.Details())
}

type saveFailureStore struct{ memoryStore }

func (*saveFailureStore) SaveOAuthState(context.Context, store.OAuthState) error {
	return errors.New("save failed")
}

type consumeFailureStore struct{ memoryStore }

func (*consumeFailureStore) ConsumeHandoff(context.Context, string, string, string, string, string, time.Time) (store.HandoffRecord, error) {
	return store.HandoffRecord{}, errors.New("consume failed")
}

type recordingLimiter struct {
	keys []string
}

func (l *recordingLimiter) Allow(key string, _ int) bool {
	l.keys = append(l.keys, key)
	return true
}
