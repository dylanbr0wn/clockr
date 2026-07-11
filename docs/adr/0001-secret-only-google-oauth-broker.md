# ADR-0001: Secret-Only OAuth Broker

- Status: Accepted
- Date: 2026-07-08
- Linear: DYL-74, DYL-95, DYL-97

## Context

shiet currently runs Google OAuth from the local desktop process. The current
implementation uses the system browser, a loopback redirect, PKCE, state
checking, and OS keychain-backed token storage. That is the right native-app
shape for local OAuth, but it does not solve the product security concern for
public shiet builds: a shared Google OAuth `client_secret` shipped in the app
binary or config can be extracted and reused by impersonators, creating quota,
reputation, and consent-screen risk for shiet's Google Cloud project.

The relevant boundary is not "can the local OAuth flow work?" It can. The
boundary is whether shiet can treat a distributed shared Google secret as a
confidential credential. It cannot.

Standards and provider guidance support that split:

- RFC 8252 recommends browser-based OAuth for native apps, loopback redirects on
  desktop, and PKCE for public native clients.
- RFC 8252 also says statically included secrets in apps distributed to multiple
  users should not be treated as confidential, and native clients that include a
  shared secret are still public clients subject to impersonation.
- Google's web-server OAuth flow uses a server-side `client_secret` when a web
  application exchanges an authorization code.
- The IETF browser-based apps draft describes backend-for-frontend and
  token-mediating backend patterns where a server-side component acts as the
  confidential OAuth client. shiet's broker is closest to a narrow
  token-mediating backend for a native desktop app, with no persistent
  server-side Google token storage.

## Decision

shiet public builds will use a small secret-only Google OAuth broker for Google
Calendar authorization. The broker owns the confidential Google Web OAuth client
and stores the Google `client_secret` only server-side. The desktop app continues
to store Google refresh and access tokens only in the OS keychain.

The broker handles:

1. Starting authorization at `https://auth.shiet.app`.
2. Receiving Google's OAuth callback.
3. Exchanging Google's authorization code for tokens with the server-side
   `client_secret`.
4. Returning token material to the initiating desktop app through a short-lived,
   one-time handoff code.
5. Refreshing tokens when the desktop sends its refresh token to the broker.
6. Revoking/disconnecting tokens at the user's request.

The broker must not persist Google refresh tokens, Google access tokens, or
Google Calendar event data. It may persist only server configuration,
short-lived OAuth state records, short-lived handoff records, logs, metrics,
rate-limit counters, audit events, and revocation or kill-switch configuration.

BYO credentials remain available for development and advanced users. Public
release builds should default to the broker when no explicit local Google OAuth
credentials are configured.

### GitHub provider extension (DYL-95)

The same confidentiality boundary applies to the shared GitHub OAuth App:
public desktop builds must not ship its `client_secret`. The broker therefore
also owns GitHub's web authorization code exchange.
[ADR-0002](0002-connect-protobuf-api-boundary.md) defines the application
transport; integration connect/disconnect uses Connect `IntegrationService` per
[ADR-0004](0004-standardized-integrations-settings-surface.md). Only the provider
callback remains an ordinary `/v1/github/oauth/callback` route. GitHub
state and handoff records use the same expiry, one-time-use, verifier binding,
rate limiting, kill switches, redacted observability, and no-persistent-token
rules as Google.

The first GitHub broker integration uses an OAuth App user access token rather
than GitHub App installation tokens. This matches shiet's user-level repository
picker and evidence access. The broker requests `repo`, hands
the resulting `gho_` token to the desktop keychain, and never persists it.

GitHub OAuth App user access tokens do not expose the short-lived installation
token refresh lifecycle used by GitHub Apps. Accordingly, this integration has
no GitHub refresh route: an invalid or revoked token moves the connection to
`needs_reauth` and the user reconnects. Disconnect sends the desktop-held access
token to the broker, which calls GitHub's app-token revocation endpoint using
server-side Basic authentication with the OAuth App client id and secret. The
broker does not retain the submitted token or a disconnected-account record.

