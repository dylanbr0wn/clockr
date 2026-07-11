# ADR-0003: Platform Adapters Behind Connect Handlers

- Status: Accepted
- Date: 2026-07-10
- Linear: DYL-110

## Context

shiet targets one frontend client for desktop today and a hosted web product later.
[ADR-0002](0002-connect-protobuf-api-boundary.md) makes Connect the sole
application API between frontend and backend. Some operations still need
platform-specific behavior at implementation time:

- OAuth connect opens a system browser and may use a loopback redirect (desktop)
  or a broker redirect in the same browser tab (hosted).
- Tokens are stored in the OS keychain on desktop; a hosted deployment will use
  a different encrypted store.
- Export may use a native save dialog on desktop and a browser download on web.

These differences are **implementation adapters**, not separate frontend API
surfaces. The mistake to avoid is exposing them as Wails JavaScript bindings
alongside Connect.

## Decision

Connect handlers own the application contract. Platform differences are hidden
behind small Go interfaces wired at process startup:

| Adapter | Desktop (today) | Hosted (future) |
|---------|-----------------|-----------------|
| `TokenStore` | OS keychain (`internal/integration/secrets`) | Encrypted server-side store (TBD) |
| `BrowserOpener` | `browser.OpenURL` from handler during OAuth | Redirect URL returned to frontend or same-tab navigation |
| `OAuthAuthorizer` | Loopback PKCE, broker handoff, or PAT path via `internal/integration/{provider}` | Broker-only or provider-hosted redirect flow |
| `FileExporter` | Native save dialog (optional thin bridge) | Browser download of rendered bytes |

Rules:

1. **No Wails methods for business operations.** Integration connect/disconnect,
   auth introspection, AI settings, and catalog reads are Connect RPCs. Wails is
   the desktop shell (webview + lifecycle), not the application transport.
2. **OAuth provider and broker callbacks stay HTTP GET routes.** Providers
   navigate the user agent to fixed URLs; that is not an RPC shape. See
   [ADR-0001](0001-secret-only-google-oauth-broker.md).
3. **Handlers delegate to existing packages.** Connect handlers in
   `internal/api/appapi` call `internal/integration/*` providers and
   `service.Service`; they do not duplicate business rules.
4. **Long-running connect RPCs.** `ConnectIntegration` may block while the user
   completes OAuth on desktop. Unary Connect is acceptable for the desktop
   product; if hosted flows need async completion, add
   `StartIntegrationConnect` / `CompleteIntegrationConnect` in a later contract
   revision without changing the adapter boundary.

### Minimal native bridge (desktop only)

A single optional Wails export is permitted when Connect cannot carry the UX:

- **Native save path picker** — after `RenderPeriodExport` returns content via
  Connect, desktop may call a tiny `PickSavePath` / `SaveExportFile` bridge.
  Prefer returning bytes over Connect and using browser download on web.

Do not add further Wails-bound application methods.

## Consequences

- Desktop and hosted frontends share generated Connect TypeScript clients.
- Provider connect logic remains in `internal/integration/*`; only wiring and
  adapter injection differ by deployment.
- DYL-113's Wails-based integration generalization is superseded by **DYL-125**.

## References

- [ADR-0001: Secret-Only OAuth Broker](0001-secret-only-google-oauth-broker.md)
- [ADR-0002: Connect and Protobuf API Boundary](0002-connect-protobuf-api-boundary.md)
- [ADR-0004: Standardized Integrations Settings Surface](0004-standardized-integrations-settings-surface.md)
