// Package codes holds stable string identifiers shared by the OAuth broker and
// desktop client. Prefer these constants over raw literals when emitting or
// matching Connect error details, metric labels, and log event names.
package codes

// Broker error identifiers returned in BrokerErrorDetail.code.
const (
	InvalidConfig                             = "invalid_config"
	ProviderNotConfigured                     = "provider_not_configured"
	DatastoreUnavailable                      = "datastore_unavailable"
	DesktopSessionAndHandoffChallengeRequired = "desktop_session_id_and_handoff_challenge_required"
	InvalidDesktopHandoffRedirect             = "invalid_desktop_handoff_redirect"
	RandomStateFailed                         = "random_state_failed"
	RandomVerifierFailed                      = "random_verifier_failed"
	StatePersistFailed                        = "state_persist_failed"
	AuthURLFailed                             = "auth_url_failed"
	HandoffFieldsRequired                     = "handoff_fields_required"
	HandoffAlreadyUsed                        = "handoff_already_used"
	HandoffExpired                            = "handoff_expired"
	HandoffNotFound                           = "handoff_not_found"
	HandoffStateMismatch                      = "handoff_state_mismatch"
	HandoffVerifierMismatch                   = "handoff_verifier_mismatch"
	HandoffConsumeFailed                      = "handoff_consume_failed"
	HandoffPayloadInvalid                     = "handoff_payload_invalid"
	RefreshTokenRequired                      = "refresh_token_required"
	AccessTokenRequired                       = "access_token_required"
	InvalidRefreshToken                       = "invalid_refresh_token"
	GoogleTokenRefreshFailed                  = "google_token_refresh_failed"
	GoogleRevokeFailed                        = "google_revoke_failed"
	GitHubRevokeFailed                        = "github_revoke_failed"
	RateLimited                               = "rate_limited"
	AuthDisabled                              = "auth_disabled"
	RefreshDisabled                           = "refresh_disabled"
	AppVersionDisabled                        = "app_version_disabled"
	OperationNotSupported                     = "operation_not_supported"
)

// Surfaces used for rate-limit keys, kill-switch metrics, and log fields.
const (
	SurfaceStart          = "start"
	SurfaceCallback       = "callback"
	SurfaceHandoff        = "handoff"
	SurfaceHandoffFailure = "handoff_failure"
	SurfaceRefresh        = "refresh"
	SurfaceRefreshFailure = "refresh_failure"
	SurfaceRevoke         = "revoke"
)

// Rate-limit key prefixes (joined with dimensions via ratelimit.Key).
const (
	LimitKeyStart       = "start"
	LimitKeyCallback    = "callback"
	LimitKeyHandoff     = "handoff"
	LimitKeyHandoffFail = "handoff_fail"
	LimitKeyRefresh     = "refresh"
	LimitKeyRefreshFail = "refresh_fail"
	LimitKeyRevoke      = "revoke"
)

// Metric / log outcome and reason labels.
const (
	OutcomeOK                 = "ok"
	OutcomeGoogleError        = "google_error"
	OutcomeProviderError      = "provider_error"
	OutcomeMissingParams      = "missing_params"
	OutcomeStateAlreadyUsed   = "state_already_used"
	OutcomeStateExpired       = "state_expired"
	OutcomeStateNotFound      = "state_not_found"
	OutcomeStateError         = "state_error"
	OutcomeTokenExchangeFail  = "token_exchange_failed"
	OutcomeHandoffMintFailed  = "handoff_mint_failed"
	OutcomeSealFailed         = "seal_failed"
	OutcomeHandoffPersistFail = "handoff_persist_failed"
	OutcomeHandoffURLFailed   = "handoff_url_failed"
	OutcomeAlreadyUsed        = "already_used"
	OutcomeExpired            = "expired"
	OutcomeNotFound           = "not_found"
	OutcomeStateMismatch      = "state_mismatch"
	OutcomeConsumeFailed      = "consume_failed"
	OutcomePayloadInvalid     = "payload_invalid"
	OutcomeInvalidGrant       = "invalid_grant"
	OutcomeGoogleFailed       = "google_failed"
	OutcomeGitHubFailed       = "github_failed"
	OutcomeAlreadyRevoked     = "already_revoked"
)

// Quota-risk metric signals.
const (
	QuotaStateReplay     = "state_replay"
	QuotaHandoffReplay   = "handoff_replay"
	QuotaHandoffMismatch = "handoff_mismatch"
	QuotaInvalidGrant    = "invalid_grant"
)

// Structured log event names.
const (
	EventAuthStart   = "auth_start"
	EventCallback    = "callback"
	EventHandoff     = "handoff"
	EventRefresh     = "refresh"
	EventRevoke      = "revoke"
	EventRateLimited = "rate_limited"
	EventKillSwitch  = "kill_switch"
)

// Google token endpoint error codes we special-case.
const (
	GoogleInvalidGrant = "invalid_grant"
)

// KillSwitchVersionSuffix is appended to a surface when an app version is blocked.
const KillSwitchVersionSuffix = "_version"