Local/BYO GitHub OAuth remains available with `github.auth_mode: local` and
locally configured OAuth App credentials. The PAT path introduced by DYL-56 is
also retained in either mode as an explicit advanced-user escape hatch. Public
builds default to `github.auth_mode: broker` and clear any loaded desktop GitHub
`client_secret` before runtime wiring.

### Provider extension boundary (DYL-97)

Adding another OAuth provider should be a small adapter change, not a copy of
the desktop broker loop or new branches throughout the HTTP server:

1. Register a static `oauth.Provider` descriptor in
   `internal/integration/oauth` (id, display name, endpoints, scopes, auth URL
   validation, auth-style, authorization params, refresh/revoke capabilities).
   Descriptors contain no client secrets or deployment credentials.
2. Inject runtime `oauth.ClientCredentials` from desktop BYO config or broker
   environment variables.
3. Call shared `oauth.BuildAuthorizationURL` and
   `oauth.ExchangeAuthorizationCode` from both local/BYO `oauth.Flow` and the
   broker callback exchange.
4. Wire desktop connect through the shared `oauth.BrokerFlow` engine; keep
   provider-specific refresh/revoke adapters only when the provider supports
   them (Google refresh+revoke; GitHub revoke-only / no refresh).
5. Reuse shared broker↔desktop JSON contract types in `oauth` protocol structs.

Local/BYO OAuth and brokered OAuth remain separate trust boundaries: local is a
direct desktop-to-provider code exchange; broker is a server-side confidential
exchange plus a short-lived one-time desktop handoff.

## Options Considered

### Local-only desktop OAuth

Keep the current local loopback OAuth flow and configure shiet's shared Google
Desktop OAuth credentials in the app.

- Pros: smallest app change; no hosted infrastructure; aligns with native OAuth
  mechanics.
- Cons: does not protect a shared shiet `client_secret`; extracted credentials
  can be reused by impersonators; limited leverage for quota abuse monitoring or
  emergency cutoff.
- Outcome: keep as the shape for BYO/development credentials, but do not use for
  public builds with shared shiet credentials.

### BYO credentials only

Require every user to create and configure their own Google OAuth client.

- Pros: no shared shiet secret or quota pool; no broker operations.
- Cons: poor public-user onboarding; users inherit Google Cloud setup burden;
  support and consent-screen instructions become part of the product.
- Outcome: keep as an escape hatch, not the public default.

### Secret-only broker

Host a minimal OAuth broker that keeps the Google client secret server-side but
returns Google tokens to the desktop for local keychain storage.

- Pros: protects the shared Google `client_secret`; keeps Calendar data and
  persistent tokens off shiet servers; adds central rate limiting, monitoring,
  quota alerts, and a kill switch.
- Cons: broker transiently handles sensitive token material; refresh requires
  sending the user's refresh token over HTTPS to the broker; adds uptime,
  security, and operations ownership.
- Outcome: accepted for public builds.

### Full BFF/proxy

Move OAuth responsibilities and Google Calendar API calls behind a hosted
backend.

- Pros: strongest server-side control over tokens, API calls, quota shaping, and
  abuse detection.
- Cons: turns shiet into a hosted calendar-sync product; requires persistent
  server-side token or session storage; likely stores or processes Calendar data
  server-side; materially larger privacy and operations footprint.
- Outcome: reject for the first broker version.

## Broker API Contract

ADR-0002 supersedes the original REST wire contract. The generated Connect
service in `proto/shiet/broker/v1/oauth_broker.proto` is the source of truth for
operation fields and encoding. The examples below use canonical Protobuf JSON
only to illustrate message shape; desktop clients use generated messages. The
provider callbacks remain HTTPS browser endpoints that render HTML. The
security properties below remain contract requirements.

### Start Auth

`OAuthBrokerService.StartAuthorization`

Request:

```json
{
  "provider": "PROVIDER_GOOGLE",
  "desktopSessionId": "random app-generated identifier",
  "handoffChallenge": "hash of a desktop-held handoff verifier",
  "application": {
    "appVersion": "0.1.0",
    "platform": "darwin-arm64"
  },
  "desktopHandoffRedirect": "http://127.0.0.1:49152/oauth/handoff"
}
```

