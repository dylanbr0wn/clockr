# shiet OAuth Broker

The OAuth broker is a separate deployable Go binary:

```bash
go run ./cmd/oauth-broker
```

It keeps Google Web OAuth and GitHub OAuth App client secrets in the server
environment and stores only short-lived coordination records in the configured
datastore. It does not create durable tables for provider access tokens,
refresh tokens, Calendar event data, or GitHub repository data.

## Environment

Required:

- `SHIET_BROKER_PUBLIC_ORIGIN`: public HTTPS origin, for example
  `https://auth.shiet.app`.
- `SHIET_BROKER_GOOGLE_CLIENT_ID`: Google Web OAuth client id.
- `SHIET_BROKER_GOOGLE_CLIENT_SECRET`: Google Web OAuth client secret.
- `SHIET_BROKER_GITHUB_CLIENT_ID`: GitHub OAuth App client id (required to
  enable GitHub routes).
- `SHIET_BROKER_GITHUB_CLIENT_SECRET`: GitHub OAuth App client secret (required
  with the GitHub client id).
- `SHIET_BROKER_SLACK_CLIENT_ID`: Slack app client id (required to enable Slack
  routes).
- `SHIET_BROKER_SLACK_CLIENT_SECRET`: Slack app client secret (required with the
  Slack client id).
- `SHIET_BROKER_BITBUCKET_CLIENT_ID`: Bitbucket OAuth consumer client id
  (required to enable Bitbucket routes).
- `SHIET_BROKER_BITBUCKET_CLIENT_SECRET`: Bitbucket OAuth consumer client secret
  (required with the Bitbucket client id).
- `SHIET_BROKER_DATASTORE_DSN`: SQLite DSN for the broker datastore.

Optional:

- `SHIET_BROKER_LISTEN_ADDR`: listen address, default `:8080`. If unset,
  Railway's `PORT` variable is used when present.
- `SHIET_BROKER_DESKTOP_HANDOFF_URL`: desktop handoff URL, default
  `shiet://oauth/google/handoff`.
- `SHIET_BROKER_STATE_TTL`: OAuth state TTL, default `5m`, maximum `10m`.
- `SHIET_BROKER_HANDOFF_TTL`: handoff TTL, default `2m`, maximum `5m`.
- `SHIET_BROKER_GOOGLE_SCOPES`: space- or comma-separated Google scopes,
  default `https://www.googleapis.com/auth/calendar.readonly`.
- `SHIET_BROKER_GITHUB_DESKTOP_HANDOFF_URL`: GitHub desktop handoff URL,
  default `shiet://oauth/github/handoff`.
- `SHIET_BROKER_GITHUB_SCOPES`: space- or comma-separated GitHub OAuth App
  scopes, default `repo`.
- `SHIET_BROKER_SLACK_DESKTOP_HANDOFF_URL`: Slack desktop handoff URL, default
  `shiet://oauth/slack/handoff`.
- `SHIET_BROKER_SLACK_SCOPES`: space- or comma-separated Slack user scopes,
  default `channels:history groups:history channels:read groups:read`.
- `SHIET_BROKER_BITBUCKET_DESKTOP_HANDOFF_URL`: Bitbucket desktop handoff URL,
  default `shiet://oauth/bitbucket/handoff`.
- `SHIET_BROKER_BITBUCKET_SCOPES`: space- or comma-separated Bitbucket OAuth
  consumer scopes, default `account repository`.
- `SHIET_BROKER_AUTH_DISABLED`: when `true`/`1`/`yes`/`on`, reject start,
  callback, and handoff with `auth_disabled`. RPCs use Connect
  `FailedPrecondition`; callbacks return an HTTP 403 page. Revoke stays enabled.
- `SHIET_BROKER_REFRESH_DISABLED`: when truthy, reject refresh with
  `refresh_disabled` and Connect `FailedPrecondition`.
- `SHIET_BROKER_DISABLED_APP_VERSIONS`: comma-separated desktop `app_version`
  values that receive `app_version_disabled` on start/refresh.

## Abuse Controls

In-process fixed-window rate limits (per IP `/24` bucket unless noted), reset
each minute:

| Surface | Limit | Notes |
|---------|-------|-------|
| start | 10 / min | shared Google + GitHub budget before minting OAuth state |
| callback | 30 / min | shared provider budget; HTML responses; 429 page on overage |
| handoff | 20 / min | shared provider budget for all exchange attempts |
| handoff failures | 5 / min | stricter: IP + desktop session + handoff-code hash |
| refresh | 60 / min | Google refresh attempts; GitHub OAuth App tokens do not refresh |
| refresh failures | 10 / min | additional budget for `invalid_grant` / Google failures |
| revoke | 20 / min | stays available under auth/refresh kill switches |

Over-limit RPCs return Connect `ResourceExhausted` with
`BrokerErrorDetail.code = "rate_limited"`. The callback returns an HTTP 429 HTML
error page.

