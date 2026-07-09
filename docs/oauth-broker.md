# shiet OAuth Broker

The OAuth broker is a separate deployable Go binary:

```bash
go run ./cmd/oauth-broker
```

It keeps the Google Web OAuth client secret in the server environment and stores
only short-lived coordination records in the configured datastore. It does not
create durable tables for Google access tokens, Google refresh tokens, or
Calendar event data.

## Environment

Required:

- `SHIET_BROKER_PUBLIC_ORIGIN`: public HTTPS origin, for example
  `https://auth.shiet.app`.
- `SHIET_BROKER_GOOGLE_CLIENT_ID`: Google Web OAuth client id.
- `SHIET_BROKER_GOOGLE_CLIENT_SECRET`: Google Web OAuth client secret.
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

## Deployment Notes

- HTTPS/domain: provision `auth.shiet.app` with HTTPS and HSTS. Configure the
  Google Web OAuth redirect URI as
  `https://auth.shiet.app/v1/google/oauth/callback`.
- Secret management: inject the Google client id and client secret from a
  managed secret store. Restrict runtime access and keep audit logs for reads
  and rotations.
- Datastore: start with a small SQLite database on durable storage for the first
  deployable broker. The schema contains OAuth state and handoff coordination
  records with expiry and one-time-use fields only.
- Logging: keep authorization codes, handoff codes, Google access tokens, and
  Google refresh tokens out of logs, metrics labels, traces, and error messages.
- Metrics: track `/healthz`, `/readyz`, start attempts, datastore failures,
  callback/handoff/refresh/revoke outcomes, expiry counts, and one-time-use
  replay attempts.
- Operational ownership: assign an owner for deploys, uptime, datastore backups,
  secret rotation, Google OAuth consent-screen health, quota alerts, and
  emergency disablement.

## Railway

This repo includes Railway config-as-code for the broker service:

- `railway.json`: selects the broker Dockerfile and `/readyz` healthcheck.
- `deploy/railway/oauth-broker.Dockerfile`: builds only `./cmd/oauth-broker`.
- `.railwayignore`: keeps the Wails frontend/build artifacts out of Railway's
  upload context.

Railway injects a `PORT` environment variable at runtime. The broker listens on
that port when `SHIET_BROKER_LISTEN_ADDR` is not set.

Recommended Railway service variables:

- `SHIET_BROKER_PUBLIC_ORIGIN=https://auth.shiet.app`
- `SHIET_BROKER_GOOGLE_CLIENT_ID=<Google Web OAuth client id>`
- `SHIET_BROKER_GOOGLE_CLIENT_SECRET=<Google Web OAuth client secret>`
- `SHIET_BROKER_DATASTORE_DSN=file:/data/oauth-broker.sqlite`

Mark the Google client secret as a sealed Railway variable. Attach a Railway
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

- `GET /healthz`: process health.
- `GET /readyz`: validates configuration and checks datastore connectivity.
- `POST /v1/google/oauth/start`: creates a short-lived broker state and returns
  a Google authorization URL.
- `GET /v1/google/oauth/callback`: exchanges Google's authorization code with
  the server-side client secret, mints a short-lived one-time handoff, and
  renders a return page with broker_state + handoff_code (no token material).
- `POST /v1/google/oauth/handoff`: exchanges a one-time handoff for Google token
  material bound to the initiating desktop session and handoff verifier.
- `POST /v1/google/oauth/refresh`: exchanges a desktop-held Google refresh token
  for new access-token material using the server-side client secret. Does not
  persist submitted or returned token material.
- `POST /v1/google/oauth/revoke`: accepts a desktop-supplied Google refresh
  token, calls Google's revoke endpoint, and returns `{ "revoked": true }`.
  Already-revoked / `invalid_token` responses are treated as success. The
  broker does not persist the token or any disconnected-account record.