Response:

```json
{
  "authUrl": "https://accounts.google.com/o/oauth2/v2/auth?...",
  "brokerState": "opaque state identifier",
  "expiresAt": "2026-07-08T12:05:00Z"
}
```

Requirements:

- Generate high-entropy OAuth state server-side.
- Generate the Google PKCE verifier and challenge server-side. The broker must
  retain the verifier only in the short-lived OAuth state record so it can
  perform the callback token exchange.
- Persist an OAuth state record with the desktop session id, Google PKCE
  verifier or encrypted verifier, Google PKCE challenge, handoff challenge,
  requested scopes, app metadata, source IP class, and expiration.
- Expire unused state after 5 minutes.
- Do not accept caller-supplied redirect URIs.
- Scope requests to the minimum Google Calendar scopes shiet needs.

### Google Callback

`GET /v1/google/oauth/callback?code=...&state=...`

Behavior:

- Validate state existence, expiry, and unused status.
- Exchange Google's authorization code with the confidential Google Web OAuth
  client id, server-side `client_secret`, broker redirect URI, and the
  server-generated Google PKCE verifier stored on the OAuth state record.
- Create a handoff record containing encrypted token material, the desktop
  session id, state id, handoff challenge, issue time, expiry, and unused status.
- Mark the OAuth state used before responding.
- Render a minimal browser page telling the user to return to shiet. The page
  may include a fixed shiet return link, such as
  `shiet://oauth/google/handoff?...`, containing only the broker state and
  handoff code. It must not include Google token material.

Requirements:

- Callback handling is idempotent only for safe duplicate browser refreshes. It
  must not mint multiple valid handoff records for the same OAuth state.
- Handoff records expire after at most 2 minutes.
- Logs must not include authorization codes, refresh tokens, access tokens, or
  handoff codes.

### One-Time Handoff Exchange

`OAuthBrokerService.ExchangeHandoff`

Request:

```json
{
  "provider": "PROVIDER_GOOGLE",
  "desktopSessionId": "same app-generated identifier",
  "brokerState": "opaque state identifier",
  "handoffCode": "short-lived one-time code",
  "handoffVerifier": "desktop-held verifier matching the start challenge",
  "application": {
    "appVersion": "0.1.0",
    "platform": "darwin-arm64"
  }
}
```

Response:

```json
{
  "provider": "PROVIDER_GOOGLE",
  "accountHint": "user@example.com",
  "scopes": ["https://www.googleapis.com/auth/calendar.readonly"],
  "token": {
    "accessToken": "google access token",
    "refreshToken": "google refresh token",
    "tokenType": "Bearer",
    "expiry": "2026-07-08T13:00:00Z"
  }
}
```

Requirements:

- Handoff code must be high entropy, single use, short lived, and stored only as
  a hash.
- Handoff exchange must match the initiating desktop session id, broker state,
  and handoff verifier.
- On successful exchange, delete token material or make it cryptographically
  unrecoverable before returning the response.
- On failed exchange, rate limit by handoff code hash, desktop session id, IP
  bucket, and app version.
- The desktop app persists the returned token only in the OS keychain.

### Refresh Token Exchange

`OAuthBrokerService.RefreshToken`

Request:

```json
{
  "provider": "PROVIDER_GOOGLE",
  "refreshToken": "google refresh token from the desktop keychain",
  "scopes": ["https://www.googleapis.com/auth/calendar.readonly"],
  "application": {
    "appVersion": "0.1.0",
    "platform": "darwin-arm64"
  }
}
```

Response:

```json
{
  "token": {
    "accessToken": "new google access token",
    "refreshToken": "optional rotated google refresh token",
    "tokenType": "Bearer",
    "expiry": "2026-07-08T13:00:00Z"
  }
}
```

Requirements:

- Broker submits the refresh token to Google's token endpoint with the
  server-side Google Web OAuth client id and `client_secret`.
- Broker must not persist the submitted refresh token or returned token
  material.
