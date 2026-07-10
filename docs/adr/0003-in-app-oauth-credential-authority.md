# ADR-0003: In-app OAuth credential authority

## Status

Accepted (2026-07-10). Implements the design grilled on DYL-124.

Supersedes the YAML/env-primary BYO path in
[ADR-0001](0001-secret-only-google-oauth-broker.md) Migration Plan steps 1–2
for desktop OAuth client credentials and per-provider auth mode.

## Context

BYO OAuth `client_id` / `client_secret` and `auth_mode` lived in layered
YAML/env, loaded once at startup. User tokens already live in the OS keychain
(DYL-44). That split made debugging painful: edit config, restart, reconnect,
and no UI showed the active credential source.

Integrations settings ([ADR-0002](0002-standardized-integrations-settings-surface.md))
is the single Settings surface for providers. Auth mode and BYO credentials
belong there as a provider-keyed concern — not three one-off panels and not
per-provider config keys.

## Decision

1. **In-app authority for mode + BYO credentials** across Google, GitHub, and
   Slack. One provider-keyed design; do not ship Google-only then copy-paste.
2. **BYO secrets** live in the OS keychain under the existing `"shiet"` service
   with namespaced keys `{provider}:oauth_app`. User tokens keep
   `{provider}:{accountID}`.
3. **Non-secret UI metadata** (configured flag, masked `client_id` hint,
   `updated_at`) may live in SQLite (`app_setting` or equivalent). Never store
   `client_secret` in SQLite or YAML as the primary path.
4. **Remove per-provider OAuth keys from config/env**: `auth_mode`,
   `client_id`, `client_secret`, and per-provider `broker_base_url` /
   `SHIET_*` equivalents are not the supported path.
5. **Single top-level broker URL**: code default `https://auth.shiet.app`.
   Optional escape hatch only: `broker.base_url` / `SHIET_BROKER_BASE_URL`.
   No Settings field for broker URL in v1.
6. **Default auth mode** is broker when nothing is saved in-app.
7. **UI** mounts on Integrations detail (auth-mode / credentials block), not
   legacy per-provider settings tabs.
8. **v1 switch UX**: persist mode/creds, warn that existing connections need
   reconnect, user reconnects via Connect — no full app restart. Live
   in-process provider hot-swap is a later enhancement.

## Consequences

- Public builds stay broker-first without shipping a shared `client_secret`.
- Self-host operators override broker URL via one env/config key.
- ADR-0001 trust boundary (broker vs local/BYO) is unchanged; only where
  desktop BYO credentials and mode are stored/edited changes.
- Config docs and `config.example.yaml` must drop per-provider OAuth credential
  keys as the documented path.