Kill-switch error codes for the desktop client:

- `auth_disabled`
- `refresh_disabled`
- `app_version_disabled`

RPC errors carry these stable identifiers in `BrokerErrorDetail.code`; clients
must not parse human-readable Connect error messages.

See ADR-0001 threat model (Broker Abuse / Quota Abuse) for the control intent;
this document is the operator runbook.

## Observability

Structured JSON logs go to **stdout** via the shared redacting zerolog logger
(`internal/log`). Same stack as the desktop app; see
[logging.md](logging.md) for the shared overview and desktop file path.
Redaction rules follow [ADR-0001](adr/0001-secret-only-google-oauth-broker.md).
Never log:

- Google or GitHub authorization codes
- handoff codes / verifiers
- Google access/refresh tokens or GitHub access tokens
- `client_secret` or other secret-bearing fields

Safe fields include event name, surface, outcome/reason codes, IP bucket,
`app_version`, and `platform`.

`GET /metrics` exposes Prometheus text counters, including:

- `broker_auth_starts_total`
- `broker_auth_start_failures_total`
- `broker_callback_outcomes_total{reason=...}`
- `broker_handoff_success_total` / `broker_handoff_failures_total{reason=...}`
- `broker_refresh_success_total` / `broker_refresh_failures_total{reason=...}`
- `broker_revoke_success_total` / `broker_revoke_outcomes_total{reason=...}`
- `broker_rate_limited_total{surface=...}`
- `broker_kill_switch_total{surface=...}`
- `broker_quota_risk_total{signal=...}` (`invalid_grant`, `handoff_replay`,
  `handoff_mismatch`, `state_replay`)

Handoff failure reasons: `already_used`, `expired`, `not_found`,
`state_mismatch`, plus internal consume/payload failures.

## Quota Alerting And Abuse Response

### Google Cloud project

1. In Google Cloud Console → APIs & Services → OAuth consent screen / Credentials
   for the Web client used by the broker, enable alerts (or Cloud Monitoring) for
   unusual OAuth token error rates and project quota exhaustion.
2. Watch for spikes in `invalid_grant`, authorization denials, and token endpoint
   4xx/5xx against the shared Web client.
3. Keep Calendar API usage local to the desktop in v1; broker quota risk is
   primarily OAuth start/token/refresh/revoke volume.

### Broker deployment (Railway)

1. Scrape or periodically fetch `GET /metrics` (or ship stdout JSON logs to a
   log drain) and alert on:
   - rising `broker_rate_limited_total`
   - `broker_quota_risk_total` for `invalid_grant` or `handoff_replay`
   - sustained `broker_handoff_failures_total` / refresh failures
2. Correlate with Railway request volume and error rates on
   `/v1/google/oauth/*` and `/v1/github/oauth/*`.

### Incident response steps

1. **Contain**: set `SHIET_BROKER_AUTH_DISABLED=true` and/or
   `SHIET_BROKER_REFRESH_DISABLED=true` on the Railway service and redeploy /
   restart so new auth and/or refresh stops. Prefer leaving revoke enabled so
   users can disconnect.
2. **Narrow**: if abuse is version-specific, set
   `SHIET_BROKER_DISABLED_APP_VERSIONS` instead of a global kill switch.
3. **Rotate** (if secret exposure is suspected): rotate the affected Google Web
   OAuth or GitHub OAuth App client secret, update the sealed Railway variable,
   and restart the broker. Existing tokens may need reconnect after the
   provider invalidates grants.
4. **Investigate**: inspect `/metrics` and redacted logs for IP buckets,
   outcomes, and quota-risk signals. Do not dump request bodies that may contain
   tokens.
5. **Restore**: clear kill-switch env vars after the spike stops; confirm start
   and refresh succeed from a known-good desktop build.

## Deployment Notes

- HTTPS/domain: provision `auth.shiet.app` with HTTPS and HSTS. Configure the
  Google Web OAuth redirect URI as
  `https://auth.shiet.app/v1/google/oauth/callback`, and configure the GitHub
  OAuth App callback URL as
  `https://auth.shiet.app/v1/github/oauth/callback`.
- Secret management: inject both provider client ids and client secrets from a
  managed secret store. Restrict runtime access and keep audit logs for reads
  and rotations.
- Datastore: start with a small SQLite database on durable storage for the first
  deployable broker. The schema contains OAuth state and handoff coordination
  records with expiry and one-time-use fields only. Rate-limit counters and
  kill-switch state are in-process / env-driven for the single-replica deploy.
- Logging / metrics: see Observability above.
- Operational ownership: assign an owner for deploys, uptime, datastore backups,
  secret rotation, Google OAuth consent-screen health, quota alerts, and
  emergency disablement.

## Railway

This repo includes Railway config-as-code for the broker service:

- `railway.json`: selects the broker Dockerfile and `/readyz` healthcheck.
- `deploy/railway/oauth-broker.Dockerfile`: installs pinned Buf, regenerates
  gitignored Connect sources under `gen/`, then builds only `./cmd/oauth-broker`.
- `.railwayignore`: keeps the Wails frontend/build artifacts out of Railway's
  upload context.

Railway injects a `PORT` environment variable at runtime. The broker listens on
that port when `SHIET_BROKER_LISTEN_ADDR` is not set.

Recommended Railway service variables:

- `SHIET_BROKER_PUBLIC_ORIGIN=https://auth.shiet.app`
- `SHIET_BROKER_GOOGLE_CLIENT_ID=<Google Web OAuth client id>`
- `SHIET_BROKER_GOOGLE_CLIENT_SECRET=<Google Web OAuth client secret>`
- `SHIET_BROKER_GITHUB_CLIENT_ID=<GitHub OAuth App client id>`
- `SHIET_BROKER_GITHUB_CLIENT_SECRET=<GitHub OAuth App client secret>`
- `SHIET_BROKER_GITHUB_SCOPES=repo`
- `SHIET_BROKER_DATASTORE_DSN=file:/data/oauth-broker.sqlite`

Mark both provider client secrets as sealed Railway variables. Attach a Railway
Volume at `/data` before using the SQLite DSN above.

To smoke-test the Docker image locally:

```bash
./scripts/railway-broker-smoke.sh
```

### Why SQLite Instead Of An In-Memory Store?

An in-memory store would be simpler and can work for a single-process local
demo, but it is brittle for the deployed OAuth flow:

- OAuth state has to survive multiple HTTP requests: start, Google callback,
  and handoff.
- A process restart, deploy, or crash would drop every in-flight auth attempt.
- Multiple replicas would not share state, so callbacks could land on an
  instance that never saw the start request.
- The DYL-81 acceptance criteria asks for short-lived records to be persisted
  with expiry and one-time-use semantics.

SQLite is the smallest durable datastore we can run without adding a separate
service. On Railway, that means mounting a Volume at `/data`. The tradeoff is
that Railway volume-backed redeploys can have a short downtime window. If we
need zero-downtime deploys, multiple replicas, or multi-region later, move the
same store contract to Redis or Postgres and keep the no-persistent-Google-token
schema guarantees.

## Current Service Surface

The broker also exposes the generated
`shiet.broker.v1.OAuthBrokerService` Connect service for start, handoff,
Google refresh, and provider revoke. Connect is the only application transport;
there are no REST aliases for these operations. Provider callbacks and
operational endpoints remain ordinary HTTP.

- `GET /healthz`: process health.
- `GET /readyz`: validates configuration and checks datastore connectivity.
- `GET /metrics`: Prometheus text metrics for auth/abuse signals (no secrets).
- `OAuthBrokerService.StartAuthorization`: creates a provider-bound,
  short-lived broker state and returns an authorization URL.
- `GET /v1/google/oauth/callback`: exchanges Google's authorization code with
  the server-side client secret, mints a short-lived one-time handoff, and
  renders a return page with broker_state + handoff_code (no token material).
- `GET /v1/github/oauth/callback`: exchanges GitHub's authorization code using
  the server-only OAuth App secret and mints a short-lived handoff.
- `OAuthBrokerService.ExchangeHandoff`: returns provider token material once,
  bound to the initiating desktop session and handoff verifier.
- `OAuthBrokerService.RefreshToken`: exchanges a desktop-held Google refresh
  token for new access-token material. GitHub returns `Unimplemented`.
- `OAuthBrokerService.RevokeToken`: revokes a Google refresh token or GitHub
  access token using server-side provider credentials.

For Google, already-revoked / `invalid_token` responses are treated as success.
The broker does not persist the token or any disconnected-account record.

GitHub refresh is deliberately unsupported. This implementation uses
GitHub OAuth App user access tokens, not GitHub App installation tokens. If a
token becomes invalid, the desktop marks the connection `needs_reauth` and the
user reconnects. GitHub documents the web authorization-code exchange and PKCE
parameters in [Authorizing OAuth apps](https://docs.github.com/en/apps/oauth-apps/building-oauth-apps/authorizing-oauth-apps),
and the server-authenticated single-token revocation operation in
[REST API endpoints for OAuth authorizations](https://docs.github.com/en/rest/apps/oauth-applications).

## Extending to another provider

Provider protocol metadata lives in `internal/integration/oauth` as a static
descriptor (endpoints, scopes, auth URL validation, capabilities). Runtime
credentials stay in broker env / desktop BYO config and are injected at call
time. Shared helpers `BuildAuthorizationURL` and `ExchangeAuthorizationCode`
are used by both local/BYO desktop OAuth and the broker callback exchange.
Desktop broker connect uses the provider-neutral `oauth.BrokerFlow`; refresh
and revoke stay as thin provider adapters when the provider supports them.

See ADR-0001 "Provider extension boundary (DYL-97)" for the full checklist.