- If Google rotates the refresh token, return the rotated value to the desktop
  so the keychain entry can be replaced.
- Apply stricter rate limits to refresh failures than successful refreshes.
- Treat repeated `invalid_grant` or suspicious refresh patterns as abuse signals.

### Disconnect/Revoke

`OAuthBrokerService.RevokeToken`

Request:

```json
{
  "provider": "PROVIDER_GOOGLE",
  "refreshToken": "google refresh token from the desktop keychain",
  "reason": "user_disconnect"
}
```

Response:

```json
{
  "revoked": true
}
```

Requirements:

- Broker calls Google's revocation endpoint with the provided token.
- Broker does not persist the token.
- Desktop deletes the local keychain token after successful revoke, and may also
  delete it after a best-effort revoke failure when the user explicitly
  disconnects.

## Threat Model

### Client Secret Extraction

Threat: an attacker extracts shiet's shared Google `client_secret` from a
public binary or config and reuses it to impersonate shiet.

Controls:

- Public builds do not contain the shared Google `client_secret`.
- The broker stores the secret in a managed secret store and injects it only into
  the server runtime.
- BYO/local credential mode is visibly separate from broker mode and is not used
  for public shared credentials.

Residual risk: if broker infrastructure or secret manager access is compromised,
the Google `client_secret` can still be abused. Mitigate with least privilege,
audit logs, rotation, and a revocation runbook.

### Broker Abuse

Threat: attackers use the broker as a free Google token exchange service or
automation surface.

Controls:

- Rate limit by IP bucket, app version, start state, handoff exchange, refresh
  failure rate, and Google account hint where available.
- Require broker-issued state and desktop-session binding for handoff.
- Consider optional release-channel attestation over time: signed app update
  channel metadata, notarized build identifiers on macOS, Windows signing
  metadata, and a rotating public-build client id. Do not treat attestation as a
  secret; use it as an abuse signal.
- Add quota alerts and dashboards for OAuth starts, callbacks, handoff failures,
  refresh failures, Google token errors, and revoke volume.
- Maintain a kill switch that can disable broker auth, refresh, or specific app
  versions without a desktop release.
- Operator runbook for rate limits, metrics, kill switches, and quota response:
  `docs/oauth-broker.md`.

Residual risk: a modified desktop client can still call public broker endpoints.
The broker reduces shared-secret extraction risk; it does not prove every caller
is an unmodified shiet binary.

### Handoff-Code Replay

Threat: a handoff code is intercepted from the browser page, logs, clipboard, or
local IPC and replayed by another caller.

Controls:

- Handoff codes are high entropy, stored hashed, one-time use, and expire within
  2 minutes.
- Handoff exchange must include the desktop session id, broker state, and a
  desktop-held handoff verifier matching the challenge supplied at start.
- Successful and terminal failed handoff attempts consume or lock the handoff
  record.
- Handoff codes never appear in logs, analytics, crash reports, or URLs loaded
  with third-party assets.

Residual risk: malware on the user's machine can steal both the handoff code and
the desktop-held verifier. That is treated as local compromise.

### Refresh-Token Exposure In Transit

Threat: the desktop sends its Google refresh token to the broker for refresh,
and the token is exposed in transit or server telemetry.

Controls:

- HTTPS-only with HSTS for `auth.shiet.app`.
- No refresh tokens in URLs, logs, metrics labels, traces, exception messages, or
  durable queues.
- Request body size limits and structured redaction at the HTTP boundary.
- Broker refresh handler keeps tokens in process memory only long enough to call
  Google and return the response.

Residual risk: the broker runtime necessarily sees refresh tokens transiently.
This is the price of keeping the Google `client_secret` server-side without
moving persistent token storage server-side.

### Quota Abuse

Threat: attackers drive OAuth starts, token refreshes, or Calendar usage against
shiet's Google project and harm quota, reputation, or consent-screen standing.

Controls:

- Central broker rate limits before token exchange and refresh.
- Google Cloud quota alerts and OAuth error dashboards.
- Per-version and per-IP anomaly detection.
- Emergency kill switch for auth and refresh.
- Keep Calendar API calls local in v1, so the broker is not a general Calendar
  API proxy.

Residual risk: once a legitimate user grants access, the desktop's local Calendar
API calls still consume shiet project quota if using shiet's OAuth client.
Future work may need per-user quota shaping, finer Google Cloud monitoring, or a
full proxy if abuse requires central API mediation.

### Local Token Theft

Threat: malware or a local attacker steals Google tokens from the user's device.

Controls:

- Desktop stores tokens only in the OS keychain via the existing local token
  store.
- Desktop should avoid logging tokens and should keep token material out of
  frontend-visible state except when strictly needed for provider calls.
- Disconnect performs broker-assisted revoke and then deletes local keychain
  material.

Residual risk: a compromised user device can still access local keychain material
or active process memory. The broker is not intended to solve local device
compromise.

## Deployment Plan

- Domain: provision `auth.shiet.app` with HTTPS, HSTS, and a Google Web OAuth
  redirect URI pinned to the callback endpoint.
- Runtime: deploy a small stateless HTTP service. Any token-bearing record must
  be short-lived and encrypted at rest if it exists outside process memory.
- Secret manager: store Google Web OAuth client id and `client_secret` in a
  managed secret store with least-privilege runtime access and audit logging.
- Datastore: use a minimal TTL-capable store for OAuth state, handoff records,
  rate-limit counters, kill-switch config, and audit metadata. Do not create
  tables for persistent Google refresh tokens, Google access tokens, or Calendar
  events.
- Observability: structured logs with token redaction, metrics for every endpoint
  outcome, dashboards for abuse signals, and alerts for Google quota/error
  spikes.
- Operations: define an owner for secret rotation, Google OAuth consent-screen
  health, broker uptime, quota alerts, incident response, and emergency feature
  disablement.
- Privacy: document that Google token material passes through the broker
  transiently for authorization, refresh, and revocation, while Calendar event
  data remains local in v1.

## Migration Plan

1. Keep the current BYO/local Google credential config for development and
   advanced users: `google.client_id`, `google.client_secret`,
   `SHIET_GOOGLE_CLIENT_ID`, and `SHIET_GOOGLE_CLIENT_SECRET`.
2. Add broker configuration separately, for example `google.auth_mode` with
   values `broker` and `local`, plus `google.broker_base_url`.
3. Public builds default to `broker` mode and must not embed a shared
   `client_secret`.
4. Development builds may default to `local` mode when local credentials are
   present, or `broker` mode when exercising production-like auth.
5. Existing local tokens stay in the OS keychain. Users do not need to reconnect
   until a token refresh requires the broker or the configured auth mode changes.
6. If a user switches from BYO/local to broker mode, require a fresh Google
   authorization because the OAuth client id changes.
7. If the broker is unavailable, show a clear retryable auth/refresh error.
   Existing local Calendar data remains readable.

## Consequences

- shiet gains a deployable path for public Google Calendar auth without shipping
  a shared Google client secret.
- shiet does not become a hosted calendar-sync product in this version.
- The broker becomes a security-sensitive service even though it does not persist
  Google tokens.
- The desktop app's local token store remains the long-term holder of user
  Google tokens.
- Broker binary (`cmd/oauth-broker`), Railway deploy config, desktop
  broker/local auth modes, and operator runbook (`docs/oauth-broker.md`) are in
  tree; ongoing work is ops (domain, secrets, monitoring) and hardening.

## References

- [RFC 8252: OAuth 2.0 for Native Apps](https://www.rfc-editor.org/rfc/rfc8252.html),
  especially native external browser flow, loopback redirects, PKCE, client
  authentication, and client impersonation.
- [Google Identity: OAuth 2.0 for Web Server Applications](https://developers.google.com/identity/protocols/oauth2/web-server),
  especially server-side use of `client_secret` during authorization-code
  exchange.
- [IETF draft: OAuth 2.0 for Browser-Based Applications](https://datatracker.ietf.org/doc/html/draft-ietf-oauth-browser-based-apps),
  especially BFF and token-mediating backend architecture patterns.
